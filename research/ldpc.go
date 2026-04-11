// ldpc.go — LDPC decoder for the research package.
//
// Port of subroutine decode174_91 from wsjt-wsjtx/lib/ft8/decode174_91.f90,
// subroutine osd174_91 from wsjt-wsjtx/lib/ft8/osd174_91.f90, and
// subroutine platanh from wsjt-wsjtx/lib/platanh.f90.
//
// Ported directly from the Fortran source — no production ft8x dependency.

package research

import (
	"math"
	"strconv"
	"sync"
)

// ────────────────────────────────────────────────────────────────────────────
// DecodeResult holds the output of Decode174_91.
// ────────────────────────────────────────────────────────────────────────────

// DecodeResult holds the output of Decode174_91.
type DecodeResult struct {
	Message91   [LDPCk]int8
	Codeword    [LDPCn]int8
	NHardErrors int
	Dmin        float64
	DecoderType int // 1=BP, 2=OSD
}

// ────────────────────────────────────────────────────────────────────────────
// platanh — piecewise-linear atanh approximation
// ────────────────────────────────────────────────────────────────────────────

// platanh is the "protected" atanh used in the BP check→variable message update.
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

// ────────────────────────────────────────────────────────────────────────────
// Generator matrix (for OSD and encoding)
// ────────────────────────────────────────────────────────────────────────────

var (
	ldpcGenOnce sync.Once
	ldpcGen     [LDPCm][LDPCk]int8
)

// ldpcGenerator returns the (83×91) generator matrix G such that
// parity[i] = (G[i] · message) mod 2.
// Built once from the hex strings in ldpc_parity.go.
func ldpcGenerator() *[LDPCm][LDPCk]int8 {
	ldpcGenOnce.Do(func() {
		for row, hex := range ldpcGeneratorHex {
			col := 0
			for j, ch := range hex {
				ibmax := 4
				if j == 22 {
					ibmax = 3 // last nibble: only top 3 bits used (91 = 22*4+3)
				}
				nib, _ := strconv.ParseInt(string(ch), 16, 64)
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

// encode174_91NoCRC encodes a 91-bit message into a 174-bit codeword
// without recomputing CRC.
//
// Port of subroutine encode174_91_nocrc from wsjt-wsjtx/lib/ft8/encode174_91.f90.
func encode174_91NoCRC(message91 [LDPCk]int8) [LDPCn]int8 {
	gen := ldpcGenerator()
	var cw [LDPCn]int8
	for i := 0; i < LDPCk; i++ {
		cw[i] = message91[i]
	}
	for i := 0; i < LDPCm; i++ {
		sum := 0
		for j := 0; j < LDPCk; j++ {
			sum += int(message91[j]) * int(gen[i][j])
		}
		cw[LDPCk+i] = int8(sum % 2)
	}
	return cw
}

// ────────────────────────────────────────────────────────────────────────────
// Decode174_91 — hybrid BP/OSD decoder
// ────────────────────────────────────────────────────────────────────────────

// Decode174_91 is the hybrid BP/OSD decoder for the (174,91) code.
//
//	maxOSD < 0: BP only
//	maxOSD = 0: BP then one OSD call with channel LLRs
//	maxOSD > 0: BP then up to maxOSD OSD calls with accumulated LLR sums
//
// ndeep controls OSD search depth (passed to osdDecode):
//
//	0=order-0, 1=order-1, 2=order-1+pre1, 3=order-1+pre1+pre2,
//	4=order-2+pre1+pre2, 5=order-3+pre, 6=order-4+pre.
//
// Port of subroutine decode174_91 from wsjt-wsjtx/lib/ft8/decode174_91.f90.
func Decode174_91(llr [LDPCn]float64, keff, maxOSD, ndeep int, apmask [LDPCn]int8) (DecodeResult, bool) {
	const (
		n             = LDPCn
		m             = LDPCm
		k             = LDPCk
		ncw           = LDPCncw
		maxIterations = 30
	)

	if maxOSD > 3 {
		maxOSD = 3
	}

	// decode174_91.f90 lines 28–37: set up OSD pass count and zsave.
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
		tov     [n][ncw]float64
		toc     [m][7]float64
		tanhtoc [m][7]float64
	)

	// decode174_91.f90 lines 43–47: initialize messages to checks.
	for j := 0; j < m; j++ {
		for i := 0; i < LDPCNrw[j]; i++ {
			toc[j][i] = llr[LDPCNm[j][i]-1]
		}
	}

	ncnt := 0
	nclast := 0
	var zsum [n]float64

	// decode174_91.f90 lines 52–135: BP iterations.
	for iter := 0; iter <= maxIterations; iter++ {
		// Update bit LLR estimates (decode174_91.f90 lines 54–60).
		var zn [n]float64
		for i := 0; i < n; i++ {
			if apmask[i] != 1 {
				sum := llr[i]
				for kk := 0; kk < ncw; kk++ {
					sum += tov[i][kk]
				}
				zn[i] = sum
			} else {
				zn[i] = llr[i]
			}
		}

		// decode174_91.f90 lines 61–64: accumulate zsum, save for OSD.
		for i := 0; i < n; i++ {
			zsum[i] += zn[i]
		}
		if iter > 0 && iter <= maxOSD {
			zsave[iter-1] = zsum
		}

		// Hard decision (decode174_91.f90 lines 67–68).
		var cw [n]int8
		for i := 0; i < n; i++ {
			if zn[i] > 0 {
				cw[i] = 1
			}
		}

		// Syndrome check (decode174_91.f90 lines 69–73).
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

		// decode174_91.f90 lines 74–88: valid codeword → check CRC.
		if ncheck == 0 {
			var m96 [96]int8
			copy(m96[:77], cw[:77])
			copy(m96[82:96], cw[77:91])
			nHard := 0
			for i := 0; i < n; i++ {
				if float64(2*int(cw[i])-1)*llr[i] < 0 {
					nHard++
				}
			}
			if crc14Bits(m96[:]) == 0 {
				var msg91 [k]int8
				copy(msg91[:], cw[:k])
				// Compute dmin (decode174_91.f90 lines 83–85).
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

		// Early stopping (decode174_91.f90 lines 91–104).
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

		// Variable→check messages (decode174_91.f90 lines 108–118).
		for j := 0; j < m; j++ {
			for i := 0; i < LDPCNrw[j]; i++ {
				bit := LDPCNm[j][i] - 1
				v := zn[bit]
				for kk := 0; kk < ncw; kk++ {
					if LDPCMn[bit][kk]-1 == j {
						v -= tov[bit][kk]
					}
				}
				toc[j][i] = v
			}
		}

		// Check→variable messages (decode174_91.f90 lines 121–133).
		for j := 0; j < m; j++ {
			for i := 0; i < 7; i++ {
				tanhtoc[j][i] = math.Tanh(-toc[j][i] / 2.0)
			}
		}

		for bit := 0; bit < n; bit++ {
			for kk := 0; kk < ncw; kk++ {
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

	// OSD passes (decode174_91.f90 lines 137–148).
	for i := 0; i < nosd; i++ {
		var zIn [n]float64
		if maxOSD == 0 {
			zIn = llr
		} else {
			zIn = zsave[i]
		}
		msg91, cw, nHard, ok := osdDecode(zIn, keff, apmask, ndeep)
		if ok && nHard > 0 {
			// Compute dmin (decode174_91.f90 lines 141–144).
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

// ────────────────────────────────────────────────────────────────────────────
// OSD decoder
// ────────────────────────────────────────────────────────────────────────────

// osdDecode is an ordered-statistics decoder for the (174,91) code.
//
// Port of subroutine osd174_91 from wsjt-wsjtx/lib/ft8/osd174_91.f90.
func osdDecode(llr [LDPCn]float64, keff int, apmask [LDPCn]int8, ndeep int) ([LDPCk]int8, [LDPCn]int8, int, bool) {
	const (
		n = LDPCn
		k = LDPCk
		m = LDPCm
	)

	// osd174_91.f90 lines 37–65: build generator matrix (cached).
	gen := osdFullGenerator(keff)

	// osd174_91.f90 lines 67–68: rx = llr, apmaskr = apmask
	rx := llr
	apmaskr := apmask

	// osd174_91.f90 lines 71–72: hard decisions.
	var hdec [n]int8
	for i := 0; i < n; i++ {
		if rx[i] >= 0 {
			hdec[i] = 1
		}
	}

	// osd174_91.f90 lines 75–76: sort by decreasing |LLR| (reliability).
	absrx := make([]float64, n)
	for i := range absrx {
		absrx[i] = math.Abs(rx[i])
	}
	indx := argsortAsc(absrx) // indx[0]=least reliable

	// osd174_91.f90 lines 79–82: reorder generator matrix by decreasing reliability.
	var genmrb [LDPCk][n]int8
	indices := make([]int, n)
	for i := 0; i < n; i++ {
		ridx := indx[n-1-i] // Fortran: indx(N+1-i)
		for row := 0; row < k; row++ {
			genmrb[row][i] = gen[row][ridx]
		}
		indices[i] = ridx
	}

	// osd174_91.f90 lines 86–107: Gaussian elimination.
	for id := 0; id < k; id++ {
		found := false
		for icol := id; icol < k+20 && icol < n; icol++ {
			if genmrb[id][icol] == 1 {
				if icol != id {
					// Swap columns id and icol.
					for r := 0; r < k; r++ {
						genmrb[r][id], genmrb[r][icol] = genmrb[r][icol], genmrb[r][id]
					}
					indices[id], indices[icol] = indices[icol], indices[id]
				}
				// Eliminate column id from other rows.
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
		if !found {
			break
		}
	}

	// osd174_91.f90 line 109: g2 = transpose(genmrb)
	var g2 [n][k]int8
	for r := 0; r < k; r++ {
		for c := 0; c < n; c++ {
			g2[c][r] = genmrb[r][c]
		}
	}

	// osd174_91.f90 lines 117–121: reorder hdec, absrx, apmaskr.
	var hdecR [n]int8
	var absR [n]float64
	var apmaskR [n]int8
	for i := 0; i < n; i++ {
		hdecR[i] = hdec[indices[i]]
		absR[i] = absrx[indices[i]]
		apmaskR[i] = apmaskr[indices[i]]
	}

	// osd174_91.f90 lines 123–128: order-0 codeword.
	var m0 [k]int8
	copy(m0[:], hdecR[:k])

	c0 := mrbEncode91(m0, g2)
	nhardMin := 0
	for i := 0; i < n; i++ {
		if c0[i] != hdecR[i] {
			nhardMin++
		}
	}
	dmin := 0.0
	for i := 0; i < n; i++ {
		if c0[i] != hdecR[i] {
			dmin += absR[i]
		}
	}
	bestCW := c0

	// osd174_91.f90 line 134: if(ndeep.eq.0) goto 998
	if ndeep == 0 {
		return osdFinish(bestCW, indices, nhardMin)
	}

	if ndeep > 6 {
		ndeep = 6
	}

	// osd174_91.f90 lines 136–177: search parameters from ndeep.
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

	// osd174_91.f90 lines 179–228: order-1..nord combinatorial flip search.
	for iorder := 1; iorder <= nord; iorder++ {
		misub := make([]int8, k)
		for i := k - iorder; i < k; i++ {
			misub[i] = 1
		}
		iflag := k - iorder

		for iflag >= 0 {
			iend := 0
			if iorder == nord && npre1 == 0 {
				iend = iflag
			}

			var d1 float64
			var e2sub [m]int8
			for n1 := iflag; n1 >= iend; n1-- {
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

				// Pre-test: bypass for order-1 base patterns (n1==iflag).
				if nd1kpt <= ntheta || (iorder == 1 && n1 == iflag) {
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
					if dd < dmin {
						dmin = dd
						bestCW = ce
						nhardMin = 0
						for j := 0; j < n; j++ {
							nhardMin += int(nxorE[j])
						}
					}
				}
			}
			iflag = nextpat91(misub, k, iorder)
		}
	}

	// osd174_91.f90 lines 230–279: npre2 hash-based pair-flip search.
	if npre2 == 1 {
		// Build hash table (osd174_91.f90 lines 231–239).
		hashFP := make(map[int][]osdPairEntry)
		for i1 := k - 1; i1 >= 0; i1-- {
			for i2 := i1 - 1; i2 >= 0; i2-- {
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

		// osd174_91.f90 lines 245–278: run through order-nord patterns.
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
			ce := mrbEncode91(me, g2)
			var e2sub2 [m]int8
			for j := 0; j < m; j++ {
				e2sub2[j] = ce[k+j] ^ hdecR[k+j]
			}

			for i2t := 0; i2t <= ntau && i2t <= m; i2t++ {
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
					ce2 := mrbEncode91(me2, g2)
					var nxorE [n]int8
					dd := 0.0
					for j := 0; j < n; j++ {
						nxorE[j] = ce2[j] ^ hdecR[j]
						dd += float64(nxorE[j]) * absR[j]
					}
					if dd < dmin {
						dmin = dd
						bestCW = ce2
						nhardMin = 0
						for j := 0; j < n; j++ {
							nhardMin += int(nxorE[j])
						}
					}
				}
			}
			iflag2 = nextpat91(misub2, k, nord)
		}
	}

	return osdFinish(bestCW, indices, nhardMin)
}

// osdFinish re-orders the codeword, checks CRC, and returns the result.
// Port of osd174_91.f90 lines 281–292.
func osdFinish(bestCW [LDPCn]int8, indices []int, nhardMin int) ([LDPCk]int8, [LDPCn]int8, int, bool) {
	const (
		n = LDPCn
		k = LDPCk
	)

	// Re-order to natural bit order (osd174_91.f90 line 283).
	var cwOut [n]int8
	for i := 0; i < n; i++ {
		cwOut[indices[i]] = bestCW[i]
	}

	// Check CRC (osd174_91.f90 lines 286–290).
	var msg91 [k]int8
	copy(msg91[:], cwOut[:k])
	if !checkCRC14Codeword(cwOut) {
		return [k]int8{}, [n]int8{}, -nhardMin, false
	}
	return msg91, cwOut, nhardMin, true
}

// ────────────────────────────────────────────────────────────────────────────
// OSD generator matrix (cached)
// ────────────────────────────────────────────────────────────────────────────

var (
	osdGenOnce sync.Once
	osdGen     [LDPCk][LDPCn]int8
)

// osdFullGenerator builds the full systematic generator matrix [k][n]
// for the OSD decoder. Cached via sync.Once.
//
// Port of osd174_91.f90 lines 37–65: the "if(first)" block.
func osdFullGenerator(keff int) *[LDPCk][LDPCn]int8 {
	osdGenOnce.Do(func() {
		gen := ldpcGenerator()
		// Build full systematic generator: I(k) | G^T
		for row := 0; row < LDPCk; row++ {
			osdGen[row][row] = 1 // identity part
			for p := 0; p < LDPCm; p++ {
				osdGen[row][LDPCk+p] = gen[p][row]
			}
		}
	})
	return &osdGen
}

// ────────────────────────────────────────────────────────────────────────────
// OSD helper types and functions
// ────────────────────────────────────────────────────────────────────────────

type osdPairEntry struct {
	i1, i2 int
}

// mrbEncode91 encodes a k-bit message using the transposed generator g2.
//
// Port of subroutine mrbencode91 from wsjt-wsjtx/lib/ft8/osd174_91.f90 lines 295–305.
func mrbEncode91(me [LDPCk]int8, g2 [LDPCn][LDPCk]int8) [LDPCn]int8 {
	var cw [LDPCn]int8
	for i := 0; i < LDPCk; i++ {
		if me[i] == 1 {
			for c := 0; c < LDPCn; c++ {
				cw[c] ^= g2[c][i]
			}
		}
	}
	return cw
}

// nextpat91 generates the next test error pattern of weight iorder among k positions.
// mi is modified in place. Returns the 0-indexed position of the lowest set bit,
// or -1 when all patterns have been exhausted.
//
// Port of subroutine nextpat91 from wsjt-wsjtx/lib/ft8/osd174_91.f90 lines 307–334.
func nextpat91(mi []int8, k, iorder int) int {
	ind := -1
	for i := 0; i < k-1; i++ {
		if mi[i] == 0 && mi[i+1] == 1 {
			ind = i
		}
	}
	if ind < 0 {
		return -1
	}

	ms := make([]int8, k)
	copy(ms, mi[:ind])
	ms[ind] = 1
	// ms[ind+1] stays 0.
	if ind+1 < k {
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

	for i := 0; i < k; i++ {
		if mi[i] == 1 {
			return i
		}
	}
	return -1
}

// argsortAsc returns indices that sort vals in ascending order.
// This matches Fortran's indexx subroutine.
func argsortAsc(vals []float64) []int {
	n := len(vals)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	// Insertion sort — n=174 so quadratic is fine.
	for i := 1; i < n; i++ {
		j := i
		for j > 0 && vals[idx[j-1]] > vals[idx[j]] {
			idx[j-1], idx[j] = idx[j], idx[j-1]
			j--
		}
	}
	return idx
}
