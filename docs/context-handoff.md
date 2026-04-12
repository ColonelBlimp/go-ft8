# Context Handoff — go-ft8 Phase 1

**Date:** 2026-04-11
**Module:** `github.com/ColonelBlimp/go-ft8`
**Go version:** 1.25
**Total code:** ~7,100 lines across 24 Go files (+ ~4,200 lines in `research/`)

---

## ⚠️ ACTIVE DEVELOPMENT IS IN THE `research/` PACKAGE ONLY

**Do NOT modify production code** (the `ft8x` package at the repository root).
All experimental changes, pipeline improvements, and diagnostic work MUST go
into the `research/` sub-package (`/home/mveary/Development/go-ft8/research/`).

### 🚫 CRITICAL RULES — READ BEFORE DOING ANYTHING

1. **DO NOT read or reference production Go code** — not `message.go`, not
   `ldpc.go`, not ANY file in the repository root. The Fortran at
   `~/Development/wsjt-wsjtx/` is the **sole source of truth** for porting.
   Reading production code wastes time and risks inheriting bugs.

2. **DO NOT run production tests** — `go test ./...` or `go test -short ./...`
   at the repository root touches the production `ft8x` package. Only run
   `go test ./research/` (with appropriate flags). Production is frozen and
   untouched; its tests are irrelevant to research work.

3. **DO NOT import production code** in new research files — the research
   package still has thin stubs that delegate to `ft8x` for un-ported
   functions. These are being eliminated one by one. New ports must be
   self-contained with zero `ft8x` imports.

4. **Port from Fortran → Go directly.** Read the `.f90` file, write the `.go`
   file. That's the workflow. No detours.

### Why research-only?

Production changes were attempted (2026-04-11) and **all regressed or had
zero net benefit**:

| Change tried in production | Result |
|---|---|
| `SubtractFT8FFT` in `DecodeIterative` | **−1 regression** in Capture 1 (lost KB7THX WB9VGJ RR73 at nsync=6 boundary), neutral in Caps 2/3 |
| Lower `basebandTimeScan` threshold 1.5→1.3 | Compounded Cap1 regression |
| Finer `basebandTimeScan` step 4→2 | Compounded Cap1 regression |
| Enable AP type 2 when MyCall set | No regression, but no improvement (opt-in, never triggered by tests) |

**All production changes were reverted. `decode.go` has zero diff from git HEAD.**

### Research workflow

1. Read the Fortran source file (the `.f90` in `~/Development/wsjt-wsjtx/`)
2. Write the Go port in `research/` — self-contained, no production imports
3. Build: `go build ./research/`
4. Test: `go test -short -count=1 ./research/` (research tests ONLY)
5. Once all stubs are ported, run `TestRootCauseAllCaptures` to measure impact

---

## Fortran porting progress

The research package is being systematically decoupled from production `ft8x`.
Each stub file either contains a **direct Fortran port** (✅) or a **thin
delegation to production** (`// TODO: port from Fortran`) that will be replaced.

The porting order follows the call chain bottom-up, so each piece can be
verified in isolation before composing:

| # | File | Functions | Fortran source | Status |
|---|---|---|---|---|
| 1 | `sync_d.go` | `Sync8d()`, `HardSync()` | `sync8d.f90`, `ft8b.f90:163–176` | ✅ Ported |
| 2 | `metrics.go` | `ComputeSymbolSpectra()`, `ComputeSoftMetrics()`, `normalizeBmet()`, `fft32()` | `ft8b.f90:154–233, 466–479` | ✅ Ported |
| 3 | `downsample.go` | `Downsampler`, `TwkFreq1()` | `ft8_downsample.f90`, `twkfreq1.f90` | ✅ Ported (trailing taper bug fixed 2026-04-12) |
| 4 | `ldpc.go` + `crc.go` | `Decode174_91()`, `BPDecode()`, `OSDDecode()`, CRC-14 | `decode174_91.f90`, `osd174_91.f90` | ✅ Ported |
| 5 | `ldpc_parity.go` | `LDPCMn`, `LDPCNm`, `LDPCNrw`, generator hex | `ldpc_174_91_c_parity.f90` | ✅ Ported |
| 6 | `message.go` | `Unpack77()`, `BitsToC77()`, `unpack28()`, grid helpers | `packjt77.f90` | ✅ Ported |
| 7 | `ap.go` | `ApplyAP()` | `ft8b.f90:243–401` | TODO |
| 8 | `encode.go` | `GenFT8Tones()`, `GenFT8CWave()` | `gen_ft8.f90` | TODO |
| 9 | `subtract.go` | `SubtractFT8()`, `SubtractFT8FFT()` | `subtractft8.f90` | TODO |
| 10 | `decode.go` | `DecodeSingle()`, `DecodeIterative()`, `Sync8FindCandidates()` | `ft8b.f90`, `ft8_decode.f90`, `sync8.f90` | TODO |
| 11 | `fft.go` | `FFT()`, `IFFT()` | `four2a.f90` | TODO (delegates to production) |
| 12 | `realfft.go` | `RealFFT()` | N/A (optimized N/2-point trick) | ✅ Already local (uses local `FFT()`) |

### Key porting notes

- **`fft32()`** — A self-contained 32-point radix-2 FFT was written in `metrics.go`
  specifically for the symbol spectra computation (matching `four2a(csymb,32,1,-1,1)`).
  This avoids depending on the general-purpose FFT router for the hot inner loop.

- **`fft.go`** delegates `FFT()`/`IFFT()` to production for now. These route to
  mixed-radix for 5-smooth sizes. Will be ported last since the FFT implementation
  is well-tested and unlikely to be the source of decode differences.

- **Test files** (`*_test.go`) have been updated to call local research functions
  instead of `ft8x.*`. No test file imports production code.

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
| 9 | Comprehensive testing | 🔶 In progress | Diagnostic tests, research pipeline, 3 captures, root cause analysis |

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

   - **FFT-based signal subtraction** (`decode.go`, `SubtractFT8FFT`) — Faithful port of WSJT-X `subtractft8.f90`. Generates GFSK reference, conjugate-multiplies, low-pass filters in frequency domain (cos² window → FFT → multiply → IFFT), applies end correction for filter transients, then subtracts. Uses `sync.Once` for one-time filter initialisation. The existing `SubtractFT8` (per-symbol) is retained and used by production `DecodeIterative`; the FFT version is used by the research pipeline.

   - **Baseband time scan** (`decode.go`, `basebandTimeScan`) — For high-sync candidates where sync8 may have found the right frequency but wrong DT, performs a coarse `Sync8d` scan over the full NP2 range of the downsampled baseband signal. Used as a retry path in `DecodeIterative` when `SyncPower ≥ 2.0`.

   - **Compound callsign validation** (`validate.go`) — Extended `PlausibleCallsign` to handle compound callsigns (PREFIX/CALL or CALL/SUFFIX patterns like V4/SP9FIH, VK/ZL4XZ). Validates by checking that at least one part is a plausible standard callsign or a short alphanumeric prefix/suffix.

   - **OSD depth alignment** (`decode.go`) — Corrected `DecodeSingle` to match Fortran `ft8b.f90` line 405: `norder=2` is hardcoded regardless of `ndepth`, so `ndeepD2=2` and `ndeepD3=2` are both order-1 with npre1 pre-test. The Fortran `ndepth` parameter only controls `maxosd` (how many OSD calls), not the OSD search order.

   - **Root cause analysis across all 3 captures** (`research/root_cause_all_test.go`, 466 lines) — Comprehensive classification of all 18 missing signals (vs 51 WSJT-X reference) into failure modes. See "Root cause analysis results" section below.

   - **Research sub-package** (`research/`, ~3,600 lines) — Experimental code for investigating sync8 improvements, pipeline diagnostics, and root cause analysis of missing signals. See "Research file inventory" section below.

### Current production test results

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

### Root cause analysis results (research pipeline, all 3 captures)

Run via: `go test -v -run "TestRootCauseAllCaptures" ./research/`

**Research pipeline decode counts** (using research sync8 + research stubs → ft8x delegation):

| Capture | Research pipeline | Production iterative | WSJT-X reference |
|---|---|---|---|
| Cap 1 | 7/13 | 8/13 | 13/13 |
| Cap 2 | 10/17 | 11/17 | 17/17 |
| Cap 3 | 16/21 | 16/21 | 21/21 |

**Note:** Production iterative slightly outperforms the research pipeline on Caps 1 and 2.
This means the research pipeline's sync8 or candidate handling has room for improvement
before it can serve as a better baseline than production.

**Failure classification (16 missing in research, 18 missing vs WSJT-X):**

| Failure mode | Count | Captures | Description |
|---|---|---|---|
| `sync_fail + ap_limitation` | 10 | Cap1: 6, Cap3: 4 | nsync ≤ 6, all non-CQ — blocked by DecodeSingle hard gate AND need AP type ≥2 |
| `ldpc_fail + ap_limitation` | 5 | Cap2: 5 | nsync 8–14 (passes sync), LDPC can't converge, all non-CQ — need AP type ≥2 |
| `subtraction_needed` | 2 | Cap2: 2 | Decode OK on original audio at exact params, missed by pipeline candidate search |
| `sync_fail` | 1 | Cap3: 1 | CQ CO8LY masked by nearby CQ 4S6ARW (6 Hz separation), genuinely at noise floor |

**Key finding:** The dominant gap is AP limitation. 15 of 18 missing signals are non-CQ
messages that WSJT-X decodes using interactive QSO-progress AP (types 2–6, injecting
32–77 known bits). Our CQ-only AP (type 1, 32 bits) can't help these. The pipeline is
numerically correct — the gap is NOT a Go vs Fortran precision issue.

### Production source file inventory

| File | Lines | Purpose | Status |
|---|---|---|---|
| `params.go` | 65 | Constants (NSPS, NP2, NFFT sizes, etc.) | Stable — DO NOT MODIFY |
| `decode.go` | 712 | `Decode()`, `DecodeSingle()`, `DecodeIterative()`, `SubtractFT8()`, `SubtractFT8FFT()`, `basebandTimeScan()` | Stable — DO NOT MODIFY |
| `sync8.go` | 320 | `Sync8FindCandidates()` — spectrogram candidate detection | Stable — DO NOT MODIFY |
| `sync.go` | 115 | `Sync8d()`, `BuildCtwk()`, `HardSync()` | Stable — DO NOT MODIFY |
| `downsample.go` | 141 | `Downsampler`, `TwkFreq1()` | Stable — DO NOT MODIFY |
| `metrics.go` | 211 | `ComputeSymbolSpectra()`, `ComputeSoftMetrics()` | Stable — DO NOT MODIFY |
| `ldpc.go` | 937 | `Decode174_91()`, `BPDecode()`, `OSD174_91()`, `nextpat91()` | Stable — DO NOT MODIFY |
| `ldpc_parity.go` | 396 | Parity check matrix hex data | Stable — DO NOT MODIFY |
| `message.go` | 806 | `Unpack77()`, `Pack28()`, all message types | Stable — DO NOT MODIFY |
| `crc.go` | 85 | CRC-14 computation | Stable — DO NOT MODIFY |
| `encode.go` | 150 | LDPC encoder, tone generation, `GenFT8CWave()`, `gfskPulse()` | Stable — DO NOT MODIFY |
| `fft.go` | 185 | FFT routing: radix-2, mixed-radix, Bluestein | Stable — DO NOT MODIFY |
| `fft_mixedradix.go` | 198 | Mixed-radix Cooley-Tukey for 5-smooth sizes | Stable — DO NOT MODIFY |
| `ap.go` | 197 | AP type constants, `ComputeAPSymbols()`, `ApplyAP()`, `APPassTypes()` | Stable — DO NOT MODIFY |
| `validate.go` | 265 | `PlausibleCallsign()`, `PlausibleMessage()` | Stable — DO NOT MODIFY |
| `ft8x_wav_test.go` | 831 | WAV integration tests (3 captures × 4 test modes) | Stable — DO NOT MODIFY |
| `diag_test.go` | 227 | Missing-signal diagnostic tracing | Stable |
| `diag_cap2_test.go` | 298 | Capture 2 iterative pipeline diagnostic | Stable |
| `ft8_test.go` | 276 | Unit tests (OSD, nextpat91, platanh, encode) | Stable |
| `ap_test.go` | 248 | AP decoding tests | Stable |
| `validate_test.go` | 112 | Callsign/message plausibility tests | Stable |
| `sync8_test.go` | 72 | Sync8 unit tests | Stable |
| `fft_mixedradix_test.go` | 195 | FFT accuracy/round-trip tests | Stable |
| `cmd/ft8decode/main.go` | 91 | CLI tool | Stable |

### Research file inventory (ACTIVE DEVELOPMENT)

All research files are in `research/` (`/home/mveary/Development/go-ft8/research/`).
The research package is being decoupled from production `ft8x` — all test files
now call local functions; stub files delegate to production only until ported.

**Ported from Fortran (self-contained, no production dependency):**

| File | Lines | Fortran source | Purpose |
|---|---|---|---|
| `sync_d.go` | 170 | `sync8d.f90`, `ft8b.f90:163–176` | `Sync8d()` — Costas correlation; `HardSync()` — nsync count |
| `metrics.go` | 260 | `ft8b.f90:154–233, 466–479` | `ComputeSymbolSpectra()` — 32-pt FFT per symbol; `ComputeSoftMetrics()` — 4-pass LLR extraction; `normalizeBmet()`; `fft32()` |
| `sync8.go` | 611 | `sync8.f90` | Research sync8 with optimized inner loops |
| `realfft.go` | 138 | N/A | Optimized N/2-point real FFT (uses local `FFT()`) |
| `params.go` | 42 | `ft8_params.f90` | Base FT8 constants (Fs, NSPS, NN, NMAX, etc.) |
| `constants.go` | 53 | `ft8_params.f90` | Derived constants (NP2, Fs2, Dt2, ScaleFac, LDPC params, GrayMap) |
| `ldpc.go` | ~800 | `decode174_91.f90`, `osd174_91.f90` | `Decode174_91()`, `BPDecode()`, `OSDDecode()`, `platanh()`, `nextpat91()` |
| `ldpc_parity.go` | ~400 | `ldpc_174_91_c_parity.f90` | Parity check matrices `LDPCMn`, `LDPCNm`, `LDPCNrw`; generator hex data |
| `crc.go` | ~50 | `crc14.cpp` | `CRC14Bits()`, `CheckCRC14Codeword()` — CRC-14 for LDPC |
| `message.go` | ~785 | `packjt77.f90` | `Unpack77()`, `BitsToC77()`, `unpack28()`, `unpacktext77()`, grid helpers |
| `downsample.go` | 221 | `ft8_downsample.f90`, `twkfreq1.f90` | `Downsampler`, `NewDownsampler()`, `Downsample()`, `TwkFreq1()`, `cshift()` |

**Stub files (delegate to production, TODO port from Fortran):**

| File | Lines | Fortran source | Functions stubbed |
|---|---|---|---|
| `ap.go` | 21 | `ft8b.f90:243–401` | `ApplyAP()` |
| `encode.go` | 29 | `gen_ft8.f90` | `GenFT8Tones()`, `GenFT8CWave()` |
| `subtract.go` | 32 | `subtractft8.f90` | `SubtractFT8()`, `SubtractFT8FFT()` |
| `decode.go` | 66 | `ft8b.f90`, `ft8_decode.f90`, `sync8.f90` | `DecodeSingle()`, `DecodeIterative()`, `Sync8FindCandidates()` |
| `fft.go` | 32 | `four2a.f90` | `FFT()`, `IFFT()` |

**Test and diagnostic files:**

| File | Lines | Purpose |
|---|---|---|
| `realfft_test.go` | 187 | RealFFT validation, capture test, and performance benchmark |
| `iwave_test.go` | 355 | WAV loader (`loadIwave`) replicating WSJT-X exact binary read |
| `candidate_comparison_test.go` | 214 | Research sync8 vs production sync8 coverage comparison |
| `iterative_decode_test.go` | 334 | Research multi-pass decode pipeline |
| `missing_signals_test.go` | 146 | Sync2d analysis of uncovered WSJT-X signals |
| `pipeline_trace_test.go` | 609 | Per-stage pipeline trace + LDPC margin analysis |
| `dt_offset_test.go` | 271 | DT offset quantification + subtraction quality |
| `synth_dt_test.go` | 216 | Synthetic signal timing + downsampler pulse verification |
| `root_cause_test.go` | 303 | Root cause analysis (capture.wav only) |
| `root_cause_all_test.go` | 466 | Root cause analysis for ALL 3 captures |
| `timing_test.go` | 214 | Per-pass/per-candidate timing |
| `twid_bench_test.go` | — | Twiddle factor benchmark |

---

## What to do next

### Continue porting stubs from Fortran (in dependency order)

The immediate task is to continue replacing stub delegations with direct Fortran
ports, working up the call chain:

1. ~~`sync_d.go` — `Sync8d`, `HardSync`~~ ✅
2. ~~`metrics.go` — `ComputeSymbolSpectra`, `ComputeSoftMetrics`~~ ✅
3. ~~`downsample.go` — `Downsampler`, `TwkFreq1()`~~ ✅ (trailing taper bug fixed 2026-04-12)
4. ~~`ldpc.go` + `crc.go` — `Decode174_91()`, `BPDecode()`, `OSDDecode()`, CRC-14~~ ✅
5. ~~`ldpc_parity.go` — `LDPCMn`, `LDPCNm`, `LDPCNrw`, generator hex~~ ✅
6. ~~`message.go` — `Unpack77()`, `BitsToC77()`~~ ✅
7. **`ap.go`** — `ApplyAP()` ← NEXT
8. **`encode.go`** — `GenFT8Tones()`, `GenFT8CWave()`
9. **`subtract.go`** — `SubtractFT8()`, `SubtractFT8FFT()`
10. **`decode.go`** — `DecodeSingle()`, `DecodeIterative()`

Once all stubs are ported, the research package will be **fully self-contained**
with zero production imports, and every line traceable to the Fortran source.
At that point we can:
- Run all 3 captures and compare decode counts
- Trace any remaining differences to specific Fortran lines
- Make targeted improvements with measurable impact

### Phase 1 success criteria (from assessment §5.5)

| Metric | Current | Target |
|---|---|---|
| Capture 1 correct | 8/13 (production iterative) | ≥ 11/13 |
| Capture 2 correct | 11/17 (production iterative) | ≥ 14/17 |
| Capture 3 correct | 16/21 (production iterative) | ≥ 18/21 |
| False decode rate | 0 | ≤ 1 per capture |
| Full decode cycle | ~13 s per capture | < 2 s per capture |

---

## Known issues and decisions

1. **Production is frozen** — No changes to production `ft8x` package until a research change is proven across all 3 captures. All experimental work in `research/`.

2. **`SubtractFT8` vs `SubtractFT8FFT`** — Two subtraction implementations coexist. `SubtractFT8` (per-symbol, ~2 ms, no FFTs) is used in production `DecodeIterative`. `SubtractFT8FFT` (FFT-based, faithful Fortran port) is used in the research pipeline. Switching production to `SubtractFT8FFT` was tested and caused a **−1 regression** in Capture 1 (KB7THX WB9VGJ RR73 lost at nsync=6 boundary), so production stays with `SubtractFT8`.

3. **Research pipeline is currently ≤ production** — The research iterative pipeline (root_cause_all_test.go) gets 7/10/16 vs production's 8/11/16. This must be resolved before research improvements can be meaningfully measured.

4. **OSD depth: ndeep=2 everywhere** — After tracing the Fortran code, `ft8b.f90` line 405 hardcodes `norder=2` regardless of `ndepth`. This means Go's `ndeepD2=2` and `ndeepD3=2` are both correct. The Fortran `ndepth` only controls `maxosd` (0, 1, or 2 OSD attempts per candidate). `Depth=2` → `maxosd=1`, `Depth=3` → `maxosd=2`.

5. **Sync8 own-candidates decode gap** — Sync8 finds candidates near all reference signals, but decode success is limited by decoder sensitivity. The iterative subtraction loop improved counts significantly (Capture 3: 14→16). Further improvement requires deeper OSD or AP decoding.

6. **Mixed-radix FFT memory** — The recursive implementation allocates O(n × log n) temporary memory. An iterative Stockham approach would reduce this to O(n). Acceptable for Phase 1; optimize in Phase 2 if profiling shows GC pressure.

7. **Package name `ft8x`** — Inherited from the goft8 baseline. Intentionally kept to avoid import collision with any future `ft8` package.

8. **WAV test files** — Seven WAV files in `testdata/`. The three primary captures (`ft8test_capture_20260410.wav`, `ft8test_capture2_20260410.wav`, `capture.wav`) are tracked. Tests skip gracefully if files are missing.

9. **platanh: math.Atanh vs Fortran piecewise** — Retained `math.Atanh` with ±19.07 clamping. The Fortran piecewise-linear `platanh` caused a 1-decode regression because the BP decoder's LLR scaling (`ScaleFac=2.83`) was tuned with `math.Atanh`. May revisit when ScaleFac is re-tuned.

10. **OSD ntheta pre-test bypass for order-1** — For order-1 base patterns (91 candidates), the pre-test is bypassed to avoid marginal-signal regressions. Higher-order patterns use the pre-test for performance.

11. **AP decoding: ncontest=0 only** — Only standard QSO mode is implemented. Contest modes can be added as a future enhancement.

12. **AP frequency guard** — AP types ≥3 are restricted to candidates within `APWidth` Hz of `NfQSO`. When `NfQSO=0` (default), the frequency guard is disabled.

13. **xdt offset convention** — `DecodeSingle` computes `xdt = (ibest-1)*Dt2` where ibest is 0-based in Go, introducing a systematic -0.005s offset vs the Fortran. This only affects display DT and subtraction alignment (60 samples ≈ 3% of a symbol).

14. **`DecodeIterative` pass structure** — The current implementation uses a fixed pass structure:
    - Pass 0: lighter OSD (`Depth=2` when `params.Depth=3`) + CQ-only AP + up to 300 candidates
    - Passes 1..N-2: full depth + CQ-only AP + up to 300 candidates
    - Pass N-1 (final): `Depth=3` + CQ-only AP + up to 100 candidates (deepest search)
    - All passes: `basebandTimeScan` retry for `SyncPower ≥ 2.0` candidates that fail on first attempt
    - Early termination when a pass produces no new decodes

15. **Research package decoupling progress** — As of 2026-04-12, 11 of 12 research
    library files have been ported from Fortran or are self-contained:
    `sync_d.go`, `metrics.go`, `downsample.go`, `sync8.go`, `realfft.go`, `params.go`,
    `constants.go`, `ldpc.go`, `ldpc_parity.go`, `crc.go`, `message.go`. The remaining
    5 stub files (`ap.go`, `encode.go`, `subtract.go`, `decode.go`, `fft.go`) still
    delegate to production `ft8x` and will be replaced one at a time.

---

## Reference codebase locations

| Path | Language | Use for |
|---|---|---|
| `~/Development/wsjt-wsjtx/` | Fortran | **SOLE SOURCE OF TRUTH** — `lib/ft8/ft8b.f90`, `sync8.f90`, `sync8d.f90`, `ft8_downsample.f90`, `decode174_91.f90`, `osd174_91.f90`, `subtractft8.f90`, `twkfreq1.f90` |
| `~/Development/ft8_lib/` | C | Secondary reference — `ft8/decode.c`, `ft8/encode.c` |
| `~/Development/jtdx/` | Fortran | WSJT-X fork with OSD/AP optimisations |
| `~/Development/goft8/ft8/` | Go | The original ft8x baseline (historical only — do NOT use as reference) |
