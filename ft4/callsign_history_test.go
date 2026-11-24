// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

import (
	"errors"
	"strings"
	"testing"

	"github.com/ColonelBlimp/go-ft8/ft8"
	"github.com/ColonelBlimp/go-ft8/internal/ftx77"
)

// These frames exercise source formats through the real FT4 signal decoder,
// including specialized payloads that the text encoder does not yet accept.
func historyPayloads(t *testing.T) map[string]Payload77 {
	t.Helper()
	standard, err := EncodeStandardMessage("W1AAA DX1ABC -10")
	if err != nil {
		t.Fatal(err)
	}
	type4, err := EncodeStandardMessage("<DX1ABC> K1ABC/P RR73")
	if err != nil {
		t.Fatal(err)
	}
	other, err := EncodeStandardMessage("N3CCC W9XYZ -10")
	if err != nil {
		t.Fatal(err)
	}
	var dx [77]byte
	copy(dx[:28], standard.Bits77[:28])
	copy(dx[28:56], other.Bits77[:28])
	hash10, _ := ft8.CallsignHash("DX1ABC", 10)
	setHistoryBits(&dx, 56, 10, hash10)
	setHistoryBits(&dx, 66, 5, 10) // -10 dB
	setHistoryBits(&dx, 71, 3, 1)
	bits22 := other.Bits77
	hash22, _ := ft8.CallsignHash("DX1ABC", 22)
	setHistoryBits(&bits22, 0, 28, 2063592+hash22)
	return map[string]Payload77{
		"standard": ft8.Payload77FromBits(standard.Bits77),
		"10":       ft8.Payload77FromBits(dx),
		"12":       ft8.Payload77FromBits(type4.Bits77),
		"22":       ft8.Payload77FromBits(bits22),
	}
}

func setHistoryBits(bits *[77]byte, start, width, value int) {
	for i := 0; i < width; i++ {
		bits[start+i] = byte(value>>uint(width-1-i)) & 1
	}
}

func historyWave(payload Payload77, frequency float64) []int16 {
	bits := payload.Bits()
	var scrambled [77]int8
	for i := range bits {
		scrambled[i] = int8(bits[i]) ^ scramblingVector[i]
	}
	wave := synthesizeOracleWave(tonesFromCodeword(ftx77.Encode17491(scrambled)), frequency)
	slot := make([]int16, SlotSamples)
	copy(slot, wave)
	return slot
}

func decodeHistorySlot(t *testing.T, decoder *Decoder, method int, samples []int16) []DecodedMessage {
	t.Helper()
	switch method {
	case 0:
		return decoder.DecodeMessages(samples)
	case 1:
		return decoder.DecodeMessagesWithReport(samples).Messages
	default:
		report, err := decoder.DecodeMessagesChecked(samples)
		if err != nil {
			t.Fatal(err)
		}
		return report.Messages
	}
}

func requireHistoryText(t *testing.T, messages []DecodedMessage, want string) {
	t.Helper()
	if len(messages) != 1 || messages[0].ParseStatus != ParseStatusParsed || messages[0].Text != want {
		t.Fatalf("messages = %+v, want %q", messages, want)
	}
}

func TestDecoderCallsignHistoryAcrossSlots(t *testing.T) {
	payloads := historyPayloads(t)
	standardWave := historyWave(payloads["standard"], 1500)
	for _, width := range []string{"10", "12", "22"} {
		t.Run(width, func(t *testing.T) {
			payload := payloads[width]
			wave := historyWave(payload, 1500)
			unresolved, _ := ft8.ParsePayload77(payload)
			resolved := strings.ReplaceAll(unresolved, "<...>", "<DX1ABC>")
			for learningMethod := 0; learningMethod < 3; learningMethod++ {
				decoder := NewDecoder()
				requireHistoryText(t, decodeHistorySlot(t, decoder, learningMethod, standardWave), "W1AAA DX1ABC -10")
				for method := 0; method < 3; method++ {
					if messages := decodeHistorySlot(t, decoder, method, make([]int16, SlotSamples)); len(messages) != 0 {
						t.Fatalf("silent slot decoded %+v", messages)
					}
					requireHistoryText(t, decodeHistorySlot(t, decoder, method, wave), resolved)
				}
			}
			requireHistoryText(t, NewDecoder().DecodeMessages(wave), unresolved)
			requireHistoryText(t, DecodeMessages(wave), unresolved)
			seeded := NewDecoderWithOptions(DecoderOptions{})
			seeded.RememberCallsigns(" <dx1abc> ", "<...>", "bad?call")
			requireHistoryText(t, seeded.DecodeMessages(wave), resolved)
		})
	}
}

func TestDecoderCallsignHistorySameSlot(t *testing.T) {
	payloads := historyPayloads(t)
	full := historyWave(payloads["standard"], 2100)
	hashed := historyWave(payloads["10"], 900)
	for _, learnFirst := range []bool{true, false} {
		mixed := make([]int16, SlotSamples)
		for i := range mixed {
			if learnFirst {
				mixed[i] = full[i]/2 + hashed[i]/10
			} else {
				mixed[i] = full[i]/10 + hashed[i]/2
			}
		}
		messages := NewDecoder().DecodeMessages(mixed)
		want := "W1AAA RR73; N3CCC <...> -10"
		if learnFirst {
			want = "W1AAA RR73; N3CCC <DX1ABC> -10"
		}
		if len(messages) != 2 || messages[0].Text != want {
			t.Fatalf("learn first %v: %+v, want hash text %q", learnFirst, messages, want)
		}
		// Package-level FT4 decoding deliberately remains history-free.
		messages = DecodeMessages(mixed)
		if len(messages) != 2 || messages[0].Text != "W1AAA RR73; N3CCC <...> -10" {
			t.Fatalf("stateless mixed slot = %+v", messages)
		}
	}
}

func TestDecoderCallsignHistoryCheckedErrors(t *testing.T) {
	payloads := historyPayloads(t)
	other, err := EncodeStandardMessage("W1AAA W9XYZ -10")
	if err != nil {
		t.Fatal(err)
	}
	otherWave := historyWave(ft8.Payload77FromBits(other.Bits77), 1500)
	decoder := NewDecoder()
	decoder.RememberCallsigns("DX1ABC")
	if _, err := decoder.DecodeMessagesChecked(otherWave[:len(otherWave)-1]); !errors.Is(err, ErrInvalidDecodeInput) {
		t.Fatalf("invalid input error = %v", err)
	}
	decoder.options.MaxCandidates = defaultMaxCand + 1
	if _, err := decoder.DecodeMessagesChecked(otherWave); !errors.Is(err, ErrInvalidDecoderOptions) {
		t.Fatalf("invalid options error = %v", err)
	}
	decoder.options = DecoderOptions{}
	requireHistoryText(t, decoder.DecodeMessages(historyWave(payloads["10"], 1500)), "W1AAA RR73; N3CCC <DX1ABC> -10")
	unlearned, err := EncodeStandardMessage("<W9XYZ> K1ABC/P RR73")
	if err != nil {
		t.Fatal(err)
	}
	requireHistoryText(t, decoder.DecodeMessages(historyWave(ft8.Payload77FromBits(unlearned.Bits77), 1500)), "<...> K1ABC/P RR73")
}
