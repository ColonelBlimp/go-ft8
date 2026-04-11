// downsample.go — Downsampler for the research package.
//
// Port of subroutine ft8_downsample from wsjt-wsjtx/lib/ft8/ft8_downsample.f90
// and subroutine twkfreq1 from wsjt-wsjtx/lib/ft8/twkfreq1.f90.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// Downsampler holds cached FFT state so the expensive 192000-point
// transform is only recomputed when the input audio changes.
//
// Port of the save'd state in ft8_downsample.f90.
//
// TODO: port from Fortran
type Downsampler = ft8x.Downsampler

// NewDownsampler creates a Downsampler and precomputes the edge taper.
//
// TODO: port from Fortran
func NewDownsampler() *Downsampler {
	return ft8x.NewDownsampler()
}

// TwkFreq1 applies a polynomial frequency correction to the complex signal.
//
// Port of subroutine twkfreq1 from wsjt-wsjtx/lib/ft8/twkfreq1.f90.
//
// TODO: port from Fortran
func TwkFreq1(ca []complex128, fsample float64, a [5]float64) []complex128 {
	return ft8x.TwkFreq1(ca, fsample, a)
}
