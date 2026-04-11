// fft.go — FFT routing for the research package.
//
// Stubs wrapping the FFT functions needed by the research pipeline.
// Port of four2a.f90 conventions.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// FFT computes the forward complex-to-complex FFT (unnormalized).
// Routes to radix-2, mixed-radix, or Bluestein depending on size.
//
// Port of call four2a(x,n,1,-1,1) from wsjt-wsjtx/lib/four2a.f90.
//
// TODO: port from Fortran
func FFT(x []complex128) []complex128 {
	return ft8x.FFT(x)
}

// IFFT computes the inverse complex-to-complex FFT (normalized by 1/N).
//
// Port of call four2a(x,n,1,+1,1) from wsjt-wsjtx/lib/four2a.f90.
//
// TODO: port from Fortran
func IFFT(x []complex128) []complex128 {
	return ft8x.IFFT(x)
}
