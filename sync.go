package ft8x

import (
	"math"
	"math/cmplx"
)

// syncWaveforms holds the precomputed complex tone waveforms for the
// 7-tone Costas array, one 32-sample waveform per tone index.
var syncWaveforms [7][32]complex128
var syncWaveformsInit bool

func initSyncWaveforms() {
	if syncWaveformsInit {
		return
	}
	twopi := 2.0 * math.Pi
	for i := 0; i < 7; i++ {
		phi := 0.0
		dphi := twopi * float64(Icos7[i]) / 32.0
		for j := 0; j < 32; j++ {
			syncWaveforms[i][j] = cmplx.Exp(complex(0, phi))
			phi = math.Mod(phi+dphi, twopi)
		}
	}
	syncWaveformsInit = true
}

// Sync8d computes the Costas-array sync power for a complex downsampled
// FT8 signal cd0 starting at sample offset i0.
//
// When itwk != 0, each Costas waveform is multiplied element-wise by ctwk
// (a 32-sample complex tone used for fine frequency tweaking).
//
// Returns the sum of correlation powers across all 3 Costas arrays.
//
// Port of subroutine sync8d from wsjt-wsjtx/lib/ft8/sync8d.f90.
func Sync8d(cd0 []complex128, i0 int, ctwk [32]complex128, itwk int) float64 {
	initSyncWaveforms()

	power := func(z complex128) float64 {
		r, im := real(z), imag(z)
		return r*r + im*im
	}

	sync := 0.0
	for i := 0; i < 7; i++ {
		i1 := i0 + i*32
		i2 := i1 + 36*32
		i3 := i1 + 72*32

		csync2 := syncWaveforms[i]
		if itwk != 0 {
			for j := 0; j < 32; j++ {
				csync2[j] = ctwk[j] * csync2[j]
			}
		}

		var z1, z2, z3 complex128
		if i1 >= 0 && i1+31 <= NP2-1 {
			for j := 0; j < 32; j++ {
				z1 += cd0[i1+j] * cmplx.Conj(csync2[j])
			}
		}
		if i2 >= 0 && i2+31 <= NP2-1 {
			for j := 0; j < 32; j++ {
				z2 += cd0[i2+j] * cmplx.Conj(csync2[j])
			}
		}
		if i3 >= 0 && i3+31 <= NP2-1 {
			for j := 0; j < 32; j++ {
				z3 += cd0[i3+j] * cmplx.Conj(csync2[j])
			}
		}
		sync += power(z1) + power(z2) + power(z3)
	}
	return sync
}

// BuildCtwk constructs the 32-sample frequency-tweak waveform for a
// fractional-Hz offset delf at sample rate fs2.
func BuildCtwk(delf, fs2 float64) [32]complex128 {
	var ctwk [32]complex128
	twopi := 2.0 * math.Pi
	dphi := twopi * delf / fs2
	phi := 0.0
	for i := 0; i < 32; i++ {
		ctwk[i] = cmplx.Exp(complex(0, phi))
		phi = math.Mod(phi+dphi, twopi)
	}
	return ctwk
}

// HardSync counts how many of the 21 Costas-array positions are correctly
// identified by taking the argmax of the magnitude spectrum.
// s8[tone][symbol] is the 8×NN magnitude array.
// Returns a count in [0, 21].
func HardSync(s8 *[8][NN]float64) int {
	count := func(offset int) int {
		n := 0
		for k := 0; k < 7; k++ {
			best := 0
			for t := 1; t < 8; t++ {
				if s8[t][offset+k] > s8[best][offset+k] {
					best = t
				}
			}
			if best == Icos7[k] {
				n++
			}
		}
		return n
	}
	return count(0) + count(36) + count(72)
}
