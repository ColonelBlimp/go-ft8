// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"math/bits"
	"sync"

	"github.com/ColonelBlimp/go-ft8/internal/ftx77"
)

var (
	osdGenOnce sync.Once
	osdGenRows [91][174]int8
)

const (
	osdRowWords = (174 + 63) / 64
	// osdTailWords covers the 83 parity-tail bits handled as bitsets in the
	// OSD flip search.
	osdTailWords = (174 - 91 + 63) / 64
)

type osdRowBits [osdRowWords]uint64

func osdSetBit(row *osdRowBits, bit int) {
	row[bit/64] |= 1 << uint(bit%64)
}

func osdBitSet(row *osdRowBits, bit int) bool {
	return row[bit/64]&(1<<uint(bit%64)) != 0
}

func osdBitValue(row *osdRowBits, bit int) int8 {
	if osdBitSet(row, bit) {
		return 1
	}
	return 0
}

func osdSwapBits(row *osdRowBits, a int, b int) {
	abit := osdBitSet(row, a)
	bbit := osdBitSet(row, b)
	if abit == bbit {
		return
	}
	row[a/64] ^= 1 << uint(a%64)
	row[b/64] ^= 1 << uint(b%64)
}

func sortOSDOrder(order *[174]int, absrx *[174]float64) {
	for _, gap := range [...]int{132, 57, 23, 10, 4, 1} {
		for i := gap; i < len(order); i++ {
			value := order[i]
			j := i
			for j >= gap && osdOrderLess(absrx, value, order[j-gap]) {
				order[j] = order[j-gap]
				j -= gap
			}
			order[j] = value
		}
	}
}

func osdOrderLess(absrx *[174]float64, a int, b int) bool {
	av := absrx[a]
	bv := absrx[b]
	if av == bv {
		return a < b
	}
	return av < bv
}

func decode17491HybridWithAP(llr *[174]float64, apmask *[174]int8) (ldpcResult, bool) {
	var saved [ldpcMaxSavedIterations][174]float64
	result, ok, savedCount := decode17491BP(llr, apmask, &saved)
	if ok {
		return result, true
	}
	for i := 0; i < savedCount; i++ {
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
	sortOSDOrder(&order, &absrx)

	var indices [n]int
	var genmrb [k]osdRowBits
	for col := 0; col < n; col++ {
		orig := order[n-1-col]
		indices[col] = orig
		for row := 0; row < k; row++ {
			if osdGenRows[row][orig] != 0 {
				osdSetBit(&genmrb[row], col)
			}
		}
	}

	// Match WSJT-X OSD's bounded pivot search window. For the (174,91)
	// code k+20 < n always holds; the guard breaks the build if the
	// dimensions ever change to violate that.
	const pivotLimit = k + 20
	const _ = uint(n - pivotLimit)

	for diag := 0; diag < k; diag++ {
		pivot := -1
		// If the MRB basis cannot be diagonalized within this window,
		// abandon this OSD attempt.
		for col := diag; col < pivotLimit; col++ {
			if osdBitSet(&genmrb[diag], col) {
				pivot = col
				break
			}
		}
		if pivot < 0 {
			return ldpcResult{}, false
		}
		if pivot != diag {
			for row := 0; row < k; row++ {
				osdSwapBits(&genmrb[row], diag, pivot)
			}
			indices[diag], indices[pivot] = indices[pivot], indices[diag]
		}
		for row := 0; row < k; row++ {
			if row == diag || !osdBitSet(&genmrb[row], diag) {
				continue
			}
			for word := range genmrb[row] {
				genmrb[row][word] ^= genmrb[diag][word]
			}
		}
	}

	var g2 [n][k]int8
	var g2Tail [k][osdTailWords]uint64
	for row := 0; row < k; row++ {
		for col := 0; col < n; col++ {
			value := osdBitValue(&genmrb[row], col)
			g2[col][row] = value
			if col >= k && value != 0 {
				g2Tail[row][(col-k)/64] |= 1 << uint((col-k)%64)
			}
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

	var rhardTail [osdTailWords]uint64
	for i := 0; i < n-k; i++ {
		if rhard[k+i] == 1 {
			rhardTail[i/64] |= 1 << uint(i%64)
		}
	}

	var m0 [k]int8
	copy(m0[:], rhard[:k])
	baseCode := mrbEncode(m0, &g2)
	best := baseCode
	bestXor := xorWeight(best[:], rhard[:])
	bestDistance := weightedDistance(best[:], rhard[:], rabs[:])

	// nd1kpt counts tail errors among the first nt parity positions, which
	// must fit inside the first tail word for the mask below.
	const ntMask = 1<<nt - 1
	_ = uint(63 - nt)

	for base := k - 1; base >= 0; base-- {
		var e2sub [osdTailWords]uint64
		var d1 float64
		var cached bool

		for flip := base; flip >= 0; flip-- {
			if rapmask[base] == 1 || flip != base && rapmask[flip] == 1 {
				continue
			}

			var ce [n]int8
			var ceValid bool
			var e2 [osdTailWords]uint64
			nd1kpt := 0
			if flip == base || !cached {
				ce = mrbEncodeFlipped(baseCode, &g2, base, flip)
				ceValid = true
				var ceTail [osdTailWords]uint64
				for i := 0; i < n-k; i++ {
					if ce[k+i] == 1 {
						ceTail[i/64] |= 1 << uint(i%64)
					}
				}
				for word := range e2sub {
					e2sub[word] = ceTail[word] ^ rhardTail[word]
				}
				e2 = e2sub
				nd1kpt = 1 + bits.OnesCount64(e2sub[0]&ntMask)
				d1 = rabs[base]
				cached = true
			} else {
				for word := range e2 {
					e2[word] = e2sub[word] ^ g2Tail[flip][word]
				}
				nd1kpt = 2 + bits.OnesCount64(e2[0]&ntMask)
			}

			if nd1kpt > ntheta {
				continue
			}
			distance := d1
			if flip != base {
				ceFlip := baseCode[flip] ^ g2[flip][base] ^ g2[flip][flip]
				if ceFlip != rhard[flip] {
					distance += rabs[flip]
				}
			}
			for word, tailBits := range e2 {
				for tailBits != 0 {
					i := word*64 + bits.TrailingZeros64(tailBits)
					distance += rabs[k+i]
					tailBits &= tailBits - 1
				}
			}
			if distance < bestDistance {
				if !ceValid {
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
	return ftx77.Encode17491NoCRC(message)
}
