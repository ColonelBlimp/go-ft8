// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
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
	SymbolPower  [8][ft8Symbols]float64
}

func analyzeCandidateWithDownsampler(dd []float32, ds *downsampler, cand candidate, recompute bool) candidateAnalysis {
	return analyzeCandidateWithDownsamplerForMetricSet(dd, ds, cand, recompute, 2)
}

func analyzeCandidateWithDownsamplerForMetricSet(dd []float32, ds *downsampler, cand candidate, recompute bool, metricSet int) candidateAnalysis {
	refined := refineCandidateDetails(dd, ds, cand, recompute)
	analysis := candidateAnalysis{
		candidate:   cand,
		Refined:     refined,
		SymbolPower: ds.symbolPower,
	}
	computeCandidateAnalysisMetrics(&analysis, ds, metricSet)
	return analysis
}

func computeSoftMetrics(cs *[8][ft8Symbols]complex128, squared bool) softMetrics {
	var m softMetrics
	var s2buf [512]float64

	for nsym := 1; nsym <= 3; nsym++ {
		nt := 1 << (3 * nsym)
		for half := 1; half <= 2; half++ {
			for k := 1; k <= 29; k += nsym {
				ks := k + 7
				if half == 2 {
					ks = k + 43
				}

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

func apCQPass(metric [174]float64) [174]float64 {
	return apProfilePass(metric, &ft8CQAPProfile)
}

func apProfilePass(metric [174]float64, profile *apProfile) [174]float64 {
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

	for i, known := range profile.mask {
		if known == 0 {
			continue
		}
		if profile.bits[i] == 1 {
			llr[i] = apMag
		} else {
			llr[i] = -apMag
		}
	}
	return llr
}

var ft8NoAPMask [174]int8
var ft8CQAPProfile = initCall1APProfile("cq", "CQ", 1)
var ft8DefaultAPProfiles = []apProfile{ft8CQAPProfile}
var ft8BroadAPProfiles = initBroadAPProfiles()

type apProfile struct {
	name   string
	source string
	bits   [174]int8
	mask   [174]int8
}

func cqAPMask() *[174]int8 {
	return &ft8CQAPProfile.mask
}

func initBroadAPProfiles() []apProfile {
	profiles := []apProfile{
		initCall1APProfile("cq-dx", "CQ_DX", 1),
		initCall1APProfile("cq-test", "CQ_TEST", 1),
		initCall1APProfile("cq-pota", "CQ_POTA", 1),
		initCall1APProfile("cq-sota", "CQ_SOTA", 1),
		initCall1APProfile("cq-qrp", "CQ_QRP", 1),
		initCall1APProfile("cq-cota", "CQ_COTA", 1),
		initCall1APProfile("cq-fd", "CQ_FD", 1),
	}
	return profiles
}

func initCall1APProfile(name, callToken string, i3 int) apProfile {
	profile := initStandardTypeAPProfile(name, i3)
	n28, ok := pack28(callToken)
	if !ok {
		panic("invalid AP call token: " + callToken)
	}
	setAPBits(&profile, 0, 28, n28)
	setAPBits(&profile, 28, 1, 0)
	return profile
}

func initStandardTypeAPProfile(name string, i3 int) apProfile {
	var profile apProfile
	profile.name = name
	profile.source = name
	setAPBits(&profile, 74, 3, i3)
	return profile
}

func setAPBits(profile *apProfile, start int, width int, value int) {
	for i := 0; i < width; i++ {
		shift := uint(width - 1 - i)
		pos := start + i
		profile.bits[pos] = int8((value >> shift) & 1)
		profile.mask[pos] = 1
	}
}
