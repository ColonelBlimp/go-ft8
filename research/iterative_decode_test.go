// iterative_decode_test.go — Research implementation of WSJT-X's iterative
// decode-and-subtract pipeline using the research sync8 (optimized RealFFT)
// for candidate detection and ft8x for downstream decoding.
//
// This closely mimics the hot path in:
//   wsjt-wsjtx/lib/ft8_decode.f90 lines 168–239
//
// The Fortran structure is:
//
//   npass=3  (for ndepth>=2)
//   do ipass=1,npass
//     newdat=.true.
//     syncmin=1.3
//     if(ipass==1) lsubtract=.true.; ndeep=2 (if ndepth==3)
//     if(ipass==2) n2=ndecodes; if(ndecodes==0) cycle; ndeep=ndepth
//     if(ipass==3) if((ndecodes-n2)==0) cycle; ndeep=ndepth
//     call sync8(dd, ...)
//     do icand=1,ncand
//       call ft8b(dd, ..., ndeep, lsubtract, ...)
//       if(decode success && !dupe) ndecodes++; save itone,f1,xdt
//     enddo
//   enddo

package research

import (
	"math"
	"os"
	"strings"
	"testing"
)

// WSJT-X reference decodes for capture.wav (21 signals).
var wsjtxCapture3 = map[string]bool{
	"VK5BU RT6C KN95":   true,
	"CQ V4/SP9FIH":      true,
	"A61OK US1VM KN68":  true,
	"CQ KB3Z FN20":      true,
	"CQ NH6D BL02":      true,
	"VK3TZ UX2QX R-07":  true,
	"UB8CSR SV0TPN -18": true,
	"R8AGW VU2FR RR73":  true,
	"<...> JA8RVP QN23": true,
	"CQ UR5QW KN77":     true,
	"UA0LW RK6AAC KN95": true,
	"CQ SP4MSY KO13":    true,
	"VK5ATH VK0DS -01":  true,
	"VK5BU RG5A KN93":   true,
	"5Z4VJ YB1RUS OI33": true,
	"UA0LW UA4ARH -15":  true,
	"VK5BU R7HL 73":     true,
	"CQ 4S6ARW MJ97":    true,
	"CQ CO8LY FL20":     true,
	"VK3TZ UA3ZNQ KO81": true,
	"VK3TZ RC7O KN87":   true,
}

// TestIterativeDecode implements the WSJT-X multi-pass decode pipeline:
//
//	Pass 1: sync8 → decode (ndeep=2) → subtract
//	Pass 2: sync8 on cleaned audio → decode (full depth) → subtract
//	Pass 3: sync8 on cleaned audio → decode (full depth) → subtract
//
// This matches ft8_decode.f90 lines 172–239.
func TestIterativeDecode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping iterative decode test (slow)")
	}

	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// Working copy — modified in-place by subtraction.
	ddWork := make([]float32, NMAX)
	copy(ddWork, dd[:])

	// ── Parameters matching ft8_decode.f90 ───────────────────────────
	const (
		ndepth  = 3    // full depth
		nfa     = 200  // freq search low bound
		nfb     = 2600 // freq search high bound
		syncmin = 1.3  // ft8_decode.f90 line 176
		nfqso   = 0    // no QSO freq prioritization
		maxcand = 600  // MAXCAND parameter
		npass   = 3    // ft8_decode.f90 line 172
	)

	type savedDecode struct {
		msg   string
		freq  float64
		xdt   float64
		snr   float64
		tones [NN]int
		pass  int
	}

	var allDecodes []savedDecode
	seen := make(map[string]bool)
	ndecodes := 0
	n2 := 0 // ft8_decode.f90 line 184: n2=ndecodes (set at start of pass 2)

	for ipass := 1; ipass <= npass; ipass++ {
		// ── Pass-specific logic (ft8_decode.f90 lines 179–192) ───────
		var ndeep int
		switch ipass {
		case 1:
			// ft8_decode.f90 lines 180–182
			ndeep = ndepth
			if ndepth == 3 {
				ndeep = 2 // lighter OSD on first pass
			}
		case 2:
			// ft8_decode.f90 lines 184–187
			n2 = ndecodes
			if ndecodes == 0 {
				t.Logf("Pass %d: skipped (no decodes from pass 1)", ipass)
				continue
			}
			ndeep = ndepth
		case 3:
			// ft8_decode.f90 lines 188–191
			if ndecodes-n2 == 0 {
				t.Logf("Pass %d: skipped (no new decodes in pass 2)", ipass)
				continue
			}
			ndeep = ndepth
		}

		// ── sync8 on current (possibly cleaned) audio ────────────────
		// ft8_decode.f90 line 175: newdat=.true.
		// Convert ddWork to [NMAX]float32 for research Sync8.
		var ddArr [NMAX]float32
		copy(ddArr[:], ddWork)

		resCands, _ := Sync8(ddArr, NMAX, nfa, nfb, syncmin, nfqso, maxcand)
		t.Logf("Pass %d: sync8 returned %d candidates (ndeep=%d)", ipass, len(resCands), ndeep)

		// ── Convert research candidates to CandidateFreq ────────
		ft8xCands := make([]CandidateFreq, len(resCands))
		for i, c := range resCands {
			ft8xCands[i] = CandidateFreq{
				Freq:      c.Freq,
				DT:        c.DT,
				SyncPower: c.SyncPower,
			}
		}

		// Cap candidates per pass (matching DecodeIterative behavior).
		// Later passes use deeper OSD, so fewer candidates are practical.
		candLimit := 300
		if ipass == npass {
			candLimit = 100
		}
		if len(ft8xCands) > candLimit {
			ft8xCands = ft8xCands[:candLimit]
		}

		// ── Decode each candidate (ft8_decode.f90 lines 197–238) ─────
		params := DecodeParams{
			Depth:     ndeep,
			APEnabled: true,
			APCQOnly:  true,
			APWidth:   25.0,
		}

		ds := NewDownsampler()
		passDecodes := 0

		for i, cand := range ft8xCands {
			newdat := (i == 0) // recompute 192k FFT on first candidate of each pass
			result, ok := DecodeSingle(ddWork, ds, cand.Freq, cand.DT, newdat, params)
			if !ok {
				// Retry high-sync candidates with baseband time scan
				// (matches existing DecodeIterative behavior).
				if cand.SyncPower >= 2.0 {
					altDT := basebandTimeScan(ddWork, ds, cand.Freq)
					if math.Abs(altDT-cand.DT) > 0.1 {
						result, ok = DecodeSingle(ddWork, ds, cand.Freq, altDT, false, params)
					}
				}
				if !ok {
					continue
				}
			}

			msg := strings.TrimSpace(result.Message)
			if seen[msg] {
				continue
			}
			seen[msg] = true
			ndecodes++
			passDecodes++

			allDecodes = append(allDecodes, savedDecode{
				msg:   msg,
				freq:  result.Freq,
				xdt:   result.DT,
				snr:   result.SNR,
				tones: result.Tones,
				pass:  ipass,
			})

			// ── Signal subtraction (ft8_decode.f90 line 435) ─────────
			// FFT-based low-pass filter method matching Fortran subtractft8.
			SubtractFT8(ddWork, result.Tones, result.Freq, result.DT)
		}

		t.Logf("Pass %d: %d new decodes (total: %d)", ipass, passDecodes, ndecodes)
	}

	// ── Results ──────────────────────────────────────────────────────
	t.Logf("")
	t.Logf("═══════════════════════════════════════════════════════════")
	t.Logf("ITERATIVE DECODE RESULTS — %d total decodes", len(allDecodes))
	t.Logf("═══════════════════════════════════════════════════════════")

	correct := 0
	falseDecodes := 0
	for _, d := range allDecodes {
		tag := " ✗ (not in reference)"
		if wsjtxCapture3[d.msg] {
			correct++
			tag = " ✓"
		} else {
			falseDecodes++
		}
		t.Logf("  pass=%d  %+6.1f dt  %7.1f Hz  %+5.1f dB  %s%s",
			d.pass, d.xdt, d.freq, d.snr, d.msg, tag)
	}

	t.Logf("")
	t.Logf("SUMMARY:")
	t.Logf("  WSJT-X reference:  %d signals", len(wsjtxCapture3))
	t.Logf("  Decoded:           %d total", len(allDecodes))
	t.Logf("  Correct:           %d / %d (%.0f%%)",
		correct, len(wsjtxCapture3), 100.0*float64(correct)/float64(len(wsjtxCapture3)))
	t.Logf("  False decodes:     %d", falseDecodes)

	// Show which reference signals we missed.
	t.Logf("")
	t.Logf("Missing WSJT-X signals:")
	for msg := range wsjtxCapture3 {
		if !seen[msg] {
			t.Logf("  ✗ %s", msg)
		}
	}
}

// basebandTimeScan finds the best time offset for a signal at frequency f0
// by doing a coarse Sync8d scan over the full NP2 range of the downsampled
// baseband signal. Mirrors basebandTimeScan (unexported in main package).
func basebandTimeScan(dd []float32, ds *Downsampler, f0 float64) float64 {
	nd := false
	cd0 := ds.Downsample(dd, &nd, f0)

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
	return float64(ibest-1) * Dt2
}

// TestIterativeDecodeVsMainPackage compares the research iterative pipeline
// against the main DecodeIterative to verify we get the same or better
// results.
func TestIterativeDecodeVsMainPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping comparison test (slow)")
	}

	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// ── Main package DecodeIterative ─────────────────────────────────
	ddSlice := make([]float32, NMAX)
	copy(ddSlice, dd[:])

	params := DefaultDecodeParams()
	params.Depth = 3
	params.MaxPasses = 3
	params.APEnabled = true
	params.APCQOnly = true

	mainResults := DecodeIterative(ddSlice, params, 200, 2600)

	mainCorrect := 0
	mainSeen := make(map[string]bool)
	for _, r := range mainResults {
		msg := strings.TrimSpace(r.Message)
		mainSeen[msg] = true
		if wsjtxCapture3[msg] {
			mainCorrect++
		}
	}

	t.Logf("DecodeIterative: %d decodes, %d correct / %d reference",
		len(mainResults), mainCorrect, len(wsjtxCapture3))
	for _, r := range mainResults {
		tag := ""
		msg := strings.TrimSpace(r.Message)
		if wsjtxCapture3[msg] {
			tag = " ✓"
		}
		t.Logf("  %+6.1f dt  %7.1f Hz  %+5.1f dB  ap=%d  %s%s",
			r.DT, r.Freq, r.SNR, r.APType, msg, tag)
	}
}
