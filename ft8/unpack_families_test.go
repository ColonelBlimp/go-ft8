// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestParseAdditionalMessageFamilies(t *testing.T) {
	t.Run("DXpedition", func(t *testing.T) {
		call1, ok := pack28("K1ABC")
		if !ok {
			t.Fatal("pack call1")
		}
		call2, ok := pack28("W9XYZ")
		if !ok {
			t.Fatal("pack call2")
		}
		var bits [77]int8
		writeBits(bits[:], 0, 28, call1)
		writeBits(bits[:], 28, 28, call2)
		writeBits(bits[:], 56, 10, hashCall("KH1/KH7Z", 10))
		writeBits(bits[:], 66, 5, 9)
		writeBits(bits[:], 71, 3, 1)

		var hashes hashTable
		hashes.Save("KH1/KH7Z")
		assertPayloadParse(t, bits, &hashes, ParseStatusParsed,
			"K1ABC RR73; W9XYZ <KH1/KH7Z> -12")
	})

	t.Run("telemetry", func(t *testing.T) {
		var bits [77]int8
		writeBits(bits[:], 0, 23, 0x123456)
		writeBits(bits[:], 23, 24, 0x789ABC)
		writeBits(bits[:], 47, 24, 0xDEF012)
		writeBits(bits[:], 71, 3, 5)
		assertPayloadParse(t, bits, nil, ParseStatusParsed, "123456789ABCDEF012")
	})

	t.Run("ARRL RTTY multiplier", func(t *testing.T) {
		bits := rttyPayloadBits(t, "W9XYZ", "K1ABC", 1, 1, 5, 8000+21)
		assertPayloadParse(t, bits, nil, ParseStatusParsed,
			"TU; W9XYZ K1ABC R 579 MA")
	})

	t.Run("ARRL RTTY serial", func(t *testing.T) {
		bits := rttyPayloadBits(t, "W9XYZ", "G8ABC", 1, 1, 3, 13)
		assertPayloadParse(t, bits, nil, ParseStatusParsed,
			"TU; W9XYZ G8ABC R 559 0013")
	})

	t.Run("EU VHF", func(t *testing.T) {
		var bits [77]int8
		writeBits(bits[:], 0, 12, hashCall("PA3XYZ", 12))
		writeBits(bits[:], 12, 22, hashCall("G4ABC/P", 22))
		writeBits(bits[:], 34, 1, 1)
		writeBits(bits[:], 35, 3, 5)
		writeBits(bits[:], 38, 11, 7)
		writeBits(bits[:], 49, 25, grid6TestIndex("JO22DB"))
		writeBits(bits[:], 74, 3, 5)

		var hashes hashTable
		hashes.Save("G4ABC/P")
		hashes.Save("PA3XYZ")
		assertPayloadParse(t, bits, &hashes, ParseStatusParsed,
			"<PA3XYZ> <G4ABC/P> R 570007 JO22DB")
	})
}

func TestParsePayloadStatuses(t *testing.T) {
	tests := []struct {
		name   string
		i3     int
		n3     int
		status ParseStatus
	}{
		{name: "assigned unsupported 0.6", i3: 0, n3: 6, status: ParseStatusUnsupported},
		{name: "reserved 0.2", i3: 0, n3: 2, status: ParseStatusUnknownType},
		{name: "reserved 0.7", i3: 0, n3: 7, status: ParseStatusUnknownType},
		{name: "reserved 6", i3: 6, status: ParseStatusUnknownType},
		{name: "reserved 7", i3: 7, status: ParseStatusUnknownType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var bits [77]int8
			writeBits(bits[:], 71, 3, test.n3)
			writeBits(bits[:], 74, 3, test.i3)
			assertPayloadParse(t, bits, nil, test.status, "")
		})
	}

	t.Run("invalid RTTY exchange", func(t *testing.T) {
		bits := rttyPayloadBits(t, "W9XYZ", "K1ABC", 0, 0, 5, 8000)
		assertPayloadParse(t, bits, nil, ParseStatusInvalid, "")
	})

	t.Run("invalid grid6", func(t *testing.T) {
		var bits [77]int8
		writeBits(bits[:], 49, 25, 18*18*10*10*24*24)
		writeBits(bits[:], 74, 3, 5)
		assertPayloadParse(t, bits, nil, ParseStatusInvalid, "")
	})
}

func TestUnpackFreeTextUsesFullBase42Payload(t *testing.T) {
	bits := freeTextTestBits("CQ TEST 123?+")
	assertPayloadParse(t, bits, nil, ParseStatusParsed, "CQ TEST 123?+")
}

func assertPayloadParse(t *testing.T, bits [77]int8, hashes *hashTable, status ParseStatus, text string) {
	t.Helper()
	got := parsePayload77WithHashes(payload77FromBits(bits[:]), hashes)
	if got.Status != status || got.Text != text {
		t.Fatalf("parse got status=%s text=%q, want status=%s text=%q", got.Status, got.Text, status, text)
	}
}

func rttyPayloadBits(t *testing.T, call1, call2 string, tu, response, report, exchange int) [77]int8 {
	t.Helper()
	n28a, ok := pack28(call1)
	if !ok {
		t.Fatalf("pack %q", call1)
	}
	n28b, ok := pack28(call2)
	if !ok {
		t.Fatalf("pack %q", call2)
	}
	var bits [77]int8
	writeBits(bits[:], 0, 1, tu)
	writeBits(bits[:], 1, 28, n28a)
	writeBits(bits[:], 29, 28, n28b)
	writeBits(bits[:], 57, 1, response)
	writeBits(bits[:], 58, 3, report)
	writeBits(bits[:], 61, 13, exchange)
	writeBits(bits[:], 74, 3, 3)
	return bits
}

func grid6TestIndex(grid string) int {
	return int(grid[0]-'A')*18*10*10*24*24 +
		int(grid[1]-'A')*10*10*24*24 +
		int(grid[2]-'0')*10*24*24 +
		int(grid[3]-'0')*24*24 +
		int(grid[4]-'A')*24 + int(grid[5]-'A')
}

func freeTextTestBits(text string) [77]int8 {
	const alphabet = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ+-./?"
	var value [9]byte
	padded := make([]byte, 13)
	for i := range padded {
		padded[i] = ' '
	}
	copy(padded[13-len(text):], text)
	for _, ch := range padded {
		carry := 0
		for i := len(value) - 1; i >= 0; i-- {
			n := int(value[i])*len(alphabet) + carry
			value[i] = byte(n)
			carry = n >> 8
		}
		index := 0
		for i := range alphabet {
			if alphabet[i] == ch {
				index = i
				break
			}
		}
		carry = index
		for i := len(value) - 1; i >= 0 && carry != 0; i-- {
			n := int(value[i]) + carry
			value[i] = byte(n)
			carry = n >> 8
		}
	}
	var bits [77]int8
	for i := 0; i < 7; i++ {
		bits[i] = int8(value[0] >> uint(6-i) & 1)
	}
	for i := 7; i < 71; i++ {
		j := i - 7
		bits[i] = int8(value[1+j/8] >> uint(7-j%8) & 1)
	}
	return bits
}
