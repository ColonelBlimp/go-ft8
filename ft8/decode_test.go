// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestWinsorizeLLRUsesEvenMedianAverage(t *testing.T) {
	var llr [174]float64
	llr[0] = 1
	llr[1] = -3

	winsorizeLLR(&llr, 1)

	if llr[0] != 1 {
		t.Fatalf("llr[0] got %v, want 1", llr[0])
	}
	if llr[1] != -2 {
		t.Fatalf("llr[1] got %v, want -2", llr[1])
	}
}

func TestRoverCandidateIsExposedAndCommitsHashTable(t *testing.T) {
	bits, ok := pack77StandardMessage("K1ABC W9XYZ/R -10")
	if !ok {
		t.Fatal("failed to pack /R message")
	}
	var hashes hashTable
	parsed := parseCandidatePayload(encode17491(bits), &hashes)
	if parsed.Status != ParseStatusParsed || parsed.Text != "K1ABC W9XYZ/R -10" {
		t.Fatalf("candidate got status=%s text=%q", parsed.Status, parsed.Text)
	}
	if got := hashes.Lookup22(hashCall("W9XYZ/R", 22)); got != "W9XYZ/R" {
		t.Fatalf("committed hash got %q, want W9XYZ/R", got)
	}
}

func TestAcceptedCandidateCommitsHashTable(t *testing.T) {
	bits, ok := pack77StandardMessage("K1ABC W9XYZ -10")
	if !ok {
		t.Fatal("failed to pack accepted message")
	}
	var hashes hashTable
	parsed := parseCandidatePayload(encode17491(bits), &hashes)
	if parsed.Status != ParseStatusParsed || parsed.Text != "K1ABC W9XYZ -10" {
		t.Fatalf("candidate got status=%s text=%q", parsed.Status, parsed.Text)
	}
	if got := hashes.Lookup22(hashCall("W9XYZ", 22)); got != "W9XYZ" {
		t.Fatalf("committed hash got %q, want W9XYZ", got)
	}
}

func TestAcceptedCandidateDoesNotMutateFrozenHashTable(t *testing.T) {
	bits, ok := pack77StandardMessage("K1ABC W9XYZ -10")
	if !ok {
		t.Fatal("failed to pack accepted message")
	}
	var hashes hashTable
	frozen := hashes.frozenSnapshot()
	parsed := parseCandidatePayload(encode17491(bits), frozen)
	if parsed.Status != ParseStatusParsed {
		t.Fatalf("status got %s, want parsed", parsed.Status)
	}
	if len(frozen.calls22) != 0 {
		t.Fatalf("worker snapshot retained %d hash entries, want 0", len(frozen.calls22))
	}
}

func TestDecodeLLRPassReturnsUnparsedPayloads(t *testing.T) {
	var unsupported [77]int8
	writeBits(unsupported[:], 71, 3, 6)

	var reserved [77]int8
	writeBits(reserved[:], 74, 3, 6)

	var invalid [77]int8
	writeBits(invalid[:], 49, 25, 18*18*10*10*24*24)
	writeBits(invalid[:], 74, 3, 5)

	tests := []struct {
		name   string
		bits   [77]int8
		status ParseStatus
	}{
		{name: "assigned unsupported", bits: unsupported, status: ParseStatusUnsupported},
		{name: "reserved", bits: reserved, status: ParseStatusUnknownType},
		{name: "known family invalid", bits: invalid, status: ParseStatusInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			llr := llrForPayload(test.bits)
			decoded, ok := decodeLLRPass(&llr, &ft8NoAPMask, "", "", true, nil,
				normalizeDecoderOptions(DecoderOptions{}), nil)
			if !ok {
				t.Fatalf("decodeLLRPass rejected a CRC-valid payload with status %s", test.status)
			}
			wantPayload := payload77FromBits(test.bits[:])
			if decoded.Payload != wantPayload || decoded.ParseStatus != test.status || decoded.Text != "" {
				t.Fatalf("decoded got payload=%v status=%s text=%q, want original payload/%s/empty",
					decoded.Payload, decoded.ParseStatus, decoded.Text, test.status)
			}
			if decoded.Provenance.Algorithm != DecodeAlgorithmBP || decoded.Provenance.APProfile != "" || decoded.Provenance.APSource != "" {
				t.Fatalf("provenance got %+v, want unassisted BP", decoded.Provenance)
			}
		})
	}
}

func TestDecodeLLRPassExposesTUSemicolonMessage(t *testing.T) {
	bits := rttyPayloadBits(t, "W9XYZ", "K1ABC", 1, 1, 5, 8000+21)
	llr := llrForPayload(bits)

	decoded, ok := decodeLLRPass(&llr, &ft8NoAPMask, "", "", true, nil,
		normalizeDecoderOptions(DecoderOptions{}), nil)
	if !ok {
		t.Fatal("decodeLLRPass rejected a TU; observation")
	}
	if decoded.Text != "TU; W9XYZ K1ABC R 579 MA" || decoded.ParseStatus != ParseStatusParsed {
		t.Fatalf("decoded got status=%s text=%q", decoded.ParseStatus, decoded.Text)
	}
}

func TestDecodedMessagesExposePayloadStatusAndProvenance(t *testing.T) {
	messages := DecodeMessages(loadCorpusWAV(t, "20m_slot1.wav"))
	if len(messages) == 0 {
		t.Fatal("DecodeMessages returned no messages")
	}
	for i, msg := range messages {
		if msg.ParseStatus != ParseStatusParsed {
			t.Fatalf("message %d status got %s, want parsed", i, msg.ParseStatus)
		}
		if msg.Payload == (Payload77{}) {
			t.Fatalf("message %d has zero payload", i)
		}
		if msg.Provenance.Algorithm != DecodeAlgorithmBP && msg.Provenance.Algorithm != DecodeAlgorithmOSD {
			t.Fatalf("message %d provenance got %+v, want BP or OSD", i, msg.Provenance)
		}
	}
}

func llrForPayload(bits [77]int8) [174]float64 {
	codeword := encode17491(bits)
	var llr [174]float64
	for i, bit := range codeword {
		llr[i] = -ft8ScaleFac
		if bit != 0 {
			llr[i] = ft8ScaleFac
		}
	}
	return llr
}
