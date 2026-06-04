// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestDecodeA7CandidateAmbiguityUsesDistinctMessages(t *testing.T) {
	target := "K1ABC W9XYZ RR73"
	bits, ok := pack77StandardMessage(target)
	if !ok {
		t.Fatalf("pack77StandardMessage(%q) failed", target)
	}
	cw := encode17491(bits)
	analysis := candidateAnalysis{
		Refined: refinedCandidate{HardSync: 7},
		Metrics: softMetrics{
			Single: metricForA7Codeword(cw, 10),
			Double: metricForA7Codeword(cw, 9),
			Triple: metricForA7Codeword(cw, 80),
			Normed: metricForA7Codeword(cw, 90),
		},
	}

	decoded, ok := decodeA7Candidate(&analysis, a7Hint{Call1: "K1ABC", Call2: "W9XYZ"}, make(map[string][174]int8))
	if !ok {
		t.Fatal("decodeA7Candidate rejected distinct-message ranking")
	}
	if decoded.Text != target {
		t.Fatalf("decoded text got %q, want %q", decoded.Text, target)
	}
}

func metricForA7Codeword(cw [174]int8, flipped int) [174]float64 {
	var metric [174]float64
	for i, bit := range cw {
		sign := -1.0
		if bit == 1 {
			sign = 1
		}
		if i < flipped {
			sign = -sign
		}
		metric[i] = sign / ft8ScaleFac
	}
	return metric
}
