// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"errors"
	"testing"
)

func TestEncodeStandardMessageRoundTrips(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "report", in: "PA9R SV9TLU -13", want: "PA9R SV9TLU -13"},
		{name: "lowercase", in: "ut7am pe9jan -10", want: "UT7AM PE9JAN -10"},
		{name: "cq", in: "CQ A61FJ LL74", want: "CQ A61FJ LL74"},
		{name: "final73", in: "PA2JFX SV3CNX 73", want: "PA2JFX SV3CNX 73"},
		{name: "responseReport", in: "G5MJF YM4KF R-11", want: "G5MJF YM4KF R-11"},
		{name: "directedCQ", in: "CQ DX S56GD JN65", want: "CQ DX S56GD JN65"},
		{name: "fieldDayCQ", in: "cq fd k1abc fn42", want: "CQ FD K1ABC FN42"},
		{name: "extraWhitespace", in: "  cq   dx   s56gd   jn65  ", want: "CQ DX S56GD JN65"},
		{name: "portableFirstCall", in: "G4ABC/P PA9XYZ JO22", want: "G4ABC/P PA9XYZ JO22"},
		{name: "portableSecondCall", in: "PA9XYZ G4ABC/P RR73", want: "PA9XYZ G4ABC/P RR73"},
		{name: "portableBothCalls", in: "PA3XYZ/P GM4ABC/P R JO22", want: "PA3XYZ/P GM4ABC/P R JO22"},
		{name: "portableCQ", in: "CQ G4ABC/P IO91", want: "CQ G4ABC/P IO91"},
		{name: "portableDirectedCQ", in: "cq test g4abc/p io91", want: "CQ TEST G4ABC/P IO91"},
		{name: "fieldDayExchange", in: "k1abc   w9xyz   6a   wi", want: "K1ABC W9XYZ 6A WI"},
		{name: "fieldDayResponseExchange", in: "w9xyz k1abc r 2b ema", want: "W9XYZ K1ABC R 2B EMA"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := EncodeStandardMessage(test.in)
			if err != nil {
				t.Fatalf("EncodeStandardMessage returned error: %v", err)
			}
			if encoded.Text != test.want {
				t.Fatalf("Text got %q, want %q", encoded.Text, test.want)
			}

			cw := codewordInt8(encoded.Codeword)
			if !crc14OK(&cw) {
				t.Fatalf("encoded codeword failed CRC")
			}
			got, ok := unpack77FromCodeword(cw)
			if !ok {
				t.Fatalf("unpack failed")
			}
			if got != test.want {
				t.Fatalf("round trip got %q, want %q", got, test.want)
			}

			for i, bit := range encoded.Bits77 {
				if bit > 1 {
					t.Fatalf("Bits77[%d] = %d, want 0 or 1", i, bit)
				}
			}
			for i, bit := range encoded.Codeword {
				if bit > 1 {
					t.Fatalf("Codeword[%d] = %d, want 0 or 1", i, bit)
				}
			}
			for i, tone := range encoded.Tones {
				if tone > 7 {
					t.Fatalf("Tones[%d] = %d, want 0..7", i, tone)
				}
			}
			for i, tone := range ft8Costas {
				if encoded.Tones[i] != uint8(tone) ||
					encoded.Tones[36+i] != uint8(tone) ||
					encoded.Tones[72+i] != uint8(tone) {
					t.Fatalf("Costas tone %d mismatch in encoded tone sequence", i)
				}
			}
		})
	}
}

func TestEncodeStandardMessageRejectsUnsupportedMessages(t *testing.T) {
	tests := []string{
		"",
		"HELLO WORLD",
		"THIS IS FREE TEXT",
		"CQ <K1ABC> FN42",
		"K1ABC W9XYZ -99",
		"CQ TEST/P G4ABC IO91",
		"G4ABC PA9XYZ/P/P JO22",
		"G4ABC PA9XYZ JO22/P",
		"CQ K1ABC 6A WI",
		"K1ABC W9XYZ 0A WI",
		"K1ABC W9XYZ 33A WI",
		"K1ABC W9XYZ 6I WI",
		"K1ABC W9XYZ 6A ZZ",
		"K1ABC W9XYZ R6A WI",
	}
	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			_, err := EncodeStandardMessage(test)
			if !errors.Is(err, ErrUnsupportedStandardMessage) {
				t.Fatalf("error got %v, want ErrUnsupportedStandardMessage", err)
			}
		})
	}
}

func codewordInt8(bits [174]byte) [174]int8 {
	var out [174]int8
	for i, bit := range bits {
		out[i] = int8(bit & 1)
	}
	return out
}
