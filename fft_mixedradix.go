// fft_mixedradix.go — Mixed-radix Cooley-Tukey FFT for 5-smooth sizes.
//
// Supports sizes whose prime factorisation contains only 2, 3, and 5.
// This includes the key FT8 sizes: 3840 (2⁸×3×5), 3200 (2⁷×5²),
// and 192000 (2⁸×3×5³).
//
// For non-5-smooth sizes, fall back to Bluestein (handled by the caller).
//
// Performance: avoids Bluestein's 3× power-of-2 overhead. For 192000
// points, Bluestein internally uses a 524288-point radix-2 FFT (×3),
// while mixed-radix works directly on 192000 points.

package ft8x

import "math"

// is5Smooth reports whether n > 0 has only prime factors 2, 3, and 5.
func is5Smooth(n int) bool {
	if n <= 0 {
		return false
	}
	for n%2 == 0 {
		n /= 2
	}
	for n%3 == 0 {
		n /= 3
	}
	for n%5 == 0 {
		n /= 5
	}
	return n == 1
}

// smallestFactor returns the smallest prime factor of n among {2, 3, 5}.
// Returns n itself if none of those divide n (caller should not use
// mixed-radix in that case).
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
	return n
}

// fftMixedRadix computes the DFT of x in-place for 5-smooth sizes using
// Cooley-Tukey decomposition with radix-2, 3, and 5 butterflies.
//
// Convention matches fftRadix2: forward (inverse=false) has no scaling;
// inverse (inverse=true) scales output by 1/n.
func fftMixedRadix(x []complex128, inverse bool) {
	n := len(x)
	if n <= 1 {
		return
	}

	sign := -1.0
	if inverse {
		sign = 1.0
	}

	out := make([]complex128, n)
	fftMRec(x, out, n, sign)
	copy(x, out)

	if inverse {
		s := complex(1.0/float64(n), 0)
		for i := range x {
			x[i] *= s
		}
	}
}

// fftMRec recursively computes the DFT of input[0:n] into output[0:n].
// sign is -1 for forward, +1 for inverse (no 1/N scaling).
func fftMRec(input, output []complex128, n int, sign float64) {
	if n == 1 {
		output[0] = input[0]
		return
	}

	f := smallestFactor(n)
	m := n / f

	// Decimation-in-time: extract f interleaved sub-sequences, transform each.
	subs := make([][]complex128, f)
	for j := 0; j < f; j++ {
		sub := make([]complex128, m)
		for k := 0; k < m; k++ {
			sub[k] = input[j+f*k]
		}
		subOut := make([]complex128, m)
		fftMRec(sub, subOut, m, sign)
		subs[j] = subOut
	}

	// Combine: for each k ∈ [0, m), compute f outputs at k, k+m, …, k+(f-1)m.
	// tw[j] = W_n^{k·j} · subs[j][k]   (twiddle-modified sub-DFT values)
	// output[k + r·m] = DFT_f(tw)[r]
	for k := 0; k < m; k++ {
		// Compute twiddle-modified values.
		var tw [5]complex128
		if k == 0 {
			for j := 0; j < f; j++ {
				tw[j] = subs[j][0]
			}
		} else {
			baseAngle := sign * 2.0 * math.Pi * float64(k) / float64(n)
			wk := complex(math.Cos(baseAngle), math.Sin(baseAngle))
			w := complex(1, 0)
			for j := 0; j < f; j++ {
				tw[j] = w * subs[j][k]
				w *= wk
			}
		}

		switch f {
		case 2:
			mrButterfly2(tw[:], output, k, m)
		case 3:
			mrButterfly3(tw[:], output, k, m, sign)
		case 5:
			mrButterfly5(tw[:], output, k, m, sign)
		default:
			// Fallback: naive DFT for any other prime (shouldn't happen
			// for 5-smooth sizes).
			for r := 0; r < f; r++ {
				var sum complex128
				for j := 0; j < f; j++ {
					angle := sign * 2.0 * math.Pi * float64(r*j) / float64(f)
					sum += complex(math.Cos(angle), math.Sin(angle)) * tw[j]
				}
				output[k+r*m] = sum
			}
		}
	}
}

// mrButterfly2 computes a 2-point DFT: out = {a+b, a−b}.
func mrButterfly2(tw []complex128, out []complex128, k, m int) {
	out[k] = tw[0] + tw[1]
	out[k+m] = tw[0] - tw[1]
}

// mrButterfly3 computes a 3-point DFT using the Winograd formulation.
//
//	X[0] = a + (b+c)
//	X[1] = (a − (b+c)/2) + j·sign·(√3/2)·(b−c)
//	X[2] = (a − (b+c)/2) − j·sign·(√3/2)·(b−c)
func mrButterfly3(tw []complex128, out []complex128, k, m int, sign float64) {
	const sqrt3over2 = 0.86602540378443865 // √3/2

	t1 := tw[1] + tw[2]
	t2 := tw[0] - t1*complex(0.5, 0)
	t3 := complex(0, sign*sqrt3over2) * (tw[1] - tw[2])

	out[k] = tw[0] + t1
	out[k+m] = t2 + t3
	out[k+2*m] = t2 - t3
}

// mrButterfly5 computes a 5-point DFT using symmetric decomposition.
//
// Uses the identities for 5th roots of unity with precomputed
// cos(2π/5), cos(4π/5), sin(2π/5), sin(4π/5).
func mrButterfly5(tw []complex128, out []complex128, k, m int, sign float64) {
	const (
		c1 = 0.30901699437494742  // cos(2π/5)
		c2 = -0.80901699437494742 // cos(4π/5)
		s1 = 0.95105651629515357  // sin(2π/5)
		s2 = 0.58778525229247313  // sin(4π/5)
	)

	t1p := tw[1] + tw[4] // symmetric pair
	t1m := tw[1] - tw[4] // antisymmetric pair
	t2p := tw[2] + tw[3]
	t2m := tw[2] - tw[3]

	a1 := tw[0] + complex(c1, 0)*t1p + complex(c2, 0)*t2p
	a2 := tw[0] + complex(c2, 0)*t1p + complex(c1, 0)*t2p

	// b = j·sign · (linear combination of antisymmetric parts)
	b1arg := complex(s1, 0)*t1m + complex(s2, 0)*t2m
	b2arg := complex(s2, 0)*t1m - complex(s1, 0)*t2m
	// Multiply by j·sign: (0+j·sign)·(re+j·im) = −sign·im + j·sign·re
	b1 := complex(-sign*imag(b1arg), sign*real(b1arg))
	b2 := complex(-sign*imag(b2arg), sign*real(b2arg))

	out[k] = tw[0] + t1p + t2p
	out[k+m] = a1 + b1
	out[k+2*m] = a2 + b2
	out[k+3*m] = a2 - b2
	out[k+4*m] = a1 - b1
}
