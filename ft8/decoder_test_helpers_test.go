// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

func findDefaultCandidates(dd []float32, minFreqHz, maxFreqHz int) []candidate {
	return findCandidates(dd, minFreqHz, maxFreqHz, ft8DefaultSyncMin, 0, ft8DefaultMaxCand)
}

func refineCandidateWithDownsampler(dd []float32, ds *downsampler, cand candidate, recompute bool) refinedCandidate {
	return refineCandidateDetails(dd, ds, cand, recompute)
}

func decodeCandidateMessage(analysis candidateAnalysis) (string, bool) {
	decoded, ok := decodeCandidate(analysis)
	if !ok {
		return "", false
	}
	return decoded.Text, true
}

func decodeCandidateVariants(dd []float32, ds *downsampler, cand candidate, recompute bool) (candidateAnalysis, candidateDecode, bool) {
	return decodeCandidateVariantsForMetricSetNoHashes(dd, ds, cand, recompute, 2)
}

func decodeCandidateVariantsForMetricSetNoHashes(dd []float32, ds *downsampler, cand candidate, recompute bool, metricSet int) (candidateAnalysis, candidateDecode, bool) {
	return decodeCandidateVariantsForMetricSet(dd, ds, cand, recompute, metricSet, nil, normalizeDecoderOptions(DecoderOptions{}), nil)
}

func decodeCandidate(analysis candidateAnalysis) (candidateDecode, bool) {
	return decodeCandidateWithMetricSetNoHashes(analysis, 2)
}

func decodeCandidateWithMetricSetNoHashes(analysis candidateAnalysis, metricSet int) (candidateDecode, bool) {
	return decodeCandidateWithMetricSet(&analysis, metricSet, nil, normalizeDecoderOptions(DecoderOptions{}), nil)
}

func unpack77FromCodeword(cw [174]int8) (string, bool) {
	return unpack77FromCodewordWithHashes(cw, nil)
}

func llrPasses(metrics softMetrics) [5][174]float64 {
	return [5][174]float64{
		scaleLLR(metrics.Single),
		scaleLLR(metrics.Double),
		scaleLLR(metrics.Triple),
		scaleLLR(metrics.Normed),
		scaleLLR(metrics.Best),
	}
}

func llrPassesWithCQAP(metrics softMetrics) [7][174]float64 {
	var out [7][174]float64
	regular := llrPasses(metrics)
	copy(out[:5], regular[:])
	out[5] = apCQPass(metrics.Single)
	out[6] = apCQPass(metrics.Triple)
	return out
}

type decoderPass struct {
	LLR    [174]float64
	APMask [174]int8
}

func analysisLLRPasses(analysis candidateAnalysis) [14]decoderPass {
	var out [14]decoderPass
	regular := llrPassesWithCQAP(analysis.Metrics)
	power := llrPassesWithCQAP(analysis.PowerMetrics)
	for i, llr := range regular {
		out[i].LLR = llr
		if i >= 5 {
			out[i].APMask = *cqAPMask()
		}
	}
	for i, llr := range power {
		out[i+7].LLR = llr
		if i >= 5 {
			out[i+7].APMask = *cqAPMask()
		}
	}
	return out
}
