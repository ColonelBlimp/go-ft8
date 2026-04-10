package ft8x

import (
	"math"
	"math/cmplx"
)

// fftRadix2 performs an in-place Cooley-Tukey radix-2 DIT FFT.
// len(x) must be a power of 2.
func fftRadix2(x []complex128, inverse bool) {
	n := len(x)
	if n <= 1 {
		return
	}

	// Bit-reversal permutation
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}

	sign := -1.0
	if inverse {
		sign = 1.0
	}

	for length := 2; length <= n; length <<= 1 {
		angle := sign * 2.0 * math.Pi / float64(length)
		wlen := cmplx.Exp(complex(0, angle))
		for i := 0; i < n; i += length {
			w := complex(1.0, 0.0)
			half := length / 2
			for k := 0; k < half; k++ {
				u := x[i+k]
				v := x[i+k+half] * w
				x[i+k] = u + v
				x[i+k+half] = u - v
				w *= wlen
			}
		}
	}

	if inverse {
		scale := complex(1.0/float64(n), 0)
		for i := range x {
			x[i] *= scale
		}
	}
}

// bluestein computes the DFT of x for arbitrary length using Bluestein's
// chirp-Z algorithm, which reduces the problem to a convolution computed
// via a power-of-2 FFT.
func bluestein(x []complex128, inverse bool) []complex128 {
	n := len(x)
	// Find smallest power of 2 m >= 2*n-1
	m := 1
	for m < 2*n-1 {
		m <<= 1
	}

	sign := -1.0
	if inverse {
		sign = 1.0
	}

	// Chirp sequence: chirp[k] = exp(j * pi * k^2 / n * sign)
	chirp := make([]complex128, n)
	for k := 0; k < n; k++ {
		angle := sign * math.Pi * float64(k) * float64(k) / float64(n)
		chirp[k] = cmplx.Exp(complex(0, angle))
	}

	// a[k] = x[k] * chirp[k]
	a := make([]complex128, m)
	for k := 0; k < n; k++ {
		a[k] = x[k] * chirp[k]
	}
	// a[n:m] = 0 (already zero-initialized)

	// b = chirp sequence padded for circular convolution
	b := make([]complex128, m)
	b[0] = cmplx.Conj(chirp[0])
	for k := 1; k < n; k++ {
		b[k] = cmplx.Conj(chirp[k])
		b[m-k] = cmplx.Conj(chirp[k])
	}

	// Convolve a and b via FFT (both lengths are power of 2 = m)
	fftRadix2(a, false)
	fftRadix2(b, false)
	c := make([]complex128, m)
	for k := range c {
		c[k] = a[k] * b[k]
	}
	fftRadix2(c, true) // IFFT (radix2 handles scaling)

	// Output: y[k] = c[k] * chirp[k]
	out := make([]complex128, n)
	for k := 0; k < n; k++ {
		out[k] = c[k] * chirp[k]
	}
	if inverse {
		scale := complex(1.0/float64(n), 0)
		for k := range out {
			out[k] *= scale
		}
	}
	return out
}

func isPow2(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// FFT computes the forward discrete Fourier transform of x.
// Uses radix-2 for power-of-2, mixed-radix for 5-smooth, Bluestein otherwise.
func FFT(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		out := make([]complex128, n)
		copy(out, x)
		return out
	}
	out := make([]complex128, n)
	copy(out, x)
	if isPow2(n) {
		fftRadix2(out, false)
	} else if is5Smooth(n) {
		fftMixedRadix(out, false)
	} else {
		return bluestein(x, false)
	}
	return out
}

// IFFT computes the inverse discrete Fourier transform of x.
func IFFT(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		out := make([]complex128, n)
		copy(out, x)
		return out
	}
	out := make([]complex128, n)
	copy(out, x)
	if isPow2(n) {
		fftRadix2(out, true)
	} else if is5Smooth(n) {
		fftMixedRadix(out, true)
	} else {
		return bluestein(x, true)
	}
	return out
}

// RealFFT computes the FFT of a real-valued signal of length n.
// Returns n/2+1 complex values (the positive-frequency half).
// len(x) must equal n; x is zero-padded internally if n > len(x).
func RealFFT(x []float32, n int) []complex128 {
	cx := make([]complex128, n)
	lx := len(x)
	if lx > n {
		lx = n
	}
	for i := 0; i < lx; i++ {
		cx[i] = complex(float64(x[i]), 0)
	}
	if isPow2(n) {
		fftRadix2(cx, false)
	} else if is5Smooth(n) {
		fftMixedRadix(cx, false)
	} else {
		cx = bluestein(cx, false)
	}
	// Return only the first n/2+1 complex values
	return cx[:n/2+1]
}
