package ft8x

import (
	"math"
	"strconv"
	"sync"
)

// BPDecode performs belief-propagation decoding of the LDPC (174,91) code.
//
// llr[i] is the log-likelihood ratio for codeword bit i (positive = bit is 1).
// apmask[i] != 0 means the corresponding LLR is a hard-pinned a-priori value
// that should not be updated by the BP iterations.
// maxIterations caps the number of BP iterations.
//
// On success (codeword found with valid CRC) returns (cw, nHardErrors, true).
// On failure returns (nil, -1, false).
//
// Port of subroutine bpdecode174_91 / the BP section of decode174_91 from
// wsjt-wsjtx/lib/ft8/bpdecode174_91.f90 and decode174_91.f90.
func BPDecode(llr [LDPCn]float64, apmask [LDPCn]int8, maxIterations int) ([LDPCn]int8, int, bool) {
	const (
		n   = LDPCn
		m   = LDPCm
		ncw = LDPCncw
	)

	var cw [n]int8
	var tov [n][ncw]float64 // check→variable messages
	var toc [m][7]float64   // variable→check messages

	// Initialise variable→check messages from channel LLRs.
	for j := 0; j < m; j++ {
		for i := 0; i < LDPCNrw[j]; i++ {
			bit := LDPCNm[j][i] - 1 // 0-indexed
			toc[j][i] = llr[bit]
		}
	}

	ncnt := 0
	nclast := 0

	for iter := 0; iter <= maxIterations; iter++ {
		// Update LLR estimates: zn[i] = llr[i] + sum of incoming check messages.
		var zn [n]float64
		for i := 0; i < n; i++ {
			if apmask[i] != 1 {
				sum := llr[i]
				for k := 0; k < ncw; k++ {
					sum += tov[i][k]
				}
				zn[i] = sum
			} else {
				zn[i] = llr[i]
			}
		}

		// Hard decision and syndrome check.
		for i := 0; i < n; i++ {
			if zn[i] > 0 {
				cw[i] = 1
			} else {
				cw[i] = 0
			}
		}
		ncheck := 0
		for j := 0; j < m; j++ {
			s := 0
			for i := 0; i < LDPCNrw[j]; i++ {
				s += int(cw[LDPCNm[j][i]-1])
			}
			if s%2 != 0 {
				ncheck++
			}
		}

		if ncheck == 0 {
			// Valid codeword; check CRC.
			nHard := 0
			for i := 0; i < n; i++ {
				if (2*int(cw[i])-1)*sign64(llr[i]) < 0 {
					nHard++
				}
			}
			if CheckCRC14Codeword(cw) {
				return cw, nHard, true
			}
		}

		// Early stopping.
		if iter > 0 {
			nd := ncheck - nclast
			if nd < 0 {
				ncnt = 0
			} else {
				ncnt++
			}
			if ncnt >= 5 && iter >= 10 && ncheck > 15 {
				return [LDPCn]int8{}, -1, false
			}
		}
		nclast = ncheck

		// Variable→check messages.
		for j := 0; j < m; j++ {
			for i := 0; i < LDPCNrw[j]; i++ {
				bit := LDPCNm[j][i] - 1
				v := zn[bit]
				// Subtract the contribution this check previously sent to bit.
				for kk := 0; kk < ncw; kk++ {
					if LDPCMn[bit][kk]-1 == j {
						v -= tov[bit][kk]
					}
				}
				toc[j][i] = v
			}
		}

		// Check→variable messages (log-domain sum-product).
		var tanhtoc [m][7]float64
		for j := 0; j < m; j++ {
			for i := 0; i < LDPCNrw[j]; i++ {
				tanhtoc[j][i] = math.Tanh(-toc[j][i] / 2.0)
			}
		}

		for bit := 0; bit < n; bit++ {
			for k := 0; k < ncw; k++ {
				chk := LDPCMn[bit][k] - 1 // 0-indexed check
				// Product of tanh(toc[chk][i]/2) for all i where Nm[chk][i]-1 != bit.
				prod := 1.0
				for i := 0; i < LDPCNrw[chk]; i++ {
					if LDPCNm[chk][i]-1 != bit {
						prod *= tanhtoc[chk][i]
					}
				}
				// tov = 2 * atanh(-prod)
				tov[bit][k] = 2.0 * platanh(-prod)
			}
		}
	}

	return [LDPCn]int8{}, -1, false
}

// platanh is the "protected" atanh used in the BP check→variable message
// update. This is the piecewise-linear approximation from WSJT-X's
// platanh.f90, co-designed with ScaleFac=2.83. It clamps at ±7.0 (much
// tighter than math.Atanh's ±∞), which prevents extreme BP messages and
// improves convergence on weak signals.
//
// Port of subroutine platanh from wsjt-wsjtx/lib/platanh.f90.
func platanh(x float64) float64 {
	sign := 1.0
	z := x
	if x < 0 {
		sign = -1.0
		z = -x
	}
	if z <= 0.664 {
		return x / 0.83
	} else if z <= 0.9217 {
		return sign * (z - 0.4064) / 0.322
	} else if z <= 0.9951 {
		return sign * (z - 0.8378) / 0.0524
	} else if z <= 0.9998 {
		return sign * (z - 0.9914) / 0.0012
	}
	return sign * 7.0
}

func sign64(x float64) int {
	if x >= 0 {
		return 1
	}
	return -1
}

// ────────────────────────────────────────────────────────────────────────────
// Generator matrix (for OSD and encoding)
// ────────────────────────────────────────────────────────────────────────────

var (
	ldpcGenOnce sync.Once
	ldpcGen     [LDPCm][LDPCk]int8 // gen[parity_row][message_col] mod 2
)

// LDPCGenerator returns the (83×91) generator matrix G such that
// parity[i] = (G[i] · message) mod 2.
// The matrix is built once from the hex strings in ldpc_parity.go.
func LDPCGenerator() *[LDPCm][LDPCk]int8 {
	ldpcGenOnce.Do(func() {
		for row, hex := range ldpcGeneratorHex {
			col := 0
			for j, ch := range hex {
				var nib int64
				ibmax := 4
				if j == 22 {
					ibmax = 3 // last nibble: only top 3 bits used (91 = 22*4+3)
				}
				nib, _ = strconv.ParseInt(string(ch), 16, 64)
				for jj := 0; jj < ibmax; jj++ {
					if col >= LDPCk {
						break
					}
					bit := int8((nib >> uint(ibmax-1-jj)) & 1)
					ldpcGen[row][col] = bit
					col++
				}
			}
		}
	})
	return &ldpcGen
}

// Encode174_91NoGRC encodes a 91-bit message (77 message + 14 CRC) into a
// 174-bit codeword without recomputing the CRC.
// message91[0..90] are the message bits; codeword[0..173] receives the result.
func Encode174_91NoGRC(message91 [LDPCk]int8) [LDPCn]int8 {
	gen := LDPCGenerator()
	var codeword [LDPCn]int8
	// First K bits are the message itself.
	for i := 0; i < LDPCk; i++ {
		codeword[i] = message91[i]
	}
	// Remaining M bits are parity.
	for i := 0; i < LDPCm; i++ {
		sum := 0
		for j := 0; j < LDPCk; j++ {
			sum += int(message91[j]) * int(gen[i][j])
		}
		codeword[LDPCk+i] = int8(sum % 2)
	}
	return codeword
}

// OSDDecode is an ordered-statistics decoder (OSD) for the (174,91) code.
// It re-orders bits by reliability, performs Gaussian elimination to find the
// Most Reliable Basis (MRB), then tests order-0 through order-N candidates
// (single-bit, pair-flip, etc.) controlled by the ndeep parameter.
//
//	ndeep=0: order-0 only
//	ndeep=1: order-1 (91 single flips)
//	ndeep=2: order-1 with npre1 (parity pre-test)
//	ndeep=3: order-1 with npre1 + npre2 (hash pair-flip)
//	ndeep=4: order-2 with npre1 + npre2
//	ndeep=5: order-3 with npre1 + npre2
//	ndeep=6: order-4 with npre1 + npre2
//
// Port of subroutine osd174_91 from wsjt-wsjtx/lib/ft8/osd174_91.f90.
func OSDDecode(llr [LDPCn]float64, keff int, apmask [LDPCn]int8, ndeep int) ([LDPCk]int8, [LDPCn]int8, int, bool) {
	const (
		n = LDPCn
		k = LDPCk
		m = LDPCm
	)

	gen := LDPCGenerator()

	// Sort bits by decreasing |LLR| (reliability).
	absLLR := make([]float64, n)
	for i := range absLLR {
		absLLR[i] = math.Abs(llr[i])
	}
	indices := argsortDesc(absLLR) // indices[0] = most reliable bit

	// Hard decisions from received symbols.
	var hdec [n]int8
	for i := 0; i < n; i++ {
		if llr[i] >= 0 {
			hdec[i] = 1
		}
	}

	// Build the full systematic generator matrix G: [k][n].
	var gFull [k][n]int8
	for row := 0; row < k; row++ {
		gFull[row][row] = 1
		for p := 0; p < m; p++ {
			gFull[row][k+p] = gen[p][row]
		}
	}

	// Re-order columns by reliability.
	var g [k][n]int8
	for row := 0; row < k; row++ {
		for col := 0; col < n; col++ {
			g[row][col] = gFull[row][indices[col]]
		}
	}

	// Re-order hard decisions and magnitudes.
	var hdecR [n]int8
	var absR [n]float64
	var apmaskR [n]int8
	for i := 0; i < n; i++ {
		hdecR[i] = hdec[indices[i]]
		absR[i] = absLLR[indices[i]]
		apmaskR[i] = apmask[indices[i]]
	}

	// Gaussian elimination to create systematic form (MRB in first k positions).
	indexMap := make([]int, n)
	copy(indexMap, indices)
	for id := 0; id < k; id++ {
		found := false
		for icol := id; icol < k+20 && icol < n; icol++ {
			if g[id][icol] == 1 {
				if icol != id {
					// Swap columns id and icol.
					for r := 0; r < k; r++ {
						g[r][id], g[r][icol] = g[r][icol], g[r][id]
					}
					indexMap[id], indexMap[icol] = indexMap[icol], indexMap[id]
					hdecR[id], hdecR[icol] = hdecR[icol], hdecR[id]
					absR[id], absR[icol] = absR[icol], absR[id]
					apmaskR[id], apmaskR[icol] = apmaskR[icol], apmaskR[id]
				}
				// Eliminate column id from other rows.
				for r2 := 0; r2 < k; r2++ {
					if r2 != id && g[r2][id] == 1 {
						for c := 0; c < n; c++ {
							g[r2][c] ^= g[id][c]
						}
					}
				}
				found = true
				break
			}
		}
		if !found {
			break
		}
	}

	// Transpose g for fast encoding: g2[col][row].
	var g2 [n][k]int8
	for r := 0; r < k; r++ {
		for c := 0; c < n; c++ {
			g2[c][r] = g[r][c]
		}
	}

	// Order-0: hard decisions on the k MRB bits.
	var m0 [k]int8
	copy(m0[:], hdecR[:k])

	c0 := mrbEncode(m0, g2, n, k)
	nxor := xorDist(c0, hdecR)
	nhardMin := sum8(nxor[:n])
	dmin := dotProduct(nxor[:n], absR[:n])
	bestCW := reorder(c0, indexMap, n)

	if ndeep == 0 {
		msg91, ok := extractMsg91(bestCW)
		if ok {
			return msg91, bestCW, nhardMin, true
		}
		return [k]int8{}, [n]int8{}, -nhardMin, false
	}

	// Determine search parameters from ndeep (matching Fortran table).
	if ndeep > 6 {
		ndeep = 6
	}
	var nord, npre1, npre2, nt, ntheta, ntau int
	switch ndeep {
	case 1:
		nord, npre1, npre2, nt, ntheta = 1, 0, 0, 40, 12
	case 2:
		nord, npre1, npre2, nt, ntheta = 1, 1, 0, 40, 10
	case 3:
		nord, npre1, npre2, nt, ntheta, ntau = 1, 1, 1, 40, 12, 14
	case 4:
		nord, npre1, npre2, nt, ntheta, ntau = 2, 1, 1, 40, 12, 17
	case 5:
		nord, npre1, npre2, nt, ntheta, ntau = 3, 1, 1, 40, 12, 15
	default: // ndeep=6
		nord, npre1, npre2, nt, ntheta, ntau = 4, 1, 1, 95, 12, 15
	}

	// Order-1..nord: combinatorial flip search with parity pre-test.
	for iorder := 1; iorder <= nord; iorder++ {
		// Initialise pattern: last `iorder` positions set to 1.
		misub := make([]int8, k)
		for i := k - iorder; i < k; i++ {
			misub[i] = 1
		}
		iflag := k - iorder // 0-indexed lowest-1 position (Fortran would be k-iorder+1)

		for iflag >= 0 {
			// Determine scan range for the npre1 optimisation.
			iend := 0
			if iorder == nord && npre1 == 0 {
				iend = iflag
			}

			var d1 float64
			var e2sub [m]int8
			for n1 := iflag; n1 >= iend; n1-- {
				// Build mi = misub with bit n1 also set.
				var mi [k]int8
				copy(mi[:], misub)
				mi[n1] = 1

				// Skip if any flipped bit overlaps an AP-pinned position.
				skip := false
				for j := 0; j < k; j++ {
					if apmaskR[j] == 1 && mi[j] == 1 {
						skip = true
						break
					}
				}
				if skip {
					continue
				}

				// me = m0 XOR mi
				var me [k]int8
				for j := 0; j < k; j++ {
					me[j] = m0[j] ^ mi[j]
				}

				var e2 [m]int8
				var nd1kpt int
				if n1 == iflag {
					ce := mrbEncode(me, g2, n, k)
					for j := 0; j < m; j++ {
						e2sub[j] = ce[k+j] ^ hdecR[k+j]
					}
					copy(e2[:], e2sub[:])
					nd1kpt = 1
					for j := 0; j < nt && j < m; j++ {
						nd1kpt += int(e2sub[j])
					}
					// Compute d1 = distance in the message part.
					d1 = 0
					for j := 0; j < k; j++ {
						d1 += float64(me[j]^hdecR[j]) * absR[j]
					}
				} else {
					// Quick update: XOR with the g2 column for bit n1.
					for j := 0; j < m; j++ {
						e2[j] = e2sub[j] ^ g2[k+j][n1]
					}
					nd1kpt = 2
					for j := 0; j < nt && j < m; j++ {
						nd1kpt += int(e2[j])
					}
				}

				if nd1kpt <= ntheta || (iorder == 1 && n1 == iflag) {
					ce := mrbEncode(me, g2, n, k)
					nxorE := xorDist(ce, hdecR)
					var dd float64
					if n1 == iflag {
						dd = d1 + dotProduct8(e2sub[:], absR[k:k+m])
					} else {
						dd = d1 + float64(ce[n1]^hdecR[n1])*absR[n1] + dotProduct8(e2[:], absR[k:k+m])
					}
					if dd < dmin {
						dmin = dd
						bestCW = reorder(ce, indexMap, n)
						nhardMin = sum8(nxorE[:n])
					}
				}
			}
			// Advance to next pattern of weight iorder.
			iflag = nextpat91(misub, k, iorder)
		}
	}

	// Second pre-processing rule: hash-based pair-flip search.
	if npre2 == 1 {
		// Build hash table: for each pair (i1, i2), compute their combined
		// parity syndrome over the first ntau parity positions and store.
		hashFP := make(map[int][]osdPairEntry)
		for i1 := k - 1; i1 >= 0; i1-- {
			for i2 := i1 - 1; i2 >= 0; i2-- {
				// Compute ntau-bit syndrome pattern.
				ipat := 0
				for t := 0; t < ntau && t < m; t++ {
					bit := g2[k+t][i1] ^ g2[k+t][i2]
					if bit == 1 {
						ipat |= 1 << uint(ntau-1-t)
					}
				}
				hashFP[ipat] = append(hashFP[ipat], osdPairEntry{i1: i1, i2: i2})
			}
		}

		// Run through order-nord patterns and look up pair-flip matches.
		misub2 := make([]int8, k)
		for i := k - nord; i < k; i++ {
			misub2[i] = 1
		}
		iflag2 := k - nord

		for iflag2 >= 0 {
			var me [k]int8
			for j := 0; j < k; j++ {
				me[j] = m0[j] ^ misub2[j]
			}
			ce := mrbEncode(me, g2, n, k)
			var e2sub2 [m]int8
			for j := 0; j < m; j++ {
				e2sub2[j] = ce[k+j] ^ hdecR[k+j]
			}

			for i2t := 0; i2t <= ntau && i2t <= m; i2t++ {
				// Build r2pat = e2sub XOR unit vector at position i2t.
				ipat := 0
				for t := 0; t < ntau && t < m; t++ {
					bit := e2sub2[t]
					if t == i2t && i2t > 0 {
						bit ^= 1
					}
					if bit == 1 {
						ipat |= 1 << uint(ntau-1-t)
					}
				}

				entries := hashFP[ipat]
				for _, ent := range entries {
					in1, in2 := ent.i1, ent.i2
					var mi [k]int8
					copy(mi[:], misub2)
					mi[in1] = 1
					mi[in2] = 1

					// Check weight and AP mask.
					wt := 0
					skip := false
					for j := 0; j < k; j++ {
						wt += int(mi[j])
						if apmaskR[j] == 1 && mi[j] == 1 {
							skip = true
							break
						}
					}
					if skip || wt < nord+npre1+npre2 {
						continue
					}

					var me2 [k]int8
					for j := 0; j < k; j++ {
						me2[j] = m0[j] ^ mi[j]
					}
					ce2 := mrbEncode(me2, g2, n, k)
					nxorE := xorDist(ce2, hdecR)
					dd := dotProduct(nxorE[:n], absR[:n])
					if dd < dmin {
						dmin = dd
						bestCW = reorder(ce2, indexMap, n)
						nhardMin = sum8(nxorE[:n])
					}
				}
			}
			iflag2 = nextpat91(misub2, k, nord)
		}
	}

	msg91, ok := extractMsg91(bestCW)
	if ok {
		return msg91, bestCW, nhardMin, true
	}
	return [k]int8{}, [n]int8{}, -nhardMin, false
}

// osdPairEntry stores a pair of MRB bit indices for the hash-based pair-flip search.
type osdPairEntry struct {
	i1, i2 int
}

// nextpat91 generates the next test error pattern of weight iorder among k positions.
// mi is modified in place. Returns the 0-indexed position of the lowest set bit,
// or -1 when all patterns of this weight have been exhausted.
//
// Port of subroutine nextpat91 from wsjt-wsjtx/lib/ft8/osd174_91.f90 lines 307–334.
func nextpat91(mi []int8, k, iorder int) int {
	// Find the rightmost 0→1 transition (0-indexed).
	ind := -1
	for i := 0; i < k-1; i++ {
		if mi[i] == 0 && mi[i+1] == 1 {
			ind = i
		}
	}
	if ind < 0 {
		return -1 // no more patterns
	}

	ms := make([]int8, k)
	copy(ms, mi[:ind])
	ms[ind] = 1
	// ms[ind+1] stays 0 (already zero from make).
	if ind+1 < k {
		// Count how many 1s we still need at the tail.
		s := 0
		for _, v := range ms {
			s += int(v)
		}
		nz := iorder - s
		for i := k - nz; i < k; i++ {
			ms[i] = 1
		}
	}
	copy(mi, ms)

	// Find the lowest-index 1 bit.
	for i := 0; i < k; i++ {
		if mi[i] == 1 {
			return i
		}
	}
	return -1
}

// dotProduct8 computes the dot product of int8 bits and float64 weights.
func dotProduct8(bits []int8, weights []float64) float64 {
	s := 0.0
	for i := range bits {
		s += float64(bits[i]) * weights[i]
	}
	return s
}

// mrbEncode encodes a k-bit message vector me using the transformed generator g2.
// g2[col][row] is the re-ordered generator matrix.
func mrbEncode(me [LDPCk]int8, g2 [LDPCn][LDPCk]int8, n, k int) [LDPCn]int8 {
	var cw [LDPCn]int8
	for i := 0; i < k; i++ {
		if me[i] == 1 {
			for c := 0; c < n; c++ {
				cw[c] ^= g2[c][i]
			}
		}
	}
	return cw
}

// reorder maps a re-ordered codeword back to the natural bit order.
func reorder(cw [LDPCn]int8, indexMap []int, n int) [LDPCn]int8 {
	var out [LDPCn]int8
	for i := 0; i < n; i++ {
		out[indexMap[i]] = cw[i]
	}
	return out
}

func xorDist(a, b [LDPCn]int8) [LDPCn]int8 {
	var out [LDPCn]int8
	for i := range out {
		out[i] = a[i] ^ b[i]
	}
	return out
}

func sum8(a []int8) int {
	s := 0
	for _, v := range a {
		s += int(v)
	}
	return s
}

func dotProduct(bits []int8, weights []float64) float64 {
	s := 0.0
	for i := range bits {
		s += float64(bits[i]) * weights[i]
	}
	return s
}

// extractMsg91 checks the CRC of cw and, if valid, returns the 91-bit message.
func extractMsg91(cw [LDPCn]int8) ([LDPCk]int8, bool) {
	if !CheckCRC14Codeword(cw) {
		return [LDPCk]int8{}, false
	}
	var msg [LDPCk]int8
	copy(msg[:], cw[:LDPCk])
	return msg, true
}

// argsortDesc returns the indices that would sort vals in descending order.
func argsortDesc(vals []float64) []int {
	n := len(vals)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	// Simple insertion sort – n=174 so quadratic is fine.
	for i := 1; i < n; i++ {
		j := i
		for j > 0 && vals[idx[j-1]] < vals[idx[j]] {
			idx[j-1], idx[j] = idx[j], idx[j-1]
			j--
		}
	}
	return idx
}

// ────────────────────────────────────────────────────────────────────────────
// Combined BP + OSD decoder (decode174_91)
// ────────────────────────────────────────────────────────────────────────────

// DecodeResult holds the output of Decode174_91.
type DecodeResult struct {
	Message91   [LDPCk]int8
	Codeword    [LDPCn]int8
	NHardErrors int
	Dmin        float64
	DecoderType int // 1=BP, 2=OSD
}

// Decode174_91 is the hybrid BP/OSD decoder for the (174,91) code.
//
//	maxOSD < 0: BP only
//	maxOSD = 0: BP then one OSD call with channel LLRs
//	maxOSD > 0: BP then up to maxOSD OSD calls with accumulated LLR sums
//
// ndeep controls OSD search depth (passed to OSDDecode):
//
//	0=order-0, 1=order-1, 2=order-1+pre1, 3=order-1+pre1+pre2,
//	4=order-2+pre1+pre2, 5=order-3+pre, 6=order-4+pre.
//
// Port of subroutine decode174_91 from wsjt-wsjtx/lib/ft8/decode174_91.f90.
func Decode174_91(llr [LDPCn]float64, keff, maxOSD, ndeep int, apmask [LDPCn]int8) (DecodeResult, bool) {
	const (
		n = LDPCn
		m = LDPCm
		k = LDPCk
	)

	if maxOSD > 3 {
		maxOSD = 3
	}

	nosd := 0
	var zsave [3][n]float64
	switch {
	case maxOSD == 0:
		nosd = 1
		zsave[0] = llr
	case maxOSD > 0:
		nosd = maxOSD
	}

	var (
		tov     [n][LDPCncw]float64
		toc     [m][7]float64
		tanhtoc [m][7]float64
	)

	// Initialise variable→check messages.
	for j := 0; j < m; j++ {
		for i := 0; i < LDPCNrw[j]; i++ {
			toc[j][i] = llr[LDPCNm[j][i]-1]
		}
	}

	ncnt := 0
	nclast := 0
	var zsum [n]float64

	for iter := 0; iter <= MaxIterations; iter++ {
		var zn [n]float64
		for i := 0; i < n; i++ {
			if apmask[i] != 1 {
				sum := llr[i]
				for kk := 0; kk < LDPCncw; kk++ {
					sum += tov[i][kk]
				}
				zn[i] = sum
			} else {
				zn[i] = llr[i]
			}
		}
		for i := 0; i < n; i++ {
			zsum[i] += zn[i]
		}
		if iter > 0 && iter <= maxOSD {
			zsave[iter-1] = zsum
		}

		// Hard decision.
		var cw [n]int8
		for i := 0; i < n; i++ {
			if zn[i] > 0 {
				cw[i] = 1
			}
		}

		// Syndrome check.
		ncheck := 0
		for j := 0; j < m; j++ {
			s := 0
			for i := 0; i < LDPCNrw[j]; i++ {
				s += int(cw[LDPCNm[j][i]-1])
			}
			if s%2 != 0 {
				ncheck++
			}
		}

		if ncheck == 0 {
			// Check all-zeros codeword (reject).
			allzero := true
			for _, b := range cw {
				if b != 0 {
					allzero = false
					break
				}
			}
			if allzero {
				break
			}

			// Build m96 for CRC check.
			var m96 [96]int8
			copy(m96[:77], cw[:77])
			copy(m96[82:96], cw[77:91])
			if CRC14Bits(m96[:]) == 0 {
				nHard := 0
				for i := 0; i < n; i++ {
					if float64(2*int(cw[i])-1)*llr[i] < 0 {
						nHard++
					}
				}
				var msg91 [k]int8
				copy(msg91[:], cw[:k])
				// Dmin computation.
				var hdec [n]int8
				for i := 0; i < n; i++ {
					if llr[i] >= 0 {
						hdec[i] = 1
					}
				}
				dmin := 0.0
				for i := 0; i < n; i++ {
					if hdec[i] != cw[i] {
						dmin += math.Abs(llr[i])
					}
				}
				return DecodeResult{
					Message91:   msg91,
					Codeword:    cw,
					NHardErrors: nHard,
					Dmin:        dmin,
					DecoderType: 1,
				}, true
			}
		}

		// Early stopping.
		if iter > 0 {
			nd := ncheck - nclast
			if nd < 0 {
				ncnt = 0
			} else {
				ncnt++
			}
			if ncnt >= 5 && iter >= 10 && ncheck > 15 {
				break
			}
		}
		nclast = ncheck

		// Variable→check.
		for j := 0; j < m; j++ {
			for i := 0; i < LDPCNrw[j]; i++ {
				bit := LDPCNm[j][i] - 1
				v := zn[bit]
				for kk := 0; kk < LDPCncw; kk++ {
					if LDPCMn[bit][kk]-1 == j {
						v -= tov[bit][kk]
					}
				}
				toc[j][i] = v
			}
		}

		// Check→variable.
		for j := 0; j < m; j++ {
			for i := 0; i < LDPCNrw[j]; i++ {
				tanhtoc[j][i] = math.Tanh(-toc[j][i] / 2.0)
			}
		}

		for bit := 0; bit < n; bit++ {
			for kk := 0; kk < LDPCncw; kk++ {
				chk := LDPCMn[bit][kk] - 1
				prod := 1.0
				for i := 0; i < LDPCNrw[chk]; i++ {
					if LDPCNm[chk][i]-1 != bit {
						prod *= tanhtoc[chk][i]
					}
				}
				tov[bit][kk] = 2.0 * platanh(-prod)
			}
		}
	}

	// OSD passes.
	for i := 0; i < nosd; i++ {
		var zIn [n]float64
		if maxOSD == 0 {
			zIn = llr
		} else {
			zIn = zsave[i]
		}
		msg91, cw, nHard, ok := OSDDecode(zIn, keff, apmask, ndeep)
		if ok && nHard > 0 {
			// Compute dmin.
			var hdec [n]int8
			for j := 0; j < n; j++ {
				if llr[j] >= 0 {
					hdec[j] = 1
				}
			}
			dmin := 0.0
			for j := 0; j < n; j++ {
				if hdec[j] != cw[j] {
					dmin += math.Abs(llr[j])
				}
			}
			return DecodeResult{
				Message91:   msg91,
				Codeword:    cw,
				NHardErrors: nHard,
				Dmin:        dmin,
				DecoderType: 2,
			}, true
		}
	}

	return DecodeResult{NHardErrors: -1}, false
}
