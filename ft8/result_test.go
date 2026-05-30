// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestDecodeStructuredStrictOnly(t *testing.T) {
	samples := loadCorpusWAV(t, "20m_slot2.wav")
	result := DecodeStructured(samples, StructuredDecodeOptions{})
	if len(result.Deep) != 0 {
		t.Fatalf("strict-only structured decode populated deep results: %d", len(result.Deep))
	}
	if len(result.Strict) == 0 || len(result.Messages) != len(result.Strict) {
		t.Fatalf("strict structured counts: strict=%d messages=%d", len(result.Strict), len(result.Messages))
	}
	for _, msg := range result.Messages {
		if msg.Mode != DecodeModeStrict || !msg.Strict || msg.Deep || msg.Duplicate {
			t.Fatalf("bad strict structured message flags: %+v", msg)
		}
	}
}

func TestDecodeStructuredDeepLabelsExtras(t *testing.T) {
	matches := corpusTruthFiles(t)

	totalStrict := 0
	totalDeep := 0
	totalMerged := 0
	totalDeepOnly := 0
	totalDuplicates := 0
	for _, truthPath := range matches {
		tf := readTruth(t, truthPath)
		samples := loadCorpusWAV(t, tf.WAV)
		result := DecodeStructured(samples, StructuredDecodeOptions{IncludeDeep: true})
		totalStrict += len(result.Strict)
		totalDeep += len(result.Deep)
		totalMerged += len(result.Messages)
		for _, msg := range result.Messages {
			if msg.Strict && msg.Deep {
				totalDuplicates++
			}
			if !msg.Strict && msg.Deep {
				totalDeepOnly++
				if msg.Mode != DecodeModeDeep || msg.Duplicate {
					t.Fatalf("bad deep-only flags for %s: %+v", tf.WAV, msg)
				}
			}
		}
	}
	if totalStrict != 144 {
		t.Fatalf("strict structured total=%d, want 144", totalStrict)
	}
	if totalDeep < 152 {
		t.Fatalf("deep structured total=%d, want at least 152", totalDeep)
	}
	if totalDeepOnly < 8 {
		t.Fatalf("deep-only structured total=%d, want at least 8", totalDeepOnly)
	}
	if totalDeep != totalDuplicates+totalDeepOnly {
		t.Fatalf("deep merge accounting mismatch: deep=%d duplicates=%d deep-only=%d",
			totalDeep, totalDuplicates, totalDeepOnly)
	}
	if totalMerged != totalStrict+totalDeepOnly {
		t.Fatalf("merged structured total=%d, want strict+deep-only=%d",
			totalMerged, totalStrict+totalDeepOnly)
	}
	if totalDuplicates != 144 {
		t.Fatalf("duplicate structured total=%d, want 144", totalDuplicates)
	}
}

func TestDecodeResultConvenienceFilters(t *testing.T) {
	samples := loadCorpusWAV(t, "20m_slot2.wav")
	result := DecodeStructured(samples, StructuredDecodeOptions{IncludeDeep: true})
	if len(result.StrictMessages()) != len(result.Strict) {
		t.Fatalf("strict filter got %d, want %d", len(result.StrictMessages()), len(result.Strict))
	}
	deepOnly := result.DeepOnly()
	want := map[string]bool{
		"JY5IB EA3DYI JN11": true,
		"CQ DL5CAT JO31":    true,
	}
	for _, msg := range deepOnly {
		delete(want, msg.Text)
	}
	for text := range want {
		t.Fatalf("missing deep-only text = %q", text)
	}
}
