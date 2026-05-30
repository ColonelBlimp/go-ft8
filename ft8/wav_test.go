// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const pcmFormat = 1

// loadWAV reads a 12 kHz mono signed-16-bit little-endian PCM WAV file.
// WAV parsing is test-only; production callers should provide PCM samples.
func loadWAV(path string) ([]int16, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	var riff [12]byte
	if _, err := io.ReadFull(f, riff[:]); err != nil {
		return nil, fmt.Errorf("read RIFF header: %w", err)
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return nil, errors.New("not a RIFF/WAVE file")
	}

	fmtSeen := false
	for {
		var hdr [8]byte
		if _, err := io.ReadFull(f, hdr[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, errors.New("no data chunk found")
			}
			return nil, fmt.Errorf("read chunk header: %w", err)
		}
		id := string(hdr[0:4])
		size := binary.LittleEndian.Uint32(hdr[4:8])

		switch id {
		case "fmt ":
			if size < 16 {
				return nil, fmt.Errorf("fmt chunk too small: %d", size)
			}
			buf := make([]byte, size)
			if _, err := io.ReadFull(f, buf); err != nil {
				return nil, fmt.Errorf("read fmt chunk: %w", err)
			}
			format := binary.LittleEndian.Uint16(buf[0:2])
			channels := binary.LittleEndian.Uint16(buf[2:4])
			rate := binary.LittleEndian.Uint32(buf[4:8])
			bits := binary.LittleEndian.Uint16(buf[14:16])
			if format != pcmFormat {
				return nil, fmt.Errorf("unsupported format %d (need PCM=%d)", format, pcmFormat)
			}
			if channels != Channels {
				return nil, fmt.Errorf("unsupported channels %d (need %d)", channels, Channels)
			}
			if rate != SampleRate {
				return nil, fmt.Errorf("unsupported sample rate %d (need %d)", rate, SampleRate)
			}
			if bits != BitsPerSample {
				return nil, fmt.Errorf("unsupported bit depth %d (need %d)", bits, BitsPerSample)
			}
			if size%2 == 1 {
				if _, err := f.Seek(1, io.SeekCurrent); err != nil {
					return nil, err
				}
			}
			fmtSeen = true

		case "data":
			if !fmtSeen {
				return nil, errors.New("data chunk before fmt chunk")
			}
			if size%2 != 0 {
				return nil, fmt.Errorf("data size %d not a multiple of 2", size)
			}
			samples := make([]int16, size/2)
			if err := binary.Read(f, binary.LittleEndian, samples); err != nil {
				return nil, fmt.Errorf("read samples: %w", err)
			}
			return samples, nil

		default:
			skip := int64(size)
			if size%2 == 1 {
				skip++
			}
			if _, err := f.Seek(skip, io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skip %q chunk: %w", id, err)
			}
		}
	}
}
