// decode.go — FT8 decode pipeline for the research package.
//
// Port of subroutine ft8b from wsjt-wsjtx/lib/ft8/ft8b.f90
// and the iterative loop from wsjt-wsjtx/lib/ft8_decode.f90.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// DecodeParams holds tunable parameters for the FT8 decoder.
type DecodeParams = ft8x.DecodeParams

// DecodeCandidate is the result of decoding one FT8 signal candidate.
type DecodeCandidate = ft8x.DecodeCandidate

// CandidateFreq is a {frequency, DT} pair to try decoding.
type CandidateFreq = ft8x.CandidateFreq

// DefaultDecodeParams returns sensible defaults matching WSJT-X ndepth=2.
//
// TODO: port from Fortran
func DefaultDecodeParams() DecodeParams {
	return ft8x.DefaultDecodeParams()
}

// DecodeSingle attempts to decode a single FT8 signal at the given frequency
// and time offset.
//
// Port of subroutine ft8b from wsjt-wsjtx/lib/ft8/ft8b.f90.
//
// TODO: port from Fortran
func DecodeSingle(
	dd []float32,
	ds *Downsampler,
	f1 float64,
	xdt float64,
	newdat bool,
	params DecodeParams,
) (DecodeCandidate, bool) {
	return ft8x.DecodeSingle(dd, ds, f1, xdt, newdat, params)
}

// DecodeIterative runs the full FT8 decode pipeline with iterative signal
// subtraction, matching WSJT-X's multi-pass approach from ft8_decode.f90.
//
// Port of the decode subroutine in wsjt-wsjtx/lib/ft8_decode.f90 lines 160–239.
//
// TODO: port from Fortran
func DecodeIterative(audio []float32, params DecodeParams, freqMin, freqMax float64) []DecodeCandidate {
	return ft8x.DecodeIterative(audio, params, freqMin, freqMax)
}

// Sync8FindCandidates searches for potential FT8 signals using the
// spectrogram-based sync8 algorithm.
//
// Port of subroutine sync8 from wsjt-wsjtx/lib/ft8/sync8.f90.
//
// TODO: port from Fortran
func Sync8FindCandidates(audio []float32, freqMin, freqMax int, syncmin float64, nfqso, maxcand int) []CandidateFreq {
	return ft8x.Sync8FindCandidates(audio, freqMin, freqMax, syncmin, nfqso, maxcand)
}
