// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"reflect"
	"testing"
)

// TestParallelDecodeMatchesSequential asserts that concurrent candidate
// analysis produces exactly the sequential decoder's output, in the same
// order, for every corpus fixture in both strict and deep modes.
func TestParallelDecodeMatchesSequential(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus decode comparison is slow")
	}
	modes := []struct {
		name    string
		options DecoderOptions
	}{
		{name: "strict", options: DecoderOptions{}},
		{name: "deep", options: DeepDecoderOptions()},
	}
	for _, wavPath := range corpusWAVFiles(t) {
		samples, err := loadWAV(wavPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range modes {
			sequential := mode.options
			sequential.Workers = 1
			want := DecodeMessagesWithOptions(samples, sequential)

			parallel := mode.options
			parallel.Workers = 0
			got := DecodeMessagesWithOptions(samples, parallel)

			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s %s: parallel decode diverged from sequential\nsequential: %v\nparallel:   %v",
					wavPath, mode.name, want, got)
			}
		}
	}
}

func TestParallelDecodeDiagnosticsTotalsMatchSequential(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus decode comparison is slow")
	}
	samples := loadCorpusWAV(t, "20m_slot2.wav")

	sequential := DeepDecoderOptions()
	sequential.Workers = 1
	seqReport := DecodeMessagesWithReport(samples, sequential)

	parallel := DeepDecoderOptions()
	parReport := DecodeMessagesWithReport(samples, parallel)

	if !reflect.DeepEqual(seqReport.Messages, parReport.Messages) {
		t.Fatalf("messages diverged\nsequential: %v\nparallel:   %v", seqReport.Messages, parReport.Messages)
	}
	seq, par := seqReport.Diagnostics, parReport.Diagnostics
	seq.Duration, par.Duration = 0, 0
	if !reflect.DeepEqual(seq, par) {
		t.Errorf("diagnostics diverged\nsequential: %+v\nparallel:   %+v", seq, par)
	}
}

func TestDecodeWorkerCount(t *testing.T) {
	cases := []struct {
		name       string
		workers    int
		candidates int
		wantMax    int
		wantMin    int
	}{
		{name: "explicit sequential", workers: 1, candidates: 100, wantMin: 1, wantMax: 1},
		{name: "capped by candidates", workers: 8, candidates: 3, wantMin: 3, wantMax: 3},
		{name: "zero candidates", workers: 0, candidates: 0, wantMin: 1, wantMax: 1},
		{name: "explicit cap", workers: 2, candidates: 100, wantMin: 2, wantMax: 2},
	}
	for _, tc := range cases {
		got := decodeWorkerCount(decodeOptions{workers: tc.workers}, tc.candidates)
		if got < tc.wantMin || got > tc.wantMax {
			t.Errorf("%s: decodeWorkerCount=%d, want in [%d,%d]", tc.name, got, tc.wantMin, tc.wantMax)
		}
	}
}

func TestCheckedDecodeRejectsNegativeWorkers(t *testing.T) {
	samples := make([]int16, ft8FrameSamples)
	_, err := DecodeMessagesChecked(samples, DecoderOptions{Workers: -1})
	if err == nil {
		t.Fatal("DecodeMessagesChecked accepted negative Workers")
	}
}
