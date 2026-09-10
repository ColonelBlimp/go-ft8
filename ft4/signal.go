// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

import (
	"math"
	"sort"
	"sync"

	"gonum.org/v1/gonum/dsp/fourier"
)

type candidateWorkspace struct {
	fft         *fourier.FFT
	input       []float64
	coeff       []complex128
	average     []float64
	baseline    []float64
	ratio       []float64
	sortScratch []float64
	candidates  []candidate
}

type decodeWorkspace struct {
	samples     []float64
	candidates  candidateWorkspace
	downsampler downsampler
}

var (
	coarseWindow  = nuttallWindow(coarseFFT)
	workspacePool = sync.Pool{New: func() any {
		return &decodeWorkspace{
			samples: make([]float64, decodeSamples),
			candidates: candidateWorkspace{
				fft:         fourier.NewFFT(coarseFFT),
				input:       make([]float64, coarseFFT),
				coeff:       make([]complex128, coarseBins+1),
				average:     make([]float64, coarseBins+1),
				baseline:    make([]float64, coarseBins+1),
				ratio:       make([]float64, coarseBins+1),
				sortScratch: make([]float64, coarseBins+1),
				candidates:  make([]candidate, 0, coarseBins),
			},
			downsampler: newDownsampler(),
		}
	}}
)

func acquireDecodeWorkspace() *decodeWorkspace {
	workspace := workspacePool.Get().(*decodeWorkspace)
	clear(workspace.samples)
	return workspace
}

func releaseDecodeWorkspace(workspace *decodeWorkspace) {
	workspacePool.Put(workspace)
}

func findCandidates(samples []float64, options decodeOptions, workspace *candidateWorkspace) []candidate {
	input := workspace.input
	average := workspace.average
	clear(average)

	for step := 0; step < coarseSteps; step++ {
		start := step * SamplesPerSymbol
		for i := range input {
			input[i] = samples[start+i] * coarseWindow[i] / 300
		}
		coeff := workspace.fft.Coefficients(workspace.coeff, input)
		for bin := 1; bin <= coarseBins; bin++ {
			average[bin] += complexPower(coeff[bin])
		}
	}
	if coarseSteps > 0 {
		for i := range average {
			average[i] /= coarseSteps
		}
	}
	if !hasPositive(average) {
		return nil
	}

	df := float64(SampleRate) / coarseFFT
	first := max(8, int(math.Round(float64(options.minFreqHz)/df)))
	last := min(coarseBins-8, int(math.Round(float64(options.maxFreqHz+2*downRate/downNSPS)/df)))
	if first >= last {
		return nil
	}
	baseline := workspace.baseline
	spectrumBaseline(average, first, last, baseline, workspace.sortScratch)
	ratio := workspace.ratio
	clear(ratio)
	for bin := first; bin <= last; bin++ {
		var smoothed float64
		for offset := -7; offset <= 7; offset++ {
			smoothed += average[bin+offset]
		}
		smoothed /= 15
		if baseline[bin] > 0 {
			ratio[bin] = smoothed / baseline[bin]
		}
	}

	baud := float64(SampleRate) / SamplesPerSymbol
	frequencyOffset := -1.5 * baud
	candidates := workspace.candidates[:0]
	for bin := first + 1; bin < last; bin++ {
		if ratio[bin] < options.syncMin || ratio[bin] < ratio[bin-1] || ratio[bin] < ratio[bin+1] {
			continue
		}
		denominator := ratio[bin-1] - 2*ratio[bin] + ratio[bin+1]
		delta := 0.0
		if denominator != 0 {
			delta = 0.5 * (ratio[bin-1] - ratio[bin+1]) / denominator
		}
		frequency := (float64(bin)+delta)*df + frequencyOffset
		if frequency < float64(options.minFreqHz) || frequency > float64(options.maxFreqHz) {
			continue
		}
		strength := ratio[bin] - 0.25*(ratio[bin-1]-ratio[bin+1])*delta
		candidates = append(candidates, candidate{freqHz: frequency, strength: strength})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].strength > candidates[j].strength
	})
	if len(candidates) > options.maxCandidates {
		candidates = candidates[:options.maxCandidates]
	}
	workspace.candidates = candidates[:0]
	return candidates
}

func nuttallWindow(n int) []float64 {
	window := make([]float64, n)
	const (
		a0 = 0.3635819
		a1 = -0.4891775
		a2 = 0.1365995
		a3 = -0.0106411
	)
	for i := range window {
		phase := 2 * math.Pi * float64(i) / float64(n)
		window[i] = a0 + a1*math.Cos(phase) + a2*math.Cos(2*phase) + a3*math.Cos(3*phase)
	}
	return window
}

func hasPositive(values []float64) bool {
	for _, value := range values {
		if value > 0 && !math.IsInf(value, 0) && !math.IsNaN(value) {
			return true
		}
	}
	return false
}

func spectrumBaseline(spectrum []float64, first, last int, baseline, scratch []float64) {
	clear(baseline)
	width := last - first + 1
	if width < 10 {
		return
	}
	center := float64(first+last) / 2
	halfWidth := math.Max(1, float64(width)/2)
	var normal [5][6]float64
	segmentLength := width / 10
	points := 0
	for segment := 0; segment < 10; segment++ {
		start := first + segment*segmentLength
		end := start + segmentLength
		if segment == 9 {
			end = last + 1
		}
		values := scratch[:end-start]
		for bin := start; bin < end; bin++ {
			values[bin-start] = powerDB(spectrum[bin])
		}
		sort.Float64s(values)
		percentile := max(0, int(math.Round(float64(len(values))*0.10))-1)
		threshold := values[percentile]
		for bin := start; bin < end; bin++ {
			y := powerDB(spectrum[bin])
			if y > threshold {
				continue
			}
			x := (float64(bin) - center) / halfWidth
			var powers [9]float64
			powers[0] = 1
			for i := 1; i < len(powers); i++ {
				powers[i] = powers[i-1] * x
			}
			for row := 0; row < 5; row++ {
				for col := 0; col < 5; col++ {
					normal[row][col] += powers[row+col]
				}
				normal[row][5] += y * powers[row]
			}
			points++
		}
	}
	coeff, ok := solve5(normal)
	if !ok || points < 5 {
		values := scratch[:width]
		copy(values, spectrum[first:last+1])
		sort.Float64s(values)
		index := int(math.Round(float64(len(values))*0.10)) - 1
		index = max(0, min(index, len(values)-1))
		fallback := values[index]
		for bin := first; bin <= last; bin++ {
			baseline[bin] = fallback
		}
		return
	}
	for bin := first; bin <= last; bin++ {
		x := (float64(bin) - center) / halfWidth
		y := coeff[0] + x*(coeff[1]+x*(coeff[2]+x*(coeff[3]+x*coeff[4]))) + 0.65
		baseline[bin] = math.Pow(10, y/10)
	}
}

func powerDB(value float64) float64 {
	if value < math.SmallestNonzeroFloat64 {
		value = math.SmallestNonzeroFloat64
	}
	return 10 * math.Log10(value)
}

func solve5(matrix [5][6]float64) ([5]float64, bool) {
	for column := 0; column < 5; column++ {
		pivot := column
		for row := column + 1; row < 5; row++ {
			if math.Abs(matrix[row][column]) > math.Abs(matrix[pivot][column]) {
				pivot = row
			}
		}
		if math.Abs(matrix[pivot][column]) < 1e-12 {
			return [5]float64{}, false
		}
		matrix[column], matrix[pivot] = matrix[pivot], matrix[column]
		divisor := matrix[column][column]
		for col := column; col < 6; col++ {
			matrix[column][col] /= divisor
		}
		for row := 0; row < 5; row++ {
			if row == column {
				continue
			}
			factor := matrix[row][column]
			for col := column; col < 6; col++ {
				matrix[row][col] -= factor * matrix[column][col]
			}
		}
	}
	var result [5]float64
	for i := range result {
		result[i] = matrix[i][5]
	}
	return result, true
}

type downsampler struct {
	forward      *fourier.FFT
	fullSpectrum []complex128
	inverse      *fourier.CmplxFFT
	spectrum     []complex128
	baseband     []complex128
}

func newDownsampler() downsampler {
	return downsampler{
		forward:      fourier.NewFFT(decodeSamples),
		fullSpectrum: make([]complex128, decodeSamples/2+1),
		inverse:      fourier.NewCmplxFFT(downFFT),
		spectrum:     make([]complex128, downFFT),
		baseband:     make([]complex128, downFFT),
	}
}

func (d *downsampler) reset(samples []float64) {
	d.forward.Coefficients(d.fullSpectrum, samples)
}

func (d *downsampler) at(frequency float64) []complex128 {
	df := float64(SampleRate) / decodeSamples
	baseBin := int(math.Round(frequency / df))
	spectrum := d.spectrum
	clear(spectrum)
	half := downFFT / 2
	for offset := 0; offset <= half; offset++ {
		offsetHz := float64(offset) * df
		if bin := baseBin + offset; bin >= 0 && bin < len(d.fullSpectrum) {
			spectrum[offset] = d.fullSpectrum[bin] * complex(ft4DownsampleWindow(offsetHz), 0)
		}
		if offset > 0 && offset < half {
			if bin := baseBin - offset; bin >= 0 && bin < len(d.fullSpectrum) {
				spectrum[downFFT-offset] = d.fullSpectrum[bin] * complex(ft4DownsampleWindow(-offsetHz), 0)
			}
		}
	}
	return d.inverse.Sequence(d.baseband, spectrum)
}

func ft4DownsampleWindow(offsetHz float64) float64 {
	baud := float64(SampleRate) / SamplesPerSymbol
	switch {
	case offsetHz < -baud || offsetHz > 4*baud:
		return 0
	case offsetHz < -0.5*baud:
		x := (offsetHz + baud) / (0.5 * baud)
		return 0.5 * (1 - math.Cos(math.Pi*x))
	case offsetHz <= 3.5*baud:
		return 1
	default:
		x := (offsetHz - 3.5*baud) / (0.5 * baud)
		return 0.5 * (1 + math.Cos(math.Pi*x))
	}
}

func normalizeComplex(samples []complex128) {
	var sum float64
	for _, sample := range samples {
		sum += complexPower(sample)
	}
	if sum <= 0 {
		return
	}
	scale := math.Sqrt(float64(len(samples)) / sum)
	for i := range samples {
		samples[i] *= complex(scale, 0)
	}
}

var syncReferences = makeSyncReferences()

func makeSyncReferences() [33][4][64]complex128 {
	var references [33][4][64]complex128
	for delta := -16; delta <= 16; delta++ {
		for block, tones := range costasArrays {
			phase := 0.0
			index := 0
			for _, tone := range tones {
				step := 4 * math.Pi * float64(tone) / downNSPS
				for sample := 0; sample < downNSPS/2; sample++ {
					t := float64(2*index) / downRate
					tweak := 2 * math.Pi * float64(delta) * t
					// Store the conjugated reference used by every sync score so
					// the hot search loop only performs the complex multiply.
					references[delta+16][block][index] = complex(math.Cos(phase+tweak), -math.Sin(phase+tweak))
					phase = math.Mod(phase+step, 2*math.Pi)
					index++
				}
			}
		}
	}
	return references
}

func refineSync(samples []complex128) (int, float64, float64) {
	bestStart := 0
	bestDelta := 0
	bestScore := -1.0
	for delta := -12; delta <= 12; delta += 3 {
		for start := -344; start <= 1012; start += 4 {
			score := syncScore(samples, start, delta)
			if score > bestScore {
				bestStart, bestDelta, bestScore = start, delta, score
			}
		}
	}
	coarseStart := bestStart
	coarseDelta := bestDelta
	for delta := coarseDelta - 4; delta <= coarseDelta+4; delta++ {
		if delta < -16 || delta > 16 {
			continue
		}
		for start := coarseStart - 5; start <= coarseStart+5; start++ {
			score := syncScore(samples, start, delta)
			if score > bestScore {
				bestStart, bestDelta, bestScore = start, delta, score
			}
		}
	}
	return bestStart, float64(bestDelta), bestScore
}

func syncScore(samples []complex128, start, delta int) float64 {
	if delta < -16 || delta > 16 {
		return 0
	}
	var score float64
	for block, symbolStart := range [...]int{0, 33, 66, 99} {
		blockStart := start + symbolStart*downNSPS
		score += syncBlockScore(samples, blockStart, &syncReferences[delta+16][block])
	}
	return score
}

func syncBlockScore(samples []complex128, start int, reference *[64]complex128) float64 {
	if start >= len(samples) || start+2*(len(reference)-1) < 0 {
		return 0
	}
	first := 0
	if start < 0 {
		first = (-start + 1) / 2
	}
	end := len(reference)
	if last := (len(samples)-1-start)/2 + 1; last < end {
		end = last
	}
	if end-first <= 16 {
		return 0
	}
	var sum complex128
	for i := first; i < end; i++ {
		sum += samples[start+2*i] * reference[i]
	}
	return cmplxAbs(sum) / float64(2*downNSPS)
}

func extractFrame(samples []complex128, start int) [ToneSymbols * downNSPS]complex128 {
	var frame [ToneSymbols * downNSPS]complex128
	for i := range frame {
		index := start + i
		if index >= 0 && index < len(samples) {
			frame[i] = samples[index]
		}
	}
	return frame
}

var correlationWaves = makeCorrelationWaves()

func makeCorrelationWaves() [4][downNSPS]complex128 {
	var waves [4][downNSPS]complex128
	for tone := range waves {
		for sample := range waves[tone] {
			phase := -2 * math.Pi * float64(tone*sample) / downNSPS
			waves[tone][sample] = complex(math.Cos(phase), math.Sin(phase))
		}
	}
	return waves
}

func symbolCorrelations(frame *[ToneSymbols * downNSPS]complex128) [4][ToneSymbols]complex128 {
	var correlations [4][ToneSymbols]complex128
	for symbol := 0; symbol < ToneSymbols; symbol++ {
		start := symbol * downNSPS
		for tone := 0; tone < 4; tone++ {
			var sum complex128
			for sample := 0; sample < downNSPS; sample++ {
				sum += frame[start+sample] * correlationWaves[tone][sample]
			}
			correlations[tone][symbol] = sum
		}
	}
	return correlations
}

func hardSyncCount(correlations *[4][ToneSymbols]complex128) int {
	wins := 0
	for block, start := range [...]int{0, 33, 66, 99} {
		for offset, want := range costasArrays[block] {
			bestTone := 0
			best := -1.0
			for tone := 0; tone < 4; tone++ {
				power := complexPower(correlations[tone][start+offset])
				if power > best {
					best = power
					bestTone = tone
				}
			}
			if uint8(bestTone) == want {
				wins++
			}
		}
	}
	return wins
}

func ft4BitMetrics(correlations *[4][ToneSymbols]complex128) [3][2 * ToneSymbols]float64 {
	var metrics [3][2 * ToneSymbols]float64
	for pass, symbols := range [...]int{1, 2, 4} {
		combinations := 1 << (2 * symbols)
		var values [256]float64
		for start := 0; start <= ToneSymbols-symbols; start += symbols {
			for sequence := 0; sequence < combinations; sequence++ {
				var sum complex128
				for offset := 0; offset < symbols; offset++ {
					shift := 2 * (symbols - 1 - offset)
					pair := sequence >> shift & 3
					tone := grayMap[pair]
					sum += correlations[tone][start+offset]
				}
				values[sequence] = cmplxAbs(sum)
			}
			for bit := 0; bit < 2*symbols; bit++ {
				one, zero := metricSplitMax(values[:combinations], 2*symbols-1-bit)
				metrics[pass][2*start+bit] = one - zero
			}
		}
	}
	copy(metrics[1][204:206], metrics[0][204:206])
	copy(metrics[2][200:204], metrics[1][200:204])
	copy(metrics[2][204:206], metrics[0][204:206])
	for pass := range metrics {
		normalizeMetric(metrics[pass][:])
	}
	return metrics
}

func metricSplitMax(values []float64, bit int) (float64, float64) {
	one := math.Inf(-1)
	zero := math.Inf(-1)
	for value, metric := range values {
		if value&(1<<bit) != 0 {
			one = math.Max(one, metric)
		} else {
			zero = math.Max(zero, metric)
		}
	}
	return one, zero
}

func normalizeMetric(metric []float64) {
	var sum, sumSquares float64
	for _, value := range metric {
		sum += value
		sumSquares += value * value
	}
	mean := sum / float64(len(metric))
	variance := sumSquares/float64(len(metric)) - mean*mean
	if variance <= 0 {
		return
	}
	scale := math.Sqrt(variance)
	for i := range metric {
		metric[i] /= scale
	}
}

func syncBitQuality(single *[2 * ToneSymbols]float64) int {
	quality := 0
	for block, start := range [...]int{0, 33, 66, 99} {
		for offset, tone := range costasArrays[block] {
			pair := grayMap[tone]
			for bit := 0; bit < 2; bit++ {
				want := pair >> uint(1-bit) & 1
				got := uint8(0)
				if single[2*(start+offset)+bit] >= 0 {
					got = 1
				}
				if got == want {
					quality++
				}
			}
		}
	}
	return quality
}

func dataLLR(metrics *[2 * ToneSymbols]float64) [174]float64 {
	var llr [174]float64
	copy(llr[0:58], metrics[8:66])
	copy(llr[58:116], metrics[74:132])
	copy(llr[116:174], metrics[140:198])
	for i := range llr {
		llr[i] *= llrScale
	}
	return llr
}

func complexPower(value complex128) float64 {
	return real(value)*real(value) + imag(value)*imag(value)
}

func cmplxAbs(value complex128) float64 {
	return math.Sqrt(complexPower(value))
}
