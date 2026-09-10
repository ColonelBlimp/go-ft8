// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ftx77

import "math"

// DecodeResult is a successfully recovered LDPC(174,91) codeword.
type DecodeResult struct {
	Message91  [91]int8
	Codeword   [174]int8
	HardErrors int
	DMin       float64
}

// Decode17491BP decodes soft bit metrics for the FT4/FT8 LDPC(174,91) code.
// Positive values favor one and negative values favor zero.
func Decode17491BP(llr *[174]float64) (DecodeResult, bool) {
	var tov [174][3]float32
	var toc [83][7]float32
	var tanhTOC [83][7]float32
	var zn [174]float32

	for check, row := range ldpcChecks {
		for edge, bit := range row {
			if bit < 0 {
				break
			}
			toc[check][edge] = float32(llr[bit])
		}
	}

	staleCount := 0
	lastUnsatisfied := 0
	for iter := 0; iter <= 40; iter++ {
		for bit := range zn {
			zn[bit] = float32(llr[bit]) + tov[bit][0] + tov[bit][1] + tov[bit][2]
		}

		cw, unsatisfied := hardDecisionAndParity(&zn)
		if unsatisfied == 0 && crc14OK(&cw) {
			result := DecodeResult{
				Codeword:   cw,
				HardErrors: hardErrors(cw, llr),
				DMin:       softDistance(cw, llr),
			}
			copy(result.Message91[:], cw[:91])
			return result, true
		}

		if iter > 0 {
			if unsatisfied < lastUnsatisfied {
				staleCount = 0
			} else {
				staleCount++
			}
			if staleCount >= 5 && iter >= 10 && unsatisfied > 15 {
				break
			}
		}
		lastUnsatisfied = unsatisfied

		for check, row := range ldpcChecks {
			for edge, bit := range row {
				if bit < 0 {
					break
				}
				toc[check][edge] = zn[bit] - tov[bit][ldpcCheckBitEdge[check][edge]]
			}
		}

		for check, row := range ldpcChecks {
			for edge, bit := range row {
				if bit < 0 {
					break
				}
				tanhTOC[check][edge] = float32(fastTanh(float64(-toc[check][edge]) / 2))
			}
		}

		for check, row := range ldpcChecks {
			nrw := checkDegree(row)
			var prefix [7]float32
			product := float32(1)
			for edge := 0; edge < nrw; edge++ {
				prefix[edge] = product
				product *= tanhTOC[check][edge]
			}
			for edge := 0; edge < nrw; edge++ {
				product = prefix[edge]
				for other := edge + 1; other < nrw; other++ {
					product *= tanhTOC[check][other]
				}
				bit := row[edge]
				tov[bit][ldpcCheckBitEdge[check][edge]] = float32(2 * platanh(float64(-product)))
			}
		}
	}

	return DecodeResult{}, false
}

func checkDegree(row [7]int) int {
	for i, bit := range row {
		if bit < 0 {
			return i
		}
	}
	return len(row)
}

var ldpcBitChecks, ldpcCheckBitEdge = initLDPCGraph()

func initLDPCGraph() ([174][3]int, [83][7]int) {
	var bitChecks [174][3]int
	var checkBitEdge [83][7]int
	for bit := range bitChecks {
		for edge := range bitChecks[bit] {
			bitChecks[bit][edge] = -1
		}
	}
	for check, row := range ldpcChecks {
		for edge, bit := range row {
			if bit < 0 {
				break
			}
			bitEdge := 0
			for bitEdge < len(bitChecks[bit]) && bitChecks[bit][bitEdge] >= 0 {
				bitEdge++
			}
			if bitEdge == len(bitChecks[bit]) {
				panic("ftx77: invalid LDPC graph")
			}
			bitChecks[bit][bitEdge] = check
			checkBitEdge[check][edge] = bitEdge
		}
	}
	return bitChecks, checkBitEdge
}

func hardDecisionAndParity(zn *[174]float32) ([174]int8, int) {
	var cw [174]int8
	var syndrome [83]uint8
	for bit, value := range zn {
		if value <= 0 {
			continue
		}
		cw[bit] = 1
		for _, check := range ldpcBitChecks[bit] {
			if check >= 0 {
				syndrome[check] ^= 1
			}
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
		state = state<<1 | uint16(cw[i]&1)
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

func hardErrors(cw [174]int8, llr *[174]float64) int {
	errors := 0
	for i, bit := range cw {
		sign := -1.0
		if bit == 1 {
			sign = 1
		}
		if sign*llr[i] < 0 {
			errors++
		}
	}
	return errors
}

func softDistance(cw [174]int8, llr *[174]float64) float64 {
	var distance float64
	for i, bit := range cw {
		hard := int8(0)
		if llr[i] > 0 {
			hard = 1
		}
		if hard != bit {
			distance += math.Abs(llr[i])
		}
	}
	return distance
}

func fastTanh(x float64) float64 {
	if x > 4.97 {
		return 1
	}
	if x < -4.97 {
		return -1
	}
	x2 := x * x
	p := x * (135135 + x2*(17325+x2*(378+x2)))
	q := 135135 + x2*(62370+x2*(3150+x2*28))
	return p / q
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
		return sign * 7
	}
}

var ldpcChecks = [83][7]int{
	{3, 30, 58, 90, 91, 95, 152},
	{4, 31, 59, 92, 114, 145, -1},
	{5, 23, 60, 93, 121, 150, -1},
	{6, 32, 61, 94, 95, 142, -1},
	{7, 24, 62, 82, 92, 95, 147},
	{5, 31, 63, 96, 125, 137, -1},
	{4, 33, 64, 77, 97, 106, 153},
	{8, 34, 65, 98, 138, 145, -1},
	{9, 35, 66, 99, 106, 125, -1},
	{10, 36, 66, 86, 100, 138, 157},
	{11, 37, 67, 101, 104, 154, -1},
	{12, 38, 68, 102, 148, 161, -1},
	{7, 39, 69, 81, 103, 113, 144},
	{13, 40, 70, 87, 101, 122, 155},
	{14, 41, 58, 105, 122, 158, -1},
	{0, 32, 71, 105, 106, 156, -1},
	{15, 42, 72, 107, 140, 159, -1},
	{16, 36, 73, 80, 108, 130, 153},
	{10, 43, 74, 109, 120, 165, -1},
	{44, 54, 63, 110, 129, 160, 172},
	{7, 45, 70, 111, 118, 165, -1},
	{17, 35, 75, 88, 112, 113, 142},
	{18, 37, 76, 103, 115, 162, -1},
	{19, 46, 69, 91, 137, 164, -1},
	{1, 47, 73, 112, 127, 159, -1},
	{20, 44, 77, 82, 116, 120, 150},
	{21, 46, 57, 117, 126, 163, -1},
	{15, 38, 61, 111, 133, 157, -1},
	{22, 42, 78, 119, 130, 144, -1},
	{18, 34, 58, 72, 109, 124, 160},
	{19, 35, 62, 93, 135, 160, -1},
	{13, 30, 78, 97, 131, 163, -1},
	{2, 43, 79, 123, 126, 168, -1},
	{18, 45, 80, 116, 134, 166, -1},
	{6, 48, 57, 89, 99, 104, 167},
	{11, 49, 60, 117, 118, 143, -1},
	{12, 50, 63, 113, 117, 156, -1},
	{23, 51, 75, 128, 147, 148, -1},
	{24, 52, 68, 89, 100, 129, 155},
	{19, 45, 64, 79, 119, 139, 169},
	{20, 53, 76, 99, 139, 170, -1},
	{34, 81, 132, 141, 170, 173, -1},
	{13, 29, 82, 112, 124, 169, -1},
	{3, 28, 67, 119, 133, 172, -1},
	{0, 3, 51, 56, 85, 135, 151},
	{25, 50, 55, 90, 121, 136, 167},
	{51, 83, 109, 114, 144, 167, -1},
	{6, 49, 80, 98, 131, 172, -1},
	{22, 54, 66, 94, 171, 173, -1},
	{25, 40, 76, 108, 140, 147, -1},
	{1, 26, 40, 60, 61, 114, 132},
	{26, 39, 55, 123, 124, 125, -1},
	{17, 48, 54, 123, 140, 166, -1},
	{5, 32, 84, 107, 115, 155, -1},
	{27, 47, 69, 84, 104, 128, 157},
	{8, 53, 62, 130, 146, 154, -1},
	{21, 52, 67, 108, 120, 173, -1},
	{2, 12, 47, 77, 94, 122, -1},
	{30, 68, 132, 149, 154, 168, -1},
	{11, 42, 65, 88, 96, 134, 158},
	{4, 38, 74, 101, 135, 166, -1},
	{1, 53, 85, 100, 134, 163, -1},
	{14, 55, 86, 107, 118, 170, -1},
	{9, 43, 81, 90, 110, 143, 148},
	{22, 33, 70, 93, 126, 152, -1},
	{10, 48, 87, 91, 141, 156, -1},
	{28, 33, 86, 96, 146, 161, -1},
	{29, 49, 59, 85, 136, 141, 161},
	{9, 52, 65, 83, 111, 127, 164},
	{21, 56, 84, 92, 139, 158, -1},
	{27, 31, 71, 102, 131, 165, -1},
	{27, 28, 83, 87, 116, 142, 149},
	{0, 25, 44, 79, 127, 146, -1},
	{16, 26, 88, 102, 115, 152, -1},
	{50, 56, 97, 162, 164, 171, -1},
	{20, 36, 72, 137, 151, 168, -1},
	{15, 46, 75, 129, 136, 153, -1},
	{2, 23, 29, 71, 103, 138, -1},
	{8, 39, 89, 105, 133, 150, -1},
	{14, 57, 59, 73, 110, 149, 162},
	{17, 41, 78, 143, 145, 151, -1},
	{24, 37, 64, 98, 121, 159, -1},
	{16, 41, 74, 128, 169, 171, -1},
}
