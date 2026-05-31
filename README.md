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
		fmt.Printf("%7.1f Hz %+5.2f s  %s\n", msg.FreqHz, msg.DTSec, msg.Text)
	}
}
```

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

Encode supported standard FT8 messages to protocol bits, LDPC codeword, and
tone sequence:

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

## Decoder Modes

- Strict mode is the default and is used by `DecodeMessages`.
- Deep mode is available through `DeepDecoderOptions` or
  `DecodeStructured(..., StructuredDecodeOptions{IncludeDeep: true})`.
- Custom search thresholds, frequency ranges, candidate caps, block counts, and
  Costas gates are available through `DecoderOptions`.

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

## Development

This repository uses [Task](https://taskfile.dev/) for common development
commands. Run the production test path with:

```sh
task test:prod
```

Run production decode benchmarks with:

```sh
task bench:prod
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
task version:tag
task version:push-tag
```

`task version:tag` validates `version.txt`, runs smoke tests for the default
and PocketFFT paths, requires a clean working tree, and creates an annotated
local tag such as `v0.1.0`. Pushing the tag is a separate explicit step.

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
