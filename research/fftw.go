// fftw.go — CGO bridge to single-precision FFTW for the spectrogram and
// downsampler FFTs.
//
// The Fortran pipeline uses sfftw (float32 FFTW) throughout. This bridge
// provides float32 FFT functions matching the exact Fortran calls:
//
//   - 3840-point r2c: spectrogram (sync8.f90)
//   - 192000-point r2c: downsampler forward FFT (ft8_downsample.f90)
//   - 3200-point c2c backward: downsampler inverse FFT (ft8_downsample.f90)

package research

/*
#cgo LDFLAGS: -lfftw3f
#include <fftw3.h>

extern void fftw_r2c_3840(const float *in, float *out);
extern void fftw_r2c_192k(const float *in, float *out);
extern void fftw_c2c_32_forward(float *inout);
extern void fftw_c2c_3200_backward(float *inout);
*/
import "C"
import (
	"math"
	"unsafe"
)

// SpectrogramFFT3840 computes a 3840-point r2c FFT in single precision and
// returns the power spectrum for bins 1..NH1, computed in float32.
func SpectrogramFFT3840(x []float32) [NH1]float64 {
	const (
		n    = NFFT1   // 3840
		nout = n/2 + 1 // 1921
		cBuf = nout * 2
	)

	var buf [cBuf]float32
	copy(buf[:], x)

	C.fftw_r2c_3840(
		(*C.float)(unsafe.Pointer(&buf[0])),
		(*C.float)(unsafe.Pointer(&buf[0])),
	)

	var pow [NH1]float64
	for i := 1; i <= NH1 && i < nout; i++ {
		re := buf[2*i]
		im := buf[2*i+1]
		pow[i-1] = float64(re*re + im*im)
	}

	return pow
}

// ── Downsampler FFTW functions ──────────────────────────────────────────

const (
	nfft1DS = NFFT1DS       // 192000
	nfft2DS = NFFT2         // 3200
	nout192 = nfft1DS/2 + 1 // 96001
)

// DownsamplerF32 is a float32 FFTW-based downsampler matching the Fortran
// ft8_downsample.f90 exactly — float32 throughout, using the same FFTW library.
type DownsamplerF32 struct {
	cx    []complex64 // Cached 192k spectrum (float32 complex)
	taper [101]float64
	ready bool
}

// NewDownsamplerF32 creates a float32 FFTW-based downsampler.
func NewDownsamplerF32() *DownsamplerF32 {
	d := &DownsamplerF32{}
	for i := 0; i <= 100; i++ {
		d.taper[i] = 0.5 * (1.0 + math.Cos(float64(i)*math.Pi/100.0))
	}
	return d
}

// Downsample mixes audio to baseband at f0 Hz using float32 FFTW,
// matching Fortran ft8_downsample.f90 exactly.
func (d *DownsamplerF32) Downsample(dd []float32, newdat *bool, f0 float64) []complex128 {
	if *newdat || d.cx == nil {
		// Compute 192k r2c FFT in float32 via FFTW
		var buf [nfft1DS + 2]float32 // room for in-place r2c
		lx := len(dd)
		if lx > NMAX {
			lx = NMAX
		}
		copy(buf[:lx], dd[:lx])
		// Zero-pad the rest (buf is already zero-initialized)

		C.fftw_r2c_192k(
			(*C.float)(unsafe.Pointer(&buf[0])),
			(*C.float)(unsafe.Pointer(&buf[0])),
		)

		// Store as complex64 (matching Fortran's complex = 2×float32)
		d.cx = make([]complex64, nout192)
		for i := 0; i < nout192; i++ {
			d.cx[i] = complex64(complex(float32(buf[2*i]), float32(buf[2*i+1])))
		}
		*newdat = false
	}

	// Frequency band extraction (matching Fortran exactly)
	df := float32(Fs) / float32(nfft1DS)
	baud := float32(Fs) / float32(NSPS)
	i0 := int(math.Round(float64(float32(f0) / df)))

	ft := float32(f0) + 8.5*baud
	fb := float32(f0) - 1.5*baud

	it := int(math.Round(float64(ft / df)))
	if it > nfft1DS/2 {
		it = nfft1DS / 2
	}
	ib := int(math.Round(float64(fb / df)))
	if ib < 1 {
		ib = 1
	}

	// Extract and zero-pad into c1 (float32 complex, matching Fortran)
	var c1 [nfft2DS]complex64
	k := 0
	for i := ib; i <= it && k < nfft2DS; i++ {
		if i < len(d.cx) {
			c1[k] = d.cx[i]
		}
		k++
	}

	// Apply taper (float32 arithmetic)
	for i := 0; i <= 100 && i < k; i++ {
		c1[i] = complex64(complex(
			float32(real(c1[i]))*float32(d.taper[100-i]),
			float32(imag(c1[i]))*float32(d.taper[100-i]),
		))
	}
	for i := 0; i <= 100; i++ {
		idx := k - 1 - 100 + i
		if idx >= 0 && idx < nfft2DS {
			c1[idx] = complex64(complex(
				float32(real(c1[idx]))*float32(d.taper[i]),
				float32(imag(c1[idx]))*float32(d.taper[i]),
			))
		}
	}

	// Circular shift: cshift(c1, i0-ib)
	shift := i0 - ib
	shift = ((shift % nfft2DS) + nfft2DS) % nfft2DS
	if shift > 0 {
		var tmp [nfft2DS]complex64
		copy(tmp[:], c1[shift:])
		copy(tmp[nfft2DS-shift:], c1[:shift])
		c1 = tmp
	}

	// 3200-point c2c backward FFT via FFTW (unnormalized, matching Fortran)
	C.fftw_c2c_3200_backward((*C.float)(unsafe.Pointer(&c1[0])))

	// Scale: fac = 1/sqrt(NFFT1 * NFFT2) (Fortran convention)
	fac := float32(1.0 / math.Sqrt(float64(nfft1DS)*float64(nfft2DS)))

	// Convert to complex128 for downstream compatibility
	result := make([]complex128, nfft2DS)
	for i := 0; i < nfft2DS; i++ {
		re := float64(float32(real(c1[i])) * fac)
		im := float64(float32(imag(c1[i])) * fac)
		result[i] = complex(re, im)
	}

	return result
}

// FFT32Forward computes a 32-point c2c forward FFT using FFTW float32,
// matching Fortran four2a(csymb,32,1,-1,1) exactly.
func FFT32Forward(x []complex64) {
	C.fftw_c2c_32_forward((*C.float)(unsafe.Pointer(&x[0])))
}
