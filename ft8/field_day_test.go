// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestARRLFieldDaySectionsPublicContract(t *testing.T) {
	sections := ARRLFieldDaySections()
	if got, want := len(sections), 86; got != want {
		t.Fatalf("len(ARRLFieldDaySections()) got %d, want %d", got, want)
	}
	if sections[0] != ARRLFieldDaySectionAB {
		t.Fatalf("first section got %q, want %q", sections[0], ARRLFieldDaySectionAB)
	}
	if sections[83] != ARRLFieldDaySectionDX {
		t.Fatalf("DX section got %q, want %q", sections[83], ARRLFieldDaySectionDX)
	}
	if sections[84] != ARRLFieldDaySectionPE {
		t.Fatalf("PE section got %q, want %q", sections[84], ARRLFieldDaySectionPE)
	}
	if sections[85] != ARRLFieldDaySectionNB {
		t.Fatalf("last section got %q, want %q", sections[85], ARRLFieldDaySectionNB)
	}

	sections[0] = "ZZ"
	if got := ARRLFieldDaySections()[0]; got != ARRLFieldDaySectionAB {
		t.Fatalf("ARRLFieldDaySections did not return a defensive copy: first got %q", got)
	}
}

func TestParseARRLFieldDaySection(t *testing.T) {
	tests := []struct {
		in      string
		want    ARRLFieldDaySection
		wantOK  bool
		wantStr string
	}{
		{in: "EMA", want: ARRLFieldDaySectionEMA, wantOK: true, wantStr: "EMA"},
		{in: " ema ", want: ARRLFieldDaySectionEMA, wantOK: true, wantStr: "EMA"},
		{in: "dx", want: ARRLFieldDaySectionDX, wantOK: true, wantStr: "DX"},
		{in: "ZZ", wantOK: false},
		{in: "", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			got, ok := ParseARRLFieldDaySection(test.in)
			if ok != test.wantOK {
				t.Fatalf("ok got %v, want %v", ok, test.wantOK)
			}
			if got != test.want {
				t.Fatalf("section got %q, want %q", got, test.want)
			}
			if ok && got.String() != test.wantStr {
				t.Fatalf("String got %q, want %q", got.String(), test.wantStr)
			}
			if ValidARRLFieldDaySection(test.in) != test.wantOK {
				t.Fatalf("ValidARRLFieldDaySection mismatch")
			}
		})
	}
}
