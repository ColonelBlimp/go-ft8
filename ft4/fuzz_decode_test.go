// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

import "testing"

func FuzzDecodePublicAPIsNoPanic(f *testing.F) {
	for api := uint8(0); api < 7; api++ {
		f.Add([]byte{api, api + 1, api + 2}, api)
	}
	f.Fuzz(func(t *testing.T, raw []byte, api uint8) {
		if len(raw) > 128 {
			raw = raw[:128]
		}
		samples := make([]int16, (len(raw)+1)/2)
		for i := range samples {
			lo := uint16(raw[2*i])
			var hi uint16
			if 2*i+1 < len(raw) {
				hi = uint16(raw[2*i+1])
			}
			samples[i] = int16(lo | hi<<8)
		}
		options := DecoderOptions{MaxCandidates: int(api % 3)}
		switch api % 7 {
		case 0:
			_ = DecodeMessages(samples)
		case 1:
			_ = DecodeMessagesWithOptions(samples, options)
		case 2:
			_ = DecodeMessagesWithReport(samples, options)
		case 3:
			_, _ = DecodeMessagesChecked(samples, options)
		case 4:
			_ = NewDecoderWithOptions(options).DecodeMessages(samples)
		case 5:
			_ = NewDecoderWithOptions(options).DecodeMessagesWithReport(samples)
		default:
			_, _ = NewDecoderWithOptions(options).DecodeMessagesChecked(samples)
		}
	})
}
