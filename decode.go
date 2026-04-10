// Package ft8x implements the FT8 digital radio protocol decoder,
// ported from the WSJT-X Fortran source code.
package ft8x

import (
	"fmt"
	"math"
	"strings"
)

// DecodeParams holds tunable parameters for the FT8 decoder.
type DecodeParams struct {
	// Depth controls the OSD search depth: 1=BP only, 2=BP+OSD(0), 3=BP+OSD(2).
	Depth int
	// APEnabled enables a-priori (AP) decoding passes.
	APEnabled bool
	// APCQOnly restricts AP decoding to CQ-only a-priori information.
	APCQOnly bool
	// APWidth is the frequency window (Hz) within which AP decoding is applied.
	APWidth float64
}

// DefaultDecodeParams returns sensible defaults matching WSJT-X ndepth=2.
func DefaultDecodeParams() DecodeParams {
	return DecodeParams{
		Depth:   2,
		APWidth: 25.0,
	}
}

// DecodeCandidate is the result of decoding one FT8 signal candidate.
type DecodeCandidate struct {
	// Message is the decoded text (up to 37 characters).
	Message string
	// Freq is the refined carrier frequency estimate (Hz).
	Freq float64
	// DT is the time offset relative to the nominal start of the 15-second period (seconds).
	DT float64
	// SNR is the estimated signal-to-noise ratio (dB, 2500 Hz bandwidth).
	SNR float64
	// NHardErrors is the number of hard-decision bit errors after decoding.
	NHardErrors int
	// Tones holds the 79 channel tone indices (0–7) for subtracting the signal.
	Tones [NN]int
}

// Decode is the top-level FT8 decoder.  It takes 15 seconds of audio sampled
// at 12000 Hz and a list of candidate {frequency, time-offset} pairs to try
// and return all successfully decoded messages.
//
// This is the Go equivalent of calling ft8b once per candidate from the
// WSJT-X decoder thread.
func Decode(audio []float32, candidates []CandidateFreq, params DecodeParams) []DecodeCandidate {
	if len(audio) < NMAX {
		padded := make([]float32, NMAX)
		copy(padded, audio)
		audio = padded
	}

	ds := NewDownsampler()
	var results []DecodeCandidate
	seen := make(map[string]bool) // deduplicate by message

	for i, cand := range candidates {
		// Only compute the big 192k FFT on the first candidate;
		// subsequent calls reuse the cached spectrum via the Downsampler.
		newdat := (i == 0)
		result, ok := DecodeSingle(audio, ds, cand.Freq, cand.DT, newdat, params)
		if !ok {
			continue
		}
		key := result.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		results = append(results, result)
	}
	return results
}

// CandidateFreq is a {frequency, DT} pair to try decoding.
type CandidateFreq struct {
	Freq      float64 // Hz
	DT        float64 // seconds
	SyncPower float64 // normalized sync metric (0 if not computed)
}

// DecodeSingle attempts to decode a single FT8 signal at the given frequency
// and time offset.  newdat=true forces recomputation of the spectrum.
//
// Port of subroutine ft8b from wsjt-wsjtx/lib/ft8/ft8b.f90.
func DecodeSingle(
	dd []float32,
	ds *Downsampler,
	f1 float64,
	xdt float64,
	newdat bool,
	params DecodeParams,
) (DecodeCandidate, bool) {
	const (
		maxOSD  = 2
		ndeepD2 = 2 // Depth 2: order-1 with npre1
		ndeepD3 = 4 // Depth 3: order-2 with npre1 + npre2
	)

	// ── Step 1: downconvert and downsample ────────────────────────────────
	cd0 := ds.Downsample(dd, &newdat, f1)

	// ── Step 2: coarse time search ────────────────────────────────────────
	i0 := int(math.Round((xdt + 0.5) * Fs2))

	smax := 0.0
	ibest := i0
	var ctwkZero [32]complex128
	for i := range ctwkZero {
		ctwkZero[i] = complex(1, 0)
	}
	for idt := i0 - 10; idt <= i0+10; idt++ {
		sync := Sync8d(cd0, idt, ctwkZero, 0)
		if sync > smax {
			smax = sync
			ibest = idt
		}
	}

	// ── Step 3: fine frequency search (±2.5 Hz in 0.5 Hz steps) ─────────
	smax = 0.0
	delfBest := 0.0
	twopi := 2.0 * math.Pi
	for ifr := -5; ifr <= 5; ifr++ {
		delf := float64(ifr) * 0.5
		dphi := twopi * delf * Dt2
		phi := 0.0
		var ctwk [32]complex128
		for i := 0; i < 32; i++ {
			ctwk[i] = complex(math.Cos(phi), math.Sin(phi))
			phi = math.Mod(phi+dphi, twopi)
		}
		sync := Sync8d(cd0, ibest, ctwk, 1)
		if sync > smax {
			smax = sync
			delfBest = delf
		}
	}

	// ── Step 4: apply frequency correction ───────────────────────────────
	var a [5]float64
	a[0] = -delfBest
	cd0 = TwkFreq1(cd0, Fs2, a)
	f1 += delfBest

	// ── Step 5: re-downsample with corrected frequency ────────────────────
	// Reuse the cached 192k FFT (newdat=false); only re-extract around corrected f1.
	// Matches WSJT-X ft8b.f90 line 140: call ft8_downsample(dd0,.false.,f1,cd0)
	newdat2 := false
	cd0 = ds.Downsample(dd, &newdat2, f1)

	// ── Step 6: refine time offset ────────────────────────────────────────
	ss := [9]float64{}
	for idt := -4; idt <= 4; idt++ {
		sync := Sync8d(cd0, ibest+idt, ctwkZero, 0)
		ss[idt+4] = sync
	}
	bestIdx := 0
	for i, v := range ss {
		if v > ss[bestIdx] {
			bestIdx = i
		}
	}
	ibest = ibest + (bestIdx - 4)
	xdt = float64(ibest-1) * Dt2

	// ── Step 7: extract symbol spectra ───────────────────────────────────
	cs, s8 := ComputeSymbolSpectra(cd0, ibest)

	// ── Step 8: hard sync quality check ──────────────────────────────────
	nsync := HardSync(&s8)
	if nsync <= 6 {
		return DecodeCandidate{}, false
	}

	// ── Step 9: compute soft metrics ─────────────────────────────────────
	bmeta, bmetb, bmetc, bmetd := ComputeSoftMetrics(&cs)

	apmag := 0.0
	for _, v := range bmeta {
		av := math.Abs(v * ScaleFac)
		if av > apmag {
			apmag = av
		}
	}
	apmag *= 1.01

	// ── Step 10: multi-pass LDPC decoding ─────────────────────────────────
	npasses := 4
	if params.APEnabled {
		npasses = 4 // AP passes would be added here
	}

	maxOSD2 := -1
	ndeep := ndeepD2
	if params.Depth == 2 {
		maxOSD2 = 0
	} else if params.Depth == 3 {
		maxOSD2 = maxOSD
		ndeep = ndeepD3
	}

	for ipass := 0; ipass < npasses; ipass++ {
		var llrz [LDPCn]float64
		var apmask [LDPCn]int8

		switch ipass {
		case 0:
			for i, v := range bmeta {
				llrz[i] = ScaleFac * v
			}
		case 1:
			for i, v := range bmetb {
				llrz[i] = ScaleFac * v
			}
		case 2:
			for i, v := range bmetc {
				llrz[i] = ScaleFac * v
			}
		case 3:
			for i, v := range bmetd {
				llrz[i] = ScaleFac * v
			}
		}

		res, ok := Decode174_91(llrz, LDPCk, maxOSD2, ndeep, apmask)
		if !ok {
			continue
		}
		if res.NHardErrors < 0 || res.NHardErrors > 36 {
			continue
		}

		// Reject all-zero codeword.
		allZero := true
		for _, b := range res.Codeword {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			continue
		}

		// Validate i3/n3 and decode the message.
		var msg77 [77]int8
		copy(msg77[:], res.Message91[:77])
		c77 := BitsToC77(msg77)

		n3 := readBits(c77, 71, 3)
		i3 := readBits(c77, 74, 3)
		if i3 > 5 || (i3 == 0 && n3 > 6) || (i3 == 0 && n3 == 2) {
			continue
		}

		msg, success := Unpack77(c77)
		if !success {
			continue
		}

		// Compute tones and SNR.
		itone := GenFT8Tones(msg77)

		xsig := 0.0
		xnoi := 0.0
		for sym := 0; sym < NN; sym++ {
			xsig += s8[itone[sym]][sym] * s8[itone[sym]][sym]
			ios := (itone[sym] + 4) % 7
			xnoi += s8[ios][sym] * s8[ios][sym]
		}
		xsnr := -24.0
		if xnoi > 0 {
			arg := xsig/xnoi - 1.0
			if arg > 0.1 {
				xsnr = 10.0*math.Log10(arg) - 27.0
			}
		}

		if nsync <= 10 && xsnr < -24.0 {
			continue // likely false decode
		}
		if xsnr < -24.0 {
			xsnr = -24.0
		}

		return DecodeCandidate{
			Message:     strings.TrimRight(msg, " "),
			Freq:        f1,
			DT:          xdt,
			SNR:         xsnr,
			NHardErrors: res.NHardErrors,
			Tones:       itone,
		}, true
	}

	return DecodeCandidate{}, false
}

// SubtractFT8 removes a decoded signal from the audio buffer to aid
// subsequent decodes (iterative subtraction).  Returns the modified audio.
//
// This is a simplified port of subtractft8.f90 – it reconstructs the ideal
// signal waveform and subtracts it from dd.
func SubtractFT8(dd []float32, itone [NN]int, f1, xdt float64) []float32 {
	out := make([]float32, len(dd))
	copy(out, dd)

	twopi := 2.0 * math.Pi
	dt := 1.0 / Fs

	// Reconstruct the ideal 8-FSK waveform for the 79 symbols.
	for sym := 0; sym < NN; sym++ {
		tone := itone[sym]
		toneFreq := f1 + float64(tone)*Baud

		// Start sample of this symbol.
		t0 := xdt + float64(sym)*float64(NSPS)*dt
		istart := int(math.Round(t0 * Fs))

		phase := 0.0
		for j := 0; j < NSPS; j++ {
			idx := istart + j
			if idx < 0 || idx >= len(out) {
				continue
			}
			// Subtract the real part of a unit-amplitude complex tone.
			out[idx] -= float32(math.Cos(twopi*toneFreq*float64(j)*dt + phase))
		}
	}
	return out
}

// FindCandidates searches for potential FT8 signals using the spectrogram-based
// sync8 algorithm (faithful port of WSJT-X sync8.f90).
//
// audio is 15 s at 12000 Hz. freqMin/freqMax define the search band in Hz.
// dtMin/dtMax are accepted for API compatibility but ignored — sync8 uses its
// own fixed ±2.5 s search range.
//
// Returns a list of {Freq, DT, SyncPower} candidates sorted by sync power.
func FindCandidates(audio []float32, freqMin, freqMax, dtMin, dtMax float64) []CandidateFreq {
	return Sync8FindCandidates(audio, int(freqMin), int(freqMax), 1.3, 0, 600)
}

// FormatDecodeResult formats a DecodeCandidate for printing in the
// WSJT-X style: "HHMMSS  DT   Freq   SNR  Message".
func FormatDecodeResult(c DecodeCandidate, secondsInPeriod int) string {
	return fmt.Sprintf("%6.1f %8.1f %5.1f  %s",
		c.DT, c.Freq, c.SNR, c.Message)
}
