// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"errors"
	"testing"
)

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

func TestDecodeStructuredWithReportDiagnostics(t *testing.T) {
	samples := loadCorpusWAV(t, "20m_slot2.wav")
	want := DecodeStructured(samples, StructuredDecodeOptions{IncludeDeep: true})
	report := DecodeStructuredWithReport(samples, StructuredDecodeOptions{IncludeDeep: true})
	if len(report.Result.Messages) != len(want.Messages) {
		t.Fatalf("Messages length got %d, want %d", len(report.Result.Messages), len(want.Messages))
	}
	if len(report.StrictReport.Messages) != len(report.Result.Strict) {
		t.Fatalf("strict report/result mismatch: report=%d result=%d",
			len(report.StrictReport.Messages), len(report.Result.Strict))
	}
	if len(report.DeepReport.Messages) != len(report.Result.Deep) {
		t.Fatalf("deep report/result mismatch: report=%d result=%d",
			len(report.DeepReport.Messages), len(report.Result.Deep))
	}
	if report.StrictReport.Diagnostics.InputSamples != len(samples) {
		t.Fatalf("strict InputSamples got %d, want %d",
			report.StrictReport.Diagnostics.InputSamples, len(samples))
	}
	if report.DeepReport.Diagnostics.CandidateSearches == 0 {
		t.Fatalf("deep CandidateSearches got 0, want nonzero")
	}
}

func TestDecodeStructuredCheckedRejectsInvalidInput(t *testing.T) {
	report, err := DecodeStructuredChecked(nil, StructuredDecodeOptions{IncludeDeep: true})
	if !errors.Is(err, ErrInvalidDecodeInput) {
		t.Fatalf("error got %v, want ErrInvalidDecodeInput", err)
	}
	if report.StrictReport.Diagnostics.ShortInputSamples != ft8FrameSamples {
		t.Fatalf("ShortInputSamples got %d, want %d",
			report.StrictReport.Diagnostics.ShortInputSamples, ft8FrameSamples)
	}
	if len(report.Result.Messages) != 0 {
		t.Fatalf("Messages length got %d, want 0", len(report.Result.Messages))
	}
}

func TestDecodeStructuredCheckedRejectsInvalidDeepOptionsBeforeDecode(t *testing.T) {
	samples := make([]int16, ft8FrameSamples)
	report, err := DecodeStructuredChecked(samples, StructuredDecodeOptions{
		IncludeDeep: true,
		DeepOptions: DecoderOptions{Blocks: []int{50, 50}},
	})
	if !errors.Is(err, ErrInvalidDecoderOptions) {
		t.Fatalf("error got %v, want ErrInvalidDecoderOptions", err)
	}
	if report.StrictReport.Diagnostics.CandidateSearches != 0 {
		t.Fatalf("strict CandidateSearches got %d, want 0", report.StrictReport.Diagnostics.CandidateSearches)
	}
	if report.DeepReport.Diagnostics.CandidateSearches != 0 {
		t.Fatalf("deep CandidateSearches got %d, want 0", report.DeepReport.Diagnostics.CandidateSearches)
	}
}

func TestDecodeStructuredCheckedAllowsEmptyDecode(t *testing.T) {
	samples := make([]int16, ft8FrameSamples)
	report, err := DecodeStructuredChecked(samples, StructuredDecodeOptions{})
	if err != nil {
		t.Fatalf("DecodeStructuredChecked returned error for valid silence: %v", err)
	}
	if len(report.Result.Messages) != 0 {
		t.Fatalf("Messages length got %d, want 0", len(report.Result.Messages))
	}
	if report.StrictReport.Diagnostics.InputSamples != len(samples) {
		t.Fatalf("InputSamples got %d, want %d", report.StrictReport.Diagnostics.InputSamples, len(samples))
	}
}

func TestDecoderDecodeStructuredWithReportAdvancesState(t *testing.T) {
	decoder := NewDecoder()
	samples := loadCorpusWAV(t, "20m_slot1.wav")
	report := decoder.DecodeStructuredWithReport(samples)
	if len(report.Result.Messages) == 0 {
		t.Fatalf("stateful DecodeStructuredWithReport returned no messages")
	}
	if decoder.seq != 1 {
		t.Fatalf("decoder seq got %d, want 1", decoder.seq)
	}
}

func TestDecoderDecodeStructuredCheckedDoesNotAdvanceStateOnError(t *testing.T) {
	decoder := NewDecoder()
	_, err := decoder.DecodeStructuredChecked(nil)
	if !errors.Is(err, ErrInvalidDecodeInput) {
		t.Fatalf("error got %v, want ErrInvalidDecodeInput", err)
	}
	if decoder.seq != 0 {
		t.Fatalf("decoder seq got %d, want 0", decoder.seq)
	}
}
