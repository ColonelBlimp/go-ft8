// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"errors"
	"fmt"
	"math"
)

// ErrInvalidDecodeInput identifies caller-supplied PCM input that cannot be
// accepted by checked decode APIs.
var ErrInvalidDecodeInput = errors.New("ft8: invalid decode input")

// ErrInvalidDecoderOptions identifies decoder options that would be silently
// normalized or ignored by permissive decode APIs.
var ErrInvalidDecoderOptions = errors.New("ft8: invalid decoder options")

// DecodeInputError describes malformed caller-supplied PCM input rejected by a
// checked decode API.
type DecodeInputError struct {
	GotSamples  int
	WantSamples int
}

// Error returns a stable human-readable validation error.
func (e *DecodeInputError) Error() string {
	return fmt.Sprintf("%v: got %d samples, want %d samples for one 15-second slot",
		ErrInvalidDecodeInput, e.GotSamples, e.WantSamples)
}

// Unwrap returns ErrInvalidDecodeInput for errors.Is compatibility.
func (e *DecodeInputError) Unwrap() error {
	return ErrInvalidDecodeInput
}

// DecoderOptionError describes one invalid decoder option rejected by a checked
// decode API.
type DecoderOptionError struct {
	Field  string
	Reason string
}

// Error returns a stable human-readable validation error.
func (e *DecoderOptionError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%v: %s", ErrInvalidDecoderOptions, e.Reason)
	}
	return fmt.Sprintf("%v: %s: %s", ErrInvalidDecoderOptions, e.Field, e.Reason)
}

// Unwrap returns ErrInvalidDecoderOptions for errors.Is compatibility.
func (e *DecoderOptionError) Unwrap() error {
	return ErrInvalidDecoderOptions
}

// DecodeMessagesChecked validates input and options, then decodes one FT8 slot
// with aggregate diagnostics.
//
// It returns ErrInvalidDecodeInput for malformed slot input and
// ErrInvalidDecoderOptions for invalid options. Empty decoded output is not an
// error when the input and options are valid.
func DecodeMessagesChecked(iwave []int16, options DecoderOptions) (DecodeReport, error) {
	if err := validateDecodeRequest(iwave, options); err != nil {
		return DecodeReport{Diagnostics: newDecodeDiagnostics(iwave)}, err
	}
	return DecodeMessagesWithReport(iwave, options), nil
}

func validateDecodeRequest(iwave []int16, options DecoderOptions) error {
	if len(iwave) != ft8FrameSamples {
		return &DecodeInputError{
			GotSamples:  len(iwave),
			WantSamples: ft8FrameSamples,
		}
	}
	return validateDecoderOptions(options)
}

func validateDecoderOptions(options DecoderOptions) error {
	if !finiteNonNegative(options.SyncMin) {
		return invalidDecoderOption("SyncMin", "must be finite and non-negative")
	}
	if options.MaxCandidates < 0 {
		return invalidDecoderOption("MaxCandidates", "must be non-negative")
	}
	if options.MaxCandidates > ft8DefaultMaxCand {
		return invalidDecoderOption("MaxCandidates", "must not exceed %d", ft8DefaultMaxCand)
	}
	if options.MinFreqHz < 0 {
		return invalidDecoderOption("MinFreqHz", "must be non-negative")
	}
	if options.MaxFreqHz < 0 {
		return invalidDecoderOption("MaxFreqHz", "must be non-negative")
	}
	minFreq := ft8DefaultMinFreq
	if options.MinFreqHz > 0 {
		minFreq = options.MinFreqHz
	}
	maxFreq := ft8DefaultMaxFreq
	if options.MaxFreqHz > 0 {
		maxFreq = options.MaxFreqHz
	}
	if minFreq >= maxFreq {
		return invalidDecoderOption("MinFreqHz", "must be less than MaxFreqHz after defaults are applied")
	}
	if maxFreq > wantSampleRate/2 {
		return invalidDecoderOption("MaxFreqHz", "must not exceed Nyquist frequency %d Hz", wantSampleRate/2)
	}
	if err := validateDecoderBlocks(options.Blocks); err != nil {
		return err
	}
	if !finiteNonNegative(options.LLRWinsorFactor) {
		return invalidDecoderOption("LLRWinsorFactor", "must be finite and non-negative")
	}
	if options.HardSyncMin < 0 || options.HardSyncMin > ft8SyncSyms {
		return invalidDecoderOption("HardSyncMin", "must be between 0 and %d", ft8SyncSyms)
	}
	if options.CostasMinWins < 0 || options.CostasMinWins > ft8SyncSyms {
		return invalidDecoderOption("CostasMinWins", "must be between 0 and %d", ft8SyncSyms)
	}
	if !finiteNonNegative(options.CostasMinGeo) {
		return invalidDecoderOption("CostasMinGeo", "must be finite and non-negative")
	}
	if !finiteNonNegative(options.CostasMinBlock) {
		return invalidDecoderOption("CostasMinBlock", "must be finite and non-negative")
	}
	return nil
}

func validateDecoderBlocks(blocks []int) error {
	if len(blocks) == 0 {
		return nil
	}
	if len(blocks) > 4 {
		return invalidDecoderOption("Blocks", "may contain at most 4 entries")
	}
	seen := make(map[int]bool, len(blocks))
	for i, block := range blocks {
		field := fmt.Sprintf("Blocks[%d]", i)
		if block <= 0 {
			return invalidDecoderOption(field, "must be positive")
		}
		if block*3456 > ft8DecodeBufferSamples {
			return invalidDecoderOption(field, "entry %d exceeds the 15-second decoder buffer", block)
		}
		if seen[block] {
			return invalidDecoderOption(field, "entries must be unique")
		}
		seen[block] = true
	}
	return nil
}

func finiteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func invalidDecoderOption(field, format string, args ...any) error {
	return &DecoderOptionError{
		Field:  field,
		Reason: fmt.Sprintf(format, args...),
	}
}
