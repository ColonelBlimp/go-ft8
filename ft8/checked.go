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
		return fmt.Errorf("%w: got %d samples, want %d samples for one 15-second slot", ErrInvalidDecodeInput, len(iwave), ft8FrameSamples)
	}
	return validateDecoderOptions(options)
}

func validateDecoderOptions(options DecoderOptions) error {
	if !finiteNonNegative(options.SyncMin) {
		return invalidDecoderOption("SyncMin must be finite and non-negative")
	}
	if options.MaxCandidates < 0 {
		return invalidDecoderOption("MaxCandidates must be non-negative")
	}
	if options.MaxCandidates > ft8DefaultMaxCand {
		return invalidDecoderOption("MaxCandidates must not exceed %d", ft8DefaultMaxCand)
	}
	if options.MinFreqHz < 0 {
		return invalidDecoderOption("MinFreqHz must be non-negative")
	}
	if options.MaxFreqHz < 0 {
		return invalidDecoderOption("MaxFreqHz must be non-negative")
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
		return invalidDecoderOption("MinFreqHz must be less than MaxFreqHz after defaults are applied")
	}
	if maxFreq > wantSampleRate/2 {
		return invalidDecoderOption("MaxFreqHz must not exceed Nyquist frequency %d Hz", wantSampleRate/2)
	}
	if err := validateDecoderBlocks(options.Blocks); err != nil {
		return err
	}
	if !finiteNonNegative(options.LLRWinsorFactor) {
		return invalidDecoderOption("LLRWinsorFactor must be finite and non-negative")
	}
	if options.HardSyncMin < 0 || options.HardSyncMin > ft8SyncSyms {
		return invalidDecoderOption("HardSyncMin must be between 0 and %d", ft8SyncSyms)
	}
	if options.CostasMinWins < 0 || options.CostasMinWins > ft8SyncSyms {
		return invalidDecoderOption("CostasMinWins must be between 0 and %d", ft8SyncSyms)
	}
	if !finiteNonNegative(options.CostasMinGeo) {
		return invalidDecoderOption("CostasMinGeo must be finite and non-negative")
	}
	if !finiteNonNegative(options.CostasMinBlock) {
		return invalidDecoderOption("CostasMinBlock must be finite and non-negative")
	}
	return nil
}

func validateDecoderBlocks(blocks []int) error {
	if len(blocks) == 0 {
		return nil
	}
	if len(blocks) > 4 {
		return invalidDecoderOption("Blocks may contain at most 4 entries")
	}
	seen := make(map[int]bool, len(blocks))
	for _, blocks := range blocks {
		if blocks <= 0 {
			return invalidDecoderOption("Blocks entries must be positive")
		}
		if blocks*3456 > ft8DecodeBufferSamples {
			return invalidDecoderOption("Blocks entry %d exceeds the 15-second decoder buffer", blocks)
		}
		if seen[blocks] {
			return invalidDecoderOption("Blocks entries must be unique")
		}
		seen[blocks] = true
	}
	return nil
}

func finiteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func invalidDecoderOption(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidDecoderOptions, fmt.Sprintf(format, args...))
}
