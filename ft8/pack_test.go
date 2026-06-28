// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestPackEncodeRoundTripsStandardMessages(t *testing.T) {
	tests := []string{
		"PA9R SV9TLU -13",
		"UT7AM PE9JAN -10",
		"CQ A61FJ LL74",
		"PA2JFX SV3CNX 73",
		"G5MJF YM4KF R-11",
		"CQ DX S56GD JN65",
		"CQ FD K1ABC FN42",
		"G4ABC/P PA9XYZ JO22",
		"PA9XYZ G4ABC/P RR73",
		"PA3XYZ/P GM4ABC/P R JO22",
		"CQ G4ABC/P IO91",
		"CQ TEST G4ABC/P IO91",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			bits, ok := pack77StandardMessage(test)
			if !ok {
				t.Fatalf("pack failed")
			}
			cw := encode17491(bits)
			if !crc14OK(&cw) {
				t.Fatalf("encoded codeword failed CRC")
			}
			got, ok := unpack77FromCodeword(cw)
			if !ok {
				t.Fatalf("unpack failed")
			}
			if got != test {
				t.Fatalf("round trip got %q, want %q", got, test)
			}
		})
	}
}

func TestPackEncodeRoundTripsARRLFieldDayMessages(t *testing.T) {
	tests := []string{
		"K1ABC W9XYZ 6A WI",
		"WA9XYZ KA1ABC R 16A EMA",
		"WA9XYZ KA1ABC 7D EMA",
		"WA9XYZ G8ABC 1D DX",
		"WA9XYZ KA1ABC R 32A EMA",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			bits, ok := pack77StandardMessage(test)
			if !ok {
				t.Fatalf("pack failed")
			}
			cw := encode17491(bits)
			if !crc14OK(&cw) {
				t.Fatalf("encoded codeword failed CRC")
			}
			got, ok := unpack77FromCodeword(cw)
			if !ok {
				t.Fatalf("unpack failed")
			}
			if got != test {
				t.Fatalf("round trip got %q, want %q", got, test)
			}
		})
	}
}

func TestPackStandardMessagePortableBits(t *testing.T) {
	tests := []struct {
		text string
		ipa  int
		ipb  int
		i3   int
	}{
		{text: "G4ABC PA9XYZ JO22", ipa: 0, ipb: 0, i3: 1},
		{text: "G4ABC/P PA9XYZ JO22", ipa: 1, ipb: 0, i3: 2},
		{text: "PA9XYZ G4ABC/P RR73", ipa: 0, ipb: 1, i3: 2},
		{text: "PA3XYZ/P GM4ABC/P R JO22", ipa: 1, ipb: 1, i3: 2},
		{text: "CQ G4ABC/P IO91", ipa: 0, ipb: 1, i3: 2},
		{text: "CQ TEST G4ABC/P IO91", ipa: 0, ipb: 1, i3: 2},
	}

	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			bits, ok := pack77StandardMessage(test.text)
			if !ok {
				t.Fatalf("pack failed")
			}
			if got := readBits(bits[:], 28, 1); got != test.ipa {
				t.Fatalf("ipa got %d, want %d", got, test.ipa)
			}
			if got := readBits(bits[:], 57, 1); got != test.ipb {
				t.Fatalf("ipb got %d, want %d", got, test.ipb)
			}
			if got := readBits(bits[:], 74, 3); got != test.i3 {
				t.Fatalf("i3 got %d, want %d", got, test.i3)
			}
		})
	}
}

func TestPackARRLFieldDayBits(t *testing.T) {
	tests := []struct {
		text   string
		ir     int
		intx   int
		nclass int
		isec   int
		n3     int
	}{
		{text: "K1ABC W9XYZ 6A WI", ir: 0, intx: 5, nclass: 0, isec: 76, n3: 3},
		{text: "WA9XYZ KA1ABC R 16A EMA", ir: 1, intx: 15, nclass: 0, isec: 11, n3: 3},
		{text: "WA9XYZ KA1ABC 17H NB", ir: 0, intx: 0, nclass: 7, isec: 86, n3: 4},
		{text: "WA9XYZ KA1ABC R 32A EMA", ir: 1, intx: 15, nclass: 0, isec: 11, n3: 4},
	}

	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			bits, ok := pack77StandardMessage(test.text)
			if !ok {
				t.Fatalf("pack failed")
			}
			if got := readBits(bits[:], 56, 1); got != test.ir {
				t.Fatalf("ir got %d, want %d", got, test.ir)
			}
			if got := readBits(bits[:], 57, 4); got != test.intx {
				t.Fatalf("intx got %d, want %d", got, test.intx)
			}
			if got := readBits(bits[:], 61, 3); got != test.nclass {
				t.Fatalf("nclass got %d, want %d", got, test.nclass)
			}
			if got := readBits(bits[:], 64, 7); got != test.isec {
				t.Fatalf("isec got %d, want %d", got, test.isec)
			}
			if got := readBits(bits[:], 71, 3); got != test.n3 {
				t.Fatalf("n3 got %d, want %d", got, test.n3)
			}
			if got := readBits(bits[:], 74, 3); got != 0 {
				t.Fatalf("i3 got %d, want 0", got)
			}
		})
	}
}
