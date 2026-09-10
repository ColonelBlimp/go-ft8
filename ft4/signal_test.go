// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

import (
	"math"
	"math/rand"
	"testing"
)

func TestSyncScoreOptimizedMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(17))
	samples := make([]complex128, downFFT)
	for i := range samples {
		samples[i] = complex(rng.NormFloat64(), rng.NormFloat64())
	}
	for _, delta := range []int{-16, -7, 0, 9, 16} {
		for _, start := range []int{-400, -344, -33, 0, 333, 1012, 3900, 4100} {
			got := syncScore(samples, start, delta)
			want := syncScoreReference(samples, start, delta)
			if math.Abs(got-want) > 1e-12 {
				t.Fatalf("syncScore(start=%d, delta=%d) got %.15g, want %.15g", start, delta, got, want)
			}
		}
	}
}

func syncScoreReference(samples []complex128, start, delta int) float64 {
	var score float64
	for block, symbolStart := range [...]int{0, 33, 66, 99} {
		var sum complex128
		count := 0
		for i, reference := range syncReferences[delta+16][block] {
			index := start + symbolStart*downNSPS + 2*i
			if index < 0 || index >= len(samples) {
				continue
			}
			sum += samples[index] * reference
			count++
		}
		if count > 16 {
			score += math.Sqrt(complexPower(sum)) / float64(2*downNSPS)
		}
	}
	return score
}

func TestDecodeWorkspaceClearsSamples(t *testing.T) {
	workspace := acquireDecodeWorkspace()
	for i := range workspace.samples {
		workspace.samples[i] = 1
	}
	releaseDecodeWorkspace(workspace)

	workspace = acquireDecodeWorkspace()
	defer releaseDecodeWorkspace(workspace)
	for i, sample := range workspace.samples {
		if sample != 0 {
			t.Fatalf("reused workspace sample %d was not cleared", i)
		}
	}
}
