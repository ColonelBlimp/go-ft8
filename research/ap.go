// ap.go — A-priori (AP) decoding support for the research package.
//
// Port of the AP pass logic in ft8b.f90 lines 243–401.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// ApplyAP injects a-priori information into the LLR and mask arrays.
//
// Port of the AP injection block in ft8b.f90 lines 300–401.
//
// TODO: port from Fortran
func ApplyAP(llrz *[LDPCn]float64, apmask *[LDPCn]int8, iaptype int, apsym [58]int, apmag float64) {
	ft8x.ApplyAP(llrz, apmask, iaptype, apsym, apmag)
}
