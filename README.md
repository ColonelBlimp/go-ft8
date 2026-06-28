# go-ft8

`go-ft8` is a Go implementation of an FT8 codec. It decodes one 15-second FT8
slot from 12 kHz mono-signed 16-bit PCM audio, encodes supported standard FT8
messages to protocol symbols, and exposes both a strict decoder path and
optional deeper experimental search modes.

This repository is intended for FT8 protocol experimentation, parity work,
profiling, and integration into larger Go applications that need FT8 message
recovery or standard-message encoding.

## Status

This is active research code. The default decoder path is designed to preserve
strict corpus parity behavior, while `DeepDecoderOptions` trades speed and
strictness for additional recall.

The implementation is not a clean-room project. It was developed through
WSJT-X/jt9 parity research, source-adjacent investigation, and behavioral
comparison against an installed `jt9 -8` decoder.

## Install

```sh
go get github.com/ColonelBlimp/go-ft8
```

Import the codec package:

```go
import "github.com/ColonelBlimp/go-ft8/ft8"
```

## Basic Usage

`DecodeMessages` expects a single FT8 receive slot as 12 kHz mono-signed
16-bit PCM samples.

```go
package main

import (
	"fmt"

	"github.com/ColonelBlimp/go-ft8/ft8"
)

func main() {
	var pcm []int16 // Fill with one 15-second, 12 kHz mono FT8 slot.

	messages := ft8.DecodeMessages(pcm)
	for _, msg := range messages {
		fmt.Printf("%3d dB %7.1f Hz %+5.2f s  %s\n", msg.SNR, msg.FreqHz, msg.DTSec, msg.Text)
	}
}
```

`DecodedMessage.SNR` is the received SNR estimate in dB using the WSJT-X/JT9
2500 Hz reference bandwidth. Use `msg.SignalReport()` when constructing a reply
that needs an encodable FT8 signal report such as `-13` or `+04`.

For a receiver stream, keep one decoder instance per stream so hash and A7
history can carry across adjacent slots:

```go
decoder := ft8.NewDecoder()
messages := decoder.DecodeMessages(pcm)
```

For labeled strict/deep output:

```go
result := ft8.DecodeStructured(pcm, ft8.StructuredDecodeOptions{
	IncludeDeep: true,
})

for _, msg := range result.Messages {
	fmt.Println(msg.Mode, msg.Text)
}
```

Structured decode also has report and checked variants for service
integrations:

```go
report, err := ft8.DecodeStructuredChecked(pcm, ft8.StructuredDecodeOptions{
	IncludeDeep: true,
})
if err != nil {
	panic(err)
}
fmt.Println(report.Result.Messages)
fmt.Printf("%+v\n", report.StrictReport.Diagnostics)
```

For production logging or empty-result investigation, use the report API:

```go
report := ft8.DecodeMessagesWithReport(pcm, ft8.DecoderOptions{})
fmt.Println(report.Messages)
fmt.Printf("%+v\n", report.Diagnostics)
```

`DecodeDiagnostics` separates non-AP LDPC attempts from AP attempts and records
AP attempt, success, and post-LDPC rejection counts by profile name and source.

For station-manager style integrations that should reject malformed slots or
configuration mistakes before decode work starts, use the checked API:

```go
report, err := ft8.DecodeMessagesChecked(pcm, ft8.DecoderOptions{})
if err != nil {
	// Invalid input length or invalid decoder options.
	panic(err)
}
fmt.Println(report.Messages)
```

Checked decode errors support `errors.Is` with `ErrInvalidDecodeInput` and
`ErrInvalidDecoderOptions`, plus `errors.As` with `*ft8.DecodeInputError` or
`*ft8.DecoderOptionError` for structured details such as sample counts or the
invalid option field.

Encode supported standard FT8 messages, including the standard `/P` variant and
ARRL Field Day exchanges, to protocol bits, LDPC codeword, and tone sequence:

```go
encoded, err := ft8.EncodeStandardMessage("CQ K1ABC FN42")
if err != nil {
	// The first encoder surface intentionally accepts standard messages only.
	panic(err)
}
fmt.Println(encoded.Text, encoded.Tones)
```

The package stops at FT8 protocol artifacts. Audio output, transmit scheduling,
PTT, and radio control belong in separate packages.

## Message Format Coverage

`go-ft8` does not yet implement every WSJT-X 77-bit message family. Current
decode support is aimed at ordinary FT8 QSO traffic and service integration, not
full contest/DXpedition parity.

Supported decode payloads:

| Type | Status |
| ---- | ------ |
| `i3=0,n3=0` | Free text, up to 13 characters |
| `i3=0,n3=3` and `i3=0,n3=4` | ARRL Field Day exchange |
| `i3=1` | Standard messages: CQ, calls, grid, reports, RRR, RR73, 73 |
| `i3=2` | Standard `/P` form used by European VHF-style messages |
| `i3=4` | Compound/nonstandard calls using 12-bit hash context |

Known decode gaps:

| Type | Missing family |
| ---- | -------------- |
| `i3=0,n3=1` | DXpedition / Fox-Hound |
| `i3=0,n3=5` | Telemetry, 18 hex characters |
| `i3=3` | ARRL RTTY Roundup |
| `i3=5` | EU VHF contest with hashed calls, report, serial, and grid6 |

Decoded candidates with unsupported payload formats are rejected during unpack
and counted in `DecodeDiagnostics.UnpackFailures`. The public decoder also
currently filters decoded text containing `/R` and text beginning with `TU; `.

The reference message-family table is WSJT-X's 77-bit format description:
https://github.com/WSJTX/wsjtx/blob/master/lib/77bit/77bit.txt

## Decoder Modes

- Strict mode is the default and is used by `DecodeMessages`.
- Deep mode is available through `DeepDecoderOptions` or
  `DecodeStructured(..., StructuredDecodeOptions{IncludeDeep: true})`.
- Custom search thresholds, frequency ranges, candidate caps, block counts, and
  Costas gates are available through `DecoderOptions`.
- The default decoder includes a conservative CQ AP pass. `EnableBroadAP` adds
  experimental standard-message AP profiles for exact directed-CQ variants used
  by deep searches.
- `APCallHints` supplies upstream-ranked callsign hints for bounded, BP-only AP.
  The decoder copies, normalizes, deduplicates, caps at 200 hints, cheaply
  scores call1/call2 hypotheses per candidate, and tries only the top
  `MaxAPCallHypotheses` matches. A long-lived `Decoder` can refresh hints with
  `SetAPCallHints`.

## AP Call Hints

AP call hints let an application provide a ranked callsign list from sources
such as recently heard calls, logbook state, watchlists, award needs, or spots.
The codec does not query databases or apply application ranking policy. It only
copies, normalizes, deduplicates, caps, cheaply scores, and tries a bounded
number of BP-only hypotheses per candidate.

For live receiver streams, prefer the stateful decoder so hash history, A7
hints, and AP hints share the same per-stream lifetime:

```go
decoder := ft8.NewDecoderWithOptions(ft8.DeepDecoderOptions())

decoder.SetAPCallHints([]ft8.APCallHint{
	{Call: "K1ABC", Source: "recent", Weight: 10},
	{Call: "W9XYZ", Source: "worked", Weight: 5},
})

report := decoder.DecodeMessagesWithReport(pcm)
fmt.Println(report.Messages)
fmt.Printf("%+v\n", report.Diagnostics)
```

Stateless decode also accepts hints through `DecoderOptions`, but callers must
pass the ranked hint list on every slot:

```go
report := ft8.DecodeMessagesWithReport(pcm, ft8.DecoderOptions{
	APCallHints: []ft8.APCallHint{
		{Call: "K1ABC", Source: "worked"},
	},
	MaxAPCallHypotheses: 2,
})
```

Hint handling is intentionally bounded:

- At most 200 normalized calls are retained.
- Duplicate and unsupported calls are ignored.
- Hints preserve caller order as upstream policy ranking.
- Only call1/call2 standard-message hypotheses are scored.
- The default `MaxAPCallHypotheses` is 2; checked APIs accept at most 8.
- Hint AP attempts are BP-only by default.

`DecodeDiagnostics` reports AP work by profile and source, plus hint-specific
counters such as `APCallHints`, `APHintProfilesScored`,
`APHintHypothesesSelected`, and `APHintHypothesesBelowThreshold`.

## Production PocketFFT Backend

The default build uses pure-Go Gonum FFT support for portability. Production
builds should use the faster CGO PocketFFT backend with the `pocketfft` build
tag:

```sh
go test -tags pocketfft ./...
```

PocketFFT is vendored under `internal/pfft/pocketfft/` and keeps its upstream
BSD 3-Clause license notices. The pure-Go Gonum backend remains the fallback for
builds where CGO or vendored C is not acceptable.

## Benchmarks

Reference decode wall-clock benchmarks for version 0.3.0, measured 2026-06-10
on an Intel Core i3-10100F, Linux amd64, Go 1.26.4-X:nodwarf5, using the
production PocketFFT backend and the six bundled WAV fixtures:

```sh
GOCACHE=/tmp/go-build go test -tags pocketfft ./ft8 -run=^$ \
  -bench='BenchmarkDecode(Messages|Structured).*PerFixture' \
  -benchmem -benchtime=1x -count=1
```

| Benchmark | Mean per 15s slot | Observed fixture range |
| --------- | ----------------- | ---------------------- |
| Strict `DecodeMessages` | 0.586 s | 0.540-0.678 s |
| Deep without broad AP | 3.20 s | 2.71-3.73 s |
| Deep `DecodeMessagesWithOptions(DeepDecoderOptions())` | 4.05 s | 3.39-4.70 s |
| Deep with 200 AP call hints | 4.08 s | 3.47-4.70 s |
| Structured strict+deep `DecodeStructured(...IncludeDeep)` | 4.56 s | 3.95-5.20 s |

The profiling refresh used the same benchmark set with CPU and memory profiles:

```sh
GOCACHE=/tmp/go-build go test -tags pocketfft ./ft8 -run=^$ \
  -bench='BenchmarkDecode(Messages|Structured).*PerFixture' \
  -benchmem -benchtime=1x -count=1 \
  -cpuprofile=/tmp/go-ft8-cpu.prof -memprofile=/tmp/go-ft8-mem.prof
```

CPU samples were concentrated in PocketFFT CGO calls and LDPC/OSD decoding:
`runtime.cgocall` 24.9% flat, `osd17491` 19.9%, `math.archExp` 10.6%,
`decode17491BP` 8.3%, `math.tanh` 5.8%, `math.Sincos` 5.4%, and
`subtractFT8` 3.9% flat / 34.7% cumulative. Allocation samples were dominated
by startup tables and decode scratch space: `init.func5` 39.0% alloc_space,
`newFT8SpectraScratch` 24.9%, `(*ft8SpectraScratch).ensureFinder` 12.8%,
`(*downsampler).downsample` 6.3%, `newRealFFTPlan` 6.1%, and
`(*realFFTPlan).coefficientsRange` 5.7%.

These numbers are local reference measurements, not performance guarantees.
Wall time varies with CPU, CGO toolchain, OS scheduling, fixture content, and
decode options.

## Development

This repository uses [Task](https://taskfile.dev/) for common development
commands. Run the production test path with:

```sh
task test:prod
```

Run production decode benchmarks with:

```sh
task bench:prod
task bench:structured-deep-prod
```

Run the fixture-independent smoke tests:

```sh
task test:smoke
```

Run race-detector smoke tests:

```sh
task test:race
task test:race-prod
```

The package version is tracked in `version.txt` as Go module SemVer without a
leading `v`. Common version tasks:

```sh
task version:get
task version:set -- 0.1.0
task version:bump:patch
task version:bump:minor
task version:tag
task version:push-tag
```

`task version:tag` validates `version.txt`, runs smoke tests for the default
and PocketFFT paths, requires a clean working tree, and creates an annotated
local tag such as `v0.1.0`. Pushing the tag is a separate explicit step.
`task version:push-tag` loads `.env` and requires `GITHUB_TOKEN` for a
non-interactive GitHub HTTPS push. The root `.env` file is ignored by git.

```sh
GITHUB_TOKEN=github_pat_...
```

The full corpus and diagnostic tests depend on the local WAV/truth fixture
corpus and its expected testdata layout. Keep decode-scale or synchronization
changes behind parity checks, because small numeric changes can affect
strict-mode message recovery.

## Licensing

This repository is distributed under GPL-3.0-only. See [LICENSE](LICENSE) and
[docs/LICENSING.md](docs/LICENSING.md).

Because this is a WSJT-X/jt9-derived implementation, redistribution of this
repository or derivative binaries should preserve the GPLv3 license text, the
project [NOTICE](NOTICE), the derivative-status note in
[docs/WSJTX_DERIVATIVE.md](docs/WSJTX_DERIVATIVE.md), and applicable
third-party notices.
