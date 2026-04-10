package ft8x

// A-priori (AP) decoding support for the FT8 (174,91) LDPC code.
//
// AP decoding injects known bit values into the LLR stream with high
// magnitude to aid decoding of weak signals.  This ports the AP pass
// logic from wsjt-wsjtx/lib/ft8/ft8b.f90 lines 243–401 (ncontest=0,
// standard QSO mode only).
//
// AP types (iaptype):
//   1  CQ     ???    ???          (29 + 3 = 32 known bits)
//   2  MyCall ???    ???          (29 + 3 = 32 known bits)
//   3  MyCall DxCall ???          (58 + 3 = 61 known bits)
//   4  MyCall DxCall RRR          (77 known bits)
//   5  MyCall DxCall 73           (77 known bits)
//   6  MyCall DxCall RR73         (77 known bits)

// AP type constants.
const (
	APTypeCQ       = 1 // CQ ??? ???
	APTypeMyCall   = 2 // MyCall ??? ???
	APTypeMyDxCall = 3 // MyCall DxCall ???
	APTypeMyDxRRR  = 4 // MyCall DxCall RRR
	APTypeMyDx73   = 5 // MyCall DxCall 73
	APTypeMyDxRR73 = 6 // MyCall DxCall RR73
)

// Pre-computed bipolar (±1) bit patterns for CQ and roger/73 messages.
// These match the Fortran data statements in ft8b.f90 lines 39–46,
// converted to bipolar: value = 2*bit − 1.
//
// mcq[0:28] = CQ packed as 29 bits (Pack28("CQ") = 2, i.e. bit 26 set).
var mcqBipolar = [29]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 1, -1, -1}

// mrrrBipolar = RRR packed as 19 bits (i3=0, n3=1, report field).
var mrrrBipolar = [19]int{-1, 1, 1, 1, 1, 1, 1, -1, 1, -1, -1, 1, -1, -1, 1, -1, -1, -1, 1}

// m73Bipolar = 73 packed as 19 bits.
var m73Bipolar = [19]int{-1, 1, 1, 1, 1, 1, 1, -1, 1, -1, -1, 1, -1, 1, -1, -1, -1, -1, 1}

// mrr73Bipolar = RR73 packed as 19 bits.
var mrr73Bipolar = [19]int{-1, 1, 1, 1, 1, 1, 1, -1, -1, 1, 1, 1, -1, 1, -1, 1, -1, -1, 1}

// i3Bipolar = the last 3 message bits for type-1 messages: i3=1 → binary 001 → bipolar [-1, -1, +1].
var i3Bipolar = [3]int{-1, -1, 1}

// ComputeAPSymbols computes the 58-element bipolar array (2*bit−1) for
// a given mycall and dxcall.  apsym[0:28] = mycall, apsym[29:57] = dxcall.
// The /R flag bit (bit 28 and 57) is set to 0 → bipolar −1.
//
// Returns (apsym, true) on success, or ([58]int{}, false) if either
// callsign cannot be packed.
func ComputeAPSymbols(mycall, dxcall string) ([58]int, bool) {
	var apsym [58]int

	if mycall != "" {
		n28 := Pack28(mycall)
		if n28 < 0 {
			return apsym, false
		}
		// Pack the 28-bit value into apsym[0:27] (MSB first), then /R flag = 0 at [28].
		for i := 0; i < 28; i++ {
			bit := (n28 >> uint(27-i)) & 1
			apsym[i] = 2*bit - 1
		}
		apsym[28] = -1 // /R flag = 0
	}

	if dxcall != "" {
		n28 := Pack28(dxcall)
		if n28 < 0 {
			return apsym, false
		}
		for i := 0; i < 28; i++ {
			bit := (n28 >> uint(27-i)) & 1
			apsym[29+i] = 2*bit - 1
		}
		apsym[57] = -1 // /R flag = 0
	}

	return apsym, true
}

// ApplyAP modifies llrz and apmask in-place to inject a-priori information
// for the given iaptype.  apmag is the magnitude used for known bits
// (typically max(|llra|)*1.01).
//
// This ports the ncontest=0 (standard QSO) AP logic from
// wsjt-wsjtx/lib/ft8/ft8b.f90 lines 300–401.
func ApplyAP(llrz *[LDPCn]float64, apmask *[LDPCn]int8, iaptype int, apsym [58]int, apmag float64) {
	// Clear apmask.
	for i := range apmask {
		apmask[i] = 0
	}

	switch iaptype {
	case APTypeCQ:
		// Pin bits 0–28 to CQ pattern.
		for i := 0; i < 29; i++ {
			apmask[i] = 1
			llrz[i] = apmag * float64(mcqBipolar[i])
		}
		// Pin i3 bits (74–76, 0-indexed) to type-1: [-1, -1, +1].
		apmask[74] = 1
		apmask[75] = 1
		apmask[76] = 1
		llrz[74] = apmag * float64(i3Bipolar[0])
		llrz[75] = apmag * float64(i3Bipolar[1])
		llrz[76] = apmag * float64(i3Bipolar[2])

	case APTypeMyCall:
		// Pin bits 0–28 to mycall pattern.
		for i := 0; i < 29; i++ {
			apmask[i] = 1
			llrz[i] = apmag * float64(apsym[i])
		}
		// Pin i3 bits.
		apmask[74] = 1
		apmask[75] = 1
		apmask[76] = 1
		llrz[74] = apmag * float64(i3Bipolar[0])
		llrz[75] = apmag * float64(i3Bipolar[1])
		llrz[76] = apmag * float64(i3Bipolar[2])

	case APTypeMyDxCall:
		// Pin bits 0–57 to mycall + dxcall pattern.
		for i := 0; i < 58; i++ {
			apmask[i] = 1
			llrz[i] = apmag * float64(apsym[i])
		}
		// Pin i3 bits.
		apmask[74] = 1
		apmask[75] = 1
		apmask[76] = 1
		llrz[74] = apmag * float64(i3Bipolar[0])
		llrz[75] = apmag * float64(i3Bipolar[1])
		llrz[76] = apmag * float64(i3Bipolar[2])

	case APTypeMyDxRRR:
		// Pin all 77 bits: mycall + dxcall + RRR.
		for i := 0; i < 77; i++ {
			apmask[i] = 1
		}
		for i := 0; i < 58; i++ {
			llrz[i] = apmag * float64(apsym[i])
		}
		for i := 0; i < 19; i++ {
			llrz[58+i] = apmag * float64(mrrrBipolar[i])
		}

	case APTypeMyDx73:
		// Pin all 77 bits: mycall + dxcall + 73.
		for i := 0; i < 77; i++ {
			apmask[i] = 1
		}
		for i := 0; i < 58; i++ {
			llrz[i] = apmag * float64(apsym[i])
		}
		for i := 0; i < 19; i++ {
			llrz[58+i] = apmag * float64(m73Bipolar[i])
		}

	case APTypeMyDxRR73:
		// Pin all 77 bits: mycall + dxcall + RR73.
		for i := 0; i < 77; i++ {
			apmask[i] = 1
		}
		for i := 0; i < 58; i++ {
			llrz[i] = apmag * float64(apsym[i])
		}
		for i := 0; i < 19; i++ {
			llrz[58+i] = apmag * float64(mrr73Bipolar[i])
		}
	}
}

// APPassTypes returns the list of AP types to try based on available
// callsign information and QSO progress.
//
// Matching the Fortran nappasses/naptypes tables (ft8b.f90 lines 60–81):
//   - CQ only (nQSOProgress=0):       [APTypeCQ, APTypeMyCall]
//   - MyCall only (nQSOProgress=1,2):  [APTypeMyCall, APTypeMyDxCall]
//   - Both calls (nQSOProgress=3,4):   [APTypeMyDxCall, APTypeMyDxRRR, APTypeMyDx73, APTypeMyDxRR73]
//
// If cqOnly is true, only [APTypeCQ] is returned regardless.
func APPassTypes(hasMyCall, hasDxCall, cqOnly bool) []int {
	if cqOnly {
		return []int{APTypeCQ}
	}
	if hasDxCall && hasMyCall {
		return []int{APTypeMyDxCall, APTypeMyDxRRR, APTypeMyDx73, APTypeMyDxRR73}
	}
	if hasMyCall {
		return []int{APTypeCQ, APTypeMyCall}
	}
	return []int{APTypeCQ}
}
