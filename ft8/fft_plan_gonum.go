// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

//go:build !pocketfft

package ft8

import "gonum.org/v1/gonum/dsp/fourier"

type realFFTPlan struct {
	fft *fourier.FFT
}

func newRealFFTPlan(n int) *realFFTPlan {
	return &realFFTPlan{fft: fourier.NewFFT(n)}
}

func (p *realFFTPlan) Coefficients(dst []complex128, seq []float64) []complex128 {
	return p.fft.Coefficients(dst, seq)
}

func (p *realFFTPlan) CoefficientsRange(dst []complex128, seq []float64, _, _ int) []complex128 {
	return p.fft.Coefficients(dst, seq)
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
