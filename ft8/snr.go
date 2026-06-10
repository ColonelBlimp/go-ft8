// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"sort"
	"sync"
)

const (
	ft8SNRReferenceOffsetDB = 27.0
	ft8SNRBaselineScale     = 3.0e6 * ft8DownsampleFFT2 * ft8DownsampleFFT2
)

var (
	snrBaselineWindowOnce sync.Once
	snrBaselineWindow     [ft8NFFT1]float64
	snrBaselineScratch    = sync.Pool{
		New: func() any {
			return &snrScratch{
				fft:   newRealFFTPlan(ft8NFFT1),
				input: make([]float64, ft8NFFT1),
				coeff: make([]complex128, ft8NFFT1/2+1),
			}
		},
	}
)

type snrScratch struct {
	fft   *realFFTPlan
	input []float64
	coeff []complex128
}

func estimateSNR(tones [ft8Symbols]int, symbolPower [8][ft8Symbols]float64, baseNoise float64) int {
	xsig := 0.0
	xnoi := 0.0
	for sym, tone := range tones {
		if tone < 0 || tone >= len(symbolPower) {
			continue
		}
		sig := symbolPower[tone][sym]
		noiseTone := (tone + 4) % 8
		noise := symbolPower[noiseTone][sym]
		xsig += sig * sig
		xnoi += noise * noise
	}

	ratio := snrRatioFromBaseline(xsig, baseNoise)
	if ratio <= 0 {
		ratio = snrRatioFromOffTone(xsig, xnoi)
	}
	snr := 10.0*math.Log10(ratio) - ft8SNRReferenceOffsetDB
	if snr < -25.0 {
		snr = -25.0
	}
	return nint(snr)
}

func snrRatioFromBaseline(xsig float64, baseNoise float64) float64 {
	if xsig <= 0 || baseNoise <= 0 || math.IsNaN(baseNoise) || math.IsInf(baseNoise, 0) {
		return 0
	}
	arg := xsig/baseNoise/ft8SNRBaselineScale - 1.0
	if arg > 0.1 && !math.IsNaN(arg) && !math.IsInf(arg, 0) {
		return arg
	}
	return 0
}

func snrRatioFromOffTone(xsig float64, xnoi float64) float64 {
	ratio := 0.001
	if xnoi > 0 {
		arg := xsig/xnoi - 1.0
		if arg > 0.1 && !math.IsNaN(arg) && !math.IsInf(arg, 0) {
			ratio = arg
		}
	}
	return ratio
}

func spectrumBaseline(dd []float32, minFreqHz int, maxFreqHz int) []float64 {
	const (
		baselineWindows  = 93
		baselineStep     = ft8NFFT1 / 2
		baselineSegments = 10
	)
	snrBaselineWindowOnce.Do(initSNRBaselineWindow)
	scratch := snrBaselineScratch.Get().(*snrScratch)
	defer snrBaselineScratch.Put(scratch)

	var savg [ft8NBins + 1]float64
	for win := 0; win < baselineWindows; win++ {
		start := win * baselineStep
		if start+ft8NFFT1 > len(dd) {
			break
		}
		for i := 0; i < ft8NFFT1; i++ {
			scratch.input[i] = float64(dd[start+i]) * snrBaselineWindow[i]
		}
		coeff := scratch.fft.workCoefficients(scratch.coeff, scratch.input)
		for bin := 1; bin <= ft8NBins; bin++ {
			z := coeff[bin]
			savg[bin] += real(z)*real(z) + imag(z)*imag(z)
		}
	}

	nfa, nfb := adjustBaselineRange(minFreqHz, maxFreqHz)
	df := float64(wantSampleRate) / float64(ft8NFFT1)
	ia := max(1, nint(float64(nfa)/df))
	ib := min(ft8NBins, nint(float64(nfb)/df))
	if ia > ib {
		return nil
	}
	s := make([]float64, ft8NBins+1)
	for i := ia; i <= ib; i++ {
		if savg[i] > 0 {
			s[i] = 10.0 * math.Log10(savg[i])
		} else {
			s[i] = -99.0
		}
	}

	return fitSpectrumBaseline(s, ia, ib, baselineSegments)
}

func initSNRBaselineWindow() {
	const (
		a0 = 0.3635819
		a1 = -0.4891775
		a2 = 0.1365995
		a3 = -0.0106411
	)
	sum := 0.0
	for i := range snrBaselineWindow {
		x := float64(i) / float64(ft8NFFT1)
		w := a0 + a1*math.Cos(2*math.Pi*x) + a2*math.Cos(4*math.Pi*x) + a3*math.Cos(6*math.Pi*x)
		snrBaselineWindow[i] = w
		sum += w
	}
	if sum == 0 {
		return
	}
	scale := float64(ft8SamplesPerSymbol) * 2.0 / 300.0 / sum
	for i := range snrBaselineWindow {
		snrBaselineWindow[i] *= scale
	}
}

func adjustBaselineRange(minFreqHz int, maxFreqHz int) (int, int) {
	nfa := minFreqHz
	nfb := maxFreqHz
	nwin := nfb - nfa
	if nfa < 100 {
		nfa = 100
		if nwin < 100 {
			nfb = nfa + nwin
		}
	}
	if nfb > 4910 {
		nfb = 4910
		if nwin < 100 {
			nfa = nfb - nwin
		}
	}
	return nfa, nfb
}

func fitSpectrumBaseline(s []float64, ia int, ib int, segments int) []float64 {
	const percentile = 10
	nlen := (ib - ia + 1) / segments
	if nlen <= 0 {
		return nil
	}
	i0 := (ib - ia + 1) / 2
	x := make([]float64, 0, 1000)
	y := make([]float64, 0, 1000)
	for segment := 0; segment < segments; segment++ {
		ja := ia + segment*nlen
		jb := ja + nlen - 1
		if ja > ib {
			break
		}
		if jb > ib {
			jb = ib
		}
		base := percentileValue(s[ja:jb+1], percentile)
		for i := ja; i <= jb && len(x) < 1000; i++ {
			if s[i] <= base {
				x = append(x, float64(i-i0))
				y = append(y, s[i])
			}
		}
	}
	coeff, ok := polyfit5(x, y)
	if !ok {
		return nil
	}
	sbase := make([]float64, ft8NBins+1)
	for i := ia; i <= ib; i++ {
		t := float64(i - i0)
		sbase[i] = coeff[0] + t*(coeff[1]+t*(coeff[2]+t*(coeff[3]+t*coeff[4]))) + 0.65
	}
	return sbase
}

func percentileValue(values []float64, pct int) float64 {
	if len(values) == 0 {
		return 1
	}
	tmp := append([]float64(nil), values...)
	sort.Float64s(tmp)
	idx := nint(float64(len(tmp)) * 0.01 * float64(pct))
	if idx < 1 {
		idx = 1
	}
	if idx > len(tmp) {
		idx = len(tmp)
	}
	return tmp[idx-1]
}

func polyfit5(x []float64, y []float64) ([5]float64, bool) {
	var coeff [5]float64
	if len(x) < len(coeff) || len(x) != len(y) {
		return coeff, false
	}
	var sumx [9]float64
	var sumy [5]float64
	for i, xi := range x {
		xterm := 1.0
		for n := range sumx {
			sumx[n] += xterm
			xterm *= xi
		}
		yterm := y[i]
		for n := range sumy {
			sumy[n] += yterm
			yterm *= xi
		}
	}
	var a [5][6]float64
	for row := 0; row < 5; row++ {
		for col := 0; col < 5; col++ {
			a[row][col] = sumx[row+col]
		}
		a[row][5] = sumy[row]
	}
	for col := 0; col < 5; col++ {
		pivot := col
		for row := col + 1; row < 5; row++ {
			if math.Abs(a[row][col]) > math.Abs(a[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(a[pivot][col]) < 1e-18 {
			return coeff, false
		}
		if pivot != col {
			a[col], a[pivot] = a[pivot], a[col]
		}
		div := a[col][col]
		for k := col; k <= 5; k++ {
			a[col][k] /= div
		}
		for row := 0; row < 5; row++ {
			if row == col {
				continue
			}
			factor := a[row][col]
			for k := col; k <= 5; k++ {
				a[row][k] -= factor * a[col][k]
			}
		}
	}
	for i := range coeff {
		coeff[i] = a[i][5]
	}
	return coeff, true
}

func baselineNoise(sbase []float64, bin int) float64 {
	if bin < 1 || bin >= len(sbase) || sbase[bin] == 0 {
		return 0
	}
	return math.Pow(10, 0.1*(sbase[bin]-40.0))
}

func baselineNoiseAtFreq(sbase []float64, freqHz float64) float64 {
	df := float64(wantSampleRate) / float64(ft8NFFT1)
	return baselineNoise(sbase, nint(freqHz/df))
}
