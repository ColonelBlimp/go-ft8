# go-ft8

`go-ft8` is a Go implementation of FT8 and FT4 decoding and protocol encoding.
It decodes 12 kHz mono signed-16-bit PCM slots, encodes supported standard FT8
and FT4 messages to protocol symbols, and exposes both a strict FT8 decoder
path and optional deeper experimental FT8 search modes.

This repository is intended for FT8/FT4 protocol experimentation, parity work,
profiling, and integration into larger Go applications that need FT-family
message recovery or standard-message encoding.

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

FT4 is provided by a sibling package:

```go
import "github.com/ColonelBlimp/go-ft8/ft4"
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
		if msg.ParseStatus != ft8.ParseStatusParsed {
			continue // Retain msg.Payload separately if raw observations matter.
		}
		fmt.Printf("%3d dB %7.1f Hz %+5.2f s  %s\n", msg.SNR, msg.FreqHz, msg.DTSec, msg.Text)
	}
}
```

`DecodedMessage.SNR` is the received SNR estimate in dB using the WSJT-X/JT9
2500 Hz reference bandwidth. Use `msg.SignalReport()` when constructing a reply
that needs an encodable FT8 signal report such as `-13` or `+04`.

Every signal-level candidate accepted after LDPC, CRC, hard-error, and all-zero
checks is returned. `DecodedMessage.Payload` contains its exact verified 77-bit
payload, `ParseStatus` says whether canonical text was produced, and
`Payload.MessageType()` returns the `i3`/`n3` family selector. `Payload.Bits()`
returns one byte per bit when a consumer needs the unpacked bit sequence.

`DecodedMessage.Provenance` identifies the decoding algorithm (`bp`, `osd`, or
`a7`). Its `APProfile` and `APSource` fields identify a-priori assistance and
are empty for an unassisted metric decode. The codec returns `/R` and `TU;`
observations without applying publication policy; callers can store every
verified payload and independently decide which parsed observations to forward.

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

Encode supported standard FT8 messages, including standard `/P` and `/R`
suffix forms, ARRL Field Day exchanges, and compound/nonstandard calls such as
`CQ PJ4/NA2AA`, `CQ YW18FIFA/P`, or `<W9XYZ> K1ABC/P`, to protocol bits, LDPC
codeword, and tone sequence:

```go
encoded, err := ft8.EncodeStandardMessage("CQ K1ABC FN42")
if err != nil {
	// Unsupported FT8 message family for this encoder surface.
	panic(err)
}
fmt.Println(encoded.Text, encoded.Tones)
```

FT4 uses the same supported source-message formats and returns the source bits,
FT4-scrambled bits, 174-bit codeword, and 103 sync/data tones:

```go
encoded, err := ft4.EncodeStandardMessage("CQ TEST K1ABC FN42")
if err != nil {
	panic(err)
}
fmt.Println(encoded.Text, encoded.Tones)
```

Decode one complete FT4 slot with the checked integration API:

```go
var pcm = make([]int16, ft4.SlotSamples) // 7.5 s at 12 kHz, mono PCM16.

report, err := ft4.DecodeMessagesChecked(pcm, ft4.DecoderOptions{})
if err != nil {
	panic(err)
}
for _, msg := range report.Messages {
	fmt.Printf("%3d dB %7.1f Hz %+5.2f s  %s\n",
		msg.SNR, msg.FreqHz, msg.DTSec, msg.Text)
}
```

`ft4.DecodedMessage` is an alias of `ft8.DecodedMessage`, so an application can
use a single FT-family result DTO. FT4 exposes `SlotSeconds`, `SlotSamples`,
`NominalStartSeconds`, `SyncStartSeconds`, `WaveformSeconds`, `ToneSpacingHz`,
and `SignalWidthHz` for scheduler and waterfall integration. `DTSec` is relative
to the nominal first Costas-symbol boundary at 0.5 seconds into the slot; the
complete ramped waveform begins at 0.452 seconds.

The baseline FT4 decoder performs unassisted belief-propagation decoding and
supports multiple signals at distinct frequencies. AP decoding, successive
interference cancellation, and deep/OSD search are deferred, so this first
receiver is intentionally less sensitive in crowded or overlapping conditions
than WSJT-X.

Maintainers with a pinned WSJT-X `jt9` binary can validate the FT4 frames
end-to-end with `WSJTX_JT9=/path/to/jt9 task test:oracle-ft4`.

For compound-call QSO messages, the non-compound counterpart is encoded as a
12-bit hash. Stateless round-trips therefore render that counterpart as
`<...>` unless decoder hash history can resolve it. Bracketed hash tokens are
accepted on the encode side as well: `<W9XYZ> K1ABC/P` hashes the bracketed
call and spells the other verbatim in the 58-bit field, exactly as WSJT-X
`pack77_4` does, so `/P`, `/R`, and other affixes survive on the air.

Two deliberate deviations from WSJT-X `pack77`: an unbracketed
compound+standard pair such as `PJ4/K1ABC W9XYZ RRR` is packed as type 4 with
the full compound call preserved (WSJT-X silently drops the prefix and packs
type 1), and messages WSJT-X would encode lossily — `CQ YW18FIFA GG66` (grid
dropped) or mixed `/R`+`/P` suffixes (the `/R` becomes `/P`) — are rejected
instead of altered.

Applications can discover and validate the ARRL/RAC Field Day sections accepted
by the encoder:

```go
sections := ft8.ARRLFieldDaySections()
section, ok := ft8.ParseARRLFieldDaySection("ema")
fmt.Println(sections, section, ok)
```

The packages stop at FT8/FT4 protocol artifacts. Audio output, transmit
scheduling, PTT, and radio control belong in separate packages.

## Message Format Coverage

The decoder returns every verified payload, including assigned-but-unsupported
and reserved families. Supported families produce canonical text; other
families remain available to storage and protocol-analysis consumers through
`DecodedMessage.Payload` and `ParseStatus`.

| Type | Parse status | Interpretation |
| ---- | ------------ | -------------- |
| `i3=0,n3=0` | `ParseStatusParsed` | Free text, up to 13 characters |
| `i3=0,n3=1` | `ParseStatusParsed` | DXpedition / Fox-Hound |
| `i3=0,n3=2` and `i3=0,n3=7` | `ParseStatusUnknownType` | Reserved subtypes |
| `i3=0,n3=3` and `i3=0,n3=4` | `ParseStatusParsed` | ARRL Field Day exchange |
| `i3=0,n3=5` | `ParseStatusParsed` | Telemetry, up to 18 hexadecimal characters |
| `i3=0,n3=6` | `ParseStatusUnsupported` | Assigned compatibility format not unpacked by this package |
| `i3=1` | `ParseStatusParsed` | Standard messages, including `/R` |
| `i3=2` | `ParseStatusParsed` | Standard `/P` form |
| `i3=3` | `ParseStatusParsed` | ARRL RTTY Roundup, including optional `TU;` |
| `i3=4` | `ParseStatusParsed` | Compound/nonstandard calls using 12-bit hash context |
| `i3=5` | `ParseStatusParsed` | EU VHF contest with hashed calls, report, serial, and grid6 |
| `i3=6` and `i3=7` | `ParseStatusUnknownType` | Reserved message types |

A known family with an invalid field combination is returned with
`ParseStatusInvalid`. `DecodeDiagnostics.UnpackFailures` and
`RejectedMessageFilter` remain as deprecated compatibility fields and are no
longer incremented.

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

## FFT Backends

CGO-enabled builds (the Go default for native compilation) use the vendored
PocketFFT C backend, which is 30-40% faster end-to-end than the pure-Go
alternative. Builds with `CGO_ENABLED=0` (including most cross-compilation)
automatically fall back to the pure-Go Gonum backend. To force the pure-Go
backend in a CGO-enabled build, use the `gonumfft` build tag:

```sh
go test -tags gonumfft ./...
```

The legacy `pocketfft` build tag is no longer needed: PocketFFT is now the
default wherever CGO is available. PocketFFT is vendored under
`internal/pfft/pocketfft/` and keeps its upstream BSD 3-Clause license
notices.

## Benchmarks

Reference decode wall-clock benchmarks for version 0.8.0, measured 2026-08-09
on an AMD Ryzen 9 9900X (12 cores / 24 threads), Linux amd64, Go 1.26.5,
using the default PocketFFT backend, concurrent candidate analysis, and the
six bundled WAV fixtures. Each table entry is the mean of the six per-fixture
medians from seven runs:

```sh
GOCACHE=/tmp/go-build go test ./ft8 -run=^$ \
  -bench='BenchmarkDecode(Messages|Structured).*PerFixture' \
  -benchmem -benchtime=1x -count=7
```

| Benchmark | Mean per 15s slot | Observed fixture range |
| --------- | ----------------- | ---------------------- |
| Strict `DecodeMessages` | 0.136 s | 0.117-0.156 s |
| Deep without broad AP | 0.516 s | 0.456-0.623 s |
| Deep `DecodeMessagesWithOptions(DeepDecoderOptions())` | 0.514 s | 0.462-0.612 s |
| Deep with 200 AP call hints | 0.516 s | 0.466-0.614 s |
| Structured strict+deep `DecodeStructured(...IncludeDeep)` | 0.651 s | 0.578-0.774 s |

Candidate analysis runs concurrently across all available CPUs by default and
produces output identical to the sequential path; set
`DecoderOptions.Workers = 1` to force sequential decoding, or a higher value
to cap the goroutine count.

Focused verified-payload benchmarks on the same host use five 200 ms samples:

| Path | Median | Memory | Allocations |
| ---- | ------ | ------ | ----------- |
| DXpedition unpack | 208 ns | 48 B | 4 |
| Telemetry unpack | 161 ns | 48 B | 4 |
| ARRL RTTY Roundup unpack | 181 ns | 80 B | 6 |
| EU VHF Contest unpack | 208 ns | 64 B | 3 |
| Parsed candidate extraction | 287 ns | 64 B | 6 |
| Unsupported candidate extraction | 92 ns | 0 B | 0 |
| Reserved candidate extraction | 87 ns | 0 B | 0 |

The profiling refresh used the same benchmark set with CPU and memory profiles:

```sh
GOCACHE=/tmp/go-build go test ./ft8 -run=^$ \
  -bench='BenchmarkDecode(Messages|Structured).*PerFixture' \
  -benchmem -benchtime=1x -count=1 \
  -cpuprofile=/tmp/go-ft8-cpu.prof -memprofile=/tmp/go-ft8-mem.prof
```

On the reference host, the combined benchmark CPU profile is concentrated in
PocketFFT/downsampling and LDPC decoding: `runtime.cgocall` is 38.2% flat,
`downsample` is 47.1% cumulative, `osd17491` is 13.2% flat / 19.5% cumulative,
`decode17491BP` is 6.7% flat / 10.5% cumulative, and `subtractFT8` is 13.2%
cumulative. Payload parsing does not appear among the reported CPU hotspots.

The allocation profile is dominated by reusable working storage rather than
payload results: subtraction scratch accounts for 29.0% of allocated bytes,
spectra scratch 26.3%, finder scratch 13.1%, and parallel candidate outcomes
8.3%. The end-of-run in-use snapshot is 228 MB, primarily scratch and FFT-plan
pools sized for concurrent decoding.

These numbers are local reference measurements, not performance guarantees.
Wall time varies with CPU, CGO toolchain, OS scheduling, fixture content, and
decode options.

### FT4 v0.9.0 slot-timing profile

The FT4 receiver was profiled on 2026-09-10 on the same Ryzen 9 9900X host.
The measured changes reuse FFT plans and transform storage across slots, reuse
the candidate-baseline scratch space, pre-conjugate Costas references, and move
sync range checks outside the per-sample loop. An optimized/reference score
test covers partial blocks at both ends of the receive window.

Using identical 100-iteration CPU/memory profiling commands, the clean-frame
benchmark changed as follows:

| State | Time per 7.5 s slot | Bytes per slot | Allocations per slot |
| ----- | -------------------: | -------------: | -------------------: |
| Before | 17.254 ms | 6,588,457 B | 170 |
| After | 10.590 ms | 1,767 B | 8 |

That is a 38.6% wall-time reduction, a 99.97% reduction in allocated bytes,
and a 95.3% reduction in allocation count. The remaining clean-frame profile
is led by the Costas sync search and Gonum FFT execution; neither exposes
another low-risk deadline optimization of comparable size.

Final steady-state benchmarks used 100 iterations per run and three runs:

```sh
GOCACHE=/tmp/go-build go test ./ft4 -run=^$ \
  -bench='BenchmarkDecodeMessages(Synthesized|Noisy|Busy)Slot$' \
  -benchmem -benchtime=100x -count=3
```

| FT4 input | Candidates | Messages | Mean | Observed range | Memory | Allocations |
| --------- | ---------: | -------: | ---: | -------------: | -----: | ----------: |
| Clean synthesized signal | 9 | 1 | 10.542 ms | 10.528–10.554 ms | 1,680 B | 8 |
| One signal at −16 dB AWGN | 1 | 1 | 4.629 ms | 4.626–4.632 ms | 240 B | 6 |
| Busy synthesized slot | 12 | 12 | 13.139 ms | 13.129–13.145 ms | 2,984 B | 46 |

The 12-message case consumes 0.175% of an FT4 slot and 1.31% of Station
Manager's provisional one-second decode budget. It is deterministic synthetic
evidence, not a substitute for the required live busy-band p95 measurement.

For comparison, a fresh strict FT8 profile over the six bundled real WAV
fixtures measured a mean of 135.533 ms per 15-second slot, with a
114.015–153.979 ms fixture range. The aggregate CPU profile remains dominated
by the already-parallel PocketFFT/downsampling path, so this pass made no FT8
algorithm changes.

## Development

This repository uses [Task](https://taskfile.dev/) for common development
commands. Run the full test suite with the default backend (PocketFFT when
CGO is available):

```sh
task test
```

Run the pure-Go Gonum fallback suite (CGO disabled):

```sh
task test:gonum
```

Run decode benchmarks with:

```sh
task bench
task bench:structured-deep
task bench:payload
task bench:ft4
```

Run the fixture-independent smoke tests:

```sh
task test:smoke
task test:smoke-gonum
```

Run race-detector smoke tests:

```sh
task test:race
task test:race-gonum
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
