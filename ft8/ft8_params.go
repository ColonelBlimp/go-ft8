// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

const (
	ft8KInfoBits = 91
	ft8DataSyms  = 58
	ft8SyncSyms  = 21
	ft8Symbols   = ft8DataSyms + ft8SyncSyms

	ft8SamplesPerSymbol = 1920
	ft8FrameSamples     = wantSampleRate * 15
	ft8SignalSamples    = ft8SamplesPerSymbol * ft8Symbols
	ft8NFFT1            = 2 * ft8SamplesPerSymbol
	ft8NBins            = ft8NFFT1 / 2
	ft8Step             = ft8SamplesPerSymbol / 4
	ft8NSymbolSpectra   = ft8FrameSamples/ft8Step - 3
	ft8Downsample       = 60
	ft8DownsampleFFT1   = 192000
	ft8DownsampleFFT2   = ft8DownsampleFFT1 / ft8Downsample
	ft8DownsampleRate   = wantSampleRate / ft8Downsample
	ft8RefineSamples    = 2812

	ft8DefaultMinFreq = 200
	ft8DefaultMaxFreq = 3200
	ft8DefaultSyncMin = 1.8
	ft8DefaultMaxCand = 1000
)

var ft8Costas = [7]int{3, 1, 4, 0, 6, 5, 2}

const (
	sgNFFT  = ft8NFFT1
	sgStep  = ft8Step
	sgNBins = ft8NBins
)
