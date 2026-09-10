// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

import (
	"math"
	"testing"
)

func BenchmarkDecodeMessagesSynthesizedSlot(b *testing.B) {
	encoded, err := EncodeStandardMessage("CQ 7Q5MLV KH67")
	if err != nil {
		b.Fatal(err)
	}
	short := synthesizeOracleWave(encoded.Tones, 1500)
	samples := make([]int16, SlotSamples)
	copy(samples, short)
	report := DecodeMessagesWithReport(samples, DecoderOptions{})
	if len(report.Messages) != 1 {
		b.Fatalf("single-signal benchmark setup decoded %d messages, want 1", len(report.Messages))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = DecodeMessages(samples)
	}
	b.ReportMetric(float64(report.Diagnostics.CandidatesAnalyzed), "candidates/slot")
	b.ReportMetric(float64(len(report.Messages)), "messages/slot")
}

func BenchmarkDecodeMessagesNoisySlot(b *testing.B) {
	encoded, err := EncodeStandardMessage("CQ 7Q5MLV KH67")
	if err != nil {
		b.Fatal(err)
	}
	short := noisyFT4(synthesizeOracleWave(encoded.Tones, 1500), -16, 1)
	samples := make([]int16, SlotSamples)
	copy(samples, short)
	report := DecodeMessagesWithReport(samples, DecoderOptions{})
	if len(report.Messages) != 1 {
		b.Fatalf("noisy benchmark setup decoded %d messages, want 1", len(report.Messages))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = DecodeMessages(samples)
	}
	b.ReportMetric(float64(report.Diagnostics.CandidatesAnalyzed), "candidates/slot")
	b.ReportMetric(float64(len(report.Messages)), "messages/slot")
}

func BenchmarkDecodeMessagesBusySlot(b *testing.B) {
	messages := []string{
		"CQ K1ABC FN42",
		"CQ W9XYZ EN50",
		"CQ 7Q5MLV KH67",
		"CQ ZS6BUN KG44",
		"CQ A61DI LL64",
		"CQ DL9UW JO41",
		"CQ G0XYZ IO83",
		"CQ JA1ABC PM95",
		"CQ VK3ABC QF22",
		"CQ PY2ABC GG66",
		"CQ VU2ABC MK82",
		"CQ 5R8XX LH41",
	}
	accumulated := make([]float64, decodeSamples)
	for i, message := range messages {
		encoded, err := EncodeStandardMessage(message)
		if err != nil {
			b.Fatal(err)
		}
		wave := synthesizeOracleWave(encoded.Tones, 500+350*float64(i))
		for sample, value := range wave {
			accumulated[sample] += float64(value) / float64(len(messages)+2)
		}
	}
	samples := make([]int16, SlotSamples)
	for i, value := range accumulated {
		samples[i] = int16(math.Max(-32768, math.Min(32767, math.Round(value))))
	}
	report := DecodeMessagesWithReport(samples, DecoderOptions{})
	if len(report.Messages) != len(messages) {
		b.Fatalf("busy benchmark setup decoded %d messages, want %d", len(report.Messages), len(messages))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = DecodeMessages(samples)
	}
	b.ReportMetric(float64(report.Diagnostics.CandidatesAnalyzed), "candidates/slot")
	b.ReportMetric(float64(len(report.Messages)), "messages/slot")
}
