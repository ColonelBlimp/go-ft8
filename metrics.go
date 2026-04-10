package ft8x

import (
	"math"
)

// ComputeSymbolSpectra extracts the complex and magnitude spectra for all
// NN=79 channel symbols from the downsampled signal cd0, starting at
// sample offset ibest.
//
// Returns:
//
//	cs  [8][NN]complex128  – complex amplitudes at each tone for each symbol
//	s8  [8][NN]float64     – |cs| (magnitude) for each tone/symbol pair
func ComputeSymbolSpectra(cd0 []complex128, ibest int) ([8][NN]complex128, [8][NN]float64) {
	var cs [8][NN]complex128
	var s8 [8][NN]float64

	for k := 0; k < NN; k++ {
		i1 := ibest + k*32

		var csymb [32]complex128
		if i1 >= 0 && i1+31 <= NP2-1 {
			for j := 0; j < 32; j++ {
				csymb[j] = cd0[i1+j]
			}
		}

		// 32-point forward FFT.
		cx := make([]complex128, 32)
		copy(cx, csymb[:])
		fftRadix2(cx, false)

		// Bins 0..7 (Go 0-indexed) correspond to the 8 FSK tones at
		// frequencies 0..7 × (fs2/32) Hz relative to baseband DC.
		// Fortran uses 1-indexed: cs(0:7,k) = csymb(1:8)/1e3
		// which is bins 0..7 in 0-indexed (DC through tone 7).
		scale := 1.0 / 1e3
		for t := 0; t < 8; t++ {
			cs[t][k] = cx[t] * complex(scale, 0)
			s8[t][k] = math.Abs(real(cs[t][k] * complexConj(cs[t][k])))
		}
	}
	// Fix magnitudes: use cmplx.Abs equivalent
	for t := 0; t < 8; t++ {
		for k := 0; k < NN; k++ {
			r := real(cs[t][k])
			im := imag(cs[t][k])
			s8[t][k] = math.Sqrt(r*r + im*im)
		}
	}

	return cs, s8
}

func complexConj(z complex128) complex128 {
	return complex(real(z), -imag(z))
}

// one returns true if bit j (0=LSB) of i is set.
// Equivalent to the Fortran one(i,j) precomputed array.
func one(i, j int) bool {
	return (i>>uint(j))&1 != 0
}

// maxvalMasked returns the maximum of vals[i] where mask(i) is true.
// If no element satisfies the mask, returns -1e30.
func maxvalMasked(vals []float64, mask func(int) bool) float64 {
	best := -1e30
	for i, v := range vals {
		if mask(i) && v > best {
			best = v
		}
	}
	return best
}

// ComputeSoftMetrics computes the four sets of soft-decision metrics
// (bmeta, bmetb, bmetc, bmetd) for the 174 LDPC LLR values from the
// complex symbol spectra.
//
// Port of the metric-computation block in subroutine ft8b from
// wsjt-wsjtx/lib/ft8/ft8b.f90.
func ComputeSoftMetrics(cs *[8][NN]complex128) (bmeta, bmetb, bmetc, bmetd [174]float64) {
	// For each of nsym = 1, 2, 3 we iterate over both halves of the data
	// (the two runs of 29 data symbols that flank the middle sync block).
	// Each symbol contributes 3 bits; two symbols 6 bits; three symbols 9 bits.
	//
	// Symbol mapping (0-indexed):
	//   ihalf=0: data symbols at channel positions k+7   for k=0..28
	//   ihalf=1: data symbols at channel positions k+43  for k=0..28
	//
	// Bit indices (1-indexed as in Fortran, mapped to 0-indexed below):
	//   i32 = (k)*3 + ihalf*87   (0-indexed)

	for nsym := 1; nsym <= 3; nsym++ {
		nt := 1 << (3 * nsym) // 8, 64, 512
		ibmax := 3*nsym - 1   // 2, 5, 8

		s2 := make([]float64, nt)

		for ihalf := 0; ihalf < 2; ihalf++ {
			for k := 0; k < 29; k += nsym {
				ks := k + 7
				if ihalf == 1 {
					ks = k + 43
				}

				// Compute s2: coherent combination of nsym symbol spectra.
				for idx := 0; idx < nt; idx++ {
					i1 := (idx >> (6)) & 7
					i2 := (idx >> 3) & 7
					i3 := idx & 7
					switch nsym {
					case 1:
						t := GrayMap[i3]
						r := real(cs[t][ks])
						im := imag(cs[t][ks])
						s2[idx] = math.Sqrt(r*r + im*im)
					case 2:
						if ks+1 < NN {
							t2 := GrayMap[i2]
							t3 := GrayMap[i3]
							z := cs[t2][ks] + cs[t3][ks+1]
							r, im := real(z), imag(z)
							s2[idx] = math.Sqrt(r*r + im*im)
						}
					case 3:
						if ks+2 < NN {
							t1 := GrayMap[i1]
							t2 := GrayMap[i2]
							t3 := GrayMap[i3]
							z := cs[t1][ks] + cs[t2][ks+1] + cs[t3][ks+2]
							r, im := real(z), imag(z)
							s2[idx] = math.Sqrt(r*r + im*im)
						}
					}
				}

				// Bit index base (0-indexed in Go; Fortran uses 1-indexed).
				i32 := k*3 + ihalf*87

				for ib := 0; ib <= ibmax; ib++ {
					bitPos := ibmax - ib // which bit of the symbol index
					max1 := maxvalMasked(s2, func(i int) bool { return one(i, bitPos) })
					max0 := maxvalMasked(s2, func(i int) bool { return !one(i, bitPos) })
					bm := max1 - max0

					idx := i32 + ib
					if idx >= 174 {
						continue
					}

					switch nsym {
					case 1:
						bmeta[idx] = bm
						den := max1
						if max0 > den {
							den = max0
						}
						if den > 0 {
							bmetd[idx] = bm / den
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

	normalizeBmet(bmeta[:])
	normalizeBmet(bmetb[:])
	normalizeBmet(bmetc[:])
	normalizeBmet(bmetd[:])

	return
}

// normalizeBmet normalizes the metric array to unit variance (in-place).
// Port of subroutine normalizebmet from ft8b.f90.
func normalizeBmet(bmet []float64) {
	n := float64(len(bmet))
	av := 0.0
	for _, v := range bmet {
		av += v
	}
	av /= n

	av2 := 0.0
	for _, v := range bmet {
		av2 += v * v
	}
	av2 /= n

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
	for i := range bmet {
		bmet[i] /= sig
	}
}
