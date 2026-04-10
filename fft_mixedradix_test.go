package ft8x

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestIs5Smooth(t *testing.T) {
	tests := []struct {
		n    int
		want bool
	}{
		{1, true}, {2, true}, {3, true}, {4, true}, {5, true},
		{6, true}, {7, false}, {8, true}, {9, true}, {10, true},
		{15, true}, {16, true}, {30, true}, {60, true},
		{3200, true},   // 2^7 × 5^2
		{3840, true},   // 2^8 × 3 × 5
		{192000, true}, // 2^8 × 3 × 5^3
		{11, false}, {13, false}, {17, false}, {77, false},
	}
	for _, tt := range tests {
		if got := is5Smooth(tt.n); got != tt.want {
			t.Errorf("is5Smooth(%d) = %v, want %v", tt.n, got, tt.want)
		}
	}
}

// TestMixedRadixRoundTrip verifies FFT→IFFT reconstructs the input
// for all key 5-smooth sizes.
func TestMixedRadixRoundTrip(t *testing.T) {
	sizes := []int{3, 5, 6, 9, 10, 15, 25, 30, 60, 120, 375, 3200, 3840}
	for _, n := range sizes {
		x := make([]complex128, n)
		for i := range x {
			x[i] = complex(float64(i+1), float64(-i))
		}
		orig := make([]complex128, n)
		copy(orig, x)

		fftMixedRadix(x, false) // forward
		fftMixedRadix(x, true)  // inverse

		maxErr := 0.0
		for i := range x {
			e := cmplx.Abs(x[i] - orig[i])
			if e > maxErr {
				maxErr = e
			}
		}
		if maxErr > 1e-9 {
			t.Errorf("n=%d: round-trip max error = %e (want < 1e-9)", n, maxErr)
		} else {
			t.Logf("n=%d: round-trip max error = %e ✓", n, maxErr)
		}
	}
}

// TestMixedRadixVsBluestein compares the mixed-radix FFT output against
// Bluestein for correctness at all key FT8 sizes.
func TestMixedRadixVsBluestein(t *testing.T) {
	sizes := []int{15, 30, 60, 120, 3200, 3840}
	for _, n := range sizes {
		// Create a test signal with some energy.
		x := make([]complex128, n)
		for i := range x {
			x[i] = complex(math.Sin(2*math.Pi*float64(i*3)/float64(n)),
				math.Cos(2*math.Pi*float64(i*7)/float64(n)))
		}

		// Bluestein result.
		xb := make([]complex128, n)
		copy(xb, x)
		refOut := bluestein(xb, false)

		// Mixed-radix result.
		xm := make([]complex128, n)
		copy(xm, x)
		fftMixedRadix(xm, false)

		maxErr := 0.0
		for i := range xm {
			e := cmplx.Abs(xm[i] - refOut[i])
			if e > maxErr {
				maxErr = e
			}
		}
		if maxErr > 1e-6 {
			t.Errorf("n=%d: mixed-radix vs Bluestein max error = %e (want < 1e-6)", n, maxErr)
		} else {
			t.Logf("n=%d: mixed-radix vs Bluestein max error = %e ✓", n, maxErr)
		}
	}
}

// TestMixedRadixSinusoid verifies the peak bin for a known sinusoid
// at a key FT8 size.
func TestMixedRadixSinusoid(t *testing.T) {
	const n = 3840
	const freq = 1000.0
	const sr = 12000.0

	x := make([]complex128, n)
	for i := range x {
		x[i] = complex(math.Sin(2*math.Pi*freq*float64(i)/sr), 0)
	}
	fftMixedRadix(x, false)

	df := sr / float64(n)
	expectedBin := int(math.Round(freq / df))

	peakBin := 0
	peakPow := 0.0
	for i := 0; i < n/2; i++ {
		p := real(x[i])*real(x[i]) + imag(x[i])*imag(x[i])
		if p > peakPow {
			peakPow = p
			peakBin = i
		}
	}

	if peakBin != expectedBin {
		t.Errorf("peak at bin %d (%.1f Hz), expected bin %d (%.1f Hz)",
			peakBin, float64(peakBin)*df, expectedBin, float64(expectedBin)*df)
	} else {
		t.Logf("3840-pt mixed-radix FFT: peak at bin %d (%.1f Hz) ✓", peakBin, float64(peakBin)*df)
	}
}

// TestRealFFTMixedRadix verifies RealFFT routes through mixed-radix
// and produces the same result as before for key FT8 sizes.
func TestRealFFTMixedRadix(t *testing.T) {
	const n = 3840
	const freq = 500.0
	const sr = 12000.0

	input := make([]float32, n)
	for i := range input {
		input[i] = float32(math.Sin(2 * math.Pi * freq * float64(i) / sr))
	}

	cx := RealFFT(input, n)
	if len(cx) != n/2+1 {
		t.Fatalf("expected %d bins, got %d", n/2+1, len(cx))
	}

	df := sr / float64(n)
	expectedBin := int(math.Round(freq / df))

	peakBin := 0
	peakPow := 0.0
	for i, v := range cx {
		p := real(v)*real(v) + imag(v)*imag(v)
		if p > peakPow {
			peakPow = p
			peakBin = i
		}
	}

	if peakBin != expectedBin {
		t.Errorf("RealFFT peak at bin %d (%.1f Hz), expected bin %d (%.1f Hz)",
			peakBin, float64(peakBin)*df, expectedBin, float64(expectedBin)*df)
	} else {
		t.Logf("RealFFT (3840-pt mixed-radix): peak at bin %d (%.1f Hz) ✓",
			peakBin, float64(peakBin)*df)
	}
}

// BenchmarkFFT192k benchmarks the 192000-point FFT (mixed-radix path).
func BenchmarkFFT192k(b *testing.B) {
	x := make([]complex128, 192000)
	for i := range x {
		x[i] = complex(float64(i), 0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmp := make([]complex128, 192000)
		copy(tmp, x)
		fftMixedRadix(tmp, false)
	}
}

// BenchmarkFFT3840 benchmarks the 3840-point FFT (mixed-radix path).
func BenchmarkFFT3840(b *testing.B) {
	x := make([]complex128, 3840)
	for i := range x {
		x[i] = complex(float64(i), 0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmp := make([]complex128, 3840)
		copy(tmp, x)
		fftMixedRadix(tmp, false)
	}
}
