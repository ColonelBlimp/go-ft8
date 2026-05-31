// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "time"

// DecodeReport contains decoded messages plus operational diagnostics for one
// decode attempt.
type DecodeReport struct {
	Messages    []DecodedMessage
	Diagnostics DecodeDiagnostics
}

// DecodeDiagnostics summarizes the work performed by a decode attempt.
//
// Counts are aggregate counters intended for production logging and monitoring,
// not a stable per-candidate trace format.
type DecodeDiagnostics struct {
	// Duration is the wall-clock time spent in the decode call.
	Duration time.Duration
	// InputSamples is the number of int16 PCM samples supplied by the caller.
	InputSamples int
	// ExpectedSamples is the nominal 15-second 12 kHz FT8 slot length.
	ExpectedSamples int
	// ShortInputSamples is positive when InputSamples is shorter than a full slot.
	ShortInputSamples int
	// ExtraInputSamples is positive when samples beyond the decoder buffer were ignored.
	ExtraInputSamples int
	// BlocksSearched records the 3456-sample block counts searched by the decoder.
	BlocksSearched []int
	// CandidateSearches is the number of sync/candidate search passes performed.
	CandidateSearches int
	// CandidatesFound is the number of coarse candidates returned by sync search.
	CandidatesFound int
	// CandidatesAnalyzed is the number of candidates refined and sent to decode gates.
	CandidatesAnalyzed int
	// RejectedHardSync counts candidates rejected by the refined hard-sync gate.
	RejectedHardSync int
	// RejectedCostas counts candidates rejected by Costas evidence gates.
	RejectedCostas int
	// LDPCAttempts is the number of metric/AP passes handed to the LDPC decoder.
	LDPCAttempts int
	// LDPCFailures counts LDPC attempts that did not produce a valid codeword.
	LDPCFailures int
	// RejectedHardErrors counts decoded codewords rejected by hard-error bounds.
	RejectedHardErrors int
	// RejectedAllZero counts decoded all-zero codewords rejected as invalid.
	RejectedAllZero int
	// UnpackFailures counts valid codewords that could not be unpacked to text.
	UnpackFailures int
	// RejectedMessageFilter counts decoded messages rejected by package filters.
	RejectedMessageFilter int
	// DecodedCandidates counts candidate decodes before duplicate suppression.
	DecodedCandidates int
	// DuplicateMessages counts decoded messages suppressed because the text was already seen.
	DuplicateMessages int
	// UniqueMessages is the final number of messages returned to the caller.
	UniqueMessages int
	// Subtractions counts decoded signal subtraction operations.
	Subtractions int
	// A7Hints is the number of retained A7 hints supplied to the decode attempt.
	A7Hints int
	// A7Decoded is the number of messages recovered by A7 hint decoding.
	A7Decoded int
}

// DecodeMessagesWithReport decodes one FT8 slot and returns aggregate
// diagnostics for production logging or empty-result investigation.
//
// This is the permissive stateless report API: short input is zero-padded,
// excess input beyond the decoder buffer is ignored, and an empty result is a
// normal no-decode outcome. Use DecodeMessagesChecked when caller input and
// options should be validated before decode work starts.
func DecodeMessagesWithReport(iwave []int16, options DecoderOptions) DecodeReport {
	var hashes hashTable
	return decodeMessagesReportCore(iwave, nil, &hashes, normalizeDecoderOptions(options))
}

func decodeMessagesReportCore(iwave []int16, a7Hints []a7Hint, hashes *hashTable, options decodeOptions) DecodeReport {
	start := time.Now()
	diagnostics := newDecodeDiagnostics(iwave)
	messages := decodeMessagesCoreWithDiagnostics(iwave, a7Hints, hashes, options, &diagnostics)
	diagnostics.Duration = time.Since(start)
	diagnostics.UniqueMessages = len(messages)
	return DecodeReport{
		Messages:    messages,
		Diagnostics: diagnostics,
	}
}

func newDecodeDiagnostics(iwave []int16) DecodeDiagnostics {
	diagnostics := DecodeDiagnostics{
		InputSamples:    len(iwave),
		ExpectedSamples: ft8FrameSamples,
	}
	if len(iwave) < ft8FrameSamples {
		diagnostics.ShortInputSamples = ft8FrameSamples - len(iwave)
	}
	if len(iwave) > ft8DecodeBufferSamples {
		diagnostics.ExtraInputSamples = len(iwave) - ft8DecodeBufferSamples
	}
	return diagnostics
}
