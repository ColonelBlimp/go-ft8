// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestWinsorizeLLRUsesEvenMedianAverage(t *testing.T) {
	var llr [174]float64
	llr[0] = 1
	llr[1] = -3

	winsorizeLLR(&llr, 1)

	if llr[0] != 1 {
		t.Fatalf("llr[0] got %v, want 1", llr[0])
	}
	if llr[1] != -2 {
		t.Fatalf("llr[1] got %v, want -2", llr[1])
	}
}
