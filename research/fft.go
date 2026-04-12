// fft.go — Mixed-radix FFT for the research package.
//
// Implements a recursive Cooley-Tukey decimation-in-time FFT supporting
// radix-2, radix-3, and radix-5 butterflies. All FT8 FFT sizes (192000,
// 180000, 3200, 1920) are 5-smooth and use this path.
//
// Replaces the FFTW-based four2a.f90 from the Fortran codebase.
//
// Pure implementation — no production dependency.

package research

import (
	"math"
	"math/cmplx"
)

// FFT computes the forward complex-to-complex FFT (unnormalized).
//
// X[k] = sum_{n=0}^{N-1} x[n] * exp(-j*2*pi*k*n/N)
//
// Input length must be 5-smooth (only factors of 2, 3, 5).
func FFT(x []complex128) []complex128 {
	out := make([]complex128, len(x))
	copy(out, x)
	return fftMixedRadix(out)
}

// IFFT computes the inverse complex-to-complex FFT (normalized by 1/N).
//
// x[n] = (1/N) * sum_{k=0}^{N-1} X[k] * exp(+j*2*pi*k*n/N)
func IFFT(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		out := make([]complex128, n)
		copy(out, x)
		return out
	}

	// Conjugate → forward FFT → conjugate → scale by 1/N.
	buf := make([]complex128, n)
	for i, v := range x {
		buf[i] = cmplx.Conj(v)
	}
	buf = fftMixedRadix(buf)
	scale := 1.0 / float64(n)
	for i, v := range buf {
		buf[i] = complex(real(v)*scale, -imag(v)*scale)
	}
	return buf
}

// smallestFactor returns the smallest prime factor of n from {2, 3, 5}.
// Panics if n has a prime factor > 5 (not 5-smooth).
func smallestFactor(n int) int {
	if n%2 == 0 {
		return 2
	}
	if n%3 == 0 {
		return 3
	}
	if n%5 == 0 {
		return 5
	}
	panic("fft: size is not 5-smooth")
}

// fftMixedRadix computes an in-place forward FFT using recursive
// decimation-in-time with mixed radix-2/3/5 butterflies.
func fftMixedRadix(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		return x
	}

	p := smallestFactor(n) // radix
	m := n / p             // sub-transform length

	// Decimation: split into p interleaved sub-sequences of length m.
	subs := make([][]complex128, p)
	for j := 0; j < p; j++ {
		subs[j] = make([]complex128, m)
		for k := 0; k < m; k++ {
			subs[j][k] = x[k*p+j]
		}
	}

	// Recurse on each sub-sequence.
	for j := 0; j < p; j++ {
		subs[j] = fftMixedRadix(subs[j])
	}

	// Combine with twiddle factors and p-point DFT butterflies.
	twopi := 2.0 * math.Pi
	result := make([]complex128, n)

	switch p {
	case 2:
		for k := 0; k < m; k++ {
			// Twiddle: W_N^k = exp(-j*2*pi*k/N)
			angle := -twopi * float64(k) / float64(n)
			sin, cos := math.Sincos(angle)
			w := complex(cos, sin)
			t := w * subs[1][k]
			result[k] = subs[0][k] + t
			result[k+m] = subs[0][k] - t
		}

	case 3:
		// W3 = exp(-j*2*pi/3) constants
		const (
			cos3 = -0.5                   // cos(2π/3)
			sin3 = -0.8660254037844386468 // -sin(2π/3)
		)
		for k := 0; k < m; k++ {
			s0 := subs[0][k]

			angle1 := -twopi * float64(k) / float64(n)
			sin1, cos1 := math.Sincos(angle1)
			w1 := complex(cos1, sin1)
			s1 := w1 * subs[1][k]

			angle2 := -twopi * float64(2*k) / float64(n)
			sin2, cos2 := math.Sincos(angle2)
			w2 := complex(cos2, sin2)
			s2 := w2 * subs[2][k]

			// 3-point DFT:
			// X[0] = s0 + s1 + s2
			// X[1] = s0 + s1*W3 + s2*W3^2
			// X[2] = s0 + s1*W3^2 + s2*W3
			t1 := s1 + s2
			t2 := s1 - s2

			result[k] = s0 + t1
			result[k+m] = s0 + complex(cos3*real(t1)-sin3*imag(t2), cos3*imag(t1)+sin3*real(t2))
			result[k+2*m] = s0 + complex(cos3*real(t1)+sin3*imag(t2), cos3*imag(t1)-sin3*real(t2))
		}

	case 5:
		// W5 = exp(-j*2*pi/5) constants
		cos1_5 := math.Cos(twopi / 5)  //  0.30901699...
		sin1_5 := -math.Sin(twopi / 5) // -0.95105652...
		cos2_5 := math.Cos(2 * twopi / 5)
		sin2_5 := -math.Sin(2 * twopi / 5)

		for k := 0; k < m; k++ {
			s0 := subs[0][k]

			// Compute twiddle factors W_N^(j*k) for j=1..4
			angle1 := -twopi * float64(k) / float64(n)
			sin1, cos1 := math.Sincos(angle1)
			w1 := complex(cos1, sin1)

			angle2 := -twopi * float64(2*k) / float64(n)
			sin2, cos2 := math.Sincos(angle2)
			w2 := complex(cos2, sin2)

			angle3 := -twopi * float64(3*k) / float64(n)
			sin3, cos3 := math.Sincos(angle3)
			w3 := complex(cos3, sin3)

			angle4 := -twopi * float64(4*k) / float64(n)
			sin4, cos4 := math.Sincos(angle4)
			w4 := complex(cos4, sin4)

			s1 := w1 * subs[1][k]
			s2 := w2 * subs[2][k]
			s3 := w3 * subs[3][k]
			s4 := w4 * subs[4][k]

			// 5-point DFT using W5 roots:
			// X[q] = s0 + sum_{j=1}^{4} s_j * W5^(j*q)
			// W5^0=1, W5^1, W5^2, W5^3=conj(W5^2), W5^4=conj(W5^1)
			result[k] = s0 + s1 + s2 + s3 + s4

			for q := 1; q < 5; q++ {
				// W5^(j*q) for each j
				var sum complex128
				sum = s0
				for j := 1; j <= 4; j++ {
					jq := (j * q) % 5
					var wq complex128
					switch jq {
					case 0:
						wq = 1
					case 1:
						wq = complex(cos1_5, sin1_5)
					case 2:
						wq = complex(cos2_5, sin2_5)
					case 3:
						wq = complex(cos2_5, -sin2_5) // conj of W5^2
					case 4:
						wq = complex(cos1_5, -sin1_5) // conj of W5^1
					}
					sj := [4]complex128{s1, s2, s3, s4}
					sum += wq * sj[j-1]
				}
				result[k+q*m] = sum
			}
		}
	}

	return result
}
