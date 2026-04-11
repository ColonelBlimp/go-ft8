// iwave_test.go — Research test that replicates the exact iwave array
// from WSJT-X's ft8d.f90 program, loaded from capture.wav.
//
// In ft8d.f90 the audio is represented as:
//
//   integer*2 iwave(NMAX)              ! 180000 int16 samples
//   integer   ihdr(11)                 ! 11×4 = 44-byte WAV header
//   real      dd(NMAX)                 ! float copy (no /32768 normalisation)
//
//   read(10,end=999) ihdr,iwave        ! binary stream read
//   dd = iwave                         ! Fortran implicit int16 → real cast
//
// This test loads testdata/capture.wav identically: it skips the 44-byte
// header, reads up to NMAX int16 samples into iwave, then converts to dd
// by simple float32 cast (NOT divided by 32768).

package research

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"testing"
)

// loadIwave reads a 16-bit PCM WAV file exactly like ft8d.f90:
//
//   - Skips the first 44 bytes (WAV header, equivalent to ihdr(11)).
//   - Reads up to NMAX little-endian int16 samples into iwave[0:NMAX-1].
//   - If the file contains fewer than NMAX samples, the remainder is zero-filled.
//   - Returns both iwave (int16) and dd (float32, unscaled — matches Fortran's dd=iwave).
func loadIwave(path string) (iwave [NMAX]int16, dd [NMAX]float32, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return iwave, dd, err
	}

	// ── Validate WAV header (minimal, matching ft8d.f90 assumptions) ──
	if len(data) < 44 {
		return iwave, dd, fmt.Errorf("file too small for WAV header (%d bytes)", len(data))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return iwave, dd, fmt.Errorf("not a RIFF/WAVE file")
	}

	// ft8d.f90 reads the 44-byte header into ihdr(11) (11 × integer*4).
	// We just skip it and go straight to the PCM data.
	//
	// NOTE: ft8d.f90 assumes data starts at byte 44 (ihdr is 11×int32).
	// We search for the actual 'data' chunk below to handle any extra
	// sub-chunks that real WAV files may contain.

	// Parse sample rate from header bytes 24..27 for a sanity check.
	sr := binary.LittleEndian.Uint32(data[24:28])
	if sr != 12000 {
		return iwave, dd, fmt.Errorf("expected 12000 Hz sample rate, got %d", sr)
	}

	// Parse bits-per-sample from header bytes 34..35.
	bps := binary.LittleEndian.Uint16(data[34:36])
	if bps != 16 {
		return iwave, dd, fmt.Errorf("expected 16-bit samples, got %d", bps)
	}

	// ── Find the 'data' chunk (handles extra sub-chunks) ──
	off := 12 // skip RIFF + size + WAVE
	dataOff := -1
	dataLen := 0
	for off+8 <= len(data) {
		chunkID := string(data[off : off+4])
		chunkSz := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if chunkID == "data" {
			dataOff = off + 8
			dataLen = chunkSz
			break
		}
		off += 8 + chunkSz
		if chunkSz%2 != 0 {
			off++ // RIFF chunks are 2-byte aligned
		}
	}
	if dataOff < 0 {
		return iwave, dd, fmt.Errorf("no 'data' chunk found in WAV file")
	}

	// ── Read int16 samples exactly like Fortran's binary stream read ──
	pcm := data[dataOff:]
	if len(pcm) > dataLen {
		pcm = pcm[:dataLen]
	}
	nSamples := len(pcm) / 2
	if nSamples > NMAX {
		nSamples = NMAX
	}

	for i := 0; i < nSamples; i++ {
		iwave[i] = int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
	}
	// Remaining elements stay zero (Fortran iwave is zero-initialised for
	// any samples beyond the file).

	// ── dd = iwave (Fortran implicit int16 → real, NO division by 32768) ──
	for i := 0; i < NMAX; i++ {
		dd[i] = float32(iwave[i])
	}

	return iwave, dd, nil
}

// TestLoadIwave verifies that capture.wav can be loaded into the exact
// same iwave / dd representation used by WSJT-X ft8d.f90.
func TestLoadIwave(t *testing.T) {
	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	iwave, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// ── Basic sanity checks ──

	// 1. iwave must have at least some non-zero samples.
	nonZero := 0
	for i := 0; i < NMAX; i++ {
		if iwave[i] != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatal("iwave is all zeros — WAV file appears empty")
	}
	t.Logf("iwave: %d / %d samples non-zero (%.1f%%)",
		nonZero, NMAX, 100.0*float64(nonZero)/float64(NMAX))

	// 2. dd must be an exact float32 cast of iwave (no scaling).
	for i := 0; i < NMAX; i++ {
		want := float32(iwave[i])
		if dd[i] != want {
			t.Fatalf("dd[%d] = %g, want %g (iwave=%d)", i, dd[i], want, iwave[i])
		}
	}
	t.Log("dd == float32(iwave) — matches Fortran dd=iwave  ✓")

	// 3. dd values must be in the int16 range [-32768, 32767].
	var minVal, maxVal float32
	for i := 0; i < NMAX; i++ {
		if dd[i] < minVal {
			minVal = dd[i]
		}
		if dd[i] > maxVal {
			maxVal = dd[i]
		}
	}
	if minVal < -32768 || maxVal > 32767 {
		t.Errorf("dd range [%g, %g] exceeds int16 bounds", minVal, maxVal)
	}
	t.Logf("dd range: [%g, %g]", minVal, maxVal)

	// 4. RMS sanity — a reasonable audio capture should have RMS > 1.
	var sumSq float64
	for i := 0; i < NMAX; i++ {
		sumSq += float64(dd[i]) * float64(dd[i])
	}
	rms := math.Sqrt(sumSq / float64(NMAX))
	t.Logf("dd RMS: %.2f", rms)
	if rms < 1.0 {
		t.Error("dd RMS suspiciously low — capture may be silence")
	}

	// 5. Log first 10 samples for visual inspection.
	t.Logf("First 10 iwave samples: %v", iwave[:10])
	t.Logf("First 10 dd samples:    %v", dd[:10])
}

// TestIwaveVsNormalized documents the exact discrepancy between the Go loadWAV
// convention (float32(v)/32768.0 → range [-1,+1]) and the Fortran ft8d.f90
// convention (dd=iwave → raw int16 magnitudes, e.g. ±8000).
//
// Analysis summary: every decode-critical path in the current Go pipeline is
// scale-invariant (ratio-metric sync, variance-normalized LLRs, ratio SNR).
// So decode results are identical either way.  But the discrepancy matters for:
//
//  1. Future porting of xsnr2 = xsig/xbase/3.0e6 (calibrated for int16 range).
//  2. Easier debugging: intermediate values match Fortran when using raw int16.
func TestIwaveVsNormalized(t *testing.T) {
	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	iwave, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// Build the normalized version (Go convention: /32768).
	var ddNorm [NMAX]float32
	for i := 0; i < NMAX; i++ {
		ddNorm[i] = float32(iwave[i]) / 32768.0
	}

	// ── Verify the scaling factor ──
	// dd (Fortran) vs ddNorm (Go) should differ by exactly 32768.
	const tol = 1e-6
	mismatch := 0
	for i := 0; i < NMAX; i++ {
		if dd[i] == 0 && ddNorm[i] == 0 {
			continue
		}
		if ddNorm[i] == 0 {
			mismatch++
			continue
		}
		ratio := float64(dd[i]) / float64(ddNorm[i])
		if math.Abs(ratio-32768.0) > tol {
			mismatch++
			if mismatch <= 5 {
				t.Logf("  sample[%d]: dd=%g, ddNorm=%g, ratio=%g (expected 32768)", i, dd[i], ddNorm[i], ratio)
			}
		}
	}
	if mismatch > 0 {
		t.Errorf("found %d samples where dd/ddNorm != 32768", mismatch)
	} else {
		t.Log("dd / ddNorm == 32768 for all non-zero samples  ✓")
	}

	// ── Show representative magnitudes to illustrate the discrepancy ──
	// Find the first 5 non-zero samples for comparison.
	shown := 0
	for i := 0; i < NMAX && shown < 5; i++ {
		if dd[i] != 0 {
			t.Logf("  sample[%d]: Fortran dd=%8.1f   Go ddNorm=%11.8f   ratio=%.0f",
				i, dd[i], ddNorm[i], float64(dd[i])/float64(ddNorm[i]))
			shown++
		}
	}

	// ── Quantify the FFT magnitude impact ──
	// sync8.f90 applies fac=1/300 then FFTs.  Show the typical magnitude
	// of the first FFT bin at both scales to illustrate debugging difficulty.
	fac := float32(1.0 / 300.0)
	t.Logf("sync8 fac=1/300 scaling comparison (sample 1000):")
	t.Logf("  Fortran: dd[1000]*fac = %g × %g = %g", dd[1000], fac, dd[1000]*fac)
	t.Logf("  Go:      ddNorm[1000]*fac = %g × %g = %g", ddNorm[1000], fac, ddNorm[1000]*fac)
	t.Logf("  Factor: %.0f×", float64(dd[1000]*fac)/float64(ddNorm[1000]*fac))
}

// TestSync8Skeleton calls the research Sync8 skeleton with the exact same
// arguments as ft8d.f90 line 45:
//
//	call sync8(iwave,NMAX,nfa,nfb,nfqso,s,candidate,ncand)
//
// For now Sync8 is a do-nothing stub — this test confirms the wiring is
// correct and the dd array flows through to the function.
func TestSync8Skeleton(t *testing.T) {
	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// Match ft8d.f90 parameters:
	//   nfa    = 100
	//   nfb    = 3000
	//   syncmin = 2.0   (set after sync8 returns, but sync8.f90 uses it internally)
	//   nfqso  = 1500
	//   maxcand = 100   (dimension of candidate(3,100))
	nfa := 100
	nfb := 3000
	syncmin := 2.0
	nfqso := 1500
	maxcand := 100

	candidates, sbase := Sync8(dd, NMAX, nfa, nfb, syncmin, nfqso, maxcand)

	t.Logf("Sync8 returned %d candidates", len(candidates))
	t.Logf("sbase[0:5] = %v", sbase[0:5])

	// Verify sync2d is producing real correlation data by calling
	// computeSync2D directly on known signal frequencies from capture.wav.
	spec := ComputeSpectrogramForTest(dd[:], NMAX)
	tstep := float64(NSTEP) / Fs
	dfVal := Fs / float64(NFFT1)
	nssy := NSPS / NSTEP
	nfosVal := NFFT1 / NSPS
	jstrt := int(0.5 / tstep)
	sync2d := ComputeSync2DForTest(spec, nfa, nfb, dfVal, nssy, nfosVal, jstrt)

	if sync2d != nil {
		// Check a few known signal frequencies from capture.wav.
		// Look for the peak sync value across all lags at each freq bin.
		for _, freq := range []float64{300.0, 568.0, 1000.0, 1097.0, 1500.0} {
			bin := int(math.Round(freq / dfVal))
			if bin < 1 || bin >= len(sync2d) {
				continue
			}
			bestSync := 0.0
			bestLag := 0
			for lag := -JZ; lag <= JZ; lag++ {
				if v := sync2d[bin][lag+JZ]; v > bestSync {
					bestSync = v
					bestLag = lag
				}
			}
			dt := (float64(bestLag) - 0.5) * tstep
			t.Logf("  sync2d at %.0f Hz (bin %d): peak=%.2f  lag=%d  DT=%.2f s",
				freq, bin, bestSync, bestLag, dt)
		}
	} else {
		t.Error("sync2d is nil — computeSync2D returned nothing")
	}
	nonZeroBins := 0
	for i := 1; i <= NH1; i++ {
		if spec.Savg[i] > 0 {
			nonZeroBins++
		}
	}
	t.Logf("Spectrogram: %d / %d freq bins have non-zero savg", nonZeroBins, NH1)
	if nonZeroBins == 0 {
		t.Error("spectrogram savg is all zeros — FFT loop not producing data")
	}

	// Log a few representative power values at known FT8 frequencies.
	df := Fs / float64(NFFT1) // 3.125 Hz
	for _, freq := range []float64{300.0, 1000.0, 1500.0, 2500.0} {
		bin := int(freq / df)
		if bin >= 1 && bin <= NH1 {
			t.Logf("  savg[bin %d = %.0f Hz] = %.2e", bin, freq, spec.Savg[bin])
		}
	}

	// For now just verify it runs without panic.
	// As Sync8 is implemented, we'll add real assertions here.
	if candidates != nil {
		t.Logf("First candidate: freq=%.1f Hz  DT=%.3f s  sync=%.2f",
			candidates[0].Freq, candidates[0].DT, candidates[0].SyncPower)
	}
}
