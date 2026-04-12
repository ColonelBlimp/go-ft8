// ap.go — A-priori (AP) decoding support for the research package.
//
// Port of the AP injection block in ft8b.f90 lines 300–401 (ncontest=0 only).
// Ported directly from the Fortran source — this file has zero production
// ft8x dependencies.

package research

import "strings"

// ── AP message constants (±1 form) ──────────────────────────────────────
//
// Port of ft8b.f90 lines 39–46 after the 2*x-1 conversion (lines 52–58).
//
// mcq encodes "CQ" in the c28 field (29 bits, ±1 form):
//
//	data mcq/0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0/
//	mcq = 2*mcq - 1
var mcq = [29]int{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, +1, -1, -1,
}

// mrrr encodes "RRR" in the r19 field (19 bits, ±1 form):
//
//	data mrrr/0,1,1,1,1,1,1,0,1,0,0,1,0,0,1,0,0,0,1/
//	mrrr = 2*mrrr - 1
var mrrr = [19]int{
	-1, +1, +1, +1, +1, +1, +1, -1, +1, -1, -1, +1, -1, -1, +1, -1, -1, -1, +1,
}

// m73 encodes "73" in the r19 field (19 bits, ±1 form):
//
//	data m73/0,1,1,1,1,1,1,0,1,0,0,1,0,1,0,0,0,0,1/
//	m73 = 2*m73 - 1
var m73 = [19]int{
	-1, +1, +1, +1, +1, +1, +1, -1, +1, -1, -1, +1, -1, +1, -1, -1, -1, -1, +1,
}

// mrr73 encodes "RR73" in the r19 field (19 bits, ±1 form):
//
//	data mrr73/0,1,1,1,1,1,1,0,0,1,1,1,0,1,0,1,0,0,1/
//	mrr73 = 2*mrr73 - 1
var mrr73 = [19]int{
	-1, +1, +1, +1, +1, +1, +1, -1, -1, +1, +1, +1, -1, +1, -1, +1, -1, -1, +1,
}

// ComputeAPSymbols computes the 58-element a-priori symbol array from
// the operator's callsign (mycall) and the DX callsign (hiscall).
//
// Port of subroutine ft8apset from wsjt-wsjtx/lib/ft8/ft8apset.f90.
//
// For a type-1 message, the first 58 bits of c77 are:
//
//	n28a (28 bits) | ipa=0 (1 bit) | n28b (28 bits) | ipb=0 (1 bit)
//
// The returned apsym values are in ±1 form. Sentinel value 99 indicates
// unknown: apsym[0]==99 means mycall is invalid, apsym[29]==99 means
// hiscall is unknown.
func ComputeAPSymbols(mycall, hiscall string) [58]int {
	var apsym [58]int

	// Sentinel defaults (ft8apset.f90 lines 11-13)
	apsym[0] = 99
	apsym[29] = 99

	mc := strings.TrimSpace(strings.ToUpper(mycall))
	if len(mc) < 3 {
		return apsym
	}

	// Pack mycall
	n28a := pack28(mc)

	// Write n28a as 28 bits (MSB first) into apsym[0:27], ipa=0 into apsym[28]
	for i := 0; i < 28; i++ {
		bit := (n28a >> uint(27-i)) & 1
		apsym[i] = 2*bit - 1
	}
	apsym[28] = -1 // ipa=0 → 2*0-1 = -1

	// Pack hiscall if provided
	hc := strings.TrimSpace(strings.ToUpper(hiscall))
	if len(hc) < 3 {
		apsym[29] = 99 // sentinel: unknown dxcall
		return apsym
	}

	n28b := pack28(hc)
	for i := 0; i < 28; i++ {
		bit := (n28b >> uint(27-i)) & 1
		apsym[29+i] = 2*bit - 1
	}
	apsym[57] = -1 // ipb=0 → 2*0-1 = -1

	return apsym
}

// ApplyAP injects a-priori information into the LLR and mask arrays.
//
// Port of ft8b.f90 lines 300–401 for ncontest=0 (standard QSO) only.
//
// Parameters:
//   - llrz: soft symbols (174 LLRs) — modified in place
//   - apmask: AP mask (174 bits) — modified in place; 1 = position has AP info
//   - iaptype: AP type (1–6), see table below
//   - apsym: AP symbols (±1 form); apsym[0:28] = mycall c28, apsym[29:57] = dxcall c28
//   - apmag: magnitude of AP LLRs
//
// AP types (ft8b.f90 lines 67–74):
//
//	1  CQ     ???    ???     (29+3=32 ap bits)
//	2  MyCall ???    ???     (29+3=32 ap bits)
//	3  MyCall DxCall ???     (58+3=61 ap bits)
//	4  MyCall DxCall RRR     (77 ap bits)
//	5  MyCall DxCall 73      (77 ap bits)
//	6  MyCall DxCall RR73    (77 ap bits)
func ApplyAP(llrz *[LDPCn]float64, apmask *[LDPCn]int8, iaptype int, apsym [58]int, apmag float64) {
	// ft8b.f90 line 270–271: apmask=0; iaptype=0  (caller zeroes for non-AP passes)
	// Here we always zero apmask first, then fill per iaptype.
	for i := range apmask {
		apmask[i] = 0
	}

	// ft8b.f90 lines 300–314: iaptype=1 — CQ
	// ncontest=0: llrz(1:29)=apmag*mcq(1:29)
	if iaptype == 1 {
		// Fortran: apmask(1:29)=1; llrz(1:29)=apmag*mcq(1:29)
		for i := 0; i < 29; i++ {
			apmask[i] = 1
			llrz[i] = apmag * float64(mcq[i])
		}
		// Fortran: apmask(75:77)=1; llrz(75:76)=apmag*(-1); llrz(77)=apmag*(+1)
		apmask[74] = 1
		apmask[75] = 1
		apmask[76] = 1
		llrz[74] = apmag * (-1)
		llrz[75] = apmag * (-1)
		llrz[76] = apmag * (+1)
	}

	// ft8b.f90 lines 316–323: iaptype=2 — MyCall,???,???
	// ncontest=0: apmask(1:29)=1; llrz(1:29)=apmag*apsym(1:29)
	if iaptype == 2 {
		for i := 0; i < 29; i++ {
			apmask[i] = 1
			llrz[i] = apmag * float64(apsym[i])
		}
		apmask[74] = 1
		apmask[75] = 1
		apmask[76] = 1
		llrz[74] = apmag * (-1)
		llrz[75] = apmag * (-1)
		llrz[76] = apmag * (+1)
	}

	// ft8b.f90 lines 356–363: iaptype=3 — MyCall,DxCall,???
	// ncontest=0: apmask(1:58)=1; llrz(1:58)=apmag*apsym
	if iaptype == 3 {
		for i := 0; i < 58; i++ {
			apmask[i] = 1
			llrz[i] = apmag * float64(apsym[i])
		}
		apmask[74] = 1
		apmask[75] = 1
		apmask[76] = 1
		llrz[74] = apmag * (-1)
		llrz[75] = apmag * (-1)
		llrz[76] = apmag * (+1)
	}

	// ft8b.f90 lines 382–389: iaptype=4,5,6 — MyCall,DxCall,RRR|73|RR73
	// ncontest=0 (ncontest.le.5): apmask(1:77)=1
	if iaptype >= 4 && iaptype <= 6 {
		for i := 0; i < 77; i++ {
			apmask[i] = 1
		}
		// Fortran: llrz(1:58)=apmag*apsym
		for i := 0; i < 58; i++ {
			llrz[i] = apmag * float64(apsym[i])
		}
		// Fortran: llrz(59:77)=apmag*mrrr|m73|mrr73
		var msg *[19]int
		switch iaptype {
		case 4:
			msg = &mrrr
		case 5:
			msg = &m73
		case 6:
			msg = &mrr73
		}
		for i := 0; i < 19; i++ {
			llrz[58+i] = apmag * float64(msg[i])
		}
	}
}
