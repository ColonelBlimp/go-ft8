// missing_signals_test.go — Deep dive into the 9 WSJT-X signals that
// our sync8 candidate detection is missing.

package research

import (
	"math"
	"os"
	"testing"
)

// The 9 signals NOT COVERED by either sync8 implementation.
var missingSignals = []struct {
	Label string
	Freq  float64
	DT    float64 // xdt
}{
	{"CQ NH6D BL02", 2251, 0.9},
	{"VK3TZ UX2QX R-07", 1776, 0.8},
	{"R8AGW VU2FR RR73", 2507, 1.5},
	{"<V4/SP9FIH> JA8RVP QN23", 1001, 0.9},
	{"CQ UR5QW KN77", 2401, 0.9},
	{"UA0LW RK6AAC KN95", 1149, 1.1},
	{"VK5BU RG5A KN93", 533, 0.9},
	{"5Z4VJ YB1RUS OI33", 1505, 0.8},
	{"VK3TZ RC7O KN87", 963, 0.9},
}

func TestMissingSignalSync2D(t *testing.T) {
	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	nfa := 200
	nfb := 2600

	// Compute spectrogram + sync2d directly.
	spec := computeSpectrogram(dd[:], NMAX)

	tstep := float64(NSTEP) / Fs
	df := Fs / float64(NFFT1)
	nssy := NSPS / NSTEP
	nfos := NFFT1 / NSPS
	jstrt := int(0.5 / tstep)

	sync2d := computeSync2D(spec, nfa, nfb, df, nssy, nfos, jstrt)

	// For each missing signal, look at the sync2d value at/near the expected
	// freq/DT, and find the actual peak in a wider window.
	t.Logf("Missing signals — sync2d analysis:")
	t.Logf("  syncmin=1.3 after 40th-percentile normalization")
	t.Logf("")

	for _, sig := range missingSignals {
		bin := int(math.Round(sig.Freq / df))
		// Convert DT to lag: dt = (lag - 0.5) * tstep → lag = dt/tstep + 0.5
		expectedLag := int(math.Round(sig.DT/tstep + 0.5))

		// Get sync2d at expected position.
		syncAtExpected := 0.0
		if bin >= 1 && bin < len(sync2d) && expectedLag+jz >= 0 && expectedLag+jz < len(sync2d[bin]) {
			syncAtExpected = sync2d[bin][expectedLag+jz]
		}

		// Find peak in ±3 bins freq, ±10 lags around expected.
		bestSync := 0.0
		bestBin := bin
		bestLag := expectedLag
		for bi := bin - 3; bi <= bin+3; bi++ {
			if bi < 1 || bi >= len(sync2d) {
				continue
			}
			for lag := expectedLag - 10; lag <= expectedLag+10; lag++ {
				li := lag + jz
				if li < 0 || li >= len(sync2d[bi]) {
					continue
				}
				if sync2d[bi][li] > bestSync {
					bestSync = sync2d[bi][li]
					bestBin = bi
					bestLag = lag
				}
			}
		}

		// Find the global peak at this freq bin across ALL lags.
		globalBestSync := 0.0
		globalBestLag := 0
		if bin >= 1 && bin < len(sync2d) {
			for lag := -jz; lag <= jz; lag++ {
				li := lag + jz
				if li >= 0 && li < len(sync2d[bin]) && sync2d[bin][li] > globalBestSync {
					globalBestSync = sync2d[bin][li]
					globalBestLag = lag
				}
			}
		}

		t.Logf("  %-32s  ref=%7.1f Hz (bin %d)  DT=%+5.2f (lag %d)",
			sig.Label, sig.Freq, bin, sig.DT, expectedLag)
		t.Logf("    sync2d at expected:    %6.2f", syncAtExpected)
		t.Logf("    best in ±3bin/±10lag:  %6.2f  at bin=%d (%.1f Hz) lag=%d (DT=%+.2f)",
			bestSync, bestBin, float64(bestBin)*df, bestLag, (float64(bestLag)-0.5)*tstep)
		t.Logf("    global peak at bin:    %6.2f  at lag=%d (DT=%+.2f)",
			globalBestSync, globalBestLag, (float64(globalBestLag)-0.5)*tstep)
		t.Logf("")
	}

	// Also show what the raw (un-normalized) sync values look like for
	// the covered signals to calibrate what "good" looks like.
	t.Logf("── For comparison: covered signals sync2d values ──")
	covered := []struct {
		Label string
		Freq  float64
		DT    float64
	}{
		{"VK5BU RT6C KN95", 2454, 0.8},
		{"A61OK US1VM KN68", 568, 1.0},
		{"UB8CSR SV0TPN -18", 1998, 1.2},
		{"CQ KB3Z FN20", 1100, 1.0},
		{"CQ CO8LY FL20", 932, 0.7},
	}
	for _, sig := range covered {
		bin := int(math.Round(sig.Freq / df))
		bestSync := 0.0
		bestLag := 0
		if bin >= 1 && bin < len(sync2d) {
			for lag := -jz; lag <= jz; lag++ {
				li := lag + jz
				if li >= 0 && li < len(sync2d[bin]) && sync2d[bin][li] > bestSync {
					bestSync = sync2d[bin][li]
					bestLag = lag
				}
			}
		}
		t.Logf("  ✓ %-32s  bin=%d  peak sync2d=%6.2f  lag=%d (DT=%+.2f)",
			sig.Label, bin, bestSync, bestLag, (float64(bestLag)-0.5)*tstep)
	}
}
