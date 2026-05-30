// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"sync"
)

func ft8Spectra(dd []float32) [][]float64 {
	scratch := newFT8SpectraScratch()
	return scratch.compute(dd)
}

var ft8SpectraScratchPool = sync.Pool{
	New: func() any {
		return newFT8SpectraScratch()
	},
}

type ft8SpectraScratch struct {
	fft          *realFFTPlan
	in           []float64
	coeff        []complex128
	spec         [][]float64
	jpeak        []int
	jpeakWide    []int
	red          []float64
	redWide      []float64
	redOrder     []int
	redWideOrder []int
	toneSum      []float64
	pre          []candidate
	candOrder    []int
}

func newFT8SpectraScratch() *ft8SpectraScratch {
	stride := ft8NSymbolSpectra + 1
	specData := make([]float64, (ft8NBins+1)*stride)
	spec := make([][]float64, ft8NBins+1)
	for i := range spec {
		spec[i] = specData[i*stride : (i+1)*stride]
	}
	return &ft8SpectraScratch{
		fft:   newRealFFTPlan(ft8NFFT1),
		in:    make([]float64, ft8NFFT1),
		coeff: make([]complex128, ft8NFFT1/2+1),
		spec:  spec,
	}
}

func (s *ft8SpectraScratch) compute(dd []float32) [][]float64 {
	return s.computeRange(dd, 1, ft8NBins)
}

func (s *ft8SpectraScratch) computeRange(dd []float32, firstBin, lastBin int) [][]float64 {
	if firstBin < 1 {
		firstBin = 1
	}
	if lastBin > ft8NBins {
		lastBin = ft8NBins
	}
	scale := 1.0 / 300.0

	for j := 1; j <= ft8NSymbolSpectra; j++ {
		start := (j - 1) * ft8Step
		clear(s.in)
		for i := 0; i < ft8SamplesPerSymbol; i++ {
			idx := start + i
			if idx < len(dd) {
				s.in[i] = float64(dd[idx]) * scale
			}
		}
		coeff := s.fft.workCoefficientsRange(s.coeff, s.in, firstBin, lastBin)
		for i := firstBin; i <= lastBin && i < len(coeff); i++ {
			s.spec[i][j] = real(coeff[i])*real(coeff[i]) + imag(coeff[i])*imag(coeff[i])
		}
	}
	return s.spec
}

func nint(x float64) int {
	if x >= 0 {
		return int(math.Floor(x + 0.5))
	}
	return int(math.Ceil(x - 0.5))
}
