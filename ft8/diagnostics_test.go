// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestDecodeMessagesWithReportEmptyInputDiagnostics(t *testing.T) {
	report := DecodeMessagesWithReport(nil, DecoderOptions{})
	if len(report.Messages) != 0 {
		t.Fatalf("Messages length got %d, want 0", len(report.Messages))
	}
	diag := report.Diagnostics
	if diag.InputSamples != 0 {
		t.Fatalf("InputSamples got %d, want 0", diag.InputSamples)
	}
	if diag.ExpectedSamples != ft8FrameSamples {
		t.Fatalf("ExpectedSamples got %d, want %d", diag.ExpectedSamples, ft8FrameSamples)
	}
	if diag.ShortInputSamples != ft8FrameSamples {
		t.Fatalf("ShortInputSamples got %d, want %d", diag.ShortInputSamples, ft8FrameSamples)
	}
	if diag.CandidateSearches == 0 {
		t.Fatalf("CandidateSearches got 0, want nonzero")
	}
	if len(diag.BlocksSearched) != 1 || diag.BlocksSearched[0] != 50 {
		t.Fatalf("BlocksSearched got %v, want [50]", diag.BlocksSearched)
	}
	if diag.UniqueMessages != 0 {
		t.Fatalf("UniqueMessages got %d, want 0", diag.UniqueMessages)
	}
	if diag.Duration <= 0 {
		t.Fatalf("Duration got %s, want positive", diag.Duration)
	}
}

func TestDecodeMessagesWithReportMatchesDecodeMessages(t *testing.T) {
	samples := loadCorpusWAV(t, "20m_slot1.wav")
	want := DecodeMessages(samples)
	report := DecodeMessagesWithReport(samples, DecoderOptions{})
	if len(report.Messages) != len(want) {
		t.Fatalf("Messages length got %d, want %d", len(report.Messages), len(want))
	}
	for i := range want {
		if report.Messages[i].Text != want[i].Text {
			t.Fatalf("Messages[%d] got %q, want %q", i, report.Messages[i].Text, want[i].Text)
		}
	}
	diag := report.Diagnostics
	if diag.InputSamples != len(samples) {
		t.Fatalf("InputSamples got %d, want %d", diag.InputSamples, len(samples))
	}
	if diag.CandidatesFound == 0 {
		t.Fatalf("CandidatesFound got 0, want nonzero")
	}
	if diag.CandidatesAnalyzed == 0 {
		t.Fatalf("CandidatesAnalyzed got 0, want nonzero")
	}
	if diag.DecodedCandidates < len(report.Messages) {
		t.Fatalf("DecodedCandidates got %d, want at least %d", diag.DecodedCandidates, len(report.Messages))
	}
	if diag.UniqueMessages != len(report.Messages) {
		t.Fatalf("UniqueMessages got %d, want %d", diag.UniqueMessages, len(report.Messages))
	}
}

func TestDecoderDecodeMessagesWithReportAdvancesState(t *testing.T) {
	decoder := NewDecoder()
	samples := loadCorpusWAV(t, "20m_slot1.wav")
	report := decoder.DecodeMessagesWithReport(samples)
	if report.Diagnostics.InputSamples != len(samples) {
		t.Fatalf("InputSamples got %d, want %d", report.Diagnostics.InputSamples, len(samples))
	}
	if len(report.Messages) == 0 {
		t.Fatalf("stateful DecodeMessagesWithReport returned no messages")
	}
	if decoder.seq != 1 {
		t.Fatalf("decoder seq got %d, want 1", decoder.seq)
	}
}
