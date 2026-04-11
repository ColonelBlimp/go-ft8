// ldpc_parity.go — LDPC parity-check matrix data for the research package.
//
// Port of wsjt-wsjtx/lib/ft8/ldpc_174_91_c_parity.f90.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// LDPCMn[i][j] gives the j-th check node that variable node i belongs to (1-indexed).
var LDPCMn = ft8x.LDPCMn

// LDPCNm[i][j] gives the j-th variable node in check i (1-indexed, 0 = unused).
var LDPCNm = ft8x.LDPCNm

// LDPCNrw[i] is the number of variable nodes in check i (either 6 or 7).
var LDPCNrw = ft8x.LDPCNrw
