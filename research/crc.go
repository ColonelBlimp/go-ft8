// crc.go — CRC-14 computation for the research package.
//
// Port of subroutine get_crc14 from wsjt-wsjtx/lib/ft8/get_crc14.f90
// and subroutine chkcrc14a from wsjt-wsjtx/lib/ft8/chkcrc14a.f90.
//
// Pure port — no production dependency.

package research

// crc14Poly is the 15-bit CRC-14 polynomial (LFSR representation).
// p = {1,1,0,0,1,1,1,0,1,0,1,0,1,1,1}  (bit 14 first)
var crc14Poly = [15]int8{1, 1, 0, 0, 1, 1, 1, 0, 1, 0, 1, 0, 1, 1, 1}

// crc14Bits computes the 14-bit CRC of the bit-string mc (values 0/1).
//
// Port of subroutine get_crc14 from wsjt-wsjtx/lib/ft8/get_crc14.f90.
func crc14Bits(mc []int8) uint16 {
	n := len(mc)
	if n < 15 {
		return 0
	}

	var r [15]int8
	copy(r[:], mc[:15])

	for i := 0; i <= n-15; i++ {
		r[14] = mc[i+14]
		if r[0] == 1 {
			for k := 0; k < 15; k++ {
				r[k] = (r[k] + crc14Poly[k]) % 2
			}
		}
		tmp := r[0]
		copy(r[:14], r[1:15])
		r[14] = tmp
	}

	var crc uint16
	for i := 0; i < 14; i++ {
		crc = (crc << 1) | uint16(r[i]&1)
	}
	return crc
}

// checkCRC14Codeword verifies the CRC embedded in a (174,91) codeword.
//
// The layout (from decode174_91.f90):
//
//	m96[0:76]  = cw[0:76]   (77 message bits)
//	m96[82:95] = cw[77:90]  (14 CRC bits)
//
// Returns true if CRC is consistent.
func checkCRC14Codeword(cw [LDPCn]int8) bool {
	var m96 [96]int8
	copy(m96[:77], cw[:77])
	copy(m96[82:96], cw[77:91])
	return crc14Bits(m96[:]) == 0
}

// computeCRC14 computes the 14-bit CRC for a 77-bit message.
func computeCRC14(msgBits [77]int8) uint16 {
	var m96 [96]int8
	copy(m96[:77], msgBits[:])
	return crc14Bits(m96[:])
}
