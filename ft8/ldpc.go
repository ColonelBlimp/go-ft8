// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "math"

const ldpcMaxSavedIterations = 2

type ldpcResult struct {
	Message91  [91]int8
	Codeword   [174]int8
	HardErrors int
	DMin       float64
	Decoder    int
}

var ldpcNmBitEdge = initLDPCNmBitEdge()

func initLDPCNmBitEdge() [83][7]int {
	var out [83][7]int
	for check := range out {
		for edge := range out[check] {
			out[check][edge] = -1
		}
	}
	for bit := 0; bit < 174; bit++ {
		for bitEdge := 0; bitEdge < 3; bitEdge++ {
			check := ldpcMn[bit][bitEdge]
			for edge := 0; edge < ldpcNrw[check]; edge++ {
				if ldpcNm[check][edge] == bit {
					out[check][edge] = bitEdge
					break
				}
			}
		}
	}
	return out
}

func decode17491BP(llr *[174]float64, apmask *[174]int8, saved *[ldpcMaxSavedIterations][174]float64) (ldpcResult, bool, int) {
	var result ldpcResult
	var llr32 [174]float32
	var tov [174][3]float32
	var toc [83][7]float32
	var tanhTOC [83][7]float32
	var zn [174]float32
	var zsum [174]float32
	savedCount := 0
	for bit := 0; bit < 174; bit++ {
		llr32[bit] = float32(llr[bit])
	}

	for check := 0; check < 83; check++ {
		for edge := 0; edge < ldpcNrw[check]; edge++ {
			bit := ldpcNm[check][edge]
			toc[check][edge] = llr32[bit]
		}
	}

	staleCount := 0
	lastUnsatisfied := 0
	for iter := 0; iter <= 30; iter++ {
		for bit := 0; bit < 174; bit++ {
			if apmask[bit] == 1 {
				zn[bit] = llr32[bit]
			} else {
				zn[bit] = llr32[bit] + tov[bit][0] + tov[bit][1] + tov[bit][2]
			}
			zsum[bit] += zn[bit]
		}
		if saved != nil && iter > 0 && savedCount < len(saved) {
			for bit, v := range zsum {
				saved[savedCount][bit] = float64(v)
			}
			savedCount++
		}

		cw, unsatisfied := hardDecisionAndParity(&zn)
		if unsatisfied == 0 && crc14OK(&cw) {
			result.Codeword = cw
			copy(result.Message91[:], cw[:91])
			result.HardErrors = hardErrors(cw, llr)
			result.DMin = softDistance(cw, llr)
			result.Decoder = 1
			return result, true, savedCount
		}

		if iter > 0 {
			if unsatisfied-lastUnsatisfied < 0 {
				staleCount = 0
			} else {
				staleCount++
			}
			if staleCount >= 5 && iter >= 10 && unsatisfied > 15 {
				break
			}
		}
		lastUnsatisfied = unsatisfied

		for check := 0; check < 83; check++ {
			for edge := 0; edge < ldpcNrw[check]; edge++ {
				bit := ldpcNm[check][edge]
				toc[check][edge] = zn[bit] - tov[bit][ldpcNmBitEdge[check][edge]]
			}
		}

		for check := 0; check < 83; check++ {
			for edge := 0; edge < ldpcNrw[check]; edge++ {
				tanhTOC[check][edge] = float32(math.Tanh(float64(-toc[check][edge] / 2)))
			}
		}

		for check := 0; check < 83; check++ {
			nrw := ldpcNrw[check]
			var prefix [7]float32
			product := float32(1.0)
			for edge := 0; edge < nrw; edge++ {
				prefix[edge] = product
				product *= tanhTOC[check][edge]
			}
			for edge := 0; edge < nrw; edge++ {
				product = prefix[edge]
				for checkEdge := edge + 1; checkEdge < nrw; checkEdge++ {
					product *= tanhTOC[check][checkEdge]
				}
				bit := ldpcNm[check][edge]
				bitEdge := ldpcNmBitEdge[check][edge]
				tov[bit][bitEdge] = float32(2 * platanh(float64(-product)))
			}
		}
	}

	return ldpcResult{}, false, savedCount
}

func hardDecisionAndParity(zn *[174]float32) ([174]int8, int) {
	var cw [174]int8
	var syndrome [83]uint8
	for bit, v := range zn {
		if v <= 0 {
			continue
		}
		cw[bit] = 1
		for edge := 0; edge < 3; edge++ {
			syndrome[ldpcMn[bit][edge]] ^= 1
		}
	}
	unsatisfied := 0
	for _, bit := range syndrome {
		unsatisfied += int(bit)
	}
	return cw, unsatisfied
}

func crc14OK(cw *[174]int8) bool {
	var state uint16
	for i := 0; i < 15; i++ {
		state = (state << 1) | uint16(cw[i]&1)
	}
	for i := 0; i <= 81; i++ {
		pos := i + 14
		var bit int8
		switch {
		case pos < 77:
			bit = cw[pos]
		case pos >= 82:
			bit = cw[pos-5]
		}
		state = crc14Step(state, bit)
	}
	return state>>1 == 0
}

const crc14PolyState uint16 = 0x6757

func crc14Remainder(bits []int8) int {
	var state uint16
	for i := 0; i < 15 && i < len(bits); i++ {
		state = (state << 1) | uint16(bits[i]&1)
	}
	for i := 0; i <= len(bits)-15; i++ {
		state = crc14Step(state, bits[i+14])
	}
	return int(state >> 1)
}

func crc14Step(state uint16, bit int8) uint16 {
	state = (state &^ 1) | uint16(bit&1)
	if state&0x4000 != 0 {
		state ^= crc14PolyState
	}
	return (state << 1) & 0x7fff
}

func hardErrors(cw [174]int8, llr *[174]float64) int {
	errs := 0
	for i, bit := range cw {
		sign := -1.0
		if bit == 1 {
			sign = 1.0
		}
		if sign*llr[i] < 0 {
			errs++
		}
	}
	return errs
}

func softDistance(cw [174]int8, llr *[174]float64) float64 {
	var d float64
	for i, bit := range cw {
		hard := int8(0)
		if llr[i] > 0 {
			hard = 1
		}
		if hard != bit {
			d += math.Abs(llr[i])
		}
	}
	return d
}

func platanh(x float64) float64 {
	sign := 1.0
	z := x
	if x < 0 {
		sign = -1
		z = -x
	}
	switch {
	case z <= 0.664:
		return x / 0.83
	case z <= 0.9217:
		return sign * (z - 0.4064) / 0.322
	case z <= 0.9951:
		return sign * (z - 0.8378) / 0.0524
	case z <= 0.9998:
		return sign * (z - 0.9914) / 0.0012
	default:
		return sign * 7.0
	}
}
