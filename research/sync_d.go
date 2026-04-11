// sync_d.go — Fine time/frequency sync for the research package.
//
// Port of subroutine sync8d from wsjt-wsjtx/lib/ft8/sync8d.f90
// and the nsync computation from wsjt-wsjtx/lib/ft8/ft8b.f90 lines 163–176.

package research

import (
	"math"
)

// csync holds the precomputed Costas waveforms: csync[tone][sample].
// Fortran: csync(0:6,32) with 1-indexed j=1..32.
// Go: csync[0..6][0..31] (0-indexed).
//
// Initialised once on first call (matches Fortran's save/first pattern).
var csync [7][32]complex128
var csyncInit bool

func initCsync() {
	if csyncInit {
		return
	}
	// sync8d.f90 lines 20–31:
	//   twopi=8.0*atan(1.0)
	//   do i=0,6
	//     phi=0.0
	//     dphi=twopi*icos7(i)/32.0
	//     do j=1,32
	//       csync(i,j)=cmplx(cos(phi),sin(phi))
	//       phi=mod(phi+dphi,twopi)
	//     enddo
	//   enddo
	twopi := 8.0 * math.Atan(1.0)
	for i := 0; i <= 6; i++ {
		phi := 0.0
		dphi := twopi * float64(Icos7[i]) / 32.0
		for j := 0; j < 32; j++ {
			csync[i][j] = complex(math.Cos(phi), math.Sin(phi))
			phi = math.Mod(phi+dphi, twopi)
		}
	}
	csyncInit = true
}

// Sync8d computes the Costas-array sync power for a complex downsampled
// FT8 signal cd0 starting at sample offset i0.
//
// When itwk==1, each Costas waveform is multiplied element-wise by ctwk
// (a 32-sample complex tone used for fine frequency tweaking).
//
// Returns the total correlation power across all 3 Costas arrays × 7 tones.
//
// Port of subroutine sync8d from wsjt-wsjtx/lib/ft8/sync8d.f90.
func Sync8d(cd0 []complex128, i0 int, ctwk [32]complex128, itwk int) float64 {
	initCsync()

	// sync8d.f90 line 17: p(z1) = real(z1)**2 + aimag(z1)**2
	p := func(z complex128) float64 {
		r, im := real(z), imag(z)
		return r*r + im*im
	}

	// sync8d.f90 lines 33–47:
	//   sync=0
	//   do i=0,6
	//     i1=i0+i*32
	//     i2=i1+36*32
	//     i3=i1+72*32
	//     csync2=csync(i,1:32)
	//     if(itwk.eq.1) csync2=ctwk*csync2
	//     z1=0; z2=0; z3=0
	//     if(i1.ge.0 .and. i1+31.le.NP2-1) z1=sum(cd0(i1:i1+31)*conjg(csync2))
	//     if(i2.ge.0 .and. i2+31.le.NP2-1) z2=sum(cd0(i2:i2+31)*conjg(csync2))
	//     if(i3.ge.0 .and. i3+31.le.NP2-1) z3=sum(cd0(i3:i3+31)*conjg(csync2))
	//     sync = sync + p(z1) + p(z2) + p(z3)
	//   enddo
	sync := 0.0
	for i := 0; i <= 6; i++ {
		i1 := i0 + i*32
		i2 := i1 + 36*32
		i3 := i1 + 72*32

		// csync2 = csync(i, 1:32)  — copy the 32-sample waveform
		var csync2 [32]complex128
		csync2 = csync[i]

		// if(itwk.eq.1) csync2 = ctwk * csync2   — element-wise multiply
		if itwk == 1 {
			for j := 0; j < 32; j++ {
				csync2[j] = ctwk[j] * csync2[j]
			}
		}

		// Correlate against each of the three Costas array positions.
		// conjg in Fortran = complex conjugate.
		var z1, z2, z3 complex128

		if i1 >= 0 && i1+31 <= NP2-1 {
			for j := 0; j < 32; j++ {
				// cd0(i1+j) * conjg(csync2(j))
				c := csync2[j]
				z1 += cd0[i1+j] * complex(real(c), -imag(c))
			}
		}
		if i2 >= 0 && i2+31 <= NP2-1 {
			for j := 0; j < 32; j++ {
				c := csync2[j]
				z2 += cd0[i2+j] * complex(real(c), -imag(c))
			}
		}
		if i3 >= 0 && i3+31 <= NP2-1 {
			for j := 0; j < 32; j++ {
				c := csync2[j]
				z3 += cd0[i3+j] * complex(real(c), -imag(c))
			}
		}

		sync += p(z1) + p(z2) + p(z3)
	}
	return sync
}

// HardSync counts how many of the 21 Costas-array positions are correctly
// identified by taking the argmax of the magnitude spectrum s8.
//
// s8[tone][symbol] is the 8×NN magnitude array (0-indexed in both dimensions).
//
// Port of ft8b.f90 lines 163–176:
//
//	is1=0; is2=0; is3=0
//	do k=1,7
//	  ip=maxloc(s8(:,k))
//	  if(icos7(k-1).eq.(ip(1)-1)) is1=is1+1
//	  ip=maxloc(s8(:,k+36))
//	  if(icos7(k-1).eq.(ip(1)-1)) is2=is2+1
//	  ip=maxloc(s8(:,k+72))
//	  if(icos7(k-1).eq.(ip(1)-1)) is3=is3+1
//	enddo
//	nsync=is1+is2+is3
//
// Fortran s8 is s8(0:7, 1:NN) — 1-indexed in symbol dimension.
// Go s8 is s8[0..7][0..NN-1] — 0-indexed in both.
// Fortran k=1..7 with s8(:,k) → Go symbol index k-1 = 0..6.
// Fortran s8(:,k+36) → Go symbol index k-1+36 = 36..42.
// Fortran s8(:,k+72) → Go symbol index k-1+72 = 72..78.
func HardSync(s8 *[8][NN]float64) int {
	is1 := 0
	is2 := 0
	is3 := 0

	for k := 1; k <= 7; k++ {
		// Fortran: ip = maxloc(s8(:,k))  — returns 1-based index of max across tones 0..7
		// Go: find argmax of s8[0..7][k-1], result is 0-based tone index
		sym1 := k - 1      // first Costas array (symbols 0..6)
		sym2 := k - 1 + 36 // second Costas array (symbols 36..42)
		sym3 := k - 1 + 72 // third Costas array (symbols 72..78)

		// Fortran: if(icos7(k-1).eq.(ip(1)-1))
		// ip(1) is 1-based, so ip(1)-1 is 0-based tone index.
		// Our argmax is already 0-based.
		if argmax8(s8, sym1) == Icos7[k-1] {
			is1++
		}
		if argmax8(s8, sym2) == Icos7[k-1] {
			is2++
		}
		if argmax8(s8, sym3) == Icos7[k-1] {
			is3++
		}
	}

	return is1 + is2 + is3
}

// argmax8 returns the tone index (0..7) with the largest value at the given symbol.
func argmax8(s8 *[8][NN]float64, sym int) int {
	best := 0
	for t := 1; t < 8; t++ {
		if s8[t][sym] > s8[best][sym] {
			best = t
		}
	}
	return best
}
