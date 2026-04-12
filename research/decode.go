// decode.go — FT8 decode pipeline for the research package.
//
// Port of subroutine ft8b from wsjt-wsjtx/lib/ft8/ft8b.f90
// and the iterative loop from wsjt-wsjtx/lib/ft8_decode.f90 lines 160–239.
//
// Pure port — no production dependency.

package research

import (
	"math"
	"strings"
)

// ── Type definitions ────────────────────────────────────────────────────────

// DecodeParams holds tunable parameters for the FT8 decoder.
type DecodeParams struct {
	// Depth controls the OSD search depth: 1=BP only, 2=BP+OSD(0), 3=BP+OSD(2).
	Depth int
	// APEnabled enables a-priori (AP) decoding passes.
	APEnabled bool
	// APCQOnly restricts AP decoding to CQ-only a-priori information.
	APCQOnly bool
	// APWidth is the frequency window (Hz) within which AP types ≥3 are applied.
	APWidth float64
	// MyCall is the operator's callsign (used for AP types 2–6). Empty = not set.
	MyCall string
	// DxCall is the DX station's callsign (used for AP types 3–6). Empty = not set.
	DxCall string
	// NfQSO is the nominal QSO frequency (Hz) for AP frequency guard.
	// AP types ≥3 are only tried if |f1 − NfQSO| ≤ APWidth.
	// Set to 0 to disable the frequency guard.
	NfQSO float64
	// MaxPasses is the number of subtraction passes for DecodeIterative (default 3).
	MaxPasses int
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
	// APType indicates the a-priori decoding type used (0 = no AP).
	APType int
}

// CandidateFreq is a {frequency, DT} pair to try decoding.
// Identical to Candidate from sync8.go.
type CandidateFreq = Candidate

// DefaultDecodeParams returns sensible defaults matching WSJT-X ndepth=2.
func DefaultDecodeParams() DecodeParams {
	return DecodeParams{
		Depth:     2,
		APWidth:   25.0,
		MaxPasses: 5,
	}
}

// ── DecodeSingle ────────────────────────────────────────────────────────────

// DecodeSingle attempts to decode a single FT8 signal at the given frequency
// and time offset.
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
	twopi := 2.0 * math.Pi
	ndepth := params.Depth
	if ndepth < 1 {
		ndepth = 2
	}

	// ── Step 1: Downsample to baseband (ft8b.f90 line 105) ──────────
	cd0 := ds.Downsample(dd, &newdat, f1)

	// ── Step 2: DT search ±10 samples (ft8b.f90 lines 108–116) ─────
	i0 := int(math.Round((xdt + 0.5) * Fs2)) // ft8b.f90 line 108: i0=nint((xdt+0.5)*fs2)
	var ctwk [32]complex128
	smax := 0.0
	ibest := 0
	for idt := i0 - 10; idt <= i0+10; idt++ {
		sync := Sync8d(cd0, idt, ctwk, 0)
		if sync > smax {
			smax = sync
			ibest = idt
		}
	}

	// ── Step 3: Frequency search ±2.5 Hz (ft8b.f90 lines 119–133) ──
	smax = 0.0
	delfbest := 0.0
	for ifr := -5; ifr <= 5; ifr++ {
		delf := float64(ifr) * 0.5
		dphi := twopi * delf * Dt2
		phi := 0.0
		for i := 0; i < 32; i++ {
			sin, cos := math.Sincos(phi)
			ctwk[i] = complex(cos, sin)
			phi = math.Mod(phi+dphi, twopi)
		}
		sync := Sync8d(cd0, ibest, ctwk, 1)
		if sync > smax {
			smax = sync
			delfbest = delf
		}
	}

	// ── Step 4: Frequency refinement (ft8b.f90 lines 134–137) ───────
	a := [5]float64{-delfbest, 0, 0, 0, 0}
	cd0 = TwkFreq1(cd0, Fs2, a)
	f1 = f1 + delfbest

	// ── Step 5: Re-downsample at refined frequency (ft8b.f90 line 140)
	noNewdat := false
	cd0 = ds.Downsample(dd, &noNewdat, f1)

	// ── Step 6: Final DT search ±4 samples (ft8b.f90 lines 143–152)
	var ss [9]float64
	for idt := -4; idt <= 4; idt++ {
		ss[idt+4] = Sync8d(cd0, ibest+idt, ctwk, 0)
	}
	smax = ss[0]
	imax := 0
	for i := 1; i < 9; i++ {
		if ss[i] > smax {
			smax = ss[i]
			imax = i
		}
	}
	ibest = imax - 4 + ibest
	xdt = float64(ibest-1) * Dt2

	// ── Step 7: Symbol spectra (ft8b.f90 lines 154–161) ─────────────
	cs, s8 := ComputeSymbolSpectra(cd0, ibest)

	// ── Step 8: Hard sync check (ft8b.f90 lines 163–180) ────────────
	nsync := HardSync(&s8)
	if nsync <= 6 {
		return DecodeCandidate{}, false
	}

	// ── Step 9: Soft metrics (ft8b.f90 lines 182–239) ───────────────
	bmeta, bmetb, bmetc, bmetd := ComputeSoftMetrics(&cs)

	var llra, llrb, llrc, llrd [LDPCn]float64
	for i := 0; i < LDPCn; i++ {
		llra[i] = ScaleFac * bmeta[i]
		llrb[i] = ScaleFac * bmetb[i]
		llrc[i] = ScaleFac * bmetc[i]
		llrd[i] = ScaleFac * bmetd[i]
	}

	// apmag = max(|llra|) * 1.01 (ft8b.f90 line 241)
	apmag := 0.0
	for i := 0; i < LDPCn; i++ {
		if v := math.Abs(llra[i]); v > apmag {
			apmag = v
		}
	}
	apmag *= 1.01

	// ── Step 10: Decode passes (ft8b.f90 lines 254–462) ─────────────
	// Compute AP symbols from callsigns (ft8apset.f90)
	apsym := ComputeAPSymbols(params.MyCall, params.DxCall)

	// Pass count: 4 regular + AP passes
	npasses := 4
	if params.APEnabled {
		if params.APCQOnly {
			npasses = 5 // ft8b.f90 line 258
		} else {
			npasses = 6 // nappasses(0)=2 → 4+2=6 for nQSOProgress=0
		}
	}

	for ipass := 1; ipass <= npasses; ipass++ {
		// Select LLR set (ft8b.f90 lines 266–269)
		var llrz [LDPCn]float64
		switch ipass {
		case 1:
			llrz = llra
		case 2:
			llrz = llrb
		case 3:
			llrz = llrc
		case 4:
			llrz = llrd
		default:
			llrz = llra // AP passes use llra (ft8b.f90 line 275)
		}

		var apmask [LDPCn]int8
		iaptype := 0

		// AP injection (ft8b.f90 lines 274–401)
		if ipass > 4 {
			if params.APCQOnly {
				iaptype = 1
			} else {
				// For ncontest=0, nQSOProgress=0: naptypes(0,1:4)=(/1,2,0,0/)
				aptypes := [4]int{1, 2, 0, 0}
				apIdx := ipass - 5
				if apIdx < 4 {
					iaptype = aptypes[apIdx]
				}
				if iaptype == 0 {
					continue
				}
			}

			// Guard: skip iaptype≥2 if mycall is unknown (ft8b.f90 line 296)
			if iaptype >= 2 && apsym[0] > 1 {
				continue
			}
			// Guard: skip iaptype≥3 if dxcall is unknown (ft8b.f90 line 298)
			if iaptype >= 3 && apsym[29] > 1 {
				continue
			}

			// Frequency guard for AP types ≥3 (ft8b.f90 line 293)
			if iaptype >= 3 && params.NfQSO > 0 && math.Abs(f1-params.NfQSO) > params.APWidth {
				continue
			}

			// Apply AP (ncontest=0 path)
			ApplyAP(&llrz, &apmask, iaptype, apsym, apmag)
		}

		// OSD depth control (ft8b.f90 lines 403–412)
		// Fortran: maxosd=2 (default), then sequential if statements:
		//   ndepth=1 → maxosd=-1 (BP only)
		//   ndepth=2 → maxosd=0
		//   ndepth=3 → maxosd stays at 2 (the conditional block is a no-op)
		norder := 2
		maxosd := 2
		if ndepth == 1 {
			maxosd = -1 // BP only
		} else if ndepth == 2 {
			maxosd = 0 // uncoupled BP+OSD
		}
		// ndepth >= 3: maxosd stays at 2 (default)

		// LDPC decode (ft8b.f90 lines 413–418)
		result, ok := Decode174_91(llrz, LDPCk, maxosd, norder, apmask)
		if !ok {
			continue
		}
		if result.NHardErrors < 0 || result.NHardErrors > 36 {
			continue
		}

		// Reject all-zero codeword (ft8b.f90 line 423)
		allZero := true
		for _, b := range result.Codeword {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			continue
		}

		// Extract message bits and validate (i3, n3) (ft8b.f90 lines 424–428)
		var msgBits [77]int8
		copy(msgBits[:], result.Message91[:77])
		c77 := BitsToC77(msgBits)

		// Parse i3 and n3 from bit positions 72–77 (ft8b.f90 lines 425–428)
		n3 := int(c77[71]-'0')<<2 | int(c77[72]-'0')<<1 | int(c77[73]-'0')
		i3 := int(c77[74]-'0')<<2 | int(c77[75]-'0')<<1 | int(c77[76]-'0')
		if i3 > 5 || (i3 == 0 && n3 > 6) {
			continue
		}
		if i3 == 0 && n3 == 2 {
			continue
		}

		// Unpack message (ft8b.f90 line 429)
		msg, unpkOK := Unpack77(c77)
		if !unpkOK {
			continue
		}

		// Generate tones for subtraction/SNR (ft8b.f90 line 432)
		itone := GenFT8Tones(msgBits)

		// SNR estimation (ft8b.f90 lines 438–461)
		// xsnr2 from xbase — but we don't have xbase here (it comes from
		// sync8 sbase in DecodeIterative). Use tone-ratio SNR instead.
		xsig := 0.0
		xnoi := 0.0
		for i := 0; i < NN; i++ {
			xsig += s8[itone[i]][i] * s8[itone[i]][i]
			ios := (itone[i] + 4) % 7
			xnoi += s8[ios][i] * s8[ios][i]
		}
		xsnr := 0.001
		arg := xsig/xnoi - 1.0
		if arg > 0.1 {
			xsnr = arg
		}
		xsnr = 10.0*math.Log10(xsnr) - 27.0

		// Bail out on likely false decode (ft8b.f90 lines 456–459)
		if nsync <= 10 && xsnr < -24.0 {
			return DecodeCandidate{}, false
		}
		if xsnr < -24.0 {
			xsnr = -24.0
		}

		return DecodeCandidate{
			Message:     msg,
			Freq:        f1,
			DT:          xdt,
			SNR:         xsnr,
			NHardErrors: result.NHardErrors,
			Tones:       itone,
			APType:      iaptype,
		}, true
	}

	return DecodeCandidate{}, false
}

// ── DecodeIterative ─────────────────────────────────────────────────────────

// DecodeIterative runs the full FT8 decode pipeline with iterative signal
// subtraction, matching WSJT-X's multi-pass approach.
//
// Port of the decode subroutine in wsjt-wsjtx/lib/ft8_decode.f90 lines 160–239.
func DecodeIterative(audio []float32, params DecodeParams, freqMin, freqMax float64) []DecodeCandidate {
	ndepth := params.Depth
	if ndepth < 1 {
		ndepth = 2
	}
	npass := 3
	if ndepth == 1 {
		npass = 2
	}
	if params.MaxPasses > 0 && params.MaxPasses < npass {
		npass = params.MaxPasses
	}

	// Work on a copy so subtraction doesn't modify the caller's audio.
	dd := make([]float32, len(audio))
	copy(dd, audio)

	var ddArr [NMAX]float32
	copy(ddArr[:], dd)

	nfa := int(freqMin)
	nfb := int(freqMax)

	var results []DecodeCandidate
	seen := make(map[string]bool)
	ndecodes := 0
	prevPassDecodes := 0

	for ipass := 0; ipass < npass; ipass++ {
		// ft8_decode.f90 lines 176–178
		syncmin := 1.3
		if ndepth <= 2 {
			syncmin = 1.6
		}

		// ft8_decode.f90 lines 179–191
		ndeep := ndepth
		if ipass == 0 && ndepth == 3 {
			ndeep = 2 // lighter OSD on first pass
		}

		// Early termination (ft8_decode.f90 lines 185, 189)
		// Pass 2: skip if no decodes at all yet
		if ipass == 1 && ndecodes == 0 {
			break
		}
		// Pass 3: skip if pass 2 added nothing
		if ipass == 2 && prevPassDecodes == 0 {
			break
		}

		// Sync8 candidate search (ft8_decode.f90 lines 193–195)
		maxcand := 600
		candidates, sbase := Sync8(ddArr, NMAX, nfa, nfb, syncmin, 0, maxcand)

		passParams := DecodeParams{
			Depth:     ndeep,
			APEnabled: params.APEnabled,
			APCQOnly:  params.APCQOnly,
			APWidth:   params.APWidth,
			MyCall:    params.MyCall,
			DxCall:    params.DxCall,
			NfQSO:     params.NfQSO,
		}

		passDecodes := 0
		for _, cand := range candidates {
			ds := NewDownsampler()
			newdat := true
			result, ok := DecodeSingle(dd, ds, cand.Freq, cand.DT, newdat, passParams)
			if !ok {
				continue
			}

			msg := strings.TrimSpace(result.Message)
			if seen[msg] {
				continue
			}

			// Compute xbase-calibrated SNR (ft8_decode.f90 line 201, ft8b.f90 lines 449-454)
			freqBin := int(math.Round(result.Freq / 3.125))
			if freqBin >= 0 && freqBin < NH1 {
				xbase := math.Pow(10.0, 0.1*(sbase[freqBin]-40.0))
				if xbase > 0 {
					// Recompute SNR using xbase (ft8b.f90 lines 449-454, nagain=false path)
					// xsnr2 = 10*log10(xsig/xbase/3e6 - 1) - 27
					// We approximate xsig from the tone-ratio SNR:
					// tone-ratio arg = xsig/xnoi - 1, so xsig ≈ (arg+1)*xnoi
					// But we don't have xsig/xnoi separately here.
					// For now, use xbase for a rough absolute SNR estimate.
					_ = xbase // TODO: full xbase SNR requires s8 from DecodeSingle
				}
			}

			// Adjust DT for display (ft8_decode.f90 line 210: xdt=xdt-0.5)
			result.DT -= 0.5

			seen[msg] = true
			ndecodes++
			passDecodes++
			results = append(results, result)

			// Subtract decoded signal (ft8_decode.f90 line ~207 via ft8b line 435)
			// Use unadjusted DT for subtraction (result.DT has been adjusted, add 0.5 back)
			SubtractFT8(dd, result.Tones, result.Freq, result.DT+0.5)
			copy(ddArr[:], dd)
		}

		prevPassDecodes = passDecodes
	}

	return results
}

// ── Sync8FindCandidates ─────────────────────────────────────────────────────

// Sync8FindCandidates searches for potential FT8 signals using the
// spectrogram-based sync8 algorithm.
//
// Wrapper around the research Sync8() function.
func Sync8FindCandidates(audio []float32, freqMin, freqMax int, syncmin float64, nfqso, maxcand int) []CandidateFreq {
	var dd [NMAX]float32
	n := len(audio)
	if n > NMAX {
		n = NMAX
	}
	copy(dd[:n], audio[:n])
	cands, _ := Sync8(dd, n, freqMin, freqMax, syncmin, nfqso, maxcand)
	return cands
}
