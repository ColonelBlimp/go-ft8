# go-ft8

`go-ft8` is a Go research implementation of an FT8 decoder. It decodes one
15-second FT8 slot from 12 kHz mono signed 16-bit PCM audio and exposes both a
strict decoder path and optional deeper experimental search modes.

This repository is intended for decoder experimentation, parity work, profiling,
and integration into larger Go applications that need FT8 message recovery.

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

Import the decoder package:

```go
import "github.com/ColonelBlimp/go-ft8/ft8"
```

## Basic Usage

`DecodeMessages` expects a single FT8 receive slot as 12 kHz mono signed
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

## Decoder Modes

- Strict mode is the default and is used by `DecodeMessages`.
- Deep mode is available through `DeepDecoderOptions` or
  `DecodeStructured(..., StructuredDecodeOptions{IncludeDeep: true})`.
- Custom search thresholds, frequency ranges, candidate caps, block counts, and
  Costas gates are available through `DecoderOptions`.

## Optional PocketFFT Backend

The default build uses Gonum FFT support. An optional CGO PocketFFT backend is
available with the `pocketfft` build tag:

```sh
go test -tags pocketfft ./... -run 'TestCRC14FastMatchesSlow|TestPackEncodeRoundTripsStandardMessages|TestDecodeOptions'
```

PocketFFT is vendored under `internal/pfft/pocketfft/` and keeps its upstream
BSD 3-Clause license notices.

## Development

Run the fixture-independent smoke tests:

```sh
go test ./... -run 'TestCRC14FastMatchesSlow|TestPackEncodeRoundTripsStandardMessages|TestDecodeOptions'
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
