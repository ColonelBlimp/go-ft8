// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

// Package ftx77 contains the channel-code primitives shared by 77-bit modes.
package ftx77

// Encode17491 appends the FT4/FT8 CRC-14 and LDPC parity to a 77-bit payload.
func Encode17491(message77 [77]int8) [174]int8 {
	var message91 [91]int8
	copy(message91[:77], message77[:])
	crc := crc14ForMessage(message77)
	for i := 0; i < 14; i++ {
		message91[77+i] = int8((crc >> uint(13-i)) & 1)
	}
	return Encode17491NoCRC(message91)
}

// Encode17491NoCRC appends the 83 LDPC parity bits to a systematic 91-bit
// message containing a 77-bit payload followed by its 14-bit CRC.
func Encode17491NoCRC(message [91]int8) [174]int8 {
	var codeword [174]int8
	copy(codeword[:91], message[:])
	for row, hexRow := range ldpcGeneratorHex {
		var sum int8
		for h := 0; h < len(hexRow); h++ {
			v := hexValue(hexRow[h])
			nibbleBits := 4
			if h == len(hexRow)-1 {
				nibbleBits = 3
			}
			for b := 0; b < nibbleBits; b++ {
				col := h*4 + b
				if col >= len(message) {
					break
				}
				if v&(1<<uint(3-b)) != 0 {
					sum ^= message[col] & 1
				}
			}
		}
		codeword[91+row] = sum
	}
	return codeword
}

// CRC14RemainderMessage77 returns the CRC remainder for a 77-bit payload
// zero-extended to the 96-bit WSJT-X CRC working layout.
func CRC14RemainderMessage77(message77 [77]int8) int {
	var state uint16
	for i := 0; i < 15; i++ {
		state = (state << 1) | uint16(message77[i]&1)
	}
	for i := 0; i <= 81; i++ {
		pos := i + 14
		var bit int8
		if pos < 77 {
			bit = message77[pos]
		}
		state = crc14Step(state, bit)
	}
	return int(state >> 1)
}

func crc14ForMessage(message77 [77]int8) int {
	return solveCRC14(CRC14RemainderMessage77(message77))
}

var crc14EncodeRows = initCRC14EncodeRows()

func initCRC14EncodeRows() [14]uint16 {
	var rows [14]uint16
	for col := 0; col < 14; col++ {
		var bits [96]int8
		bits[82+col] = 1
		rem := crc14Remainder(bits[:])
		for row := 0; row < 14; row++ {
			if rem&(1<<uint(13-row)) != 0 {
				rows[row] |= 1 << uint(13-col)
			}
		}
	}
	return rows
}

func solveCRC14(target int) int {
	rows := crc14EncodeRows
	var rhs [14]int
	for row := 0; row < 14; row++ {
		rhs[row] = (target >> uint(13-row)) & 1
	}

	for col := 0; col < 14; col++ {
		mask := uint16(1 << uint(13-col))
		pivot := -1
		for row := col; row < 14; row++ {
			if rows[row]&mask != 0 {
				pivot = row
				break
			}
		}
		if pivot < 0 {
			panic("crc14: singular generator submatrix")
		}
		if pivot != col {
			rows[col], rows[pivot] = rows[pivot], rows[col]
			rhs[col], rhs[pivot] = rhs[pivot], rhs[col]
		}
		for row := 0; row < 14; row++ {
			if row == col || rows[row]&mask == 0 {
				continue
			}
			rows[row] ^= rows[col]
			rhs[row] ^= rhs[col]
		}
	}

	crc := 0
	for row := 0; row < 14; row++ {
		crc |= rhs[row] << uint(13-row)
	}
	return crc
}

func crc14Remainder(bits []int8) int {
	var state uint16
	for i := 0; i < 15 && i < len(bits); i++ {
		state = (state << 1) | uint16(bits[i]&1)
	}
	for i := 0; i <= len(bits)-15; i++ {
		state = crc14Step(state, bits[i+14])
	}
	return int(state >> 1)
}

const crc14PolyState uint16 = 0x6757

func crc14Step(state uint16, bit int8) uint16 {
	state = (state &^ 1) | uint16(bit&1)
	if state&0x4000 != 0 {
		state ^= crc14PolyState
	}
	return (state << 1) & 0x7fff
}

func hexValue(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	default:
		return 0
	}
}

var ldpcGeneratorHex = [...]string{
	"8329ce11bf31eaf509f27fc",
	"761c264e25c259335493132",
	"dc265902fb277c6410a1bdc",
	"1b3f417858cd2dd33ec7f62",
	"09fda4fee04195fd034783a",
	"077cccc11b8873ed5c3d48a",
	"29b62afe3ca036f4fe1a9da",
	"6054faf5f35d96d3b0c8c3e",
	"e20798e4310eed27884ae90",
	"775c9c08e80e26ddae56318",
	"b0b811028c2bf997213487c",
	"18a0c9231fc60adf5c5ea32",
	"76471e8302a0721e01b12b8",
	"ffbccb80ca8341fafb47b2e",
	"66a72a158f9325a2bf67170",
	"c4243689fe85b1c51363a18",
	"0dff739414d1a1b34b1c270",
	"15b48830636c8b99894972e",
	"29a89c0d3de81d665489b0e",
	"4f126f37fa51cbe61bd6b94",
	"99c47239d0d97d3c84e0940",
	"1919b75119765621bb4f1e8",
	"09db12d731faee0b86df6b8",
	"488fc33df43fbdeea4eafb4",
	"827423ee40b675f756eb5fe",
	"abe197c484cb74757144a9a",
	"2b500e4bc0ec5a6d2bdbdd0",
	"c474aa53d70218761669360",
	"8eba1a13db3390bd6718cec",
	"753844673a27782cc42012e",
	"06ff83a145c37035a5c1268",
	"3b37417858cc2dd33ec3f62",
	"9a4a5a28ee17ca9c324842c",
	"bc29f465309c977e89610a4",
	"2663ae6ddf8b5ce2bb29488",
	"46f231efe457034c1814418",
	"3fb2ce85abe9b0c72e06fbe",
	"de87481f282c153971a0a2e",
	"fcd7ccf23c69fa99bba1412",
	"f0261447e9490ca8e474cec",
	"4410115818196f95cdd7012",
	"088fc31df4bfbde2a4eafb4",
	"b8fef1b6307729fb0a078c0",
	"5afea7acccb77bbc9d99a90",
	"49a7016ac653f65ecdc9076",
	"1944d085be4e7da8d6cc7d0",
	"251f62adc4032f0ee714002",
	"56471f8702a0721e00b12b8",
	"2b8e4923f2dd51e2d537fa0",
	"6b550a40a66f4755de95c26",
	"a18ad28d4e27fe92a4f6c84",
	"10c2e586388cb82a3d80758",
	"ef34a41817ee02133db2eb0",
	"7e9c0c54325a9c15836e000",
	"3693e572d1fde4cdf079e86",
	"bfb2cec5abe1b0c72e07fbe",
	"7ee18230c583cccc57d4b08",
	"a066cb2fedafc9f52664126",
	"bb23725abc47cc5f4cc4cd2",
	"ded9dba3bee40c59b5609b4",
	"d9a7016ac653e6decdc9036",
	"9ad46aed5f707f280ab5fc4",
	"e5921c77822587316d7d3c2",
	"4f14da8242a8b86dca73352",
	"8b8b507ad467d4441df770e",
	"22831c9cf1169467ad04b68",
	"213b838fe2ae54c38ee7180",
	"5d926b6dd71f085181a4e12",
	"66ab79d4b29ee6e69509e56",
	"958148682d748a38dd68baa",
	"b8ce020cf069c32a723ab14",
	"f4331d6d461607e95752746",
	"6da23ba424b9596133cf9c8",
	"a636bcbc7b30c5fbeae67fe",
	"5cb0d86a07df654a9089a20",
	"f11f106848780fc9ecdd80a",
	"1fbb5364fb8d2c9d730d5ba",
	"fcb86bc70a50c9d02a5d034",
	"a534433029eac15f322e34c",
	"c989d9c7c3d3b8c55d75130",
	"7bb38b2f0186d46643ae962",
	"2644ebadeb44b9467d1f42c",
	"608cc857594bfbb55d69600",
}
