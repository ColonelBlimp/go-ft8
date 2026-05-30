// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

//go:build !pocketfft

package ft8

import "gonum.org/v1/gonum/dsp/fourier"

type realFFTPlan struct {
	fft  *fourier.FFT
	work []complex128
}

func newRealFFTPlan(n int) *realFFTPlan {
	return &realFFTPlan{
		fft:  fourier.NewFFT(n),
		work: make([]complex128, n/2+1),
	}
}

func (p *realFFTPlan) Coefficients(dst []complex128, seq []float64) []complex128 {
	return p.coefficientsRange(dst, seq, 0, len(p.work)-1)
}

func (p *realFFTPlan) CoefficientsRange(dst []complex128, seq []float64, firstBin, lastBin int) []complex128 {
	return p.coefficientsRange(dst, seq, firstBin, lastBin)
}

func (p *realFFTPlan) workCoefficients(_ []complex128, seq []float64) []complex128 {
	return p.fft.Coefficients(p.work, seq)
}

func (p *realFFTPlan) workCoefficientsRange(_ []complex128, seq []float64, _, _ int) []complex128 {
	return p.fft.Coefficients(p.work, seq)
}

func (p *realFFTPlan) coefficientsRange(dst []complex128, seq []float64, firstBin, lastBin int) []complex128 {
	want := len(p.work)
	if dst == nil {
		dst = make([]complex128, want)
	} else if len(dst) != want {
		panic("real fft: destination length mismatch")
	}
	clear(dst)
	p.fft.Coefficients(p.work, seq)
	firstBin, lastBin = clampFFTBinRange(firstBin, lastBin, want)
	for k := firstBin; k <= lastBin; k++ {
		dst[k] = p.work[k]
	}
	return dst
}

type complexFFTPlan struct {
	fft *fourier.CmplxFFT
}

func newComplexFFTPlan(n int) *complexFFTPlan {
	return &complexFFTPlan{fft: fourier.NewCmplxFFT(n)}
}

func (p *complexFFTPlan) Coefficients(dst, seq []complex128) []complex128 {
	return p.fft.Coefficients(dst, seq)
}

func (p *complexFFTPlan) Sequence(dst, coeff []complex128) []complex128 {
	return p.fft.Sequence(dst, coeff)
}
