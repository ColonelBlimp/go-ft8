// realfft.go — Optimized real-to-complex FFT for the research package.
//
// The standard approach (used by ft8x.RealFFT) converts N real values to
// N complex values and computes a full N-point complex FFT, discarding
// the redundant upper half.  This wastes ~2× work.
//
// This implementation uses the "pack and unpack" trick (identical to what
// FFTW's r2c does internally):
//
//  1. Pack N real values into N/2 complex values:
//       z[k] = x[2k] + j·x[2k+1]
//  2. Compute an N/2-point complex FFT of z → Z[0..N/2-1]
//  3. Unpack Z into the N/2+1 unique real-FFT outputs using Hermitian
//     symmetry relations.
//
// For NFFT1=3840, the half-size FFT is 1920-point (also 5-smooth: 2⁷×3×5),
// so it hits the fast mixed-radix path.  Net result: ~2× faster than the
// naive approach.
//
// Reference: Brigham, "The Fast Fourier Transform", §10-5;
// or any DSP textbook section on "real-valued FFTs".

package research

import (
	"math"
)

// realFFTTwiddles stores pre-computed twiddle factors for the RealFFT unpack
// stage when n == NFFT1 (the only size used in the spectrogram hot-path).
//
// twid[k] = exp(−j·2π·k/NFFT1) for k = 0..NFFT1/4−1.
//
// Twiddles for the upper half of the loop (k = NFFT1/4+1 .. NFFT1/2−1)
// are derived via half-cycle symmetry:  W^(half−k) = −conj(W^k).
// The midpoint k = NFFT1/4 is the constant −j.
var realFFTTwiddles [NFFT1 / 4]complex128 // [960]complex128

func init() {
	const n = NFFT1 // 3840
	for k := range realFFTTwiddles {
		angle := -2.0 * math.Pi * float64(k) / float64(n)
		realFFTTwiddles[k] = complex(math.Cos(angle), math.Sin(angle))
	}
}

// RealFFT computes the forward FFT of a real-valued signal.
//
// x contains the real samples (maybe shorter than n; missing values are
// treated as zero).  n must be even.
//
// Returns n/2+1 complex values representing the positive-frequency half
// of the spectrum (the negative half is the complex conjugate mirror).
//
// The output matches ft8x.RealFFT exactly — the same DFT definition,
// the same unnormalized convention — but runs in roughly half the time.
func RealFFT(x []float32, n int) []complex128 {
	half := n / 2
	lx := len(x)

	// ── Step 1: Pack N real values into N/2 complex values ───────────
	// z[k] = x[2k] + j·x[2k+1]
	z := make([]complex128, half)
	for k := 0; k < half; k++ {
		var re, im float64
		if 2*k < lx {
			re = float64(x[2*k])
		}
		if 2*k+1 < lx {
			im = float64(x[2*k+1])
		}
		z[k] = complex(re, im)
	}

	// ── Step 2: N/2-point complex FFT ────────────────────────────────
	// FFT routes to mixed-radix for 5-smooth sizes (1920 = 2⁷×3×5).
	Z := FFT(z)

	// ── Step 3: Unpack to N/2+1 real-FFT outputs ────────────────────
	//
	// Given Z[k] = DFT_{N/2}(z)[k], the full-length DFT X[k] is:
	//
	//   Xe[k] = (Z[k] + Z*[N/2−k]) / 2          (even-indexed DFT)
	//   Xo[k] = −j · (Z[k] − Z*[N/2−k]) / 2    (odd-indexed DFT)
	//   X[k]  = Xe[k] + W_N^k · Xo[k]           (recombine)
	//
	// where W_N^k = exp(−j·2π·k/N), and Z indices are mod N/2.

	out := make([]complex128, half+1)

	// Special cases: k=0 and k=N/2 (both real-valued for real input)
	out[0] = complex(real(Z[0])+imag(Z[0]), 0)
	out[half] = complex(real(Z[0])-imag(Z[0]), 0)

	// General case: k = 1 .. N/2−1
	//
	// For n == NFFT1 (the spectrogram hot-path), twiddle factors are read
	// from the pre-computed realFFTTwiddles table using half-cycle symmetry:
	//   k < quarter:  tw = twid[k]
	//   k == quarter: tw = −j
	//   k > quarter:  tw = −conj(twid[half−k])
	quarter := half / 2
	useTable := n == NFFT1

	for k := 1; k < half; k++ {
		A := Z[k]               // Z[k]
		B := conj128(Z[half-k]) // Z*[N/2−k]

		xe := (A + B) * complex(0.5, 0) // even part
		// −j · (A − B) / 2  =  (imag(A−B) − j·real(A−B)) / 2
		diff := A - B
		xo := complex(imag(diff), -real(diff)) * complex(0.5, 0) // odd part

		var tw complex128
		if useTable {
			if k < quarter {
				tw = realFFTTwiddles[k]
			} else if k == quarter {
				tw = complex(0, -1) // exp(−jπ/2) = −j
			} else {
				tw = -conj128(realFFTTwiddles[half-k])
			}
		} else {
			angle := -2.0 * math.Pi * float64(k) / float64(n)
			tw = complex(math.Cos(angle), math.Sin(angle))
		}

		out[k] = xe + tw*xo
	}

	return out
}

// conj128 returns the complex conjugate.
func conj128(z complex128) complex128 {
	return complex(real(z), -imag(z))
}
