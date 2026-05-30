// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

//go:build pocketfft

package ft8

import "github.com/ColonelBlimp/go-ft8/internal/pfft"

type realFFTPlan struct {
	plan *pfft.RealPlan
	n    int
}

func newRealFFTPlan(n int) *realFFTPlan {
	plan, err := pfft.NewRealPlan(n)
	if err != nil {
		panic(err)
	}
	return &realFFTPlan{plan: plan, n: n}
}

func (p *realFFTPlan) Coefficients(dst []complex128, seq []float64) []complex128 {
	return p.coefficientsRange(dst, seq, 0, p.n/2)
}

func (p *realFFTPlan) CoefficientsRange(dst []complex128, seq []float64, firstBin, lastBin int) []complex128 {
	return p.coefficientsRange(dst, seq, firstBin, lastBin)
}

func (p *realFFTPlan) coefficientsRange(dst []complex128, seq []float64, firstBin, lastBin int) []complex128 {
	if len(seq) != p.n {
		panic("real fft: source length mismatch")
	}
	want := p.n/2 + 1
	if dst == nil {
		dst = make([]complex128, want)
	} else if len(dst) != want {
		panic("real fft: destination length mismatch")
	}
	if err := p.plan.Forward(seq); err != nil {
		panic(err)
	}
	if firstBin < 0 {
		firstBin = 0
	}
	if lastBin >= len(dst) {
		lastBin = len(dst) - 1
	}
	for k := firstBin; k <= lastBin; k++ {
		dst[k] = p.plan.Bin(seq, k)
	}
	return dst
}

type complexFFTPlan struct {
	plan *pfft.ComplexPlan
	n    int
}

func newComplexFFTPlan(n int) *complexFFTPlan {
	plan, err := pfft.NewComplexPlan(n)
	if err != nil {
		panic(err)
	}
	return &complexFFTPlan{plan: plan, n: n}
}

func (p *complexFFTPlan) Coefficients(dst, seq []complex128) []complex128 {
	dst = p.prepare(dst, seq)
	if err := p.plan.Forward(dst); err != nil {
		panic(err)
	}
	return dst
}

func (p *complexFFTPlan) Sequence(dst, coeff []complex128) []complex128 {
	dst = p.prepare(dst, coeff)
	if err := p.plan.Backward(dst); err != nil {
		panic(err)
	}
	return dst
}

func (p *complexFFTPlan) prepare(dst, src []complex128) []complex128 {
	if len(src) != p.n {
		panic("complex fft: source length mismatch")
	}
	if dst == nil {
		dst = make([]complex128, p.n)
	} else if len(dst) != p.n {
		panic("complex fft: destination length mismatch")
	}
	if &dst[0] != &src[0] {
		copy(dst, src)
	}
	return dst
}
