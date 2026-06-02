// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func FuzzDecodePublicAPIsNoPanic(f *testing.F) {
	for api := uint8(0); api < 13; api++ {
		f.Add([]byte{api, api + 1, api + 2}, api, api%10)
	}

	f.Fuzz(func(t *testing.T, raw []byte, api uint8, optionSeed uint8) {
		samples := fuzzInt16Samples(raw)
		options := fuzzDecoderOptions(optionSeed)

		switch api % 13 {
		case 0:
			_ = DecodeMessages(samples)
		case 1:
			_ = DecodeMessagesWithOptions(samples, options)
		case 2:
			_ = DecodeMessagesWithReport(samples, options)
		case 3:
			_, _ = DecodeMessagesChecked(samples, options)
		case 4:
			decoder := NewDecoderWithOptions(options)
			_ = decoder.DecodeMessages(samples)
		case 5:
			decoder := NewDecoderWithOptions(options)
			_ = decoder.DecodeMessagesWithReport(samples)
		case 6:
			decoder := NewDecoderWithOptions(options)
			_, _ = decoder.DecodeMessagesChecked(samples)
		case 7:
			_ = DecodeStructured(samples, StructuredDecodeOptions{})
		case 8:
			_ = DecodeStructuredWithReport(samples, StructuredDecodeOptions{})
		case 9:
			_, _ = DecodeStructuredChecked(samples, StructuredDecodeOptions{})
		case 10:
			decoder := NewDecoderWithOptions(options)
			_ = decoder.DecodeStructured(samples)
		case 11:
			decoder := NewDecoderWithOptions(options)
			_ = decoder.DecodeStructuredWithReport(samples)
		default:
			decoder := NewDecoderWithOptions(options)
			_, _ = decoder.DecodeStructuredChecked(samples)
		}
	})
}

func fuzzInt16Samples(raw []byte) []int16 {
	if len(raw) > 128 {
		raw = raw[:128]
	}
	samples := make([]int16, (len(raw)+1)/2)
	for i := range samples {
		lo := uint16(raw[i*2])
		var hi uint16
		if j := i*2 + 1; j < len(raw) {
			hi = uint16(raw[j])
		}
		samples[i] = int16(lo | hi<<8)
	}
	return samples
}

func fuzzDecoderOptions(seed uint8) DecoderOptions {
	switch seed % 10 {
	case 0:
		return DecoderOptions{}
	case 1:
		return DecoderOptions{MaxCandidates: 1}
	case 2:
		return DecoderOptions{SyncMin: 3.0, MaxCandidates: 2}
	case 3:
		return DecoderOptions{Blocks: []int{50}}
	case 4:
		return DecoderOptions{MaxCandidates: -1}
	case 5:
		return DecoderOptions{Blocks: []int{50, 50}}
	case 6:
		return DecoderOptions{MinFreqHz: 3200, MaxFreqHz: 200}
	case 7:
		return DecoderOptions{CostasMinGeo: 1.1, CostasMinBlock: 0.8}
	case 8:
		return DecoderOptions{EnableBroadAP: true}
	case 9:
		return DecoderOptions{APCallHints: []APCallHint{{Call: "K1ABC", Source: "fuzz"}}, MaxAPCallHypotheses: 1}
	default:
		return DecoderOptions{}
	}
}
