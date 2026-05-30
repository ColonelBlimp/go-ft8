// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"sort"
	"strings"
)

type DecodedMessage struct {
	Text           string
	FreqHz         float64
	DTSec          float64
	Sync           float64
	HardSync       int
	CostasGeo      float64
	CostasMinBlock float64
	Blocks         int
	HardErrors     int
	DMin           float64
}

// DecodeMessages decodes one 15-second FT8 slot from 12 kHz mono signed-16-bit
// PCM samples. It is stateless; use Decoder when hash/history state should be
// retained across adjacent slots.
func DecodeMessages(iwave []int16) []DecodedMessage {
	return DecodeMessagesWithOptions(iwave, DecoderOptions{})
}

// DecodeMessagesWithOptions decodes one 15-second FT8 slot with explicit
// options. The zero-value options preserve strict-mode behavior.
func DecodeMessagesWithOptions(iwave []int16, options DecoderOptions) []DecodedMessage {
	var hashes hashTable
	return decodeMessagesCore(iwave, nil, &hashes, normalizeDecoderOptions(options))
}

func decodeMessagesCore(iwave []int16, a7Hints []a7Hint, hashes *hashTable, options decodeOptions) []DecodedMessage {
	seen := make(map[string]bool)
	var out []DecodedMessage
	var fullDD []float32
	for blockIndex := 0; blockIndex < options.blockCount; blockIndex++ {
		blocks := options.blocks[blockIndex]
		dd := decodeBlocks(iwave, blocks)
		keepDD := false
		if blocks == 50 && len(a7Hints) > 0 {
			// Keep the full-slot buffer after subtraction; A7 hints use the
			// residual so strong decoded signals do not mask weak follow-ups.
			fullDD = dd
			keepDD = true
		}
		for pass := 0; pass < 2; pass++ {
			candidates := findCandidates(dd, options.minFreqHz, options.maxFreqHz, options.syncMin, 0, options.maxCandidates)
			ds := getDownsampler()
			var subtract []DecodedMessage
			var subtractCodewords [][174]int8
			for candIndex, cand := range candidates {
				analysis, decoded, ok := decodeCandidateVariantsForMetricSet(dd, ds, cand, candIndex == 0, pass, hashes, options)
				if !ok {
					continue
				}
				msg := DecodedMessage{
					Text:           decoded.Text,
					FreqHz:         analysis.Refined.FreqHz,
					DTSec:          analysis.Refined.DTSec - 0.5,
					Sync:           analysis.Refined.Sync,
					HardSync:       analysis.Refined.HardSync,
					CostasGeo:      analysis.Refined.CostasGeo,
					CostasMinBlock: analysis.Refined.CostasMinBlock,
					Blocks:         blocks,
					HardErrors:     decoded.Result.HardErrors,
					DMin:           decoded.Result.DMin,
				}
				subtract = append(subtract, msg)
				subtractCodewords = append(subtractCodewords, decoded.Result.Codeword)
				if seen[decoded.Text] {
					continue
				}
				seen[decoded.Text] = true
				out = append(out, msg)
			}
			putDownsampler(ds)
			if len(subtract) == 0 {
				break
			}
			for i, msg := range subtract {
				subtractFT8(dd, tonesFromCodeword(subtractCodewords[i]), msg.FreqHz, msg.DTSec+0.5)
			}
		}
		if !keepDD {
			putDecodeBlocks(dd)
		}
	}
	if len(a7Hints) > 0 && fullDD != nil {
		out = append(out, decodeA7Hints(fullDD, a7Hints, seen)...)
	}
	if fullDD != nil {
		putDecodeBlocks(fullDD)
	}
	return out
}

type candidateDecode struct {
	Text   string
	Result ldpcResult
}

func decodeCandidateVariantsForMetricSet(dd []float32, ds *downsampler, cand candidate, recompute bool, metricSet int, hashes *hashTable, options decodeOptions) (candidateAnalysis, candidateDecode, bool) {
	analysis := analyzeCandidateWithDownsamplerForMetricSet(dd, ds, cand, recompute, metricSet)
	if decoded, ok := decodeCandidateWithMetricSet(&analysis, metricSet, hashes, options); ok {
		return analysis, decoded, true
	}
	return analysis, candidateDecode{}, false
}

func decodeCandidateWithMetricSet(analysis *candidateAnalysis, metricSet int, hashes *hashTable, options decodeOptions) (candidateDecode, bool) {
	if analysis.Refined.HardSync <= options.hardSyncMin {
		return candidateDecode{}, false
	}
	if !passesCostasGate(analysis.Refined, options) {
		return candidateDecode{}, false
	}
	if metricSet != 1 {
		if decoded, ok := decodeCandidateMetrics(&analysis.Metrics, hashes, options); ok {
			return decoded, true
		}
	}
	if metricSet != 0 {
		if decoded, ok := decodeCandidateMetrics(&analysis.PowerMetrics, hashes, options); ok {
			return decoded, true
		}
	}
	return candidateDecode{}, false
}

func passesCostasGate(refined refinedCandidate, options decodeOptions) bool {
	if options.costasMinWins > 0 && refined.CostasWins < options.costasMinWins {
		return false
	}
	if options.costasMinGeo > 0 && refined.CostasGeo < options.costasMinGeo {
		return false
	}
	if options.costasMinBlock > 0 && refined.CostasMinBlock < options.costasMinBlock {
		return false
	}
	return true
}

func decodeCandidateMetrics(metrics *softMetrics, hashes *hashTable, options decodeOptions) (candidateDecode, bool) {
	for pass := 0; pass < 5; pass++ {
		if decoded, ok := decodeMetricPass(metricPassSource(metrics, pass), &ft8NoAPMask, hashes, options); ok {
			return decoded, true
		}
	}
	mask := cqAPMask()
	if decoded, ok := decodeAPMetricPass(&metrics.Single, mask, hashes, options); ok {
		return decoded, true
	}
	return decodeAPMetricPass(&metrics.Triple, mask, hashes, options)
}

func metricPassSource(metrics *softMetrics, pass int) *[174]float64 {
	switch pass {
	case 0:
		return &metrics.Single
	case 1:
		return &metrics.Double
	case 2:
		return &metrics.Triple
	case 3:
		return &metrics.Normed
	default:
		return &metrics.Best
	}
}

func decodeMetricPass(metric *[174]float64, apmask *[174]int8, hashes *hashTable, options decodeOptions) (candidateDecode, bool) {
	var llr [174]float64
	for i, v := range metric {
		llr[i] = ft8ScaleFac * v
	}
	shapeLLR(&llr, options)
	return decodeLLRPass(&llr, apmask, hashes, options)
}

func decodeAPMetricPass(metric *[174]float64, apmask *[174]int8, hashes *hashTable, options decodeOptions) (candidateDecode, bool) {
	llr := apCQPass(*metric)
	shapeLLR(&llr, options)
	return decodeLLRPass(&llr, apmask, hashes, options)
}

func shapeLLR(llr *[174]float64, options decodeOptions) {
	if options.llrWinsorFactor > 0 {
		winsorizeLLR(llr, options.llrWinsorFactor)
	}
}

func winsorizeLLR(llr *[174]float64, factor float64) {
	if factor <= 0 {
		return
	}
	var absVals [174]float64
	n := 0
	for _, v := range llr {
		a := math.Abs(v)
		if a > 0 && !math.IsNaN(a) && !math.IsInf(a, 0) {
			absVals[n] = a
			n++
		}
	}
	if n == 0 {
		return
	}
	sort.Float64s(absVals[:n])
	median := absVals[n/2]
	if median <= 0 || math.IsNaN(median) || math.IsInf(median, 0) {
		return
	}
	capAbs := factor * median
	if capAbs <= 0 || math.IsNaN(capAbs) || math.IsInf(capAbs, 0) {
		return
	}
	for i, v := range llr {
		switch {
		case v > capAbs:
			llr[i] = capAbs
		case v < -capAbs:
			llr[i] = -capAbs
		}
	}
}

func decodeLLRPass(llr *[174]float64, apmask *[174]int8, hashes *hashTable, options decodeOptions) (candidateDecode, bool) {
	var result ldpcResult
	var ok bool
	if options.enableOSD {
		result, ok = decode17491HybridWithAP(llr, apmask)
	} else {
		result, ok, _ = decode17491BP(llr, apmask, 0)
	}
	if !ok {
		return candidateDecode{}, false
	}
	if result.HardErrors < 0 || result.HardErrors > 36 {
		return candidateDecode{}, false
	}
	if allZeroCodeword(result.Codeword) {
		return candidateDecode{}, false
	}
	msg, ok := unpack77FromCodewordWithHashes(result.Codeword, hashes)
	if !ok {
		return candidateDecode{}, false
	}
	if strings.Contains(msg, "/R") || strings.HasPrefix(msg, "TU; ") {
		return candidateDecode{}, false
	}
	return candidateDecode{Text: msg, Result: result}, true
}

func allZeroCodeword(cw [174]int8) bool {
	for _, bit := range cw {
		if bit != 0 {
			return false
		}
	}
	return true
}
