// Package ft8x implements the FT8 digital radio protocol decoder.
// Ported from the WSJT-X Fortran source code.
package ft8x

const (
	// KK is the number of information bits (77 message + 14 CRC).
	KK = 91
	// ND is the number of data symbols.
	ND = 58
	// NS is the number of sync symbols (3 Costas 7×7 arrays).
	NS = 21
	// NN is the total number of channel symbols.
	NN = NS + ND // 79
	// NSPS is the number of audio samples per symbol at 12000 S/s.
	NSPS = 1920
	// NZ is the number of audio samples in the full 15 s waveform.
	NZ = NSPS * NN // 151680
	// NMAX is the number of audio samples in the full input buffer.
	NMAX = 15 * 12000 // 180000
	// NDOWN is the downsample factor (12000 / 200).
	NDOWN = 60
	// NP2 is the length of the downsampled complex buffer.
	NP2 = 2812
	// NFFT2 is the FFT size for the downsampled signal.
	NFFT2 = 3200
	// NFFT1DS is the FFT size used during downsampling.
	NFFT1DS = 192000
	// ScaleFac is the LLR scaling factor applied before LDPC.
	ScaleFac = 2.83
	// MaxIterations is the maximum number of BP decoder iterations.
	MaxIterations = 30

	// Fs is the audio sample rate.
	Fs = 12000.0
	// Fs2 is the downsampled sample rate.
	Fs2 = Fs / NDOWN // 200 Hz
	// Dt2 is the sample period at the downsampled rate.
	Dt2 = 1.0 / Fs2
	// Baud is the symbol rate.
	Baud = Fs / NSPS // 6.25 Hz
)

// Icos7 is the Costas 7×7 sync array (flipped w.r.t. original FT8).
var Icos7 = [7]int{3, 1, 4, 0, 6, 5, 2}

// GrayMap maps 3-bit binary index to 8-FSK tone number.
var GrayMap = [8]int{0, 1, 3, 2, 5, 6, 4, 7}

// GrayUnmap is the inverse of GrayMap: tone number → 3-bit binary value.
var GrayUnmap = [8]int{0, 1, 3, 2, 6, 7, 5, 4}
