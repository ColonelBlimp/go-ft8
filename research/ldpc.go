// ldpc.go — LDPC decoder for the research package.
//
// Port of subroutine decode174_91 from wsjt-wsjtx/lib/ft8/decode174_91.f90
// and subroutine osd174_91 from wsjt-wsjtx/lib/ft8/osd174_91.f90.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// DecodeResult holds the output of Decode174_91.
type DecodeResult = ft8x.DecodeResult

// Decode174_91 is the hybrid BP/OSD decoder for the (174,91) code.
//
// Port of subroutine decode174_91 from wsjt-wsjtx/lib/ft8/decode174_91.f90.
//
// TODO: port from Fortran
func Decode174_91(llr [LDPCn]float64, keff, maxOSD, ndeep int, apmask [LDPCn]int8) (DecodeResult, bool) {
	return ft8x.Decode174_91(llr, keff, maxOSD, ndeep, apmask)
}
