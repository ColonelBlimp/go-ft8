// encode.go — Tone generation and signal synthesis for the research package.
//
// Port of gen_ft8.f90 and subtractft8.f90 support routines.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// GenFT8Tones generates the 79 channel tones from 77 message bits.
//
// Port of get_ft8_tones_from_77bits.
//
// TODO: port from Fortran
func GenFT8Tones(msgBits [77]int8) [NN]int {
	return ft8x.GenFT8Tones(msgBits)
}

// GenFT8CWave generates the complex GFSK reference waveform for a signal
// at frequency f0 with the given tone sequence.
//
// TODO: port from Fortran
func GenFT8CWave(itone [NN]int, f0 float64) []complex128 {
	return ft8x.GenFT8CWave(itone, f0)
}
