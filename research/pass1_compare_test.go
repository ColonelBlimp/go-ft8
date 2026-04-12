// pass1_compare_test.go — Compare Go pass 1 decode results against Fortran.
//
// Fortran dump_pass1 on Cap 1 (ndepth=2, maxosd=0, no subtraction) decodes 8:
//   cand  1: 1862.500 Hz  xdt=-0.070  nsync=19  nhard= 6
//   cand  2: 1309.875 Hz  xdt= 0.145  nsync=21  nhard=10
//   cand  7: 1903.125 Hz  xdt= 0.025  nsync=17  nhard=18
//   cand  9: 2098.375 Hz  xdt= 0.085  nsync=16  nhard=24  ← RA1OHX
//   cand 11: 1692.250 Hz  xdt=-0.030  nsync=18  nhard=20
//   cand 21:  949.500 Hz  xdt=-0.060  nsync=18  nhard=13
//   cand 37:  579.125 Hz  xdt= 0.065  nsync=10  nhard=30  ← A61CK W3DQS
//   cand 50: 2208.875 Hz  xdt= 1.075  nsync=10  nhard=32  ← HZ1TT RU1AB

package research

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

// fortranPass1 are the 8 signals Fortran decodes on pass 1 of Cap 1.
var fortranPass1 = []struct {
	CandIdx int     // 1-indexed Fortran candidate number
	Freq    float64 // refined f1
	XDT     float64 // refined xdt
	Nsync   int
	Nhard   int
}{
	{1, 1862.500, -0.070, 19, 6},
	{2, 1309.875, 0.145, 21, 10},
	{7, 1903.125, 0.025, 17, 18},
	{9, 2098.375, 0.085, 16, 24},
	{11, 1692.250, -0.030, 18, 20},
	{21, 949.500, -0.060, 18, 13},
	{37, 579.125, 0.065, 10, 30},
	{50, 2208.875, 1.075, 10, 32},
}

// TestLLRCompareCand9 compares Go and Fortran LLR values for candidate 9.
func TestLLRCompareCand9(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LLR comparison (slow)")
	}

	const wavPath = "../testdata/ft8test_capture_20260410.wav"
	const llrPath = "../llr_cand9.txt"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}
	if _, err := os.Stat(llrPath); os.IsNotExist(err) {
		t.Skipf("Fortran LLR file not found: %s (run dump_llr first)", llrPath)
	}

	// Load Fortran LLR values
	fdata, err := os.ReadFile(llrPath)
	if err != nil {
		t.Fatalf("read LLR file: %v", err)
	}
	var fortranLLR [4][174]float64
	lines := strings.Split(strings.TrimSpace(string(fdata)), "\n")
	for _, line := range lines {
		var idx int
		var a, b, c, d float64
		n, _ := fmt.Sscanf(line, "%d %f %f %f %f", &idx, &a, &b, &c, &d)
		if n == 5 && idx >= 1 && idx <= 174 {
			fortranLLR[0][idx-1] = a
			fortranLLR[1][idx-1] = b
			fortranLLR[2][idx-1] = c
			fortranLLR[3][idx-1] = d
		}
	}

	// Run Go pipeline
	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}
	ddNorm := make([]float32, NMAX)
	for i := 0; i < NMAX; i++ {
		ddNorm[i] = dd[i] / 32768.0
	}

	cands, _ := Sync8(dd, NMAX, 200, 2600, 1.3, 0, 600)
	// Candidate 9 (1-indexed) = cands[8]
	cand := cands[8]
	t.Logf("Candidate 9: freq=%.3f DT=%.4f", cand.Freq, cand.DT)

	ds := NewDownsampler()
	newdat := true
	cd0 := ds.Downsample(ddNorm, &newdat, cand.Freq)

	twopi := 2.0 * math.Pi
	i0 := int(math.Round((cand.DT + 0.5) * Fs2))
	var ctwk [32]complex128
	smax := 0.0
	ibest := 0
	for idt := i0 - 10; idt <= i0+10; idt++ {
		sync := Sync8d(cd0, idt, ctwk, 0)
		if sync > smax {
			smax = sync
			ibest = idt
		}
	}

	smax = 0.0
	delfbest := 0.0
	for ifr := -5; ifr <= 5; ifr++ {
		delf := float64(ifr) * 0.5
		dphi := twopi * delf * Dt2
		phi := 0.0
		for i := 0; i < 32; i++ {
			sin, cos := math.Sincos(phi)
			ctwk[i] = complex(cos, sin)
			phi = math.Mod(phi+dphi, twopi)
		}
		sync := Sync8d(cd0, ibest, ctwk, 1)
		if sync > smax {
			smax = sync
			delfbest = delf
		}
	}
	f1 := cand.Freq + delfbest
	a := [5]float64{-delfbest, 0, 0, 0, 0}
	cd0 = TwkFreq1(cd0, Fs2, a)
	noNewdat := false
	cd0 = ds.Downsample(ddNorm, &noNewdat, f1)

	var ss [9]float64
	for idt := -4; idt <= 4; idt++ {
		ss[idt+4] = Sync8d(cd0, ibest+idt, ctwk, 0)
	}
	imax := 0
	for i := 1; i < 9; i++ {
		if ss[i] > ss[imax] {
			imax = i
		}
	}
	ibest = imax - 4 + ibest
	t.Logf("ibest=%d  f1=%.3f", ibest, f1)

	cs, _ := ComputeSymbolSpectra(cd0, ibest)
	bmeta, bmetb, bmetc, bmetd := ComputeSoftMetrics(&cs)

	goLLR := [4][174]float64{}
	for i := 0; i < 174; i++ {
		goLLR[0][i] = ScaleFac * bmeta[i]
		goLLR[1][i] = ScaleFac * bmetb[i]
		goLLR[2][i] = ScaleFac * bmetc[i]
		goLLR[3][i] = ScaleFac * bmetd[i]
	}

	// Compare
	names := []string{"bmeta", "bmetb", "bmetc", "bmetd"}
	for p := 0; p < 4; p++ {
		maxDiff := 0.0
		maxDiffIdx := 0
		diffCount := 0
		for i := 0; i < 174; i++ {
			diff := math.Abs(goLLR[p][i] - fortranLLR[p][i])
			if diff > maxDiff {
				maxDiff = diff
				maxDiffIdx = i
			}
			if diff > 1e-4 {
				diffCount++
			}
		}
		t.Logf("%s: maxDiff=%.6e at idx=%d  values>1e-4 apart: %d/174",
			names[p], maxDiff, maxDiffIdx, diffCount)

		// Show largest differences
		if diffCount > 0 {
			t.Logf("  Largest diffs in %s:", names[p])
			type diffEntry struct {
				idx  int
				go_  float64
				f_   float64
				diff float64
			}
			var diffs []diffEntry
			for i := 0; i < 174; i++ {
				d := math.Abs(goLLR[p][i] - fortranLLR[p][i])
				if d > 1e-4 {
					diffs = append(diffs, diffEntry{i, goLLR[p][i], fortranLLR[p][i], d})
				}
			}
			// Show up to 10 largest
			for j := 0; j < len(diffs) && j < 10; j++ {
				best := j
				for k := j + 1; k < len(diffs); k++ {
					if diffs[k].diff > diffs[best].diff {
						best = k
					}
				}
				diffs[j], diffs[best] = diffs[best], diffs[j]
				d := diffs[j]
				t.Logf("    idx=%3d  Go=%+10.6f  F77=%+10.6f  diff=%+.6e",
					d.idx, d.go_, d.f_, d.go_-d.f_)
			}
		}
	}

	// Check sort order — are there near-ties that could swap between Go and Fortran?
	t.Logf("")
	t.Logf("Reliability sort order comparison:")
	absGo := make([]float64, 174)
	absF77 := make([]float64, 174)
	for i := 0; i < 174; i++ {
		absGo[i] = math.Abs(goLLR[0][i])
		absF77[i] = math.Abs(fortranLLR[0][i])
	}
	indxGo := argsortAsc(absGo)
	indxF77 := argsortAsc(absF77)

	mismatches := 0
	for i := 0; i < 174; i++ {
		if indxGo[i] != indxF77[i] {
			mismatches++
			if mismatches <= 10 {
				t.Logf("  Sort mismatch at rank %d: Go idx=%d (|llr|=%.8f)  F77 idx=%d (|llr|=%.8f)",
					i, indxGo[i], absGo[indxGo[i]], indxF77[i], absF77[indxF77[i]])
			}
		}
	}
	t.Logf("  Total sort order mismatches: %d / 174", mismatches)

	// Check order-0 OSD metrics
	t.Logf("")
	t.Logf("Order-0 OSD comparison (Fortran: nhardmin=36, dmin=17.0401):")
	{
		absrxG := make([]float64, 174)
		for i := 0; i < 174; i++ {
			absrxG[i] = math.Abs(goLLR[0][i])
		}
		indxG := argsortAsc(absrxG)

		// hard decisions
		var hdec [174]int8
		for i := 0; i < 174; i++ {
			if goLLR[0][i] >= 0 {
				hdec[i] = 1
			}
		}

		// reorder
		var hdecR [174]int8
		var absR [174]float64
		for i := 0; i < 174; i++ {
			ridx := indxG[174-1-i]
			hdecR[i] = hdec[ridx]
			absR[i] = absrxG[ridx]
		}

		// order-0 nhard
		nhard0 := 0
		dmin0 := 0.0
		// Count XOR of m0-encoded codeword with hdecR
		// (This matches Go's mrbEncode91 usage)
		// We'll just count nhard0 from the values directly
		for i := 0; i < 174; i++ {
			// Simple check: count hard errors
			if goLLR[0][indxG[174-1-i]] < 0 {
				// hdecR[i] = 0
			} else {
				// hdecR[i] = 1
			}
		}
		// Actually, just report what our decoder produces
		_ = nhard0
		_ = dmin0
		_ = hdecR
		_ = absR
		t.Logf("  (Need to trace OSD internals for exact comparison)")
	}

	// Try feeding Fortran LLRs directly to Go OSD to check if the decoder itself works
	t.Logf("")
	t.Logf("Decode with FORTRAN LLRs fed into Go decoder:")
	for p := 0; p < 4; p++ {
		var llrz [LDPCn]float64
		var apmask [LDPCn]int8
		copy(llrz[:], fortranLLR[p][:])
		res, ok := Decode174_91(llrz, LDPCk, 0, 2, apmask)
		t.Logf("  %s: ok=%v nhard=%d", names[p], ok, res.NHardErrors)
	}

	t.Logf("")
	t.Logf("Decode with GO LLRs:")
	for p := 0; p < 4; p++ {
		var llrz [LDPCn]float64
		var apmask [LDPCn]int8
		copy(llrz[:], goLLR[p][:])
		res, ok := Decode174_91(llrz, LDPCk, 0, 2, apmask)
		t.Logf("  %s: ok=%v nhard=%d", names[p], ok, res.NHardErrors)
	}
}

func TestPass1Compare(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping pass1 comparison (slow)")
	}

	const wavPath = "../testdata/ft8test_capture_20260410.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// Normalized audio for decode pipeline
	ddNorm := make([]float32, NMAX)
	for i := 0; i < NMAX; i++ {
		ddNorm[i] = dd[i] / 32768.0
	}

	// Sync8 on raw audio (matching Fortran)
	cands, _ := Sync8(dd, NMAX, 200, 2600, 1.3, 0, 600)
	t.Logf("Go sync8: %d candidates", len(cands))

	// Decode each candidate with ndepth=2 (maxosd=0), no AP, no subtraction
	params := DecodeParams{
		Depth: 2, // ndepth=2 → maxosd=0
	}

	goDecoded := make(map[string]struct {
		freq  float64
		xdt   float64
		nhard int
		cand  int
	})

	ds := NewDownsampler()
	for i, cand := range cands {
		newdat := (i == 0)
		result, ok := DecodeSingle(ddNorm, ds, cand.Freq, cand.DT, newdat, params)
		if !ok {
			continue
		}
		msg := strings.TrimSpace(result.Message)
		if _, dup := goDecoded[msg]; dup {
			continue
		}
		goDecoded[msg] = struct {
			freq  float64
			xdt   float64
			nhard int
			cand  int
		}{result.Freq, result.DT, result.NHardErrors, i + 1}
		t.Logf("  Go cand %3d: %7.3f Hz  xdt=%+.3f  nhard=%2d  %s",
			i+1, result.Freq, result.DT, result.NHardErrors, msg)
	}

	t.Logf("")
	t.Logf("Go pass 1: %d decodes  (Fortran: 8)", len(goDecoded))

	// Check which Fortran signals Go decoded
	t.Logf("")
	t.Logf("Fortran pass 1 signals — Go match:")
	for _, f := range fortranPass1 {
		found := false
		for msg, g := range goDecoded {
			if math.Abs(g.freq-f.Freq) < 2.0 {
				t.Logf("  ✓ cand %2d %7.3f Hz nhard=%2d — Go: %7.3f Hz nhard=%2d  %s",
					f.CandIdx, f.Freq, f.Nhard, g.freq, g.nhard, msg)
				found = true
				break
			}
		}
		if !found {
			t.Logf("  ✗ cand %2d %7.3f Hz nhard=%2d — NOT DECODED BY GO",
				f.CandIdx, f.Freq, f.Nhard)
		}
	}
}
