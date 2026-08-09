// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
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
	// MetricLDPCAttempts is the number of non-AP metric passes handed to the LDPC decoder.
	MetricLDPCAttempts int
	// APAttempts is the number of a-priori passes handed to the LDPC decoder.
	APAttempts int
	// APAttemptsByProfile records AP attempts by profile name.
	APAttemptsByProfile map[string]int
	// APAttemptsBySource records AP attempts by upstream or built-in AP source.
	APAttemptsBySource map[string]int
	// APSuccesses counts AP passes that produced an accepted candidate decode.
	APSuccesses int
	// APSuccessesByProfile records accepted AP decodes by profile name.
	APSuccessesByProfile map[string]int
	// APSuccessesBySource records accepted AP decodes by upstream or built-in AP source.
	APSuccessesBySource map[string]int
	// APRejectedAfterLDPC counts AP passes that produced a valid codeword that
	// was later rejected by hard-error or all-zero guards.
	APRejectedAfterLDPC int
	// APRejectedAfterLDPCByProfile records post-LDPC AP rejections by profile name.
	APRejectedAfterLDPCByProfile map[string]int
	// APRejectedAfterLDPCBySource records post-LDPC AP rejections by AP source.
	APRejectedAfterLDPCBySource map[string]int
	// APCallHints is the number of normalized call hints supplied to the decoder.
	APCallHints int
	// APHintProfilesScored is the number of call-field hint profiles cheaply scored.
	APHintProfilesScored int
	// APHintHypothesesSelected is the number of scored hint hypotheses selected for LDPC.
	APHintHypothesesSelected int
	// APHintHypothesesBelowThreshold is the number of scored hint hypotheses rejected by the cheap score gate.
	APHintHypothesesBelowThreshold int
	// LDPCFailures counts LDPC attempts that did not produce a valid codeword.
	LDPCFailures int
	// RejectedHardErrors counts decoded codewords rejected by hard-error bounds.
	RejectedHardErrors int
	// RejectedAllZero counts decoded all-zero codewords rejected as invalid.
	RejectedAllZero int
	// UnpackFailures is retained for source compatibility. Verified payloads are
	// returned with their ParseStatus instead of being rejected during unpack.
	// Deprecated: this counter is no longer incremented.
	UnpackFailures int
	// RejectedMessageFilter is retained for source compatibility. Publication
	// filtering belongs to the caller.
	// Deprecated: this counter is no longer incremented.
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

// merge folds candidate-level work counters recorded by one concurrent decode
// worker into the caller's diagnostics. Only counters touched during candidate
// analysis are merged; slot-level fields such as Duration, InputSamples,
// BlocksSearched, and the commit-phase counters stay owned by the caller.
func (diagnostics *DecodeDiagnostics) merge(worker *DecodeDiagnostics) {
	diagnostics.RejectedHardSync += worker.RejectedHardSync
	diagnostics.RejectedCostas += worker.RejectedCostas
	diagnostics.LDPCAttempts += worker.LDPCAttempts
	diagnostics.MetricLDPCAttempts += worker.MetricLDPCAttempts
	diagnostics.APAttempts += worker.APAttempts
	diagnostics.APSuccesses += worker.APSuccesses
	diagnostics.APRejectedAfterLDPC += worker.APRejectedAfterLDPC
	if worker.APCallHints > diagnostics.APCallHints {
		diagnostics.APCallHints = worker.APCallHints
	}
	diagnostics.APHintProfilesScored += worker.APHintProfilesScored
	diagnostics.APHintHypothesesSelected += worker.APHintHypothesesSelected
	diagnostics.APHintHypothesesBelowThreshold += worker.APHintHypothesesBelowThreshold
	diagnostics.LDPCFailures += worker.LDPCFailures
	diagnostics.RejectedHardErrors += worker.RejectedHardErrors
	diagnostics.RejectedAllZero += worker.RejectedAllZero
	diagnostics.UnpackFailures += worker.UnpackFailures
	diagnostics.RejectedMessageFilter += worker.RejectedMessageFilter
	diagnostics.APAttemptsByProfile = mergeCountMap(diagnostics.APAttemptsByProfile, worker.APAttemptsByProfile)
	diagnostics.APAttemptsBySource = mergeCountMap(diagnostics.APAttemptsBySource, worker.APAttemptsBySource)
	diagnostics.APSuccessesByProfile = mergeCountMap(diagnostics.APSuccessesByProfile, worker.APSuccessesByProfile)
	diagnostics.APSuccessesBySource = mergeCountMap(diagnostics.APSuccessesBySource, worker.APSuccessesBySource)
	diagnostics.APRejectedAfterLDPCByProfile = mergeCountMap(diagnostics.APRejectedAfterLDPCByProfile, worker.APRejectedAfterLDPCByProfile)
	diagnostics.APRejectedAfterLDPCBySource = mergeCountMap(diagnostics.APRejectedAfterLDPCBySource, worker.APRejectedAfterLDPCBySource)
}

func mergeCountMap(dst, src map[string]int) map[string]int {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]int, len(src))
	}
	for key, count := range src {
		dst[key] += count
	}
	return dst
}

func (diagnostics *DecodeDiagnostics) recordLDPCAttempt(apProfileName string, apSource string) {
	diagnostics.LDPCAttempts++
	if apProfileName == "" {
		diagnostics.MetricLDPCAttempts++
		return
	}
	diagnostics.APAttempts++
	if diagnostics.APAttemptsByProfile == nil {
		diagnostics.APAttemptsByProfile = make(map[string]int)
	}
	diagnostics.APAttemptsByProfile[apProfileName]++
	if apSource == "" {
		apSource = "ap"
	}
	if diagnostics.APAttemptsBySource == nil {
		diagnostics.APAttemptsBySource = make(map[string]int)
	}
	diagnostics.APAttemptsBySource[apSource]++
}

func (diagnostics *DecodeDiagnostics) recordAPSuccess(apProfileName string, apSource string) {
	if apProfileName == "" {
		return
	}
	diagnostics.APSuccesses++
	if diagnostics.APSuccessesByProfile == nil {
		diagnostics.APSuccessesByProfile = make(map[string]int)
	}
	diagnostics.APSuccessesByProfile[apProfileName]++
	if apSource == "" {
		apSource = "ap"
	}
	if diagnostics.APSuccessesBySource == nil {
		diagnostics.APSuccessesBySource = make(map[string]int)
	}
	diagnostics.APSuccessesBySource[apSource]++
}

func (diagnostics *DecodeDiagnostics) recordAPRejectedAfterLDPC(apProfileName string, apSource string) {
	if apProfileName == "" {
		return
	}
	diagnostics.APRejectedAfterLDPC++
	if diagnostics.APRejectedAfterLDPCByProfile == nil {
		diagnostics.APRejectedAfterLDPCByProfile = make(map[string]int)
	}
	diagnostics.APRejectedAfterLDPCByProfile[apProfileName]++
	if apSource == "" {
		apSource = "ap"
	}
	if diagnostics.APRejectedAfterLDPCBySource == nil {
		diagnostics.APRejectedAfterLDPCBySource = make(map[string]int)
	}
	diagnostics.APRejectedAfterLDPCBySource[apSource]++
}

func (diagnostics *DecodeDiagnostics) recordAPHintScored(hints int, scored int, selected int, belowThreshold int) {
	diagnostics.APCallHints = hints
	diagnostics.APHintProfilesScored += scored
	diagnostics.APHintHypothesesSelected += selected
	diagnostics.APHintHypothesesBelowThreshold += belowThreshold
}
