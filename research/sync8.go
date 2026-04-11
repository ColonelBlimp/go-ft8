// sync8.go — Research port of WSJT-X's sync8 subroutine.
//
// Port of subroutine sync8 from wsjt-wsjtx/lib/ft8/sync8.f90.
//
// The pipeline is scaffolded as separate functions matching the Fortran
// steps.  Each step is initially a do-nothing stub; implement from top
// to bottom.

package research

import (
	"math"
	"sort"
)

// ── Constants ────────────────────────────────────────────────────────────

const (
	jz         = 62   // max sync lag ±2.5s  (sync8.f90 line 7: JZ=62)
	maxPreCand = 1000 // pre-filter cap       (sync8.f90 line 4: MAXPRECAND=1000)
)

// ── Types ────────────────────────────────────────────────────────────────

// Candidate holds a single sync8 candidate detection result.
type Candidate struct {
	Freq      float64 // Hz
	DT        float64 // seconds (relative to nominal 0.5 s TX start)
	SyncPower float64 // normalized sync metric
}

// Spectrogram holds the power spectrogram s(freq, time) and its
// derived quantities, matching the Fortran local variables.
//
// Indices are 1-based to match Fortran: s[i][j] where
//
//	i = 1..NH1   (frequency bin, df = 3.125 Hz)
//	j = 1..NHSYM (time step, tstep = 0.04 s)
//
// Index 0 is allocated but unused.
type Spectrogram struct {
	S    [][]float64 // power: s[freq][time], 1-indexed, padded in freq
	Savg []float64   // average spectrum across all time steps, 1-indexed
}

// ── Sync8 — top-level entry point ────────────────────────────────────────

// Sync8 performs spectrogram-based candidate detection using the
// Costas-array sync pattern.
//
// Port of subroutine sync8(dd,npts,nfa,nfb,syncmin,nfqso,maxcand,candidate,ncand,sbase)
// from wsjt-wsjtx/lib/ft8/sync8.f90.
func Sync8(dd [NMAX]float32, npts int, nfa, nfb int, syncmin float64, nfqso int, maxcand int) (candidates []Candidate, sbase [NH1]float64) {

	// ── Derived constants (sync8.f90 lines 29–31) ────────────────────
	tstep := float64(NSTEP) / Fs // 0.04 s per spectrogram step
	df := Fs / float64(NFFT1)    // 3.125 Hz per frequency bin
	nssy := NSPS / NSTEP         // 4: spectrogram steps per symbol
	nfos := NFFT1 / NSPS         // 2: frequency oversampling factor
	jstrt := int(0.5 / tstep)    // 12: 0.5 s offset in steps (Fortran truncation of 12.5)

	// ── Step 1: Compute spectrogram (sync8.f90 lines 28–43) ──────────
	// *** START HERE ***
	spec := computeSpectrogram(dd[:], npts)

	// ── Step 1b: Spectrum baseline (sync8.f90 line 44) ───────────────
	sbase = getSpectrumBaseline(dd[:], nfa, nfb)

	// ── Step 2: 2D Costas correlation (sync8.f90 lines 53–84) ────────
	sync2d := computeSync2D(spec, nfa, nfb, df, nssy, nfos, jstrt)

	// ── Step 3: Peak finding (sync8.f90 lines 86–98) ─────────────────
	jpeak, red, jpeak2, red2 := findPeaks(sync2d, nfa, nfb, df)

	// ── Step 4: 40th-percentile normalization (sync8.f90 lines 99–116)
	normalizeByPercentile(red, red2, nfa, nfb, df)

	// ── Step 5: Extract pre-candidates (sync8.f90 lines 117–134) ─────
	preCands := extractPreCandidates(red, red2, jpeak, jpeak2, nfa, nfb, df, tstep, syncmin)

	// ── Step 6: Near-dupe suppression (sync8.f90 lines 137–149) ──────
	suppressDuplicates(preCands)

	// ── Step 7: Sort + QSO prioritization (sync8.f90 lines 153–174) ──
	candidates = finalSort(preCands, syncmin, nfqso, maxcand)

	return candidates, sbase
}

// ── Step 1: Compute spectrogram ──────────────────────────────────────────
//
// sync8.f90 lines 28–43:
//
//	fac=1.0/300.0
//	do j=1,NHSYM
//	   ia=(j-1)*NSTEP + 1
//	   ib=ia+NSPS-1
//	   x(1:NSPS)=fac*dd(ia:ib)
//	   x(NSPS+1:)=0.
//	   call four2a(x,NFFT1,1,-1,0)           !r2c FFT
//	   do i=1,NH1
//	      s(i,j)=real(cx(i))**2 + aimag(cx(i))**2
//	   enddo
//	   savg=savg + s(1:NH1,j)
//	enddo
//
// For each of NHSYM=372 time steps:
//  1. Extract NSPS=1920 samples from dd, scale by fac=1/300
//  2. Zero-pad to NFFT1=3840 points
//  3. Real-to-complex FFT → NH1=1920 complex bins
//  4. Store power s(i,j) = re² + im²
//  5. Accumulate average spectrum savg
func computeSpectrogram(dd []float32, npts int) *Spectrogram {
	const fac float32 = 1.0 / 300.0 // sync8.f90 line 32

	// Allocate s[0..NH1+nfos*6][0..NHSYM], 1-indexed (index 0 unused).
	// Frequency padding of nfos*6 = 12 bins handles Costas tone offsets
	// in the correlation step.
	// Single contiguous backing array: 1 allocation instead of ~1934,
	// and adjacent freq rows are physically adjacent in memory.
	nfos := NFFT1 / NSPS // 2
	nh1Pad := NH1 + nfos*6 + 1
	cols := NHSYM + 1
	backing := make([]float64, (nh1Pad+1)*cols)
	s := make([][]float64, nh1Pad+1)
	for i := range s {
		s[i] = backing[i*cols : (i+1)*cols]
	}
	savg := make([]float64, NH1+1)

	buf := make([]float32, NSPS) // reusable FFT input buffer

	for j := 1; j <= NHSYM; j++ {
		ia := (j - 1) * NSTEP // 0-based start index in dd
		// x(1:NSPS) = fac * dd(ia:ib)
		for k := 0; k < NSPS; k++ {
			idx := ia + k
			if idx < npts {
				buf[k] = fac * dd[idx]
			} else {
				buf[k] = 0
			}
		}
		// x(NSPS+1:) = 0   — handled by RealFFT zero-padding to NFFT1
		// call four2a(x,NFFT1,1,-1,0)   — r2c FFT
		cx := RealFFT(buf, NFFT1) // optimized: N/2-point FFT + unpack (~2× faster)

		for i := 1; i <= NH1 && i < len(cx); i++ {
			re := real(cx[i])
			im := imag(cx[i])
			pow := re*re + im*im
			s[i][j] = pow
			savg[i] += pow
		}
	}

	return &Spectrogram{S: s, Savg: savg}
}

// ComputeSpectrogramForTest exposes computeSpectrogram for testing.
func ComputeSpectrogramForTest(dd []float32, npts int) *Spectrogram {
	return computeSpectrogram(dd, npts)
}

// ComputeSync2DForTest exposes computeSync2D for testing.
func ComputeSync2DForTest(spec *Spectrogram, nfa, nfb int, df float64, nssy, nfos, jstrt int) [][]float64 {
	return computeSync2D(spec, nfa, nfb, df, nssy, nfos, jstrt)
}

// ── Step 1b: Spectrum baseline ───────────────────────────────────────────
//
// sync8.f90 line 44:
//
//	call get_spectrum_baseline(dd,nfa,nfb,sbase)
//
// Computes noise floor per frequency bin.  Used by ft8b for xsnr2, not
// by the candidate detection itself.  Stub for now.
func getSpectrumBaseline(dd []float32, nfa, nfb int) [NH1]float64 {
	// TODO: port get_spectrum_baseline.f90
	var sbase [NH1]float64
	return sbase
}

// ── Step 2: 2D Costas correlation ────────────────────────────────────────
//
// sync8.f90 lines 53–84:
//
// For each freq bin i in [ia..ib] and lag j in [-JZ..+JZ]:
//
//	Correlate spectrogram against three Costas arrays (a, b, c)
//	Compute ratio-metric sync: signal / noise
//	Store sync2d[i][j+jz] = max(sync_abc, sync_bc)
//
// Returns sync2d[0..NH1+pad][0..2*jz], offset by +jz in second index.
func computeSync2D(spec *Spectrogram, nfa, nfb int, df float64, nssy, nfos, jstrt int) [][]float64 {
	s := spec.S

	// sync8.f90 lines 46–47: frequency bin bounds
	iaFreq := int(math.Round(float64(nfa) / df)) // nint(nfa/df)
	if iaFreq < 1 {
		iaFreq = 1
	}
	ibFreq := int(math.Round(float64(nfb) / df)) // nint(nfb/df)
	// Clamp ibFreq so i+nfos*6 stays within padded s dimension.
	if ibFreq+nfos*6 >= len(s) {
		ibFreq = len(s) - nfos*6 - 1
	}
	if ibFreq < iaFreq {
		return nil
	}

	// Allocate sync2d[0..nh1Pad][0..2*jz].
	// Second index: Fortran j ∈ [-jz,+jz] maps to Go index j+jz ∈ [0,2*jz].
	// Use contiguous backing array (same optimization as spectrogram).
	nh1Pad := NH1 + nfos*6 + 1
	lagCols := 2*jz + 1
	backing := make([]float64, (nh1Pad+1)*lagCols)
	sync2d := make([][]float64, nh1Pad+1)
	for i := range sync2d {
		sync2d[i] = backing[i*lagCols : (i+1)*lagCols]
	}

	// sync8.f90 lines 54–85: double loop over freq bins and time lags.
	for i := iaFreq; i <= ibFreq; i++ {
		for j := -jz; j <= jz; j++ {
			var ta, tb, tc float64
			var t0a, t0b, t0c float64

			for n := 0; n <= 6; n++ {
				// sync8.f90 line 63: m = j + jstrt + nssy*n
				m := j + jstrt + nssy*n

				// ── Array a: first Costas (symbols 0–6) ──────────
				// sync8.f90 lines 64–67
				if m >= 1 && m <= NHSYM {
					ta += s[i+nfos*Icos7[n]][m]
					// sum(s(i:i+nfos*6:nfos, m)) = s[i][m] + s[i+2][m] + ... + s[i+12][m]
					for k := 0; k <= 6; k++ {
						t0a += s[i+nfos*k][m]
					}
				}

				// ── Array b: second Costas (symbols 36–42) ───────
				// sync8.f90 lines 68–69 (no bounds check in Fortran)
				mb := m + nssy*36
				if mb >= 1 && mb <= NHSYM {
					tb += s[i+nfos*Icos7[n]][mb]
					for k := 0; k <= 6; k++ {
						t0b += s[i+nfos*k][mb]
					}
				}

				// ── Array c: third Costas (symbols 72–78) ────────
				// sync8.f90 lines 70–73
				mc := m + nssy*72
				if mc >= 1 && mc <= NHSYM {
					tc += s[i+nfos*Icos7[n]][mc]
					for k := 0; k <= 6; k++ {
						t0c += s[i+nfos*k][mc]
					}
				}
			}

			// sync8.f90 lines 75–78: ratio-metric sync for all three arrays
			t := ta + tb + tc
			t0 := t0a + t0b + t0c
			t0 = (t0 - t) / 6.0
			syncABC := 0.0
			if t0 > 0 {
				syncABC = t / t0
			}

			// sync8.f90 lines 79–82: ratio-metric sync for b+c only
			// (helps late-arriving signals where array a is clipped)
			t = tb + tc
			t0 = t0b + t0c
			t0 = (t0 - t) / 6.0
			syncBC := 0.0
			if t0 > 0 {
				syncBC = t / t0
			}

			// sync8.f90 line 83: sync2d(i,j) = max(sync_abc, sync_bc)
			if syncBC > syncABC {
				sync2d[i][j+jz] = syncBC
			} else {
				sync2d[i][j+jz] = syncABC
			}
		}
	}

	return sync2d
}

// ── Step 3: Peak finding ─────────────────────────────────────────────────
//
// sync8.f90 lines 86–98:
//
// For each freq bin i in [ia..ib]:
//
//	jpeak[i]  = lag of max sync2d within ±10 (narrow search)
//	red[i]    = sync2d at jpeak[i]
//	jpeak2[i] = lag of max sync2d within ±JZ (wide search)
//	red2[i]   = sync2d at jpeak2[i]
func findPeaks(sync2d [][]float64, nfa, nfb int, df float64) (jpeak []int, red []float64, jpeak2 []int, red2 []float64) {
	// TODO: port peak finding
	jpeak = make([]int, NH1+1)
	red = make([]float64, NH1+1)
	jpeak2 = make([]int, NH1+1)
	red2 = make([]float64, NH1+1)
	return
}

// ── Step 4: 40th-percentile normalization ────────────────────────────────
//
// sync8.f90 lines 99–116:
//
// Sort red and red2, find the 40th percentile value as baseline,
// divide all values by it.  This normalizes so that syncmin thresholds
// are relative to the noise floor.
func normalizeByPercentile(red, red2 []float64, nfa, nfb int, df float64) {
	// TODO: port percentile normalization
}

// ── Step 5: Extract pre-candidates ───────────────────────────────────────
//
// sync8.f90 lines 117–134:
//
// Walk frequency bins in descending sync order.
// For each bin where red[n] >= syncmin: emit candidate from narrow peak.
// If wide peak differs from narrow: emit second candidate from wide peak.
// Up to MAXPRECAND=1000 pre-candidates.
func extractPreCandidates(red, red2 []float64, jpeak, jpeak2 []int, nfa, nfb int, df, tstep, syncmin float64) []Candidate {
	// TODO: port pre-candidate extraction
	return nil
}

// ── Step 6: Near-dupe suppression ────────────────────────────────────────
//
// sync8.f90 lines 137–149:
//
// For any two candidates within 4 Hz and 0.04 s, zero out the weaker one.
func suppressDuplicates(cands []Candidate) {
	// TODO: port near-dupe suppression
}

// ── Step 7: Sort + QSO-frequency prioritization ─────────────────────────
//
// sync8.f90 lines 153–174:
//
// 1. Place candidates within ±10 Hz of nfqso at the top.
// 2. Append remaining in descending sync power order.
// 3. Cap at maxcand.
func finalSort(cands []Candidate, syncmin float64, nfqso, maxcand int) []Candidate {
	// TODO: port final sort + prioritization
	_ = sort.Slice // ensure import is used
	return nil
}
