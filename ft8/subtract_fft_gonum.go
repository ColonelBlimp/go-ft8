// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

//go:build !pocketfft

package ft8

import (
	"sync"

	"gonum.org/v1/gonum/dsp/fourier"
)

type subtractFFTPlan struct {
	mu  sync.Mutex
	fft *fourier.CmplxFFT
}

func newSubtractFFTPlan(n int) *subtractFFTPlan {
	return &subtractFFTPlan{fft: fourier.NewCmplxFFT(n)}
}

func (p *subtractFFTPlan) Coefficients(dst, seq []complex128) []complex128 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fft.Coefficients(dst, seq)
}

func (p *subtractFFTPlan) Sequence(dst, coeff []complex128) []complex128 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fft.Sequence(dst, coeff)
}
