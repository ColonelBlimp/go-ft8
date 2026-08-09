// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

var benchmarkPayloadParseResult payloadParseResult

func BenchmarkParsePayload77GoldenVectors(b *testing.B) {
	for _, vector := range unpack77GoldenVectors {
		b.Run(vector.name, func(b *testing.B) {
			payload := payload77FromGoldenBits(b, vector.payload)
			var hashes hashTable
			for _, call := range vector.calls {
				hashes.Save(call)
			}
			b.ReportAllocs()
			b.ResetTimer()
			var parsed payloadParseResult
			for i := 0; i < b.N; i++ {
				parsed = parsePayload77WithHashes(payload, &hashes)
			}
			benchmarkPayloadParseResult = parsed
		})
	}
}

func BenchmarkParseCandidatePayload(b *testing.B) {
	parsedBits, ok := pack77StandardMessage("K1ABC W9XYZ/R -10")
	if !ok {
		b.Fatal("pack standard message")
	}
	var unsupportedBits [77]int8
	writeBits(unsupportedBits[:], 71, 3, 6)
	var reservedBits [77]int8
	writeBits(reservedBits[:], 74, 3, 6)

	tests := []struct {
		name string
		bits [77]int8
	}{
		{name: "parsed", bits: parsedBits},
		{name: "unsupported", bits: unsupportedBits},
		{name: "reserved", bits: reservedBits},
	}
	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			codeword := encode17491(test.bits)
			var hashes hashTable
			b.ReportAllocs()
			b.ResetTimer()
			var parsed payloadParseResult
			for i := 0; i < b.N; i++ {
				parsed = parseCandidatePayload(codeword, &hashes)
			}
			benchmarkPayloadParseResult = parsed
		})
	}
}
