# Context Handoff — go-ft8 Phase 1

**Date:** 2026-04-11
**Module:** `github.com/ColonelBlimp/go-ft8`
**Package:** `ft8x`
**Go version:** 1.25
**Total code:** ~7,100 lines across 24 Go files (+ ~3,600 lines in `research/`)

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
| 9 | Comprehensive testing | 🔶 In progress | Diagnostic tests, research pipeline, 3rd capture added |

### Detailed changelog

#### Steps 1–8 (completed 2026-04-10, unchanged)

1. **Baseline reset** — Removed old modular layout (`codec/`, `dsp/`, `message/`, `synth/`, `timing/`). Copied all 13 Go files from `goft8/ft8/` into the repo root. Updated `ft8x_wav_test.go` paths from `../ft8/dsp/testdata/` to `testdata/`. Preserved WAV captures in `testdata/`. Cleaned `go.mod` to zero dependencies.

2. **Sync8 candidate detection** (`sync8.go`, 320 lines) — Faithful port of WSJT-X `sync8.f90`. Computes a spectrogram (372 × 1920 time-freq grid via 3840-point FFTs), correlates with the Costas sync pattern using ratio-metric scoring, applies 40th-percentile normalisation, near-dupe suppression, and QSO-frequency prioritisation. `FindCandidates()` in `decode.go` now calls `Sync8FindCandidates()`. Added `SyncPower` field to `CandidateFreq`. Added sync8 spectrogram constants to `params.go` (`NFFT1`, `NH1`, `NSTEP`, `NHSYM`).

3. **Mixed-radix FFT** (`fft_mixedradix.go`, 198 lines) — Recursive Cooley-Tukey with explicit radix-2, radix-3, and radix-5 butterfly routines (`mrButterfly2`, `mrButterfly3`, `mrButterfly5`). Updated `fft.go` to route 5-smooth sizes through `fftMixedRadix()` instead of Bluestein. All three key FT8 sizes (3840, 3200, 192000) are 5-smooth and now use this path.

4. **CLI** (`cmd/ft8decode/main.go`, 92 lines) — Minimal CLI that loads a WAV file, runs `DecodeIterative`, and prints results. Flags: `-wav`, `-fmin`, `-fmax`, `-passes`.

5. **OSD order-2 with zsave chain** (`ldpc.go`, `decode.go`) — Faithful port of WSJT-X `osd174_91.f90`. `OSDDecode()` accepts a `ndeep` parameter (0–6) controlling search depth. Key additions: `nextpat91()` combinatorial pattern generator, `ntheta` parity pre-test with order-1 bypass, hash-based pair-flip search (`npre2`), and `Decode174_91` signature updated to `ndeep` parameter.

6. **AP (a-priori) decoding** (`ap.go`, `ap_test.go`, `decode.go`) — Faithful port of WSJT-X `ft8b.f90` AP pass logic (lines 243–401, `ncontest=0` standard QSO mode). Six AP types (CQ through MyDxRR73), `ComputeAPSymbols`, `ApplyAP`, `APPassTypes`. AP is opt-in; existing tests and `DefaultDecodeParams()` have `APEnabled=false`.

7. **Plausibility filters** (`validate.go`, `validate_test.go`, `decode.go`) — Callsign and message plausibility validation. `PlausibleCallsign()` validates ITU structure matching WSJT-X `packjt77.f90` rules. `PlausibleMessage()` parses decoded messages and validates each callsign field. `DecodeSingle` calls `PlausibleMessage()` after `Unpack77`.

8. **Iterative signal subtraction** (`encode.go`, `decode.go`, `ft8x_wav_test.go`, `cmd/ft8decode/main.go`) — Multi-pass decode loop with signal subtraction. `GenFT8CWave()` (GFSK waveform), `SubtractFT8()` (per-symbol amplitude estimation), `DecodeIterative()` (multi-pass entry point with sync8 → decode → subtract loop). Default `MaxPasses=5`.

#### Step 9 work (2026-04-11, current session)

9. **Diagnostic and research infrastructure** — Extensive investigation into missing signals and candidate pipeline improvements. Key additions:

   - **Third WAV capture** (`testdata/capture.wav`) — FTdx10 DATA-mode capture with 21 WSJT-X reference decodes. Added `wsjtxCapture3` reference set and full test suite (`TestFt8xWAVCapture3ProvidedCandidates`, `TestFt8xWAVCapture3OwnCandidates`, `TestFt8xWAVIterativeCapture3`). Regression baselines: provided candidates ≥14/21, iterative ≥16/21.

   - **Diagnostic tests** (`diag_test.go`, 228 lines; `diag_cap2_test.go`, 299 lines) — Per-signal diagnostic tracing for all three captures. `TestDiagMissing` tests each missing signal by trying `DecodeSingle` at exact WSJT-X freq/DT to identify the failure stage (sync, LDPC, unpack, plausibility). `diag_cap2_test.go` traces the iterative pipeline for Capture 2, including sync8 2D correlation profiling at specific frequencies.

   - **FFT-based signal subtraction** (`decode.go`, `SubtractFT8FFT`) — Faithful port of WSJT-X `subtractft8.f90`. Generates GFSK reference, conjugate-multiplies, low-pass filters in frequency domain (cos² window → FFT → multiply → IFFT), applies end correction for filter transients, then subtracts. Uses `sync.Once` for one-time filter initialisation. The existing `SubtractFT8` (per-symbol) is retained and still used by `DecodeIterative`; the FFT version is available as an alternative.

   - **Baseband time scan** (`decode.go`, `basebandTimeScan`) — For high-sync candidates where sync8 may have found the right frequency but wrong DT, performs a coarse `Sync8d` scan over the full NP2 range of the downsampled baseband signal. Used as a retry path in `DecodeIterative` when `SyncPower ≥ 2.0`.

   - **Compound callsign validation** (`validate.go`) — Extended `PlausibleCallsign` to handle compound callsigns (PREFIX/CALL or CALL/SUFFIX patterns like V4/SP9FIH, VK/ZL4XZ). Validates by checking that at least one part is a plausible standard callsign or a short alphanumeric prefix/suffix.

   - **OSD depth alignment** (`decode.go`) — Corrected `DecodeSingle` to match Fortran `ft8b.f90` line 405: `norder=2` is hardcoded regardless of `ndepth`, so `ndeepD2=2` and `ndeepD3=2` are both order-1 with npre1 pre-test. The Fortran `ndepth` parameter only controls `maxosd` (how many OSD calls), not the OSD search order.

   - **Research sub-package** (`research/`, ~3,600 lines) — Experimental code for investigating sync8 improvements, pipeline diagnostics, and root cause analysis of missing signals:
     - `sync8.go` (611 lines) — Complete research sync8 implementation faithfully porting WSJT-X `sync8.f90`. Uses scaffolded functions matching each Fortran step: `computeSpectrogram`, `computeSync2D`, `findPeaks`, `normalizeByPercentile`, `extractPreCandidates`, `suppressDuplicates`, `finalSort`. Operates on raw int16 audio (Fortran convention, no /32768 normalisation). Optimizations: `computeSync2D` pre-computes `sCos[7]` and `sNoise[7]` row slice pointers per frequency bin (hoisted outside the 125-lag inner loop), `extractPreCandidates` and `finalSort` pre-allocate slice capacity.
     - `realfft.go` / `realfft_test.go` (140 + 187 lines) — Optimized real-to-complex FFT using the "pack and unpack" trick (N/2-point complex FFT instead of full N-point). Pre-computed twiddle factor table (`[960]complex128` at package init) with half-cycle symmetry eliminates all `math.Cos`/`math.Sin` calls in the unpack loop for the NFFT1=3840 spectrogram path. Benchmarked: ~9% faster unpack loop (536 µs vs 587 µs per FFT), ~2× total speedup over `ft8x.RealFFT`. Validated against `ft8x.RealFFT` to <1e-6 relative error.
     - `params.go` (42 lines) — FT8 constants duplicated from `ft8x` to keep research self-contained. Matches `ft8_params.f90` exactly.
     - `iwave_test.go` (355 lines) — WAV loader (`loadIwave`) replicating WSJT-X `ft8d.f90` exact binary read: skips 44-byte header, reads int16 samples, converts to unscaled float32 (no /32768). Validates dd/ddNorm scaling factor is exactly 32768. Verifies that all decode-critical paths are scale-invariant.
     - `candidate_comparison_test.go` (214 lines) — Side-by-side comparison of research sync8 vs production `ft8x.Sync8FindCandidates` against all 21 WSJT-X reference signals from capture.wav. Measures coverage (within ±10 Hz, ±0.5s tolerance) and shows top-20 candidates from each.
     - `iterative_decode_test.go` (334 lines) — Research iterative decode pipeline closely matching `ft8_decode.f90` lines 168–239. Three-pass structure: pass 1 with lighter OSD (ndeep=2), passes 2–3 at full depth with early termination. Uses `SubtractFT8FFT` for signal subtraction. Includes comparison test against `ft8x.DecodeIterative`.
     - `missing_signals_test.go` (146 lines) — Deep dive into the 9 WSJT-X signals not covered by either sync8 implementation. Analyses sync2d values at expected freq/DT positions, searches for peaks in ±3 bin / ±10 lag windows, and compares against covered signals for calibration.
     - `pipeline_trace_test.go` (609 lines) — Comprehensive per-stage pipeline trace for each of the 5 missing signals from capture.wav (16/21 decoded, 5 missing). Traces: downsample → coarse sync → fine freq → refine time → symbol spectra/hard sync → soft metrics/LDPC. Tests each LLR pass (bmeta/b/c/d) both with and without AP. Also tests with subtraction (cleaned audio) and wide freq search (±20 Hz). Includes LDPC margin analysis: syndrome weight, mean |LLR|, and nhard for both decoded and missing signals.
     - `dt_offset_test.go` (271 lines) — Investigation of the systematic ~1.0s DT offset between our decoder and WSJT-X. Quantifies the offset (convention: WSJT-X DT = ft8b_xdt − 0.5). Tests subtraction quality: measures energy at nearby frequencies before/after `SubtractFT8` vs `SubtractFT8FFT`, and attempts to decode masked signals (CQ CO8LY FL20 at 932 Hz after subtracting CQ 4S6ARW at 938 Hz).
     - `synth_dt_test.go` (216 lines) — Synthetic signal timing verification. Generates a pure sinusoid and Gaussian-windowed pulse at known time positions, downsamples, and verifies the energy appears at the expected sample index. Confirms downsampler timing is correct.
     - `root_cause_test.go` (303 lines) — Definitive root cause analysis for the 5 missing capture.wav signals. Runs full `DecodeIterative`, classifies all 21 reference signals as decoded/missing, then for each missing signal tests on both original and cleaned audio with broad search (±10 Hz / ±0.5s). Conclusions: (1) AP limitation — most missing signals are non-CQ requiring AP types 2–6 with callsign knowledge; (2) weak signals at LDPC threshold edge; (3) pipeline is numerically correct.
     - `timing_test.go` (214 lines) — Per-pass and per-candidate timing measurement for the iterative decode pipeline. Reports sync8 time, decode loop time, per-candidate stats (avg/median/min/max ms), and decode rate (candidates/sec). Checks against WSJT-X real-time budget (15s FT8 period, 13.4s bail-out).

### Current test results

```
Capture 1 provided candidates:   7/13 correct, 0 false  (baseline: ≥7)
Capture 1 own candidates:        5/13 correct, 0 false
Capture 1 iterative:             8/13 correct, 0 false  (baseline: ≥8)

Capture 2 provided candidates:   9/17 correct, 0 false  (baseline: ≥9)
Capture 2 own candidates:        8/17 correct, 0 false
Capture 2 iterative:            11/17 correct, 0 false  (baseline: ≥11)

Capture 3 provided candidates:  14/21 correct, 0 false  (baseline: ≥14)
Capture 3 own candidates:       14/21 correct, 0 false
Capture 3 iterative:            16/21 correct, 0 false  (baseline: ≥16)

All unit tests:                  PASS
Full test suite (WAV, 3 caps):   ~39 s
```

### Source file inventory

| File | Lines | Purpose | Status |
|---|---|---|---|
| `params.go` | 65 | Constants (NSPS, NP2, NFFT sizes, etc.) | Stable |
| `decode.go` | 712 | `Decode()`, `DecodeSingle()`, `DecodeIterative()`, `SubtractFT8()`, `SubtractFT8FFT()`, `basebandTimeScan()` | Active development |
| `sync8.go` | 320 | `Sync8FindCandidates()` — spectrogram candidate detection | Stable |
| `sync.go` | 115 | `Sync8d()`, `BuildCtwk()`, `HardSync()` | Stable |
| `downsample.go` | 141 | `Downsampler`, `TwkFreq1()` | Stable |
| `metrics.go` | 211 | `ComputeSymbolSpectra()`, `ComputeSoftMetrics()` | Stable |
| `ldpc.go` | 937 | `Decode174_91()`, `BPDecode()`, `OSD174_91()`, `nextpat91()` | Stable |
| `ldpc_parity.go` | 396 | Parity check matrix hex data | Stable |
| `message.go` | 806 | `Unpack77()`, `Pack28()`, all message types | Stable |
| `crc.go` | 85 | CRC-14 computation | Stable |
| `encode.go` | 150 | LDPC encoder, tone generation, `GenFT8CWave()`, `gfskPulse()` | Stable |
| `fft.go` | 185 | FFT routing: radix-2, mixed-radix, Bluestein | Stable |
| `fft_mixedradix.go` | 198 | Mixed-radix Cooley-Tukey for 5-smooth sizes | Stable |
| `ap.go` | 197 | AP type constants, `ComputeAPSymbols()`, `ApplyAP()`, `APPassTypes()` | Stable |
| `validate.go` | 265 | `PlausibleCallsign()`, `PlausibleMessage()` | Updated (compound callsigns) |
| `ft8x_wav_test.go` | 831 | WAV integration tests (3 captures × 4 test modes) | Updated (capture 3) |
| `diag_test.go` | 227 | Missing-signal diagnostic tracing | New |
| `diag_cap2_test.go` | 298 | Capture 2 iterative pipeline diagnostic | New |
| `ft8_test.go` | 276 | Unit tests (OSD, nextpat91, platanh, encode) | Stable |
| `ap_test.go` | 248 | AP decoding tests | Stable |
| `validate_test.go` | 112 | Callsign/message plausibility tests | Stable |
| `sync8_test.go` | 72 | Sync8 unit tests | Stable |
| `fft_mixedradix_test.go` | 195 | FFT accuracy/round-trip tests | Stable |
| `cmd/ft8decode/main.go` | 91 | CLI tool | Stable |

### Research file inventory

| File | Lines | Purpose |
|---|---|---|
| `sync8.go` | 611 | Complete research sync8 port (spectrogram, sync2d, peak finding, normalization, candidates). Inner-loop index caching: pre-computed row pointers for Costas and noise rows per freq bin. Pre-allocated candidate slices. |
| `realfft.go` | 140 | Optimized N/2-point real FFT with pack/unpack. Pre-computed twiddle table (`[960]complex128`) with half-cycle symmetry for the NFFT1=3840 hot-path. |
| `realfft_test.go` | 187 | RealFFT validation, capture test, and performance benchmark |
| `params.go` | 42 | Duplicated FT8 constants |
| `iwave_test.go` | 355 | WAV loader (Fortran-matching), iwave/ddNorm scaling verification |
| `candidate_comparison_test.go` | 214 | Research sync8 vs production sync8 coverage comparison |
| `iterative_decode_test.go` | 334 | Research multi-pass decode pipeline, comparison vs `ft8x.DecodeIterative` |
| `missing_signals_test.go` | 146 | Sync2d analysis of 9 uncovered WSJT-X signals |
| `pipeline_trace_test.go` | 609 | Per-stage pipeline trace + LDPC margin analysis for 5 missing signals |
| `dt_offset_test.go` | 271 | DT offset quantification + subtraction quality measurement |
| `synth_dt_test.go` | 216 | Synthetic signal timing + downsampler pulse verification |
| `root_cause_test.go` | 303 | Root cause analysis — AP limitation, weak signal classification |
| `timing_test.go` | 214 | Per-pass/per-candidate timing, real-time budget check |

---

## What to do next

### Step 9 (continued): Close the gap to Phase 1 targets

**Current gaps vs Phase 1 targets:**

| Metric | Current | Target | Gap |
|---|---|---|---|
| Capture 1 iterative | 8/13 | ≥ 11/13 | 3 more decodes needed |
| Capture 2 iterative | 11/17 | ≥ 14/17 | 3 more decodes needed |
| Capture 3 iterative | 16/21 | ≥ 18/21 (extrapolated) | 2 more decodes needed |
| False decode rate | 0 | ≤ 1 per capture | ✅ Met |
| Full decode cycle | ~13 s per capture | < 2 s per capture | Need ~6× speedup |

**Missing signals analysis (from diagnostic tests):**

Capture 1 missing (5 signals): `<...> RA1OHX KP91` (DT mismatch), `<...> RA6ABC KN96` (weak, nhard>30), `ES2AJ UA3LAR KO75` (very weak), `HZ1TT RU1AB R-10` (late DT), `<...> RV6ASU KN94` (weak). Most are borderline SNR signals requiring deeper OSD or AP with callsign knowledge.

Capture 2 missing (6 signals): `UY7VV KE6SU DM14` (decoded with provided candidates but lost in iterative — possible subtraction damage), `RU4LM 4X5JK R-14`, `JT1CO IZ7DIO 73`, `VK3ZSJ US7KC KO21`, `JR3UIC SP7IIT RR73`, `JT1CO YO3HST KN24` (all weak, nhard>20).

Capture 3 missing (5 signals): `5Z4VJ YB1RUS OI33`, `UA0LW UA4ARH -15`, `CQ CO8LY FL20`, `VK3TZ UA3ZNQ KO81`, `VK3TZ RC7O KN87`. Root cause analysis (`root_cause_test.go`) confirms: most are non-CQ signals requiring AP types 2–6 with callsign knowledge that our CQ-only AP cannot provide. All 5 fail LDPC even on cleaned audio with broad search (±10 Hz / ±0.5s). The pipeline is numerically correct — the gap is due to AP limitation and weak signal levels, not a Go vs Fortran precision issue.

**Highest-impact improvement paths (ordered by expected yield):**

1. **Use `SubtractFT8FFT` in the iterative loop** — The FFT-based subtraction (already implemented) should produce cleaner subtraction than per-symbol averaging, potentially recovering signals lost to subtraction artifacts. Swap `SubtractFT8` → `SubtractFT8FFT` in `DecodeIterative` and measure regression.

2. **Enable deeper AP on later passes** — Currently `DecodeIterative` uses CQ-only AP. For later passes (after strong signals are subtracted), enabling AP types 2–6 with `MyCall`/`DxCall` could recover weak QSO signals. The frequency guard (`NfQSO + APWidth`) needs tuning to avoid false positives.

3. **Sync8 candidate DT accuracy** — Some signals are found by sync8 at correct frequency but with DT offsets that `DecodeSingle`'s ±10-step coarse search can't recover. The `basebandTimeScan` retry path (already implemented) helps for `SyncPower ≥ 2.0` candidates. Consider widening the coarse search to ±20 steps, or lowering the `SyncPower` threshold for the retry.

4. **Coarse time search ±10 → ±20** — The Fortran `ft8b.f90` uses `idt = isync - 10..isync + 10` but the Fortran's `isync` value from sync8 is more accurate because sync8.f90 uses finer DT resolution (0.04s steps vs our coarse mapping). Widening the Go search compensates for this resolution difference.

5. **Performance: FFT caching across passes** — The 192k-point FFT (64 ms) is recomputed on every pass. After subtraction, only the subtracted signal's frequency bins change. Consider either (a) caching the 192k FFT and doing frequency-domain subtraction, or (b) only recomputing the 192k FFT on passes where signals were actually subtracted.

6. **Performance: OSD budget limiting** — Add a time or iteration budget to OSD to prevent slow passes. The Fortran limits OSD to `maxosd` calls per candidate; the Go code does this but `maxosd=2` means every failed candidate still runs 2 full OSD searches.

7. **ScaleFac re-tuning** — The BP LLR scaling factor (2.83) was tuned for the original decoder. With the upgraded OSD and AP, re-tuning could improve convergence for marginal signals. Consider testing ScaleFac values from 2.5–3.2 against all three captures.

### Phase 1 success criteria (from assessment §5.5)

| Metric | Current | Target |
|---|---|---|
| Capture 1 correct | 8/13 (iterative) | ≥ 11/13 |
| Capture 2 correct | 11/17 (iterative) | ≥ 14/17 |
| Capture 3 correct | 16/21 (iterative) | ≥ 18/21 |
| False decode rate | 0 | ≤ 1 per capture |
| Full decode cycle | ~13 s per capture | < 2 s per capture |

---

## Known issues and decisions

1. **`SubtractFT8` vs `SubtractFT8FFT`** — Two subtraction implementations coexist. `SubtractFT8` (per-symbol, ~2 ms, no FFTs) is used in production. `SubtractFT8FFT` (FFT-based, faithful Fortran port with cos² filter window, end correction) is implemented but not yet used by `DecodeIterative`. The FFT version requires two 180k-point FFTs per call but produces smoother subtraction. Switch to `SubtractFT8FFT` once measured on all captures.

2. **OSD depth: ndeep=2 everywhere** — After tracing the Fortran code, `ft8b.f90` line 405 hardcodes `norder=2` regardless of `ndepth`. This means Go's `ndeepD2=2` and `ndeepD3=2` are both correct. The Fortran `ndepth` only controls `maxosd` (0, 1, or 2 OSD attempts per candidate). `Depth=2` → `maxosd=1`, `Depth=3` → `maxosd=2`.

3. **Sync8 own-candidates decode gap** — Sync8 finds candidates near all reference signals, but decode success is limited by decoder sensitivity. The iterative subtraction loop improved counts significantly (Capture 3: 14→16). Further improvement requires deeper OSD or AP decoding.

4. **Mixed-radix FFT memory** — The recursive implementation allocates O(n × log n) temporary memory. An iterative Stockham approach would reduce this to O(n). Acceptable for Phase 1; optimize in Phase 2 if profiling shows GC pressure.

5. **Package name `ft8x`** — Inherited from the goft8 baseline. Intentionally kept to avoid import collision with any future `ft8` package.

6. **WAV test files** — Seven WAV files in `testdata/`. The three primary captures (`ft8test_capture_20260410.wav`, `ft8test_capture2_20260410.wav`, `capture.wav`) are tracked. Tests skip gracefully if files are missing.

7. **platanh: math.Atanh vs Fortran piecewise** — Retained `math.Atanh` with ±19.07 clamping. The Fortran piecewise-linear `platanh` caused a 1-decode regression because the BP decoder's LLR scaling (`ScaleFac=2.83`) was tuned with `math.Atanh`. May revisit when ScaleFac is re-tuned.

8. **OSD ntheta pre-test bypass for order-1** — For order-1 base patterns (91 candidates), the pre-test is bypassed to avoid marginal-signal regressions. Higher-order patterns use the pre-test for performance.

9. **AP decoding: ncontest=0 only** — Only standard QSO mode is implemented. Contest modes can be added as a future enhancement.

10. **AP frequency guard** — AP types ≥3 are restricted to candidates within `APWidth` Hz of `NfQSO`. When `NfQSO=0` (default), the frequency guard is disabled.

11. **xdt offset convention** — `DecodeSingle` computes `xdt = (ibest-1)*Dt2` where ibest is 0-based in Go, introducing a systematic -0.005s offset vs the Fortran. This only affects display DT and subtraction alignment (60 samples ≈ 3% of a symbol).

12. **`DecodeIterative` pass structure** — The current implementation uses a fixed pass structure:
    - Pass 0: lighter OSD (`Depth=2` when `params.Depth=3`) + CQ-only AP + up to 300 candidates
    - Passes 1..N-2: full depth + CQ-only AP + up to 300 candidates
    - Pass N-1 (final): `Depth=3` + CQ-only AP + up to 100 candidates (deepest search)
    - All passes: `basebandTimeScan` retry for `SyncPower ≥ 2.0` candidates that fail on first attempt
    - Early termination when a pass produces no new decodes

13. **Research sub-package** — The `research/` directory (~3,600 lines across 13 Go files) contains experimental code that is not part of the production decoder. It is used for investigating sync8 improvements, pipeline diagnostics, and root cause analysis of missing signals. Tests in `research/` import the production `ft8x` package for comparison. Key findings from the research:
    - **DT convention**: WSJT-X reports DT = ft8b_xdt − 0.5. Our decoder reports xdt directly. This is a display convention difference, NOT a bug — subtraction and decode are unaffected.
    - **Downsampler timing**: Verified correct with synthetic pulse test — energy appears at the expected sample index.
    - **Subtraction quality**: Both `SubtractFT8` and `SubtractFT8FFT` reduce signal energy at the target frequency. Residual energy at nearby frequencies is from other signals, not subtraction artifacts.
    - **Root cause of missing signals**: The 5 missing signals from capture.wav (16→21 gap) fail due to (a) insufficient AP — most are non-CQ messages requiring AP types 2–6 with callsign knowledge, and (b) weak signal levels at the LDPC threshold edge. The pipeline is numerically correct.
    - **Research sync8**: Complete port of sync8.f90 with optimized RealFFT (~2× speedup). Coverage matches production sync8 on capture.wav reference signals.
    - **Performance optimization evaluation**: Three proposed sync8 micro-optimizations were evaluated: (1) Pre-computed twiddle table in RealFFT — ~9% per-FFT improvement, modest but zero-risk (implemented). (2) Inner-loop index caching in `computeSync2D` — pre-computed row pointers eliminate redundant index arithmetic across 125 lag iterations (implemented). (3) `suppressDuplicates` O(n²) bucketing — rejected: only ~200 candidates in practice (not 1000), runs in <0.1ms, bucketing adds complexity for no measurable gain. (4) Spectrogram/sync2d over-allocation trim — rejected: spectrogram writes all 1920 FFT bins (can't shrink without coupling stages), sync2d savings ~0.14 MB (trivial), untouched pages likely uncommitted by OS anyway. The dominant cost remains the 372 mixed-radix FFTs per spectrogram (~200ms total), not the twiddle/index arithmetic.
    - **Timing**: Per-candidate decode takes ~1–10 ms depending on OSD depth. Full 3-pass pipeline on capture.wav runs within the 15s real-time budget.

---

## Reference codebase locations

| Path | Language | Use for |
|---|---|---|
| `~/Development/wsjt-wsjtx/` | Fortran | Gold standard — `lib/ft8/ft8b.f90`, `sync8.f90`, `decode174_91.f90`, `subtractft8.f90` |
| `~/Development/ft8_lib/` | C | Clean LDPC reference — `ft8/decode.c`, `ft8/encode.c` |
| `~/Development/jtdx/` | Fortran | WSJT-X fork with OSD/AP optimisations |
| `~/Development/goft8/ft8/` | Go | The original ft8x baseline these files were copied from |
