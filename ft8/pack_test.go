// SPDX-FileCopyrightText: 2026 go-ft8 authors
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
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			bits, ok := pack77StandardMessage(test)
			if !ok {
				t.Fatalf("pack failed")
			}
			cw := encode17491(bits)
			if !crc14OK(cw) {
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
