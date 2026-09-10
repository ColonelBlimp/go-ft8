// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ftx77

import "testing"

func TestDecode17491BPRoundTrip(t *testing.T) {
	var payload [77]int8
	for i := range payload {
		payload[i] = int8((i*17 + i/3) & 1)
	}
	want := Encode17491(payload)
	var llr [174]float64
	for i, bit := range want {
		llr[i] = -6
		if bit == 1 {
			llr[i] = 6
		}
	}
	got, ok := Decode17491BP(&llr)
	if !ok {
		t.Fatal("Decode17491BP rejected an exact codeword")
	}
	if got.Codeword != want {
		t.Fatal("decoded codeword differs from encoded codeword")
	}
	if got.HardErrors != 0 || got.DMin != 0 {
		t.Fatalf("exact decode metrics got hard=%d dmin=%v", got.HardErrors, got.DMin)
	}

	llr[23] = -llr[23]
	corrected, ok := Decode17491BP(&llr)
	if !ok || corrected.Codeword != want {
		t.Fatal("Decode17491BP did not correct one inverted channel decision")
	}
	if corrected.HardErrors != 1 {
		t.Fatalf("corrected hard errors got %d, want 1", corrected.HardErrors)
	}
}
