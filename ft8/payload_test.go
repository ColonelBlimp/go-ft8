// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestPayload77PackingAndBitOrder(t *testing.T) {
	var bits [77]int8
	for _, index := range []int{0, 7, 8, 63, 64, 71, 73, 74, 76} {
		bits[index] = 1
	}

	payload := payload77FromBits(bits[:])
	if payload[0] != 0x81 || payload[1] != 0x80 || payload[7] != 0x01 ||
		payload[8] != 0x81 || payload[9] != 0x68 {
		t.Fatalf("packed payload got %x", payload)
	}
	if payload[9]&0x07 != 0 {
		t.Fatalf("padding bits got %#x, want zero", payload[9]&0x07)
	}

	got := payload.int8Bits()
	if got != bits {
		t.Fatalf("round trip got %v, want %v", got, bits)
	}
	for i, bit := range payload.Bits() {
		if bit != byte(bits[i]) {
			t.Fatalf("Bits()[%d] got %d, want %d", i, bit, bits[i])
		}
	}
}

func TestPayload77MessageType(t *testing.T) {
	tests := []struct {
		name string
		i3   int
		n3   int
		want MessageType
		text string
	}{
		{name: "subtype", i3: 0, n3: 5, want: MessageType{I3: 0, N3: 5}, text: "0.5"},
		{name: "main type", i3: 5, n3: 7, want: MessageType{I3: 5}, text: "5"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var bits [77]int8
			writeBits(bits[:], 71, 3, test.n3)
			writeBits(bits[:], 74, 3, test.i3)
			got := payload77FromBits(bits[:]).MessageType()
			if got != test.want || got.String() != test.text {
				t.Fatalf("MessageType got %+v (%q), want %+v (%q)", got, got.String(), test.want, test.text)
			}
		})
	}
}

func TestParseStatusString(t *testing.T) {
	tests := map[ParseStatus]string{
		ParseStatusUnset:       "unset",
		ParseStatusParsed:      "parsed",
		ParseStatusUnsupported: "unsupported",
		ParseStatusUnknownType: "unknown_type",
		ParseStatusInvalid:     "invalid",
		ParseStatus(255):       "unset",
	}
	for status, want := range tests {
		if got := status.String(); got != want {
			t.Fatalf("ParseStatus(%d).String got %q, want %q", status, got, want)
		}
	}
}

func TestPayload77DXpedition(t *testing.T) {
	payload := payload77FromGoldenBits(t, unpack77GoldenVectors[0].payload)
	message, ok := payload.DXpedition()
	if !ok || message.Call1 != "K1ABC" || message.Call2 != "W9XYZ" || message.Report != -12 {
		t.Fatalf("DXpedition = %+v, %v", message, ok)
	}
	hash, ok := CallsignHash("KH1/KH7Z", 10)
	if !ok || message.SenderHash10 != hash {
		t.Fatalf("sender hash = %d, want %d", message.SenderHash10, hash)
	}
	for _, n5 := range []int{0, 31} {
		bits := payload.int8Bits()
		writeBits(bits[:], 66, 5, n5)
		got, ok := payload77FromBits(bits[:]).DXpedition()
		if !ok || got.Report != 2*n5-30 {
			t.Fatalf("boundary report = %+v, %v", got, ok)
		}
	}
	for _, field := range []int{0, 28} {
		bits := payload.int8Bits()
		writeBits(bits[:], field, 28, ft8NTokens+hashCall("DX1ABC", 22))
		got, ok := payload77FromBits(bits[:]).DXpedition()
		if !ok || (field == 0 && got.Call1 != "<...>") || (field == 28 && got.Call2 != "<...>") {
			t.Fatalf("hashed call field %d = %+v, %v", field, got, ok)
		}
		for _, invalid := range []int{0, 1, 2, 10, 1003, ft8NTokens - 1, ft8NTokens + ft8Max22} {
			writeBits(bits[:], field, 28, invalid)
			if got, ok := payload77FromBits(bits[:]).DXpedition(); ok || got != (DXpeditionMessage{}) {
				t.Fatalf("accepted invalid call %d in field %d: %+v, %v", invalid, field, got, ok)
			}
		}
	}
	// Keep legacy text parsing permissive while the new accessor rejects tokens.
	bits := payload.int8Bits()
	writeBits(bits[:], 0, 28, 10)
	if text, status := ParsePayload77(payload77FromBits(bits[:])); status != ParseStatusParsed || text != "CQ_007 RR73; W9XYZ <...> -12" {
		t.Fatalf("legacy token rendering = %q, %s", text, status)
	}
	for _, vector := range unpack77GoldenVectors[1:] {
		if got, ok := payload77FromGoldenBits(t, vector.payload).DXpedition(); ok || got != (DXpeditionMessage{}) {
			t.Fatalf("accepted %s as DXpedition: %+v, %v", vector.name, got, ok)
		}
	}
}
