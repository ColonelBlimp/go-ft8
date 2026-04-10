package ft8x

import (
	"os"
	"strings"
	"testing"
)

// TestSync8DiagCapture1 checks whether sync8 produces candidates near the
// known WSJT-X reference positions, and how syncmin affects decode count.
func TestSync8DiagCapture1(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping diagnostic test")
	}
	wavPath := "testdata/ft8test_capture_20260410.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}
	samples, sr, err := loadWAV(wavPath)
	if err != nil {
		t.Fatalf("load WAV: %v", err)
	}
	if sr != 12000 {
		t.Fatalf("expected 12000 Hz, got %d", sr)
	}

	// Run sync8 with a low threshold to see all candidates near known signals.
	cands := Sync8FindCandidates(samples, 200, 2600, 0.5, 0, 600)
	t.Logf("Found %d candidates with syncmin=0.5", len(cands))

	// Check if candidates exist near each WSJT-X reference signal.
	for i, ref := range capture1Candidates {
		best := -1
		bestDist := 999.0
		for j, c := range cands {
			df := c.Freq - ref.Freq
			ddt := c.DT - ref.DT
			dist := df*df + ddt*ddt*1e4
			if dist < bestDist {
				bestDist = dist
				best = j
			}
		}
		if best >= 0 {
			c := cands[best]
			t.Logf("  [%2d] ref=%7.1f Hz dt=%+5.2f → nearest cand=%7.1f Hz dt=%+5.2f sync=%.2f (Δf=%.1f ΔDT=%.3f)",
				i+1, ref.Freq, ref.DT, c.Freq, c.DT, c.SyncPower,
				c.Freq-ref.Freq, c.DT-ref.DT)
		} else {
			t.Logf("  [%2d] ref=%7.1f Hz dt=%+5.2f → NO candidate found", i+1, ref.Freq, ref.DT)
		}
	}

	// Now try different syncmin levels.
	for _, sm := range []float64{0.5, 0.8, 1.0, 1.3, 1.6, 2.0} {
		cands := Sync8FindCandidates(samples, 200, 2600, sm, 0, 600)
		params := DefaultDecodeParams()
		maxC := 200
		if len(cands) > maxC {
			cands = cands[:maxC]
		}
		results := Decode(samples, cands, params)
		correct := 0
		for _, r := range results {
			if wsjtxCapture1[strings.TrimSpace(r.Message)] {
				correct++
			}
		}
		t.Logf("syncmin=%.1f: %d candidates, %d decoded, %d correct",
			sm, len(cands), len(results), correct)
	}
}
