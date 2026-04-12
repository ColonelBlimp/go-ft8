// fftw.go — CGO bridge to single-precision FFTW for the spectrogram FFT.
//
// The Fortran sync8 pipeline uses sfftw_plan_dft_r2c_1d (float32 FFTW) and
// stores the power spectrum as float32.  Our pure-Go FFT computes in float64,
// causing ~5% divergence in spectrogram bin magnitudes that pushes marginal
// signals below the syncmin threshold.
//
// This bridge calls libfftw3f for the 3840-point r2c FFT used in
// computeSpectrogram(), and computes the power spectrum in float32 to match
// the Fortran: s(i,j) = real(cx(i))**2 + aimag(cx(i))**2  (all float32).
//
// All other FFTs (192k downsample, 3200 IFFT, etc.) remain pure Go float64.

package research

/*
#cgo LDFLAGS: -lfftw3f
#include <fftw3.h>

extern void fftw_r2c_3840(const float *in, float *out);
*/
import "C"
import "unsafe"

// SpectrogramFFT3840 computes a 3840-point r2c FFT in single precision and
// returns the power spectrum (re²+im²) for bins 1..NH1, computed entirely
// in float32 to match the Fortran spectrogram path.
//
// This matches the Fortran:
//
//	call four2a(x, NFFT1, 1, -1, 0)          ! sfftw r2c, float32
//	s(i,j) = real(cx(i))**2 + aimag(cx(i))**2 ! power in float32
//
// Input: up to NSPS=1920 float32 samples (zero-padded to 3840 internally).
// Output: NH1=1920 float64 power values for bins 1..NH1.
func SpectrogramFFT3840(x []float32) [NH1]float64 {
	const (
		n    = NFFT1    // 3840
		nout = n/2 + 1  // 1921
		cBuf = nout * 2 // 3842 floats for in-place r2c
	)

	// Prepare input buffer (zero-padded to 3842 floats for in-place r2c)
	var buf [cBuf]float32
	copy(buf[:], x)

	// Call FFTW via C wrapper
	C.fftw_r2c_3840(
		(*C.float)(unsafe.Pointer(&buf[0])),
		(*C.float)(unsafe.Pointer(&buf[0])),
	)

	// Compute power spectrum in float32, matching Fortran:
	//   s(i,j) = real(cx(i))**2 + aimag(cx(i))**2
	// Then widen to float64 for storage in the Go spectrogram.
	var pow [NH1]float64
	for i := 1; i <= NH1 && i < nout; i++ {
		re := buf[2*i]
		im := buf[2*i+1]
		pow[i-1] = float64(re*re + im*im) // float32 multiply, then widen
	}

	return pow
}
