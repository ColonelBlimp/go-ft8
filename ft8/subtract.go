// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"sync"
)

const (
	ft8SubtractFilter = 4000
	ft8SubtractMinFFT = ft8SignalSamples + ft8SubtractFilter/2 + 1

	// Smooth FFT length above ft8SubtractMinFFT.
	ft8SubtractFFT = 155520

	_ = uint(ft8SubtractFFT - ft8SubtractMinFFT)
)

var (
	subtractOnce           sync.Once
	subtractFilterSpectrum []complex128
	subtractEndCorrection  [ft8SubtractFilter/2 + 1]float64
	subtractPulseOnce      sync.Once
	subtractPulse          [3 * ft8SamplesPerSymbol]float64
	subtractRampOnce       sync.Once
	subtractRampUp         [ft8SamplesPerSymbol / 8]float64
	subtractRampDown       [ft8SamplesPerSymbol / 8]float64
	subtractScratchPool    = sync.Pool{
		New: func() any {
			return &subtractScratch{
				camp: make([]complex128, ft8SubtractFFT),
				cref: make([]complex128, ft8SignalSamples),
				dphi: make([]float64, (ft8Symbols+2)*ft8SamplesPerSymbol),
			}
		},
	}
	subtractFFTPlanPool = sync.Pool{
		New: func() any {
			return newSubtractFFTPlan(ft8SubtractFFT)
		},
	}
)

type subtractScratch struct {
	camp []complex128
	cref []complex128
	dphi []float64
}

func tonesFromCodeword(cw [174]int8) [ft8Symbols]int {
	var tones [ft8Symbols]int
	for i, tone := range ft8Costas {
		tones[i] = tone
		tones[36+i] = tone
		tones[72+i] = tone
	}
	k := 7
	for j := 1; j <= ft8DataSyms; j++ {
		bit := 3*j - 3
		if j == 30 {
			k += 7
		}
		idx := int(cw[bit])*4 + int(cw[bit+1])*2 + int(cw[bit+2])
		tones[k] = ft8GrayMap[idx]
		k++
	}
	return tones
}

func subtractFT8(dd []float32, tones [ft8Symbols]int, f0 float64, dtSec float64) {
	subtractOnce.Do(initSubtractFilter)
	scratch := subtractScratchPool.Get().(*subtractScratch)
	defer subtractScratchPool.Put(scratch)

	cref := genFT8WaveComplexInto(tones, f0, scratch.cref, scratch.dphi)
	nstart := nint(dtSec * wantSampleRate)
	camp := scratch.camp[:ft8SubtractFFT]
	validStart := 0
	if nstart < 0 {
		validStart = -nstart
	}
	validEnd := ft8SignalSamples
	if end := len(dd) - nstart; end < validEnd {
		validEnd = end
	}
	if validStart >= validEnd {
		return
	}
	fft := subtractFFTPlanPool.Get().(*subtractFFTPlan)
	defer subtractFFTPlanPool.Put(fft)

	clear(camp[:validStart])
	clear(camp[validEnd:])
	for i := validStart; i < validEnd; i++ {
		sample := float64(dd[nstart+i])
		ref := cref[i]
		camp[i] = complex(sample*real(ref), -sample*imag(ref))
	}

	// Forward transform in place on camp; camp is not read again after
	// this point, so we transform it directly instead of copying into a
	// separate spectrum buffer (saves a 180000-element complex128
	// memmove and one scratch allocation per subtraction).
	spec := fft.Coefficients(camp, camp)
	for i := range spec {
		spec[i] *= subtractFilterSpectrum[i]
	}
	cfilt := fft.Sequence(spec, spec)
	for i := 0; i <= ft8SubtractFilter/2; i++ {
		cfilt[i] *= complex(subtractEndCorrection[i], 0)
		j := ft8SignalSamples - 1 - i
		if j >= 0 {
			cfilt[j] *= complex(subtractEndCorrection[i], 0)
		}
	}

	for i := validStart; i < validEnd; i++ {
		j := nstart + i
		z := cfilt[i] * cref[i]
		dd[j] = float32(float64(dd[j]) - 2*real(z))
	}
}

func initSubtractFilter() {
	fft := subtractFFTPlanPool.Get().(*subtractFFTPlan)
	defer subtractFFTPlanPool.Put(fft)

	kernel := make([]complex128, ft8SubtractFFT)
	window := make([]float64, ft8SubtractFilter+1)
	var sumw float64
	for i := 0; i <= ft8SubtractFilter; i++ {
		j := i - ft8SubtractFilter/2
		window[i] = math.Pow(math.Cos(math.Pi*float64(j)/ft8SubtractFilter), 2)
		sumw += window[i]
	}
	for i, w := range window {
		kernel[i] = complex(w/sumw, 0)
	}
	kernel = cshift(kernel, ft8SubtractFilter/2+1)
	subtractFilterSpectrum = fft.Coefficients(nil, kernel)
	scale := 1.0 / float64(ft8SubtractFFT)
	for i := range subtractFilterSpectrum {
		subtractFilterSpectrum[i] *= complex(scale, 0)
	}

	for j := 0; j <= ft8SubtractFilter/2; j++ {
		var tail float64
		for i := ft8SubtractFilter/2 + j; i <= ft8SubtractFilter; i++ {
			tail += window[i]
		}
		den := 1 - tail/sumw
		if den == 0 {
			subtractEndCorrection[j] = 1
		} else {
			subtractEndCorrection[j] = 1 / den
		}
	}
}

func genFT8WaveComplexInto(tones [ft8Symbols]int, f0 float64, out []complex128, dphi []float64) []complex128 {
	const (
		nsps = ft8SamplesPerSymbol
	)

	pulse := ft8SubtractPulse()
	if len(dphi) < (ft8Symbols+2)*nsps {
		dphi = make([]float64, (ft8Symbols+2)*nsps)
	} else {
		dphi = dphi[:(ft8Symbols+2)*nsps]
		clear(dphi)
	}

	carrierPhaseStep := 2 * math.Pi * f0 / float64(wantSampleRate)
	for j, tone := range tones {
		base := j * nsps
		toneScale := float64(tone)
		for i := 0; i < 3*nsps; i++ {
			dphi[base+i] += pulse[i] * toneScale
		}
	}
	firstTone := float64(tones[0])
	lastTone := float64(tones[ft8Symbols-1])
	for i := 0; i < 2*nsps; i++ {
		dphi[i] += pulse[nsps+i] * firstTone
		dphi[ft8Symbols*nsps+i] += pulse[i] * lastTone
	}

	if len(out) < ft8SignalSamples {
		out = make([]complex128, ft8SignalSamples)
	} else {
		out = out[:ft8SignalSamples]
	}
	phase := 0.0
	const twoPi = 2 * math.Pi
	for j := nsps; j < nsps+ft8SignalSamples; j++ {
		s, c := math.Sincos(phase)
		out[j-nsps] = complex(c, s)
		phase += carrierPhaseStep + dphi[j]
		if phase >= twoPi {
			phase -= twoPi
		}
	}

	upRamp, downRamp := ft8SubtractRamps()
	for i := range upRamp {
		out[i] *= complex(upRamp[i], 0)
		out[ft8SignalSamples-len(upRamp)+i] *= complex(downRamp[i], 0)
	}
	return out
}

func ft8SubtractPulse() *[3 * ft8SamplesPerSymbol]float64 {
	subtractPulseOnce.Do(func() {
		const bt = 2.0
		dphiPeak := 2 * math.Pi / float64(ft8SamplesPerSymbol)
		for i := 0; i < len(subtractPulse); i++ {
			tt := (float64(i+1) - 1.5*float64(ft8SamplesPerSymbol)) / float64(ft8SamplesPerSymbol)
			subtractPulse[i] = dphiPeak * gfskPulse(bt, tt)
		}
	})
	return &subtractPulse
}

func ft8SubtractRamps() (*[ft8SamplesPerSymbol / 8]float64, *[ft8SamplesPerSymbol / 8]float64) {
	subtractRampOnce.Do(func() {
		ramp := len(subtractRampUp)
		for i := 0; i < ramp; i++ {
			subtractRampUp[i] = (1 - math.Cos(2*math.Pi*float64(i)/(2*float64(ramp)))) / 2
			subtractRampDown[i] = (1 + math.Cos(2*math.Pi*float64(i)/(2*float64(ramp)))) / 2
		}
	})
	return &subtractRampUp, &subtractRampDown
}

func gfskPulse(b float64, t float64) float64 {
	c := math.Pi * math.Sqrt(2/math.Log(2))
	return 0.5 * (math.Erf(c*b*(t+0.5)) - math.Erf(c*b*(t-0.5)))
}
