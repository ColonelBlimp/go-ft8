# go-ft8

Phase 1 reset of the FT8 decoder library using the `goft8` `ft8x` baseline.

## What is included

- Flat, standalone Go package at repo root (`package ft8x`)
- Core FT8 decode pipeline (downsample, sync, metrics, LDPC, message unpack)
- Unit tests and WAV integration tests
- `cmd/ft8decode` CLI for quick local decode runs

## Quick test

```bash
go test ./...
```

## Run the CLI

```bash
go run ./cmd/ft8decode -wav testdata/ft8test_capture_20260410.wav
```

Optional tuning flags:

```bash
go run ./cmd/ft8decode -wav testdata/ft8test_capture_20260410.wav -fmin 200 -fmax 3200 -dtmin -0.5 -dtmax 2.5 -max 60
```

