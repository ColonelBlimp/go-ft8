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
