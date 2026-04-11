// message.go — Message packing/unpacking for the research package.
//
// Port of packjt77.f90 from wsjt-wsjtx/lib/packjt77.f90.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// Unpack77 decodes a 77-character binary string into a human-readable message.
//
// Port of subroutine unpack77 from wsjt-wsjtx/lib/packjt77.f90.
//
// TODO: port from Fortran
func Unpack77(c77 string) (string, bool) {
	return ft8x.Unpack77(c77)
}

// BitsToC77 converts 77 int8 bits to a string of '0'/'1' characters.
//
// TODO: port from Fortran
func BitsToC77(bits [77]int8) string {
	return ft8x.BitsToC77(bits)
}
