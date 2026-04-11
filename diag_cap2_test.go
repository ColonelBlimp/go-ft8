package ft8x

// Diagnostic test: trace the iterative pipeline for Capture 2 to understand
// why UY7VV KE6SU (553 Hz) and JT1CO YO3HST (1096 Hz) are lost.

import (
	"math"
	"os"
	"strings"
	"testing"
)

// diagSync8ForFreq computes the sync8 2D correlation profile at a specific
// frequency, logging the narrow-search peak, wide-search peak, and the
// sync value at the expected DT.
func diagSync8ForFreq(t *testing.T, dd []float32, targetFreq, expectedDT float64) {
	t.Helper()

	const (
		nfft1 = NFFT1
		nh1   = NH1
		nstep = NSTEP
		nhsym = NHSYM
		jz    = jzSync
	)

	tstep := float64(nstep) / Fs
	df := Fs / float64(nfft1)
	fac := float32(1.0 / 300.0)
	nssy := NSPS / nstep
	nfos := nfft1 / NSPS
	jstrt := int(0.5 / tstep)

	// Compute spectrogram
	nh1Pad := nh1 + nfos*6 + 1
	s := make([][]float64, nh1Pad+1)
	for i := range s {
		s[i] = make([]float64, nhsym+1)
	}
	buf := make([]float32, NSPS)
	for j := 1; j <= nhsym; j++ {
		ia := (j - 1) * nstep
		for k := 0; k < NSPS; k++ {
			idx := ia + k
			if idx < len(dd) {
				buf[k] = dd[idx] * fac
			} else {
				buf[k] = 0
			}
		}
		cx := RealFFT(buf, nfft1)
		for i := 1; i <= nh1 && i < len(cx); i++ {
			r, im := real(cx[i]), imag(cx[i])
			s[i][j] = r*r + im*im
		}
	}

	// Compute sync for target frequency bin
	iBin := int(math.Round(targetFreq / df))
	t.Logf("  Freq=%.0f Hz → bin=%d (actual=%.1f Hz)", targetFreq, iBin, float64(iBin)*df)

	sync2d := make([]float64, 2*jz+1)
	for j := -jz; j <= jz; j++ {
		var ta, tb, tc float64
		var t0a, t0b, t0c float64
		for n := 0; n <= 6; n++ {
			m := j + jstrt + nssy*n
			if m >= 1 && m <= nhsym {
				ta += s[iBin+nfos*Icos7[n]][m]
				for k := 0; k <= 6; k++ {
					t0a += s[iBin+nfos*k][m]
				}
			}
			mb := m + nssy*36
			if mb >= 1 && mb <= nhsym {
				tb += s[iBin+nfos*Icos7[n]][mb]
				for k := 0; k <= 6; k++ {
					t0b += s[iBin+nfos*k][mb]
				}
			}
			mc := m + nssy*72
			if mc >= 1 && mc <= nhsym {
				tc += s[iBin+nfos*Icos7[n]][mc]
				for k := 0; k <= 6; k++ {
					t0c += s[iBin+nfos*k][mc]
				}
			}
		}
		t := ta + tb + tc
		t0 := t0a + t0b + t0c
		t0 = (t0 - t) / 6.0
		syncABC := 0.0
		if t0 > 0 {
			syncABC = t / t0
		}
		tbc := tb + tc
		t0bc := t0b + t0c
		t0bc = (t0bc - tbc) / 6.0
		syncBC := 0.0
		if t0bc > 0 {
			syncBC = tbc / t0bc
		}
		if syncBC > syncABC {
			sync2d[j+jz] = syncBC
		} else {
			sync2d[j+jz] = syncABC
		}
	}

	// Find narrow peak (±10 lags)
	narrowBest := -10
	for lag := -10; lag <= 10; lag++ {
		if sync2d[lag+jz] > sync2d[narrowBest+jz] {
			narrowBest = lag
		}
	}
	narrowDT := (float64(narrowBest) - 0.5) * tstep

	// Find wide peak (±62 lags)
	wideBest := -jz
	for lag := -jz; lag <= jz; lag++ {
		if sync2d[lag+jz] > sync2d[wideBest+jz] {
			wideBest = lag
		}
	}
	wideDT := (float64(wideBest) - 0.5) * tstep

	// Sync value at expected DT
	expectedLag := int(math.Round(expectedDT/tstep + 0.5))
	expectedSync := 0.0
	if expectedLag >= -jz && expectedLag <= jz {
		expectedSync = sync2d[expectedLag+jz]
	}

	t.Logf("    Narrow peak (±10): lag=%d DT=%.2f sync=%.2f", narrowBest, narrowDT, sync2d[narrowBest+jz])
	t.Logf("    Wide peak  (±62): lag=%d DT=%.2f sync=%.2f", wideBest, wideDT, sync2d[wideBest+jz])
	t.Logf("    At expected DT=%.1f: lag=%d sync=%.2f", expectedDT, expectedLag, expectedSync)
}

func TestDiagCapture2Lost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping diagnostic test (slow)")
	}

	wavPath := "testdata/ft8test_capture2_20260410.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("WAV not found: %s", wavPath)
	}

	samples, sr, err := loadWAV(wavPath)
	if err != nil {
		t.Fatalf("load WAV: %v", err)
	}
	if sr != 12000 {
		t.Fatalf("expected 12000 Hz, got %d", sr)
	}

	// Check what sync8 sees for the 553 Hz and 1096 Hz bins.
	t.Log("=== Sync8 internal diagnostics ===")
	diagSync8ForFreq(t, samples, 553.0, 1.8)
	diagSync8ForFreq(t, samples, 1096.0, 1.8)

	// Check what basebandTimeScan finds for the target frequencies.
	t.Log("=== basebandTimeScan diagnostics ===")

	type targetSig struct {
		label string
		freq  float64
		dt    float64 // xdt
	}
	targets := []targetSig{
		{"UY7VV KE6SU DM14", 553.0, 1.8},
		{"JT1CO YO3HST KN24", 1096.0, 1.8},
	}

	scanDS := NewDownsampler()
	for _, tgt := range targets {
		nd := true
		cd0 := scanDS.Downsample(samples, &nd, tgt.freq)

		var ctwkZero [32]complex128
		for i := range ctwkZero {
			ctwkZero[i] = complex(1, 0)
		}
		smax := 0.0
		ibest := 0
		for idt := 0; idt <= NP2; idt += 4 {
			sync := Sync8d(cd0, idt, ctwkZero, 0)
			if sync > smax {
				smax = sync
				ibest = idt
			}
		}
		altDT := float64(ibest-1) * Dt2
		t.Logf("  %s: basebandTimeScan → ibest=%d altDT=%.3f sync=%.2f (expected DT=%.1f)",
			tgt.label, ibest, altDT, smax, tgt.dt)

		// Also show sync at expected position
		i0target := int(math.Round((tgt.dt + 0.5) * Fs2))
		syncTarget := Sync8d(cd0, i0target, ctwkZero, 0)
		t.Logf("    at expected i0=%d (DT=%.1f): sync=%.2f", i0target, tgt.dt, syncTarget)
	}

	// Now run the full iterative pipeline trace.
	t.Log("\n=== Iterative pipeline trace ===")
	dd := make([]float32, max(len(samples), NMAX))
	copy(dd, samples)

	params := DefaultDecodeParams()
	maxPasses := params.MaxPasses
	if maxPasses <= 0 {
		maxPasses = 3
	}
	seen := make(map[string]bool)

	for pass := 0; pass < maxPasses; pass++ {
		t.Logf("══════ PASS %d ══════", pass)

		// Step 1: Check if targets are decodable BEFORE candidate detection
		t.Logf("  Pre-scan: can targets decode from current audio?")
		preDS := NewDownsampler()
		for _, tgt := range targets {
			result, ok := DecodeSingle(dd, preDS, tgt.freq, tgt.dt, true, params)
			if ok {
				t.Logf("    ✓ %s → %q nhard=%d", tgt.label, strings.TrimSpace(result.Message), result.NHardErrors)
			} else {
				t.Logf("    ✗ %s → not decodable", tgt.label)
			}
		}

		// Step 2: Find candidates
		syncmin := 1.3
		candidates := Sync8FindCandidates(dd, 200, 2600, syncmin, 0, 600)
		t.Logf("  Found %d candidates", len(candidates))

		// Step 3: Check if targets appear in candidate list
		for _, tgt := range targets {
			found := 0
			for i, c := range candidates {
				if math.Abs(c.Freq-tgt.freq) < 10.0 {
					t.Logf("    Candidate near %s: rank=%d freq=%.1f dt=%.2f sync=%.2f",
						tgt.label, i+1, c.Freq, c.DT, c.SyncPower)
					found++
				}
			}
			if found == 0 {
				t.Logf("    ✗ No candidate within 10 Hz of %s", tgt.label)
			}
		}

		// Step 4: Decode pass (mirroring DecodeIterative logic)
		passParams := params
		passParams.APEnabled = true
		passParams.APCQOnly = true
		if pass == 0 && params.Depth == 3 {
			passParams.Depth = 2
		}
		if pass == maxPasses-1 && params.Depth >= 2 {
			passParams.Depth = 3
			if len(candidates) > 100 {
				candidates = candidates[:100]
			}
		} else {
			if len(candidates) > 300 {
				candidates = candidates[:300]
			}
		}

		ds := NewDownsampler()
		newDecodes := 0
		for i, cand := range candidates {
			newdat := (i == 0)
			result, ok := DecodeSingle(dd, ds, cand.Freq, cand.DT, newdat, passParams)
			if !ok {
				continue
			}
			if seen[result.Message] {
				continue
			}
			seen[result.Message] = true
			newDecodes++

			// Log the decode
			msg := strings.TrimSpace(result.Message)
			t.Logf("    DECODE: %7.1f Hz dt=%+.1f nhard=%d ap=%d  %s",
				result.Freq, result.DT, result.NHardErrors, result.APType, msg)

			// Subtract
			SubtractFT8(dd, result.Tones, result.Freq, result.DT)
		}

		t.Logf("  Pass %d: %d new decodes", pass, newDecodes)
		if newDecodes == 0 {
			t.Log("  No new decodes — stopping")
			break
		}
	}
}
