// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"testing"
)

func TestCostasEvidenceClampsZeroRatios(t *testing.T) {
	var s8 [8][ft8Symbols]float64
	for k, targetTone := range ft8Costas {
		for _, sym := range [...]int{k, k + 36, k + 72} {
			s8[targetTone][sym] = 2
			s8[(targetTone+1)%8][sym] = 1
		}
	}
	s8[ft8Costas[0]][0] = 0

	result := costasEvidence(&s8)
	if result.Geo <= 0 || math.IsInf(result.Geo, 0) || math.IsNaN(result.Geo) {
		t.Fatalf("CostasGeo got %v, want finite positive value", result.Geo)
	}
	if result.MinBlock <= 0 || math.IsInf(result.MinBlock, 0) || math.IsNaN(result.MinBlock) {
		t.Fatalf("CostasMinBlock got %v, want finite positive value", result.MinBlock)
	}
}
