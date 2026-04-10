package ft8x

import "math"

// Encode174_91 adds a 14-bit CRC to a 77-bit message and returns the
// 174-bit LDPC codeword.
//
// Port of subroutine encode174_91 from wsjt-wsjtx/lib/ft8/encode174_91.f90.
func Encode174_91(message77 [77]int8) [LDPCn]int8 {
	// Compute and append CRC-14.
	crcVal := ComputeCRC14(message77)
	var message91 [LDPCk]int8
	copy(message91[:77], message77[:])
	// Append the 14 CRC bits (MSB first).
	for i := 0; i < 14; i++ {
		message91[77+i] = int8((crcVal >> uint(13-i)) & 1)
	}
	return Encode174_91NoGRC(message91)
}

// GenFT8Tones converts a 77-bit message into the 79 channel tone indices
// (0..7) that are transmitted over the air.
//
// Message structure: S7 D29 S7 D29 S7  (sync=Costas, data=Gray-coded 8-FSK)
//
// Port of subroutine get_ft8_tones_from_77bits / entry point in genft8.f90.
func GenFT8Tones(msgBits [77]int8) [NN]int {
	codeword := Encode174_91(msgBits)
	return TonesToneFromCodeword(codeword)
}

// TonesToneFromCodeword maps a 174-bit LDPC codeword to the 79 on-air tones.
func TonesToneFromCodeword(codeword [LDPCn]int8) [NN]int {
	var itone [NN]int

	// Three sync blocks: positions 0..6, 36..42, 72..78.
	for k := 0; k < 7; k++ {
		itone[k] = Icos7[k]
		itone[36+k] = Icos7[k]
		itone[72+k] = Icos7[k]
	}

	// 58 data symbols: 3 codeword bits per symbol, Gray coded.
	// j indexes the data symbol (0-based); i = 3*j indexes the first codeword bit.
	pos := 7 // channel position (0-indexed), starting after first sync block
	for j := 0; j < ND; j++ {
		i := 3 * j
		indx := int(codeword[i])*4 + int(codeword[i+1])*2 + int(codeword[i+2])
		itone[pos] = GrayMap[indx]
		pos++
		if j == 28 { // after the 29th data symbol, skip the middle sync block
			pos += 7
		}
	}

	return itone
}

// gfskPulse computes the GFSK frequency pulse shape for bandwidth-time
// product bt at normalised time t (in symbol periods).
//
// Port of function gfsk_pulse from wsjt-wsjtx/lib/ft2/gfsk_pulse.f90.
func gfskPulse(bt, t float64) float64 {
	c := math.Pi * math.Sqrt(2.0/math.Log(2.0))
	return 0.5 * (math.Erf(c*bt*(t+0.5)) - math.Erf(c*bt*(t-0.5)))
}

// GenFT8CWave generates a complex reference waveform for an FT8 signal
// with GFSK pulse shaping (bt=2.0, hmod=1.0).
//
// Port of subroutine gen_ft8wave from wsjt-wsjtx/lib/ft8/gen_ft8wave.f90
// (icmplx=1 mode, complex output only).
//
// The returned slice has NFRAME = NN×NSPS = 151680 complex samples
// at the audio sample rate (12000 Hz), representing the ideal signal at
// carrier frequency f0.
func GenFT8CWave(itone [NN]int, f0 float64) []complex128 {
	const (
		nsym  = NN
		nsps  = NSPS
		bt    = 2.0
		nwave = NFRAME // 151680
	)

	twopi := 2.0 * math.Pi
	dt := 1.0 / Fs

	// Compute GFSK frequency-smoothing pulse (3×nsps samples, centred).
	pulse := make([]float64, 3*nsps)
	for i := range pulse {
		tt := (float64(i) - 1.5*float64(nsps)) / float64(nsps)
		pulse[i] = gfskPulse(bt, tt)
	}

	// Build the smoothed phase-increment array dphi.
	// Length = (nsym+2)×nsps to accommodate dummy symbols at each end.
	ndphi := (nsym + 2) * nsps
	dphi := make([]float64, ndphi)
	dphiPeak := twopi / float64(nsps) // hmod = 1.0

	// Accumulate pulse contributions from each data symbol.
	for j := 0; j < nsym; j++ {
		ib := j * nsps
		for k := 0; k < 3*nsps; k++ {
			if ib+k < ndphi {
				dphi[ib+k] += dphiPeak * pulse[k] * float64(itone[j])
			}
		}
	}
	// Dummy symbol at beginning: extend first tone backward.
	for k := 0; k < 2*nsps; k++ {
		dphi[k] += dphiPeak * float64(itone[0]) * pulse[nsps+k]
	}
	// Dummy symbol at end: extend last tone forward.
	for k := 0; k < 2*nsps; k++ {
		idx := nsym*nsps + k
		if idx < ndphi {
			dphi[idx] += dphiPeak * float64(itone[nsym-1]) * pulse[k]
		}
	}

	// Add carrier frequency to every sample.
	dphiCarrier := twopi * f0 * dt
	for i := range dphi {
		dphi[i] += dphiCarrier
	}

	// Generate the complex waveform, skipping the first nsps dummy samples.
	cwave := make([]complex128, nwave)
	phi := 0.0
	for k := 0; k < nwave; k++ {
		j := nsps + k
		cwave[k] = complex(math.Cos(phi), math.Sin(phi))
		phi = math.Mod(phi+dphi[j], twopi)
	}

	// Raised-cosine envelope shaping on the first and last nramp samples.
	nramp := nsps / 8 // 240
	for i := 0; i < nramp; i++ {
		env := (1.0 - math.Cos(twopi*float64(i)/float64(2*nramp))) / 2.0
		cwave[i] *= complex(env, 0)
	}
	k1 := nsym*nsps - nramp
	for i := 0; i < nramp; i++ {
		env := (1.0 + math.Cos(twopi*float64(i)/float64(2*nramp))) / 2.0
		cwave[k1+i] *= complex(env, 0)
	}

	return cwave
}
