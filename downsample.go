package ft8x

import (
	"math"
	"math/cmplx"
)

// Downsampler holds cached FFT state so the expensive 192000-point
// transform is only recomputed when the input audio changes.
type Downsampler struct {
	cx    []complex128 // Cached spectrum (NFFT1DS/2+1 elements)
	taper [101]float64 // Raised-cosine edge taper
	ready bool
}

// NewDownsampler creates a Downsampler and precomputes the edge taper.
func NewDownsampler() *Downsampler {
	d := &Downsampler{}
	pi := math.Pi
	for i := 0; i <= 100; i++ {
		d.taper[i] = 0.5 * (1.0 + math.Cos(float64(i)*pi/100.0))
	}
	return d
}

// Downsample mixes the audio in dd to baseband at f0 Hz, then decimates
// from 12000 Hz to 200 Hz (NDOWN=60×), returning a complex signal of
// length NFFT2 (3200 samples).
//
// When newdat is true the forward FFT of dd is recomputed; when false the
// cached spectrum from the previous call is reused.  On return newdat is
// set to false.
//
// This is a direct port of subroutine ft8_downsample from
// wsjt-wsjtx/lib/ft8/ft8_downsample.f90.
func (d *Downsampler) Downsample(dd []float32, newdat *bool, f0 float64) []complex128 {
	const (
		nfft1 = NFFT1DS // 192000
		nfft2 = NFFT2   // 3200
		nmax  = NMAX    // 180000
	)

	if *newdat || d.cx == nil {
		// Real-to-complex FFT of the 15-second audio buffer.
		// dd has NMAX=180000 samples; zero-pad to nfft1=192000.
		d.cx = RealFFT(dd, nfft1)
		*newdat = false
	}

	df := Fs / float64(nfft1) // Hz per FFT bin (~0.0625 Hz)
	i0 := int(math.Round(f0 / df))

	baud := Fs / NSPS // 6.25 Hz
	ft := f0 + 8.5*baud
	fb := f0 - 1.5*baud

	it := int(math.Round(ft / df))
	if it > nfft1/2 {
		it = nfft1 / 2
	}
	ib := int(math.Round(fb / df))
	if ib < 1 {
		ib = 1
	}

	// Build the NFFT2-point complex array c1 from the spectral window.
	c1 := make([]complex128, nfft2)
	k := 0
	for i := ib; i <= it && k < nfft2; i++ {
		c1[k] = d.cx[i]
		k++
	}

	// Apply raised-cosine taper to the first and last 101 elements.
	for i := 0; i <= 100 && i < k; i++ {
		c1[i] *= complex(d.taper[100-i], 0)
	}
	for i := 0; i <= 100 && k-1-i >= 0; i++ {
		c1[k-1-i] *= complex(d.taper[i], 0)
	}

	// Circular shift so that the signal at f0 sits at DC.
	shift := i0 - ib
	c1 = cshift(c1, shift)

	// IFFT back to time domain (c2c IFFT of nfft2 points).
	result := IFFT(c1)

	// Scale to match the Fortran normalisation: 1/sqrt(nfft1 * nfft2).
	// Go's IFFT already divides by nfft2, but Fortran's four2a(c2c, isign=1)
	// does NOT normalise. So we undo the 1/nfft2 and apply the Fortran scaling.
	fac := float64(nfft2) / math.Sqrt(float64(nfft1)*float64(nfft2))
	for i := range result {
		result[i] *= complex(fac, 0)
	}

	return result
}

// cshift is Fortran's CSHIFT(array, shift): circular left-shift by shift
// positions.
func cshift(x []complex128, shift int) []complex128 {
	n := len(x)
	if n == 0 {
		return x
	}
	shift = ((shift % n) + n) % n
	if shift == 0 {
		return x
	}
	out := make([]complex128, n)
	copy(out, x[shift:])
	copy(out[n-shift:], x[:shift])
	return out
}

// TwkFreq1 applies a polynomial frequency correction to the complex signal ca,
// returning the corrected signal cb.  a[0] is the primary frequency offset in Hz
// (with sign flipped: positive a[0] shifts the signal down by a[0] Hz).
//
// Port of subroutine twkfreq1 from wsjt-wsjtx/lib/ft8/twkfreq1.f90.
func TwkFreq1(ca []complex128, fsample float64, a [5]float64) []complex128 {
	npts := len(ca)
	cb := make([]complex128, npts)
	twopi := 2.0 * math.Pi
	w := complex(1.0, 0.0)
	x0 := 0.5 * float64(npts+1)
	s := 2.0 / float64(npts)

	for i := 1; i <= npts; i++ {
		x := s * (float64(i) - x0)
		p2 := 1.5*x*x - 0.5
		p3 := 2.5*(x*x*x) - 1.5*x
		p4 := 4.375*(x*x*x*x) - 3.75*(x*x) + 0.375
		dphi := (a[0] + x*a[1] + p2*a[2] + p3*a[3] + p4*a[4]) * (twopi / fsample)
		wstep := cmplx.Exp(complex(0, dphi))
		w *= wstep
		cb[i-1] = w * ca[i-1]
	}
	return cb
}
