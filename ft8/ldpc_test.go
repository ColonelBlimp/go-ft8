// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"testing"
)

func TestFastTanhAccuracyAndBounds(t *testing.T) {
	const step = 1e-4
	maxErr := 0.0
	for x := -10.0; x <= 10.0; x += step {
		got := fastTanh(x)
		if math.Abs(got) > 1 {
			t.Fatalf("fastTanh(%v)=%v exceeds ±1", x, got)
		}
		if err := math.Abs(got - math.Tanh(x)); err > maxErr {
			maxErr = err
		}
	}
	if maxErr > 1e-4 {
		t.Fatalf("max |fastTanh - math.Tanh| = %v, want <= 1e-4", maxErr)
	}
	if fastTanh(0) != 0 {
		t.Fatalf("fastTanh(0) = %v, want 0", fastTanh(0))
	}
	if fastTanh(100) != 1 || fastTanh(-100) != -1 {
		t.Fatal("fastTanh saturation broken")
	}
}
