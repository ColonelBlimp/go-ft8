// subtract.go — Signal subtraction for the research package.
//
// Port of subroutine subtractft8 from wsjt-wsjtx/lib/ft8/subtractft8.f90.
//
// Pure port — no production dependency.

package research

import (
	"math"
	"math/cmplx"
	"sync"
)

const subtractNFILT = 4000

var (
	subtractOnce          sync.Once
	subtractFilterFreq    []complex128 // FFT of normalized cos² window (length NMAX)
	subtractEndCorrection [subtractNFILT/2 + 1]float64
)

// initSubtractFilter builds the low-pass filter and end-correction table.
//
// Port of the first==.true. block in subtractft8.f90 (lines 25–42).
//
// The filter is a cos² window of length NFILT+1, normalized by its sum,
// circular-shifted, and forward-FFT'd. The end-correction factors compensate
// for the filter transient at the edges of the frame.
//
// Normalization note: the Fortran applies fac=1/N after the forward FFT
// because four2a is unnormalized in both directions. Our FFT() is also
// unnormalized, but IFFT() normalizes by 1/N. To get the correct convolution
// result from IFFT(FFT(signal) * filter), we must NOT divide the filter by N.
func initSubtractFilter() {
	subtractOnce.Do(func() {
		halfFilt := subtractNFILT / 2 // 2000

		// Build cos² window: window[j] for j = -halfFilt..+halfFilt
		// Indexed as window[j+halfFilt] for j in [-halfFilt, halfFilt].
		windowLen := subtractNFILT + 1
		window := make([]float64, windowLen)
		sumw := 0.0
		for j := -halfFilt; j <= halfFilt; j++ {
			v := math.Cos(math.Pi * float64(j) / float64(subtractNFILT))
			window[j+halfFilt] = v * v
			sumw += v * v
		}

		// Place normalized window into complex array of length NMAX,
		// then circular-shift by NFILT/2+1.
		cw := make([]complex128, NMAX)
		for i := 0; i < windowLen; i++ {
			cw[i] = complex(window[i]/sumw, 0)
		}
		cw = cshift(cw, halfFilt+1)

		// Forward FFT (unnormalized).
		subtractFilterFreq = FFT(cw)

		// Precompute end-correction factors.
		// endcorrection[j] = 1 / (1 - sum(window[j-1:halfFilt]) / sumw)
		// where j runs from 1 to halfFilt+1 (Fortran 1-based).
		// In 0-based window indexing: window[j-1+halfFilt .. 2*halfFilt].
		for j := 1; j <= halfFilt+1; j++ {
			partialSum := 0.0
			for k := j - 1; k <= halfFilt; k++ {
				partialSum += window[k+halfFilt]
			}
			subtractEndCorrection[j-1] = 1.0 / (1.0 - partialSum/sumw)
		}
	})
}

// SubtractFT8 removes a decoded signal from audio using the FFT-based
// low-pass filter method.
//
// Port of subroutine subtractft8 from wsjt-wsjtx/lib/ft8/subtractft8.f90
// (lrefinedt=.false. path).
//
// Algorithm:
//   - Generate GFSK reference waveform cref at frequency f0
//   - Conjugate-multiply audio with reference to get complex amplitude
//   - Low-pass filter via FFT convolution with cos² window
//   - Apply end-correction for filter transients
//   - Subtract reconstructed signal: dd[j] -= 2 * real(cfilt[i] * cref[i])
func SubtractFT8(dd []float32, itone [NN]int, f0, xdt float64) {
	initSubtractFilter()

	halfFilt := subtractNFILT / 2

	// Generate complex reference waveform.
	cref := GenFT8CWave(itone, f0)

	// Compute starting sample index.
	nstart := int(xdt*Fs) + 1 // Fortran: nstart = dt*12000 + 1

	// Conjugate-multiply: camp[i] = dd[nstart-1+i] * conj(cref[i])
	cfilt := make([]complex128, NMAX)
	for i := 0; i < NFRAME; i++ {
		j := nstart - 1 + i // 0-based index into dd
		if j >= 0 && j < NMAX {
			cfilt[i] = complex(float64(dd[j]), 0) * cmplx.Conj(cref[i])
		}
	}
	// cfilt[NFRAME:] is already zero (zero-padded to NMAX).

	// Forward FFT.
	cfilt = FFT(cfilt)

	// Multiply by filter in frequency domain.
	for i := range cfilt {
		cfilt[i] *= subtractFilterFreq[i]
	}

	// Inverse FFT.
	cfilt = IFFT(cfilt)

	// Apply end-correction to compensate for filter transients.
	// First NFILT/2+1 samples.
	for j := 0; j <= halfFilt; j++ {
		cfilt[j] *= complex(subtractEndCorrection[j], 0)
	}
	// Last NFILT/2+1 samples (reversed).
	for j := 0; j <= halfFilt; j++ {
		idx := NFRAME - 1 - j
		cfilt[idx] *= complex(subtractEndCorrection[j], 0)
	}

	// Subtract the reconstructed signal.
	for i := 0; i < NFRAME; i++ {
		j := nstart - 1 + i
		if j >= 0 && j < NMAX {
			z := cfilt[i] * complex(real(cref[i]), imag(cref[i]))
			dd[j] -= 2.0 * float32(real(z))
		}
	}
}
