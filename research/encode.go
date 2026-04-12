// encode.go — Tone generation and GFSK signal synthesis for the research package.
//
// Port of genft8.f90 (entry get_ft8_tones_from_77bits) and gen_ft8wave.f90.
//
// Pure port — no production dependency.

package research

import "math"

// GenFT8Tones generates the 79 channel tones from 77 message bits.
//
// Port of genft8.f90 entry get_ft8_tones_from_77bits (lines 28–44).
//
// Steps: append CRC-14 → encode174_91 → Costas sync + Gray-mapped data tones.
func GenFT8Tones(msgBits [77]int8) [NN]int {
	// Build 91-bit message: 77 message bits + 14 CRC bits.
	crc := computeCRC14(msgBits)
	var message91 [LDPCk]int8
	copy(message91[:77], msgBits[:])
	for i := 0; i < 14; i++ {
		message91[77+i] = int8((crc >> uint(13-i)) & 1)
	}

	// Encode to 174-bit codeword.
	codeword := encode174_91NoCRC(message91)

	// Build tone array: S7 D29 S7 D29 S7
	var itone [NN]int

	// Costas sync arrays at positions 0–6, 36–42, 72–78.
	for i := 0; i < 7; i++ {
		itone[i] = Icos7[i]
		itone[36+i] = Icos7[i]
		itone[NN-7+i] = Icos7[i]
	}

	// 58 data tones: Gray-map each 3-bit group from the codeword.
	k := 7
	for j := 0; j < 58; j++ {
		i := 3 * j
		if j == 29 {
			k += 7 // skip second Costas block
		}
		indx := int(codeword[i])*4 + int(codeword[i+1])*2 + int(codeword[i+2])
		itone[k] = GrayMap[indx]
		k++
	}

	return itone
}

// gfskPulse computes the GFSK frequency-smoothing pulse.
//
// Port of gfsk_pulse.f90.
func gfskPulse(bt, t float64) float64 {
	c := math.Pi * math.Sqrt(2.0/math.Ln2)
	return 0.5 * (math.Erf(c*bt*(t+0.5)) - math.Erf(c*bt*(t-0.5)))
}

// GenFT8CWave generates the complex GFSK reference waveform for a signal
// at frequency f0 with the given tone sequence.
//
// Port of gen_ft8wave.f90 with FT8 parameters (nsym=79, nsps=1920, bt=2.0,
// fsample=12000, icmplx=1).
//
// Returns a []complex128 of length NFRAME (= NN*NSPS = 151680).
func GenFT8CWave(itone [NN]int, f0 float64) []complex128 {
	const (
		nsym    = NN
		nsps    = NSPS
		bt      = 2.0
		fsample = Fs
		nwave   = nsym * nsps // NFRAME = 151680
		twopi   = 2.0 * math.Pi
		dt      = 1.0 / fsample
	)

	// Step 1: Compute the GFSK frequency-smoothing pulse (3*nsps samples).
	pulseLen := 3 * nsps
	pulse := make([]float64, pulseLen)
	for i := 0; i < pulseLen; i++ {
		tt := (float64(i) - 1.5*float64(nsps)) / float64(nsps)
		pulse[i] = gfskPulse(bt, tt)
	}

	// Step 2: Build the smoothed frequency waveform dphi.
	// Length = (nsym+2)*nsps samples; first and last symbols are extended dummies.
	dphiLen := (nsym + 2) * nsps
	dphi := make([]float64, dphiLen)
	dphiPeak := twopi * 1.0 / float64(nsps) // hmod = 1.0

	// Accumulate pulse-shaped frequency deviation for each symbol.
	for j := 0; j < nsym; j++ {
		ib := j * nsps
		tone := float64(itone[j])
		for s := 0; s < pulseLen; s++ {
			dphi[ib+s] += dphiPeak * pulse[s] * tone
		}
	}

	// Dummy symbol at beginning (tone = itone[0]).
	tone0 := float64(itone[0])
	for s := nsps; s < pulseLen; s++ {
		dphi[s-nsps] += dphiPeak * tone0 * pulse[s]
	}

	// Dummy symbol at end (tone = itone[nsym-1]).
	toneLast := float64(itone[nsym-1])
	ib := nsym * nsps
	for s := 0; s < 2*nsps; s++ {
		dphi[ib+s] += dphiPeak * toneLast * pulse[s]
	}

	// Add carrier frequency offset.
	f0dphi := twopi * f0 * dt
	for i := range dphi {
		dphi[i] += f0dphi
	}

	// Step 3: Generate complex waveform (skip the leading dummy symbol).
	cwave := make([]complex128, nwave)
	phi := 0.0
	for k := 0; k < nwave; k++ {
		j := nsps + k // offset past the dummy symbol
		sin, cos := math.Sincos(phi)
		cwave[k] = complex(cos, sin)
		phi = math.Mod(phi+dphi[j], twopi)
	}

	// Step 4: Envelope shaping — raised-cosine ramp on first and last nramp samples.
	nramp := nsps / 8 // 240
	for i := 0; i < nramp; i++ {
		ramp := (1.0 - math.Cos(twopi*float64(i)/float64(2*nramp))) / 2.0
		cwave[i] = complex(
			real(cwave[i])*ramp,
			imag(cwave[i])*ramp,
		)
	}
	k1 := nsym*nsps - nramp
	for i := 0; i < nramp; i++ {
		ramp := (1.0 + math.Cos(twopi*float64(i)/float64(2*nramp))) / 2.0
		cwave[k1+i] = complex(
			real(cwave[k1+i])*ramp,
			imag(cwave[k1+i])*ramp,
		)
	}

	return cwave
}
