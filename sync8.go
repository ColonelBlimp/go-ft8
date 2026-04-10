package ft8x

import (
	"math"
	"sort"
)

// Sync8 algorithm constants.
const (
	jzSync     = 62   // max sync lag ±2.5s (62 × 0.04s = 2.48s)
	maxPreCand = 1000 // max pre-filtering candidates (MAXPRECAND in Fortran)
)

// Sync8FindCandidates performs spectrogram-based candidate detection using
// the Costas-array sync pattern. This is a faithful port of WSJT-X's
// sync8.f90.
//
// Parameters:
//   - dd: audio samples at 12000 Hz (up to NMAX=180000 samples)
//   - nfa, nfb: frequency search range in Hz
//   - syncmin: minimum normalized sync power threshold (1.3–2.0 typical)
//   - nfqso: QSO frequency in Hz for candidate prioritization (0 to disable)
//   - maxcand: maximum number of candidates to return
//
// Returns candidates sorted by sync power (best first), with candidates
// near nfqso prioritized at the top of the list.
func Sync8FindCandidates(dd []float32, nfa, nfb int, syncmin float64, nfqso int, maxcand int) []CandidateFreq {
	const (
		nfft1 = NFFT1  // 3840
		nh1   = NH1    // 1920
		nstep = NSTEP  // 480
		nhsym = NHSYM  // 372
		jz    = jzSync // 62
	)

	tstep := float64(nstep) / Fs // 0.04 s per spectrogram step
	df := Fs / float64(nfft1)    // 3.125 Hz per frequency bin
	fac := float32(1.0 / 300.0)  // amplitude scaling (numerical stability)
	nssy := NSPS / nstep         // 4 (spectrogram steps per symbol)
	nfos := nfft1 / NSPS         // 2 (frequency oversampling factor)
	jstrt := int(0.5 / tstep)    // 12 (Fortran integer truncation of 12.5)

	// ── Step 1: Compute spectrogram ──────────────────────────────────────
	// s[i][j]: power at freq bin i (1-indexed), time step j (1-indexed).
	// Padded in frequency for i + nfos*6 = i + 12 offsets.
	nh1Pad := nh1 + nfos*6 + 1
	s := make([][]float64, nh1Pad+1)
	for i := range s {
		s[i] = make([]float64, nhsym+1)
	}

	buf := make([]float32, NSPS) // reusable FFT input buffer
	for j := 1; j <= nhsym; j++ {
		ia := (j - 1) * nstep // 0-based audio start index
		for k := 0; k < NSPS; k++ {
			idx := ia + k
			if idx < len(dd) {
				buf[k] = dd[idx] * fac
			} else {
				buf[k] = 0
			}
		}
		cx := RealFFT(buf, nfft1) // returns [0..nh1] complex values
		for i := 1; i <= nh1 && i < len(cx); i++ {
			r, im := real(cx[i]), imag(cx[i])
			s[i][j] = r*r + im*im
		}
	}

	// ── Step 2: Compute 2D sync correlation ──────────────────────────────
	iaFreq := int(math.Max(1.0, math.Round(float64(nfa)/df)))
	ibFreq := int(math.Round(float64(nfb) / df))
	if ibFreq+nfos*6 > nh1 {
		ibFreq = nh1 - nfos*6
	}
	if ibFreq < iaFreq {
		return nil
	}

	// sync2d[i][j+jz]: ratio-metric sync for freq bin i, lag j ∈ [-jz, +jz].
	sync2d := make([][]float64, nh1Pad+1)
	for i := range sync2d {
		sync2d[i] = make([]float64, 2*jz+1)
	}

	for i := iaFreq; i <= ibFreq; i++ {
		for j := -jz; j <= jz; j++ {
			var ta, tb, tc float64
			var t0a, t0b, t0c float64

			for n := 0; n <= 6; n++ {
				m := j + jstrt + nssy*n

				// Array a (first Costas, symbols 0–6)
				if m >= 1 && m <= nhsym {
					ta += s[i+nfos*Icos7[n]][m]
					for k := 0; k <= 6; k++ {
						t0a += s[i+nfos*k][m]
					}
				}

				// Array b (second Costas, symbols 36–42)
				// Always in bounds: min m+nssy*36 = 94, max = 242, nhsym = 372.
				mb := m + nssy*36
				if mb >= 1 && mb <= nhsym {
					tb += s[i+nfos*Icos7[n]][mb]
					for k := 0; k <= 6; k++ {
						t0b += s[i+nfos*k][mb]
					}
				}

				// Array c (third Costas, symbols 72–78)
				mc := m + nssy*72
				if mc >= 1 && mc <= nhsym {
					tc += s[i+nfos*Icos7[n]][mc]
					for k := 0; k <= 6; k++ {
						t0c += s[i+nfos*k][mc]
					}
				}
			}

			// Ratio-metric sync: signal / noise for all three Costas arrays.
			t := ta + tb + tc
			t0 := t0a + t0b + t0c
			t0 = (t0 - t) / 6.0 // average noise per tone (6 non-signal tones)
			syncABC := 0.0
			if t0 > 0 {
				syncABC = t / t0
			}

			// Also try b+c only (helps late-arriving signals where array a
			// falls off the beginning of the capture).
			t = tb + tc
			t0 = t0b + t0c
			t0 = (t0 - t) / 6.0
			syncBC := 0.0
			if t0 > 0 {
				syncBC = t / t0
			}

			if syncBC > syncABC {
				sync2d[i][j+jz] = syncBC
			} else {
				sync2d[i][j+jz] = syncABC
			}
		}
	}

	// ── Step 3: Peak finding + 40th-percentile normalization ─────────────
	jpeak := make([]int, nh1+1)   // narrow-search peak lag per freq bin
	red := make([]float64, nh1+1) // narrow-search peak sync value
	jpeak2 := make([]int, nh1+1)  // wide-search peak lag
	red2 := make([]float64, nh1+1)

	mlag := 10 // narrow search ±10 lags (±0.4s)
	for i := iaFreq; i <= ibFreq; i++ {
		// Narrow search: ±mlag
		bestJ, bestV := -mlag, sync2d[i][-mlag+jz]
		for lag := -mlag + 1; lag <= mlag; lag++ {
			if v := sync2d[i][lag+jz]; v > bestV {
				bestV = v
				bestJ = lag
			}
		}
		jpeak[i] = bestJ
		red[i] = bestV

		// Wide search: ±jz
		bestJ2, bestV2 := -jz, sync2d[i][0]
		for lag := -jz + 1; lag <= jz; lag++ {
			if v := sync2d[i][lag+jz]; v > bestV2 {
				bestV2 = v
				bestJ2 = lag
			}
		}
		jpeak2[i] = bestJ2
		red2[i] = bestV2
	}

	// Sort and find 40th percentile for normalization.
	iz := ibFreq - iaFreq + 1
	if iz < 1 {
		return nil
	}

	type indexedFreq struct {
		bin int
		val float64
	}

	// Sort red (ascending) by value.
	redSorted := make([]indexedFreq, iz)
	for k := 0; k < iz; k++ {
		redSorted[k] = indexedFreq{bin: iaFreq + k, val: red[iaFreq+k]}
	}
	sort.Slice(redSorted, func(a, b int) bool {
		return redSorted[a].val < redSorted[b].val
	})

	npctile := int(math.Round(0.40 * float64(iz)))
	if npctile < 1 {
		return nil
	}

	base := redSorted[npctile-1].val
	if base <= 0 {
		base = 1e-30
	}
	for i := iaFreq; i <= ibFreq; i++ {
		red[i] /= base
	}

	// Same for red2 (wide search).
	red2Sorted := make([]indexedFreq, iz)
	for k := 0; k < iz; k++ {
		red2Sorted[k] = indexedFreq{bin: iaFreq + k, val: red2[iaFreq+k]}
	}
	sort.Slice(red2Sorted, func(a, b int) bool {
		return red2Sorted[a].val < red2Sorted[b].val
	})
	base2 := red2Sorted[npctile-1].val
	if base2 <= 0 {
		base2 = 1e-30
	}
	for i := iaFreq; i <= ibFreq; i++ {
		red2[i] /= base2
	}

	// ── Step 4: Extract pre-candidates (descending sync power) ───────────
	type preCandidate struct {
		freq      float64
		dt        float64
		syncPower float64
	}
	var preCands []preCandidate

	for idx := iz - 1; idx >= 0 && len(preCands) < maxPreCand; idx-- {
		n := redSorted[idx].bin
		if red[n] >= syncmin && !math.IsNaN(red[n]) {
			preCands = append(preCands, preCandidate{
				freq:      float64(n) * df,
				dt:        (float64(jpeak[n]) - 0.5) * tstep,
				syncPower: red[n],
			})
		}
		if jpeak2[n] == jpeak[n] {
			continue
		}
		if len(preCands) >= maxPreCand {
			break
		}
		if red2[n] >= syncmin && !math.IsNaN(red2[n]) {
			preCands = append(preCands, preCandidate{
				freq:      float64(n) * df,
				dt:        (float64(jpeak2[n]) - 0.5) * tstep,
				syncPower: red2[n],
			})
		}
	}

	// ── Step 5: Near-dupe suppression ────────────────────────────────────
	// If two candidates are within 4 Hz and 0.04s, keep the stronger one.
	for i := range preCands {
		if preCands[i].syncPower <= 0 {
			continue
		}
		for j := 0; j < i; j++ {
			if preCands[j].syncPower <= 0 {
				continue
			}
			fdiff := math.Abs(preCands[i].freq) - math.Abs(preCands[j].freq)
			tdiff := math.Abs(preCands[i].dt - preCands[j].dt)
			if math.Abs(fdiff) < 4.0 && tdiff < 0.04 {
				if preCands[i].syncPower >= preCands[j].syncPower {
					preCands[j].syncPower = 0
				} else {
					preCands[i].syncPower = 0
				}
			}
		}
	}

	// ── Step 6: Final sort + QSO-frequency prioritization ────────────────
	var result []CandidateFreq

	// Place candidates within ±10 Hz of nfqso at the top.
	if nfqso > 0 {
		for i := range preCands {
			if preCands[i].syncPower >= syncmin &&
				math.Abs(preCands[i].freq-float64(nfqso)) <= 10.0 {
				result = append(result, CandidateFreq{
					Freq:      preCands[i].freq,
					DT:        preCands[i].dt,
					SyncPower: preCands[i].syncPower,
				})
				preCands[i].syncPower = 0 // consumed
			}
		}
	}

	// Sort remaining by descending sync power.
	sort.Slice(preCands, func(a, b int) bool {
		return preCands[a].syncPower > preCands[b].syncPower
	})

	for _, pc := range preCands {
		if len(result) >= maxcand {
			break
		}
		if pc.syncPower >= syncmin {
			result = append(result, CandidateFreq{
				Freq:      math.Abs(pc.freq),
				DT:        pc.dt,
				SyncPower: pc.syncPower,
			})
		}
	}

	return result
}
