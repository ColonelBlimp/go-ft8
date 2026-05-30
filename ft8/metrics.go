// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "math"

const ft8ScaleFac = 2.83

var ft8GrayMap = [8]int{0, 1, 3, 2, 5, 6, 4, 7}

type softMetrics struct {
	Single [174]float64
	Double [174]float64
	Triple [174]float64
	Normed [174]float64
	Best   [174]float64
}

type candidateAnalysis struct {
	candidate    candidate
	Refined      refinedCandidate
	Metrics      softMetrics
	PowerMetrics softMetrics
}

func analyzeCandidate(dd []float32, cand candidate) candidateAnalysis {
	ds := newDownsampler()
	return analyzeCandidateWithDownsampler(dd, ds, cand, true)
}

func analyzeCandidateWithDownsampler(dd []float32, ds *downsampler, cand candidate, recompute bool) candidateAnalysis {
	return analyzeCandidateWithDownsamplerForMetricSet(dd, ds, cand, recompute, 2)
}

func analyzeCandidateWithDownsamplerForMetricSet(dd []float32, ds *downsampler, cand candidate, recompute bool, metricSet int) candidateAnalysis {
	refined, cd0, start := refineCandidateDetails(dd, ds, cand, recompute)
	cs, _ := ds.symbolSpectra(cd0, start)
	analysis := candidateAnalysis{
		candidate: cand,
		Refined:   refined,
	}
	if metricSet != 1 {
		analysis.Metrics = computeSoftMetrics(cs, false)
	}
	if metricSet != 0 {
		analysis.PowerMetrics = computeSoftMetrics(cs, true)
	}
	return analysis
}

func computeSoftMetrics(cs [8][ft8Symbols]complex128, squared bool) softMetrics {
	var m softMetrics

	for nsym := 1; nsym <= 3; nsym++ {
		nt := 1 << (3 * nsym)
		for half := 1; half <= 2; half++ {
			for k := 1; k <= 29; k += nsym {
				ks := k + 7
				if half == 2 {
					ks = k + 43
				}

				var s2buf [512]float64
				s2 := s2buf[:nt]
				for i := 0; i < nt; i++ {
					i1 := i / 64
					i2 := (i & 63) / 8
					i3 := i & 7
					switch nsym {
					case 1:
						s2[i] = metricMagnitude(cs[ft8GrayMap[i3]][ks-1], squared)
					case 2:
						s2[i] = metricMagnitude(cs[ft8GrayMap[i2]][ks-1]+cs[ft8GrayMap[i3]][ks], squared)
					case 3:
						s2[i] = metricMagnitude(cs[ft8GrayMap[i1]][ks-1]+cs[ft8GrayMap[i2]][ks]+cs[ft8GrayMap[i3]][ks+1], squared)
					}
				}

				i32 := 1 + (k-1)*3 + (half-1)*87
				ibmax := 2
				if nsym == 2 {
					ibmax = 5
				} else if nsym == 3 {
					ibmax = 8
				}
				for ib := 0; ib <= ibmax; ib++ {
					idx := i32 + ib - 1
					if idx >= 174 {
						continue
					}
					oneMax, zeroMax := splitMax(s2, ibmax-ib)
					bm := oneMax - zeroMax
					switch nsym {
					case 1:
						m.Single[idx] = bm
						den := math.Max(oneMax, zeroMax)
						if den > 0 {
							m.Normed[idx] = bm / den
						}
					case 2:
						m.Double[idx] = bm
					case 3:
						m.Triple[idx] = bm
					}
				}
			}
		}
	}

	for i := range m.Best {
		m.Best[i] = m.Single[i]
		if math.Abs(m.Double[i]) > math.Abs(m.Best[i]) {
			m.Best[i] = m.Double[i]
		}
		if math.Abs(m.Triple[i]) > math.Abs(m.Best[i]) {
			m.Best[i] = m.Triple[i]
		}
	}

	normalizeMetric(&m.Single)
	normalizeMetric(&m.Double)
	normalizeMetric(&m.Triple)
	normalizeMetric(&m.Normed)
	normalizeMetric(&m.Best)
	return m
}

func metricMagnitude(z complex128, squared bool) float64 {
	if squared {
		return complexPower(z)
	}
	return cmplxAbs(z)
}

func splitMax(values []float64, bit int) (float64, float64) {
	oneMax := math.Inf(-1)
	zeroMax := math.Inf(-1)
	block := 1 << bit
	step := block << 1
	for start := 0; start < len(values); start += step {
		for i := 0; i < block; i++ {
			z := values[start+i]
			if z > zeroMax {
				zeroMax = z
			}
			o := values[start+block+i]
			if o > oneMax {
				oneMax = o
			}
		}
	}
	return oneMax, zeroMax
}

func normalizeMetric(metric *[174]float64) {
	var sum, sumSq float64
	for _, v := range metric {
		sum += v
		sumSq += v * v
	}
	mean := sum / float64(len(metric))
	meanSq := sumSq / float64(len(metric))
	variance := meanSq - mean*mean
	scale := math.Sqrt(meanSq)
	if variance > 0 {
		scale = math.Sqrt(variance)
	}
	if scale == 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return
	}
	for i := range metric {
		metric[i] /= scale
	}
}

func llrPasses(metrics softMetrics) [5][174]float64 {
	var out [5][174]float64
	sources := [5][174]float64{metrics.Single, metrics.Double, metrics.Triple, metrics.Normed, metrics.Best}
	for pass := range sources {
		for i, v := range sources[pass] {
			out[pass][i] = ft8ScaleFac * v
		}
	}
	return out
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
			out[i].APMask = cqAPMask()
		}
	}
	for i, llr := range power {
		out[i+7].LLR = llr
		if i >= 5 {
			out[i+7].APMask = cqAPMask()
		}
	}
	return out
}

func apCQPass(metric [174]float64) [174]float64 {
	var llr [174]float64
	maxAbs := 0.0
	for i, v := range metric {
		llr[i] = ft8ScaleFac * v
		if math.Abs(llr[i]) > maxAbs {
			maxAbs = math.Abs(llr[i])
		}
	}
	apMag := maxAbs * 1.1
	if apMag == 0 {
		apMag = ft8ScaleFac
	}

	for i, bit := range ft8APCQBits {
		if bit == 1 {
			llr[i] = apMag
		} else {
			llr[i] = -apMag
		}
	}
	llr[74] = -apMag
	llr[75] = -apMag
	llr[76] = apMag
	return llr
}

var ft8APCQBits = [29]int8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0}

func cqAPMask() [174]int8 {
	var mask [174]int8
	for i := 0; i < 29; i++ {
		mask[i] = 1
	}
	mask[74] = 1
	mask[75] = 1
	mask[76] = 1
	return mask
}
