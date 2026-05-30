// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

//go:build pocketfft

// Package pfft wraps the vendored PocketFFT C library for optional
// CGO-backed FFT experiments.
//
// Plans are constructed once via NewRealPlan / NewComplexPlan and
// reused across many same-size transforms. PocketFFT documents plans
// as thread-safe for concurrent invocation (read-only after creation;
// scratch is allocated per call). Plans hold C-allocated memory and
// MUST be released via Close — a runtime finalizer is set as a
// safety net but explicit Close is the documented pattern.
//
// Provenance: pocketfft/ contains a verbatim copy of upstream
// commit 81d171a6 (2019-05-10) from
// https://gitlab.mpcdf.mpg.de/mtr/pocketfft. BSD 3-Clause licence.
// See pocketfft/VENDORED.md.
package pfft

/*
#cgo CFLAGS: -I${SRCDIR}/pocketfft -O3 -std=c99
#cgo LDFLAGS: -lm
#include "pocketfft.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// RealPlan is a forward/inverse plan for real-input FFTs of length N.
// Output of Forward is in FFTPack packed real-FFT layout; use Bin to
// extract individual frequency-domain values.
type RealPlan struct {
	plan C.rfft_plan
	n    int
}

// NewRealPlan builds a PocketFFT plan for in-place real-to-half-complex
// transforms of length n. Returns an error if n < 1 or if PocketFFT's
// plan allocation fails (typically because n contains an unsupported
// large prime factor — though Bluestein's algorithm in PocketFFT
// handles arbitrary sizes, so this should not happen in practice).
func NewRealPlan(n int) (*RealPlan, error) {
	if n < 1 {
		return nil, fmt.Errorf("pfft: plan length must be positive, got %d", n)
	}
	plan := C.make_rfft_plan(C.size_t(n))
	if plan == nil {
		return nil, fmt.Errorf("pfft: make_rfft_plan(%d) returned nil", n)
	}
	p := &RealPlan{plan: plan, n: n}
	runtime.SetFinalizer(p, (*RealPlan).Close)
	return p, nil
}

// Close releases the underlying PocketFFT plan. Safe to call multiple
// times. After Close, Forward and Backward will panic.
func (p *RealPlan) Close() {
	if p.plan != nil {
		C.destroy_rfft_plan(p.plan)
		p.plan = nil
	}
}

// Length returns the plan's configured FFT length (real-sample count).
func (p *RealPlan) Length() int { return p.n }

// Forward runs an in-place real-to-half-complex FFT. The input buffer
// must have length exactly Length(); on return it holds the FFTPack
// packed real-FFT output. Forward applies no normalisation.
func (p *RealPlan) Forward(x []float64) error {
	if len(x) != p.n {
		return fmt.Errorf("pfft: RealPlan.Forward expected length %d, got %d", p.n, len(x))
	}
	rc := C.rfft_forward(p.plan, (*C.double)(unsafe.Pointer(&x[0])), C.double(1.0))
	runtime.KeepAlive(x)
	if rc != 0 {
		return fmt.Errorf("pfft: rfft_forward returned %d", int(rc))
	}
	return nil
}

// Backward runs an in-place half-complex-to-real inverse FFT. The
// input buffer must be in FFTPack packed real-FFT layout (Forward's
// output format) with length exactly Length(). Backward applies no
// normalisation — divide the output by Length() if a strict
// round-trip is required.
func (p *RealPlan) Backward(x []float64) error {
	if len(x) != p.n {
		return fmt.Errorf("pfft: RealPlan.Backward expected length %d, got %d", p.n, len(x))
	}
	rc := C.rfft_backward(p.plan, (*C.double)(unsafe.Pointer(&x[0])), C.double(1.0))
	runtime.KeepAlive(x)
	if rc != 0 {
		return fmt.Errorf("pfft: rfft_backward returned %d", int(rc))
	}
	return nil
}

// Bin returns the complex value of frequency bin k from a buffer in
// FFTPack packed real-FFT layout. Valid k range is [0, Length()/2]:
// bin 0 is DC, bin Length()/2 is Nyquist (when Length() is even).
// Bin values for k > Length()/2 are conjugate-symmetric (X[N-k] =
// conj(X[k])) and not stored.
//
// FFTPack layout for length n:
//   - out[0] = real(X[0]) (DC)
//   - For k in [1, (n-1)/2]: out[2k-1] = real(X[k]), out[2k] = imag(X[k])
//   - If n is even: out[n-1] = real(X[n/2]) (Nyquist)
func (p *RealPlan) Bin(out []float64, k int) complex128 {
	if k == 0 {
		return complex(out[0], 0)
	}
	if p.n%2 == 0 && k == p.n/2 {
		return complex(out[p.n-1], 0)
	}
	return complex(out[2*k-1], out[2*k])
}

// ComplexPlan is a forward/inverse plan for complex-input FFTs of
// length N. Input and output share the same standard interleaved
// (real, imag) layout — which exactly matches Go's complex128
// memory representation, so we pass []complex128 slices directly to
// the C side with no marshalling.
type ComplexPlan struct {
	plan C.cfft_plan
	n    int
}

// NewComplexPlan builds a PocketFFT plan for in-place complex-to-
// complex transforms of length n.
func NewComplexPlan(n int) (*ComplexPlan, error) {
	if n < 1 {
		return nil, fmt.Errorf("pfft: plan length must be positive, got %d", n)
	}
	plan := C.make_cfft_plan(C.size_t(n))
	if plan == nil {
		return nil, fmt.Errorf("pfft: make_cfft_plan(%d) returned nil", n)
	}
	p := &ComplexPlan{plan: plan, n: n}
	runtime.SetFinalizer(p, (*ComplexPlan).Close)
	return p, nil
}

// Close releases the underlying PocketFFT plan. Safe to call multiple
// times. After Close, Forward and Backward will panic.
func (p *ComplexPlan) Close() {
	if p.plan != nil {
		C.destroy_cfft_plan(p.plan)
		p.plan = nil
	}
}

// Length returns the plan's configured FFT length (complex-sample count).
func (p *ComplexPlan) Length() int { return p.n }

// Forward runs an in-place complex-to-complex forward FFT. No
// normalisation applied.
func (p *ComplexPlan) Forward(x []complex128) error {
	if len(x) != p.n {
		return fmt.Errorf("pfft: ComplexPlan.Forward expected length %d, got %d", p.n, len(x))
	}
	rc := C.cfft_forward(p.plan, (*C.double)(unsafe.Pointer(&x[0])), C.double(1.0))
	runtime.KeepAlive(x)
	if rc != 0 {
		return fmt.Errorf("pfft: cfft_forward returned %d", int(rc))
	}
	return nil
}

// Backward runs an in-place complex-to-complex inverse FFT. No
// normalisation applied — divide by Length() for a strict round-trip.
func (p *ComplexPlan) Backward(x []complex128) error {
	if len(x) != p.n {
		return fmt.Errorf("pfft: ComplexPlan.Backward expected length %d, got %d", p.n, len(x))
	}
	rc := C.cfft_backward(p.plan, (*C.double)(unsafe.Pointer(&x[0])), C.double(1.0))
	runtime.KeepAlive(x)
	if rc != 0 {
		return fmt.Errorf("pfft: cfft_backward returned %d", int(rc))
	}
	return nil
}
