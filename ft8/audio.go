// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

const (
	// SampleRate is the required decoder input rate in Hz.
	SampleRate = 12000
	// Channels is the required decoder input channel count.
	Channels = 1
	// BitsPerSample is the required PCM depth for DecodeMessages input.
	BitsPerSample = 16

	wantSampleRate = SampleRate
)

// decodeBlocks builds the zero-padded FT8 float32 working array from raw int16
// PCM samples.
func decodeBlocks(iwave []int16, blocks int) []float32 {
	const nBuf = 180000
	nCopy := blocks * 3456
	if nCopy > nBuf {
		nCopy = nBuf
	}
	if nCopy > len(iwave) {
		nCopy = len(iwave)
	}
	out := make([]float32, nBuf)
	for i, s := range iwave[:nCopy] {
		out[i] = float32(s)
	}
	return out
}
