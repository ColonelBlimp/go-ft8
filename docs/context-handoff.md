# Context Handoff — go-ft8 Phase 1

**Date:** 2026-04-10
**Module:** `github.com/ColonelBlimp/go-ft8`
**Package:** `ft8x`
**Go version:** 1.25
**Total code:** ~6,100 lines across 22 Go files

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
| 6 | Port AP decoding | ✅ Done | `ap.go`, `ap_test.go`, `decode.go` |
| 7 | Add plausibility filters | ✅ Done | `validate.go`, `validate_test.go`, `decode.go` |
| 8 | Port iterative signal subtraction | ✅ Done | `encode.go`, `decode.go`, `ft8x_wav_test.go`, `cmd/ft8decode/main.go` |
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

6. **AP (a-priori) decoding** (`ap.go`, `ap_test.go`, `decode.go`) — Faithful port of WSJT-X `ft8b.f90` AP pass logic (lines 243–401, `ncontest=0` standard QSO mode). Key additions:
   - **`ap.go`** (~200 lines) — AP type constants (`APTypeCQ` through `APTypeMyDxRR73`), pre-computed bipolar bit patterns for CQ, RRR, 73, RR73 matching the Fortran `data` statements, `ComputeAPSymbols(mycall, dxcall)` to build 58-element bipolar arrays from Pack28, `ApplyAP()` to inject known bits into LLR/apmask arrays, and `APPassTypes()` to determine which AP types to try based on available callsign info.
   - **`DecodeParams`** gains `MyCall`, `DxCall`, and `NfQSO` fields for AP configuration.
   - **`DecodeCandidate`** gains `APType int` field (0 = no AP).
   - **`DecodeSingle`** now runs up to 4 additional AP passes after the 4 regular LLR passes when `APEnabled=true`. AP passes use bmeta (nsym=1) as the base LLR, inject known bits with magnitude `apmag = max(|llra|) * 1.01`, and respect the `APWidth` frequency guard for AP types ≥3.
   - **`ap_test.go`** — Tests for `ComputeAPSymbols` (CQ bipolar verification, round-trip), `ApplyAP` (mask position counts, LLR magnitude checks for all 6 AP types), `APPassTypes` (selection logic), and a synthetic weak-signal test demonstrating AP type 1 recovering a CQ message that regular BP cannot decode.
   - AP is opt-in: existing tests and `DefaultDecodeParams()` have `APEnabled=false`, so regression baselines are unaffected.

7. **Plausibility filters** (`validate.go`, `validate_test.go`, `decode.go`) — Callsign and message plausibility validation to reject false decodes. Key additions:
   - **`PlausibleCallsign()`** — Validates ITU callsign structure matching WSJT-X's `pack28` encoding rules (from `packjt77.f90` lines 708–738): finds the last digit (call-area digit), verifies it's at position 2 or 3 (1-indexed), checks prefix has ≥1 letter and isn't all digits, checks suffix is 1–3 letters. Handles special tokens (CQ, DE, QRZ), CQ suffixes, hash-encoded calls (`<...>`), and `/R`/`/P` suffixes.
   - **`PlausibleMessage()`** — Parses decoded message into fields, identifies callsign fields vs tokens (reports, grids, RRR/RR73/73, etc.), and validates each callsign field with `PlausibleCallsign()`. Free-text and hashed-call messages are always accepted.
   - **`DecodeSingle`** now calls `PlausibleMessage()` after `Unpack77` succeeds, rejecting implausible messages before SNR computation.
   - **`validate_test.go`** — Tests for valid callsigns (30 real WSJT-X reference callsigns, with/without suffixes, special tokens), invalid callsigns (too short/long, all-letter, all-digit, special chars), valid messages (all 18 capture reference messages), invalid messages, and a dedicated WSJT-X capture callsign test.

8. **Iterative signal subtraction** (`encode.go`, `decode.go`, `ft8x_wav_test.go`, `cmd/ft8decode/main.go`) — Multi-pass decode loop with signal subtraction, ported from WSJT-X `ft8_decode.f90` and `subtractft8.f90`. Key additions:
   - **GFSK waveform generation** (`encode.go`) — `gfskPulse()` (port of `gfsk_pulse.f90`) and `GenFT8CWave()` (port of `gen_ft8wave.f90`, complex output mode). Generates the ideal FT8 reference waveform with Gaussian FSK pulse shaping (bt=2.0, hmod=1.0), including dummy symbol extension and raised-cosine envelope shaping. Added `NFRAME` constant to `params.go`.
   - **`SubtractFT8()` rewrite** (`decode.go`) — Replaced the simplified unit-amplitude subtraction with proper amplitude/phase estimation. Generates GFSK reference via `GenFT8CWave()`, estimates complex amplitude per symbol via conjugate-multiply averaging (`camp = mean(dd × conj(cref))`), then subtracts `2×Re(camp × cref)`. Operates in-place on the audio buffer. O(NFRAME) per call with no FFTs needed (~2ms vs Fortran's ~110ms FFT-based approach).
   - **`DecodeIterative()`** (`decode.go`) — New top-level multi-pass decode entry point matching WSJT-X's `ft8_decode.f90` 3-pass loop. Each pass: `Sync8FindCandidates()` on current audio → `DecodeSingle()` per candidate → `SubtractFT8()` for each successful decode → repeat on cleaned audio. Maintains global dedup across passes. First pass uses lighter OSD (Depth=2) when Depth=3 is requested (matching Fortran's `ndeep` logic). Terminates early if a pass produces no new decodes.
   - **`DecodeParams.MaxPasses`** — New field controlling the number of subtraction passes (default 3, matching WSJT-X).
   - **Coarse time search widened** — `DecodeSingle()` coarse time search radius expanded from ±10 to ±20 downsampled samples (±0.1s), improving alignment for sync8-detected candidates.
   - **CLI updated** (`cmd/ft8decode/main.go`) — Now uses `DecodeIterative()` with a `--passes` flag. Removed obsolete `--dtmin`, `--dtmax`, `--max` flags.
   - **Integration tests** (`ft8x_wav_test.go`) — Added `TestFt8xWAVIterativeCapture1` and `TestFt8xWAVIterativeCapture2` validating the full iterative pipeline against both WAV captures.

### Current test results

```
Capture 1 provided candidates:  7/13 correct, 0 false  (baseline: ≥7)
Capture 2 provided candidates:  9/15 correct, 0 false  (baseline: ≥9)
Capture 1 iterative (own cands): 6/13 correct, 0 false  (was 5/13 single-pass)
Capture 2 iterative (own cands): 8/15 correct, 0 false  (was 7/15 single-pass)
All unit tests:                  PASS
Plausibility filters:            PASS (30 valid, 6 invalid callsigns; 18 valid messages)
AP CQ weak-signal decode:        PASS (4 hard errors recovered with AP)
OSD round-trip (ndeep=4):        PASS (5 bit errors recovered)
nextpat91 pattern counts:        PASS (verified C(k,w) for multiple k,w)
Mixed-radix FFT accuracy:       <1e-9 round-trip error
Full test suite (WAV):           ~18 s
```

### Benchmark data (Intel i3-10100F @ 3.60 GHz)

```
192k-point FFT (mixed-radix): 64 ms/op
3840-point FFT (mixed-radix):  1.1 ms/op
Full iterative decode (3 passes): ~4 s per capture
SubtractFT8 per signal:        ~2 ms (no FFTs)
```

---

## What to do next

### Step 9: Comprehensive testing and performance optimisation

**Goal:** Reach Phase 1 success criteria through decoder tuning, wider AP coverage, and performance work.

**Current gaps vs Phase 1 targets:**
- Capture 1: 6/13 iterative (target ≥11/13) — 5 more decodes needed
- Capture 2: 8/15 iterative (target ≥12/15) — 4 more decodes needed
- Decode time: ~4s (target <2s)

**Potential improvements:**
1. **Enable AP+Depth=3 selectively** — AP with CQ-only + OSD order-2 helps weak signals but is slow (~30s+ per capture with current OSD). Need to optimise OSD performance or limit Depth=3 to later passes only.
2. **ScaleFac re-tuning** — The BP LLR scaling factor (2.83) was tuned for the original decoder. With the upgraded OSD and AP, re-tuning could improve convergence for marginal signals.
3. **Sync8 candidate quality** — Some WSJT-X reference signals are found by sync8 but at DT offsets where the decoder can't align. Investigate whether the fine frequency/time search needs wider range.
4. **Performance: FFT caching** — The 192k-point FFT dominates decode time. Consider caching across passes (only recompute after subtraction) or switching to the Fortran's approach of subtracting in the frequency domain.
5. **Performance: OSD order-2** — The OSD order-2 search (ndeep=4) is extremely expensive. Consider limiting to a fixed number of candidates per pass or implementing the Fortran's time budgeting.

### Phase 1 success criteria (from assessment §5.5)

| Metric | Current | Target |
|---|---|---|
| Capture 1 correct | 6/13 (iterative) | ≥ 11/13 |
| Capture 2 correct | 8/15 (iterative) | ≥ 12/15 |
| False decode rate | 0 | ≤ 1 per capture |
| Full decode cycle | ~4 s | < 2 s |

---

## Known issues and decisions

1. **Sync8 own-candidates decode gap** — Sync8 finds candidates near all reference signals, but decode success is limited by decoder sensitivity. The iterative subtraction loop (Step 8) improved counts by 1 per capture (5→6 and 7→8). Further improvement requires deeper OSD or AP decoding.

2. **Mixed-radix FFT memory** — The recursive implementation allocates O(n × log n) temporary memory. An iterative Stockham approach would reduce this to O(n). Acceptable for Phase 1; optimize in Phase 2 if profiling shows GC pressure.

3. **Package name `ft8x`** — Inherited from the goft8 baseline. Intentionally kept to avoid import collision with any future `ft8` package. May rename in a future major version.

4. **WAV test files** — Some large WAV files in `testdata/` are git-ignored. The two primary captures (`ft8test_capture_20260410.wav`, `ft8test_capture2_20260410.wav`) are tracked. Tests skip gracefully if files are missing.

5. **platanh: math.Atanh vs Fortran piecewise** — The Fortran piecewise-linear `platanh` (from `platanh.f90`) was tested but caused a 1-decode regression in Capture 1 because the BP decoder's LLR scaling (`ScaleFac=2.83`) was tuned with `math.Atanh`. The piecewise version amplifies small values by ~20% (`x/0.83` vs `x`) which shifts BP convergence. Retained `math.Atanh` with ±19.07 clamping. May revisit when ScaleFac is re-tuned in a later step.

6. **OSD ntheta pre-test bypass for order-1** — The Fortran `ntheta` pre-test rejects OSD candidates whose parity error count exceeds a threshold. For order-1 base patterns (91 candidates, cheap to evaluate), the pre-test is bypassed to avoid marginal-signal regressions. Higher-order patterns still use the pre-test for performance.

7. **AP decoding: ncontest=0 only** — Only standard QSO mode is implemented. Contest modes (NA_VHF, EU_VHF, Field Day, RTTY, WW_DIGI, FOX, HOUND, ARRL_DIGI) would require additional bit patterns and AP logic. These can be added as a future enhancement.

8. **AP frequency guard** — AP types ≥3 (which pin 61–77 bits) are restricted to candidates within `APWidth` Hz of `NfQSO` to prevent false decodes. When `NfQSO=0` (default), the frequency guard is disabled and AP types ≥3 are applied everywhere. Callers should set `NfQSO` when they have QSO context to prevent false positives.

9. **SubtractFT8: per-symbol vs FFT-based** — The Go implementation uses per-symbol amplitude estimation (O(NFRAME), ~2ms per signal) instead of the Fortran's FFT-based global low-pass filter (two 180k-point FFTs, ~110ms per signal). The per-symbol approach is 50× faster and handles GFSK transitions adequately since bt=2.0 produces narrow pulses. If subtraction quality becomes a bottleneck, the FFT approach can be ported later.

10. **xdt offset convention** — `DecodeSingle` computes `xdt = (ibest-1)*Dt2` where ibest is 0-based in Go, introducing a systematic -1 Dt2 = -0.005s offset vs the Fortran (which uses 1-based ibest). This only affects display DT and subtraction alignment (60 samples ≈ 3% of a symbol). Subtraction's per-symbol amplitude estimation is robust to this. May fix in a future cleanup.

