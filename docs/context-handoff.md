# Context Handoff — go-ft8 Phase 1

**Date:** 2026-04-12
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

3. **DO NOT import production code** in research files — the research
   package is now **fully self-contained** with zero `ft8x` imports.
   All 16 library files have been ported. Keep it that way.

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
5. All stubs are ported — run `TestRootCauseAllCaptures` to measure impact

---

## Fortran porting progress

The research package has been **fully decoupled** from production `ft8x`.
All 17 library files are direct Fortran ports or self-contained implementations
with zero production imports.

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
| 7 | `ap.go` | `ApplyAP()`, `ComputeAPSymbols()` | `ft8b.f90:243–401`, `ft8apset.f90` | ✅ Ported (ncontest=0 only) |
| 7b | `pack28.go` | `pack28()`, `ihashcall()`, `stdcall()` | `packjt77.f90:621–751`, `packjt77.f90:64–79`, `q65_set_list.f90:66–97` | ✅ Ported |
| 8 | `encode.go` | `GenFT8Tones()`, `GenFT8CWave()`, `gfskPulse()` | `genft8.f90`, `gen_ft8wave.f90`, `gfsk_pulse.f90` | ✅ Ported |
| 9 | `subtract.go` | `SubtractFT8()` | `subtractft8.f90` | ✅ Ported |
| 10 | `decode.go` | `DecodeSingle()`, `DecodeIterative()`, `Sync8FindCandidates()`, types | `ft8b.f90`, `ft8_decode.f90` | ✅ Ported |
| 11 | `fft.go` | `FFT()`, `IFFT()`, `fftMixedRadix()` | `four2a.f90` (FFTW wrapper — algorithm is standard mixed-radix Cooley-Tukey) | ⚠️ Ported (float64 — Fortran uses float32 FFTW `sfftw_*`; see item 20) |
| 12 | `realfft.go` | `RealFFT()` | N/A (optimized N/2-point trick) | ✅ Already local (uses local `FFT()`) |

### Key porting notes

- **`fft32()`** — A self-contained 32-point radix-2 FFT was written in `metrics.go`
  specifically for the symbol spectra computation (matching `four2a(csymb,32,1,-1,1)`).
  This avoids depending on the general-purpose FFT router for the hot inner loop.

- **`fft.go`** — Recursive mixed-radix Cooley-Tukey FFT with radix-2, radix-3, and
  radix-5 butterflies. All FT8 FFT sizes (192000, 180000, 3200, 1920) are 5-smooth.
  `IFFT()` uses the conjugate trick (conj → FFT → conj → scale). The Fortran
  `four2a.f90` is a thin FFTW wrapper — it calls `sfftw_*` (single-precision/float32
  FFTW). Our Go FFT computes in float64, causing ~5% divergence in spectrogram bin
  magnitudes. A CGO wrapper around `libfftw3f` for the spectrogram path is planned
  (see item 20 in Known Issues).

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

**WSJT-X reference corrected 2026-04-12:** Cap 2 had 2 phantom signals (CQ HA1BF,
UA4CCH VK2VT) that WSJT-X did NOT decode — our pipeline decodes them as bonus.
Cap 2 WSJT-X reference is 15 signals, not 17. All DTs corrected to WSJT-X ALL.TXT
display DT values (previously some were 1.800 placeholders). Recording station: 7Q5MLV
(monitoring only, not transmitting — AP types 2+ cannot help).

**Research pipeline decode counts:**

| Capture | Research | WSJT-X ref | Bonus (we decode, WSJT-X doesn't) |
|---|---|---|---|
| Cap 1 | 7/13 | 13 | 0 |
| Cap 2 | 8/15 | 15 | 2 (CQ HA1BF, UA4CCH VK2VT) |
| Cap 3 | 16/21 | 21 | 0 |

**Pipeline verification (2026-04-12):** Compiled and ran a Fortran reference program
(`research/fortran_test/dump_bmet.f90`) that calls the exact WSJT-X ft8b pipeline on
the RA6ABC signal (1814 Hz, Cap 1). **Result: Go and Fortran produce bit-identical
soft metrics** (all 4 bmet variants match to 4+ decimal places, cd0 matches, ibest/
nsync identical). The decode pipeline IS a faithful port. See also the detailed
sync8 and subtractft8 comparison below.

**Failure classification (corrected, 18 missing vs WSJT-X):**

| Failure mode | Count | Captures | Description |
|---|---|---|---|
| `subtraction_needed` | 4 | Cap1: 2, Cap2: 2 | Decode OK at exact params, missed by iterative candidate search |
| `ldpc_fail` | 4 | Cap1: 4 | Strong sync (nsync 10-16) but LDPC fails; wide grid search (±20Hz/±2s) also fails; likely depends on subtraction order from WSJT-X session context |
| `ldpc_fail + ap_limitation` | 4 | Cap2: 4 | Non-CQ, sync OK but LDPC fails; AP can't help (7Q5MLV not in messages) |
| `sync_fail + ap_limitation` | 3 | Cap2: 1, Cap3: 2 | nsync ≤ 6, non-CQ |
| `masked` | 1 | Cap3: 1 | CQ CO8LY blocked by nearby CQ 4S6ARW (6 Hz separation) |
| `sync_fail` (CQ) | 2 | Cap2: 1 (TN8GD), Cap3: 0 | CQ signal, sync8 candidate below syncmin threshold |

**Key findings:**

1. **Pipeline is faithful:** Go produces identical soft metrics to Fortran (verified
   by compiling and running Fortran reference). The decode gap is NOT in the pipeline
   algorithm — it's in candidate search coverage and iterative subtraction order.

2. **AP is not the gap:** The recording station (7Q5MLV) was monitoring only. None of
   the missing signals contain 7Q5MLV. AP types 2-6 cannot help for these captures.
   The `ap_limitation` label was misleading — WSJT-X also cannot use AP for these.

3. **4 signals are recoverable (`subtraction_needed`):** RA1OHX, WB9VGJ (Cap 1),
   UY7VV (Cap 2), TN8GD (Cap 2) decode at exact params but sync8 doesn't produce
   the right candidates. Fixing candidate coverage is the clear next step.

4. **4 Cap 1 signals depend on session context:** RA6ABC, RV6ASU, UA3LAR fail at
   ALL (freq, DT) in ±20Hz/±2s grid on raw audio. Both Go and Fortran produce the
   same (failing) LLR values. WSJT-X likely decoded these through a different
   subtraction order or session-specific context we can't reproduce from WAV files.

5. **CGO FFTW provides 43× speedup** on the spectrogram FFT (sync8 hot path).
   Overall decode time for 3 passes is ~4.5s, well within the 15s FT8 window.

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
The research package is **fully self-contained** — zero production `ft8x` imports.
All 17 library files are direct Fortran ports or standard algorithm implementations.

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
| `ap.go` | ~190 | `ft8b.f90:300–401`, `ft8apset.f90` | `ApplyAP()`, `ComputeAPSymbols()`, mcq/mrrr/m73/mrr73 constants (ncontest=0 only) |
| `pack28.go` | ~170 | `packjt77.f90:621–751, 64–79`, `q65_set_list.f90:66–97` | `pack28()` — callsign→28-bit encoding; `ihashcall()` — hash for non-standard calls; `stdcall()` — standard callsign check |
| `encode.go` | ~130 | `genft8.f90`, `gen_ft8wave.f90`, `gfsk_pulse.f90` | `GenFT8Tones()`, `GenFT8CWave()`, `gfskPulse()` |
| `subtract.go` | ~130 | `subtractft8.f90` | `SubtractFT8()` (FFT-based LPF method, `lrefinedt=false` path) |
| `decode.go` | ~300 | `ft8b.f90`, `ft8_decode.f90` | `DecodeSingle()`, `DecodeIterative()`, `Sync8FindCandidates()`, `DecodeParams`, `DecodeCandidate`, `CandidateFreq` |
| `fft.go` | ~180 | N/A (standard algorithm) | `FFT()`, `IFFT()`, `fftMixedRadix()`, `smallestFactor()` — recursive mixed-radix Cooley-Tukey for 5-smooth sizes |

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
| `pack28_test.go` | ~340 | pack28/ihashcall/stdcall/ComputeAPSymbols tests with round-trip verification |
| `twid_bench_test.go` | — | Twiddle factor benchmark |

---

## What to do next

### Porting complete — all stubs eliminated

All 16 research library files have been ported from Fortran (or are standard
algorithm implementations). The research package has **zero production `ft8x`
imports**. Porting completed 2026-04-12:

1. ~~`sync_d.go` — `Sync8d`, `HardSync`~~ ✅
2. ~~`metrics.go` — `ComputeSymbolSpectra`, `ComputeSoftMetrics`~~ ✅
3. ~~`downsample.go` — `Downsampler`, `TwkFreq1()`~~ ✅
4. ~~`ldpc.go` + `crc.go` — `Decode174_91()`, `BPDecode()`, `OSDDecode()`, CRC-14~~ ✅
5. ~~`ldpc_parity.go` — `LDPCMn`, `LDPCNm`, `LDPCNrw`, generator hex~~ ✅
6. ~~`message.go` — `Unpack77()`, `BitsToC77()`~~ ✅
7. ~~`ap.go` — `ApplyAP()`~~ ✅
8. ~~`encode.go` — `GenFT8Tones()`, `GenFT8CWave()`, `gfskPulse()`~~ ✅
9. ~~`subtract.go` — `SubtractFT8()`~~ ✅
10. ~~`decode.go` — `DecodeSingle()`, `DecodeIterative()`, types~~ ✅
11. ~~`fft.go` — `FFT()`, `IFFT()`, mixed-radix Cooley-Tukey~~ ✅

### Timing budget (2026-04-12, with CGO FFTW spectrogram)

| Component | Time | Notes |
|---|---|---|
| Sync8 (FFTW spectrogram) | 32 ms/pass | 43× faster than pure Go |
| 192k downsample FFT | 31 ms (once/pass) | Pure Go; could move to CGO FFTW (~1ms) |
| DecodeSingle (ndepth=2) | 4.5 ms/candidate | BP only |
| DecodeSingle (ndepth=3) | 7.0 ms/candidate | BP + OSD |
| **Total 3-pass decode** | **~4.5 s** | 260 candidates/pass, well within 15s FT8 window |

Headroom: ~10s available for improvements (retries, wider search, etc.)

### What to do next

**Priority 1: Fix Go OSD decoder — remaining sort precision issue in `osdDecode`.**

The OSD bug has been partially fixed and narrowed to a float32/float64 precision
issue in the reliability sort within `osdDecode` itself. Details below.

**What has been fixed (2026-04-12 current session, committed to working tree):**

1. **`argsortAsc` rewritten** — The previous implementation was a standard quicksort
   with median-of-three pivot, NOT a port of Fortran's `indexx`. This caused
   different tie-breaking for the 42 tied groups in typical LLR arrays. The function
   has been rewritten as an exact port of `wsjt-wsjtx/lib/indexx.f90` (Numerical
   Recipes quicksort with insertion sort for small partitions, M=7, NSTACK=50).
   Verified: standalone sort order now matches Fortran 0/174 positions.

2. **Float32 truncation added to `osdDecode`** — Two changes in `research/ldpc.go`:
   - `rx[i] = float64(float32(llr[i]))` — truncates incoming float64 LLR values to
     float32 precision, matching Fortran's `real rx(N)`.
   - `absrx[i] = float64(float32(math.Abs(rx[i])))` — truncates absolute values to
     float32 before sorting, matching Fortran's `real absrx(N)`.

3. **Pre-test bypass removed** — Go line 540 had `|| (iorder == 1 && n1 == iflag)`
   which is not present in Fortran line 206. Removed to match Fortran exactly.

4. **FFTW 32-point c2c forward plan added** — `fftw_wrapper.c` now has
   `fftw_c2c_32_forward()` and `fftw.go` has `FFT32Forward()`, matching the Fortran
   `four2a(csymb,32,1,-1,1)` call for symbol spectra. Not currently used in the
   main pipeline but available for testing.

**What is NOT yet fixed — the remaining divergence:**

The standalone sort test (`TestSortOrderDump`) confirms Go's `argsortAsc` matches
Fortran's `indexx` exactly (0/174 differences). However, when `osdDecode` runs
inside `Decode174_91` with real pipeline LLR values (which arrive as float64 from
`ScaleFac * bmet[i]`), the `genmrb` matrix after column reordering still differs
from Fortran at rows 89-91 (the last 3 of 91 rows). This causes:

- Order-0: nhardmin=42 (Go) vs 36 (Fortran)
- Order-1: nhardmin=26 (Go) vs 24 (Fortran), passed=2 (matches)
- CRC check fails in Go, passes in Fortran

The root cause of the remaining 3-row difference: the LLR values entering
`osdDecode` are computed as `ScaleFac * float64(bmet[i])` in `DecodeSingle`
(float64 multiplication), while Fortran computes `scalefac * bmet(i)` (float32
multiplication). Even though `osdDecode` truncates `rx` to float32, the
multiplication precision has already produced different float64 values. At the
42 tied positions, `float32(abs(float64_llr))` can produce different float32
values than `abs(float32_llr)` because the float64 intermediate is not the
same as the float32 intermediate.

**Next step to fix:** Change the LLR computation in `DecodeSingle` (or at the
`osdDecode` entry point) to use float32 multiplication: `float64(float32(ScaleFac) *
float32(bmet[i]))` instead of `ScaleFac * float64(bmet[i])`. This ensures the
`rx` values in `osdDecode` are bit-identical to Fortran's. Alternatively, add
float32 truncation at the `Decode174_91` entry point before passing to `osdDecode`.

**Test infrastructure (in working tree):**

| File | Purpose |
|---|---|
| `research/pass1_compare_test.go` | Compares Go vs Fortran pass 1 decode counts; includes LLR comparison and sort order verification |
| `research/osd_binary_test.go` | Feeds bit-exact Fortran float32 bmet values into Go's `Decode174_91` |
| `research/osd_trace_test.go` | Step-by-step OSD trace matching `/tmp/dump_osd_trace_bin`; dumps pre/post-GE genmrb for diff |
| `research/gen_compare_test.go` | Verifies Go and Fortran generator matrices are identical |
| `bmet_cand9.bin` (repo root) | Fortran binary float32 bmet arrays (4×174×float32 = 2784 bytes) |
| `llr_cand9.txt` (repo root) | Fortran text LLR values (E20.12 precision) |

**Fortran trace programs (source in `research/fortran_test/`, compile in `/tmp/`):**

These are our own diagnostic programs (not GPL code). They link against WSJT-X
subroutines at compile time, so compiled binaries must NOT be committed or
distributed. Source files are safe to commit — they contain only subroutine
calls (API usage), not copied GPL source.

| Source | Purpose |
|---|---|
| `dump_pass1.f90` | Full pass 1 decode (8 signals on Cap 1) |
| `dump_llr.f90` | LLR text dump + OSD decode for candidate 9 |
| `dump_llr_bin.f90` | Binary bmet/cd0 dump for candidate 9 (generates `bmet_cand9.bin`) |
| `dump_osd_trace.f90` | OSD step-by-step trace (reads text LLR file) |
| `dump_osd_trace_bin.f90` | OSD step-by-step trace (reads binary bmet — key diagnostic) |
| `dump_sync8.f90` | Sync8 candidate list dump |
| `dump_osd_order.f90` | OSD reliability ordering dump |
| `dump_gen.f90` | Generator matrix dump |

**Regenerating test artifacts (needed after fresh clone or reboot):**

The binary test artifacts (`bmet_cand9.bin`, `llr_cand9.txt`) are gitignored.
Go tests skip gracefully when they're missing. To regenerate:

```bash
# 1. Compile dump_llr_bin (generates bmet_cand9.bin)
cd /tmp
sed "s|~/Development|$HOME/Development|g" \
  ~/Development/go-ft8/research/fortran_test/dump_llr_bin.f90 > tmp.f90
gfortran -O2 -o dump_llr_bin tmp.f90 \
  <full link list from compile recipe below>
cd ~/Development/go-ft8
/tmp/dump_llr_bin testdata/ft8test_capture_20260410.wav

# 2. Compile dump_llr (generates llr_cand9.txt with E20.12 precision)
#    Modify the format string in dump_llr.f90 from F12.6 to E20.12 first,
#    or use the existing llr_cand9.txt if precision is sufficient.
```

**Compile recipe (all programs):**
```
cd /tmp
sed "s|~/Development|$HOME/Development|g" <source.f90> > tmp.f90
gfortran -O2 -o <binary> tmp.f90 \
  ~/Development/wsjt-wsjtx/lib/crc.f90 \
  ~/Development/wsjt-wsjtx/lib/fftw3mod.f90 \
  ~/Development/wsjt-wsjtx/lib/four2a.f90 \
  ~/Development/wsjt-wsjtx/lib/ft8/sync8.f90 \
  ~/Development/wsjt-wsjtx/lib/ft8/sync8d.f90 \
  ~/Development/wsjt-wsjtx/lib/ft8/ft8_downsample.f90 \
  ~/Development/wsjt-wsjtx/lib/ft8/twkfreq1.f90 \
  ~/Development/wsjt-wsjtx/lib/ft8/get_spectrum_baseline.f90 \
  ~/Development/wsjt-wsjtx/lib/ft8/baseline.f90 \
  ~/Development/wsjt-wsjtx/lib/nuttal_window.f90 \
  ~/Development/wsjt-wsjtx/lib/ft8/decode174_91.f90 \
  ~/Development/wsjt-wsjtx/lib/ft8/osd174_91.f90 \
  ~/Development/wsjt-wsjtx/lib/ft8/encode174_91_nocrc.f90 \
  ~/Development/wsjt-wsjtx/lib/ft8/get_crc14.f90 \
  ~/Development/wsjt-wsjtx/lib/indexx.f90 \
  ~/Development/wsjt-wsjtx/lib/platanh.f90 \
  ~/Development/wsjt-wsjtx/lib/pctile.f90 \
  ~/Development/wsjt-wsjtx/lib/polyfit.f90 \
  ~/Development/wsjt-wsjtx/lib/shell.f90 \
  ~/Development/wsjt-wsjtx/lib/determ.f90 \
  -lfftw3f -lm
```
Note: `g++` is not installed; `crc14.cpp` (Boost) can't compile. Use
`get_crc14.f90` + `crc.f90` (pure Fortran CRC) instead. The `.mod` files
generated by gfortran must NOT be in `research/fortran_test/` or they break
`go build` — compile in `/tmp`. Some programs (like `dump_osd_trace_bin`)
don't need the full sync8/downsample chain — use the shorter link list.

**Verified facts (do not re-investigate):**

- Generator matrices are identical between Go and Fortran (verified by
  `TestGeneratorCompare` — both parity-matrix-based and encode-based match).
- Sort algorithm matches Fortran `indexx` exactly (verified by
  `TestSortOrderDump` — 0/174 position differences).
- LLR values match to ~1e-6 across all 174 positions and all 4 bmet arrays.
- Float32 metrics (`ComputeSoftMetrics`, `ComputeSymbolSpectra`) do NOT
  improve decode counts — the issue is entirely in the OSD decoder.
- The pre-test bypass (`|| (iorder == 1 && n1 == iflag)`) is not the cause.
- Float32 distance metrics (`dd`, `dmin` as float32) in OSD are not the cause.

**Priority 2: Measure impact across all captures.**

After OSD fix, the 3 recovered signals on pass 1 get subtracted, potentially
enabling more signals on passes 2-3. Expect Cap 1: 7→10+.

**Priority 3: Performance.**

Move 192k downsampler FFT to CGO FFTW (~30ms/pass saved). Current decode time
~4.5s is within the 15s budget but leaves less headroom.

### Phase 1 success criteria (from assessment §5.5)

| Metric | Current | Target |
|---|---|---|
| Capture 1 correct | 7/13 (research) | ≥ 11/13 |
| Capture 2 correct | 8/15 + 2 bonus (research) | ≥ 12/15 |
| Capture 3 correct | 16/21 (research) | ≥ 18/21 |
| False decode rate | 0 | ≤ 1 per capture |
| Full decode cycle | ~4.5 s (with FFTW) | < 2 s per capture |

---

## Known issues and decisions

1. **Production is frozen** — No changes to production `ft8x` package until a research change is proven across all 3 captures. All experimental work in `research/`.

2. **`SubtractFT8` in research vs production** — Production has two subtraction
   variants (`SubtractFT8` per-symbol and `SubtractFT8FFT` FFT-based). The research
   package has a single `SubtractFT8` ported faithfully from Fortran `subtractft8.f90`
   (FFT-based LPF method, `lrefinedt=false` path). The production per-symbol variant
   has no Fortran equivalent.

3. **Research pipeline is currently ≤ production** — The research iterative pipeline (root_cause_all_test.go) gets 7/10/16 vs production's 8/11/16. This must be resolved before research improvements can be meaningfully measured.

4. **OSD depth: ndeep=2 everywhere** — After tracing the Fortran code, `ft8b.f90` line 405 hardcodes `norder=2` regardless of `ndepth`. This means Go's `ndeepD2=2` and `ndeepD3=2` are both correct. The Fortran `ndepth` only controls `maxosd` (0, 1, or 2 OSD attempts per candidate). `Depth=2` → `maxosd=1`, `Depth=3` → `maxosd=2`.

5. **Sync8 own-candidates decode gap** — Sync8 finds candidates near all reference signals, but decode success is limited by decoder sensitivity. The iterative subtraction loop improved counts significantly (Capture 3: 14→16). Further improvement requires deeper OSD or AP decoding.

6. **Mixed-radix FFT memory** — The recursive implementation allocates O(n × log n) temporary memory. An iterative Stockham approach would reduce this to O(n). Acceptable for Phase 1; optimize in Phase 2 if profiling shows GC pressure.

7. **Package name `ft8x`** — Inherited from the goft8 baseline. Intentionally kept to avoid import collision with any future `ft8` package.

8. **WAV test files** — Seven WAV files in `testdata/`. The three primary captures (`ft8test_capture_20260410.wav`, `ft8test_capture2_20260410.wav`, `capture.wav`) are tracked. Tests skip gracefully if files are missing.

9. **platanh: math.Atanh vs Fortran piecewise** — Retained `math.Atanh` with ±19.07 clamping. The Fortran piecewise-linear `platanh` caused a 1-decode regression because the BP decoder's LLR scaling (`ScaleFac=2.83`) was tuned with `math.Atanh`. May revisit when ScaleFac is re-tuned.

10. **OSD ntheta pre-test bypass removed** — The Go code had an extra condition
    `|| (iorder == 1 && n1 == iflag)` on the pre-test that is NOT in the Fortran
    (osd174_91.f90 line 206). This was removed (2026-04-12) to match Fortran exactly.
    The pre-test is now enforced uniformly for all patterns.

11. **AP decoding: ncontest=0, types 1-2 active** — Standard QSO mode only. AP type 1
    (CQ) and type 2 (MyCall) are implemented. Types 3-6 (MyCall+DxCall, +RRR/73/RR73)
    are structurally wired but require `DxCall` to be set in `DecodeParams`. `pack28`,
    `ihashcall`, `stdcall`, and `ComputeAPSymbols` are ported from Fortran
    (`packjt77.f90`, `ft8apset.f90`, `q65_set_list.f90`). Contest modes can be added
    as a future enhancement.

12. **AP frequency guard** — AP types ≥3 are restricted to candidates within `APWidth` Hz of `NfQSO`. When `NfQSO=0` (default), the frequency guard is disabled.

13. **xdt offset convention** — `DecodeSingle` computes `xdt = (ibest-1)*Dt2` where ibest is 0-based in Go, introducing a systematic -0.005s offset vs the Fortran. This only affects display DT and subtraction alignment (60 samples ≈ 3% of a symbol).

14. **`DecodeIterative` pass structure** — The research implementation faithfully ports
    `ft8_decode.f90` lines 160–239:
    - `npass=3` for `ndepth≥2`, `npass=2` for `ndepth=1`
    - Pass 0: lighter OSD (`ndeep = min(ndepth, 2)`)
    - Passes 1+: full `ndepth`
    - `syncmin=1.6` for `ndepth≤2`, `syncmin=1.3` otherwise
    - Up to 600 candidates per pass (`MAXCAND=600`)
    - Early termination when a pass produces no new decodes

15. **Research package fully decoupled** — As of 2026-04-12, all 17 research
    library files have been ported from Fortran or are self-contained. Zero
    production `ft8x` imports remain. The package is ready for independent
    development and measurement.

16. **`decode.go` ported — types now local** — `DecodeParams`, `DecodeCandidate`, and
    `CandidateFreq` were previously type aliases to production `ft8x.*`. They are now
    defined locally in research `decode.go` with identical fields. `CandidateFreq` is a
    type alias for `Candidate` (from `sync8.go`) since they have identical fields.
    `ResetProdDS` and `prodDS` (production scaffolding) were removed. The SNR computation
    uses tone-ratio only (no `xbase` from `sbase`, since `s8` is local to `DecodeSingle`
    and not available in `DecodeIterative`). AP types 1-6 are structurally wired;
    `ComputeAPSymbols` computes the ±1 `apsym` array from `params.MyCall`/`params.DxCall`
    via `pack28`. Guards skip iaptype≥2 if mycall unknown, iaptype≥3 if dxcall unknown.

17. **`fft.go` + CGO FFTW** — The Fortran `four2a.f90` wraps single-precision FFTW
    (`sfftw_*`). The research package now uses CGO FFTW (`fftw.go` + `fftw_wrapper.c`)
    for the spectrogram 3840-point r2c FFT, providing a 43× speedup. Testing showed
    that float32 vs float64 precision has **no impact on decode results** — both
    produce identical soft metrics (verified with compiled Fortran reference). The
    FFTW path is retained for performance (32ms vs ~1.4s per sync8 pass). The pure-Go
    `FFT`/`IFFT` remain for non-spectrogram paths. A float32 LDPC decoder variant
    (`ldpc_f32.go`) was also tested and confirmed to produce identical results.

18. **`decode.go` review fixes (2026-04-12)** — Line-by-line comparison against
    `ft8b.f90` and `ft8_decode.f90` found and fixed:
    - **maxosd for ndepth=3**: was incorrectly set to 0 (should be 2). The Fortran
      uses sequential `if` statements, not if-else — `maxosd=2` is the default and
      stays at 2 for ndepth=3 unconditionally. This was the Cap 2 regression cause
      (9→10 recovered).
    - **Early termination**: `prevPassDecodes` tracking was broken; pass 3 never
      properly checked if pass 2 added decodes.
    - **DT -0.5 display adjustment**: `ft8_decode.f90:210` subtracts 0.5 from xdt
      after ft8b returns (display only, not subtraction). Added to DecodeIterative.
    - **sbase retained**: Sync8 `sbase` is now kept for future xbase-based SNR.

19. **`SubtractFT8FFT` removed** — The production codebase had two subtraction
    variants (`SubtractFT8` per-symbol and `SubtractFT8FFT` FFT-based). The Fortran
    source (`subtractft8.f90`) contains only the FFT-based method. The research
    package now has a single `SubtractFT8` ported faithfully from Fortran. The
    `SubtractFT8FFT` name was removed; all test callers updated to use `SubtractFT8`.

20. **FFT precision: thoroughly investigated, NOT the cause** —
    Exhaustive testing (2026-04-12) showed float32 vs float64 FFT precision has
    **zero impact on decode results**:

    - CGO FFTW float32 spectrogram (3840-point): identical decode counts
    - CGO FFTW float32 downsampler (192k-point): relRMS 1.5e-7, identical decodes
    - Float32 LDPC decoder: identical decode counts
    - Compiled Fortran reference: produces identical soft metrics to Go

    The bin 144 TN8GD case (normalized sync 1.23 vs syncmin 1.3) is a property of
    the signal, not FFT precision — both Go and Fortran produce the same value.
    TN8GD is classified as `subtraction_needed` (decodable at exact params, missed
    by candidate search).

    CGO FFTW is retained in the spectrogram hot path for **performance** (43× faster),
    not for precision. Additional FFTW plans for 192k r2c and 3200 c2c backward are
    available in `fftw_wrapper.c` for future performance optimization.

21. **Root cause confirmed: OSD decoder bug in `research/ldpc.go`** —

    **2026-04-12 (previous session) hypothesis:** Float32 rounding in
    `ComputeSoftMetrics` was suspected. This was **WRONG**.

    **2026-04-12 (current session) — definitive proof and partial fix:**

    Fed bit-exact Fortran float32 bmet values into Go's `Decode174_91` → ALL
    4 passes FAIL. The Fortran OSD decodes with nhard=24 on the same values.
    **This proves the bug is in Go's `osdDecode`, NOT in LLR precision.**

    The `argsortAsc` function was NOT a faithful port of `indexx.f90` — it was
    a completely different quicksort. Rewritten as exact port. Float32 truncation
    of `rx` and `absrx` added. Pre-test bypass removed. Details, test artifacts,
    compile recipes, and remaining work are in the "Priority 1" section above.

22. **sync8 and subtractft8 comparison** — Line-by-line comparison found no
    algorithmic differences affecting real signals:
    - sync8: candidates match (250 Fortran vs 261 Go, same top candidates in
      same order; 11 extra Go candidates are wide-peak near-dupes from float32
      tie-breaking — harmless)
    - sync8: division-by-zero guard in sync2d (Go returns 0, Fortran produces Inf
      for degenerate inputs) — no practical impact
    - sync8: normalization scope (Go normalizes [ia..ib] only, Fortran normalizes
      entire array) — no downstream impact since only [ia..ib] is accessed
    - subtractft8: algebraically equivalent FFT normalization path
    - Iterative loop: identical pass structure, newdat caching, subtraction timing
    - `indexx` sort: ported Numerical Recipes quicksort to match Fortran tie-breaking

23. **Recording station 7Q5MLV** — All 3 captures were recorded by 7Q5MLV
    (monitoring only, not transmitting). AP types 2-6 cannot help because none
    of the missing signals contain 7Q5MLV as call1 or call2.

24. **WSJT-X license: GPLv3** — Cannot link or include Fortran source via CGO.
    All code must be clean-room Go reimplementation of the same algorithms.

---

## Reference codebase locations

| Path | Language | Use for |
|---|---|---|
| `~/Development/wsjt-wsjtx/` | Fortran | **SOLE SOURCE OF TRUTH** — `lib/ft8/ft8b.f90`, `sync8.f90`, `sync8d.f90`, `ft8_downsample.f90`, `decode174_91.f90`, `osd174_91.f90`, `subtractft8.f90`, `twkfreq1.f90` |
| `~/Development/ft8_lib/` | C | Secondary reference — `ft8/decode.c`, `ft8/encode.c` |
| `~/Development/jtdx/` | Fortran | WSJT-X fork with OSD/AP optimisations |
| `~/Development/goft8/ft8/` | Go | The original ft8x baseline (historical only — do NOT use as reference) |
