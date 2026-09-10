// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/ColonelBlimp/go-ft8/ft8"
	"github.com/ColonelBlimp/go-ft8/internal/ftx77"
)

const (
	defaultMinFreqHz   = 200
	defaultMaxFreqHz   = 4910
	defaultSyncMin     = 1.2
	defaultMaxCand     = 100
	defaultHardSyncMin = 8
	maxHardErrors      = 36
	llrScale           = 2.83
)

// Payload77 is the source-encoded message shared by FT4 and FT8.
type Payload77 = ft8.Payload77

// ParseStatus describes whether a verified payload could be rendered.
type ParseStatus = ft8.ParseStatus

const (
	ParseStatusUnset       = ft8.ParseStatusUnset
	ParseStatusParsed      = ft8.ParseStatusParsed
	ParseStatusUnsupported = ft8.ParseStatusUnsupported
	ParseStatusUnknownType = ft8.ParseStatusUnknownType
	ParseStatusInvalid     = ft8.ParseStatusInvalid
)

// DecodedMessage is shared with FT8 so FT-family consumers can use one decode
// DTO without copying fields at the mode boundary.
type DecodedMessage = ft8.DecodedMessage

// DecoderOptions controls the basic FT4 signal search. Its zero value uses
// the WSJT-X-derived defaults.
type DecoderOptions struct {
	SyncMin       float64
	MaxCandidates int
	MinFreqHz     int
	MaxFreqHz     int
	HardSyncMin   int
}

type decodeOptions struct {
	syncMin       float64
	maxCandidates int
	minFreqHz     int
	maxFreqHz     int
	hardSyncMin   int
}

func normalizeDecoderOptions(options DecoderOptions) decodeOptions {
	out := decodeOptions{
		syncMin:       defaultSyncMin,
		maxCandidates: defaultMaxCand,
		minFreqHz:     defaultMinFreqHz,
		maxFreqHz:     defaultMaxFreqHz,
		hardSyncMin:   defaultHardSyncMin,
	}
	if options.SyncMin > 0 {
		out.syncMin = options.SyncMin
	}
	if options.MaxCandidates > 0 {
		out.maxCandidates = min(options.MaxCandidates, defaultMaxCand)
	}
	if options.MinFreqHz > 0 {
		out.minFreqHz = options.MinFreqHz
	}
	if options.MaxFreqHz > 0 {
		out.maxFreqHz = options.MaxFreqHz
	}
	if options.HardSyncMin > 0 {
		out.hardSyncMin = min(options.HardSyncMin, SyncSymbols)
	}
	return out
}

// DecodeMessages decodes one 7.5-second FT4 slot from 12 kHz mono signed-16-bit
// PCM. Short input is zero-padded and samples beyond the working interval are
// ignored.
func DecodeMessages(iwave []int16) []DecodedMessage {
	return DecodeMessagesWithOptions(iwave, DecoderOptions{})
}

// DecodeMessagesWithOptions decodes one FT4 slot with explicit search options.
func DecodeMessagesWithOptions(iwave []int16, options DecoderOptions) []DecodedMessage {
	return decodeMessages(iwave, normalizeDecoderOptions(options), nil)
}

func decodeMessages(iwave []int16, options decodeOptions, diagnostics *DecodeDiagnostics) []DecodedMessage {
	workspace := acquireDecodeWorkspace()
	defer releaseDecodeWorkspace(workspace)
	dd := workspace.samples
	for i := 0; i < len(dd) && i < len(iwave); i++ {
		dd[i] = float64(iwave[i])
	}
	if !hasSignal(dd) {
		return nil
	}

	candidates := findCandidates(dd, options, &workspace.candidates)
	if diagnostics != nil {
		diagnostics.CandidatesFound = len(candidates)
	}
	if len(candidates) == 0 {
		return nil
	}

	ds := &workspace.downsampler
	ds.reset(dd)
	seen := make(map[Payload77]bool)
	out := make([]DecodedMessage, 0, len(candidates))
	for _, candidate := range candidates {
		if diagnostics != nil {
			diagnostics.CandidatesAnalyzed++
		}
		baseband := ds.at(candidate.freqHz)
		normalizeComplex(baseband)
		start, deltaHz, sync := refineSync(baseband)
		if sync < options.syncMin {
			if diagnostics != nil {
				diagnostics.RejectedSync++
			}
			continue
		}

		freqHz := candidate.freqHz + deltaHz
		if freqHz <= 10 || freqHz >= 4990 {
			continue
		}
		if deltaHz != 0 {
			baseband = ds.at(freqHz)
			normalizeComplex(baseband)
		}
		frame := extractFrame(baseband, start)
		correlations := symbolCorrelations(&frame)
		hardSync := hardSyncCount(&correlations)
		if hardSync < options.hardSyncMin {
			if diagnostics != nil {
				diagnostics.RejectedHardSync++
			}
			continue
		}

		metrics := ft4BitMetrics(&correlations)
		if syncBitQuality(&metrics[0]) < 20 {
			if diagnostics != nil {
				diagnostics.RejectedHardSync++
			}
			continue
		}

		var decoded ftx77.DecodeResult
		ok := false
		for pass := range metrics {
			llr := dataLLR(&metrics[pass])
			if diagnostics != nil {
				diagnostics.LDPCAttempts++
			}
			decoded, ok = ftx77.Decode17491BP(&llr)
			if ok {
				break
			}
		}
		if !ok {
			if diagnostics != nil {
				diagnostics.LDPCFailures++
			}
			continue
		}
		if decoded.HardErrors > maxHardErrors || allZero77(decoded.Codeword) {
			continue
		}

		var payloadBits [77]byte
		for i := range payloadBits {
			payloadBits[i] = byte(decoded.Codeword[i]^scramblingVector[i]) & 1
		}
		payload := ft8.Payload77FromBits(payloadBits)
		if seen[payload] {
			if diagnostics != nil {
				diagnostics.DuplicateMessages++
			}
			continue
		}
		seen[payload] = true
		text, status := ft8.ParsePayload77(payload)
		out = append(out, DecodedMessage{
			Text:        text,
			Payload:     payload,
			ParseStatus: status,
			Provenance: ft8.DecodeProvenance{
				Algorithm: ft8.DecodeAlgorithmBP,
			},
			SNR:        candidate.snr(),
			FreqHz:     freqHz,
			DTSec:      float64(start)/downRate - SyncStartSeconds,
			Sync:       sync,
			HardSync:   hardSync,
			HardErrors: decoded.HardErrors,
			DMin:       decoded.DMin,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].FreqHz == out[j].FreqHz {
			return out[i].DTSec < out[j].DTSec
		}
		return out[i].FreqHz < out[j].FreqHz
	})
	if diagnostics != nil {
		diagnostics.UniqueMessages = len(out)
	}
	return out
}

func hasSignal(samples []float64) bool {
	for _, sample := range samples {
		if sample != 0 {
			return true
		}
	}
	return false
}

func allZero77(codeword [174]int8) bool {
	for i := 0; i < 77; i++ {
		if codeword[i] != 0 {
			return false
		}
	}
	return true
}

type candidate struct {
	freqHz   float64
	strength float64
}

func (candidate candidate) snr() int {
	if candidate.strength <= 1 {
		return -21
	}
	snr := 10*math.Log10(candidate.strength-1) - 14.8
	return max(-21, int(math.Round(snr)))
}

// DecodeReport contains messages and operational counters for one FT4 slot.
type DecodeReport struct {
	Messages    []DecodedMessage
	Diagnostics DecodeDiagnostics
}

// DecodeDiagnostics summarizes a basic FT4 decode attempt.
type DecodeDiagnostics struct {
	Duration           time.Duration
	InputSamples       int
	ExpectedSamples    int
	ShortInputSamples  int
	ExtraInputSamples  int
	CandidatesFound    int
	CandidatesAnalyzed int
	RejectedSync       int
	RejectedHardSync   int
	LDPCAttempts       int
	LDPCFailures       int
	// UnpackFailures is retained for a common FT8/FT4 diagnostics adapter.
	// Verified unrenderable payloads carry ParseStatus instead.
	UnpackFailures    int
	DuplicateMessages int
	UniqueMessages    int
}

// DecodeMessagesWithReport decodes one FT4 slot and returns aggregate
// diagnostics for integration logging.
func DecodeMessagesWithReport(iwave []int16, options DecoderOptions) DecodeReport {
	start := time.Now()
	diagnostics := newDecodeDiagnostics(iwave)
	messages := decodeMessages(iwave, normalizeDecoderOptions(options), &diagnostics)
	diagnostics.Duration = time.Since(start)
	return DecodeReport{Messages: messages, Diagnostics: diagnostics}
}

func newDecodeDiagnostics(iwave []int16) DecodeDiagnostics {
	diagnostics := DecodeDiagnostics{
		InputSamples:    len(iwave),
		ExpectedSamples: SlotSamples,
	}
	if len(iwave) < SlotSamples {
		diagnostics.ShortInputSamples = SlotSamples - len(iwave)
	}
	if len(iwave) > SlotSamples {
		diagnostics.ExtraInputSamples = len(iwave) - SlotSamples
	}
	return diagnostics
}

// ErrInvalidDecodeInput identifies PCM input rejected by a checked API.
var ErrInvalidDecodeInput = errors.New("ft4: invalid decode input")

// ErrInvalidDecoderOptions identifies invalid checked decoder options.
var ErrInvalidDecoderOptions = errors.New("ft4: invalid decoder options")

// DecodeInputError describes a PCM slot rejected by DecodeMessagesChecked.
type DecodeInputError struct {
	GotSamples  int
	WantSamples int
}

func (e *DecodeInputError) Error() string {
	return fmt.Sprintf("%v: got %d samples, want %d samples for one 7.5-second slot",
		ErrInvalidDecodeInput, e.GotSamples, e.WantSamples)
}

// Unwrap returns ErrInvalidDecodeInput for errors.Is compatibility.
func (e *DecodeInputError) Unwrap() error { return ErrInvalidDecodeInput }

// DecoderOptionError describes one checked decoder option rejection.
type DecoderOptionError struct {
	Field  string
	Reason string
}

func (e *DecoderOptionError) Error() string {
	return fmt.Sprintf("%v: %s: %s", ErrInvalidDecoderOptions, e.Field, e.Reason)
}

// Unwrap returns ErrInvalidDecoderOptions for errors.Is compatibility.
func (e *DecoderOptionError) Unwrap() error { return ErrInvalidDecoderOptions }

// DecodeMessagesChecked requires exactly one full slot and validates options.
func DecodeMessagesChecked(iwave []int16, options DecoderOptions) (DecodeReport, error) {
	if len(iwave) != SlotSamples {
		return DecodeReport{Diagnostics: newDecodeDiagnostics(iwave)}, &DecodeInputError{
			GotSamples: len(iwave), WantSamples: SlotSamples,
		}
	}
	if err := validateDecoderOptions(options); err != nil {
		return DecodeReport{Diagnostics: newDecodeDiagnostics(iwave)}, err
	}
	return DecodeMessagesWithReport(iwave, options), nil
}

func validateDecoderOptions(options DecoderOptions) error {
	if options.SyncMin < 0 || math.IsNaN(options.SyncMin) || math.IsInf(options.SyncMin, 0) {
		return invalidDecoderOption("SyncMin", "must be finite and non-negative")
	}
	if options.MaxCandidates < 0 || options.MaxCandidates > defaultMaxCand {
		return invalidDecoderOption("MaxCandidates", "must be between 0 and %d", defaultMaxCand)
	}
	if options.MinFreqHz < 0 || options.MaxFreqHz < 0 {
		return invalidDecoderOption("MinFreqHz", "frequency bounds must be non-negative")
	}
	minFreq := defaultMinFreqHz
	maxFreq := defaultMaxFreqHz
	if options.MinFreqHz > 0 {
		minFreq = options.MinFreqHz
	}
	if options.MaxFreqHz > 0 {
		maxFreq = options.MaxFreqHz
	}
	if minFreq >= maxFreq || maxFreq > SampleRate/2 {
		return invalidDecoderOption("MinFreqHz", "frequency bounds must be ordered and at or below Nyquist")
	}
	if options.HardSyncMin < 0 || options.HardSyncMin > SyncSymbols {
		return invalidDecoderOption("HardSyncMin", "must be between 0 and %d", SyncSymbols)
	}
	return nil
}

func invalidDecoderOption(field, format string, args ...any) error {
	return &DecoderOptionError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// Decoder retains a stream-oriented API compatible with ft8.Decoder. The
// initial FT4 decoder is stateless; the instance boundary lets callers adopt
// future hash/AP state without another integration change.
type Decoder struct {
	options DecoderOptions
}

// NewDecoder returns a decoder using default FT4 options.
func NewDecoder() *Decoder { return &Decoder{} }

// NewDecoderWithOptions returns a decoder using the supplied options.
func NewDecoderWithOptions(options DecoderOptions) *Decoder {
	return &Decoder{options: options}
}

// DecodeMessages decodes one slot with the decoder's configured options.
func (d *Decoder) DecodeMessages(iwave []int16) []DecodedMessage {
	return DecodeMessagesWithOptions(iwave, d.options)
}

// DecodeMessagesWithReport decodes one slot and returns diagnostics.
func (d *Decoder) DecodeMessagesWithReport(iwave []int16) DecodeReport {
	return DecodeMessagesWithReport(iwave, d.options)
}

// DecodeMessagesChecked validates and decodes one slot.
func (d *Decoder) DecodeMessagesChecked(iwave []int16) (DecodeReport, error) {
	return DecodeMessagesChecked(iwave, d.options)
}
