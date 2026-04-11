package research

import (
	"math"
	"testing"
)

// realFFTNoTable is the old version without pre-computed twiddles.
func realFFTNoTable(x []float32, n int) []complex128 {
	half := n / 2
	lx := len(x)
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
	Z := FFT(z)
	out := make([]complex128, half+1)
	out[0] = complex(real(Z[0])+imag(Z[0]), 0)
	out[half] = complex(real(Z[0])-imag(Z[0]), 0)
	for k := 1; k < half; k++ {
		A := Z[k]
		B := conj128(Z[half-k])
		xe := (A + B) * complex(0.5, 0)
		diff := A - B
		xo := complex(imag(diff), -real(diff)) * complex(0.5, 0)
		angle := -2.0 * math.Pi * float64(k) / float64(n)
		tw := complex(math.Cos(angle), math.Sin(angle))
		out[k] = xe + tw*xo
	}
	return out
}
func BenchmarkRealFFT_WithTable(b *testing.B) {
	buf := make([]float32, NSPS)
	for i := range buf {
		buf[i] = float32(i) * 0.001
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RealFFT(buf, NFFT1)
	}
}
func BenchmarkRealFFT_NoTable(b *testing.B) {
	buf := make([]float32, NSPS)
	for i := range buf {
		buf[i] = float32(i) * 0.001
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		realFFTNoTable(buf, NFFT1)
	}
}
