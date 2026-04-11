// sync_d.go — Fine time/frequency sync for the research package.
//
// Port of subroutine sync8d from wsjt-wsjtx/lib/ft8/sync8d.f90.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// Sync8d computes the Costas-array sync power for a complex downsampled
// FT8 signal cd0 starting at sample offset i0.
//
// Port of subroutine sync8d from wsjt-wsjtx/lib/ft8/sync8d.f90.
//
// TODO: port from Fortran
func Sync8d(cd0 []complex128, i0 int, ctwk [32]complex128, itwk int) float64 {
	return ft8x.Sync8d(cd0, i0, ctwk, itwk)
}

// HardSync counts how many of the 21 Costas-array positions are correctly
// identified by taking the argmax of the magnitude spectrum.
//
// Port of the nsync computation in ft8b.f90 lines 163–176.
//
// TODO: port from Fortran
func HardSync(s8 *[8][NN]float64) int {
	return ft8x.HardSync(s8)
}
