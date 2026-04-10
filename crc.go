package ft8x

// CRC-14 implementation.
//
// Generator polynomial 0x6757 in the LFSR representation used by WSJT-X:
//   p = {1,1,0,0,1,1,1,0,1,0,1,0,1,1,1}  (15 coefficients, bit 14 first)
//
// Port of subroutine get_crc14 from wsjt-wsjtx/lib/ft8/get_crc14.f90 and
// subroutine chkcrc14a from wsjt-wsjtx/lib/ft8/chkcrc14a.f90.

// crc14Poly is the 15-bit CRC-14 polynomial as used in the Fortran LFSR:
// each element corresponds to one polynomial coefficient (bit 14 down to bit 0).
var crc14Poly = [15]int8{1, 1, 0, 0, 1, 1, 1, 0, 1, 0, 1, 0, 1, 1, 1}

// CRC14Bits computes the 14-bit CRC of the bit-string mc (values 0/1).
//
// Usage 1 – compute CRC:
//
//	mc[0:len-14] = message bits, mc[len-14:len] = 0  →  returned value = CRC
//
// Usage 2 – verify CRC:
//
//	mc[0:len] = message + received CRC  →  returned value = 0 if OK
//
// This is a direct port of the Fortran LFSR loop in get_crc14.f90.
func CRC14Bits(mc []int8) uint16 {
	n := len(mc)
	if n < 15 {
		return 0
	}

	// Initialise the 15-element shift register with the first 15 message bits.
	var r [15]int8
	copy(r[:], mc[:15])

	for i := 0; i <= n-15; i++ {
		// Load new input bit into r[14] (Fortran r(15)).
		r[14] = mc[i+14]
		// XOR feedback: if r[0] (Fortran r(1)) is 1, XOR the register with the polynomial.
		if r[0] == 1 {
			for k := 0; k < 15; k++ {
				r[k] = (r[k] + crc14Poly[k]) % 2
			}
		}
		// Circular left-shift by 1: r[0]=r[1], ..., r[13]=r[14], r[14]=old r[0].
		tmp := r[0]
		copy(r[:14], r[1:15])
		r[14] = tmp
	}

	// Pack the 14-bit remainder r[0:14] into an integer.
	var crc uint16
	for i := 0; i < 14; i++ {
		crc = (crc << 1) | uint16(r[i]&1)
	}
	return crc
}

// CheckCRC14Codeword verifies the CRC embedded in a (174,91) codeword.
//
// The message layout (as in decode174_91.f90) is:
//
//	m96[0:76]   = codeword[0:76]   (77 message bits)
//	m96[82:95]  = codeword[77:90]  (14 CRC bits)
//
// Returns true if the CRC is consistent.
func CheckCRC14Codeword(cw [LDPCn]int8) bool {
	// Build the 96-bit array: message (77 bits) then zeros (5) then CRC (14).
	var m96 [96]int8
	copy(m96[:77], cw[:77])
	copy(m96[82:96], cw[77:91])
	return CRC14Bits(m96[:]) == 0
}

// ComputeCRC14 computes and returns the 14-bit CRC for a 77-bit message.
// msgBits[0..76] are the message bits (values 0 or 1).
// Returns the CRC as a 14-bit integer (bits 13..0).
func ComputeCRC14(msgBits [77]int8) uint16 {
	// Build the 96-bit input: 77 message bits, 5 zeros, 14 zero placeholders.
	var m96 [96]int8
	copy(m96[:77], msgBits[:])
	// m96[77:82] = 0 (already zero)
	// m96[82:96] = 0 (already zero)
	return CRC14Bits(m96[:])
}
