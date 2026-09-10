# FT4 Contest Delivery

This is the durable handoff for the FT4 work needed by the Africa FT4 DX
Contest on 12 September 2026.

## Status

The basic FT4 library boundary is implemented. It includes protocol encoding,
an unassisted BP receiver, typed checked-API errors, diagnostics, timing and
tone geometry, and a decoded-message type shared with FT8.

## Contest scope

The [official contest rules](https://mysarl.org.za/wp-content/uploads/2025/11/2026-Africa-FT4-DX-Contest-v1.1.pdf)
specify ordinary FT4 messages: callsigns, four-character Maidenhead locators,
signal reports, and RRR/73/RR73. Dedicated contest messages and Fox/Hound
operation are explicitly excluded. The library therefore does not need
contest-specific message packing for the first usable release.

The 2026 event is 15:00-18:00 UTC on Saturday 12 September. The rules list
3.576, 7.0475, and 14.080 MHz USB as the default dial frequencies. These are
application configuration, not codec constants.

## Delivery plan

- [x] Encode the supported standard 77-bit message formats into FT4 channel
  symbols.
- [x] Decode one 7.5-second, 12 kHz, mono signed-16-bit PCM slot into verified
  standard FT4 messages, frequency, time offset, and signal report.
- [x] Cover frequency discovery, four Costas arrays, time/frequency refinement,
  FT4 bit metrics, descrambling, CRC-14, and LDPC(174,91) recovery.
- [x] Provide permissive and checked public APIs suitable for the frontend.
- [x] Verify noise-free and noisy synthesized frames, multiple messages in one
  slot, malformed input, both FFT build configurations, and race safety.
- [x] Document the frontend contract and the contest's standard FT4 exchange.

## Application contract

Use `ft4.DecodeMessagesChecked` at the ingestion boundary. It accepts exactly
`ft4.SlotSamples` (90,000) signed 16-bit samples: mono, 12,000 Hz, one 7.5-second
slot. The permissive `DecodeMessages` variant zero-pads short input and ignores
the unused tail of longer input.

```go
report, err := ft4.DecodeMessagesChecked(pcm, ft4.DecoderOptions{})
if err != nil {
	return err
}
for _, decoded := range report.Messages {
	// decoded is the same DTO as ft8.DecodedMessage.
	publish(decoded)
}
```

Useful exported geometry:

| Constant | Value | Meaning |
| --- | ---: | --- |
| `SampleRate` | 12,000 Hz | PCM sample rate |
| `SlotSeconds` | 7.5 s | receive/transmit period |
| `SlotSamples` | 90,000 | samples in a checked decoder input |
| `NominalStartSeconds` | 0.452 s | start of the complete ramped waveform |
| `SyncStartSeconds` | 0.500 s | first Costas-symbol boundary; decoder DT zero |
| `WaveformSeconds` | 5.040 s | complete waveform including both ramps |
| `ToneSpacingHz` | 20.8333 Hz | adjacent-tone separation |
| `SignalWidthHz` | 83.3333 Hz | nominal occupied four-tone span |

The decoder returns verified payloads even when their 77-bit message family is
not renderable. Consumers should publish text only when `ParseStatus` is
`ParseStatusParsed`, while retaining `Payload` if raw observations matter.

The standard contest sequence is built from ordinary messages such as
`CQ 7Q5MLV KH67`, `7Q5MLV ZS6BUN -10`, `ZS6BUN 7Q5MLV R-08`, and the usual
`RRR`, `RR73`, or `73` completion. `EncodeStandardMessage` handles these forms;
sequencing and transmit policy remain application responsibilities.

## Verification evidence

- `task test` and `task test:gonum` pass.
- `task test:race` and `task test:race-gonum` pass.
- A five-second decode-API fuzz run completed 8,736 executions without a
  failure.
- Profiling reduced the clean synthesized decode from 17.254 ms and 6.59 MB
  allocated per slot to 10.590 ms and 1.8 KB under the same profiler: 38.6%
  faster with 99.97% fewer allocated bytes.
- Final three-run, 100-iteration means on the station host's Ryzen 9 9900X are
  10.542 ms for a clean frame, 4.629 ms for a one-candidate frame at -16 dB,
  and 13.139 ms for a busy slot containing 12 decoded signals. The busy case
  uses 0.175% of the 7.5-second slot and 1.31% of the provisional one-second
  decode budget.
- The six-fixture strict FT8 refresh measured 114.015-153.979 ms per 15-second
  slot (135.533 ms mean). Its aggregate CPU profile remains dominated by the
  existing parallel PocketFFT/downsampling path, so no FT8 algorithm was
  changed in this deadline pass.
- These are deterministic synthetic FT4 measurements, not the live busy-band
  p95 required by the Station Manager passive-RX gate.
- The opt-in `jt9` oracle was not rerun because no pinned `jt9` executable is
  installed in this workspace. The test remains available through
  `WSJTX_JT9=/path/to/jt9 task test:oracle-ft4`.

## Deferred until after the contest baseline

- A-priori (AP) decoding and callsign hints.
- Successive interference cancellation/subtraction.
- Deep/OSD search modes and stateful hashed-callsign history.
- Audio-device I/O, transmit scheduling, PTT, CAT control, and logging policy.

## Reference baseline

The signal-processing behavior is based on the GPLv3 WSJT-X FT4 decoder at
commit `b4f9a431bcf6449df8f37b56de79d48b665b044c` (`wsjtx-2.7.0`) and the
[published FT4 protocol](https://wsjt.sourceforge.io/FT4_Protocol.pdf). The
existing project derivative-work and licensing notices apply.
