// fftw_wrapper.c — Thin C wrapper around single-precision FFTW (fftwf).
//
// Provides cached r2c plans matching Fortran four2a.f90 (sfftw_plan_dft_r2c_1d):
//   - 3840-point for the spectrogram FFT
//   - 192000-point for the downsampler forward FFT
//
// Also provides a cached c2c backward plan:
//   - 3200-point for the downsampler inverse FFT

#include <fftw3.h>
#include <string.h>

// ── 3840-point r2c (spectrogram) ─────────────────────────────────────────

static fftwf_plan plan3840 = NULL;

void fftw_r2c_3840(const float *in, float *out) {
    if (plan3840 == NULL) {
        plan3840 = fftwf_plan_dft_r2c_1d(3840, (float *)out, (fftwf_complex *)out,
                                          FFTW_ESTIMATE);
    }
    memcpy(out, in, 3840 * sizeof(float));
    fftwf_execute_dft_r2c(plan3840, (float *)out, (fftwf_complex *)out);
}

// ── 192000-point r2c (downsampler forward) ───────────────────────────────

static fftwf_plan plan192k = NULL;

// fftw_r2c_192k computes a 192000-point real-to-complex FFT in single precision.
//
// in:  up to 180000 float32 values (zero-padded to 192000 by caller)
// out: 96001 complex float32 values (N/2+1 bins) = 192002 floats
//
// Matches Fortran ft8_downsample.f90:
//   call four2a(cx, NFFT1, 1, -1, 0)   ! sfftw r2c, NFFT1=192000
void fftw_r2c_192k(const float *in, float *out) {
    if (plan192k == NULL) {
        plan192k = fftwf_plan_dft_r2c_1d(192000, (float *)out, (fftwf_complex *)out,
                                           FFTW_ESTIMATE);
    }
    memcpy(out, in, 192000 * sizeof(float));
    fftwf_execute_dft_r2c(plan192k, (float *)out, (fftwf_complex *)out);
}

// ── 32-point c2c forward (symbol spectra) ────────────────────────────────

static fftwf_plan plan32_fw = NULL;

// fftw_c2c_32_forward computes a 32-point c2c forward FFT in single precision,
// UNNORMALIZED (matching Fortran four2a isign=-1 for symbol spectra).
//
// inout: 32 complex float32 values (64 floats), modified in-place.
//
// Matches Fortran ft8b.f90:
//   call four2a(csymb,32,1,-1,1)   ! sfftw c2c forward
void fftw_c2c_32_forward(float *inout) {
    if (plan32_fw == NULL) {
        plan32_fw = fftwf_plan_dft_1d(32, (fftwf_complex *)inout,
                                       (fftwf_complex *)inout,
                                       FFTW_FORWARD, FFTW_ESTIMATE);
    }
    fftwf_execute_dft(plan32_fw, (fftwf_complex *)inout, (fftwf_complex *)inout);
}

// ── 3200-point c2c backward (downsampler inverse) ────────────────────────

static fftwf_plan plan3200_bw = NULL;

// fftw_c2c_3200_backward computes a 3200-point c2c backward (inverse) FFT
// in single precision, UNNORMALIZED (matching Fortran four2a isign=+1).
//
// inout: 3200 complex float32 values (6400 floats), modified in-place.
//
// Matches Fortran ft8_downsample.f90:
//   call four2a(c1, NFFT2, 1, 1, 1)   ! sfftw c2c backward, NFFT2=3200
void fftw_c2c_3200_backward(float *inout) {
    if (plan3200_bw == NULL) {
        plan3200_bw = fftwf_plan_dft_1d(3200, (fftwf_complex *)inout,
                                         (fftwf_complex *)inout,
                                         FFTW_BACKWARD, FFTW_ESTIMATE);
    }
    fftwf_execute_dft(plan3200_bw, (fftwf_complex *)inout, (fftwf_complex *)inout);
}
