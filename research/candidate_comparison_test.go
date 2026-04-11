// candidate_comparison_test.go — Compares research sync8 candidate detection
// against the 21 WSJT-X reference decodes for capture.wav and against the
// main ft8x.Sync8FindCandidates output.

package research

import (
	"fmt"
	"math"
	"os"
	"testing"

	ft8x "github.com/ColonelBlimp/go-ft8"
)

// wsjtxRef lists the 21 WSJT-X reference decodes for capture.wav.
// Freq/DT are from the WSJT-X ALL.TXT output. DT here is the xdt
// convention (WSJT-X DT − 0.5).
type wsjtxRefSignal struct {
	Label string
	Freq  float64
	DT    float64 // xdt = WSJT-X DT − 0.5
}

var wsjtxRef = []wsjtxRefSignal{
	{"VK5BU RT6C KN95", 2454, 1.3 - 0.5},
	{"CQ V4/SP9FIH", 298, 1.3 - 0.5},
	{"A61OK US1VM KN68", 568, 1.5 - 0.5},
	{"CQ KB3Z FN20", 1100, 1.5 - 0.5},
	{"CQ NH6D BL02", 2251, 1.4 - 0.5},
	{"VK3TZ UX2QX R-07", 1776, 1.3 - 0.5},
	{"UB8CSR SV0TPN -18", 1998, 1.7 - 0.5},
	{"R8AGW VU2FR RR73", 2507, 2.0 - 0.5},
	{"<V4/SP9FIH> JA8RVP QN23", 1001, 1.4 - 0.5},
	{"CQ UR5QW KN77", 2401, 1.4 - 0.5},
	{"UA0LW RK6AAC KN95", 1149, 1.6 - 0.5},
	{"CQ SP4MSY KO13", 1250, 1.2 - 0.5},
	{"VK5ATH VK0DS -01", 1648, 1.4 - 0.5},
	{"VK5BU RG5A KN93", 533, 1.4 - 0.5},
	{"5Z4VJ YB1RUS OI33", 1505, 1.3 - 0.5},
	{"UA0LW UA4ARH -15", 1211, 1.5 - 0.5},
	{"VK5BU R7HL 73", 2110, 1.3 - 0.5},
	{"CQ 4S6ARW MJ97", 938, 1.8 - 0.5},
	{"CQ CO8LY FL20", 932, 1.2 - 0.5},
	{"VK3TZ UA3ZNQ KO81", 888, 1.6 - 0.5},
	{"VK3TZ RC7O KN87", 963, 1.4 - 0.5},
}

// candidateCovers returns true if a candidate is "close enough" to a reference
// signal to potentially decode it. Uses the same tolerance WSJT-X's ft8b
// allows for the initial sync8 → DecodeSingle hand-off:
//   - ±10 Hz in frequency
//   - ±0.5 s in DT
func candidateCovers(candFreq, candDT, refFreq, refDT float64) bool {
	return math.Abs(candFreq-refFreq) <= 10.0 && math.Abs(candDT-refDT) <= 0.5
}

func TestCandidateComparison(t *testing.T) {
	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// ── Parameters matching WSJT-X / DecodeIterative ─────────────────
	nfa := 200
	nfb := 2600
	syncmin := 1.3
	nfqso := 0
	maxcand := 600

	// ══════════════════════════════════════════════════════════════════
	// 1. Research sync8 (raw int16 audio, optimized RealFFT)
	// ══════════════════════════════════════════════════════════════════
	resCands, _ := Sync8(dd, NMAX, nfa, nfb, syncmin, nfqso, maxcand)

	t.Logf("═══════════════════════════════════════════════════════")
	t.Logf("Research sync8: %d candidates (syncmin=%.1f, %d–%d Hz)", len(resCands), syncmin, nfa, nfb)
	t.Logf("═══════════════════════════════════════════════════════")

	// Check coverage of WSJT-X reference signals.
	resCovered := 0
	for _, ref := range wsjtxRef {
		found := false
		bestDist := math.Inf(1)
		bestFreq := 0.0
		bestDT := 0.0
		bestSync := 0.0
		for _, c := range resCands {
			if candidateCovers(c.Freq, c.DT, ref.Freq, ref.DT) {
				dist := math.Abs(c.Freq-ref.Freq) + math.Abs(c.DT-ref.DT)*100
				if !found || dist < bestDist {
					bestDist = dist
					bestFreq = c.Freq
					bestDT = c.DT
					bestSync = c.SyncPower
					found = true
				}
			}
		}
		if found {
			resCovered++
			t.Logf("  ✓ %-30s  ref=%7.1f/%+5.2f  cand=%7.1f/%+5.2f  sync=%5.1f",
				ref.Label, ref.Freq, ref.DT, bestFreq, bestDT, bestSync)
		} else {
			t.Logf("  ✗ %-30s  ref=%7.1f/%+5.2f  NOT COVERED",
				ref.Label, ref.Freq, ref.DT)
		}
	}
	t.Logf("Research sync8 coverage: %d / %d WSJT-X signals (%.0f%%)",
		resCovered, len(wsjtxRef), 100.0*float64(resCovered)/float64(len(wsjtxRef)))

	// ══════════════════════════════════════════════════════════════════
	// 2. Main ft8x.Sync8FindCandidates (normalized audio /32768)
	// ══════════════════════════════════════════════════════════════════
	// The main codebase uses /32768 normalized float32 audio.
	ddNorm := make([]float32, NMAX)
	for i := 0; i < NMAX; i++ {
		ddNorm[i] = dd[i] / 32768.0
	}

	ft8xCands := ft8x.Sync8FindCandidates(ddNorm, nfa, nfb, syncmin, nfqso, maxcand)

	t.Logf("")
	t.Logf("═══════════════════════════════════════════════════════")
	t.Logf("ft8x.Sync8FindCandidates: %d candidates (syncmin=%.1f, %d–%d Hz)", len(ft8xCands), syncmin, nfa, nfb)
	t.Logf("═══════════════════════════════════════════════════════")

	ft8xCovered := 0
	for _, ref := range wsjtxRef {
		found := false
		bestDist := math.Inf(1)
		bestFreq := 0.0
		bestDT := 0.0
		bestSync := 0.0
		for _, c := range ft8xCands {
			if candidateCovers(c.Freq, c.DT, ref.Freq, ref.DT) {
				dist := math.Abs(c.Freq-ref.Freq) + math.Abs(c.DT-ref.DT)*100
				if !found || dist < bestDist {
					bestDist = dist
					bestFreq = c.Freq
					bestDT = c.DT
					bestSync = c.SyncPower
					found = true
				}
			}
		}
		if found {
			ft8xCovered++
			t.Logf("  ✓ %-30s  ref=%7.1f/%+5.2f  cand=%7.1f/%+5.2f  sync=%5.1f",
				ref.Label, ref.Freq, ref.DT, bestFreq, bestDT, bestSync)
		} else {
			t.Logf("  ✗ %-30s  ref=%7.1f/%+5.2f  NOT COVERED",
				ref.Label, ref.Freq, ref.DT)
		}
	}
	t.Logf("ft8x coverage: %d / %d WSJT-X signals (%.0f%%)",
		ft8xCovered, len(wsjtxRef), 100.0*float64(ft8xCovered)/float64(len(wsjtxRef)))

	// ══════════════════════════════════════════════════════════════════
	// 3. Summary comparison
	// ══════════════════════════════════════════════════════════════════
	t.Logf("")
	t.Logf("═══════════════════════════════════════════════════════")
	t.Logf("SUMMARY")
	t.Logf("═══════════════════════════════════════════════════════")
	t.Logf("WSJT-X reference:    %d decoded signals", len(wsjtxRef))
	t.Logf("Research sync8:      %d candidates, covers %d/%d (%.0f%%)",
		len(resCands), resCovered, len(wsjtxRef), 100.0*float64(resCovered)/float64(len(wsjtxRef)))
	t.Logf("ft8x sync8:          %d candidates, covers %d/%d (%.0f%%)",
		len(ft8xCands), ft8xCovered, len(wsjtxRef), 100.0*float64(ft8xCovered)/float64(len(wsjtxRef)))

	// Also show top-20 candidates from each for visual comparison.
	t.Logf("")
	t.Logf("── Top 20 research candidates ──")
	n := 20
	if len(resCands) < n {
		n = len(resCands)
	}
	for i := 0; i < n; i++ {
		c := resCands[i]
		tag := ""
		for _, ref := range wsjtxRef {
			if candidateCovers(c.Freq, c.DT, ref.Freq, ref.DT) {
				tag = fmt.Sprintf(" ← %s", ref.Label)
				break
			}
		}
		t.Logf("  #%2d  %7.1f Hz  DT=%+6.2f  sync=%6.2f%s", i+1, c.Freq, c.DT, c.SyncPower, tag)
	}

	t.Logf("")
	t.Logf("── Top 20 ft8x candidates ──")
	n = 20
	if len(ft8xCands) < n {
		n = len(ft8xCands)
	}
	for i := 0; i < n; i++ {
		c := ft8xCands[i]
		tag := ""
		for _, ref := range wsjtxRef {
			if candidateCovers(c.Freq, c.DT, ref.Freq, ref.DT) {
				tag = fmt.Sprintf(" ← %s", ref.Label)
				break
			}
		}
		t.Logf("  #%2d  %7.1f Hz  DT=%+6.2f  sync=%6.2f%s", i+1, c.Freq, c.DT, c.SyncPower, tag)
	}
}
