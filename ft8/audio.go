// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "sync"

const (
	// SampleRate is the required decoder input rate in Hz.
	SampleRate = 12000
	// Channels is the required decoder input channel count.
	Channels = 1
	// BitsPerSample is the required PCM depth for DecodeMessages input.
	BitsPerSample = 16

	wantSampleRate = SampleRate

	ft8DecodeBufferSamples = 180000
)

var decodeBlockPool = sync.Pool{
	New: func() any {
		return make([]float32, ft8DecodeBufferSamples)
	},
}

// decodeBlocks builds the zero-padded FT8 float32 working array from raw int16
// PCM samples.
func decodeBlocks(iwave []int16, blocks int) []float32 {
	nCopy := blocks * 3456
	if nCopy > ft8DecodeBufferSamples {
		nCopy = ft8DecodeBufferSamples
	}
	if nCopy > len(iwave) {
		nCopy = len(iwave)
	}
	out := decodeBlockPool.Get().([]float32)
	clear(out)
	for i, s := range iwave[:nCopy] {
		out[i] = float32(s)
	}
	return out
}

func putDecodeBlocks(dd []float32) {
	if cap(dd) < ft8DecodeBufferSamples {
		return
	}
	decodeBlockPool.Put(dd[:ft8DecodeBufferSamples])
}
