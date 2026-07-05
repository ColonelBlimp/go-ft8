// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"strings"
	"testing"
)

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
		"CQ PJ4/NA2AA",
		"CQ YW18FIFA",
		"K1ABC/R W9XYZ RR73",
		"W9XYZ K1ABC/R R FN42",
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

func TestPackEncodeType4NonstandardMessages(t *testing.T) {
	tests := []struct {
		text string
		want string
		nrpt int
		icq  int
	}{
		{text: "CQ PJ4/NA2AA", want: "CQ PJ4/NA2AA", nrpt: 0, icq: 1},
		{text: "<K1ABC> PJ4/NA2AA", want: "<...> PJ4/NA2AA", nrpt: 0, icq: 0},
		{text: "PJ4/NA2AA <K1ABC> RRR", want: "PJ4/NA2AA <...> RRR", nrpt: 1, icq: 0},
		{text: "K1ABC PJ4/NA2AA RR73", want: "<...> PJ4/NA2AA RR73", nrpt: 2, icq: 0},
		{text: "PJ4/NA2AA K1ABC 73", want: "PJ4/NA2AA <...> 73", nrpt: 3, icq: 0},
		{text: "CQ YW18FIFA", want: "CQ YW18FIFA", nrpt: 0, icq: 1},
		{text: "K1ABC YW18FIFA RR73", want: "<...> YW18FIFA RR73", nrpt: 2, icq: 0},
	}

	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			bits, ok := pack77StandardMessage(test.text)
			if !ok {
				t.Fatalf("pack failed")
			}
			if got := readBits(bits[:], 71, 2); got != test.nrpt {
				t.Fatalf("nrpt got %d, want %d", got, test.nrpt)
			}
			if got := readBits(bits[:], 73, 1); got != test.icq {
				t.Fatalf("icq got %d, want %d", got, test.icq)
			}
			if got := readBits(bits[:], 74, 3); got != 4 {
				t.Fatalf("i3 got %d, want 4", got)
			}

			cw := encode17491(bits)
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
		})
	}
}

func TestPackType4CallRightAlignsC58(t *testing.T) {
	bits, ok := pack77StandardMessage("CQ PJ4/NA2AA")
	if !ok {
		t.Fatalf("pack failed")
	}

	got := readBits64(bits[:], 12, 58)
	if want := base38TestValue(t, "  PJ4/NA2AA"); got != want {
		t.Fatalf("c58 got %d, want right-aligned %d", got, want)
	}
	if leftAligned := base38TestValue(t, "PJ4/NA2AA  "); got == leftAligned {
		t.Fatalf("c58 got left-aligned value %d", got)
	}
}

func TestPackRejectsInvalidType4Messages(t *testing.T) {
	tests := []string{
		"CQ PJ4/NA2AA RR73",
		"PJ4/NA2AA 73",
		"CQ /",
		"CQ //",
		"CQ A/B",
		"CQ 0/0",
		"K1ABC / 73",
		"AB PJ4/NA2AA RR73",
		"DE PJ4/NA2AA",
		"QRZ PJ4/NA2AA",
		"RRR PJ4/NA2AA",
		"K1ABC/R W9XYZ/P RR73",
	}
	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			if _, ok := pack77StandardMessage(test); ok {
				t.Fatalf("pack succeeded, want rejection")
			}
		})
	}
}

func base38TestValue(t *testing.T, text string) uint64 {
	t.Helper()
	if len(text) != 11 {
		t.Fatalf("base38 test value %q has length %d, want 11", text, len(text))
	}
	var n uint64
	for i := 0; i < len(text); i++ {
		idx := strings.IndexByte(ft8Base38Alphabet, text[i])
		if idx < 0 {
			t.Fatalf("invalid base38 test byte %q", text[i])
		}
		n = 38*n + uint64(idx)
	}
	return n
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
		{text: "K1ABC/R W9XYZ RR73", ipa: 1, ipb: 0, i3: 1},
		{text: "W9XYZ K1ABC/R R FN42", ipa: 0, ipb: 1, i3: 1},
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
