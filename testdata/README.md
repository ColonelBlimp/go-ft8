# DSP Test Data

This directory holds WAV files for testing the FT8 RX pipeline against real
signals.

## Obtaining WSJT-X sample recordings

The WSJT-X project ships sample `.wav` recordings of FT8 signals. These files
are **not bundled** in this repository (they are GPLv3-licensed and relatively
large). Tests that depend on them will skip gracefully with `t.Skipf` if the
file is not found.

To obtain the sample files:

1. Download WSJT-X source from https://sourceforge.net/projects/wsjt/
2. Look in the `samples/` directory for files like `FT8/` recordings
3. Copy any 12 kHz mono WAV files into this directory

Alternatively, record your own FT8 signals using WSJT-X or `arecord`:

```bash
arecord -f S16_LE -r 12000 -c 1 -d 15 ft8_capture.wav
```

## Expected file format

- **Sample rate:** 12 000 Hz (standard for WSJT modes)
- **Channels:** 1 (mono)
- **Format:** PCM 16-bit signed (most common) or IEEE float 32-bit
- **Duration:** 15 seconds (one FT8 window = 180 000 samples)

Files at other sample rates (e.g., 48 kHz) will need resampling before use.

## File naming convention

Place files here with descriptive names:

```
ft8_sample_01.wav      — general FT8 recording with multiple signals
ft8_weak_signals.wav   — recording with weak signals (low SNR)
ft8_single_signal.wav  — recording with a single strong signal
```

The test `TestProcessWindowWAVFile` in `dsp_wav_test.go` will attempt to load
any `.wav` file in this directory and run it through the RX pipeline.

