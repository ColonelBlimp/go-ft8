# FT8 Go Library — Project Assessment

**Date:** 2026-04-10
**Purpose:** Assessment document for a new standalone Go FT8 decoding library, intended to match or exceed WSJT-X decode performance. This document compares the two existing codebases in station-manager (`internal/ft8/` and `internal/ft8x/`), evaluates their strengths and weaknesses, and recommends the starting point for the new library.

---

## 1. Executive Summary

Station-manager contains **two independent FT8 decoder implementations** in Go:

| | `internal/ft8/` (modular) | `internal/ft8x/` (monolithic) |
|---|---|---|
| **Architecture** | Multi-package: `codec/`, `dsp/`, `message/`, `synth/`, `timing/`, `service/` | Single flat package, ~2,500 lines total |
| **Origin** | Evolved organically; DSP started from Goertzel, later added WSJT-X baseband port | Direct port of WSJT-X Fortran `ft8b.f90`, `decode174_91.f90`, `packjt77.f90` |
| **Decode rate (capture 1, 13 ref)** | 10/13 correct, 1 false (with AP) | 7/13 correct, 0 false (provided candidates); 9/13 with own candidates |
| **Decode rate (capture 2, 15 ref)** | 8/15 correct, 2 false | 9/15 correct, 0 false (provided candidates) |
| **False alarm rate** | Higher (1–3 per capture) | Lower (0 with provided candidates) |
| **AP decoding** | ✅ Implemented (6 AP types, QSO progress states) | ❌ Stubbed only |
| **Signal subtraction** | ✅ Multi-pass iterative | ✅ Basic (waveform reconstruction) |
| **Candidate detection** | ✅ Sync8 (WSJT-X faithful) | ✅ Brute-force freq/DT grid scan |
| **OSD depth** | Order-2 with zsave chain | Order-1 (no zsave chain in OSD path) |
| **Message types** | Type 0 (free text), 1, 2, 4 | Types 0.0, 0.1, 0.3/0.4, 0.5, 0.6, 1, 2, 3, 4, 5 |
| **Dependencies** | `internal/errors`, `internal/logging`, `internal/config` via station-manager | Zero external deps (stdlib only) |
| **Test coverage** | Extensive unit + integration + pipeline stage tests | Unit tests + WAV integration tests |
| **Lines of code** | ~8,000+ across 40+ files | ~2,500 across 13 files |

**Recommendation:** Use `ft8x` as the foundation for the new library. It is closer to WSJT-X's structure, has zero external dependencies, lower false alarm rate, broader message type coverage, and is simpler to understand and maintain. Selectively port the superior components from `ft8/` (AP decoding, sync8 candidates, OSD-2 with zsave, mixed-radix FFT).

---

## 2. Detailed Codebase Comparison

### 2.1 DSP Pipeline

#### ft8/ (modular)
- **Two pipelines**: Original Goertzel-based (`ProcessWindow`) and WSJT-X baseband (`ProcessWindowBaseband`).
- Baseband pipeline: `LongFFT` (192k mixed-radix) → `DownsampleBaseband` → `Sync8d` fine sync → 32-pt DFT per symbol → multi-pass LLR extraction → LDPC decode.
- Mixed-radix FFT (`fft_mixedradix.go`): supports 5-smooth sizes via Cooley-Tukey radix-2/3/5 butterflies. 1.29× faster than Bluestein for 192k points with 63% less memory.
- Sync8 candidate detection (`sync8.go`): linear-power spectrogram, ratio-metric scoring, dual sync modes (abc + bc), 40th-percentile normalisation, near-dupe suppression. Faithful port of WSJT-X `sync8.f90`.
- Signal subtraction: iterative multi-pass — decoded signals are subtracted from the audio, then the pipeline re-runs with fresh candidates.
- Post-decode SNR: matches WSJT-X `ft8b.f90` lines 438–452.
- **Known issues**: DC bin inclusion bug was fixed; NP2 bounds were corrected; Goertzel refinement was removed. These fixes are critical and must be carried forward.

#### ft8x/ (monolithic)
- **Single pipeline**: Direct port of `ft8b.f90` → `Downsample` → `Sync8d` → `ComputeSymbolSpectra` → `ComputeSoftMetrics` → `Decode174_91`.
- FFT: Bluestein chirp-Z for arbitrary sizes + radix-2 for power-of-2. Correct but slower for 192k (Bluestein requires a 524,288-point FFT internally).
- Candidate detection: brute-force grid scan over freq/DT — O(n²) but functional. No sync8-style scoring.
- Signal subtraction: basic waveform reconstruction (`SubtractFT8`).
- Downsampler correctly caches the 192k FFT and reuses across candidates.
- **DC bin fix already applied**: `ib < 1` check matches WSJT-X. NP2=2812 constant present and used correctly.
- Frequency correction via `TwkFreq1` (Legendre polynomial frequency tweaking) — port of `twkfreq1.f90`. The ft8/ pipeline doesn't have this; it re-downsamples instead.

**Assessment**: Both pipelines follow the same WSJT-X demodulation flow. ft8x is closer to the Fortran line-for-line, making it easier to cross-reference and debug. ft8/ has better candidate detection (sync8) and FFT performance (mixed-radix). The new library should combine ft8x's decode core with ft8/'s sync8 and mixed-radix FFT.

### 2.2 LDPC Decoder

#### ft8/codec/
- **Sum-product BP** with tanh/atanh (matches WSJT-X `decode174_91.f90`).
- **OSD order-2**: 91 single-bit flips + 4,095 pair flips.
- **zsave chain**: BP collects cumulative posterior LLR snapshots at iterations 1–3; OSD tries zsave[0], zsave[1] in sequence.
- `platanh()`: piecewise-linear atanh matching WSJT-X `platanh.f90`.
- AP support: `DecodeAP()` holds AP-masked bits at channel value during BP; `DecodeOSDAP()` skips flipping AP-masked bits.
- Multiple entry points: `DecodeMessage()`, `DecodeMessageShallow()` (OSD-1), `DecodeMessageAP()`, `DecodeMessageAPWithDepth()`.
- All-zero codeword rejection.
- Callsign plausibility filter (`PlausibleCallsign`, `PlausibleMessage`).

#### ft8x/
- **Sum-product BP** with tanh/atanh — essentially the same algorithm.
- `platanh()` present but uses a simpler clamp (±19.07 via `math.Atanh`) rather than the piecewise-linear approximation.
- **OSD order-1** only (no order-2 pair flips).
- **zsave chain**: `Decode174_91` accumulates zsave across BP iterations — same approach as WSJT-X.
- AP support: `apmask` parameter flows through BP and OSD, but no AP pass orchestration (just the plumbing).
- All-zero codeword rejection.
- Combined `Decode174_91` function matches WSJT-X's `decode174_91.f90` structure more closely than the split in ft8/codec/.

**Assessment**: ft8/codec/ has the stronger decoder (OSD-2, AP orchestration, plausibility filters). These components should be ported into the new library on top of ft8x's `Decode174_91` structure. The piecewise-linear `platanh` from ft8/ should replace the simpler version in ft8x.

### 2.3 Message Packing/Unpacking

#### ft8/message/
- Types supported: 0 (free text), 1 (standard), 4 (non-standard callsign).
- Clean separation: `type0.go`, `type1.go`, `type4.go`, `callsign.go`, `grid.go`, `freetext.go`, `crc14.go`, `validate.go`.
- Validation: `PlausibleCallsign()` verifies ITU callsign structure (letters + digits).
- Well-tested with cross-checked vectors from ft8_lib.

#### ft8x/
- **All message types**: 0.0 (free text), 0.1 (DXpedition), 0.3/0.4 (ARRL Field Day), 0.5 (telemetry), 0.6 (WSPR), 1/2 (standard/EU VHF), 3 (RTTY Roundup), 4 (non-standard), 5 (EU VHF contest).
- Single file (`message.go`) — 807 lines but complete coverage.
- Pack28/Unpack28 includes all special tokens (DE, QRZ, CQ, CQ_NNN, CQ_LLLL, hashed calls).
- ARRL section table, RTTY multiplier table — all present.
- 3DA0/3X callsign workarounds.

**Assessment**: ft8x has significantly broader message type coverage. The new library should use ft8x's `Unpack77` as the base. The validation functions from ft8/message/ (`PlausibleCallsign`, `PlausibleMessage`) should be added on top.

### 2.4 Test Infrastructure

#### ft8/
- Unit tests per component: FFT, spectrogram, candidates, decoder, encoder, CRC, messages.
- Integration tests: `dsp_wav_test.go`, `baseband_pipeline_test.go` with regression baselines.
- Stage-by-stage CLI (`cmd/ft8test/`): devices, capture, spectrogram, candidates, decode.
- Debug decoder (`decoder_debug.go`): per-iteration BP diagnostics.
- Cross-verification tests: constants verified against ft8_lib C reference.

#### ft8x/
- Unit tests: CRC, encode/decode round-trip, FFT, generator matrix, message unpacking.
- WAV integration tests: both captures, with provided-candidate and own-candidate variants.
- Single-candidate diagnostic test: tests each WSJT-X reference signal individually.
- Bluestein FFT accuracy test for 192k points.
- Inline WAV loader (no external deps).

**Assessment**: Both have solid test infrastructure. ft8x's WAV tests are self-contained (no external deps), which is better for a standalone library. The stage-by-stage CLI from ft8/ is valuable for debugging and should be created as a separate cmd in the new repo.

---

## 3. Performance Comparison

### Capture 1 (13 WSJT-X decodes)

| Signal | SNR (dB) | ft8/ baseband | ft8x provided | ft8x own candidates |
|---|---|---|---|---|
| SV2SIH ES2AJ -16 | -3 | ✅ | ✅ | ✅ |
| VE1WT K4GBI 73 | -11 | ✅ | ✅ | ✅ |
| SV2SIH KI8JP -10 | -6 | ✅ | ✅ | ✅ |
| CQ PV8AJ FJ92 | -12 | ✅ | ✅ | ✅ |
| <...> RA1OHX KP91 | -15 | ✅ | ✅ | ✅ |
| KB7THX WB9VGJ RR73 | -21 | ✅ (AP) | ✅ | ✅ |
| A61CK UA1CEI KP50 | -15 | ✅ | ✅ | ✅ |
| <...> LU3DXU GF05 | -15 | ❌ | ❌ | ❌ |
| <...> RA6ABC KN96 | -17 | ❌ | ❌ | ❌ |
| ES2AJ UA3LAR KO75 | -23 | ❌ | ❌ | ❌ |
| A61CK W3DQS -12 | -19 | ✅ | ✅ | ✅ |
| HZ1TT RU1AB R-10 | -21 | ✅ | ❌ | ✅ |
| <...> RV6ASU KN94 | -16 | ✅ | ❌ | ✅ |
| **False decodes** | | 1 | 0 | 0 |
| **Total correct** | | **10/13** | **7/13** | **9/13** |

**Key observation**: ft8x decodes KB7THX WB9VGJ RR73 (-21 dB) **without AP context** — ft8/ requires `--mycall KB7THX --dxcall WB9VGJ`. This suggests ft8x's decoder path may be slightly more sensitive for this particular signal. However, ft8x with provided candidates misses HZ1TT and RV6ASU that its own-candidates path finds — this is purely a candidate-quality issue.

### Capture 2 (15 WSJT-X decodes)

ft8x with provided candidates: **9/15 correct, 0 false** — better than ft8/'s 8/15 correct, 2 false.

The ft8x decoder achieves a better correct/false ratio on capture 2. This is likely because:
1. ft8x uses OSD order-1 (fewer false alarm paths than order-2)
2. ft8x has the `nsync <= 10 && xsnr < -24.0` rejection filter from WSJT-X
3. ft8/ added multiple mitigation layers (shallow OSD, AP CQ depth limits, plausibility filters) that still don't fully eliminate false alarms

---

## 4. Architecture Differences: Fortran vs Go

### Why WSJT-X (Fortran) decodes better

WSJT-X's Fortran implementation has several advantages:

1. **Mature, battle-tested code**: 20+ years of refinement by K1JT and contributors. Every threshold, scale factor, and heuristic has been tuned against thousands of real-world captures.

2. **Native Fortran FFT (`four2a`)**: Hand-optimised for the specific sizes used (192000, 3200, 32). No memory allocation overhead — uses pre-allocated arrays.

3. **Tight integration**: The Fortran code doesn't have package boundaries. `ft8b` can directly read `sync8` state, share arrays with `decode174_91`, and modify candidate lists in-place. No data copying between components.

4. **FFTW availability**: Production WSJT-X can use FFTW (Fastest Fourier Transform in the West) via Fortran interfaces. FFTW uses CPU-specific SIMD (SSE, AVX, AVX-512) and adaptive algorithm selection.

5. **Numerical precision**: Fortran's default `real*8` (64-bit float) is used throughout. Go's `float64` is equivalent, but the Fortran compiler may optimise fused multiply-add (FMA) instructions differently.

6. **Multi-threaded OSD**: WSJT-X can parallelise OSD-2 across CPU cores using OpenMP. The Go implementations are single-threaded per candidate.

### Why a Go port can potentially match or exceed WSJT-X

1. **Algorithm is the same**: The LDPC code, Costas sync, baseband demodulation, and OSD are mathematical algorithms. A faithful port should produce identical results to within floating-point rounding.

2. **CGo bridge for critical paths**: The new library can use C (not Fortran) for the most performance-critical inner loops — particularly the 192k-point FFT and OSD-2 pair-flip search. CGo overhead is ~100ns per call, negligible compared to the milliseconds spent in these routines.

3. **FFTW via CGo**: Go can call FFTW through CGo bindings, getting the same SIMD-optimised FFT as WSJT-X.

4. **Goroutine parallelism**: Go's goroutines are lighter than OpenMP threads. Candidate processing, OSD search, and multi-pass LLR extraction can all be parallelised with `sync.WaitGroup` or worker pools.

5. **Better memory management**: Go's garbage collector handles allocation/deallocation. Fortran's manual allocation in `ft8b` uses stack-allocated arrays — efficient but inflexible. Go can use `sync.Pool` for the hot-path allocations (FFT buffers, LLR arrays).

6. **Modern tooling**: Go's benchmark framework, profiler (`pprof`), and race detector make it easier to find and fix performance issues than Fortran's limited tooling.

### The fundamental bottleneck is LLR quality, not the decoder

Both codebases demonstrate the same pattern: signals that produce good LLRs (rawσ > 0.0008) decode reliably; signals with poor LLRs fail regardless of decoder sophistication. The remaining undecoded signals in both captures are limited by:

- **Candidate detection**: signals not found by sync8 (too weak for Costas correlation)
- **Fine frequency sync**: sub-bin frequency errors contaminate all LLR passes
- **Per-symbol DFT quality**: adjacent strong signals leak energy into weak signal bins

A C/asm FFT kernel won't fix these — they're algorithmic issues that require either better candidate detection (wider search, lower thresholds) or better interference mitigation (signal subtraction before re-processing).

---

## 5. Plan for the New Library

### 5.1 Repository Structure

```
go-ft8/
├── README.md
├── AGENTS.md
├── go.mod                  # module github.com/<user>/go-ft8
├── LICENSE
├── Taskfile.yml
│
├── ft8.go                  # Top-level API: Decode(audio, config) → []Message
├── params.go               # Constants (NSPS, NP2, NFFT2, etc.)
├── crc.go                  # CRC-14
├── encode.go               # LDPC encoder + tone generation
├── decode.go               # Top-level decode orchestration
├── downsample.go           # Baseband downsampling (ft8_downsample port)
├── sync.go                 # Sync8d fine sync + sync8 candidate detection
├── metrics.go              # Soft metric extraction (LLR computation)
├── ldpc.go                 # BP + OSD decoder (decode174_91)
├── ldpc_parity.go          # Parity check matrix data
├── message.go              # Pack/unpack 77-bit messages (all types)
├── fft.go                  # FFT (radix-2 + Bluestein + mixed-radix)
├── ap.go                   # A priori decoding context
├── validate.go             # Callsign plausibility, message validation
│
├── testdata/               # WAV captures for integration tests
│   ├── capture1.wav
│   └── capture2.wav
│
├── cmd/
│   └── ft8decode/          # CLI tool for testing/benchmarking
│       └── main.go
│
└── c/                      # Optional: C kernels for hot paths (Phase 2)
    ├── fft_fftw.c          # FFTW wrapper
    └── osd_simd.c          # SIMD-optimised OSD search
```

### 5.2 Phase 1: Pure Go (weeks 1–4)

1. **Copy ft8x files** as the starting point (zero station-manager deps).
2. **Replace logging/errors** with `log/slog` (Go stdlib structured logging) and `fmt.Errorf` with `%w` wrapping.
3. **Port sync8 candidate detection** from `internal/ft8/dsp/sync8.go`.
4. **Port mixed-radix FFT** from `internal/ft8/dsp/fft_mixedradix.go`.
5. **Upgrade OSD to order-2** with zsave chain from `internal/ft8/codec/osd.go`.
6. **Port AP decoding** from `internal/ft8/dsp/ap.go` and `internal/ft8/codec/decoder.go`.
7. **Add plausibility filters** from `internal/ft8/message/validate.go`.
8. **Port iterative signal subtraction** from `internal/ft8/dsp/baseband_pipeline.go`.
9. **Comprehensive testing**: both captures should match or exceed current best (10/13, 9/15).

### 5.3 Phase 2: C/SIMD Acceleration (weeks 5–8)

1. **FFTW integration** via CGo for the 192k-point FFT.
2. **SIMD OSD search**: the order-2 pair-flip inner loop (`91 × 90 / 2 = 4,095` candidates) is embarrassingly parallel and benefits from AVX2 bitwise operations.
3. **Benchmark** against WSJT-X using a large corpus of captures (not just 2).

### 5.4 Phase 3: Exceed WSJT-X (weeks 9+)

1. **Goroutine parallelism**: process candidates concurrently.
2. **Adaptive OSD depth**: use higher OSD order near the TX frequency (where signals are expected), lower elsewhere.
3. **Hash table for non-standard callsigns**: resolve `<...>` placeholders using a callsign database.
4. **FT4 support**: the (152,76) code shares most infrastructure.
5. **Additional test captures**: collect a diverse corpus covering all message types, SNR ranges, and band conditions.

### 5.5 Success Criteria

| Metric | Target | WSJT-X baseline |
|---|---|---|
| Capture 1 correct | ≥ 11/13 | 13/13 |
| Capture 2 correct | ≥ 12/15 | 15/15 |
| False decode rate | ≤ 1 per capture | 0 |
| 192k FFT time | < 5 ms | ~3 ms (FFTW) |
| Full decode cycle | < 2 s (15s audio) | ~1 s (WSJT-X) |
| Memory per decode | < 50 MB | ~30 MB |

---

## 6. Files to Copy from Station-Manager

### From `internal/ft8x/` (primary — copy all)
| File | Lines | Description |
|---|---|---|
| `params.go` | 51 | Constants (NSPS, NP2, NFFT2, etc.) |
| `crc.go` | 86 | CRC-14 computation and verification |
| `encode.go` | 56 | LDPC encoder + tone generation |
| `decode.go` | 397 | Top-level decode orchestration |
| `downsample.go` | 142 | Baseband downsampling |
| `sync.go` | 116 | Sync8d fine sync + HardSync |
| `metrics.go` | 212 | Soft metric extraction |
| `ldpc.go` | 681 | BP + OSD decoder |
| `ldpc_parity.go` | ~1,600 | Parity check matrix hex data |
| `message.go` | 807 | Message pack/unpack (all types) |
| `fft.go` | 178 | FFT (radix-2 + Bluestein) |
| `ft8_test.go` | 154 | Unit tests |
| `ft8x_wav_test.go` | 515 | WAV integration tests |

### From `internal/ft8/` (selective port)
| File | Lines | What to port |
|---|---|---|
| `dsp/sync8.go` | ~300 | Sync8 candidate detection algorithm |
| `dsp/fft_mixedradix.go` | ~400 | Mixed-radix FFT for 5-smooth sizes |
| `dsp/ap.go` | ~200 | AP context, pass tables, LLR injection |
| `dsp/baseband_pipeline.go` | ~350 | Signal subtraction, multi-pass orchestration |
| `codec/osd.go` | ~250 | OSD order-2 with pair-flip search |
| `codec/decoder.go` | ~300 | Sum-product BP with zsave, AP decode |
| `message/validate.go` | ~80 | Callsign plausibility filter |

### Do NOT copy
- `internal/errors/` — replace with `fmt.Errorf` / `errors.New` (stdlib)
- `internal/logging/` — replace with `log/slog`
- `internal/config/` — not needed for a library
- `internal/audio/` — not part of the decode library
- `internal/ft8/service/` — station-manager specific lifecycle
- `internal/ft8/timing/` — station-manager specific timing
- `internal/ft8/synth/` — TX synthesis (separate concern)

---

## 7. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Floating-point divergence from Fortran | Medium | High | Bit-exact comparison tests using WSJT-X diagnostic output |
| CGo overhead exceeds benefit | Low | Medium | Benchmark before/after; only use CGo for 192k FFT |
| FFTW licensing (GPL) | High | High | FFTW is GPL; use it only if the library is GPL-compatible, or use the pure-Go mixed-radix FFT |
| Test captures not representative | Medium | Medium | Collect captures across bands (80m–6m), times (day/night), and conditions (contest/DX) |
| Scope creep into TX/QSO state machine | Medium | Low | Strict scope: decode library only. TX and QSO state stay in station-manager |

---

## 8. Reference Codebases to Attach

As specified in the plan, these external codebases should be attached to the project for cross-reference:

1. **WSJT-X** (Fortran): https://sourceforge.net/p/wsjt/wsjtx/ci/master/tree/ — the gold standard. Key files: `lib/ft8/ft8b.f90`, `lib/ft8/decode174_91.f90`, `lib/ft8/sync8.f90`, `lib/ft8/ft8_downsample.f90`, `lib/ft8/osd174_91.f90`, `lib/77bit/packjt77.f90`.
2. **ft8_lib** (C): https://github.com/kgoba/ft8_lib — clean C reference with well-commented LDPC code.
3. **JTDX** (Fortran): https://github.com/jtdx-project/jtdx — fork of WSJT-X with additional decode optimisations (worth studying their OSD and AP modifications).

