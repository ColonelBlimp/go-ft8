// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

var unpack77GoldenVectors = []struct {
	name    string
	payload string
	typeID  MessageType
	calls   []string
	text    string
}{
	{
		name:    "DXpedition",
		payload: "00001001101111011110001101010000110000101001001110111000001100100101001001000",
		typeID:  MessageType{I3: 0, N3: 1},
		calls:   []string{"KH1/KH7Z"},
		text:    "K1ABC RR73; W9XYZ <KH1/KH7Z> -12",
	},
	{
		name:    "telemetry",
		payload: "00100100011010001010110011110001001101010111100110111101111000000010010101000",
		typeID:  MessageType{I3: 0, N3: 5},
		text:    "123456789ABCDEF012",
	},
	{
		name:    "ARRL RTTY Roundup",
		payload: "10000110000101001001110111000000010011011110111100011010111011111101010101011",
		typeID:  MessageType{I3: 3, N3: 0},
		text:    "TU; W9XYZ K1ABC R 579 MA",
	},
	{
		name:    "EU VHF Contest",
		payload: "11001000101110000111101100010111111101000000001110100110101110000111001001101",
		typeID:  MessageType{I3: 5, N3: 0},
		calls:   []string{"PA9XYZ", "G4ABC/P"},
		text:    "<G4ABC/P> <PA9XYZ> R 570007 JO22DB",
	},
}

func TestUnpack77GoldenVectors(t *testing.T) {
	for _, test := range unpack77GoldenVectors {
		t.Run(test.name, func(t *testing.T) {
			payload := payload77FromGoldenBits(t, test.payload)
			if got := payload.MessageType(); got != test.typeID {
				t.Fatalf("message type = %v, want %v", got, test.typeID)
			}

			var hashes hashTable
			for _, call := range test.calls {
				hashes.Save(call)
			}
			got := parsePayload77WithHashes(payload, &hashes)
			if got.Status != ParseStatusParsed || got.Text != test.text {
				t.Fatalf("parse got status=%s text=%q, want status=%s text=%q",
					got.Status, got.Text, ParseStatusParsed, test.text)
			}
		})
	}
}

func payload77FromGoldenBits(t testing.TB, encoded string) Payload77 {
	t.Helper()
	if len(encoded) != 77 {
		t.Fatalf("golden payload has %d bits, want 77", len(encoded))
	}
	var bits [77]int8
	for i, ch := range encoded {
		switch ch {
		case '0':
		case '1':
			bits[i] = 1
		default:
			t.Fatalf("golden payload bit %d is %q, want 0 or 1", i, ch)
		}
	}
	return payload77FromBits(bits[:])
}
