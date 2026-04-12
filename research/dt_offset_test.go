// dt_offset_test.go — Investigate the systematic ~1.0s DT offset between
// our decoder and WSJT-X's reported values.
//
// Observation: ALL successfully decoded signals show our DT = WSJT-X DT + ~1.0s.
// If signal subtraction uses this DT, the subtracted waveform may be misaligned,
// leaving residuals that mask weak nearby signals.
//
// This test quantifies the offset and traces its source.

package research

import (
	"math"
	"os"
	"strings"
	"testing"
)

// decodedSignals pairs WSJT-X reference DT with our decoder's DT.
var decodedSignals = []struct {
	Label   string
	WsjtxDT float64 // WSJT-X DT from ALL.TXT
	WsjtxF  float64 // WSJT-X frequency
}{
	{"VK5BU RT6C KN95", 1.3, 2454},
	{"CQ V4/SP9FIH", 1.3, 298},
	{"A61OK US1VM KN68", 1.5, 568},
	{"CQ KB3Z FN20", 1.5, 1100},
	{"CQ NH6D BL02", 1.4, 2251},
	{"UB8CSR SV0TPN -18", 1.7, 1998},
	{"CQ SP4MSY KO13", 1.2, 1250},
	{"CQ 4S6ARW MJ97", 1.8, 938},
	{"CQ CO8LY FL20", 1.2, 932},
}

func TestDTOffset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DT offset test (slow)")
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

	// Decode all signals
	params := DecodeParams{
		Depth:     3,
		APEnabled: true,
		APCQOnly:  true,
		APWidth:   25.0,
		MaxPasses: 3,
	}
	results := DecodeIterative(ddNorm, params, 200, 2600)

	// Build lookup by message
	decoded := make(map[string]DecodeCandidate)
	for _, r := range results {
		decoded[strings.TrimSpace(r.Message)] = r
	}

	t.Logf("DT Offset Analysis:")
	t.Logf("%-30s  WSJT-X DT  Our DT  Offset  WSJT-X xdt  Our xdt", "Signal")
	t.Logf("%-30s  ---------  ------  ------  ----------  -------", "------")

	var offsets []float64
	for _, sig := range decodedSignals {
		r, ok := decoded[sig.Label]
		if !ok {
			t.Logf("%-30s  %+5.1f      ---     ---    (not decoded)", sig.Label, sig.WsjtxDT)
			continue
		}

		wsjtxXdt := sig.WsjtxDT - 0.5
		ourDT := r.DT // our pipeline returns xdt
		offset := ourDT - wsjtxXdt

		offsets = append(offsets, offset)
		t.Logf("%-30s  %+5.1f     %+5.1f   %+5.1f    %+5.2f       %+5.2f",
			sig.Label, sig.WsjtxDT, ourDT+0.5, offset, wsjtxXdt, ourDT)
	}

	if len(offsets) > 0 {
		sum := 0.0
		for _, o := range offsets {
			sum += o
		}
		mean := sum / float64(len(offsets))
		t.Logf("")
		t.Logf("Mean DT offset: %+.3f seconds", mean)
		t.Logf("At 200 Hz sample rate: %.1f samples", mean*Fs2)
		t.Logf("")
		if math.Abs(mean) > 0.1 {
			t.Logf("⚠ SIGNIFICANT systematic DT offset detected!")
			t.Logf("  This offset in subtraction could leave ~%.0f%% of signal energy as residual,",
				100.0*(1.0-math.Exp(-2.0*math.Pi*6.25*math.Abs(mean)*math.Abs(mean))))
			t.Logf("  masking weak nearby signals on subsequent passes.")
		}
	}
}

// TestSubtractionQuality measures how well our subtraction removes a decoded signal
// by comparing the energy at the signal's frequency before and after subtraction.
func TestSubtractionQuality(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subtraction quality test (slow)")
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

	// Pick CQ 4S6ARW MJ97 at 938 Hz — it's near CQ CO8LY FL20 at 932 Hz.
	// Decode it, then measure residual energy.
	ds := NewDownsampler()
	params := DecodeParams{
		Depth:     3,
		APEnabled: true,
		APCQOnly:  true,
		APWidth:   25.0,
	}

	// First, decode 4S6ARW
	// Use the candidate search to find it
	candidates := Sync8FindCandidates(ddNorm, 200, 2600, 1.3, 0, 600)
	var target4S6 DecodeCandidate
	found := false
	for i, c := range candidates {
		newdat := (i == 0)
		r, ok := DecodeSingle(ddNorm, ds, c.Freq, c.DT, newdat, params)
		if ok && strings.Contains(r.Message, "4S6ARW") {
			target4S6 = r
			found = true
			break
		}
	}
	if !found {
		t.Skip("Could not decode 4S6ARW — skipping subtraction quality test")
	}

	t.Logf("Decoded: %s at freq=%.1f DT=%.1f", target4S6.Message, target4S6.Freq, target4S6.DT)

	// Measure energy at 932 Hz BEFORE subtraction
	ds932 := NewDownsampler()
	newdat := true
	cd0Before := ds932.Downsample(ddNorm, &newdat, 932.0)
	energyBefore := 0.0
	for _, z := range cd0Before {
		r, im := real(z), imag(z)
		energyBefore += r*r + im*im
	}

	// Subtract using per-symbol method
	ddAfterSimple := make([]float32, NMAX)
	copy(ddAfterSimple, ddNorm)
	SubtractFT8(ddAfterSimple, target4S6.Tones, target4S6.Freq, target4S6.DT)

	ds932s := NewDownsampler()
	newdat = true
	cd0AfterSimple := ds932s.Downsample(ddAfterSimple, &newdat, 932.0)
	energyAfterSimple := 0.0
	for _, z := range cd0AfterSimple {
		r, im := real(z), imag(z)
		energyAfterSimple += r*r + im*im
	}

	t.Logf("")
	t.Logf("Energy at 932 Hz baseband (CQ CO8LY FL20 frequency):")
	t.Logf("  Before 4S6ARW subtraction:     %.4e", energyBefore)
	t.Logf("  After SubtractFT8:             %.4e  (%.1f%% remaining)",
		energyAfterSimple, 100*energyAfterSimple/energyBefore)

	// Also measure at 938 Hz (the 4S6ARW signal itself)
	ds938 := NewDownsampler()
	newdat = true
	cd0_938Before := ds938.Downsample(ddNorm, &newdat, 938.0)
	energy938Before := 0.0
	for _, z := range cd0_938Before {
		r, im := real(z), imag(z)
		energy938Before += r*r + im*im
	}

	ds938s := NewDownsampler()
	newdat = true
	cd0_938AfterS := ds938s.Downsample(ddAfterSimple, &newdat, 938.0)
	energy938AfterS := 0.0
	for _, z := range cd0_938AfterS {
		r, im := real(z), imag(z)
		energy938AfterS += r*r + im*im
	}

	t.Logf("")
	t.Logf("Energy at 938 Hz baseband (4S6ARW signal itself):")
	t.Logf("  Before subtraction:            %.4e", energy938Before)
	t.Logf("  After SubtractFT8:             %.4e  (%.1f%% remaining)",
		energy938AfterS, 100*energy938AfterS/energy938Before)

	// Try decoding CO8LY on the cleaned audio
	t.Logf("")
	t.Logf("── Attempting to decode CQ CO8LY FL20 on cleaned audio ──")
	{
		ddClean := ddAfterSimple
		dsC := NewDownsampler()
		label := "SubtractFT8"
		for _, f := range []float64{930, 931, 932, 933, 934} {
			for _, dt := range []float64{0.2, 0.4, 0.6, 0.7, 0.8, 1.0, 1.2} {
				r, ok := DecodeSingle(ddClean, dsC, f, dt, true, params)
				if ok && strings.Contains(r.Message, "CO8LY") {
					t.Logf("  ✓ [%s] Decoded at f=%.0f dt=%.1f: %s snr=%.1f",
						label, f, dt, strings.TrimSpace(r.Message), r.SNR)
				}
			}
		}
	}
}
