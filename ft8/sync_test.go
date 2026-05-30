// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type truthFile struct {
	WAV     string `json:"wav"`
	Signals []struct {
		Text   string  `json:"text"`
		FreqHz int     `json:"freq_hz"`
		DTSec  float64 `json:"dt_s"`
	} `json:"signals"`
}

type decodeRun struct {
	Blocks     int
	DD         []float32
	Candidates []candidate
	DS         *downsampler
}

func TestFindCandidatesCoversOracleFrequencies(t *testing.T) {
	matches := corpusTruthFiles(t)

	for _, truthPath := range matches {
		t.Run(filepath.Base(truthPath), func(t *testing.T) {
			tf := readTruth(t, truthPath)
			samples := loadCorpusWAV(t, tf.WAV)
			candidates := findDefaultCandidates(decodeBlocks(samples, 50), ft8DefaultMinFreq, ft8DefaultMaxFreq)

			matched := 0
			for _, sig := range tf.Signals {
				if hasCandidateNear(candidates, float64(sig.FreqHz), 4.0) {
					matched++
				}
			}
			coverage := float64(matched) / float64(len(tf.Signals))
			t.Logf("matched %d of %d oracle frequencies with %d candidates", matched, len(tf.Signals), len(candidates))
			if coverage < 0.80 {
				t.Fatalf("candidate frequency coverage %.1f%%: matched %d of %d", 100*coverage, matched, len(tf.Signals))
			}
		})
	}
}

func TestRefineCandidatesForOracleSignals(t *testing.T) {
	matches := corpusTruthFiles(t)
	for _, truthPath := range matches {
		t.Run(filepath.Base(truthPath), func(t *testing.T) {
			tf := readTruth(t, truthPath)
			samples := loadCorpusWAV(t, tf.WAV)
			dd := decodeBlocks(samples, 50)
			candidates := findDefaultCandidates(dd, ft8DefaultMinFreq, ft8DefaultMaxFreq)
			ds := newDownsampler()

			refined := 0
			strongSync := 0
			for _, sig := range tf.Signals {
				cand, ok := nearestCandidate(candidates, float64(sig.FreqHz), 4.0)
				if !ok {
					continue
				}
				r := refineCandidateWithDownsampler(dd, ds, cand, refined == 0)
				refined++
				if r.HardSync > 8 {
					strongSync++
				}
			}
			t.Logf("refined %d oracle candidates; %d have hard-sync > 8", refined, strongSync)
			if refined == 0 {
				t.Fatal("no oracle candidates refined")
			}
			if float64(strongSync)/float64(len(tf.Signals)) < 0.70 {
				t.Fatalf("hard-sync coverage too low: %d of %d", strongSync, len(tf.Signals))
			}
		})
	}
}

func TestSoftMetricsForStrongOracleSignals(t *testing.T) {
	matches := corpusTruthFiles(t)
	for _, truthPath := range matches {
		t.Run(filepath.Base(truthPath), func(t *testing.T) {
			tf := readTruth(t, truthPath)
			samples := loadCorpusWAV(t, tf.WAV)
			dd := decodeBlocks(samples, 50)
			candidates := findDefaultCandidates(dd, ft8DefaultMinFreq, ft8DefaultMaxFreq)
			ds := newDownsampler()

			checked := 0
			for _, sig := range tf.Signals {
				cand, ok := nearestCandidate(candidates, float64(sig.FreqHz), 4.0)
				if !ok {
					continue
				}
				analysis := analyzeCandidateWithDownsampler(dd, ds, cand, checked == 0)
				if analysis.Refined.HardSync <= 8 {
					continue
				}
				checked++
				if !metricUsable(analysis.Metrics) {
					t.Fatalf("unusable metrics for %s at %.0f Hz", sig.Text, cand.FreqHz)
				}
			}
			t.Logf("checked usable soft metrics for %d strong oracle candidates", checked)
			if checked == 0 {
				t.Fatal("no strong oracle candidates checked")
			}
		})
	}
}

func TestBPDecoderProducesCRCValidCodewords(t *testing.T) {
	matches := corpusTruthFiles(t)
	totalDecoded := 0
	totalStrong := 0
	for _, truthPath := range matches {
		t.Run(filepath.Base(truthPath), func(t *testing.T) {
			tf := readTruth(t, truthPath)
			samples := loadCorpusWAV(t, tf.WAV)
			dd := decodeBlocks(samples, 50)
			candidates := findDefaultCandidates(dd, ft8DefaultMinFreq, ft8DefaultMaxFreq)
			ds := newDownsampler()

			decoded := 0
			strong := 0
			for _, sig := range tf.Signals {
				cand, ok := nearestCandidate(candidates, float64(sig.FreqHz), 4.0)
				if !ok {
					continue
				}
				analysis := analyzeCandidateWithDownsampler(dd, ds, cand, strong == 0)
				if analysis.Refined.HardSync <= 8 {
					continue
				}
				strong++
				for _, llr := range llrPasses(analysis.Metrics) {
					if _, ok := decode17491BPOnly(llr); ok {
						decoded++
						break
					}
				}
			}
			t.Logf("BP-only decoded %d CRC-valid codewords from %d strong oracle candidates", decoded, strong)
			totalDecoded += decoded
			totalStrong += strong
		})
	}
	if totalStrong == 0 {
		t.Fatal("no strong candidates")
	}
	if totalDecoded == 0 {
		t.Fatal("BP-only decoder produced no CRC-valid codewords")
	}
}

func TestBPDecoderUnpacksFixtureMessages(t *testing.T) {
	matches := corpusTruthFiles(t)
	totalMatched := 0
	totalDecoded := 0
	for _, truthPath := range matches {
		t.Run(filepath.Base(truthPath), func(t *testing.T) {
			tf := readTruth(t, truthPath)
			samples := loadCorpusWAV(t, tf.WAV)
			dd := decodeBlocks(samples, 50)
			candidates := findDefaultCandidates(dd, ft8DefaultMinFreq, ft8DefaultMaxFreq)
			ds := newDownsampler()

			matched := 0
			decoded := 0
			for _, sig := range tf.Signals {
				cand, ok := nearestCandidate(candidates, float64(sig.FreqHz), 4.0)
				if !ok {
					continue
				}
				analysis := analyzeCandidateWithDownsampler(dd, ds, cand, decoded == 0)
				if analysis.Refined.HardSync <= 8 {
					continue
				}
				for _, llr := range llrPasses(analysis.Metrics) {
					result, ok := decode17491BPOnly(llr)
					if !ok {
						continue
					}
					decoded++
					msg, ok := unpack77FromCodeword(result.Codeword)
					if ok && msg == normalizeTruthText(sig.Text) {
						matched++
					}
					break
				}
			}
			t.Logf("unpacked %d exact message matches from %d BP CRC-valid codewords", matched, decoded)
			totalMatched += matched
			totalDecoded += decoded
		})
	}
	if totalDecoded == 0 {
		t.Fatal("no BP decoded messages to unpack")
	}
	if totalMatched == 0 {
		t.Fatal("unpacker produced no exact fixture message matches")
	}
}

func TestHybridDecoderUnpacksFixtureMessages(t *testing.T) {
	matches := corpusTruthFiles(t)
	options := normalizeDecoderOptions(DeepDecoderOptions())
	totalMatched := 0
	totalDecoded := 0
	for _, truthPath := range matches {
		t.Run(filepath.Base(truthPath), func(t *testing.T) {
			tf := readTruth(t, truthPath)
			samples := loadCorpusWAV(t, tf.WAV)
			runs := buildDecodeRuns(samples)

			matched := 0
			decoded := 0
			for _, sig := range tf.Signals {
				want := normalizeTruthText(sig.Text)
				found := false
				for i := range runs {
					for _, cand := range candidatesNearFrequency(runs[i].Candidates, float64(sig.FreqHz), 5.0) {
						_, dec, ok := decodeCandidateVariantsForMetricSet(runs[i].DD, runs[i].DS, cand, decoded == 0, 2, nil, options)
						if ok {
							decoded++
						}
						if ok && dec.Text == want {
							matched++
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			t.Logf("hybrid decoded %d exact message matches from %d CRC-valid codewords", matched, decoded)
			totalMatched += matched
			totalDecoded += decoded
		})
	}
	if totalDecoded == 0 {
		t.Fatal("no hybrid decoded messages to unpack")
	}
	if totalMatched < 100 {
		t.Fatalf("hybrid exact fixture matches too low: %d", totalMatched)
	}
}

func TestDiagnosticMisses(t *testing.T) {
	if os.Getenv("DECODE_DIAGNOSTICS") == "" {
		t.Skip("set DECODE_DIAGNOSTICS=1")
	}
	matches := corpusTruthFiles(t)
	for _, truthPath := range matches {
		tf := readTruth(t, truthPath)
		samples := loadCorpusWAV(t, tf.WAV)
		runs := buildDecodeRuns(samples)

		for _, sig := range tf.Signals {
			want := normalizeTruthText(sig.Text)
			matched := false
			decodedAny := false
			bestGot := ""
			var bestAnalysis candidateAnalysis
			nearbyCount := 0
			bestBlocks := 0
			for i := range runs {
				nearby := candidatesNearFrequency(runs[i].Candidates, float64(sig.FreqHz), 5.0)
				nearbyCount += len(nearby)
				for _, cand := range nearby {
					analysis, dec, decoded := decodeCandidateVariants(runs[i].DD, runs[i].DS, cand, false)
					got := dec.Text
					if decoded {
						decodedAny = true
						bestGot = got
						bestAnalysis = analysis
						bestBlocks = runs[i].Blocks
					}
					if decoded && got == want {
						matched = true
						break
					}
					if bestAnalysis.candidate == (candidate{}) || analysis.Refined.HardSync > bestAnalysis.Refined.HardSync {
						bestAnalysis = analysis
						bestBlocks = runs[i].Blocks
					}
				}
				if matched {
					break
				}
			}
			if nearbyCount == 0 {
				t.Logf("%s %.0fHz no candidate: %s", tf.WAV, float64(sig.FreqHz), want)
				continue
			}
			if !matched {
				t.Logf("%s want=%q got=%q decoded=%v nearby=%d blocks=%d cand=%.1fHz dt=%.2f sync=%.2f refined=%.1fHz rdt=%.2f hard=%d",
					tf.WAV, want, bestGot, decodedAny, nearbyCount, bestBlocks, bestAnalysis.candidate.FreqHz,
					bestAnalysis.candidate.DTSec, bestAnalysis.candidate.Sync, bestAnalysis.Refined.FreqHz,
					bestAnalysis.Refined.DTSec, bestAnalysis.Refined.HardSync)
			}
		}
	}
}

func TestDiagnosticExecutableParity(t *testing.T) {
	if os.Getenv("DECODE_DIAGNOSTICS") == "" {
		t.Skip("set DECODE_DIAGNOSTICS=1")
	}
	matches := corpusTruthFiles(t)
	for _, truthPath := range matches {
		tf := readTruth(t, truthPath)
		samples := loadCorpusWAV(t, tf.WAV)
		want := make(map[string]bool)
		for _, sig := range tf.Signals {
			want[normalizeTruthText(sig.Text)] = true
		}
		got := DecodeMessages(samples)
		hit := 0
		for _, msg := range got {
			if want[msg.Text] {
				hit++
				continue
			}
			t.Logf("%s false-positive %q f=%.1f dt=%.2f sync=%d he=%d d=%.1f blocks=%d",
				tf.WAV, msg.Text, msg.FreqHz, msg.DTSec, msg.HardSync, msg.HardErrors, msg.DMin, msg.Blocks)
		}
		for msg := range want {
			found := false
			for _, dec := range got {
				if dec.Text == msg {
					found = true
					break
				}
			}
			if !found {
				t.Logf("%s missing %q", tf.WAV, msg)
			}
		}
		t.Logf("%s executable hits=%d truth=%d output=%d", tf.WAV, hit, len(want), len(got))
	}
}

func TestStrictCorpusDecodeParity(t *testing.T) {
	matches := corpusTruthFiles(t)

	totalHit := 0
	totalTruth := 0
	totalOutput := 0
	for _, truthPath := range matches {
		tf := readTruth(t, truthPath)
		samples := loadCorpusWAV(t, tf.WAV)
		want := make(map[string]bool)
		for _, sig := range tf.Signals {
			want[normalizeTruthText(sig.Text)] = true
		}
		got := DecodeMessages(samples)

		hit := 0
		var extras []string
		for _, msg := range got {
			if want[msg.Text] {
				hit++
			} else {
				extras = append(extras, msg.Text)
			}
		}
		var missing []string
		for msg := range want {
			found := false
			for _, dec := range got {
				if dec.Text == msg {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, msg)
			}
		}

		totalHit += hit
		totalTruth += len(want)
		totalOutput += len(got)
		if hit != len(want) || len(extras) != 0 || len(missing) != 0 {
			t.Fatalf("%s strict parity failed: hit=%d truth=%d output=%d extras=%v missing=%v",
				tf.WAV, hit, len(want), len(got), extras, missing)
		}
	}
	if totalHit != 144 || totalTruth != 144 || totalOutput != 144 {
		t.Fatalf("strict corpus totals changed: hit=%d truth=%d output=%d, want 144/144/144",
			totalHit, totalTruth, totalOutput)
	}
}

func TestDiagnosticOracleGrid(t *testing.T) {
	if os.Getenv("DECODE_DIAGNOSTICS") == "" {
		t.Skip("set DECODE_DIAGNOSTICS=1")
	}
	matches := corpusTruthFiles(t)
	for _, truthPath := range matches {
		tf := readTruth(t, truthPath)
		samples := loadCorpusWAV(t, tf.WAV)
		for _, sig := range tf.Signals {
			want := normalizeTruthText(sig.Text)
			found := false
			var best candidateAnalysis
			for _, blocks := range []int{41, 47, 50} {
				dd := decodeBlocks(samples, blocks)
				ds := newDownsampler()
				for df := -4.0; df <= 4.0 && !found; df += 1.0 {
					for dt := -0.24; dt <= 0.24 && !found; dt += 0.04 {
						cand := candidate{FreqHz: float64(sig.FreqHz) + df, DTSec: sig.DTSec + dt, Sync: 99}
						analysis := analyzeCandidateWithDownsampler(dd, ds, cand, false)
						msg, ok := decodeCandidateMessage(analysis)
						if ok && msg == want {
							t.Logf("%s oracle-grid decoded %q blocks=%d seedF=%.1f seedDT=%.2f refinedF=%.1f refinedDT=%.2f hard=%d",
								tf.WAV, want, blocks, cand.FreqHz, cand.DTSec, analysis.Refined.FreqHz,
								analysis.Refined.DTSec-0.5, analysis.Refined.HardSync)
							found = true
						}
						if best.candidate == (candidate{}) || analysis.Refined.HardSync > best.Refined.HardSync {
							best = analysis
						}
					}
				}
			}
			if !found {
				t.Logf("%s oracle-grid miss %q bestF=%.1f bestDT=%.2f hard=%d",
					tf.WAV, want, best.Refined.FreqHz, best.Refined.DTSec-0.5, best.Refined.HardSync)
			}
		}
	}
}

func TestDiagnosticA7History(t *testing.T) {
	if os.Getenv("DECODE_DIAGNOSTICS") == "" {
		t.Skip("set DECODE_DIAGNOSTICS=1")
	}
	pairs := [][2]string{
		{corpusPath("20m_slot1.truth.json"), corpusPath("20m_slot3.truth.json")},
		{corpusPath("live_slot1.truth.json"), corpusPath("live_slot3.truth.json")},
	}
	for _, pair := range pairs {
		prev := readTruth(t, pair[0])
		next := readTruth(t, pair[1])
		prevMessages := make([]DecodedMessage, 0, len(prev.Signals))
		for _, sig := range prev.Signals {
			prevMessages = append(prevMessages, DecodedMessage{
				Text:   normalizeTruthText(sig.Text),
				FreqHz: float64(sig.FreqHz),
				DTSec:  sig.DTSec,
			})
		}
		hints := collectA7Hints(prevMessages)
		samples := loadCorpusWAV(t, next.WAV)
		decoded := decodeA7Hints(decodeBlocks(samples, 50), hints, make(map[string]bool))
		want := make(map[string]bool)
		for _, sig := range next.Signals {
			want[normalizeTruthText(sig.Text)] = true
		}
		hit := 0
		for _, msg := range decoded {
			if want[msg.Text] {
				hit++
			}
			t.Logf("%s a7 %q f=%.1f dt=%.2f hard=%d d=%.1f",
				next.WAV, msg.Text, msg.FreqHz, msg.DTSec, msg.HardErrors, msg.DMin)
		}
		t.Logf("%s a7 added %d truth hits from %d hints", next.WAV, hit, len(hints))
	}
}

func TestDiagnosticTruthCodewordDistances(t *testing.T) {
	if os.Getenv("DECODE_DIAGNOSTICS") == "" {
		t.Skip("set DECODE_DIAGNOSTICS=1")
	}
	matches := corpusTruthFiles(t)
	for _, truthPath := range matches {
		tf := readTruth(t, truthPath)
		samples := loadCorpusWAV(t, tf.WAV)
		for _, sig := range tf.Signals {
			want := normalizeTruthText(sig.Text)
			bits, ok := pack77StandardMessage(want)
			if !ok {
				continue
			}
			cw := encode17491(bits)
			bestD := math.Inf(1)
			bestHard := 999
			bestHS := 0
			bestBlocks := 0
			bestF := 0.0
			bestDT := 0.0
			decoded := false
			for _, blocks := range []int{41, 47, 50} {
				dd := decodeBlocks(samples, blocks)
				ds := newDownsampler()
				for df := -4.0; df <= 4.0; df += 1.0 {
					for dt := -0.24; dt <= 0.24; dt += 0.04 {
						cand := candidate{FreqHz: float64(sig.FreqHz) + df, DTSec: sig.DTSec + dt, Sync: 99}
						analysis := analyzeCandidateWithDownsampler(dd, ds, cand, false)
						if msg, ok := decodeCandidateMessage(analysis); ok && msg == want {
							decoded = true
						}
						passes := analysisLLRPasses(analysis)
						for passIndex := range passes {
							d := softDistance(cw, &passes[passIndex].LLR)
							h := hardErrors(cw, &passes[passIndex].LLR)
							if d < bestD {
								bestD = d
								bestHard = h
								bestHS = analysis.Refined.HardSync
								bestBlocks = blocks
								bestF = analysis.Refined.FreqHz
								bestDT = analysis.Refined.DTSec - 0.5
							}
						}
					}
				}
			}
			if !decoded {
				t.Logf("%s truth-distance miss %q d=%.1f hard=%d sync=%d blocks=%d f=%.1f dt=%.2f",
					tf.WAV, want, bestD, bestHard, bestHS, bestBlocks, bestF, bestDT)
			}
		}
	}
}

func readTruth(t *testing.T, path string) truthFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("missing truth fixture %s", path)
		}
		t.Fatal(err)
	}
	var tf truthFile
	if err := json.Unmarshal(data, &tf); err != nil {
		t.Fatal(err)
	}
	return tf
}

func hasCandidateNear(candidates []candidate, freq, tolerance float64) bool {
	_, ok := nearestCandidate(candidates, freq, tolerance)
	return ok
}

func nearestCandidate(candidates []candidate, freq, tolerance float64) (candidate, bool) {
	var best candidate
	bestDiff := tolerance
	ok := false
	for _, c := range candidates {
		diff := math.Abs(c.FreqHz - freq)
		if diff <= bestDiff {
			best = c
			bestDiff = diff
			ok = true
		}
	}
	return best, ok
}

func candidatesNearFrequency(candidates []candidate, freq, tolerance float64) []candidate {
	out := make([]candidate, 0)
	for _, c := range candidates {
		if math.Abs(c.FreqHz-freq) <= tolerance {
			out = append(out, c)
		}
	}
	return out
}

func buildDecodeRuns(samples []int16) []decodeRun {
	blocks := []int{41, 47, 50}
	runs := make([]decodeRun, 0, len(blocks))
	for _, n := range blocks {
		dd := decodeBlocks(samples, n)
		runs = append(runs, decodeRun{
			Blocks:     n,
			DD:         dd,
			Candidates: findDefaultCandidates(dd, ft8DefaultMinFreq, ft8DefaultMaxFreq),
			DS:         newDownsampler(),
		})
	}
	return runs
}

func metricUsable(metrics softMetrics) bool {
	passes := llrPasses(metrics)
	for _, pass := range passes {
		nonzero := 0
		for _, v := range pass {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return false
			}
			if math.Abs(v) > 1e-9 {
				nonzero++
			}
		}
		if nonzero < 160 {
			return false
		}
	}
	return true
}

func normalizeTruthText(s string) string {
	fields := strings.Fields(s)
	if len(fields) > 0 && fields[len(fields)-1] == "a1" {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

func decode17491BPOnly(llr [174]float64) (ldpcResult, bool) {
	result, ok, _ := decode17491BP(&llr, &ft8NoAPMask, 0)
	return result, ok
}
