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
// Existing DecodeMessages APIs remain strict-only.
func DecodeStructured(iwave []int16, options StructuredDecodeOptions) DecodeResult {
	strict := DecodeMessages(iwave)
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

	if options.IncludeDeep {
		deepOptions := options.DeepOptions
		if decoderOptionsEmpty(deepOptions) {
			deepOptions = DeepDecoderOptions()
		}
		deep := DecodeMessagesWithOptions(iwave, deepOptions)
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
