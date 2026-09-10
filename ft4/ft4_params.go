// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

const (
	// SampleRate is the FT4 audio sample rate used by WSJT-X.
	SampleRate = 12000
	// Channels is the required decoder input channel count.
	Channels = 1
	// BitsPerSample is the required PCM depth for DecodeMessages input.
	BitsPerSample = 16
	// SlotSeconds is the duration of one FT4 transmit/receive period.
	SlotSeconds = 7.5
	// SlotSamples is one complete FT4 period at SampleRate.
	SlotSamples = 90000
	// SamplesPerSymbol is the number of 12 kHz samples in one FT4 symbol.
	SamplesPerSymbol = 576
	// DataSymbols is the number of two-bit channel symbols carrying the codeword.
	DataSymbols = 87
	// SyncSymbols is the total number of symbols in the four Costas arrays.
	SyncSymbols = 16
	// ToneSymbols is the number of sync and data tone symbols. The two waveform
	// ramp intervals are implicit and are not included in an encoded tone array.
	ToneSymbols = DataSymbols + SyncSymbols
	// GFSKBT is the Gaussian pulse bandwidth-time product used by FT4.
	GFSKBT = 1.0
	// SymbolSeconds is one FT4 symbol duration.
	SymbolSeconds = float64(SamplesPerSymbol) / SampleRate
	// SyncStartSeconds is the first Costas-symbol boundary that produces a
	// decoder DT of zero.
	SyncStartSeconds = 0.5
	// NominalStartSeconds is where the complete GFSK waveform, including its
	// leading ramp symbol, starts for a decoder DT of zero.
	NominalStartSeconds = SyncStartSeconds - SymbolSeconds
	// WaveformSamples includes the leading and trailing GFSK ramp symbols.
	WaveformSamples = (ToneSymbols + 2) * SamplesPerSymbol
	// WaveformSeconds is the duration of one complete FT4 waveform.
	WaveformSeconds = float64(WaveformSamples) / SampleRate
	// ToneSpacingHz is the separation between adjacent FT4 tones.
	ToneSpacingHz = 1 / SymbolSeconds
	// SignalWidthHz is the nominal four-tone span used by passband and
	// occupancy consumers.
	SignalWidthHz = 4 * ToneSpacingHz

	decodeSamples = 21 * 3456
	downsample    = 18
	downRate      = SampleRate / downsample
	downNSPS      = SamplesPerSymbol / downsample
	downFFT       = decodeSamples / downsample
	coarseFFT     = 4 * SamplesPerSymbol
	coarseBins    = coarseFFT / 2
	coarseSteps   = (decodeSamples - coarseFFT) / SamplesPerSymbol
)

var costasArrays = [4][4]uint8{
	{0, 1, 3, 2},
	{1, 0, 2, 3},
	{2, 3, 1, 0},
	{3, 2, 0, 1},
}

var grayMap = [4]uint8{0, 1, 3, 2}

// FT4 XORs this vector with the source-encoded payload before calculating the
// CRC and LDPC parity. This is rvec from WSJT-X 3.0.2 lib/ft4/genft4.f90.
var scramblingVector = [77]int8{
	0, 1, 0, 0, 1, 0, 1, 0, 0, 1, 0, 1, 1, 1, 1, 0, 1, 0, 0, 0,
	1, 0, 0, 1, 1, 0, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 0, 0, 0,
	1, 0, 0, 0, 1, 0, 1, 0, 0, 1, 1, 1, 1, 0, 0, 1, 0, 1, 0, 1,
	0, 1, 0, 1, 1, 0, 1, 1, 1, 1, 1, 0, 0, 0, 1, 0, 1,
}
