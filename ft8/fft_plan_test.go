// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math/cmplx"
	"testing"
)

func TestRealFFTPlanRangeContract(t *testing.T) {
	seq := []float64{1, -2, 3, -4, 5, -6, 7, -8}
	original := append([]float64(nil), seq...)
	plan := newRealFFTPlan(len(seq))
	full := plan.Coefficients(nil, seq)
	assertFloat64SliceEqual(t, seq, original)

	ranged := plan.CoefficientsRange(nil, seq, 2, 3)
	assertFloat64SliceEqual(t, seq, original)
	if len(ranged) != len(full) {
		t.Fatalf("range length=%d, want %d", len(ranged), len(full))
	}
	for i := range ranged {
		switch {
		case i == 2 || i == 3:
			if cmplx.Abs(ranged[i]-full[i]) > 1e-9 {
				t.Fatalf("bin %d = %v, want %v", i, ranged[i], full[i])
			}
		case ranged[i] != 0:
			t.Fatalf("out-of-range bin %d = %v, want zero", i, ranged[i])
		}
	}
}

func TestRealFFTPlanWorkRangeContract(t *testing.T) {
	seq := []float64{1, -2, 3, -4, 5, -6, 7, -8}
	plan := newRealFFTPlan(len(seq))
	full := plan.Coefficients(nil, seq)

	dst := make([]complex128, len(full))
	for i := range dst {
		dst[i] = complex(99, 99)
	}
	ranged := plan.workCoefficientsRange(dst, seq, 2, 3)
	if &ranged[0] != &dst[0] {
		t.Fatalf("workCoefficientsRange returned a different backing array")
	}
	for i := range ranged {
		switch {
		case i == 2 || i == 3:
			if cmplx.Abs(ranged[i]-full[i]) > 1e-9 {
				t.Fatalf("bin %d = %v, want %v", i, ranged[i], full[i])
			}
		case ranged[i] != 0:
			t.Fatalf("out-of-range bin %d = %v, want zero", i, ranged[i])
		}
	}

	work := plan.workCoefficients(dst, seq)
	if &work[0] != &dst[0] {
		t.Fatalf("workCoefficients returned a different backing array")
	}
	for i := range work {
		if cmplx.Abs(work[i]-full[i]) > 1e-9 {
			t.Fatalf("bin %d = %v, want %v", i, work[i], full[i])
		}
	}
}

func assertFloat64SliceEqual(t *testing.T, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length=%d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("slice[%d]=%v, want %v", i, got[i], want[i])
		}
	}
}
