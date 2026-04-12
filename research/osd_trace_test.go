// osd_trace_test.go — Step-by-step OSD trace matching Fortran dump_osd_trace_bin.
// Compares every intermediate value to find the exact divergence point.

package research

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"testing"
)

func TestOSDTrace(t *testing.T) {
	const bmetPath = "../bmet_cand9.bin"
	if _, err := os.Stat(bmetPath); os.IsNotExist(err) {
		t.Skipf("binary bmet not found: %s", bmetPath)
	}
	data, err := os.ReadFile(bmetPath)
	if err != nil {
		t.Fatalf("read bmet: %v", err)
	}

	// Read bmeta (first 174 float32 values)
	var bmeta [174]float32
	for i := 0; i < 174; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : i*4+4])
		bmeta[i] = math.Float32frombits(bits)
	}

	const (
		n = LDPCn // 174
		k = LDPCk // 91
		m = LDPCm // 83
	)

	// Build LLR (matching Fortran: llr = scalefac * bmeta — float32 throughout)
	sf32 := float32(ScaleFac)
	var llr [n]float64
	for i := 0; i < n; i++ {
		llr[i] = float64(sf32 * bmeta[i]) // float32 multiply then widen — matches rx truncation in osdDecode
	}

	// Check bmet and LLR values
	t.Logf("bmeta[0:5] = [%.8f, %.8f, %.8f, %.8f, %.8f]",
		bmeta[0], bmeta[1], bmeta[2], bmeta[3], bmeta[4])
	t.Logf("llr[0:5] = [%.8f, %.8f, %.8f, %.8f, %.8f]",
		llr[0], llr[1], llr[2], llr[3], llr[4])
	t.Logf("abs(llr)[0:5] = [%.8f, %.8f, %.8f, %.8f, %.8f]",
		math.Abs(llr[0]), math.Abs(llr[1]), math.Abs(llr[2]), math.Abs(llr[3]), math.Abs(llr[4]))

	// Find and print the max |LLR| position
	maxVal := 0.0
	maxIdx := 0
	for i, v := range llr {
		if math.Abs(v) > maxVal {
			maxVal = math.Abs(v)
			maxIdx = i
		}
	}
	t.Logf("Max |LLR|: idx=%d (1-based=%d) val=%.8f  bmeta[%d]=%.8f",
		maxIdx, maxIdx+1, maxVal, maxIdx, bmeta[maxIdx])

	// Fortran says max is at origIdx=92 (1-based = 91 0-based) with absrx=5.376
	t.Logf("Fortran max position check: llr[91]=%.8f abs=%.8f bmeta[91]=%.8f",
		llr[91], math.Abs(llr[91]), bmeta[91])

	// Check Go's Keff — osdFullGenerator takes keff parameter
	t.Logf("LDPCk=%d (keff passed to generator)", k)

	// Build generator matrix (same as osdDecode)
	gen := osdFullGenerator(k)

	// Hard decisions
	var hdec [n]int8
	for i := 0; i < n; i++ {
		if llr[i] >= 0 {
			hdec[i] = 1
		}
	}

	// Sort by ascending |LLR| — truncate to float32 matching Fortran
	absrx := make([]float64, n)
	for i := range absrx {
		absrx[i] = float64(float32(math.Abs(llr[i])))
	}
	indx := argsortAsc(absrx)

	// Reorder by decreasing reliability
	var genmrb [k][n]int8
	indices := make([]int, n)
	for i := 0; i < n; i++ {
		ridx := indx[n-1-i]
		for row := 0; row < k; row++ {
			genmrb[row][i] = gen[row][ridx]
		}
		indices[i] = ridx
	}

	// Compare sort order with Fortran
	// Fortran: i=1 origIdx=92 → Go: i=0 origIdx=91 (0-based)
	fortranIdx := []int{92, 93, 102, 100, 95, 97, 96, 101, 147, 103}
	t.Logf("Sort order comparison (first 10):")
	for i := 0; i < 10; i++ {
		goIdx := indices[i] + 1 // convert to 1-based for comparison
		match := ""
		if i < len(fortranIdx) && goIdx == fortranIdx[i] {
			match = " ✓"
		} else if i < len(fortranIdx) {
			match = " ✗ MISMATCH"
		}
		t.Logf("  rank %d: Go origIdx=%d (1-based)  F77 origIdx=%d  absrx=%.8f%s",
			i, goIdx, fortranIdx[i], absrx[indx[n-1-i]], match)
	}

	// Dump pre-GE genmrb and indices
	{
		f, _ := os.Create("/tmp/go_genmrb_pre.txt")
		for r := 0; r < k; r++ {
			for c := 0; c < n; c++ {
				fmt.Fprintf(f, "%d", genmrb[r][c])
			}
			fmt.Fprintln(f)
		}
		f.Close()
		// Print indices around position 70 (where first diff is)
		t.Logf("Pre-GE indices around position 70 (0-based → 1-based):")
		for i := 68; i <= 72; i++ {
			t.Logf("  pos %d: idx=%d (1-based=%d)", i, indices[i], indices[i]+1)
		}
		// Fortran: pos 69→53, pos 70→158, pos 71→150
	}

	// GE
	for id := 0; id < k; id++ {
		found := false
		for icol := id; icol < k+20 && icol < n; icol++ {
			if genmrb[id][icol] == 1 {
				if icol != id {
					for r := 0; r < k; r++ {
						genmrb[r][id], genmrb[r][icol] = genmrb[r][icol], genmrb[r][id]
					}
					indices[id], indices[icol] = indices[icol], indices[id]
				}
				for ii := 0; ii < k; ii++ {
					if ii != id && genmrb[ii][id] == 1 {
						for c := 0; c < n; c++ {
							genmrb[ii][c] ^= genmrb[id][c]
						}
					}
				}
				found = true
				break
			}
		}
		_ = found
	}

	// Print post-GE indices (convert to 1-based for comparison)
	t.Logf("Post-GE indices[0:9] (1-based):")
	for i := 0; i < 10; i++ {
		t.Logf("  i=%d: %d", i, indices[i]+1)
	}
	// Fortran: 92 93 102 100 95 97 96 101 147 103

	// Dump genmrb to file for comparison
	{
		f, _ := os.Create("/tmp/go_genmrb.txt")
		for r := 0; r < k; r++ {
			for c := 0; c < n; c++ {
				fmt.Fprintf(f, "%d", genmrb[r][c])
			}
			fmt.Fprintln(f)
		}
		f.Close()
		t.Logf("Post-GE genmrb written to /tmp/go_genmrb.txt")
	}

	var g2 [n][k]int8
	for r := 0; r < k; r++ {
		for c := 0; c < n; c++ {
			g2[c][r] = genmrb[r][c]
		}
	}

	// Reorder hdec, absrx
	var hdecR [n]int8
	var absR [n]float64
	for i := 0; i < n; i++ {
		hdecR[i] = hdec[indices[i]]
		absR[i] = absrx[indices[i]]
	}

	// Print hdecR for comparison
	t.Logf("hdecR[0:19]:")
	for i := 0; i < 20; i++ {
		t.Logf("  hdecR[%d]=%d", i, hdecR[i])
	}
	// Fortran: 1 0 1 0 0 1 1 0 1 1 1 1 1 0 0 0 1 1 0 1

	var m0 [k]int8
	copy(m0[:], hdecR[:k])

	// Order-0
	c0 := mrbEncode91(m0, g2)

	// Print c0 first 20 and last 20 bits
	t.Logf("c0[0:19]:")
	for i := 0; i < 20; i++ {
		t.Logf("  c0[%d]=%d hdecR[%d]=%d match=%v", i, c0[i], i, hdecR[i], c0[i] == hdecR[i])
	}

	nhardMin := 0
	dmin := 0.0
	for i := 0; i < n; i++ {
		if c0[i] != hdecR[i] {
			nhardMin++
			dmin += absR[i]
		}
	}
	t.Logf("")
	t.Logf("Order-0: nhardmin=%d dmin=%.6f (Fortran: nhardmin=36 dmin=17.040117)", nhardMin, dmin)

	// Order-1 search (ndeep=2: nord=1, npre1=1, nt=40, ntheta=10)
	nt := 40
	ntheta := 10
	misub := make([]int8, k)
	misub[k-1] = 1
	iflag := k - 1 // 0-based: k-1 = 90

	npassed := 0
	ntotal := 0

	for iflag >= 0 {
		iend := 0 // Go 0-based

		var d1 float64
		var e2sub [m]int8

		for n1 := iflag; n1 >= iend; n1-- {
			ntotal++
			var mi [k]int8
			copy(mi[:], misub)
			mi[n1] = 1

			var me [k]int8
			for j := 0; j < k; j++ {
				me[j] = m0[j] ^ mi[j]
			}

			var e2 [m]int8
			var nd1kpt int
			if n1 == iflag {
				ce := mrbEncode91(me, g2)
				for j := 0; j < m; j++ {
					e2sub[j] = ce[k+j] ^ hdecR[k+j]
				}
				copy(e2[:], e2sub[:])
				nd1kpt = 1
				for j := 0; j < nt && j < m; j++ {
					nd1kpt += int(e2sub[j])
				}
				d1 = 0
				for j := 0; j < k; j++ {
					d1 += float64(me[j]^hdecR[j]) * absR[j]
				}
			} else {
				for j := 0; j < m; j++ {
					e2[j] = e2sub[j] ^ g2[k+j][n1]
				}
				nd1kpt = 2
				for j := 0; j < nt && j < m; j++ {
					nd1kpt += int(e2[j])
				}
			}

			if nd1kpt <= ntheta {
				npassed++
				ce := mrbEncode91(me, g2)
				var nxorE [n]int8
				for j := 0; j < n; j++ {
					nxorE[j] = ce[j] ^ hdecR[j]
				}

				var dd float64
				if n1 == iflag {
					dd = d1
					for j := 0; j < m; j++ {
						dd += float64(e2sub[j]) * absR[k+j]
					}
				} else {
					dd = d1 + float64(ce[n1]^hdecR[n1])*absR[n1]
					for j := 0; j < m; j++ {
						dd += float64(e2[j]) * absR[k+j]
					}
				}
				nhard := 0
				for j := 0; j < n; j++ {
					nhard += int(nxorE[j])
				}

				t.Logf("  Passed: n1=%d nhard=%d dd=%.6f nd1kpt=%d iflag=%d",
					n1, nhard, dd, nd1kpt, iflag)

				if dd < dmin {
					t.Logf("  → BETTER (dmin was %.6f)", dmin)
					dmin = dd
					nhardMin = nhard
				}
			}
		}
		iflag = nextpat91(misub, k, 1)
	}

	t.Logf("")
	t.Logf("Order-1: total=%d passed=%d nhardmin=%d", ntotal, npassed, nhardMin)
	t.Logf("(Fortran: total=4186 passed=2 nhardmin=24)")

	if nhardMin == 36 && npassed == 0 {
		t.Errorf("Go OSD found 0 passing candidates — BUG in pre-test or search loop")
	}
}

func TestSortOrderDump(t *testing.T) {
	const bmetPath = "../bmet_cand9.bin"
	data, err := os.ReadFile(bmetPath)
	if err != nil { t.Skip(err) }
	var bmeta [174]float32
	for i := 0; i < 174; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : i*4+4])
		bmeta[i] = math.Float32frombits(bits)
	}
	sf32 := float32(ScaleFac)
	var llr [174]float64
	absrx := make([]float64, 174)
	for i := 0; i < 174; i++ {
		llr[i] = float64(sf32 * bmeta[i])
		absrx[i] = math.Abs(llr[i])
	}
	indx := argsortAsc(absrx)
	f, _ := os.Create("/tmp/go_sort_order.txt")
	for i := 0; i < 174; i++ {
		ridx := indx[174-1-i] // descending
		fmt.Fprintf(f, "%4d %4d %12.8f\n", i+1, ridx+1, absrx[ridx])
	}
	f.Close()
	t.Logf("Sort order written to /tmp/go_sort_order.txt")
}
