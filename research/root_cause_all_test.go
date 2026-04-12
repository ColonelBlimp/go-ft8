// root_cause_all_test.go — Root cause analysis for ALL three captures.
//
// For each capture: runs the research iterative decode pipeline, identifies
// missing signals, and classifies each by failure mode:
//
//   sync_fail      — Hard sync (nsync ≤ 6) blocks decode even at exact params
//   ldpc_fail      — Reaches LDPC but fails to converge (weak LLRs)
//   ap_limitation  — Non-CQ signal needing AP type ≥2 with callsign knowledge
//   masked         — CQ signal blocked by nearby strong signal (<10 Hz)
//   subtraction    — Signal present with provided candidates but lost in iterative
//   dt_miss        — Sync8 returns wrong DT; correct DT does decode
//
// This test does NOT modify production code. It uses the research pipeline
// (research sync8 + DecodeSingle + SubtractFT8) to diagnose.

package research

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

// refSignal describes a single WSJT-X reference decode.
type refSignal struct {
	Label  string
	Freq   float64
	Ft8bDT float64 // ft8b-convention xdt (= WSJT-X DT + 0.5)
	IsCQ   bool
}

// captureSpec bundles a WAV file with its WSJT-X reference decodes.
type captureSpec struct {
	Name    string
	WAVPath string
	Refs    []refSignal
}

// allCaptures defines the three test captures.
// DT values are ft8b convention (xdt = WSJT-X ALL.TXT DT + 0.5).
var allCaptures = []captureSpec{
	{
		Name:    "Capture 1",
		WAVPath: "../testdata/ft8test_capture_20260410.wav",
		Refs: []refSignal{
			// DTs from WSJT-X ALL.TXT (display DT used directly as ft8b_xdt)
			{"SV2SIH ES2AJ -16", 1860.5, -0.6, false},
			{"VE1WT K4GBI 73", 1309.9, -0.4, false},
			{"SV2SIH KI8JP -10", 1902.9, -0.5, false},
			{"CQ PV8AJ FJ92", 1691.8, -0.5, true},
			{"<...> RA1OHX KP91", 2098.6, -0.4, false},
			{"KB7THX WB9VGJ RR73", 2328.0, -0.5, false},
			{"A61CK UA1CEI KP50", 948.2, -0.6, false},
			{"<...> LU3DXU GF05", 1273.0, -0.6, false},
			{"<...> RA6ABC KN96", 1814.0, -0.5, false},
			{"ES2AJ UA3LAR KO75", 835.0, -0.6, false},
			{"A61CK W3DQS -12", 579.1, -0.4, false},
			{"HZ1TT RU1AB R-10", 2208.4, 0.6, false},
			{"<...> RV6ASU KN94", 460.9, -0.3, false},
		},
	},
	{
		Name:    "Capture 2",
		WAVPath: "../testdata/ft8test_capture2_20260410.wav",
		Refs: []refSignal{
			// DTs from WSJT-X ALL.TXT (display DT used directly as ft8b_xdt)
			{"HA5LB 5B4AMX RR73", 815.6, 2.4, false},
			{"CQ ZS4AW KG31", 1776.0, 1.9, true},
			{"CQ SV0TPN KM28", 2100.0, 1.9, true},
			{"CQ Z62NS KN02", 745.6, 1.8, true},
			{"VK/ZL4XZ <...> RR73", 331.8, 1.8, false},
			{"VK3ZSJ YO8RQP KN37", 862.5, 1.9, false},
			{"R1QD KB2ELA -12", 1463.8, 1.8, false},
			{"UY7VV KE6SU DM14", 553.0, 1.9, false},
			{"TL8GD UT2VX KN69", 319.0, 1.7, false},
			{"RU4LM 4X5JK R-14", 1840.0, 1.6, false},
			{"JT1CO IZ7DIO 73", 1768.0, 1.7, false},
			{"VK3ZSJ US7KC KO21", 1502.0, 1.9, false},
			{"JR3UIC SP7IIT RR73", 1410.0, 1.8, false},
			{"JT1CO YO3HST KN24", 1096.0, 1.9, false},
			{"CQ TN8GD JI75", 451.0, 1.7, true},
		},
	},
	{
		Name:    "Capture 3",
		WAVPath: "../testdata/capture.wav",
		Refs: []refSignal{
			// DTs from WSJT-X ALL.TXT (display DT used directly as ft8b_xdt)
			{"VK5BU RT6C KN95", 2454, 1.3, false},
			{"CQ V4/SP9FIH", 298, 1.3, true},
			{"A61OK US1VM KN68", 568, 1.5, false},
			{"CQ KB3Z FN20", 1100, 1.5, true},
			{"CQ NH6D BL02", 2251, 1.4, true},
			{"VK3TZ UX2QX R-07", 1776, 1.3, false},
			{"UB8CSR SV0TPN -18", 1998, 1.7, false},
			{"R8AGW VU2FR RR73", 2507, 2.0, false},
			{"<...> JA8RVP QN23", 1001, 1.4, false},
			{"CQ UR5QW KN77", 2401, 1.4, true},
			{"UA0LW RK6AAC KN95", 1149, 1.6, false},
			{"CQ SP4MSY KO13", 1250, 1.2, true},
			{"VK5ATH VK0DS -01", 1648, 1.4, false},
			{"VK5BU RG5A KN93", 533, 1.4, false},
			{"5Z4VJ YB1RUS OI33", 1505, 1.3, false},
			{"UA0LW UA4ARH -15", 1211, 1.5, false},
			{"VK5BU R7HL 73", 2110, 1.3, false},
			{"CQ 4S6ARW MJ97", 938, 1.8, true},
			{"CQ CO8LY FL20", 932, 1.2, true},
			{"VK3TZ UA3ZNQ KO81", 888, 1.6, false},
			{"VK3TZ RC7O KN87", 963, 1.4, false},
		},
	},
}

// TestRootCauseAllCaptures runs root cause analysis on all three captures.
func TestRootCauseAllCaptures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping root cause analysis (slow)")
	}

	for _, capt := range allCaptures {
		t.Run(capt.Name, func(t *testing.T) {
			analyseCapture(t, capt)
		})
	}
}

func analyseCapture(t *testing.T, cap captureSpec) {
	if _, err := os.Stat(cap.WAVPath); os.IsNotExist(err) {
		t.Skipf("WAV not found: %s", cap.WAVPath)
	}

	_, dd, err := loadIwave(cap.WAVPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// Normalised copy for ft8x functions (which expect /32768).
	ddNorm := make([]float32, NMAX)
	for i := 0; i < NMAX; i++ {
		ddNorm[i] = dd[i] / 32768.0
	}

	// ── Phase 1: Run research iterative pipeline ─────────────────────
	ddWork := make([]float32, NMAX)
	copy(ddWork, ddNorm)

	const (
		nfa     = 200
		nfb     = 2600
		syncmin = 1.3
		nfqso   = 0
		maxcand = 600
		npass   = 3
		ndepth  = 3
	)

	type decodedSignal struct {
		msg   string
		freq  float64
		xdt   float64
		snr   float64
		tones [NN]int
		pass  int
	}

	var allDecodes []decodedSignal
	seen := make(map[string]bool)
	ndecodes := 0
	n2 := 0

	for ipass := 1; ipass <= npass; ipass++ {
		ndeep := ndepth
		switch ipass {
		case 1:
			if ndepth == 3 {
				ndeep = 2
			}
		case 2:
			n2 = ndecodes
			if ndecodes == 0 {
				continue
			}
		case 3:
			if ndecodes-n2 == 0 {
				continue
			}
		}

		var ddArr [NMAX]float32
		copy(ddArr[:], ddWork)
		resCands, _ := Sync8(ddArr, NMAX, nfa, nfb, syncmin, nfqso, maxcand)

		ft8xCands := make([]CandidateFreq, len(resCands))
		for i, c := range resCands {
			ft8xCands[i] = CandidateFreq{
				Freq:      c.Freq,
				DT:        c.DT,
				SyncPower: c.SyncPower,
			}
		}
		// Fortran uses MAXCAND=600 for all passes (ft8_decode.f90 line 194)
		if len(ft8xCands) > 600 {
			ft8xCands = ft8xCands[:600]
		}

		params := DecodeParams{
			Depth:     ndeep,
			APEnabled: true,
			APCQOnly:  true,
			APWidth:   25.0,
		}

		ds := NewDownsampler()
		passDecodes := 0

		for i, cand := range ft8xCands {
			newdat := (i == 0)
			result, ok := DecodeSingle(ddWork, ds, cand.Freq, cand.DT, newdat, params)
			if !ok {
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

			allDecodes = append(allDecodes, decodedSignal{
				msg: msg, freq: result.Freq, xdt: result.DT,
				snr: result.SNR, tones: result.Tones, pass: ipass,
			})

			SubtractFT8(ddWork, result.Tones, result.Freq, result.DT)
		}
		t.Logf("Pass %d: %d new decodes (total: %d, ndeep=%d, cands=%d)",
			ipass, passDecodes, ndecodes, ndeep, len(ft8xCands))
	}

	// ── Phase 2: Identify missing signals ────────────────────────────
	t.Logf("")
	t.Logf("═══════════════════════════════════════════════════════════")
	t.Logf("%s — %d decoded / %d reference", cap.Name, ndecodes, len(cap.Refs))
	t.Logf("═══════════════════════════════════════════════════════════")

	// Build ref→decoded map
	refMap := make(map[string]bool)
	for _, r := range cap.Refs {
		refMap[r.Label] = false
	}
	for _, d := range allDecodes {
		if _, ok := refMap[d.msg]; ok {
			refMap[d.msg] = true
		}
	}

	correct := 0
	for _, v := range refMap {
		if v {
			correct++
		}
	}
	t.Logf("Correct: %d / %d", correct, len(cap.Refs))
	t.Logf("")

	// Show decoded signals
	t.Logf("Decoded signals:")
	for _, d := range allDecodes {
		tag := ""
		if _, ok := refMap[d.msg]; ok {
			tag = " ✓"
		}
		t.Logf("  pass=%d  %+6.1f dt  %7.1f Hz  %+5.1f dB  %s%s",
			d.pass, d.xdt, d.freq, d.snr, d.msg, tag)
	}

	// ── Phase 3: Classify each missing signal ────────────────────────
	var missing []refSignal
	for _, ref := range cap.Refs {
		if !refMap[ref.Label] {
			missing = append(missing, ref)
		}
	}

	if len(missing) == 0 {
		t.Logf("\nAll reference signals decoded!")
		return
	}

	t.Logf("")
	t.Logf("═══════════════════════════════════════════════════════════")
	t.Logf("MISSING SIGNAL ANALYSIS (%d signals)", len(missing))
	t.Logf("═══════════════════════════════════════════════════════════")

	// Use cleaned audio (after subtracting all decoded signals)
	ddClean := ddWork // already cleaned by the iterative pipeline

	var ctwkZero [32]complex128
	for i := range ctwkZero {
		ctwkZero[i] = complex(1, 0)
	}

	for _, ref := range missing {
		t.Logf("")
		t.Logf("── %s (%.0f Hz, ft8b_xdt=%.2f, CQ=%v) ──", ref.Label, ref.Freq, ref.Ft8bDT, ref.IsCQ)

		// ── Check 1: sync quality at exact params on ORIGINAL audio ──
		dsOrig := NewDownsampler()
		newdat := true
		cd0 := dsOrig.Downsample(ddNorm, &newdat, ref.Freq)

		i0 := int(math.Round((ref.Ft8bDT + 0.5) * Fs2))
		smaxOrig := 0.0
		ibestOrig := i0
		for idt := i0 - 20; idt <= i0+20; idt++ {
			sync := Sync8d(cd0, idt, ctwkZero, 0)
			if sync > smaxOrig {
				smaxOrig = sync
				ibestOrig = idt
			}
		}
		_, s8Orig := ComputeSymbolSpectra(cd0, ibestOrig)
		nsyncOrig := HardSync(&s8Orig)

		// ── Check 2: sync quality on CLEANED audio ──
		dsClean := NewDownsampler()
		newdat2 := true
		cd0C := dsClean.Downsample(ddClean, &newdat2, ref.Freq)

		smaxClean := 0.0
		ibestClean := i0
		for idt := i0 - 20; idt <= i0+20; idt++ {
			sync := Sync8d(cd0C, idt, ctwkZero, 0)
			if sync > smaxClean {
				smaxClean = sync
				ibestClean = idt
			}
		}
		_, s8Clean := ComputeSymbolSpectra(cd0C, ibestClean)
		nsyncClean := HardSync(&s8Clean)

		// ── Check 3: DecodeSingle at exact params (original) ──
		dsD1 := NewDownsampler()
		_, okOrig := DecodeSingle(ddNorm, dsD1, ref.Freq, ref.Ft8bDT, true, DecodeParams{
			Depth: 3, APEnabled: true, APCQOnly: true, APWidth: 25.0,
		})

		// ── Check 4: DecodeSingle at exact params (cleaned) ──
		dsD2 := NewDownsampler()
		resultClean, okClean := DecodeSingle(ddClean, dsD2, ref.Freq, ref.Ft8bDT, true, DecodeParams{
			Depth: 3, APEnabled: true, APCQOnly: true, APWidth: 25.0,
		})

		// ── Check 5: Broad search on cleaned audio (±10Hz, ±0.5s) ──
		foundBroad := false
		broadMsg := ""
		broadFreq := 0.0
		broadDT := 0.0
		for df := -10.0; df <= 10.0; df += 1.0 {
			for ddt := -0.5; ddt <= 0.5; ddt += 0.1 {
				dsB := NewDownsampler()
				r, ok := DecodeSingle(ddClean, dsB, ref.Freq+df, ref.Ft8bDT+ddt, true, DecodeParams{
					Depth: 3, APEnabled: true, APCQOnly: true, APWidth: 25.0,
				})
				if ok {
					foundBroad = true
					broadMsg = strings.TrimSpace(r.Message)
					broadFreq = ref.Freq + df
					broadDT = ref.Ft8bDT + ddt
					break
				}
			}
			if foundBroad {
				break
			}
		}

		// ── Check 6: nearby decoded signals ──
		var nearby []string
		for _, d := range allDecodes {
			if math.Abs(d.freq-ref.Freq) < 20 {
				nearby = append(nearby, fmt.Sprintf("%s@%.0fHz", d.msg, d.freq))
			}
		}

		// ── Classify failure mode ──
		failureMode := classifyFailure(ref, nsyncOrig, nsyncClean, okOrig, okClean, foundBroad, broadMsg, nearby)

		// ── Report ──
		t.Logf("  Original: sync=%.1f ibest=%d nsync=%d decode=%v", smaxOrig, ibestOrig, nsyncOrig, okOrig)
		t.Logf("  Cleaned:  sync=%.1f ibest=%d nsync=%d decode=%v", smaxClean, ibestClean, nsyncClean, okClean)
		if okClean {
			t.Logf("    → decoded on clean: %s", strings.TrimSpace(resultClean.Message))
		}
		t.Logf("  Broad search (±10Hz/±0.5s clean): %v", foundBroad)
		if foundBroad {
			t.Logf("    → %s at %.0f Hz, xdt=%.2f", broadMsg, broadFreq, broadDT)
		}
		if len(nearby) > 0 {
			t.Logf("  Nearby decoded (<20 Hz): %v", nearby)
		}
		t.Logf("  ▶ Failure mode: %s", failureMode)
	}

	// ── Summary table ──
	t.Logf("")
	t.Logf("═══════════════════════════════════════════════════════════")
	t.Logf("SUMMARY: %s — %d / %d decoded, %d missing", cap.Name, correct, len(cap.Refs), len(missing))
	t.Logf("═══════════════════════════════════════════════════════════")
	for _, ref := range missing {
		t.Logf("  ✗ %-35s  %7.0f Hz  CQ=%v", ref.Label, ref.Freq, ref.IsCQ)
	}
}

// classifyFailure determines the root cause of a decode failure.
func classifyFailure(ref refSignal, nsyncOrig, nsyncClean int, okOrig, okClean, foundBroad bool, broadMsg string, nearby []string) string {
	// If it decodes on cleaned audio at exact params, it was masked by interference
	if okClean {
		return "subtraction_needed — decodes on cleaned audio"
	}

	// If broad search finds it on cleaned audio, DT/freq from sync8 was wrong
	if foundBroad {
		if broadMsg == ref.Label {
			return "dt_miss — correct signal found at different freq/DT on cleaned audio"
		}
		return fmt.Sprintf("dt_miss — broad search finds '%s' (different signal?)", broadMsg)
	}

	// If hard sync fails even on cleaned audio, signal is too weak for sync
	if nsyncClean <= 6 && nsyncOrig <= 6 {
		if !ref.IsCQ {
			return "sync_fail + ap_limitation — nsync≤6 and non-CQ (needs AP type ≥2)"
		}
		return "sync_fail — nsync≤6 even on cleaned audio; signal too weak for sync"
	}

	// Sync passes but LDPC fails
	if !ref.IsCQ {
		return "ldpc_fail + ap_limitation — sync OK but LDPC fails; non-CQ needs AP type ≥2"
	}

	if len(nearby) > 0 {
		return fmt.Sprintf("masked — CQ signal with nearby interference: %v", nearby)
	}

	return "ldpc_fail — sync OK but LDPC fails; weak signal at threshold edge"
}
