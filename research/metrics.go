// metrics.go — Symbol spectra and soft-metric extraction for the research package.
//
// Port of the spectra/metric blocks in ft8b.f90 lines 154–233.
// Source of truth: wsjt-wsjtx/lib/ft8/ft8b.f90.

package research

import (
	"math"
)

// ComputeSymbolSpectra extracts the complex and magnitude spectra for all
// NN=79 channel symbols from the downsampled signal cd0, starting at
// sample offset ibest.
//
// Returns:
//
//	cs[tone][symbol]  — complex amplitude (scaled by 1e-3)
//	s8[tone][symbol]  — magnitude of raw FFT output (UNscaled)
//
// Port of ft8b.f90 lines 154–161:
//
//	do k=1,NN
//	  i1=ibest+(k-1)*32
//	  csymb=cmplx(0.0,0.0)
//	  if( i1.ge.0 .and. i1+31 .le. NP2-1 ) csymb=cd0(i1:i1+31)
//	  call four2a(csymb,32,1,-1,1)          !c2c forward FFT
//	  cs(0:7,k)=csymb(1:8)/1e3
//	  s8(0:7,k)=abs(csymb(1:8))
//	enddo
//
// Fortran cs is cs(0:7, 1:NN) — 0-indexed tone, 1-indexed symbol.
// Go cs is cs[0..7][0..NN-1] — 0-indexed in both.
// Fortran k=1 → Go index 0; Fortran csymb(1:8) → Go FFT output bins 0..7.
func ComputeSymbolSpectra(cd0 []complex128, ibest int) ([8][NN]complex128, [8][NN]float64) {
	var cs [8][NN]complex128
	var s8 [8][NN]float64

	for k := 1; k <= NN; k++ {
		i1 := ibest + (k-1)*32

		// csymb = cmplx(0.0, 0.0)
		// if( i1.ge.0 .and. i1+31 .le. NP2-1 ) csymb = cd0(i1:i1+31)
		var csymb [32]complex128
		if i1 >= 0 && i1+31 <= NP2-1 {
			for j := 0; j < 32; j++ {
				csymb[j] = cd0[i1+j]
			}
		}

		// call four2a(csymb,32,1,-1,1)   — 32-point c2c forward FFT
		cx := make([]complex128, 32)
		copy(cx, csymb[:])
		fft32(cx) // in-place radix-2 forward FFT, unnormalized

		// cs(0:7,k) = csymb(1:8) / 1e3
		// s8(0:7,k) = abs(csymb(1:8))
		//
		// Fortran csymb(1:8) is 1-indexed → Go cx[0:8] is 0-indexed.
		// abs() on a complex number = sqrt(re² + im²).
		for t := 0; t < 8; t++ {
			cs[t][k-1] = cx[t] * complex(1e-3, 0) // /1e3
			r, im := real(cx[t]), imag(cx[t])
			s8[t][k-1] = math.Sqrt(r*r + im*im) // abs(complex)
		}
	}

	return cs, s8
}

// fft32 computes a 32-point in-place forward FFT (decimation-in-time radix-2).
// Matches four2a with isign=-1: X[k] = sum_n x[n] * exp(-j*2*pi*n*k/32).
// Unnormalized (no 1/N scaling).
func fft32(x []complex128) {
	n := len(x)
	// Bit-reversal permutation.
	j := 0
	for i := 0; i < n-1; i++ {
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
		m := n >> 1
		for m >= 1 && j >= m {
			j -= m
			m >>= 1
		}
		j += m
	}
	// Cooley-Tukey butterfly stages.
	// isign = -1 → exp(-j*2*pi/N) per Fortran convention.
	for stage := 1; stage < n; stage <<= 1 {
		theta := -math.Pi / float64(stage)
		wm := complex(math.Cos(theta), math.Sin(theta))
		for k := 0; k < n; k += stage << 1 {
			w := complex(1, 0)
			for jj := 0; jj < stage; jj++ {
				t := w * x[k+jj+stage]
				u := x[k+jj]
				x[k+jj] = u + t
				x[k+jj+stage] = u - t
				w *= wm
			}
		}
	}
}

// ComputeSoftMetrics computes the four sets of soft-decision metrics
// (bmeta, bmetb, bmetc, bmetd) for the 174 LDPC LLR values from the
// complex symbol spectra.
//
// Port of ft8b.f90 lines 182–233:
//
//	do nsym=1,3
//	  nt=2**(3*nsym)
//	  do ihalf=1,2
//	    do k=1,29,nsym
//	      if(ihalf.eq.1) ks=k+7
//	      if(ihalf.eq.2) ks=k+43
//	      ...
//	      i32=1+(k-1)*3+(ihalf-1)*87
//	      ...
//	    enddo
//	  enddo
//	enddo
//	call normalizebmet(bmeta,174)
//	call normalizebmet(bmetb,174)
//	call normalizebmet(bmetc,174)
//	call normalizebmet(bmetd,174)
func ComputeSoftMetrics(cs *[8][NN]complex128) (bmeta, bmetb, bmetc, bmetd [174]float64) {

	// Fortran: logical one(0:511,0:8)
	//   one(i,j) = iand(i, 2**j) .ne. 0
	one := func(i, j int) bool {
		return (i>>uint(j))&1 != 0
	}

	// Fortran: data graymap/0,1,3,2,5,6,4,7/
	graymap := GrayMap

	for nsym := 1; nsym <= 3; nsym++ {
		nt := 1 << (3 * nsym) // 8, 64, 512

		s2 := make([]float64, nt)

		for ihalf := 1; ihalf <= 2; ihalf++ {
			for k := 1; k <= 29; k += nsym {
				// Fortran: if(ihalf.eq.1) ks=k+7; if(ihalf.eq.2) ks=k+43
				// Fortran ks is 1-indexed symbol index.
				// Go cs is 0-indexed in symbol dim, so ks-1 for access.
				var ks int
				if ihalf == 1 {
					ks = k + 7
				} else {
					ks = k + 43
				}

				// Fortran lines 189–202: compute s2(0:nt-1)
				for i := 0; i < nt; i++ {
					i1 := i / 64
					i2 := (i & 63) / 8
					i3 := i & 7

					switch nsym {
					case 1:
						// s2(i) = abs(cs(graymap(i3), ks))
						z := cs[graymap[i3]][ks-1] // ks-1: Fortran→Go index
						r, im := real(z), imag(z)
						s2[i] = math.Sqrt(r*r + im*im)
					case 2:
						// s2(i) = abs(cs(graymap(i2),ks) + cs(graymap(i3),ks+1))
						z := cs[graymap[i2]][ks-1] + cs[graymap[i3]][ks]
						r, im := real(z), imag(z)
						s2[i] = math.Sqrt(r*r + im*im)
					case 3:
						// s2(i) = abs(cs(graymap(i1),ks) + cs(graymap(i2),ks+1) + cs(graymap(i3),ks+2))
						z := cs[graymap[i1]][ks-1] + cs[graymap[i2]][ks] + cs[graymap[i3]][ks+1]
						r, im := real(z), imag(z)
						s2[i] = math.Sqrt(r*r + im*im)
					}
				}

				// Fortran line 203: i32 = 1 + (k-1)*3 + (ihalf-1)*87
				// This is 1-based in Fortran. For Go 0-based: i32 = (k-1)*3 + (ihalf-1)*87
				i32 := (k-1)*3 + (ihalf-1)*87

				// Fortran: ibmax = 2,5,8 for nsym = 1,2,3
				var ibmax int
				switch nsym {
				case 1:
					ibmax = 2
				case 2:
					ibmax = 5
				case 3:
					ibmax = 8
				}

				for ib := 0; ib <= ibmax; ib++ {
					bitPos := ibmax - ib

					// bm = maxval(s2(0:nt-1), one(0:nt-1, ibmax-ib))
					//    - maxval(s2(0:nt-1), .not.one(0:nt-1, ibmax-ib))
					max1 := -1e30
					max0 := -1e30
					for idx := 0; idx < nt; idx++ {
						if one(idx, bitPos) {
							if s2[idx] > max1 {
								max1 = s2[idx]
							}
						} else {
							if s2[idx] > max0 {
								max0 = s2[idx]
							}
						}
					}
					bm := max1 - max0

					// Fortran: if(i32+ib .gt. 174) cycle
					// Fortran i32 is 1-based, so i32+ib > 174.
					// Go i32 is 0-based, so i32+ib >= 174.
					idx := i32 + ib
					if idx >= 174 {
						continue
					}

					switch nsym {
					case 1:
						bmeta[idx] = bm
						// den = max(maxval with one, maxval without one)
						den := max1
						if max0 > den {
							den = max0
						}
						if den > 0.0 {
							bmetd[idx] = bm / den
						} else {
							bmetd[idx] = 0.0
						}
					case 2:
						bmetb[idx] = bm
					case 3:
						bmetc[idx] = bm
					}
				}
			}
		}
	}

	// Fortran lines 230–233: normalize all four metric arrays.
	normalizeBmet(bmeta[:])
	normalizeBmet(bmetb[:])
	normalizeBmet(bmetc[:])
	normalizeBmet(bmetd[:])

	return
}

// normalizeBmet normalizes a metric array to unit variance.
//
// Port of subroutine normalizebmet from ft8b.f90 lines 466–479:
//
//	bmetav  = sum(bmet) / n
//	bmet2av = sum(bmet*bmet) / n
//	var = bmet2av - bmetav*bmetav
//	if(var > 0) then bmetsig = sqrt(var)
//	else             bmetsig = sqrt(bmet2av)
//	bmet = bmet / bmetsig
func normalizeBmet(bmet []float64) {
	n := float64(len(bmet))

	// bmetav = sum(bmet) / n
	av := 0.0
	for _, v := range bmet {
		av += v
	}
	av /= n

	// bmet2av = sum(bmet*bmet) / n
	av2 := 0.0
	for _, v := range bmet {
		av2 += v * v
	}
	av2 /= n

	// var = bmet2av - bmetav**2
	variance := av2 - av*av
	var sig float64
	if variance > 0 {
		sig = math.Sqrt(variance)
	} else {
		sig = math.Sqrt(av2)
	}
	if sig == 0 {
		return
	}

	// bmet = bmet / bmetsig
	for i := range bmet {
		bmet[i] /= sig
	}
}
