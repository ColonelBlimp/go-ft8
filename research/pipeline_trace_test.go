// pipeline_trace_test.go — Deep diagnostic: trace each of the 5 missing
// signals through every stage of the decode pipeline.
//
// For each missing signal we force-feed the exact WSJT-X reference freq/DT
// into DecodeSingle and instrument each intermediate step to find where
// the decode fails. This answers: is the gap a Go vs Fortran numerical
// issue, or a logic/algorithm difference?
//
// The 5 missing signals from capture.wav (16/21, missing 5):
//   1. UA0LW UA4ARH -15     1211 Hz  DT=+1.0
//   2. VK3TZ UA3ZNQ KO81     888 Hz  DT=+1.1
//   3. 5Z4VJ YB1RUS OI33    1505 Hz  DT=+0.8
//   4. VK3TZ RC7O KN87       963 Hz  DT=+0.9
//   5. CQ CO8LY FL20         932 Hz  DT=+0.7

package research

import (
	"math"
	"os"
	"strings"
	"testing"

	ft8x "github.com/ColonelBlimp/go-ft8"
)

// missingRef holds the 5 signals WSJT-X decodes but we don't.
var missingRef = []struct {
	Label string
	Freq  float64
	DT    float64 // WSJT-X DT (not xdt)
}{
	{"UA0LW UA4ARH -15", 1211, 1.5},
	{"VK3TZ UA3ZNQ KO81", 888, 1.6},
	{"5Z4VJ YB1RUS OI33", 1505, 1.3},
	{"VK3TZ RC7O KN87", 963, 1.4},
	{"CQ CO8LY FL20", 932, 1.2},
}

// TestPipelineTrace feeds each missing signal's exact WSJT-X freq/DT into
// DecodeSingle and traces where it fails:
//
//	Stage 1: Downsampling
//	Stage 2: Coarse time search (sync8d)
//	Stage 3: Fine frequency search
//	Stage 4: Refined time offset
//	Stage 5: Symbol spectra + hard sync
//	Stage 6: Soft metrics → LDPC
func TestPipelineTrace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping pipeline trace (slow)")
	}

	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// Convert to normalized float32 (production pipeline convention).
	ddNorm := make([]float32, NMAX)
	for i := 0; i < NMAX; i++ {
		ddNorm[i] = dd[i] / 32768.0
	}

	for _, sig := range missingRef {
		t.Logf("")
		t.Logf("═══════════════════════════════════════════════════════════════")
		t.Logf("  SIGNAL: %-30s  freq=%.1f Hz  DT=%.1f s", sig.Label, sig.Freq, sig.DT)
		t.Logf("═══════════════════════════════════════════════════════════════")

		// Compute xdt convention: xdt = WSJT-X DT - 0.5
		xdt := sig.DT - 0.5

		// ── Try DecodeSingle at exact WSJT-X parameters first ──
		ds := ft8x.NewDownsampler()
		params := ft8x.DecodeParams{
			Depth:     3,
			APEnabled: true,
			APCQOnly:  true,
			APWidth:   25.0,
		}
		result, ok := ft8x.DecodeSingle(ddNorm, ds, sig.Freq, xdt, true, params)
		if ok {
			t.Logf("  ✓ DecodeSingle SUCCESS at exact ref params: %s", strings.TrimSpace(result.Message))
			continue
		}
		t.Logf("  ✗ DecodeSingle FAILED at exact ref (freq=%.1f, xdt=%.2f)", sig.Freq, xdt)

		// ── Try a grid search around the reference freq/DT ──
		t.Logf("  ── Grid search: ±5 Hz, ±0.5 s ──")
		bestMsg := ""
		bestFreq := 0.0
		bestDT := 0.0
		bestSync := 0.0
		for df := -5.0; df <= 5.0; df += 1.0 {
			for ddt := -0.5; ddt <= 0.5; ddt += 0.1 {
				f := sig.Freq + df
				xd := xdt + ddt
				ds2 := ft8x.NewDownsampler()
				r, ok2 := ft8x.DecodeSingle(ddNorm, ds2, f, xd, true, params)
				if ok2 {
					bestMsg = strings.TrimSpace(r.Message)
					bestFreq = f
					bestDT = xd
					bestSync = r.SNR
				}
			}
		}
		if bestMsg != "" {
			t.Logf("  ✓ Grid search found: %s at freq=%.1f xdt=%.2f snr=%.1f", bestMsg, bestFreq, bestDT, bestSync)
		} else {
			t.Logf("  ✗ Grid search FAILED (no decode in ±5Hz/±0.5s)")
		}

		// ── Trace the pipeline stages manually ──
		t.Logf("  ── Pipeline stage trace at exact ref ──")

		// Stage 1: Downsample
		ds3 := ft8x.NewDownsampler()
		newdat := true
		cd0 := ds3.Downsample(ddNorm, &newdat, sig.Freq)
		t.Logf("  Stage 1 (downsample): cd0 len=%d", len(cd0))

		// Check energy in downsampled signal
		energy := 0.0
		for _, z := range cd0 {
			r, im := real(z), imag(z)
			energy += r*r + im*im
		}
		t.Logf("    cd0 total energy: %.2e  mean energy/sample: %.2e", energy, energy/float64(len(cd0)))

		// Stage 2: Coarse time search
		i0 := int(math.Round((xdt + 0.5) * ft8x.Fs2))
		var ctwkZero [32]complex128
		for i := range ctwkZero {
			ctwkZero[i] = complex(1, 0)
		}

		smax := 0.0
		ibest := i0
		for idt := i0 - 20; idt <= i0+20; idt++ {
			sync := ft8x.Sync8d(cd0, idt, ctwkZero, 0)
			if sync > smax {
				smax = sync
				ibest = idt
			}
		}
		t.Logf("  Stage 2 (coarse sync): i0=%d → ibest=%d  smax=%.4f", i0, ibest, smax)

		// Also scan full range for comparison
		smaxFull := 0.0
		ibestFull := 0
		for idt := 0; idt <= ft8x.NP2; idt += 2 {
			sync := ft8x.Sync8d(cd0, idt, ctwkZero, 0)
			if sync > smaxFull {
				smaxFull = sync
				ibestFull = idt
			}
		}
		t.Logf("    Full range scan: ibest=%d  smax=%.4f  (xdt=%.3f)", ibestFull, smaxFull, float64(ibestFull-1)*ft8x.Dt2)

		// Stage 3: Fine frequency search
		smax2 := 0.0
		delfBest := 0.0
		twopi := 2.0 * math.Pi
		for ifr := -5; ifr <= 5; ifr++ {
			delf := float64(ifr) * 0.5
			dphi := twopi * delf * ft8x.Dt2
			phi := 0.0
			var ctwk [32]complex128
			for i := 0; i < 32; i++ {
				ctwk[i] = complex(math.Cos(phi), math.Sin(phi))
				phi = math.Mod(phi+dphi, twopi)
			}
			sync := ft8x.Sync8d(cd0, ibest, ctwk, 1)
			if sync > smax2 {
				smax2 = sync
				delfBest = delf
			}
		}
		t.Logf("  Stage 3 (fine freq): delf=%.1f Hz  sync=%.4f  → f1=%.1f Hz", delfBest, smax2, sig.Freq+delfBest)

		// Stage 4: Apply freq correction and re-downsample
		var a [5]float64
		a[0] = -delfBest
		cd0corr := ft8x.TwkFreq1(cd0, ft8x.Fs2, a)
		f1 := sig.Freq + delfBest

		newdat2 := false
		cd0corr = ds3.Downsample(ddNorm, &newdat2, f1)

		// Refine time
		ss := [9]float64{}
		for idt := -4; idt <= 4; idt++ {
			sync := ft8x.Sync8d(cd0corr, ibest+idt, ctwkZero, 0)
			ss[idt+4] = sync
		}
		bestIdx := 0
		for i, v := range ss {
			if v > ss[bestIdx] {
				bestIdx = i
			}
		}
		ibest = ibest + (bestIdx - 4)
		xdtFinal := float64(ibest-1) * ft8x.Dt2
		t.Logf("  Stage 4 (refine time): ibest=%d  xdt=%.3f", ibest, xdtFinal)

		// Stage 5: Symbol spectra + hard sync
		cs, s8 := ft8x.ComputeSymbolSpectra(cd0corr, ibest)
		nsync := ft8x.HardSync(&s8)
		t.Logf("  Stage 5 (hard sync): nsync=%d (threshold: >6)", nsync)

		if nsync <= 6 {
			t.Logf("    ✗ FAILED: hard sync too low (%d ≤ 6) — signal not found in baseband", nsync)
			// Also try with full-range ibest
			cs2, s8_2 := ft8x.ComputeSymbolSpectra(cd0corr, ibestFull)
			nsync2 := ft8x.HardSync(&s8_2)
			t.Logf("    Retry with full-range ibest=%d: nsync=%d", ibestFull, nsync2)
			if nsync2 > 6 {
				// Use the better position
				cs = cs2
				s8 = s8_2
				nsync = nsync2
				ibest = ibestFull
				t.Logf("    → Using full-range ibest, nsync=%d", nsync)
			} else {
				t.Logf("    Still failed. This signal is NOT detectable in baseband.")
				continue
			}
		}

		// Show per-Costas-array sync breakdown
		t.Logf("    s8 tone magnitudes for first few data symbols:")
		for sym := 7; sym < 12 && sym < ft8x.NN; sym++ {
			line := "      sym %2d: "
			for tone := 0; tone < 8; tone++ {
				line += " %6.3f"
			}
			t.Logf("      sym %2d: %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f",
				sym, s8[0][sym], s8[1][sym], s8[2][sym], s8[3][sym],
				s8[4][sym], s8[5][sym], s8[6][sym], s8[7][sym])
		}

		// Stage 6: Soft metrics + LDPC
		bmeta, bmetb, bmetc, bmetd := ft8x.ComputeSoftMetrics(&cs)

		// Stats on LLRs
		for label, bmet := range map[string][174]float64{
			"bmeta": bmeta, "bmetb": bmetb, "bmetc": bmetc, "bmetd": bmetd,
		} {
			var minV, maxV, sumAbs float64
			minV = 1e30
			maxV = -1e30
			for _, v := range bmet {
				if v < minV {
					minV = v
				}
				if v > maxV {
					maxV = v
				}
				if v < 0 {
					sumAbs -= v
				} else {
					sumAbs += v
				}
			}
			t.Logf("  Stage 6 (%s): min=%.3f max=%.3f mean|LLR|=%.3f",
				label, minV, maxV, sumAbs/174.0)
		}

		// Try each LLR pass
		apmag := 0.0
		for _, v := range bmeta {
			av := math.Abs(v * ft8x.ScaleFac)
			if av > apmag {
				apmag = av
			}
		}
		apmag *= 1.01

		for ipass, bmet := range [][174]float64{bmeta, bmetb, bmetc, bmetd} {
			names := []string{"bmeta", "bmetb", "bmetc", "bmetd"}
			var llrz [ft8x.LDPCn]float64
			var apmask [ft8x.LDPCn]int8
			for i, v := range bmet {
				llrz[i] = ft8x.ScaleFac * v
			}

			res, ok := ft8x.Decode174_91(llrz, ft8x.LDPCk, 2, 2, apmask)
			if ok {
				var msg77 [77]int8
				copy(msg77[:], res.Message91[:77])
				c77 := ft8x.BitsToC77(msg77)
				msg, unpackOK := ft8x.Unpack77(c77)
				t.Logf("    pass %d (%s): DECODE OK  nhard=%d  msg=%q  unpack=%v",
					ipass, names[ipass], res.NHardErrors, strings.TrimSpace(msg), unpackOK)
			} else {
				t.Logf("    pass %d (%s): DECODE FAILED", ipass, names[ipass])
			}
		}

		// Also try with AP (CQ)
		for ipass, bmet := range [][174]float64{bmeta, bmetb, bmetc, bmetd} {
			names := []string{"bmeta+AP", "bmetb+AP", "bmetc+AP", "bmetd+AP"}
			var llrz [ft8x.LDPCn]float64
			var apmask [ft8x.LDPCn]int8
			for i, v := range bmet {
				llrz[i] = ft8x.ScaleFac * v
			}
			// Apply AP type 1 (CQ)
			ft8x.ApplyAP(&llrz, &apmask, 1, [58]int{}, apmag)

			res, ok := ft8x.Decode174_91(llrz, ft8x.LDPCk, 2, 2, apmask)
			if ok {
				var msg77 [77]int8
				copy(msg77[:], res.Message91[:77])
				c77 := ft8x.BitsToC77(msg77)
				msg, unpackOK := ft8x.Unpack77(c77)
				if strings.Contains(strings.TrimSpace(msg), "CQ") || !unpackOK {
					t.Logf("    pass %d (%s): DECODE OK  nhard=%d  msg=%q  unpack=%v",
						ipass+4, names[ipass], res.NHardErrors, strings.TrimSpace(msg), unpackOK)
				} else {
					t.Logf("    pass %d (%s): DECODE OK (non-CQ)  nhard=%d  msg=%q",
						ipass+4, names[ipass], res.NHardErrors, strings.TrimSpace(msg))
				}
			} else {
				t.Logf("    pass %d (%s): DECODE FAILED", ipass+4, names[ipass])
			}
		}
	}
}

// TestPipelineTraceWithSubtraction runs the same trace but AFTER subtracting
// the 15 signals that DO decode (the strong signals may be masking the weak ones).
func TestPipelineTraceWithSubtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping pipeline trace with subtraction (slow)")
	}

	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// Normalized float32
	ddWork := make([]float32, NMAX)
	for i := 0; i < NMAX; i++ {
		ddWork[i] = dd[i] / 32768.0
	}

	// ── First, decode all the signals we CAN decode and subtract them ──
	params := ft8x.DecodeParams{
		Depth:     3,
		APEnabled: true,
		APCQOnly:  true,
		APWidth:   25.0,
		MaxPasses: 3,
	}
	results := ft8x.DecodeIterative(ddWork, params, 200, 2600)
	t.Logf("Pre-subtraction: decoded %d signals", len(results))
	for _, r := range results {
		t.Logf("  subtracted: %7.1f Hz  dt=%+.1f  %s", r.Freq, r.DT, strings.TrimSpace(r.Message))
	}

	// ── Now try each missing signal on the CLEANED audio ──
	t.Logf("")
	t.Logf("═══════════════════════════════════════════════════════════════")
	t.Logf("  TRACING MISSING SIGNALS ON CLEANED AUDIO")
	t.Logf("═══════════════════════════════════════════════════════════════")

	for _, sig := range missingRef {
		t.Logf("")
		t.Logf("── %s  freq=%.1f  DT=%.1f ──", sig.Label, sig.Freq, sig.DT)

		xdt := sig.DT - 0.5

		// Try at exact params
		ds := ft8x.NewDownsampler()
		deepParams := ft8x.DecodeParams{
			Depth:     3,
			APEnabled: true,
			APCQOnly:  true,
			APWidth:   25.0,
		}
		result, ok := ft8x.DecodeSingle(ddWork, ds, sig.Freq, xdt, true, deepParams)
		if ok {
			t.Logf("  ✓ DECODED on cleaned audio: %s  snr=%.1f", strings.TrimSpace(result.Message), result.SNR)
			continue
		}

		// Downsample and trace
		newdat := true
		cd0 := ds.Downsample(ddWork, &newdat, sig.Freq)

		// Full range sync scan
		var ctwkZero [32]complex128
		for i := range ctwkZero {
			ctwkZero[i] = complex(1, 0)
		}
		smaxFull := 0.0
		ibestFull := 0
		for idt := 0; idt <= ft8x.NP2; idt++ {
			sync := ft8x.Sync8d(cd0, idt, ctwkZero, 0)
			if sync > smaxFull {
				smaxFull = sync
				ibestFull = idt
			}
		}
		t.Logf("  Full sync scan: ibest=%d  sync=%.4f  (xdt=%.3f)", ibestFull, smaxFull, float64(ibestFull-1)*ft8x.Dt2)

		// Compare to expected i0
		i0 := int(math.Round((xdt + 0.5) * ft8x.Fs2))
		syncAtRef := ft8x.Sync8d(cd0, i0, ctwkZero, 0)
		t.Logf("  Sync at ref i0=%d: %.4f", i0, syncAtRef)

		// Try decode at full-range best
		result2, ok2 := ft8x.DecodeSingle(ddWork, ds, sig.Freq, float64(ibestFull-1)*ft8x.Dt2, false, deepParams)
		if ok2 {
			t.Logf("  ✓ DECODED with full-range sync: %s", strings.TrimSpace(result2.Message))
			continue
		}

		// Hard sync check
		_, s8 := ft8x.ComputeSymbolSpectra(cd0, ibestFull)
		nsync := ft8x.HardSync(&s8)
		t.Logf("  HardSync at ibest=%d: nsync=%d", ibestFull, nsync)

		// Try wider freq search: ±20 Hz
		t.Logf("  ── Wide freq search ±20 Hz on cleaned audio ──")
		for df := -20.0; df <= 20.0; df += 2.0 {
			f := sig.Freq + df
			ds4 := ft8x.NewDownsampler()
			r, ok4 := ft8x.DecodeSingle(ddWork, ds4, f, xdt, true, deepParams)
			if ok4 {
				t.Logf("  ✓ Found at freq=%.1f (df=%+.1f): %s snr=%.1f",
					f, df, strings.TrimSpace(r.Message), r.SNR)
			}
		}

		t.Logf("  ✗ Signal NOT decodable even on cleaned audio")
	}
}

// TestLDPCMarginAnalysis checks how close each missing signal comes to
// decoding by examining the BP decoder's syndrome count trajectory.
func TestLDPCMarginAnalysis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LDPC margin analysis (slow)")
	}

	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	ddNorm := make([]float32, NMAX)
	for i := 0; i < NMAX; i++ {
		ddNorm[i] = dd[i] / 32768.0
	}

	t.Logf("LDPC Margin Analysis — how close are the missing signals to decoding?")
	t.Logf("")

	// Also test signals that DO decode for comparison
	goodSignals := []struct {
		Label string
		Freq  float64
		DT    float64
	}{
		{"CQ KB3Z FN20", 1100, 1.5},
		{"CQ SP4MSY KO13", 1250, 1.2},
		{"VK5ATH VK0DS -01", 1648, 1.4},
	}

	type testSig struct {
		Label   string
		Freq    float64
		DT      float64
		Missing bool
	}
	var allSigs []testSig
	for _, s := range goodSignals {
		allSigs = append(allSigs, testSig{s.Label, s.Freq, s.DT, false})
	}
	for _, s := range missingRef {
		allSigs = append(allSigs, testSig{s.Label, s.Freq, s.DT, true})
	}

	for _, sig := range allSigs {
		xdt := sig.DT - 0.5
		tag := "  ✓"
		if sig.Missing {
			tag = "  ✗"
		}

		ds := ft8x.NewDownsampler()
		newdat := true
		cd0 := ds.Downsample(ddNorm, &newdat, sig.Freq)

		// Find best sync
		var ctwkZero [32]complex128
		for i := range ctwkZero {
			ctwkZero[i] = complex(1, 0)
		}
		i0 := int(math.Round((xdt + 0.5) * ft8x.Fs2))
		smax := 0.0
		ibest := i0
		for idt := i0 - 20; idt <= i0+20; idt++ {
			sync := ft8x.Sync8d(cd0, idt, ctwkZero, 0)
			if sync > smax {
				smax = sync
				ibest = idt
			}
		}

		// Fine freq
		smax2 := 0.0
		delfBest := 0.0
		twopi := 2.0 * math.Pi
		for ifr := -5; ifr <= 5; ifr++ {
			delf := float64(ifr) * 0.5
			dphi := twopi * delf * ft8x.Dt2
			phi := 0.0
			var ctwk [32]complex128
			for i := 0; i < 32; i++ {
				ctwk[i] = complex(math.Cos(phi), math.Sin(phi))
				phi = math.Mod(phi+dphi, twopi)
			}
			sync := ft8x.Sync8d(cd0, ibest, ctwk, 1)
			if sync > smax2 {
				smax2 = sync
				delfBest = delf
			}
		}
		f1 := sig.Freq + delfBest

		// Re-downsample with corrected freq
		newdat2 := false
		cd0 = ds.Downsample(ddNorm, &newdat2, f1)

		// Symbol spectra
		cs, s8 := ft8x.ComputeSymbolSpectra(cd0, ibest)
		nsync := ft8x.HardSync(&s8)

		bmeta, _, _, _ := ft8x.ComputeSoftMetrics(&cs)

		// LLR quality metric: how confident are the LLRs?
		var llrz [ft8x.LDPCn]float64
		for i, v := range bmeta {
			llrz[i] = ft8x.ScaleFac * v
		}

		// Compute a "LLR confidence" = mean(|llr|) — higher means cleaner signal
		sumAbs := 0.0
		for _, v := range llrz {
			if v < 0 {
				sumAbs -= v
			} else {
				sumAbs += v
			}
		}
		meanLLR := sumAbs / float64(ft8x.LDPCn)

		// Try decode to get nhard or failure
		var apmask [ft8x.LDPCn]int8
		res, decOK := ft8x.Decode174_91(llrz, ft8x.LDPCk, 2, 2, apmask)
		nhard := -1
		if decOK {
			nhard = res.NHardErrors
		}

		// Count initial syndrome weight (how far from valid codeword)
		syndW := 0
		var cw [ft8x.LDPCn]int8
		for i := 0; i < ft8x.LDPCn; i++ {
			if llrz[i] > 0 {
				cw[i] = 1
			}
		}
		for j := 0; j < ft8x.LDPCm; j++ {
			s := 0
			for i := 0; i < ft8x.LDPCNrw[j]; i++ {
				s += int(cw[ft8x.LDPCNm[j][i]-1])
			}
			if s%2 != 0 {
				syndW++
			}
		}

		t.Logf("%s %-25s  sync=%.2f  nsync=%2d  delf=%+.1f  mean|LLR|=%.2f  syndW=%d  nhard=%d  decoded=%v",
			tag, sig.Label, smax, nsync, delfBest, meanLLR, syndW, nhard, decOK)
	}
}
