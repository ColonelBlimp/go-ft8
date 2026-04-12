// ldpc_f32.go — Float32 variant of the LDPC BP decoder for the research package.
//
// This is a precision-matched port of subroutine decode174_91 from
// wsjt-wsjtx/lib/ft8/decode174_91.f90, using float32 for all BP internal
// arrays (tov, toc, tanhtoc, zn, zsum, zsave) to match Fortran's default
// "real" type. The OSD path remains float64.

package research

import (
	"math"
)

// Decode174_91_F32 is the hybrid BP/OSD decoder for the (174,91) code,
// using float32 precision for the BP iterations to match the Fortran
// implementation's default real type.
//
// Parameters and return values are identical to Decode174_91.
func Decode174_91_F32(llr [LDPCn]float64, keff, maxOSD, ndeep int, apmask [LDPCn]int8) (DecodeResult, bool) {
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

	// Convert input LLR to float32.
	var llr32 [n]float32
	for i := 0; i < n; i++ {
		llr32[i] = float32(llr[i])
	}

	// decode174_91.f90 lines 28–37: set up OSD pass count and zsave.
	nosd := 0
	var zsave [3][n]float32
	switch {
	case maxOSD == 0:
		nosd = 1
		zsave[0] = llr32
	case maxOSD > 0:
		nosd = maxOSD
	}

	var (
		tov     [n][ncw]float32
		toc     [m][7]float32
		tanhtoc [m][7]float32
	)

	// decode174_91.f90 lines 43–47: initialize messages to checks.
	for j := 0; j < m; j++ {
		for i := 0; i < LDPCNrw[j]; i++ {
			toc[j][i] = llr32[LDPCNm[j][i]-1]
		}
	}

	ncnt := 0
	nclast := 0
	var zsum [n]float32

	// decode174_91.f90 lines 52–135: BP iterations.
	for iter := 0; iter <= maxIterations; iter++ {
		// Update bit LLR estimates (decode174_91.f90 lines 54–60).
		var zn [n]float32
		for i := 0; i < n; i++ {
			if apmask[i] != 1 {
				sum := llr32[i]
				for kk := 0; kk < ncw; kk++ {
					sum += tov[i][kk]
				}
				zn[i] = sum
			} else {
				zn[i] = llr32[i]
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
				tanhtoc[j][i] = float32(math.Tanh(float64(-toc[j][i]) / 2.0))
			}
		}

		for bit := 0; bit < n; bit++ {
			for kk := 0; kk < ncw; kk++ {
				chk := LDPCMn[bit][kk] - 1
				prod := float32(1.0)
				for i := 0; i < LDPCNrw[chk]; i++ {
					if LDPCNm[chk][i]-1 != bit {
						prod *= tanhtoc[chk][i]
					}
				}
				tov[bit][kk] = float32(2.0 * platanh(float64(-prod)))
			}
		}
	}

	// OSD passes (decode174_91.f90 lines 137–148).
	// Convert zsave back to float64 for the OSD path.
	for i := 0; i < nosd; i++ {
		var zIn [n]float64
		if maxOSD == 0 {
			zIn = llr
		} else {
			for j := 0; j < n; j++ {
				zIn[j] = float64(zsave[i][j])
			}
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
