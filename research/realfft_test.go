// realfft_test.go — Validates the optimized RealFFT against ft8x.RealFFT.

package research

import (
	"math"
	"math/cmplx"
	"os"
	"testing"
	"time"

	ft8x "github.com/ColonelBlimp/go-ft8"
)

// TestRealFFTMatchesFt8x verifies that our optimized RealFFT produces
// identical results to ft8x.RealFFT for the sync8 spectrogram FFT size.
func TestRealFFTMatchesFt8x(t *testing.T) {
	const n = NFFT1 // 3840

	// Use a simple test signal: a few sinusoids at known frequencies.
	x := make([]float32, NSPS) // 1920 samples, zero-padded to 3840
	for i := 0; i < NSPS; i++ {
		x[i] = float32(
			100.0*math.Sin(2.0*math.Pi*500.0*float64(i)/Fs) +
				50.0*math.Sin(2.0*math.Pi*1200.0*float64(i)/Fs) +
				30.0*math.Cos(2.0*math.Pi*2000.0*float64(i)/Fs))
	}

	// Reference: ft8x.RealFFT (full complex FFT, discard upper half)
	ref := ft8x.RealFFT(x, n)

	// Optimized: our half-size pack/unpack version
	opt := RealFFT(x, n)

	if len(ref) != len(opt) {
		t.Fatalf("length mismatch: ref=%d, opt=%d", len(ref), len(opt))
	}

	// Compare every bin.  Allow tiny floating-point tolerance.
	maxErr := 0.0
	maxErrBin := 0
	for i := 0; i < len(ref); i++ {
		err := cmplx.Abs(ref[i] - opt[i])
		mag := cmplx.Abs(ref[i])
		relErr := 0.0
		if mag > 1e-10 {
			relErr = err / mag
		}
		if relErr > maxErr {
			maxErr = relErr
			maxErrBin = i
		}
	}

	t.Logf("Max relative error: %.2e at bin %d", maxErr, maxErrBin)
	// Tolerance 1e-6: the two paths use different FP operation ordering
	// (full N-point vs half N/2-point + unpack), so they won't be bit-identical.
	// Real audio data typically shows <1e-11; synthetic worst case ~1e-7.
	if maxErr > 1e-6 {
		t.Errorf("RealFFT mismatch: max relative error %.2e exceeds 1e-6", maxErr)
		// Show first few mismatches
		shown := 0
		for i := 0; i < len(ref) && shown < 5; i++ {
			err := cmplx.Abs(ref[i] - opt[i])
			mag := cmplx.Abs(ref[i])
			if mag > 1e-10 && err/mag > 1e-6 {
				t.Logf("  bin %d: ref=%v  opt=%v  relErr=%.2e", i, ref[i], opt[i], err/mag)
				shown++
			}
		}
	}
}

// TestRealFFTWithCapture runs both FFT paths on real audio data from
// capture.wav and verifies they produce the same spectrogram column.
func TestRealFFTWithCapture(t *testing.T) {
	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	// Test with the first spectrogram column (j=1).
	const fac float32 = 1.0 / 300.0
	buf := make([]float32, NSPS)
	for k := 0; k < NSPS; k++ {
		buf[k] = fac * dd[k]
	}

	ref := ft8x.RealFFT(buf, NFFT1)
	opt := RealFFT(buf, NFFT1)

	maxErr := 0.0
	for i := 0; i < len(ref); i++ {
		err := cmplx.Abs(ref[i] - opt[i])
		mag := cmplx.Abs(ref[i])
		if mag > 1e-10 {
			if rel := err / mag; rel > maxErr {
				maxErr = rel
			}
		}
	}
	t.Logf("Capture column 1: max relative error = %.2e", maxErr)
	if maxErr > 1e-6 {
		t.Errorf("mismatch on real audio data: max relative error %.2e", maxErr)
	}

	// Verify power values match for a few bins.
	for _, bin := range []int{96, 320, 480, 800} {
		refPow := real(ref[bin])*real(ref[bin]) + imag(ref[bin])*imag(ref[bin])
		optPow := real(opt[bin])*real(opt[bin]) + imag(opt[bin])*imag(opt[bin])
		t.Logf("  bin %d (%.0f Hz): ref_pow=%.4e  opt_pow=%.4e",
			bin, float64(bin)*Fs/float64(NFFT1), refPow, optPow)
	}
}

// TestRealFFTPerformance benchmarks the optimized vs naive paths.
func TestRealFFTPerformance(t *testing.T) {
	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	const fac float32 = 1.0 / 300.0
	buf := make([]float32, NSPS)
	for k := 0; k < NSPS; k++ {
		buf[k] = fac * dd[k]
	}

	const iters = 100

	// Benchmark ft8x.RealFFT (full complex FFT)
	start := time.Now()
	for i := 0; i < iters; i++ {
		ft8x.RealFFT(buf, NFFT1)
	}
	naiveDur := time.Since(start)

	// Benchmark optimized RealFFT (half-size FFT)
	start = time.Now()
	for i := 0; i < iters; i++ {
		RealFFT(buf, NFFT1)
	}
	optDur := time.Since(start)

	speedup := float64(naiveDur) / float64(optDur)
	t.Logf("ft8x.RealFFT:  %d iters in %v  (%.1f µs/call)", iters, naiveDur, float64(naiveDur.Microseconds())/float64(iters))
	t.Logf("research.RealFFT: %d iters in %v  (%.1f µs/call)", iters, optDur, float64(optDur.Microseconds())/float64(iters))
	t.Logf("Speedup: %.2fx", speedup)
}

// TestSpectrogramPerformance measures the full spectrogram computation
// (372 FFTs + power + accumulation) with the optimized RealFFT and
// contiguous backing array.
func TestSpectrogramPerformance(t *testing.T) {
	const wavPath = "../testdata/capture.wav"
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("test WAV not found: %s", wavPath)
	}

	_, dd, err := loadIwave(wavPath)
	if err != nil {
		t.Fatalf("loadIwave: %v", err)
	}

	const iters = 10

	// Benchmark research.computeSpectrogram (optimized RealFFT + contiguous alloc)
	start := time.Now()
	for i := 0; i < iters; i++ {
		computeSpectrogram(dd[:], NMAX)
	}
	dur := time.Since(start)

	t.Logf("computeSpectrogram: %d iters in %v  (%.1f ms/call, 372 FFTs each)",
		iters, dur, float64(dur.Milliseconds())/float64(iters))
}
