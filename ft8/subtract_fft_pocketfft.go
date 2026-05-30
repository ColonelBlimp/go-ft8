// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

//go:build pocketfft

package ft8

import (
	"sync"

	"github.com/ColonelBlimp/go-ft8/internal/pfft"
)

type subtractFFTPlan struct {
	mu   sync.Mutex
	plan *pfft.ComplexPlan
	n    int
}

func newSubtractFFTPlan(n int) *subtractFFTPlan {
	plan, err := pfft.NewComplexPlan(n)
	if err != nil {
		panic(err)
	}
	return &subtractFFTPlan{plan: plan, n: n}
}

func (p *subtractFFTPlan) Coefficients(dst, seq []complex128) []complex128 {
	dst = p.prepare(dst, seq)
	p.mu.Lock()
	err := p.plan.Forward(dst)
	p.mu.Unlock()
	if err != nil {
		panic(err)
	}
	return dst
}

func (p *subtractFFTPlan) Sequence(dst, coeff []complex128) []complex128 {
	dst = p.prepare(dst, coeff)
	p.mu.Lock()
	err := p.plan.Backward(dst)
	p.mu.Unlock()
	if err != nil {
		panic(err)
	}
	return dst
}

func (p *subtractFFTPlan) prepare(dst, src []complex128) []complex128 {
	if len(src) != p.n {
		panic("subtract fft: source length mismatch")
	}
	if dst == nil {
		dst = make([]complex128, p.n)
	} else if len(dst) != p.n {
		panic("subtract fft: destination length mismatch")
	}
	if &dst[0] != &src[0] {
		copy(dst, src)
	}
	return dst
}
