// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

const (
	// SampleRate is the FT4 audio sample rate used by WSJT-X.
	SampleRate = 12000
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
