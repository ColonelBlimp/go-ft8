// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"errors"
	"math"
	"testing"
)

func TestDecodeMessagesCheckedRejectsInvalidInput(t *testing.T) {
	report, err := DecodeMessagesChecked(nil, DecoderOptions{})
	if !errors.Is(err, ErrInvalidDecodeInput) {
		t.Fatalf("error got %v, want ErrInvalidDecodeInput", err)
	}
	var inputErr *DecodeInputError
	if !errors.As(err, &inputErr) {
		t.Fatalf("error got %T, want *DecodeInputError", err)
	}
	if inputErr.GotSamples != 0 || inputErr.WantSamples != ft8FrameSamples {
		t.Fatalf("input error got got=%d want=%d, want got=0 want=%d",
			inputErr.GotSamples, inputErr.WantSamples, ft8FrameSamples)
	}
	if report.Diagnostics.InputSamples != 0 {
		t.Fatalf("InputSamples got %d, want 0", report.Diagnostics.InputSamples)
	}
	if report.Diagnostics.ShortInputSamples != ft8FrameSamples {
		t.Fatalf("ShortInputSamples got %d, want %d", report.Diagnostics.ShortInputSamples, ft8FrameSamples)
	}
}

func TestDecodeMessagesCheckedRejectsInvalidOptions(t *testing.T) {
	samples := make([]int16, ft8FrameSamples)
	tests := []struct {
		name    string
		options DecoderOptions
	}{
		{name: "negative sync", options: DecoderOptions{SyncMin: -1}},
		{name: "nan sync", options: DecoderOptions{SyncMin: math.NaN()}},
		{name: "negative max candidates", options: DecoderOptions{MaxCandidates: -1}},
		{name: "too many candidates", options: DecoderOptions{MaxCandidates: ft8DefaultMaxCand + 1}},
		{name: "bad frequency range", options: DecoderOptions{MinFreqHz: 3200, MaxFreqHz: 200}},
		{name: "over nyquist", options: DecoderOptions{MaxFreqHz: wantSampleRate/2 + 1}},
		{name: "invalid block", options: DecoderOptions{Blocks: []int{50, 0}}},
		{name: "duplicate blocks", options: DecoderOptions{Blocks: []int{50, 50}}},
		{name: "too many blocks", options: DecoderOptions{Blocks: []int{50, 41, 43, 45, 47}}},
		{name: "hard sync too high", options: DecoderOptions{HardSyncMin: ft8SyncSyms + 1}},
		{name: "costas wins too high", options: DecoderOptions{CostasMinWins: ft8SyncSyms + 1}},
		{name: "nan costas geo", options: DecoderOptions{CostasMinGeo: math.NaN()}},
		{name: "too many AP hypotheses", options: DecoderOptions{MaxAPCallHypotheses: ft8MaxAPCallHypotheses + 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeMessagesChecked(samples, test.options)
			if !errors.Is(err, ErrInvalidDecoderOptions) {
				t.Fatalf("error got %v, want ErrInvalidDecoderOptions", err)
			}
		})
	}
}

func TestDecodeMessagesCheckedReturnsTypedOptionError(t *testing.T) {
	samples := make([]int16, ft8FrameSamples)
	_, err := DecodeMessagesChecked(samples, DecoderOptions{MaxCandidates: -1})
	if !errors.Is(err, ErrInvalidDecoderOptions) {
		t.Fatalf("error got %v, want ErrInvalidDecoderOptions", err)
	}
	var optionErr *DecoderOptionError
	if !errors.As(err, &optionErr) {
		t.Fatalf("error got %T, want *DecoderOptionError", err)
	}
	if optionErr.Field != "MaxCandidates" {
		t.Fatalf("option field got %q, want MaxCandidates", optionErr.Field)
	}
	if optionErr.Reason == "" {
		t.Fatal("option reason is empty")
	}
}

func TestDecodeMessagesCheckedReturnsTypedBlockOptionError(t *testing.T) {
	samples := make([]int16, ft8FrameSamples)
	_, err := DecodeMessagesChecked(samples, DecoderOptions{Blocks: []int{50, 50}})
	if !errors.Is(err, ErrInvalidDecoderOptions) {
		t.Fatalf("error got %v, want ErrInvalidDecoderOptions", err)
	}
	var optionErr *DecoderOptionError
	if !errors.As(err, &optionErr) {
		t.Fatalf("error got %T, want *DecoderOptionError", err)
	}
	if optionErr.Field != "Blocks[1]" {
		t.Fatalf("option field got %q, want Blocks[1]", optionErr.Field)
	}
	if optionErr.Reason == "" {
		t.Fatal("option reason is empty")
	}
}

func TestDecodeMessagesCheckedAllowsEmptyDecode(t *testing.T) {
	samples := make([]int16, ft8FrameSamples)
	report, err := DecodeMessagesChecked(samples, DecoderOptions{})
	if err != nil {
		t.Fatalf("DecodeMessagesChecked returned error for valid silence: %v", err)
	}
	if len(report.Messages) != 0 {
		t.Fatalf("Messages length got %d, want 0", len(report.Messages))
	}
	if report.Diagnostics.InputSamples != len(samples) {
		t.Fatalf("InputSamples got %d, want %d", report.Diagnostics.InputSamples, len(samples))
	}
	if report.Diagnostics.ShortInputSamples != 0 {
		t.Fatalf("ShortInputSamples got %d, want 0", report.Diagnostics.ShortInputSamples)
	}
}

func TestDecoderDecodeMessagesCheckedDoesNotAdvanceStateOnError(t *testing.T) {
	decoder := NewDecoder()
	_, err := decoder.DecodeMessagesChecked(nil)
	if !errors.Is(err, ErrInvalidDecodeInput) {
		t.Fatalf("error got %v, want ErrInvalidDecodeInput", err)
	}
	if decoder.seq != 0 {
		t.Fatalf("decoder seq got %d, want 0", decoder.seq)
	}
}

func TestDecoderDecodeMessagesCheckedRejectsConstructorOptions(t *testing.T) {
	decoder := NewDecoderWithOptions(DecoderOptions{Blocks: []int{50, 50}})
	_, err := decoder.DecodeMessagesChecked(make([]int16, ft8FrameSamples))
	if !errors.Is(err, ErrInvalidDecoderOptions) {
		t.Fatalf("error got %v, want ErrInvalidDecoderOptions", err)
	}
	if decoder.seq != 0 {
		t.Fatalf("decoder seq got %d, want 0", decoder.seq)
	}
}
