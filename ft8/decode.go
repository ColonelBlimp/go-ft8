// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"sort"
	"strings"
)

// DecodedMessage describes one recovered FT8 message and the signal/decode
// metrics associated with the candidate that produced it.
type DecodedMessage struct {
	Text   string
	FreqHz float64
	DTSec  float64
	// Sync is the normalized coarse sync-search score used by SyncMin.
	// Hint-only A7 decodes do not have a coarse search score and report zero.
	Sync           float64
	HardSync       int
	CostasGeo      float64
	CostasMinBlock float64
	Blocks         int
	HardErrors     int
	DMin           float64
}

// DecodeMessages decodes one 15-second FT8 slot from 12 kHz mono signed-16-bit
// PCM samples.
//
// This is the permissive stateless API: short input is zero-padded, excess
// input beyond the decoder buffer is ignored, and an empty result is a normal
// no-decode outcome. Use DecodeMessagesWithReport for diagnostics or
// DecodeMessagesChecked when caller input and options should be validated.
func DecodeMessages(iwave []int16) []DecodedMessage {
	return DecodeMessagesWithOptions(iwave, DecoderOptions{})
}

// DecodeMessagesWithOptions decodes one 15-second FT8 slot with explicit
// options.
//
// The zero-value options preserve strict-mode behavior. This API is permissive
// and normalizes unsupported option values where possible; use
// DecodeMessagesChecked to reject invalid input or options before decode work
// starts.
func DecodeMessagesWithOptions(iwave []int16, options DecoderOptions) []DecodedMessage {
	var hashes hashTable
	return decodeMessagesCore(iwave, nil, &hashes, normalizeDecoderOptions(options))
}

func decodeMessagesCore(iwave []int16, a7Hints []a7Hint, hashes *hashTable, options decodeOptions) []DecodedMessage {
	return decodeMessagesCoreWithDiagnostics(iwave, a7Hints, hashes, options, nil)
}

func decodeMessagesCoreWithDiagnostics(iwave []int16, a7Hints []a7Hint, hashes *hashTable, options decodeOptions, diagnostics *DecodeDiagnostics) []DecodedMessage {
	seen := make(map[string]bool)
	var out []DecodedMessage
	var fullDD []float32
	if diagnostics != nil {
		diagnostics.A7Hints = len(a7Hints)
	}
	for blockIndex := 0; blockIndex < options.blockCount; blockIndex++ {
		blocks := options.blocks[blockIndex]
		if diagnostics != nil {
			diagnostics.BlocksSearched = append(diagnostics.BlocksSearched, blocks)
		}
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
			if diagnostics != nil {
				diagnostics.CandidateSearches++
				diagnostics.CandidatesFound += len(candidates)
			}
			ds := getDownsampler()
			var subtract []DecodedMessage
			var subtractCodewords [][174]int8
			for candIndex, cand := range candidates {
				if diagnostics != nil {
					diagnostics.CandidatesAnalyzed++
				}
				analysis, decoded, ok := decodeCandidateVariantsForMetricSet(dd, ds, cand, candIndex == 0, pass, hashes, options, diagnostics)
				if !ok {
					continue
				}
				if diagnostics != nil {
					diagnostics.DecodedCandidates++
				}
				msg := DecodedMessage{
					Text:           decoded.Text,
					FreqHz:         analysis.Refined.FreqHz,
					DTSec:          analysis.Refined.DTSec - 0.5,
					Sync:           analysis.candidate.Sync,
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
					if diagnostics != nil {
						diagnostics.DuplicateMessages++
					}
					continue
				}
				seen[decoded.Text] = true
				out = append(out, msg)
			}
			putDownsampler(ds)
			if len(subtract) == 0 {
				break
			}
			if diagnostics != nil {
				diagnostics.Subtractions += len(subtract)
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
		a7Decoded := decodeA7Hints(fullDD, a7Hints, seen)
		if diagnostics != nil {
			diagnostics.A7Decoded = len(a7Decoded)
		}
		out = append(out, a7Decoded...)
	}
	if fullDD != nil {
		putDecodeBlocks(fullDD)
	}
	if diagnostics != nil {
		diagnostics.UniqueMessages = len(out)
	}
	return out
}

type candidateDecode struct {
	Text   string
	Result ldpcResult
}

func decodeCandidateVariantsForMetricSet(dd []float32, ds *downsampler, cand candidate, recompute bool, metricSet int, hashes *hashTable, options decodeOptions, diagnostics *DecodeDiagnostics) (candidateAnalysis, candidateDecode, bool) {
	analysis := candidateAnalysis{
		candidate: cand,
		Refined:   refineCandidateDetails(dd, ds, cand, recompute),
	}
	if analysis.Refined.HardSync <= options.hardSyncMin {
		if diagnostics != nil {
			diagnostics.RejectedHardSync++
		}
		return analysis, candidateDecode{}, false
	}
	if !passesCostasGate(analysis.Refined, options) {
		if diagnostics != nil {
			diagnostics.RejectedCostas++
		}
		return analysis, candidateDecode{}, false
	}
	computeCandidateAnalysisMetrics(&analysis, ds, metricSet)
	if decoded, ok := decodeCandidateWithMetricSet(&analysis, metricSet, hashes, options, diagnostics); ok {
		return analysis, decoded, true
	}
	return analysis, candidateDecode{}, false
}

func decodeCandidateWithMetricSet(analysis *candidateAnalysis, metricSet int, hashes *hashTable, options decodeOptions, diagnostics *DecodeDiagnostics) (candidateDecode, bool) {
	if metricSet != 1 {
		if decoded, ok := decodeCandidateMetrics(&analysis.Metrics, hashes, options, diagnostics); ok {
			return decoded, true
		}
	}
	if metricSet != 0 {
		if decoded, ok := decodeCandidateMetrics(&analysis.PowerMetrics, hashes, options, diagnostics); ok {
			return decoded, true
		}
	}
	return candidateDecode{}, false
}

func computeCandidateAnalysisMetrics(analysis *candidateAnalysis, ds *downsampler, metricSet int) {
	if metricSet != 1 {
		analysis.Metrics = computeSoftMetrics(&ds.cs, false)
	}
	if metricSet != 0 {
		analysis.PowerMetrics = computeSoftMetrics(&ds.cs, true)
	}
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

func decodeCandidateMetrics(metrics *softMetrics, hashes *hashTable, options decodeOptions, diagnostics *DecodeDiagnostics) (candidateDecode, bool) {
	for pass := 0; pass < 5; pass++ {
		if decoded, ok := decodeMetricPass(metricPassSource(metrics, pass), hashes, options, diagnostics); ok {
			return decoded, true
		}
	}
	if decoded, ok := decodeAPProfiles(metrics, ft8DefaultAPProfiles, true, hashes, options, diagnostics); ok {
		return decoded, true
	}
	if options.enableBroadAP {
		if decoded, ok := decodeAPProfiles(metrics, ft8BroadAPProfiles, false, hashes, options, diagnostics); ok {
			return decoded, true
		}
	}
	return decodeAPCallHints(metrics, hashes, options, diagnostics)
}

func decodeAPProfiles(metrics *softMetrics, profiles []apProfile, allowOSD bool, hashes *hashTable, options decodeOptions, diagnostics *DecodeDiagnostics) (candidateDecode, bool) {
	for i := range profiles {
		profile := &profiles[i]
		if decoded, ok := decodeAPMetricPass(&metrics.Single, profile, allowOSD, hashes, options, diagnostics); ok {
			return decoded, true
		}
		if decoded, ok := decodeAPMetricPass(&metrics.Triple, profile, allowOSD, hashes, options, diagnostics); ok {
			return decoded, true
		}
	}
	return candidateDecode{}, false
}

func decodeAPCallHints(metrics *softMetrics, hashes *hashTable, options decodeOptions, diagnostics *DecodeDiagnostics) (candidateDecode, bool) {
	var selections [ft8MaxAPCallHypotheses]apHintSelection
	n := selectAPCallHintHypotheses(metrics, options.apCallHints, options.maxAPCallHypotheses, diagnostics, &selections)
	for i := 0; i < n; i++ {
		selection := selections[i]
		profile := apHintProfile(options.apCallHints[selection.hint], selection.field)
		if decoded, ok := decodeAPMetricPass(apHintMetric(metrics, selection), &profile, false, hashes, options, diagnostics); ok {
			return decoded, true
		}
	}
	return candidateDecode{}, false
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

func decodeMetricPass(metric *[174]float64, hashes *hashTable, options decodeOptions, diagnostics *DecodeDiagnostics) (candidateDecode, bool) {
	var llr [174]float64
	for i, v := range metric {
		llr[i] = ft8ScaleFac * v
	}
	shapeLLR(&llr, options)
	return decodeLLRPass(&llr, &ft8NoAPMask, "", "", true, hashes, options, diagnostics)
}

func decodeAPMetricPass(metric *[174]float64, profile *apProfile, allowOSD bool, hashes *hashTable, options decodeOptions, diagnostics *DecodeDiagnostics) (candidateDecode, bool) {
	llr := apProfilePass(*metric, profile)
	shapeLLR(&llr, options)
	return decodeLLRPass(&llr, &profile.mask, profile.name, profile.source, allowOSD, hashes, options, diagnostics)
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
	if n%2 == 0 {
		median = (absVals[n/2-1] + absVals[n/2]) / 2
	}
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

func decodeLLRPass(llr *[174]float64, apmask *[174]int8, apProfileName string, apSource string, allowOSD bool, hashes *hashTable, options decodeOptions, diagnostics *DecodeDiagnostics) (candidateDecode, bool) {
	if diagnostics != nil {
		diagnostics.recordLDPCAttempt(apProfileName, apSource)
	}
	var result ldpcResult
	var ok bool
	if options.enableOSD && allowOSD {
		result, ok = decode17491HybridWithAP(llr, apmask)
	} else {
		result, ok, _ = decode17491BP(llr, apmask, nil)
	}
	if !ok {
		if diagnostics != nil {
			diagnostics.LDPCFailures++
		}
		return candidateDecode{}, false
	}
	if result.HardErrors < 0 || result.HardErrors > ft8MaxHardErrors {
		if diagnostics != nil {
			diagnostics.RejectedHardErrors++
			diagnostics.recordAPRejectedAfterLDPC(apProfileName, apSource)
		}
		return candidateDecode{}, false
	}
	if allZeroCodeword(result.Codeword) {
		if diagnostics != nil {
			diagnostics.RejectedAllZero++
			diagnostics.recordAPRejectedAfterLDPC(apProfileName, apSource)
		}
		return candidateDecode{}, false
	}
	msg, ok := unpack77FromCodewordWithHashes(result.Codeword, hashes)
	if !ok {
		if diagnostics != nil {
			diagnostics.UnpackFailures++
			diagnostics.recordAPRejectedAfterLDPC(apProfileName, apSource)
		}
		return candidateDecode{}, false
	}
	if strings.Contains(msg, "/R") || strings.HasPrefix(msg, "TU; ") {
		if diagnostics != nil {
			diagnostics.RejectedMessageFilter++
			diagnostics.recordAPRejectedAfterLDPC(apProfileName, apSource)
		}
		return candidateDecode{}, false
	}
	if diagnostics != nil {
		diagnostics.recordAPSuccess(apProfileName, apSource)
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
