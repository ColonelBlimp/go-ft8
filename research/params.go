// params.go — FT8 constants for the research package.
//
// Duplicated from the root ft8x package to keep research self-contained.
// These match ft8_params.f90 exactly.

package research

const (
	// Fs is the audio sample rate (Hz).
	Fs = 12000.0

	// NSPS is samples per symbol at 12000 S/s.
	NSPS = 1920

	// NN is the total number of channel symbols (21 sync + 58 data).
	NN = 79

	// NMAX is the number of audio samples in the input buffer (15 s × 12000 S/s).
	NMAX = 15 * 12000 // 180000

	// NFFT1 is the FFT length for symbol spectra (2 × NSPS).
	NFFT1 = 2 * NSPS // 3840

	// NH1 is the number of positive-frequency bins in the symbol FFT.
	NH1 = NFFT1 / 2 // 1920

	// NSTEP is the spectrogram time-step size in samples (NSPS/4).
	NSTEP = NSPS / 4 // 480

	// NHSYM is the number of spectrogram time columns.
	NHSYM = NMAX/NSTEP - 3 // 372

	// NDOWN is the downsample factor (12000 / 200).
	NDOWN = 60
)

// Icos7 is the Costas 7×7 sync array.
var Icos7 = [7]int{3, 1, 4, 0, 6, 5, 2}
