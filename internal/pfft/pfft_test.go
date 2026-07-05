// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo && !gonumfft

package pfft

import (
	"runtime"
	"testing"
)

// TestPlanCleanupAndClose exercises both plan release paths: explicit Close
// (which must stop the GC cleanup so the C plan is not destroyed twice) and
// abandonment (where the cleanup alone must free the plan without crashing).
func TestPlanCleanupAndClose(t *testing.T) {
	for i := 0; i < 100; i++ {
		rp, err := NewRealPlan(64)
		if err != nil {
			t.Fatal(err)
		}
		if err := rp.Forward(make([]float64, 64)); err != nil {
			t.Fatal(err)
		}
		cp, err := NewComplexPlan(64)
		if err != nil {
			t.Fatal(err)
		}
		if err := cp.Forward(make([]complex128, 64)); err != nil {
			t.Fatal(err)
		}
		if i%2 == 0 {
			rp.Close()
			rp.Close() // Close must be idempotent.
			cp.Close()
			cp.Close()
		}
		// Odd iterations abandon the plans to the GC cleanup.
	}
	for i := 0; i < 3; i++ {
		runtime.GC()
	}
}
