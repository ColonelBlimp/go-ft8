// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"errors"
	"fmt"
)

// ErrUnsupportedStandardMessage is returned when a message cannot be encoded
// by EncodeStandardMessage.
var ErrUnsupportedStandardMessage = errors.New("ft8: unsupported standard message")

// EncodedMessage is the protocol-level representation of one FT8 message.
//
// It deliberately stops at FT8 symbols. Audio generation, transmit scheduling,
// PTT, and device I/O belong outside this package.
type EncodedMessage struct {
	// Text is the canonical decoded form of the encoded message.
	Text string
	// Bits77 is the packed FT8 payload before CRC and LDPC parity.
	Bits77 [77]byte
	// Codeword is the 174-bit LDPC codeword after CRC and parity.
	Codeword [174]byte
	// Tones is the 79-symbol FT8 tone sequence, with each tone in [0, 7].
	Tones [79]uint8
}

// EncodeStandardMessage encodes a supported standard FT8 message.
//
// This first encoder surface intentionally covers the standard structured
// messages supported by this package's packer, including the standard /P
// variant and ARRL Field Day exchange messages. Free text, telemetry, compound
// calls, and other specialized FT8 message types are not accepted here.
func EncodeStandardMessage(text string) (EncodedMessage, error) {
	bits77, ok := pack77StandardMessage(text)
	if !ok {
		return EncodedMessage{}, fmt.Errorf("%w: %q", ErrUnsupportedStandardMessage, text)
	}
	codeword := encode17491(bits77)
	canonical, ok := unpack77FromCodewordWithHashes(codeword, nil)
	if !ok {
		return EncodedMessage{}, fmt.Errorf("ft8: encoded standard message did not round trip: %q", text)
	}
	tones := tonesFromCodeword(codeword)

	return EncodedMessage{
		Text:     canonical,
		Bits77:   bits77Bytes(bits77),
		Codeword: codewordBytes(codeword),
		Tones:    toneBytes(tones),
	}, nil
}

func bits77Bytes(bits [77]int8) [77]byte {
	var out [77]byte
	for i, bit := range bits {
		out[i] = byte(bit & 1)
	}
	return out
}

func codewordBytes(bits [174]int8) [174]byte {
	var out [174]byte
	for i, bit := range bits {
		out[i] = byte(bit & 1)
	}
	return out
}

func toneBytes(tones [ft8Symbols]int) [79]uint8 {
	var out [79]uint8
	for i, tone := range tones {
		out[i] = uint8(tone)
	}
	return out
}
