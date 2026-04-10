package ft8x

// Encode174_91 adds a 14-bit CRC to a 77-bit message and returns the
// 174-bit LDPC codeword.
//
// Port of subroutine encode174_91 from wsjt-wsjtx/lib/ft8/encode174_91.f90.
func Encode174_91(message77 [77]int8) [LDPCn]int8 {
	// Compute and append CRC-14.
	crcVal := ComputeCRC14(message77)
	var message91 [LDPCk]int8
	copy(message91[:77], message77[:])
	// Append the 14 CRC bits (MSB first).
	for i := 0; i < 14; i++ {
		message91[77+i] = int8((crcVal >> uint(13-i)) & 1)
	}
	return Encode174_91NoGRC(message91)
}

// GenFT8Tones converts a 77-bit message into the 79 channel tone indices
// (0..7) that are transmitted over the air.
//
// Message structure: S7 D29 S7 D29 S7  (sync=Costas, data=Gray-coded 8-FSK)
//
// Port of subroutine get_ft8_tones_from_77bits / entry point in genft8.f90.
func GenFT8Tones(msgBits [77]int8) [NN]int {
	codeword := Encode174_91(msgBits)
	return TonesToneFromCodeword(codeword)
}

// TonesToneFromCodeword maps a 174-bit LDPC codeword to the 79 on-air tones.
func TonesToneFromCodeword(codeword [LDPCn]int8) [NN]int {
	var itone [NN]int

	// Three sync blocks: positions 0..6, 36..42, 72..78.
	for k := 0; k < 7; k++ {
		itone[k] = Icos7[k]
		itone[36+k] = Icos7[k]
		itone[72+k] = Icos7[k]
	}

	// 58 data symbols: 3 codeword bits per symbol, Gray coded.
	// j indexes the data symbol (0-based); i = 3*j indexes the first codeword bit.
	pos := 7 // channel position (0-indexed), starting after first sync block
	for j := 0; j < ND; j++ {
		i := 3 * j
		indx := int(codeword[i])*4 + int(codeword[i+1])*2 + int(codeword[i+2])
		itone[pos] = GrayMap[indx]
		pos++
		if j == 28 { // after the 29th data symbol, skip the middle sync block
			pos += 7
		}
	}

	return itone
}
