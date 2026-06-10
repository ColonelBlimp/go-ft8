// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"path/filepath"
	"testing"
)

func BenchmarkDecodeMessagesPerFixture(b *testing.B) {
	benchmarkDecodeMessagesPerFixture(b, DecoderOptions{})
}

func BenchmarkDecodeMessagesDeepPerFixture(b *testing.B) {
	benchmarkDecodeMessagesPerFixture(b, DeepDecoderOptions())
}

func BenchmarkDecodeMessagesDeepNoBroadAPPerFixture(b *testing.B) {
	options := DeepDecoderOptions()
	options.EnableBroadAP = false
	benchmarkDecodeMessagesPerFixture(b, options)
}

func BenchmarkDecodeMessagesDeepAPCallHintsPerFixture(b *testing.B) {
	options := DeepDecoderOptions()
	options.APCallHints = benchmarkAPCallHints(ft8MaxAPCallHints)
	options.MaxAPCallHypotheses = ft8DefaultMaxAPCallHypotheses
	benchmarkDecodeMessagesPerFixture(b, options)
}

func BenchmarkDecodeMessagesSilence(b *testing.B) {
	samples := make([]int16, ft8FrameSamples)
	for i := 0; i < b.N; i++ {
		_ = DecodeMessages(samples)
	}
}

func BenchmarkDecodeMessagesDeepSilence(b *testing.B) {
	samples := make([]int16, ft8FrameSamples)
	options := DeepDecoderOptions()
	for i := 0; i < b.N; i++ {
		_ = DecodeMessagesWithOptions(samples, options)
	}
}

func BenchmarkDecodeStructuredDeepPerFixture(b *testing.B) {
	matches := corpusWAVFiles(b)
	options := StructuredDecodeOptions{IncludeDeep: true}
	for _, wavPath := range matches {
		samples, err := loadWAV(wavPath)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(filepath.Base(wavPath), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = DecodeStructured(samples, options)
			}
		})
	}
}

func benchmarkAPCallHints(n int) []APCallHint {
	hints := make([]APCallHint, 0, n)
	for i := 0; len(hints) < n; i++ {
		call := testAPCall(i)
		if _, ok := pack28(call); !ok {
			continue
		}
		hints = append(hints, APCallHint{Call: call, Source: "bench"})
	}
	return hints
}

func benchmarkDecodeMessagesPerFixture(b *testing.B, options DecoderOptions) {
	matches := corpusWAVFiles(b)
	for _, wavPath := range matches {
		samples, err := loadWAV(wavPath)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(filepath.Base(wavPath), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = DecodeMessagesWithOptions(samples, options)
			}
		})
	}
}
