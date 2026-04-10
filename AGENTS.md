# AGENTS.md — AI Agent Instructions for go-ft8

## Project Overview

`go-ft8` is a standalone Go library implementing the FT8 digital radio protocol decoder, ported from WSJT-X's Fortran source code. The module path is `github.com/ColonelBlimp/go-ft8` and it uses **Go 1.25** with zero external dependencies (stdlib only).

The package name is `ft8x`. All source files live at the repository root (flat layout, no sub-packages except `cmd/ft8decode`).

## Architecture

The decode pipeline follows WSJT-X's `ft8b.f90`:

```
Audio (15s @ 12000 Hz)
  → Sync8FindCandidates (spectrogram-based candidate detection)
  → for each candidate:
      Downsample (192k FFT → baseband at 200 Hz)
      → Sync8d (fine time/freq sync via Costas correlation)
      → ComputeSymbolSpectra (32-pt DFT per symbol)
      → ComputeSoftMetrics (4 LLR extraction passes)
      → Decode174_91 (BP + OSD LDPC decoder)
      → Unpack77 (message unpacking)
```

### Key files

| File | Purpose | Fortran origin |
|---|---|---|
| `params.go` | Constants (NSPS, NP2, NFFT sizes, etc.) | `ft8_params.f90` |
| `decode.go` | `Decode()`, `DecodeSingle()`, `FindCandidates()` | `ft8b.f90` |
| `sync8.go` | `Sync8FindCandidates()` — spectrogram candidate detection | `sync8.f90` |
| `sync.go` | `Sync8d()`, `BuildCtwk()`, `HardSync()` | `sync8d.f90` |
| `downsample.go` | `Downsampler`, `TwkFreq1()` | `ft8_downsample.f90`, `twkfreq1.f90` |
| `metrics.go` | `ComputeSymbolSpectra()`, `ComputeSoftMetrics()` | `ft8b.f90` lines 200–400 |
| `ldpc.go` | `Decode174_91()`, `BPDecode()`, `OSD174_91()` | `decode174_91.f90` |
| `ldpc_parity.go` | Parity check matrix hex data | `ldpc_174_91_c_parity.f90` |
| `message.go` | `Unpack77()`, `Pack28()`, all message types | `packjt77.f90` |
| `crc.go` | CRC-14 computation | — |
| `encode.go` | LDPC encoder, tone generation | — |
| `fft.go` | FFT routing: radix-2, mixed-radix, Bluestein | — |
| `fft_mixedradix.go` | Mixed-radix Cooley-Tukey for 5-smooth sizes | — |

### Important constants

- `Fs = 12000` Hz (audio sample rate)
- `NSPS = 1920` samples/symbol → `Baud = 6.25` Hz
- `NMAX = 180000` samples (15 seconds)
- `NFFT1DS = 192000` (downsampler FFT size, 5-smooth)
- `NFFT1 = 3840` (sync8 spectrogram FFT size, 5-smooth)
- `NFFT2 = 3200` (downsampled IFFT size, 5-smooth)
- `NP2 = 2812` (downsampled signal length)

## Coding Guidelines

### Fortran porting conventions

- All ported code includes a comment referencing the Fortran source file and line range (e.g., `// Port of subroutine sync8d from wsjt-wsjtx/lib/ft8/sync8d.f90`).
- Fortran 1-indexed arrays: when the Go code uses 1-indexed arrays to match Fortran, allocate with `n+1` elements and document that index 0 is unused.
- Fortran integer truncation: use `int(x)` (not `int(math.Round(x))`) when translating Fortran implicit integer assignment.
- The `sign` variable convention: `sign = -1.0` for forward FFT/DFT, `sign = +1.0` for inverse.

### FFT usage

- Power-of-2 sizes → `fftRadix2` (in-place)
- 5-smooth sizes (factors 2, 3, 5 only) → `fftMixedRadix` (in-place)
- Other sizes → `bluestein` (out-of-place)
- All three are routed automatically by `FFT()`, `IFFT()`, and `RealFFT()`.
- The key FT8 sizes (3840, 3200, 192000) are all 5-smooth and use the mixed-radix path.

### Testing

```bash
go test -short ./...        # unit tests only (~0.1s)
go test ./...               # full suite including WAV integration (~10s)
go test -run TestFt8xWAV    # WAV integration tests only
go test -bench BenchmarkFFT  # FFT benchmarks
```

- Test WAV files live in `testdata/`. The captures `ft8test_capture_20260410.wav` and `ft8test_capture2_20260410.wav` are the primary regression fixtures.
- Regression baselines: Capture 1 provided candidates ≥ 7/13 correct; Capture 2 ≥ 9/15 correct.
- Tests skip gracefully if WAV files are not found.

### Reference codebases

These are available in the workspace for cross-reference:

| Path | Language | Use for |
|---|---|---|
| `~/Development/wsjt-wsjtx/` | Fortran | Gold standard — `lib/ft8/ft8b.f90`, `sync8.f90`, `decode174_91.f90` |
| `~/Development/ft8_lib/` | C | Clean LDPC reference — `ft8/decode.c`, `ft8/encode.c` |
| `~/Development/jtdx/` | Fortran | WSJT-X fork with OSD/AP optimisations |
| `~/Development/goft8/ft8/` | Go | The original ft8x baseline these files were copied from |

### Style

- No external dependencies. Stdlib only.
- Package name is `ft8x` (not `ft8` — avoids collision with the old modular package).
- Exported functions use doc comments. Unexported helpers get a one-liner.
- Error handling: return `(result, bool)` not `(result, error)` for decode paths (matching Fortran's success/fail pattern).

## Do NOT

- Add dependencies outside the Go standard library.
- Change the package name from `ft8x`.
- Modify test regression baselines downward without explicit approval.
- Port TX/synthesis code — this is a decode-only library.
- Use CGo in Phase 1 — that is Phase 2 work.

