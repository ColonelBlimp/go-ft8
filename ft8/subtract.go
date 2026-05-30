// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"sync"

	"gonum.org/v1/gonum/dsp/fourier"
)

const (
	ft8SubtractFFT    = ft8FrameSamples
	ft8SubtractFilter = 4000
)

var (
	subtractOnce           sync.Once
	subtractFFT            = newSubtractFFTPlan(ft8SubtractFFT)
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
				spec: make([]complex128, ft8SubtractFFT),
				cref: make([]complex128, ft8SignalSamples),
				dphi: make([]float64, (ft8Symbols+2)*ft8SamplesPerSymbol),
			}
		},
	}
)

type subtractScratch struct {
	camp     []complex128
	spec     []complex128
	cref     []complex128
	dphi     []float64
	residual []float64
	realFFT  *fourier.FFT
	realSpec []complex128
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

func subtractFT8(dd []float32, tones [ft8Symbols]int, f0 float64, dtSec float64, refineTime bool) {
	subtractOnce.Do(initSubtractFilter)
	scratch := subtractScratchPool.Get().(*subtractScratch)
	defer subtractScratchPool.Put(scratch)

	cref := genFT8WaveComplexInto(tones, f0, scratch.cref, scratch.dphi)
	sqf := func(idt int, apply bool) float64 {
		nstart := nint(dtSec*wantSampleRate) + idt
		camp := scratch.camp[:ft8SubtractFFT]
		clear(camp)
		if refineTime {
			clear(scratch.residual)
		}
		for i := 0; i < ft8SignalSamples; i++ {
			j := nstart + i
			if j >= 0 && j < len(dd) {
				camp[i] = complex(float64(dd[j]), 0) * complex(real(cref[i]), -imag(cref[i]))
			}
		}

		spec := subtractFFT.Coefficients(scratch.spec[:ft8SubtractFFT], camp)
		for i := range spec {
			spec[i] *= subtractFilterSpectrum[i]
		}
		cfilt := subtractFFT.Sequence(spec, spec)
		for i := 0; i <= ft8SubtractFilter/2; i++ {
			cfilt[i] *= complex(subtractEndCorrection[i], 0)
			j := ft8SignalSamples - 1 - i
			if j >= 0 {
				cfilt[j] *= complex(subtractEndCorrection[i], 0)
			}
		}

		for i := 0; i < ft8SignalSamples; i++ {
			j := nstart + i
			if j >= 0 && j < len(dd) {
				z := cfilt[i] * cref[i]
				value := float64(dd[j]) - 2*real(z)
				if apply {
					dd[j] = float32(value)
				}
				if refineTime {
					scratch.residual[i] = value
				}
			}
		}

		if !refineTime {
			return 0
		}
		if scratch.realFFT == nil {
			scratch.realFFT = fourier.NewFFT(ft8SubtractFFT)
			scratch.realSpec = make([]complex128, ft8SubtractFFT/2+1)
		}
		rx := scratch.realFFT.Coefficients(scratch.realSpec, scratch.residual)
		df := float64(wantSampleRate) / float64(ft8SubtractFFT)
		ia := int((f0 - 1.5*6.25) / df)
		ib := int((f0 + 8.5*6.25) / df)
		if ia < 0 {
			ia = 0
		}
		if ib >= len(rx) {
			ib = len(rx) - 1
		}
		var score float64
		for i := ia; i <= ib; i++ {
			score += real(rx[i])*real(rx[i]) + imag(rx[i])*imag(rx[i])
		}
		return score
	}

	idt := 0
	if refineTime {
		if scratch.residual == nil {
			scratch.residual = make([]float64, ft8SubtractFFT)
		}
		sqa := sqf(-90, false)
		sq0 := sqf(0, false)
		sqb := sqf(90, false)
		den := sqb + sqa - 2*sq0
		if den != 0 {
			dx := -(sqb - sqa) / (2 * den)
			if math.Abs(dx) > 1 {
				return
			}
			idt = nint(90 * dx)
		}
	}
	_ = sqf(idt, true)
}

func initSubtractFilter() {
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
	subtractFilterSpectrum = subtractFFT.Coefficients(nil, kernel)
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

func genFT8WaveComplex(tones [ft8Symbols]int, f0 float64) []complex128 {
	return genFT8WaveComplexInto(tones, f0, nil, nil)
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
