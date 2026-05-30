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
	wantChannels   = Channels
	wantBitsPerSam = BitsPerSample
)

// decodeBlocks builds the FT8 float32 working array from raw int16 PCM samples.
// The decoder currently uses the full 50-block, 180000-sample path.
func decodeBlocks(iwave []int16, blocks int) []float32 {
	const nBuf = 180000
	nCopy := blocks * 3456
	if nCopy > nBuf {
		nCopy = nBuf
	}
	if nCopy > len(iwave) {
		nCopy = len(iwave)
	}
	itmp := make([]int16, nBuf)
	copy(itmp[:nCopy], iwave[:nCopy])
	return toFloat32(itmp)
}

// toFloat32 converts raw signed-16-bit PCM samples to float32 element-wise
// without normalization. int16 values are exactly representable as float32.
func toFloat32(in []int16) []float32 {
	out := make([]float32, len(in))
	for i, s := range in {
		out[i] = float32(s)
	}
	return out
}
