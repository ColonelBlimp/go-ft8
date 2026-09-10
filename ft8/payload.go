// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "fmt"

const payload77Bits = 77

// Payload77 is a packed FT8 message payload before CRC and LDPC parity.
//
// Bits are stored most-significant bit first: payload bit 0 is bit 7 of byte
// 0, and payload bit 76 is bit 3 of byte 9. The low three bits of byte 9 are
// always zero.
type Payload77 [10]byte

// MessageType identifies a 77-bit message family. N3 is meaningful only when
// I3 is zero; it is zero for all other I3 values.
type MessageType struct {
	I3 uint8
	N3 uint8
}

// String returns the conventional message-family label, such as "0.1" or
// "5".
func (t MessageType) String() string {
	if t.I3 == 0 {
		return fmt.Sprintf("0.%d", t.N3)
	}
	return fmt.Sprintf("%d", t.I3)
}

// ParseStatus describes whether a verified payload could be interpreted as a
// supported message family.
type ParseStatus uint8

const (
	// ParseStatusUnset is the zero value and does not describe a parse attempt.
	ParseStatusUnset ParseStatus = iota
	// ParseStatusParsed means the payload was converted to canonical message text.
	ParseStatusParsed
	// ParseStatusUnsupported means the family is assigned but not implemented.
	ParseStatusUnsupported
	// ParseStatusUnknownType means the family selector is reserved or unknown.
	ParseStatusUnknownType
	// ParseStatusInvalid means a known family contained an invalid field value or
	// field combination.
	ParseStatusInvalid
)

// String returns a stable diagnostic label for the parse status.
func (s ParseStatus) String() string {
	switch s {
	case ParseStatusParsed:
		return "parsed"
	case ParseStatusUnsupported:
		return "unsupported"
	case ParseStatusUnknownType:
		return "unknown_type"
	case ParseStatusInvalid:
		return "invalid"
	default:
		return "unset"
	}
}

// MessageType returns the i3/n3 family selector carried by the payload.
func (p Payload77) MessageType() MessageType {
	i3 := p.readBits(74, 3)
	t := MessageType{I3: uint8(i3)}
	if i3 == 0 {
		t.N3 = uint8(p.readBits(71, 3))
	}
	return t
}

// Bits returns the payload as one byte per bit, with every element equal to
// zero or one.
func (p Payload77) Bits() [payload77Bits]byte {
	var out [payload77Bits]byte
	for i := range out {
		out[i] = p.bit(i)
	}
	return out
}

// Payload77FromBits packs one byte per bit into a Payload77. Only the low bit
// of each input element is used.
//
// FT4 and FT8 share the same 77-bit source-message formats, so this helper can
// also be used by other FTx mode packages after mode-specific descrambling.
func Payload77FromBits(bits [payload77Bits]byte) Payload77 {
	var out Payload77
	for i, bit := range bits {
		if bit&1 != 0 {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}
	return out
}

// ParsePayload77 renders a verified 77-bit payload without decoder hash
// history. Standard callsigns render normally; unresolved hashed callsigns are
// represented by the package's usual angle-bracket placeholder.
func ParsePayload77(payload Payload77) (string, ParseStatus) {
	parsed := parsePayload77WithHashes(payload, nil)
	return parsed.Text, parsed.Status
}

func payload77FromBits(bits []int8) Payload77 {
	var out Payload77
	for i := 0; i < payload77Bits && i < len(bits); i++ {
		if bits[i]&1 != 0 {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}
	return out
}

func (p Payload77) int8Bits() [payload77Bits]int8 {
	var out [payload77Bits]int8
	for i := range out {
		out[i] = int8(p.bit(i))
	}
	return out
}

func (p Payload77) bit(index int) byte {
	if index < 0 || index >= payload77Bits {
		return 0
	}
	return p[index/8] >> uint(7-index%8) & 1
}

func (p Payload77) readBits(start, width int) int {
	value := 0
	for i := 0; i < width; i++ {
		value = value<<1 | int(p.bit(start+i))
	}
	return value
}
