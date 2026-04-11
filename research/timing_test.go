// timing_test.go — Measures per-pass and per-candidate timing for the
// iterative decode pipeline to assess real-time viability.
//
// WSJT-X's real-time constraint: all decoding must complete within ~15 s
// (one FT8 period). In practice WSJT-X targets ~12 s to leave headroom.
//
// ft8_decode.f90 uses timestamp/bail-out logic:
//   line 236–237: if tseq >= 13.4 → bail out (nzhsym=41, early decode)
//   line 137:     if tseq >= 14.3 → bail out (nzhsym=47, subtraction)

package research

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIterativeDecodeTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing test (slow)")
	}

	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	ddWork := make([]float32, NMAX)
	copy(ddWork, dd[:])

	const (
		ndepth  = 3
		nfa     = 200
		nfb     = 2600
		syncmin = 1.3
		nfqso   = 0
		maxcand = 600
		npass   = 3
	)

	seen := make(map[string]bool)
	ndecodes := 0
	n2 := 0

	totalStart := time.Now()

	for ipass := 1; ipass <= npass; ipass++ {
		passStart := time.Now()

		var ndeep int
		switch ipass {
		case 1:
			ndeep = 2
		case 2:
			n2 = ndecodes
			if ndecodes == 0 {
				t.Logf("Pass %d: skipped", ipass)
				continue
			}
			ndeep = ndepth
		case 3:
			if ndecodes-n2 == 0 {
				t.Logf("Pass %d: skipped", ipass)
				continue
			}
			ndeep = ndepth
		}

		// ── Time: sync8 ──────────────────────────────────────────────
		sync8Start := time.Now()
		var ddArr [NMAX]float32
		copy(ddArr[:], ddWork)
		resCands, _ := Sync8(ddArr, NMAX, nfa, nfb, syncmin, nfqso, maxcand)
		sync8Dur := time.Since(sync8Start)

		// Cap candidates
		candLimit := 300
		if ipass == npass {
			candLimit = 100
		}
		if len(resCands) > candLimit {
			resCands = resCands[:candLimit]
		}

		ft8xCands := make([]CandidateFreq, len(resCands))
		for i, c := range resCands {
			ft8xCands[i] = CandidateFreq{
				Freq:      c.Freq,
				DT:        c.DT,
				SyncPower: c.SyncPower,
			}
		}

		params := DecodeParams{
			Depth:     ndeep,
			APEnabled: true,
			APCQOnly:  true,
			APWidth:   25.0,
		}

		// ── Time: decode loop ────────────────────────────────────────
		decodeStart := time.Now()
		ds := NewDownsampler()
		passDecodes := 0
		candsTried := 0
		candsDecoded := 0
		candsFailed := 0

		// Track per-candidate timing
		var decodeTimesMs []float64

		for i, cand := range ft8xCands {
			candStart := time.Now()
			newdat := (i == 0)
			result, ok := DecodeSingle(ddWork, ds, cand.Freq, cand.DT, newdat, params)
			if !ok {
				if cand.SyncPower >= 2.0 {
					altDT := basebandTimeScan(ddWork, ds, cand.Freq)
					if math.Abs(altDT-cand.DT) > 0.1 {
						result, ok = DecodeSingle(ddWork, ds, cand.Freq, altDT, false, params)
					}
				}
			}
			candDur := time.Since(candStart)
			decodeTimesMs = append(decodeTimesMs, float64(candDur.Microseconds())/1000.0)
			candsTried++

			if !ok {
				candsFailed++
				continue
			}

			msg := strings.TrimSpace(result.Message)
			if seen[msg] {
				continue
			}
			seen[msg] = true
			ndecodes++
			passDecodes++
			candsDecoded++

			SubtractFT8(ddWork, result.Tones, result.Freq, result.DT)
		}
		decodeDur := time.Since(decodeStart)
		passDur := time.Since(passStart)

		// ── Compute timing stats ─────────────────────────────────────
		var sumMs, maxMs, minMs float64
		minMs = 1e9
		for _, ms := range decodeTimesMs {
			sumMs += ms
			if ms > maxMs {
				maxMs = ms
			}
			if ms < minMs {
				minMs = ms
			}
		}
		avgMs := 0.0
		if len(decodeTimesMs) > 0 {
			avgMs = sumMs / float64(len(decodeTimesMs))
		}

		// Find median
		medianMs := 0.0
		if len(decodeTimesMs) > 0 {
			sorted := make([]float64, len(decodeTimesMs))
			copy(sorted, decodeTimesMs)
			// Simple sort
			for i := 0; i < len(sorted); i++ {
				for j := i + 1; j < len(sorted); j++ {
					if sorted[j] < sorted[i] {
						sorted[i], sorted[j] = sorted[j], sorted[i]
					}
				}
			}
			medianMs = sorted[len(sorted)/2]
		}

		t.Logf("")
		t.Logf("═══ Pass %d (ndeep=%d) ═══════════════════════════════════", ipass, ndeep)
		t.Logf("  sync8:          %v", sync8Dur)
		t.Logf("  decode loop:    %v  (%d candidates tried)", decodeDur, candsTried)
		t.Logf("  total pass:     %v", passDur)
		t.Logf("  decoded:        %d new, %d failed", candsDecoded, candsFailed)
		t.Logf("  per-candidate:  avg=%.1f ms  median=%.1f ms  min=%.1f ms  max=%.1f ms",
			avgMs, medianMs, minMs, maxMs)
		t.Logf("  decode rate:    %.0f candidates/sec", float64(candsTried)/decodeDur.Seconds())
	}

	totalDur := time.Since(totalStart)
	t.Logf("")
	t.Logf("═══ TOTAL ════════════════════════════════════════════════")
	t.Logf("  Total wall time:     %v", totalDur)
	t.Logf("  Total decodes:       %d", ndecodes)
	t.Logf("  FT8 period budget:   15.0 s")
	t.Logf("  WSJT-X bail-out:     ~13.4 s")
	if totalDur.Seconds() < 15.0 {
		t.Logf("  ✓ WITHIN real-time budget")
	} else {
		t.Logf("  ✗ EXCEEDS real-time budget by %.1f s", totalDur.Seconds()-15.0)
	}
}
