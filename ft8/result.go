// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

// DecodeMode identifies which decoder search mode produced a message.
type DecodeMode string

const (
	// DecodeModeStrict is the parity-safe default decode mode.
	DecodeModeStrict DecodeMode = "strict"
	// DecodeModeDeep is the experimental maximum-recall decode mode.
	DecodeModeDeep DecodeMode = "deep"
)

// DecodeResult groups strict and optional deep decode output for one slot.
type DecodeResult struct {
	// Messages contains one merged row per unique decoded message text.
	Messages []StructuredMessage
	// Strict contains the strict-mode flat decode list.
	Strict []DecodedMessage
	// Deep contains the deep-mode flat decode list when requested.
	Deep []DecodedMessage
}

// StructuredDecodeReport contains structured decode output plus per-pass
// diagnostics for the strict and optional deep decode passes.
type StructuredDecodeReport struct {
	// Result contains the merged strict/deep structured output.
	Result DecodeResult
	// StrictReport contains the strict decode messages and diagnostics.
	StrictReport DecodeReport
	// DeepReport contains deep decode messages and diagnostics when IncludeDeep
	// was set; otherwise it is the zero value.
	DeepReport DecodeReport
}

// DeepOnly returns messages found by deep mode but not strict mode.
func (r DecodeResult) DeepOnly() []StructuredMessage {
	out := make([]StructuredMessage, 0)
	for _, msg := range r.Messages {
		if msg.Deep && !msg.Strict {
			out = append(out, msg)
		}
	}
	return out
}

// StrictMessages returns merged messages found by strict mode.
func (r DecodeResult) StrictMessages() []StructuredMessage {
	out := make([]StructuredMessage, 0, len(r.Strict))
	for _, msg := range r.Messages {
		if msg.Strict {
			out = append(out, msg)
		}
	}
	return out
}

// StructuredMessage is a decoded message with mode/source labeling.
type StructuredMessage struct {
	DecodedMessage
	Mode       DecodeMode
	Strict     bool
	Deep       bool
	Duplicate  bool
	StrictCopy *DecodedMessage
	DeepCopy   *DecodedMessage
}

// StructuredDecodeOptions controls DecodeStructured behavior.
type StructuredDecodeOptions struct {
	// IncludeDeep runs DeepDecoderOptions and labels messages found only there.
	IncludeDeep bool
	// DeepOptions overrides DeepDecoderOptions when IncludeDeep is true.
	DeepOptions DecoderOptions
}

// DecodeStructured decodes one slot and returns strict/deep-labeled results.
//
// This is the permissive structured API: short input is zero-padded, excess
// input beyond the decoder buffer is ignored, and invalid deep options are
// normalized where possible. Use DecodeStructuredWithReport for diagnostics or
// DecodeStructuredChecked when caller input and deep options should be
// validated before decode work starts.
func DecodeStructured(iwave []int16, options StructuredDecodeOptions) DecodeResult {
	strict := DecodeMessages(iwave)
	var deep []DecodedMessage
	includeDeep := options.IncludeDeep
	if includeDeep {
		deep = DecodeMessagesWithOptions(iwave, structuredDeepOptions(options))
	}
	return mergeStructuredDecode(strict, deep, includeDeep)
}

// DecodeStructuredWithReport decodes one slot and returns strict/deep-labeled
// results plus aggregate diagnostics for each decode pass that was run.
//
// This is the permissive structured report API: short input is zero-padded,
// excess input beyond the decoder buffer is ignored, and invalid deep options
// are normalized where possible. Use DecodeStructuredChecked when caller input
// and deep options should be validated before decode work starts.
func DecodeStructuredWithReport(iwave []int16, options StructuredDecodeOptions) StructuredDecodeReport {
	strictReport := DecodeMessagesWithReport(iwave, DecoderOptions{})
	var deepReport DecodeReport
	includeDeep := options.IncludeDeep
	if includeDeep {
		deepReport = DecodeMessagesWithReport(iwave, structuredDeepOptions(options))
	}
	return StructuredDecodeReport{
		Result:       mergeStructuredDecode(strictReport.Messages, deepReport.Messages, includeDeep),
		StrictReport: strictReport,
		DeepReport:   deepReport,
	}
}

// DecodeStructuredChecked validates input and optional deep options, then
// decodes one FT8 slot with structured output and aggregate diagnostics.
//
// It returns ErrInvalidDecodeInput for malformed slot input and
// ErrInvalidDecoderOptions for invalid deep options. Empty decoded output is not
// an error when the input and options are valid.
func DecodeStructuredChecked(iwave []int16, options StructuredDecodeOptions) (StructuredDecodeReport, error) {
	if err := validateDecodeRequest(iwave, DecoderOptions{}); err != nil {
		return StructuredDecodeReport{
			StrictReport: DecodeReport{Diagnostics: newDecodeDiagnostics(iwave)},
		}, err
	}
	if options.IncludeDeep {
		if err := validateDecoderOptions(structuredDeepOptions(options)); err != nil {
			diagnostics := newDecodeDiagnostics(iwave)
			return StructuredDecodeReport{
				StrictReport: DecodeReport{Diagnostics: diagnostics},
				DeepReport:   DecodeReport{Diagnostics: diagnostics},
			}, err
		}
	}
	return DecodeStructuredWithReport(iwave, options), nil
}

func structuredDeepOptions(options StructuredDecodeOptions) DecoderOptions {
	deepOptions := options.DeepOptions
	if decoderOptionsEmpty(deepOptions) {
		deepOptions = DeepDecoderOptions()
	}
	return deepOptions
}

func mergeStructuredDecode(strict, deep []DecodedMessage, includeDeep bool) DecodeResult {
	result := DecodeResult{
		Strict: strict,
	}
	merged := make([]StructuredMessage, 0, len(strict))
	byText := make(map[string]int, len(strict))
	for _, msg := range strict {
		copyMsg := msg
		byText[msg.Text] = len(merged)
		merged = append(merged, StructuredMessage{
			DecodedMessage: msg,
			Mode:           DecodeModeStrict,
			Strict:         true,
			StrictCopy:     &copyMsg,
		})
	}

	if includeDeep {
		result.Deep = deep
		for _, msg := range deep {
			if idx, ok := byText[msg.Text]; ok {
				copyMsg := msg
				merged[idx].Deep = true
				merged[idx].Duplicate = true
				merged[idx].DeepCopy = &copyMsg
				continue
			}
			copyMsg := msg
			byText[msg.Text] = len(merged)
			merged = append(merged, StructuredMessage{
				DecodedMessage: msg,
				Mode:           DecodeModeDeep,
				Deep:           true,
				DeepCopy:       &copyMsg,
			})
		}
	}

	result.Messages = merged
	return result
}
