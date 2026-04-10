# Context Handoff — go-ft8 Phase 1

**Date:** 2026-04-10
**Module:** `github.com/ColonelBlimp/go-ft8`
**Package:** `ft8x`
**Go version:** 1.25
**Total code:** ~5,100 lines across 18 Go files

---

## What has been done

### Phase 1 steps completed (from `docs/ft8-library-assessment.md` §5.2)

| # | Step | Status | Files |
|---|---|---|---|
| 1 | Copy ft8x files as baseline | ✅ Done | All `*.go` at root |
| 2 | Replace logging/errors (stdlib) | ✅ Done (was already stdlib) | — |
| 3 | Port sync8 candidate detection | ✅ Done | `sync8.go`, `sync8_test.go` |
| 4 | Port mixed-radix FFT | ✅ Done | `fft_mixedradix.go`, `fft_mixedradix_test.go`, `fft.go` updated |
| 5 | Upgrade OSD to order-2 with zsave | ✅ Done | `ldpc.go`, `decode.go`, `ft8_test.go` |
| 6 | Port AP decoding | ❌ Not started | New `ap.go` + `decode.go` changes |
| 7 | Add plausibility filters | ❌ Not started | New `validate.go` |
| 8 | Port iterative signal subtraction | ❌ Not started | `decode.go` changes |
| 9 | Comprehensive testing | ❌ Not started | Test updates |

### Detailed changelog

1. **Baseline reset** — Removed old modular layout (`codec/`, `dsp/`, `message/`, `synth/`, `timing/`). Copied all 13 Go files from `goft8/ft8/` into the repo root. Updated `ft8x_wav_test.go` paths from `../ft8/dsp/testdata/` to `testdata/`. Preserved WAV captures in `testdata/`. Cleaned `go.mod` to zero dependencies.

2. **Sync8 candidate detection** (`sync8.go`, 320 lines) — Faithful port of WSJT-X `sync8.f90`. Computes a spectrogram (372 × 1920 time-freq grid via 3840-point FFTs), correlates with the Costas sync pattern using ratio-metric scoring, applies 40th-percentile normalisation, near-dupe suppression, and QSO-frequency prioritisation. `FindCandidates()` in `decode.go` now calls `Sync8FindCandidates()`. Added `SyncPower` field to `CandidateFreq`. Added sync8 spectrogram constants to `params.go` (`NFFT1`, `NH1`, `NSTEP`, `NHSYM`).

3. **Mixed-radix FFT** (`fft_mixedradix.go`, 198 lines) — Recursive Cooley-Tukey with explicit radix-2, radix-3, and radix-5 butterfly routines (`mrButterfly2`, `mrButterfly3`, `mrButterfly5`). Updated `fft.go` to route 5-smooth sizes through `fftMixedRadix()` instead of Bluestein. All three key FT8 sizes (3840, 3200, 192000) are 5-smooth and now use this path.

4. **CLI** (`cmd/ft8decode/main.go`, 97 lines) — Minimal CLI that loads a WAV file, runs `FindCandidates` + `Decode`, and prints results.

5. **OSD order-2 with zsave chain** (`ldpc.go`, `decode.go`) — Faithful port of WSJT-X `osd174_91.f90`. `OSDDecode()` now accepts a `ndeep` parameter (0–6) controlling search depth, matching the Fortran's `ndeep` table: `ndeep=1` → order-1; `ndeep=2` → order-1 with npre1 pre-test; `ndeep=3` → order-1 with npre1 + npre2 hash pair-flip; `ndeep=4` → order-2 with pre-processing; etc. Key additions:
   - **`nextpat91()`** — Port of the Fortran combinatorial pattern generator for weight-N error patterns among k=91 positions.
   - **`ntheta` parity pre-test** — Fast rejection of unlikely candidates using the first `nt` parity syndrome bits, with a bypass for order-1 base patterns to avoid marginal-signal regressions.
   - **Hash-based pair-flip search** (`npre2`) — For `ndeep≥3`, builds a hash table mapping `ntau`-bit parity syndromes to `(i1, i2)` bit-flip pairs, then looks up matching pair-flips for each base pattern. Uses Go `map[int][]osdPairEntry` instead of Fortran linked-list common block.
   - **`Decode174_91`** signature updated: `norder` → `ndeep` parameter.
   - **`DecodeSingle`** maps `Depth=2` → `ndeep=2` (order-1 + pre-test), `Depth=3` → `ndeep=4` (order-2 + pair-flip).
   - **`platanh`** retained as `math.Atanh` with clamping (not the Fortran piecewise-linear version), since the BP scaling was tuned with it.
   - Added `TestNextpat91`, `TestOSDRoundTrip`, and `TestPlatanh` unit tests.
   - The zsave chain (cumulative posterior LLR snapshots at BP iterations 1–3) was already structurally correct; now feeds into the upgraded OSD.

### Current test results

```
Capture 1 provided candidates:  7/13 correct, 0 false  (baseline: ≥7)
Capture 2 provided candidates:  9/15 correct, 0 false  (baseline: ≥9)
Capture 1 sync8 own candidates: 5/13 correct, 0 false
Capture 2 sync8 own candidates: 7/15 correct, 0 false
All unit tests:                  PASS
OSD round-trip (ndeep=4):        PASS (5 bit errors recovered)
nextpat91 pattern counts:        PASS (verified C(k,w) for multiple k,w)
Mixed-radix FFT accuracy:       <1e-9 round-trip error
Full test suite:                 27 s
```

### Benchmark data (Intel i3-10100F @ 3.60 GHz)

```
192k-point FFT (mixed-radix): 64 ms/op
3840-point FFT (mixed-radix):  1.1 ms/op
Full WAV decode (sync8 candidates): ~1.0 s
Full WAV decode (provided candidates): ~4 s
```

---

## What to do next

### Step 6: Port AP decoding

**Goal:** Add a-priori (AP) decoding passes that inject known bits (from CQ, mycall, dxcall) into the LLR stream to decode weaker signals.

**Reference:** WSJT-X `ft8b.f90` AP pass logic, the assessment §2.2.

### Step 7: Add plausibility filters

**Goal:** Reject false decodes by validating callsign structure (ITU format) and message plausibility.

**Reference:** The old `message/validate.go` had `PlausibleCallsign()` and `PlausibleMessage()`.

### Step 8: Port iterative signal subtraction

**Goal:** After each decode pass, subtract decoded signals from audio and re-run candidate detection + decoding. This is how WSJT-X achieves high decode counts.

**Reference:** WSJT-X `ft8_decode.f90` multi-pass loop, `subtractft8.f90`. The existing `SubtractFT8()` in `decode.go` does basic waveform subtraction already.

### Phase 1 success criteria (from assessment §5.5)

| Metric | Current | Target |
|---|---|---|
| Capture 1 correct | 7/13 (provided) | ≥ 11/13 |
| Capture 2 correct | 9/15 (provided) | ≥ 12/15 |
| False decode rate | 0 | ≤ 1 per capture |
| Full decode cycle | ~4 s | < 2 s |

---

## Known issues and decisions

1. **Sync8 own-candidates decode gap** — Sync8 finds candidates near all 13 reference signals, but only 5 decode vs 7 with provided candidates. The gap is caused by the decoder's coarse time search radius (±10 samples = ±0.05s) being too narrow for some sync8 candidate positions. This will improve with signal subtraction (iterative re-processing) and AP decoding.

2. **Mixed-radix FFT memory** — The recursive implementation allocates O(n × log n) temporary memory. An iterative Stockham approach would reduce this to O(n). Acceptable for Phase 1; optimize in Phase 2 if profiling shows GC pressure.

3. **Package name `ft8x`** — Inherited from the goft8 baseline. Intentionally kept to avoid import collision with any future `ft8` package. May rename in a future major version.

4. **WAV test files** — Some large WAV files in `testdata/` are git-ignored. The two primary captures (`ft8test_capture_20260410.wav`, `ft8test_capture2_20260410.wav`) are tracked. Tests skip gracefully if files are missing.

5. **platanh: math.Atanh vs Fortran piecewise** — The Fortran piecewise-linear `platanh` (from `platanh.f90`) was tested but caused a 1-decode regression in Capture 1 because the BP decoder's LLR scaling (`ScaleFac=2.83`) was tuned with `math.Atanh`. The piecewise version amplifies small values by ~20% (`x/0.83` vs `x`) which shifts BP convergence. Retained `math.Atanh` with ±19.07 clamping. May revisit when ScaleFac is re-tuned in a later step.

6. **OSD ntheta pre-test bypass for order-1** — The Fortran `ntheta` pre-test rejects OSD candidates whose parity error count exceeds a threshold. For order-1 base patterns (91 candidates, cheap to evaluate), the pre-test is bypassed to avoid marginal-signal regressions. Higher-order patterns still use the pre-test for performance.
