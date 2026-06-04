// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"sort"
	"sync"
)

var (
	osdGenOnce sync.Once
	osdGenRows [91][174]int8
)

func decode17491HybridWithAP(llr *[174]float64, apmask *[174]int8) (ldpcResult, bool) {
	result, ok, saved := decode17491BP(llr, apmask, 2)
	if ok {
		return result, true
	}
	for i := range saved {
		if osd, ok := osd17491(&saved[i], llr, apmask); ok {
			return osd, true
		}
	}
	return ldpcResult{}, false
}

func osd17491(rx *[174]float64, channel *[174]float64, apmask *[174]int8) (ldpcResult, bool) {
	osdGenOnce.Do(initOSDGenerator)

	const (
		n      = 174
		k      = 91
		nt     = 40
		ntheta = 10
	)

	var hard [n]int8
	var absrx [n]float64
	var order [n]int
	for i, v := range rx {
		if v > 0 {
			hard[i] = 1
		}
		absrx[i] = math.Abs(v)
		order[i] = i
	}
	sort.Slice(order[:], func(i, j int) bool {
		if absrx[order[i]] == absrx[order[j]] {
			return order[i] < order[j]
		}
		return absrx[order[i]] < absrx[order[j]]
	})

	var indices [n]int
	var genmrb [k][n]int8
	for col := 0; col < n; col++ {
		orig := order[n-1-col]
		indices[col] = orig
		for row := 0; row < k; row++ {
			genmrb[row][col] = osdGenRows[row][orig]
		}
	}

	for diag := 0; diag < k; diag++ {
		pivot := -1
		limit := k + 20
		if limit > n {
			limit = n
		}
		// Match WSJT-X OSD's bounded pivot search. If the MRB basis cannot be
		// diagonalized within this window, abandon this OSD attempt.
		for col := diag; col < limit; col++ {
			if genmrb[diag][col] == 1 {
				pivot = col
				break
			}
		}
		if pivot < 0 {
			return ldpcResult{}, false
		}
		if pivot != diag {
			for row := 0; row < k; row++ {
				genmrb[row][diag], genmrb[row][pivot] = genmrb[row][pivot], genmrb[row][diag]
			}
			indices[diag], indices[pivot] = indices[pivot], indices[diag]
		}
		for row := 0; row < k; row++ {
			if row == diag || genmrb[row][diag] == 0 {
				continue
			}
			for col := 0; col < n; col++ {
				genmrb[row][col] ^= genmrb[diag][col]
			}
		}
	}

	var g2 [n][k]int8
	for row := 0; row < k; row++ {
		for col := 0; col < n; col++ {
			g2[col][row] = genmrb[row][col]
		}
	}

	var rhard [n]int8
	var rabs [n]float64
	var rapmask [n]int8
	for i, orig := range indices {
		rhard[i] = hard[orig]
		if apmask[orig] == 1 {
			rhard[i] = 0
			if channel[orig] > 0 {
				rhard[i] = 1
			}
		}
		rabs[i] = absrx[orig]
		rapmask[i] = apmask[orig]
	}

	var m0 [k]int8
	copy(m0[:], rhard[:k])
	baseCode := mrbEncode(m0, &g2)
	best := baseCode
	bestXor := xorWeight(best[:], rhard[:])
	bestDistance := weightedDistance(best[:], rhard[:], rabs[:])

	for base := k - 1; base >= 0; base-- {
		var e2sub [n - k]int8
		var d1 float64
		var cached bool

		for flip := base; flip >= 0; flip-- {
			if rapmask[base] == 1 || flip != base && rapmask[flip] == 1 {
				continue
			}

			var ce [n]int8
			var e2 [n - k]int8
			nd1kpt := 0
			if flip == base || !cached {
				ce = mrbEncodeFlipped(baseCode, &g2, base, flip)
				nd1kpt = 1
				for i := 0; i < nt; i++ {
					bit := ce[k+i] ^ rhard[k+i]
					e2sub[i] = bit
					e2[i] = bit
					nd1kpt += int(bit)
				}
				for i := nt; i < n-k; i++ {
					bit := ce[k+i] ^ rhard[k+i]
					e2sub[i] = bit
					e2[i] = bit
				}
				d1 = rabs[base]
				cached = true
			} else {
				nd1kpt = 2
				for i := 0; i < nt; i++ {
					bit := e2sub[i] ^ g2[k+i][flip]
					e2[i] = bit
					nd1kpt += int(bit)
				}
				for i := nt; i < n-k; i++ {
					e2[i] = e2sub[i] ^ g2[k+i][flip]
				}
			}

			if nd1kpt > ntheta {
				continue
			}
			var distance float64
			if flip == base {
				distance = d1
				for i := 0; i < n-k; i++ {
					if e2sub[i] != 0 {
						distance += rabs[k+i]
					}
				}
			} else {
				distance = d1
				ceFlip := baseCode[flip] ^ g2[flip][base] ^ g2[flip][flip]
				if ceFlip != rhard[flip] {
					distance += rabs[flip]
				}
				for i := 0; i < n-k; i++ {
					if e2[i] != 0 {
						distance += rabs[k+i]
					}
				}
			}
			if distance < bestDistance {
				if flip != base {
					ce = mrbEncodeFlipped(baseCode, &g2, base, flip)
				}
				bestDistance = distance
				best = ce
				bestXor = xorWeight(best[:], rhard[:])
			}
		}
	}

	var cw [n]int8
	for i, orig := range indices {
		cw[orig] = best[i]
	}
	if !crc14OK(&cw) {
		return ldpcResult{}, false
	}

	var result ldpcResult
	result.Codeword = cw
	copy(result.Message91[:], cw[:91])
	result.HardErrors = bestXor
	result.DMin = softDistance(cw, channel)
	result.Decoder = 2
	return result, true
}

func mrbEncodeFlipped(base [174]int8, g2 *[174][91]int8, bit1, bit2 int) [174]int8 {
	codeword := base
	for i := 0; i < 174; i++ {
		codeword[i] ^= g2[i][bit1]
	}
	if bit2 != bit1 {
		for i := 0; i < 174; i++ {
			codeword[i] ^= g2[i][bit2]
		}
	}
	return codeword
}

func mrbEncode(message [91]int8, g2 *[174][91]int8) [174]int8 {
	var codeword [174]int8
	for bit, value := range message {
		if value == 0 {
			continue
		}
		for i := 0; i < 174; i++ {
			codeword[i] ^= g2[i][bit]
		}
	}
	return codeword
}

func xorWeight(a []int8, b []int8) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	weight := 0
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			weight++
		}
	}
	return weight
}

func weightedDistance(a []int8, b []int8, weights []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if len(weights) < n {
		n = len(weights)
	}
	var d float64
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			d += weights[i]
		}
	}
	return d
}

func initOSDGenerator() {
	for bit := 0; bit < 91; bit++ {
		var message [91]int8
		message[bit] = 1
		osdGenRows[bit] = encode17491NoCRC(message)
	}
}

func encode17491NoCRC(message [91]int8) [174]int8 {
	var codeword [174]int8
	copy(codeword[:91], message[:])
	gen := ldpcGeneratorMatrix()
	for row := 0; row < 83; row++ {
		sum := int8(0)
		for col := 0; col < 91; col++ {
			sum ^= message[col] & gen[row][col]
		}
		codeword[91+row] = sum
	}
	return codeword
}

func ldpcGeneratorMatrix() [83][91]int8 {
	var gen [83][91]int8
	for row, hexRow := range ldpcGeneratorHex {
		for h := 0; h < len(hexRow); h++ {
			v := hexValue(hexRow[h])
			bits := 4
			if h == len(hexRow)-1 {
				bits = 3
			}
			for b := 0; b < bits; b++ {
				col := h*4 + b
				if col >= 91 {
					break
				}
				if v&(1<<uint(3-b)) != 0 {
					gen[row][col] = 1
				}
			}
		}
	}
	return gen
}

func hexValue(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	default:
		return 0
	}
}

var ldpcGeneratorHex = [...]string{
	"8329ce11bf31eaf509f27fc",
	"761c264e25c259335493132",
	"dc265902fb277c6410a1bdc",
	"1b3f417858cd2dd33ec7f62",
	"09fda4fee04195fd034783a",
	"077cccc11b8873ed5c3d48a",
	"29b62afe3ca036f4fe1a9da",
	"6054faf5f35d96d3b0c8c3e",
	"e20798e4310eed27884ae90",
	"775c9c08e80e26ddae56318",
	"b0b811028c2bf997213487c",
	"18a0c9231fc60adf5c5ea32",
	"76471e8302a0721e01b12b8",
	"ffbccb80ca8341fafb47b2e",
	"66a72a158f9325a2bf67170",
	"c4243689fe85b1c51363a18",
	"0dff739414d1a1b34b1c270",
	"15b48830636c8b99894972e",
	"29a89c0d3de81d665489b0e",
	"4f126f37fa51cbe61bd6b94",
	"99c47239d0d97d3c84e0940",
	"1919b75119765621bb4f1e8",
	"09db12d731faee0b86df6b8",
	"488fc33df43fbdeea4eafb4",
	"827423ee40b675f756eb5fe",
	"abe197c484cb74757144a9a",
	"2b500e4bc0ec5a6d2bdbdd0",
	"c474aa53d70218761669360",
	"8eba1a13db3390bd6718cec",
	"753844673a27782cc42012e",
	"06ff83a145c37035a5c1268",
	"3b37417858cc2dd33ec3f62",
	"9a4a5a28ee17ca9c324842c",
	"bc29f465309c977e89610a4",
	"2663ae6ddf8b5ce2bb29488",
	"46f231efe457034c1814418",
	"3fb2ce85abe9b0c72e06fbe",
	"de87481f282c153971a0a2e",
	"fcd7ccf23c69fa99bba1412",
	"f0261447e9490ca8e474cec",
	"4410115818196f95cdd7012",
	"088fc31df4bfbde2a4eafb4",
	"b8fef1b6307729fb0a078c0",
	"5afea7acccb77bbc9d99a90",
	"49a7016ac653f65ecdc9076",
	"1944d085be4e7da8d6cc7d0",
	"251f62adc4032f0ee714002",
	"56471f8702a0721e00b12b8",
	"2b8e4923f2dd51e2d537fa0",
	"6b550a40a66f4755de95c26",
	"a18ad28d4e27fe92a4f6c84",
	"10c2e586388cb82a3d80758",
	"ef34a41817ee02133db2eb0",
	"7e9c0c54325a9c15836e000",
	"3693e572d1fde4cdf079e86",
	"bfb2cec5abe1b0c72e07fbe",
	"7ee18230c583cccc57d4b08",
	"a066cb2fedafc9f52664126",
	"bb23725abc47cc5f4cc4cd2",
	"ded9dba3bee40c59b5609b4",
	"d9a7016ac653e6decdc9036",
	"9ad46aed5f707f280ab5fc4",
	"e5921c77822587316d7d3c2",
	"4f14da8242a8b86dca73352",
	"8b8b507ad467d4441df770e",
	"22831c9cf1169467ad04b68",
	"213b838fe2ae54c38ee7180",
	"5d926b6dd71f085181a4e12",
	"66ab79d4b29ee6e69509e56",
	"958148682d748a38dd68baa",
	"b8ce020cf069c32a723ab14",
	"f4331d6d461607e95752746",
	"6da23ba424b9596133cf9c8",
	"a636bcbc7b30c5fbeae67fe",
	"5cb0d86a07df654a9089a20",
	"f11f106848780fc9ecdd80a",
	"1fbb5364fb8d2c9d730d5ba",
	"fcb86bc70a50c9d02a5d034",
	"a534433029eac15f322e34c",
	"c989d9c7c3d3b8c55d75130",
	"7bb38b2f0186d46643ae962",
	"2644ebadeb44b9467d1f42c",
	"608cc857594bfbb55d69600",
}
