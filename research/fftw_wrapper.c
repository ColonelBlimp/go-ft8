// fftw_wrapper.c — Thin C wrapper around single-precision FFTW (fftwf).
//
// Provides a cached r2c plan for the 3840-point spectrogram FFT,
// matching the Fortran four2a.f90 call: sfftw_plan_dft_r2c_1d(plan, 3840, ...).
//
// Only the spectrogram path uses this; all other FFTs remain pure Go.

#include <fftw3.h>
#include <string.h>

// Cached plan for NFFT1=3840 r2c transform.
static fftwf_plan plan3840 = NULL;

// fftw_r2c_3840 computes a 3840-point real-to-complex FFT in single precision.
//
// in:  3840 float32 values (real input, zero-padded by caller)
// out: 1921 complex float32 values (N/2+1 unique bins)
//
// The plan is created on first call with FFTW_ESTIMATE (matching Fortran default)
// and reused on subsequent calls.
//
// FFTW's r2c overwrites the input array, so we work on a copy.
void fftw_r2c_3840(const float *in, float *out) {
    // Lazy plan creation (not thread-safe, but sync8 is single-threaded)
    if (plan3840 == NULL) {
        // FFTW_ESTIMATE does not modify the input/output arrays during planning
        plan3840 = fftwf_plan_dft_r2c_1d(3840, (float *)out, (fftwf_complex *)out,
                                          FFTW_ESTIMATE);
    }

    // Copy input into output buffer (FFTW r2c can work in-place when
    // the output array has room for N/2+1 complex values = N+2 floats).
    // out must be at least 3842 floats (1921 complex values).
    memcpy(out, in, 3840 * sizeof(float));

    // Execute the cached plan on the output buffer (in-place)
    fftwf_execute_dft_r2c(plan3840, (float *)out, (fftwf_complex *)out);
}
