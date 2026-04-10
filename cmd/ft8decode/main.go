package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"

	ft8x "github.com/ColonelBlimp/go-ft8"
)

func main() {
	var (
		wavPath = flag.String("wav", "", "Path to 16-bit PCM mono WAV file")
		freqMin = flag.Float64("fmin", 200.0, "Minimum candidate search frequency (Hz)")
		freqMax = flag.Float64("fmax", 3200.0, "Maximum candidate search frequency (Hz)")
		passes  = flag.Int("passes", 3, "Number of iterative subtraction passes")
	)
	flag.Parse()

	if *wavPath == "" {
		fmt.Fprintln(os.Stderr, "missing required -wav argument")
		os.Exit(2)
	}

	samples, sr, err := loadWAV(*wavPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load wav: %v\n", err)
		os.Exit(1)
	}
	if sr != 12000 {
		fmt.Fprintf(os.Stderr, "expected 12000 Hz, got %d Hz\n", sr)
		os.Exit(1)
	}

	params := ft8x.DefaultDecodeParams()
	params.MaxPasses = *passes
	results := ft8x.DecodeIterative(samples, params, *freqMin, *freqMax)
	for _, r := range results {
		fmt.Println(ft8x.FormatDecodeResult(r, 0))
	}
}

func loadWAV(path string) ([]float32, uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	if len(data) < 44 {
		return nil, 0, fmt.Errorf("file too small: %d", len(data))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("not a WAV file")
	}
	if string(data[12:16]) != "fmt " {
		return nil, 0, fmt.Errorf("expected fmt chunk")
	}
	if binary.LittleEndian.Uint16(data[20:22]) != 1 {
		return nil, 0, fmt.Errorf("unsupported format")
	}
	nCh := binary.LittleEndian.Uint16(data[22:24])
	sr := binary.LittleEndian.Uint32(data[24:28])
	bps := binary.LittleEndian.Uint16(data[34:36])
	if bps != 16 {
		return nil, 0, fmt.Errorf("unsupported bits/sample: %d", bps)
	}

	off := 36
	for off+8 <= len(data) {
		id := string(data[off : off+4])
		sz := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if id == "data" {
			pcm := data[off+8 : off+8+sz]
			bytesPerFrame := int(nCh) * int(bps) / 8
			nFrames := len(pcm) / bytesPerFrame
			samples := make([]float32, nFrames)
			for i := 0; i < nFrames; i++ {
				o := i * bytesPerFrame
				v := int16(binary.LittleEndian.Uint16(pcm[o : o+2]))
				samples[i] = float32(v) / 32768.0
			}
			return samples, sr, nil
		}
		off += 8 + sz
		if sz%2 != 0 {
			off++
		}
	}

	return nil, 0, fmt.Errorf("no data chunk found")
}
