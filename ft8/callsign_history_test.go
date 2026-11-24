// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"reflect"
	"strings"
	"testing"
)

func TestCallsignHash(t *testing.T) {
	// Expected hashes come from the existing on-air-format golden payloads.
	dx := payload77FromGoldenBits(t, unpack77GoldenVectors[0].payload)
	eu := payload77FromGoldenBits(t, unpack77GoldenVectors[3].payload)
	for _, test := range []struct {
		call string
		bits int
		want int
	}{
		{"KH1/KH7Z", 10, dx.readBits(56, 10)},
		{"  <kh1/kh7z>  ", 10, dx.readBits(56, 10)},
		{"g4abc/p", 12, eu.readBits(0, 12)},
		{"PA9XYZ", 22, eu.readBits(12, 22)},
	} {
		if got, ok := CallsignHash(test.call, test.bits); !ok || got != test.want {
			t.Errorf("CallsignHash(%q, %d) = %d, %v; want %d, true", test.call, test.bits, got, ok, test.want)
		}
	}
	for _, width := range []int{-1, 0, 9, 11, 13, 21, 23, 64, 100} {
		if got, ok := CallsignHash("DX1ABC", width); ok || got != 0 {
			t.Errorf("accepted width %d: %d, %v", width, got, ok)
		}
	}
	var history CallsignHistory
	for _, call := range []string{"", "...", "<...>", "AB", "CQ", "RR73", "<DX1ABC", "DX1ABC>", "<<DX1ABC>>", "DX1?ABC", "DX1 ABC", "DX1\tABC", "DX1ÉBC", "dx1\u017fbc", "\u00a0DX1ABC", "DX1234567890"} {
		if got, ok := CallsignHash(call, 10); ok || got != 0 {
			t.Errorf("accepted invalid call %q: %d, %v", call, got, ok)
		}
		history.RememberCallsigns(call)
	}
	if len(history.hashes.calls22) != 0 {
		t.Fatalf("invalid seeds entered history: %+v", history.hashes.calls22)
	}
}

func TestCallsignHistoryLearningAndIsolation(t *testing.T) {
	encoded, err := EncodeStandardMessage("W1AAA DX1ABC -10")
	if err != nil {
		t.Fatal(err)
	}
	var history CallsignHistory
	if text, status := history.ParsePayload77(Payload77FromBits(encoded.Bits77)); status != ParseStatusParsed || text != encoded.Text {
		t.Fatalf("learning parse = %q, %s", text, status)
	}
	if history.hashes.Lookup22(hashCall("W1AAA", 22)) != "" {
		t.Fatal("first call was learned, unlike FT8")
	}
	for _, width := range []int{10, 12, 22} {
		if got := historyHashLookup(&history, width, hashCall("DX1ABC", width)); got != "DX1ABC" {
			t.Errorf("%d-bit lookup = %q", width, got)
		}
	}
	var fresh CallsignHistory
	if fresh.hashes.Lookup22(hashCall("DX1ABC", 22)) != "" {
		t.Fatal("history shared across instances")
	}

	type4, err := EncodeStandardMessage("<DX1ABC> K1ABC/P RR73")
	if err != nil {
		t.Fatal(err)
	}
	payload := Payload77FromBits(type4.Bits77)
	if text, status := history.ParsePayload77(payload); status != ParseStatusParsed || text != "<DX1ABC> K1ABC/P RR73" {
		t.Fatalf("type 4 parse = %q, %s", text, status)
	}
	if got := history.hashes.Lookup22(hashCall("K1ABC/P", 22)); got != "K1ABC/P" {
		t.Fatalf("spelled call learned as %q", got)
	}
	if text, _ := ParsePayload77(payload); text != "<...> K1ABC/P RR73" {
		t.Fatalf("stateless parse used history: %q", text)
	}
}

func historyHashLookup(history *CallsignHistory, width, hash int) string {
	switch width {
	case 10:
		return history.hashes.Lookup10(hash)
	case 12:
		return history.hashes.Lookup12(hash)
	default:
		return history.hashes.Lookup22(hash)
	}
}

func TestCallsignHistoryRejectsInvalidPayloadWithoutLearning(t *testing.T) {
	bits, ok := pack77StandardMessage("CQ TEST DX1ABC FN42")
	if !ok {
		t.Fatal("pack standard message")
	}
	// A directed CQ with a response bit is invalid, after the call's Save is queued.
	writeBits(bits[:], 58, 1, 1)
	var history CallsignHistory
	history.RememberCallsigns("W1AAA")
	before := append([]hashEntry(nil), history.hashes.calls22...)
	if text, status := history.ParsePayload77(payload77FromBits(bits[:])); status != ParseStatusInvalid || text != "" {
		t.Fatalf("invalid parse = %q, %s", text, status)
	}
	if !reflect.DeepEqual(history.hashes.calls22, before) {
		t.Fatalf("invalid payload changed history: %+v", history.hashes.calls22)
	}
}

func TestCallsignHistoryDoesNotLearnProtocolTokens(t *testing.T) {
	standard, ok := pack77StandardMessage("W1AAA DX1ABC FN42")
	if !ok {
		t.Fatal("pack standard message")
	}
	type4, ok := pack77StandardMessage("<DX1ABC> K1ABC/P RR73")
	if !ok {
		t.Fatal("pack type 4 message")
	}
	tests := []struct {
		name string
		bits [77]int8
	}{}
	for _, token := range []struct {
		name   string
		n28    int
		suffix int
	}{
		{"DE", 0, 0}, {"QRZ", 1, 0}, {"CQ", 2, 0},
		{"QRZ/R", 1, 1}, {"CQ 123/R", 126, 1},
		{"reserved", ft8NTokens - 1, 0},
	} {
		bits := standard
		writeBits(bits[:], 29, 28, token.n28)
		writeBits(bits[:], 57, 1, token.suffix)
		tests = append(tests, struct {
			name string
			bits [77]int8
		}{token.name, bits})
	}
	for _, text := range []string{"DX1 ABC", "QRZ", "CQ"} {
		n58, ok := packBase38Call(text)
		if !ok {
			t.Fatalf("pack type 4 text %q", text)
		}
		bits := type4
		writeBits64(bits[:], 12, 58, n58)
		tests = append(tests, struct {
			name string
			bits [77]int8
		}{"type 4 " + text, bits})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var history CallsignHistory
			history.RememberCallsigns("DX1ABC")
			before := append([]hashEntry(nil), history.hashes.calls22...)
			payload := payload77FromBits(test.bits[:])
			if _, status := history.ParsePayload77(payload); status != ParseStatusParsed {
				t.Fatalf("legacy message rendering was rejected: %s", status)
			}
			if !reflect.DeepEqual(before, history.hashes.calls22) {
				t.Fatalf("non-call was learned: %+v", history.hashes.calls22)
			}
			decoder := NewDecoder()
			parsed := parseCandidatePayload(encode17491(test.bits), &decoder.hashes)
			if parsed.Status != ParseStatusParsed || len(decoder.hashes.calls22) != 0 {
				t.Fatalf("FT8 token parse: %+v, history %+v", parsed, decoder.hashes.calls22)
			}
		})
	}

	// Reproduce the reported false attribution after receiving a QRZ token.
	writeBits(standard[:], 29, 28, 1)
	var history CallsignHistory
	history.ParsePayload77(payload77FromBits(standard[:]))
	dx := payload77FromGoldenBits(t, unpack77GoldenVectors[0].payload).int8Bits()
	writeBits(dx[:], 56, 10, hashCall("QRZ", 10))
	text, status := history.ParsePayload77(payload77FromBits(dx[:]))
	if status != ParseStatusParsed || !strings.Contains(text, "<...>") || strings.Contains(text, "<QRZ>") {
		t.Fatalf("protocol token attributed as sender: %q, %s", text, status)
	}
}

func TestRememberCallsignsSharesDecoderRules(t *testing.T) {
	older, newer, hash := findHash12Collision(t)
	decoder := NewDecoder()
	decoder.RememberCallsigns(" <"+strings.ToLower(older)+"> ", newer)
	if got := decoder.hashes.Lookup12(hash); got != newer {
		t.Fatalf("collision = %q, want newest %q", got, newer)
	}
	decoder.RememberCallsigns(older)
	if got := decoder.hashes.Lookup12(hash); got != older {
		t.Fatalf("refreshed collision = %q, want %q", got, older)
	}
	for i := 0; i < 1100; i++ {
		decoder.RememberCallsigns(hashTestCall(10000 + i))
	}
	if len(decoder.hashes.calls22) != ft8MaxHash || containsHashCall(decoder.hashes.calls22, older) {
		t.Fatalf("seeds did not respect rolling cap: %d entries", len(decoder.hashes.calls22))
	}
	if NewDecoder().hashes.Lookup12(hash) != "" {
		t.Fatal("seed history shared across decoders")
	}

	decoder = NewDecoder()
	decoder.SetAPCallHints([]APCallHint{{Call: "DX1ABC"}})
	if len(decoder.hashes.calls22) != 0 {
		t.Fatal("AP hints seeded rendering history")
	}
	options, rawOptions, mode := decoder.options, decoder.rawOptions, decoder.mode
	decoder.RememberCallsigns("dx1abc")
	if !reflect.DeepEqual(options, decoder.options) || !reflect.DeepEqual(rawOptions, decoder.rawOptions) || mode != decoder.mode {
		t.Fatal("remembering calls changed AP options or mode")
	}
	golden := payload77FromGoldenBits(t, unpack77GoldenVectors[0].payload)
	decoder.RememberCallsigns("kh1/kh7z")
	var cw [174]int8
	bits := golden.int8Bits()
	copy(cw[:77], bits[:])
	if parsed := parseCandidatePayload(cw, &decoder.hashes); parsed.Text != unpack77GoldenVectors[0].text {
		t.Fatalf("FT8 seeded render = %q", parsed.Text)
	}
}
