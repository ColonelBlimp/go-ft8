// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

import (
	"errors"
	"math"
	"math/rand"
	"testing"

	goft8 "github.com/ColonelBlimp/go-ft8/ft8"
)

var _ []goft8.DecodedMessage = DecodeMessages(nil)

func TestDecodeMessagesStandardContestExchange(t *testing.T) {
	tests := []struct {
		message   string
		frequency float64
	}{
		{message: "CQ 7Q5MLV KH67", frequency: 700},
		{message: "7Q5MLV ZS6BUN -10", frequency: 1100},
		{message: "ZS6BUN 7Q5MLV R-08", frequency: 1500},
		{message: "7Q5MLV ZS6BUN RRR", frequency: 1900},
		{message: "ZS6BUN 7Q5MLV RR73", frequency: 2300},
		{message: "7Q5MLV ZS6BUN 73", frequency: 2700},
	}

	for _, test := range tests {
		t.Run(test.message, func(t *testing.T) {
			encoded, err := EncodeStandardMessage(test.message)
			if err != nil {
				t.Fatal(err)
			}
			decoded := DecodeMessages(synthesizeOracleWave(encoded.Tones, test.frequency))
			if len(decoded) != 1 || decoded[0].Text != encoded.Text {
				t.Fatalf("contest exchange decode got %+v, want %q", decoded, encoded.Text)
			}
		})
	}
}

func TestDecodeMessagesSynthesizedFrame(t *testing.T) {
	encoded, err := EncodeStandardMessage("CQ 7Q5MLV KH67")
	if err != nil {
		t.Fatal(err)
	}
	samples := synthesizeOracleWave(encoded.Tones, 1500)
	decoded := DecodeMessages(samples)
	if len(decoded) != 1 {
		t.Fatalf("DecodeMessages returned %d messages: %+v", len(decoded), decoded)
	}
	if decoded[0].Text != encoded.Text {
		t.Fatalf("decoded text got %q, want %q", decoded[0].Text, encoded.Text)
	}
	if math.Abs(decoded[0].FreqHz-1500) > 2 {
		t.Fatalf("decoded frequency got %.2f Hz, want 1500 Hz", decoded[0].FreqHz)
	}
	if math.Abs(decoded[0].DTSec) > 0.02 {
		t.Fatalf("decoded DT got %.4f s, want approximately zero", decoded[0].DTSec)
	}
	if decoded[0].ParseStatus != ParseStatusParsed {
		t.Fatalf("parse status got %s", decoded[0].ParseStatus)
	}
}

func TestDecodeMessagesRefinesFrequencyAndTime(t *testing.T) {
	encoded, err := EncodeStandardMessage("CQ 7Q5MLV KH67")
	if err != nil {
		t.Fatal(err)
	}
	const (
		frequency = 1500.7
		delay     = 0.012
	)
	samples := synthesizeOracleWave(encoded.Tones, frequency)
	shifted := make([]int16, len(samples))
	shift := int(delay * SampleRate)
	copy(shifted[shift:], samples[:len(samples)-shift])
	decoded := DecodeMessages(shifted)
	if len(decoded) != 1 {
		t.Fatalf("DecodeMessages returned %d messages: %+v", len(decoded), decoded)
	}
	if math.Abs(decoded[0].FreqHz-frequency) > 1.5 {
		t.Fatalf("decoded frequency got %.2f Hz, want %.2f Hz", decoded[0].FreqHz, frequency)
	}
	if math.Abs(decoded[0].DTSec-delay) > 0.005 {
		t.Fatalf("decoded DT got %.4f s, want %.4f s", decoded[0].DTSec, delay)
	}
}

func TestDecodeMessagesNoisyFrame(t *testing.T) {
	encoded, err := EncodeStandardMessage("7Q5MLV ZS6BUN -10")
	if err != nil {
		t.Fatal(err)
	}
	clean := synthesizeOracleWave(encoded.Tones, 1800)
	for seed := int64(1); seed <= 5; seed++ {
		samples := noisyFT4(clean, -16, seed)
		decoded := DecodeMessages(samples)
		if len(decoded) != 1 || decoded[0].Text != encoded.Text {
			t.Fatalf("seed %d noisy decode got %+v, want %q", seed, decoded, encoded.Text)
		}
		if math.Abs(float64(decoded[0].SNR)-(-16)) > 2 {
			t.Fatalf("seed %d estimated SNR got %d dB, want within 2 dB of -16", seed, decoded[0].SNR)
		}
	}
}

func TestDecodeMessagesMultipleSignals(t *testing.T) {
	messages := []string{"CQ 7Q5MLV KH67", "ZS6BUN 7Q5MLV -08"}
	frequencies := []float64{900, 2100}
	mixed := make([]int16, decodeSamples)
	for i, message := range messages {
		encoded, err := EncodeStandardMessage(message)
		if err != nil {
			t.Fatal(err)
		}
		wave := synthesizeOracleWave(encoded.Tones, frequencies[i])
		for j, sample := range wave {
			mixed[j] += sample / 3
		}
	}
	decoded := DecodeMessages(mixed)
	got := make(map[string]bool, len(decoded))
	for _, message := range decoded {
		got[message.Text] = true
	}
	for _, want := range messages {
		if !got[want] {
			t.Fatalf("multi-signal decode got %+v, missing %q", decoded, want)
		}
	}
}

func TestDecodeMessagesTruncatedStart(t *testing.T) {
	encoded, err := EncodeStandardMessage("CQ 7Q5MLV KH67")
	if err != nil {
		t.Fatal(err)
	}
	samples := synthesizeOracleWave(encoded.Tones, 1500)
	lateStart := int((NominalStartSeconds + 1.5) * SampleRate)
	clear(samples[:lateStart])
	decoded := DecodeMessages(samples)
	if len(decoded) != 1 || decoded[0].Text != encoded.Text {
		t.Fatalf("truncated-start decode got %+v, want %q", decoded, encoded.Text)
	}
}

func noisyFT4(clean []int16, snrDB float64, seed int64) []int16 {
	rng := rand.New(rand.NewSource(seed))
	signalScale := 100 * math.Sqrt(2*(2500.0/(SampleRate/2.0))) * math.Pow(10, snrDB/20) / 32766.9
	out := make([]int16, len(clean))
	for i, sample := range clean {
		value := signalScale*float64(sample) + 100*rng.NormFloat64()
		value = math.Max(-32768, math.Min(32767, math.Round(value)))
		out[i] = int16(value)
	}
	return out
}

func TestDecodeMessagesCheckedValidation(t *testing.T) {
	if _, err := DecodeMessagesChecked(make([]int16, decodeSamples), DecoderOptions{}); !errors.Is(err, ErrInvalidDecodeInput) {
		t.Fatalf("short input error got %v, want ErrInvalidDecodeInput", err)
	} else {
		var inputErr *DecodeInputError
		if !errors.As(err, &inputErr) || inputErr.GotSamples != decodeSamples || inputErr.WantSamples != SlotSamples {
			t.Fatalf("typed input error got %#v", err)
		}
	}
	if _, err := DecodeMessagesChecked(make([]int16, SlotSamples), DecoderOptions{MaxCandidates: defaultMaxCand + 1}); !errors.Is(err, ErrInvalidDecoderOptions) {
		t.Fatalf("option error got %v, want ErrInvalidDecoderOptions", err)
	} else {
		var optionErr *DecoderOptionError
		if !errors.As(err, &optionErr) || optionErr.Field != "MaxCandidates" {
			t.Fatalf("typed option error got %#v", err)
		}
	}
	report, err := DecodeMessagesChecked(make([]int16, SlotSamples), DecoderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Messages) != 0 || report.Diagnostics.ExpectedSamples != SlotSamples {
		t.Fatalf("silence report got %+v", report)
	}
}

func TestDecoderCheckedSynthesizedFrame(t *testing.T) {
	encoded, err := EncodeStandardMessage("CQ 7Q5MLV KH67")
	if err != nil {
		t.Fatal(err)
	}
	short := synthesizeOracleWave(encoded.Tones, 1500)
	slot := make([]int16, SlotSamples)
	copy(slot, short)
	report, err := NewDecoder().DecodeMessagesChecked(slot)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Messages) != 1 || report.Messages[0].Text != encoded.Text {
		t.Fatalf("stateful checked decode got %+v", report.Messages)
	}
}
