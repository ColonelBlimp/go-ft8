// subtract.go — Signal subtraction for the research package.
//
// Port of subroutine subtractft8 from wsjt-wsjtx/lib/ft8/subtractft8.f90.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// SubtractFT8 removes a decoded signal from audio using per-symbol
// amplitude estimation.
//
// Port of subtractft8.f90 (simplified per-symbol path).
//
// TODO: port from Fortran
func SubtractFT8(dd []float32, itone [NN]int, f0, xdt float64) {
	ft8x.SubtractFT8(dd, itone, f0, xdt)
}

// SubtractFT8FFT removes a decoded signal from audio using the FFT-based
// low-pass filter method from WSJT-X.
//
// Port of subroutine subtractft8 from wsjt-wsjtx/lib/ft8/subtractft8.f90.
//
// TODO: port from Fortran
func SubtractFT8FFT(dd []float32, itone [NN]int, f0, xdt float64) {
	ft8x.SubtractFT8FFT(dd, itone, f0, xdt)
}
