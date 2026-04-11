// ft8x_wav_test.go — WAV-based integration tests for the ft8x decoder.
//
// These tests validate the ft8x package (a close port of WSJT-X's Fortran
// decoder) against real FT8 captures with known WSJT-X decode results.
//
// Test WAV files live in testdata/ — tests skip gracefully if
// not found.

package ft8x

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// WAV file loader (minimal, no external deps)
// ────────────────────────────────────────────────────────────────────────────

func loadWAV(path string) ([]float32, uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	if len(data) < 44 {
		return nil, 0, fmt.Errorf("file too small (%d bytes)", len(data))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("not a WAV file")
	}
	if string(data[12:16]) != "fmt " {
		return nil, 0, fmt.Errorf("expected fmt chunk")
	}
	format := u16LE(data[20:22])
	if format != 1 {
		return nil, 0, fmt.Errorf("unsupported format %d (want PCM=1)", format)
	}
	nCh := u16LE(data[22:24])
	sr := u32LE(data[24:28])
	bps := u16LE(data[34:36])
	if bps != 16 {
		return nil, 0, fmt.Errorf("unsupported bits/sample %d (want 16)", bps)
	}

	off := 36
	for off+8 <= len(data) {
		id := string(data[off : off+4])
		sz := int(u32LE(data[off+4 : off+8]))
		if id == "data" {
			pcm := data[off+8 : off+8+sz]
			bytesPerFrame := int(nCh) * int(bps) / 8
			nFrames := len(pcm) / bytesPerFrame
			samples := make([]float32, nFrames)
			for i := 0; i < nFrames; i++ {
				o := i * bytesPerFrame
				v := int16(uint16(pcm[o]) | uint16(pcm[o+1])<<8)
				samples[i] = float32(v) / 32768.0
			}
			return samples, sr, nil
		}
		off += 8 + sz
		if sz%2 != 0 {
			off++
		}
	}
	return nil, 0, fmt.Errorf("no data chunk found")
}

func u16LE(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
func u32LE(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// ────────────────────────────────────────────────────────────────────────────
// Verified reference decodes
// ────────────────────────────────────────────────────────────────────────────

// Capture 1: 13 WSJT-X decodes.
var wsjtxCapture1 = map[string]bool{
	"SV2SIH ES2AJ -16":   true,
	"VE1WT K4GBI 73":     true,
	"SV2SIH KI8JP -10":   true,
	"CQ PV8AJ FJ92":      true,
	"<...> RA1OHX KP91":  true,
	"KB7THX WB9VGJ RR73": true,
	"A61CK UA1CEI KP50":  true,
	"<...> LU3DXU GF05":  true,
	"<...> RA6ABC KN96":  true,
	"ES2AJ UA3LAR KO75":  true,
	"A61CK W3DQS -12":    true,
	"HZ1TT RU1AB R-10":   true,
	"<...> RV6ASU KN94":  true,
}

// Capture 1: WSJT-X reference with approximate freq and DT (xdt convention: xdt=0 at 0.5s).
// Frequencies and TimeOff values from the modular pipeline's baseband decode output.
// xdt = TimeOff - 0.5
var capture1Candidates = []CandidateFreq{
	{Freq: 1860.5, DT: 0.000 - 0.5}, // SV2SIH ES2AJ -16
	{Freq: 1309.9, DT: 0.140 - 0.5}, // VE1WT K4GBI 73
	{Freq: 1902.9, DT: 0.030 - 0.5}, // SV2SIH KI8JP -10
	{Freq: 1691.8, DT: 0.000 - 0.5}, // CQ PV8AJ FJ92
	{Freq: 2098.6, DT: 0.090 - 0.5}, // <...> RA1OHX KP91
	{Freq: 2328.0, DT: -0.2},        // KB7THX WB9VGJ RR73 (weak, -21 dB)
	{Freq: 948.2, DT: 0.000 - 0.5},  // A61CK UA1CEI KP50
	{Freq: 1273.0, DT: -0.1 - 0.5},  // <...> LU3DXU GF05
	{Freq: 1814.0, DT: -0.2 - 0.5},  // <...> RA6ABC KN96
	{Freq: 835.0, DT: -0.1 - 0.5},   // ES2AJ UA3LAR KO75 (weak)
	{Freq: 579.1, DT: 0.050 - 0.5},  // A61CK W3DQS -12
	{Freq: 2208.4, DT: 1.080 - 0.5}, // HZ1TT RU1AB R-10
	{Freq: 460.9, DT: 0.180 - 0.5},  // <...> RV6ASU KN94
}

// Capture 2: 17 verified decodes (15 from WSJT-X + 2 additional found by go-ft8).
// The two extra decodes (HA1BF, VK2VT) have been verified: callsigns and
// gridsquares check out as real stations.
var wsjtxCapture2 = map[string]bool{
	"HA5LB 5B4AMX RR73":   true,
	"CQ ZS4AW KG31":       true,
	"CQ SV0TPN KM28":      true,
	"CQ Z62NS KN02":       true,
	"VK/ZL4XZ <...> RR73": true,
	"VK3ZSJ YO8RQP KN37":  true,
	"R1QD KB2ELA -12":     true,
	"UY7VV KE6SU DM14":    true,
	"TL8GD UT2VX KN69":    true,
	"RU4LM 4X5JK R-14":    true,
	"JT1CO IZ7DIO 73":     true,
	"VK3ZSJ US7KC KO21":   true,
	"JR3UIC SP7IIT RR73":  true,
	"JT1CO YO3HST KN24":   true,
	"CQ TN8GD JI75":       true,
	"CQ HA1BF JN86":       true, // not in WSJT-X, verified real
	"UA4CCH VK2VT RR73":   true, // not in WSJT-X, verified real
}

// Capture 2: verified reference with approximate freq and DT (xdt convention).
// Frequencies from WSJT-X (15 signals) plus 2 additional found by go-ft8.
// TimeOff values for capture 2 are approximately 2.3-2.4 s → xdt ≈ 1.8-1.9.
var capture2Candidates = []CandidateFreq{
	{Freq: 815.6, DT: 2.960 - 0.5},  // HA5LB 5B4AMX RR73
	{Freq: 1776.0, DT: 2.350 - 0.5}, // CQ ZS4AW KG31
	{Freq: 2100.0, DT: 2.360 - 0.5}, // CQ SV0TPN KM28
	{Freq: 745.6, DT: 2.310 - 0.5},  // CQ Z62NS KN02
	{Freq: 331.8, DT: 2.320 - 0.5},  // VK/ZL4XZ <...> RR73
	{Freq: 862.5, DT: 2.360 - 0.5},  // VK3ZSJ YO8RQP KN37
	{Freq: 1463.8, DT: 2.310 - 0.5}, // R1QD KB2ELA -12
	{Freq: 553.0, DT: 1.8},          // UY7VV KE6SU DM14
	{Freq: 319.0, DT: 1.8},          // TL8GD UT2VX KN69
	{Freq: 1840.0, DT: 1.8},         // RU4LM 4X5JK R-14
	{Freq: 1768.0, DT: 1.8},         // JT1CO IZ7DIO 73
	{Freq: 1502.0, DT: 1.8},         // VK3ZSJ US7KC KO21
	{Freq: 1410.0, DT: 1.8},         // JR3UIC SP7IIT RR73
	{Freq: 1096.0, DT: 1.8},         // JT1CO YO3HST KN24
	{Freq: 451.0, DT: 1.8},          // CQ TN8GD JI75
	{Freq: 1272.0, DT: 1.8},         // CQ HA1BF JN86 (not in WSJT-X, verified real)
	{Freq: 2593.0, DT: 1.9},         // UA4CCH VK2VT RR73 (not in WSJT-X, verified real)
}

// Capture 3 (capture.wav): 21 WSJT-X decodes from FTdx10 DATA-mode capture.
var wsjtxCapture3 = map[string]bool{
	"VK5BU RT6C KN95":   true,
	"CQ V4/SP9FIH":      true, // compound callsign (type 4)
	"A61OK US1VM KN68":  true,
	"CQ KB3Z FN20":      true,
	"CQ NH6D BL02":      true,
	"VK3TZ UX2QX R-07":  true,
	"UB8CSR SV0TPN -18": true,
	"R8AGW VU2FR RR73":  true,
	"<...> JA8RVP QN23": true, // hashed V4/SP9FIH
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

// Capture 3: WSJT-X reference with approximate freq and DT (xdt convention).
// DT values from WSJT-X output; xdt = TimeOff - 0.5.
var capture3Candidates = []CandidateFreq{
	{Freq: 2454, DT: 1.3 - 0.5}, // VK5BU RT6C KN95
	{Freq: 298, DT: 1.3 - 0.5},  // CQ V4/SP9FIH
	{Freq: 568, DT: 1.5 - 0.5},  // A61OK US1VM KN68
	{Freq: 1100, DT: 1.5 - 0.5}, // CQ KB3Z FN20
	{Freq: 2251, DT: 1.4 - 0.5}, // CQ NH6D BL02
	{Freq: 1776, DT: 1.3 - 0.5}, // VK3TZ UX2QX R-07
	{Freq: 1998, DT: 1.7 - 0.5}, // UB8CSR SV0TPN -18
	{Freq: 2507, DT: 2.0 - 0.5}, // R8AGW VU2FR RR73
	{Freq: 1001, DT: 1.4 - 0.5}, // <V4/SP9FIH> JA8RVP QN23
	{Freq: 2401, DT: 1.4 - 0.5}, // CQ UR5QW KN77
	{Freq: 1149, DT: 1.6 - 0.5}, // UA0LW RK6AAC KN95
	{Freq: 1250, DT: 1.2 - 0.5}, // CQ SP4MSY KO13
	{Freq: 1648, DT: 1.4 - 0.5}, // VK5ATH VK0DS -01
	{Freq: 533, DT: 1.4 - 0.5},  // VK5BU RG5A KN93
	{Freq: 1505, DT: 1.3 - 0.5}, // 5Z4VJ YB1RUS OI33
	{Freq: 1211, DT: 1.5 - 0.5}, // UA0LW UA4ARH -15
	{Freq: 2110, DT: 1.3 - 0.5}, // VK5BU R7HL 73
	{Freq: 938, DT: 1.8 - 0.5},  // CQ 4S6ARW MJ97
	{Freq: 932, DT: 1.2 - 0.5},  // CQ CO8LY FL20
	{Freq: 888, DT: 1.6 - 0.5},  // VK3TZ UA3ZNQ KO81
	{Freq: 963, DT: 1.4 - 0.5},  // VK3TZ RC7O KN87
}

// ────────────────────────────────────────────────────────────────────────────
// Helper: expand candidate list with freq/DT offsets for search
// ────────────────────────────────────────────────────────────────────────────

// expandCandidates generates a grid of candidates around each reference point
// by varying freq ±freqRange in freqStep Hz and DT ±dtRange in dtStep s.
func expandCandidates(refs []CandidateFreq, freqRange, freqStep, dtRange, dtStep float64) []CandidateFreq {
	var out []CandidateFreq
	for _, ref := range refs {
		for df := -freqRange; df <= freqRange; df += freqStep {
			for ddt := -dtRange; ddt <= dtRange; ddt += dtStep {
				out = append(out, CandidateFreq{
					Freq: ref.Freq + df,
					DT:   ref.DT + ddt,
				})
			}
		}
	}
	return out
}

// ────────────────────────────────────────────────────────────────────────────
// Test: ft8x with provided candidates (known freq/DT from WSJT-X)
// ────────────────────────────────────────────────────────────────────────────

func TestFt8xWAVCapture1ProvidedCandidates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAV decode test (slow)")
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
		t.Fatalf("expected 12000 Hz, got %d Hz", sr)
	}
	t.Logf("Loaded %d samples (%.2f s)", len(samples), float64(len(samples))/float64(sr))

	// Expand candidate search grid around WSJT-X reference positions.
	// ±10 Hz freq, ±0.5 s DT should be enough for DecodeSingle's internal refinement.
	candidates := expandCandidates(capture1Candidates, 10.0, 2.5, 0.5, 0.1)
	t.Logf("Testing %d expanded candidates from %d reference positions", len(candidates), len(capture1Candidates))

	params := DefaultDecodeParams()
	results := Decode(samples, candidates, params)

	correct := 0
	falseDecodes := 0
	t.Logf("Decoded %d message(s):", len(results))
	for _, r := range results {
		match := ""
		msg := strings.TrimSpace(r.Message)
		if wsjtxCapture1[msg] {
			correct++
			match = " ✓"
		} else {
			falseDecodes++
			match = " ✗ (not in reference)"
		}
		t.Logf("  %+6.1f dt  %7.1f Hz  %+5.1f dB  nhard=%d  %s%s",
			r.DT, r.Freq, r.SNR, r.NHardErrors, msg, match)
	}

	t.Logf("Summary: %d correct, %d false, out of %d verified reference",
		correct, falseDecodes, len(wsjtxCapture1))

	// Regression baseline: 7 correct decodes with provided candidates.
	// (SV2SIH ES2AJ, VE1WT K4GBI, SV2SIH KI8JP, CQ PV8AJ,
	//  KB7THX WB9VGJ, A61CK UA1CEI, A61CK W3DQS)
	const minCorrect = 7
	if correct < minCorrect {
		t.Errorf("REGRESSION: capture1 correct = %d, expected >= %d", correct, minCorrect)
	}
	if falseDecodes > 0 {
		t.Logf("NOTE: %d false decode(s) detected", falseDecodes)
	}
}

func TestFt8xWAVCapture2ProvidedCandidates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAV decode test (slow)")
	}

	wavPath := "testdata/ft8test_capture2_20260410.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	samples, sr, err := loadWAV(wavPath)
	if err != nil {
		t.Fatalf("load WAV: %v", err)
	}
	if sr != 12000 {
		t.Fatalf("expected 12000 Hz, got %d Hz", sr)
	}
	t.Logf("Loaded %d samples (%.2f s)", len(samples), float64(len(samples))/float64(sr))

	candidates := expandCandidates(capture2Candidates, 10.0, 2.5, 0.5, 0.1)
	t.Logf("Testing %d expanded candidates from %d reference positions", len(candidates), len(capture2Candidates))

	params := DefaultDecodeParams()
	results := Decode(samples, candidates, params)

	correct := 0
	falseDecodes := 0
	t.Logf("Decoded %d message(s):", len(results))
	for _, r := range results {
		match := ""
		msg := strings.TrimSpace(r.Message)
		if wsjtxCapture2[msg] {
			correct++
			match = " ✓"
		} else {
			falseDecodes++
			match = " ✗ (not in reference)"
		}
		t.Logf("  %+6.1f dt  %7.1f Hz  %+5.1f dB  nhard=%d  %s%s",
			r.DT, r.Freq, r.SNR, r.NHardErrors, msg, match)
	}

	t.Logf("Summary: %d correct, %d false, out of %d verified reference",
		correct, falseDecodes, len(wsjtxCapture2))

	// Regression baseline: 9 correct decodes with provided candidates.
	// (HA5LB, CQ ZS4AW, CQ SV0TPN, CQ Z62NS, VK/ZL4XZ,
	//  VK3ZSJ YO8RQP, R1QD KB2ELA, UY7VV KE6SU, CQ TN8GD)
	const minCorrect = 9
	if correct < minCorrect {
		t.Errorf("REGRESSION: capture2 correct = %d, expected >= %d", correct, minCorrect)
	}
	if falseDecodes > 0 {
		t.Logf("NOTE: %d false decode(s) detected", falseDecodes)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test: ft8x with its own FindCandidates (fully independent)
// ────────────────────────────────────────────────────────────────────────────

func TestFt8xWAVCapture1OwnCandidates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAV decode test (slow — brute-force candidate scan)")
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
		t.Fatalf("expected 12000 Hz, got %d Hz", sr)
	}
	t.Logf("Loaded %d samples (%.2f s)", len(samples), float64(len(samples))/float64(sr))

	// Use ft8x's own candidate finder.
	t.Log("Running FindCandidates (200-2600 Hz, -0.5 to +2.5 s)...")
	candidates := FindCandidates(samples, 200, 2600, -0.5, 2.5)
	t.Logf("Found %d candidates", len(candidates))

	// Limit to top N candidates by sync power (already sorted by FindCandidates).
	maxCandidates := 200
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}

	params := DefaultDecodeParams()
	results := Decode(samples, candidates, params)

	correct := 0
	falseDecodes := 0
	t.Logf("Decoded %d message(s):", len(results))
	for _, r := range results {
		match := ""
		msg := strings.TrimSpace(r.Message)
		if wsjtxCapture1[msg] {
			correct++
			match = " ✓"
		} else {
			falseDecodes++
			match = " ✗ (not in reference)"
		}
		t.Logf("  %+6.1f dt  %7.1f Hz  %+5.1f dB  nhard=%d  %s%s",
			r.DT, r.Freq, r.SNR, r.NHardErrors, msg, match)
	}

	t.Logf("Summary: %d correct, %d false, out of %d verified reference",
		correct, falseDecodes, len(wsjtxCapture1))
}

func TestFt8xWAVCapture2OwnCandidates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAV decode test (slow — brute-force candidate scan)")
	}

	wavPath := "testdata/ft8test_capture2_20260410.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	samples, sr, err := loadWAV(wavPath)
	if err != nil {
		t.Fatalf("load WAV: %v", err)
	}
	if sr != 12000 {
		t.Fatalf("expected 12000 Hz, got %d Hz", sr)
	}
	t.Logf("Loaded %d samples (%.2f s)", len(samples), float64(len(samples))/float64(sr))

	t.Log("Running FindCandidates (200-2600 Hz, -0.5 to +2.5 s)...")
	candidates := FindCandidates(samples, 200, 2600, -0.5, 2.5)
	t.Logf("Found %d candidates", len(candidates))

	maxCandidates := 200
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}

	params := DefaultDecodeParams()
	results := Decode(samples, candidates, params)

	correct := 0
	falseDecodes := 0
	t.Logf("Decoded %d message(s):", len(results))
	for _, r := range results {
		match := ""
		msg := strings.TrimSpace(r.Message)
		if wsjtxCapture2[msg] {
			correct++
			match = " ✓"
		} else {
			falseDecodes++
			match = " ✗ (not in reference)"
		}
		t.Logf("  %+6.1f dt  %7.1f Hz  %+5.1f dB  nhard=%d  %s%s",
			r.DT, r.Freq, r.SNR, r.NHardErrors, msg, match)
	}

	t.Logf("Summary: %d correct, %d false, out of %d verified reference",
		correct, falseDecodes, len(wsjtxCapture2))
}

// ────────────────────────────────────────────────────────────────────────────
// Test: single-candidate decode (diagnostic — test one specific signal)
// ────────────────────────────────────────────────────────────────────────────

func TestFt8xSingleCandidateCapture1(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAV decode test")
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
		t.Fatalf("expected 12000 Hz, got %d Hz", sr)
	}

	// Test each WSJT-X reference candidate individually with a fine search grid.
	params := DefaultDecodeParams()
	ds := NewDownsampler()
	firstCall := true

	for i, ref := range capture1Candidates {
		// Try a grid of freq ± 5 Hz and DT ± 0.3 s around the reference.
		var bestResult *DecodeCandidate
		for df := -5.0; df <= 5.0; df += 1.0 {
			for ddt := -0.3; ddt <= 0.3; ddt += 0.05 {
				freq := ref.Freq + df
				dt := ref.DT + ddt
				// Only compute the 192k FFT on the very first call.
				newdat := firstCall
				firstCall = false
				result, ok := DecodeSingle(samples, ds, freq, dt, newdat, params)
				if ok {
					if bestResult == nil || result.NHardErrors < bestResult.NHardErrors {
						r := result
						bestResult = &r
					}
					break // found a decode at this freq
				}
			}
			if bestResult != nil {
				break
			}
		}

		if bestResult != nil {
			status := "✓"
			if !wsjtxCapture1[strings.TrimSpace(bestResult.Message)] {
				status = "✗ MISMATCH"
			}
			t.Logf("  [%2d] ref=%7.1f Hz dt=%+.1f → %s  %7.1f Hz  %+5.1f dB  %s",
				i+1, ref.Freq, ref.DT, status, bestResult.Freq, bestResult.SNR,
				strings.TrimSpace(bestResult.Message))
		} else {
			t.Logf("  [%2d] ref=%7.1f Hz dt=%+.1f → ❌ no decode", i+1, ref.Freq, ref.DT)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test: Bluestein FFT accuracy for 192000-point transform
// ────────────────────────────────────────────────────────────────────────────

func TestBluesteinFFT192k(t *testing.T) {
	// Verify that a 192000-point FFT of a known sinusoid produces the
	// expected peak at the right bin.
	const n = 192000
	const freq = 1000.0 // Hz
	const sr = 12000.0

	input := make([]float32, n)
	for i := range input {
		input[i] = float32(math.Sin(2 * math.Pi * freq * float64(i) / sr))
	}

	cx := RealFFT(input, n)
	if len(cx) != n/2+1 {
		t.Fatalf("expected %d bins, got %d", n/2+1, len(cx))
	}

	// Find peak bin.
	df := sr / float64(n) // ~0.0625 Hz per bin
	expectedBin := int(math.Round(freq / df))

	peakBin := 0
	peakPow := 0.0
	for i, v := range cx {
		pow := real(v)*real(v) + imag(v)*imag(v)
		if pow > peakPow {
			peakPow = pow
			peakBin = i
		}
	}

	if peakBin != expectedBin {
		t.Errorf("peak at bin %d (%.1f Hz), expected bin %d (%.1f Hz)",
			peakBin, float64(peakBin)*df, expectedBin, float64(expectedBin)*df)
	} else {
		t.Logf("192k-point FFT: peak at bin %d (%.2f Hz) ✓", peakBin, float64(peakBin)*df)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test: DecodeIterative (multi-pass with signal subtraction)
// ────────────────────────────────────────────────────────────────────────────

func TestFt8xWAVIterativeCapture1(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAV decode test (slow)")
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
		t.Fatalf("expected 12000 Hz, got %d Hz", sr)
	}
	t.Logf("Loaded %d samples (%.2f s)", len(samples), float64(len(samples))/float64(sr))

	params := DefaultDecodeParams()
	results := DecodeIterative(samples, params, 200, 2600)

	correct := 0
	falseDecodes := 0
	t.Logf("Decoded %d message(s):", len(results))
	for _, r := range results {
		match := ""
		msg := strings.TrimSpace(r.Message)
		if wsjtxCapture1[msg] {
			correct++
			match = " ✓"
		} else {
			falseDecodes++
			match = " ✗ (not in reference)"
		}
		t.Logf("  %+6.1f dt  %7.1f Hz  %+5.1f dB  nhard=%d  ap=%d  %s%s",
			r.DT, r.Freq, r.SNR, r.NHardErrors, r.APType, msg, match)
	}

	t.Logf("Summary: %d correct, %d false, out of %d verified reference",
		correct, falseDecodes, len(wsjtxCapture1))

	const minCorrect = 8
	if correct < minCorrect {
		t.Errorf("REGRESSION: capture1 iterative correct = %d, expected >= %d", correct, minCorrect)
	}
}

func TestFt8xWAVIterativeCapture2(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAV decode test (slow)")
	}

	wavPath := "testdata/ft8test_capture2_20260410.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	samples, sr, err := loadWAV(wavPath)
	if err != nil {
		t.Fatalf("load WAV: %v", err)
	}
	if sr != 12000 {
		t.Fatalf("expected 12000 Hz, got %d Hz", sr)
	}
	t.Logf("Loaded %d samples (%.2f s)", len(samples), float64(len(samples))/float64(sr))

	params := DefaultDecodeParams()
	results := DecodeIterative(samples, params, 200, 2600)

	correct := 0
	falseDecodes := 0
	t.Logf("Decoded %d message(s):", len(results))
	for _, r := range results {
		match := ""
		msg := strings.TrimSpace(r.Message)
		if wsjtxCapture2[msg] {
			correct++
			match = " ✓"
		} else {
			falseDecodes++
			match = " ✗ (not in reference)"
		}
		t.Logf("  %+6.1f dt  %7.1f Hz  %+5.1f dB  nhard=%d  ap=%d  %s%s",
			r.DT, r.Freq, r.SNR, r.NHardErrors, r.APType, msg, match)
	}

	t.Logf("Summary: %d correct, %d false, out of %d verified reference",
		correct, falseDecodes, len(wsjtxCapture2))

	const minCorrect = 11
	if correct < minCorrect {
		t.Errorf("REGRESSION: capture2 iterative correct = %d, expected >= %d", correct, minCorrect)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test: Capture 3 (capture.wav) — FTdx10 DATA-mode capture
// ────────────────────────────────────────────────────────────────────────────

func TestFt8xWAVCapture3ProvidedCandidates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAV decode test (slow)")
	}

	wavPath := "testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	samples, sr, err := loadWAV(wavPath)
	if err != nil {
		t.Fatalf("load WAV: %v", err)
	}
	if sr != 12000 {
		t.Fatalf("expected 12000 Hz, got %d Hz", sr)
	}
	t.Logf("Loaded %d samples (%.2f s)", len(samples), float64(len(samples))/float64(sr))

	candidates := expandCandidates(capture3Candidates, 10.0, 2.5, 0.5, 0.1)
	t.Logf("Testing %d expanded candidates from %d reference positions", len(candidates), len(capture3Candidates))

	params := DefaultDecodeParams()
	results := Decode(samples, candidates, params)

	correct := 0
	falseDecodes := 0
	t.Logf("Decoded %d message(s):", len(results))
	for _, r := range results {
		match := ""
		msg := strings.TrimSpace(r.Message)
		if wsjtxCapture3[msg] {
			correct++
			match = " ✓"
		} else {
			falseDecodes++
			match = " ✗ (not in reference)"
		}
		t.Logf("  %+6.1f dt  %7.1f Hz  %+5.1f dB  nhard=%d  %s%s",
			r.DT, r.Freq, r.SNR, r.NHardErrors, msg, match)
	}

	t.Logf("Summary: %d correct, %d false, out of %d verified reference",
		correct, falseDecodes, len(wsjtxCapture3))

	// Regression baseline: 14 correct decodes with provided candidates.
	const minCorrect = 14
	if correct < minCorrect {
		t.Errorf("REGRESSION: capture3 correct = %d, expected >= %d", correct, minCorrect)
	}
	if falseDecodes > 0 {
		t.Logf("NOTE: %d false decode(s) detected", falseDecodes)
	}
}

func TestFt8xWAVCapture3OwnCandidates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAV decode test (slow — brute-force candidate scan)")
	}

	wavPath := "testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	samples, sr, err := loadWAV(wavPath)
	if err != nil {
		t.Fatalf("load WAV: %v", err)
	}
	if sr != 12000 {
		t.Fatalf("expected 12000 Hz, got %d Hz", sr)
	}
	t.Logf("Loaded %d samples (%.2f s)", len(samples), float64(len(samples))/float64(sr))

	t.Log("Running FindCandidates (200-2600 Hz, -0.5 to +2.5 s)...")
	candidates := FindCandidates(samples, 200, 2600, -0.5, 2.5)
	t.Logf("Found %d candidates", len(candidates))

	maxCandidates := 200
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}

	params := DefaultDecodeParams()
	results := Decode(samples, candidates, params)

	correct := 0
	falseDecodes := 0
	t.Logf("Decoded %d message(s):", len(results))
	for _, r := range results {
		match := ""
		msg := strings.TrimSpace(r.Message)
		if wsjtxCapture3[msg] {
			correct++
			match = " ✓"
		} else {
			falseDecodes++
			match = " ✗ (not in reference)"
		}
		t.Logf("  %+6.1f dt  %7.1f Hz  %+5.1f dB  nhard=%d  %s%s",
			r.DT, r.Freq, r.SNR, r.NHardErrors, msg, match)
	}

	t.Logf("Summary: %d correct, %d false, out of %d verified reference",
		correct, falseDecodes, len(wsjtxCapture3))
}

func TestFt8xWAVIterativeCapture3(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAV decode test (slow)")
	}

	wavPath := "testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	samples, sr, err := loadWAV(wavPath)
	if err != nil {
		t.Fatalf("load WAV: %v", err)
	}
	if sr != 12000 {
		t.Fatalf("expected 12000 Hz, got %d Hz", sr)
	}
	t.Logf("Loaded %d samples (%.2f s)", len(samples), float64(len(samples))/float64(sr))

	params := DefaultDecodeParams()
	results := DecodeIterative(samples, params, 200, 2600)

	correct := 0
	falseDecodes := 0
	t.Logf("Decoded %d message(s):", len(results))
	for _, r := range results {
		match := ""
		msg := strings.TrimSpace(r.Message)
		if wsjtxCapture3[msg] {
			correct++
			match = " ✓"
		} else {
			falseDecodes++
			match = " ✗ (not in reference)"
		}
		t.Logf("  %+6.1f dt  %7.1f Hz  %+5.1f dB  nhard=%d  ap=%d  %s%s",
			r.DT, r.Freq, r.SNR, r.NHardErrors, r.APType, msg, match)
	}

	t.Logf("Summary: %d correct, %d false, out of %d verified reference",
		correct, falseDecodes, len(wsjtxCapture3))

	// Regression baseline: 16 correct decodes with iterative decode.
	const minCorrect = 16
	if correct < minCorrect {
		t.Errorf("REGRESSION: capture3 iterative correct = %d, expected >= %d", correct, minCorrect)
	}
}
