// downsample.go — Downsampler for the research package.
//
// Port of subroutine ft8_downsample from wsjt-wsjtx/lib/ft8/ft8_downsample.f90
// and subroutine twkfreq1 from wsjt-wsjtx/lib/ft8/twkfreq1.f90.
//
// Ported directly from the Fortran source — this file has zero production
// ft8x dependencies.

package research

import (
	"math"
	"math/cmplx"
)

// Downsampler holds cached FFT state so the expensive 192000-point
// transform is only recomputed when the input audio changes.
//
// Port of the save'd state in ft8_downsample.f90 (lines 8–14):
//
//	complex cx(0:NFFT1/2)
//	real taper(0:100)
//	save x,cx,first,taper
type Downsampler struct {
	cx    []complex128 // Cached spectrum (NFFT1DS/2+1 elements)
	taper [101]float64 // Raised-cosine edge taper
	ready bool
}

// NewDownsampler creates a Downsampler and precomputes the edge taper.
//
// Port of ft8_downsample.f90 lines 16–21:
//
//	if(first) then
//	   pi=4.0*atan(1.0)
//	   do i=0,100
//	     taper(i)=0.5*(1.0+cos(i*pi/100))
//	   enddo
//	   first=.false.
//	endif
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
// wsjt-wsjtx/lib/ft8/ft8_downsample.f90 (all 52 lines).
func (d *Downsampler) Downsample(dd []float32, newdat *bool, f0 float64) []complex128 {
	const (
		nfft1 = NFFT1DS // 192000
		nfft2 = NFFT2   // 3200
	)

	// Fortran lines 23–29:
	//   if(newdat) then
	//     x(1:NMAX)=dd
	//     x(NMAX+1:NFFT1+2)=0.
	//     call four2a(cx,NFFT1,1,-1,0)    !r2c FFT
	//     newdat=.false.
	//   endif
	if *newdat || d.cx == nil {
		d.cx = RealFFT(dd, nfft1)
		*newdat = false
	}

	// Fortran lines 30–36:
	//   df=12000.0/NFFT1
	//   baud=12000.0/NSPS
	//   i0=nint(f0/df)
	//   ft=f0+8.5*baud
	//   it=min(nint(ft/df),NFFT1/2)
	//   fb=f0-1.5*baud
	//   ib=max(1,nint(fb/df))
	df := Fs / float64(nfft1)
	i0 := int(math.Round(f0 / df))

	baud := Fs / NSPS
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

	// Fortran lines 37–42:
	//   k=0
	//   c1=0.
	//   do i=ib,it
	//    c1(k)=cx(i)
	//    k=k+1
	//   enddo
	c1 := make([]complex128, nfft2)
	k := 0
	for i := ib; i <= it && k < nfft2; i++ {
		c1[k] = d.cx[i]
		k++
	}

	// Fortran lines 43–44:
	//   c1(0:100)=c1(0:100)*taper(100:0:-1)
	//   c1(k-1-100:k-1)=c1(k-1-100:k-1)*taper
	for i := 0; i <= 100 && i < k; i++ {
		c1[i] *= complex(d.taper[100-i], 0)
	}
	for i := 0; i <= 100 && k-1-i >= 0; i++ {
		c1[k-1-i] *= complex(d.taper[i], 0)
	}

	// Fortran line 45:
	//   c1=cshift(c1,i0-ib)
	c1 = cshift(c1, i0-ib)

	// Fortran line 46:
	//   call four2a(c1,NFFT2,1,1,1)       !c2c FFT back to time domain
	//
	// four2a with isign=+1 is an unnormalized inverse FFT.
	// Go's IFFT normalizes by 1/N, so we must compensate.
	result := IFFT(c1)

	// Fortran lines 47–48:
	//   fac=1.0/sqrt(float(NFFT1)*NFFT2)
	//   c1=fac*c1
	//
	// The Fortran four2a(isign=+1) does NOT divide by N. Our Go IFFT
	// already divided by nfft2, so we undo that and apply the Fortran
	// scaling: multiply by nfft2 / sqrt(nfft1 * nfft2).
	fac := float64(nfft2) / math.Sqrt(float64(nfft1)*float64(nfft2))
	for i := range result {
		result[i] *= complex(fac, 0)
	}

	return result
}

// cshift is Fortran's CSHIFT(array, shift): circular left-shift by shift
// positions.  Matches Fortran intrinsic CSHIFT semantics exactly.
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
// Port of subroutine twkfreq1 from wsjt-wsjtx/lib/ft8/twkfreq1.f90 (all 27 lines).
//
// The polynomial basis functions are Legendre polynomials P2..P4, evaluated
// on the normalized range x ∈ [−1, +1] across the signal duration:
//
//	x = s*(i − x0),  s = 2/npts,  x0 = (npts+1)/2
//
// a[0] = constant frequency offset (Hz)
// a[1] = linear chirp (Hz)
// a[2..4] = higher-order corrections (rarely non-zero)
func TwkFreq1(ca []complex128, fsample float64, a [5]float64) []complex128 {
	npts := len(ca)
	cb := make([]complex128, npts)
	twopi := 2.0 * math.Pi

	// Fortran lines 10–13:
	//   w=1.0
	//   wstep=1.0
	//   x0=0.5*(npts+1)
	//   s=2.0/npts
	w := complex(1.0, 0.0)
	x0 := 0.5 * float64(npts+1)
	s := 2.0 / float64(npts)

	// Fortran lines 14–23:
	//   do i=1,npts
	//      x=s*(i-x0)
	//      p2=1.5*x*x - 0.5
	//      p3=2.5*(x**3) - 1.5*x
	//      p4=4.375*(x**4) - 3.75*(x**2) + 0.375
	//      dphi=(a(1) + x*a(2) + p2*a(3) + p3*a(4) + p4*a(5)) * (twopi/fsample)
	//      wstep=cmplx(cos(dphi),sin(dphi))
	//      w=w*wstep
	//      cb(i)=w*ca(i)
	//   enddo
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
