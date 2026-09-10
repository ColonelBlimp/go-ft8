// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

// Package ft4 encodes and decodes FT4 messages.
//
// DecodeMessages accepts one 7.5-second, 12 kHz mono signed-16-bit PCM slot.
// EncodeStandardMessage produces protocol bits and tones; callers own audio
// generation, transmit scheduling, PTT, and radio control. The initial decoder
// provides unassisted BP decoding for standard contest operation. A-priori
// decoding and successive-interference subtraction are intentionally deferred.
package ft4

import (
	"errors"
	"fmt"

	"github.com/ColonelBlimp/go-ft8/ft8"
	"github.com/ColonelBlimp/go-ft8/internal/ftx77"
)

// ErrUnsupportedStandardMessage is returned when a message cannot be encoded
// by EncodeStandardMessage.
var ErrUnsupportedStandardMessage = errors.New("ft4: unsupported standard message")

// EncodedMessage is the protocol-level representation of one FT4 message.
type EncodedMessage struct {
	// Text is the canonical decoded form of the encoded message.
	Text string
	// Bits77 is the source-encoded payload before FT4 scrambling, CRC, and LDPC.
	Bits77 [77]byte
	// ScrambledBits77 is Bits77 XORed with the FT4 scrambling vector. These are
	// the first 77 systematic bits of Codeword.
	ScrambledBits77 [77]byte
	// Codeword is the scrambled payload, CRC, and parity as a 174-bit LDPC word.
	Codeword [174]byte
	// Tones contains the 103 sync and data symbols, each in [0,3]. Waveform
	// ramp-up and ramp-down intervals are implicit and are not included.
	Tones [ToneSymbols]uint8
}

// EncodeStandardMessage encodes a supported standard message as an FT4 frame.
//
// FT4 and FT8 share the 77-bit source-message formats, so this surface has the
// same current message coverage as ft8.EncodeStandardMessage: standard /P and
// /R forms, ARRL Field Day, and type-4 compound or nonstandard calls. Free
// text, telemetry, and specialized contest encoders are not yet accepted.
func EncodeStandardMessage(text string) (EncodedMessage, error) {
	packed, err := ft8.EncodeStandardMessage(text)
	if err != nil {
		if errors.Is(err, ft8.ErrUnsupportedStandardMessage) {
			return EncodedMessage{}, fmt.Errorf("%w: %q", ErrUnsupportedStandardMessage, text)
		}
		return EncodedMessage{}, fmt.Errorf("ft4: source encode failed: %w", err)
	}

	var scrambled [77]int8
	var scrambledBytes [77]byte
	for i, bit := range packed.Bits77 {
		scrambled[i] = int8(bit) ^ scramblingVector[i]
		scrambledBytes[i] = byte(scrambled[i])
	}
	codeword := ftx77.Encode17491(scrambled)

	return EncodedMessage{
		Text:            packed.Text,
		Bits77:          packed.Bits77,
		ScrambledBits77: scrambledBytes,
		Codeword:        codewordBytes(codeword),
		Tones:           tonesFromCodeword(codeword),
	}, nil
}

func codewordBytes(bits [174]int8) [174]byte {
	var out [174]byte
	for i, bit := range bits {
		out[i] = byte(bit & 1)
	}
	return out
}

func tonesFromCodeword(codeword [174]int8) [ToneSymbols]uint8 {
	var data [DataSymbols]uint8
	for i := range data {
		value := int(codeword[2*i]&1)<<1 | int(codeword[2*i+1]&1)
		data[i] = grayMap[value]
	}

	var tones [ToneSymbols]uint8
	copy(tones[0:4], costasArrays[0][:])
	copy(tones[4:33], data[0:29])
	copy(tones[33:37], costasArrays[1][:])
	copy(tones[37:66], data[29:58])
	copy(tones[66:70], costasArrays[2][:])
	copy(tones[70:99], data[58:87])
	copy(tones[99:103], costasArrays[3][:])
	return tones
}
