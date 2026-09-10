// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

import (
	"errors"
	"math"
	"testing"

	"github.com/ColonelBlimp/go-ft8/ft8"
)

func TestEncodeStandardMessageFrameConstruction(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "cq test k1abc fn42", want: "CQ TEST K1ABC FN42"},
		{in: "K1ABC W9XYZ -13", want: "K1ABC W9XYZ -13"},
		{in: "W9XYZ K1ABC R-11", want: "W9XYZ K1ABC R-11"},
		{in: "K1ABC W9XYZ RR73", want: "K1ABC W9XYZ RR73"},
		{in: "K1ABC W9XYZ 6A WI", want: "K1ABC W9XYZ 6A WI"},
		{in: "CQ PJ4/NA2AA", want: "CQ PJ4/NA2AA"},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			got, err := EncodeStandardMessage(test.in)
			if err != nil {
				t.Fatalf("EncodeStandardMessage returned error: %v", err)
			}
			if got.Text != test.want {
				t.Fatalf("Text got %q, want %q", got.Text, test.want)
			}

			ft8Encoded, err := ft8.EncodeStandardMessage(test.in)
			if err != nil {
				t.Fatalf("FT8 source encode returned error: %v", err)
			}
			if got.Bits77 != ft8Encoded.Bits77 {
				t.Fatal("FT4 and FT8 source payloads differ")
			}
			for i, bit := range got.Bits77 {
				want := bit ^ byte(scramblingVector[i])
				if got.ScrambledBits77[i] != want {
					t.Fatalf("ScrambledBits77[%d] got %d, want %d", i, got.ScrambledBits77[i], want)
				}
				if got.Codeword[i] != want {
					t.Fatalf("Codeword[%d] got %d, want systematic bit %d", i, got.Codeword[i], want)
				}
			}
			for i, bit := range got.Codeword {
				if bit > 1 {
					t.Fatalf("Codeword[%d] = %d, want 0 or 1", i, bit)
				}
			}
			for i, tone := range got.Tones {
				if tone > 3 {
					t.Fatalf("Tones[%d] = %d, want 0..3", i, tone)
				}
			}
		})
	}
}

func TestEncodeStandardMessageCostasAndDataPlacement(t *testing.T) {
	encoded, err := EncodeStandardMessage("CQ TEST K1ABC FN42")
	if err != nil {
		t.Fatal(err)
	}

	for block, start := range [...]int{0, 33, 66, 99} {
		for i, want := range costasArrays[block] {
			if got := encoded.Tones[start+i]; got != want {
				t.Fatalf("Costas block %d tone %d got %d, want %d", block, i, got, want)
			}
		}
	}

	dataPositions := make([]int, 0, DataSymbols)
	for _, block := range [...][2]int{{4, 33}, {37, 66}, {70, 99}} {
		for i := block[0]; i < block[1]; i++ {
			dataPositions = append(dataPositions, i)
		}
	}
	if len(dataPositions) != DataSymbols {
		t.Fatalf("collected %d data positions, want %d", len(dataPositions), DataSymbols)
	}
	for i, pos := range dataPositions {
		value := int(encoded.Codeword[2*i])<<1 | int(encoded.Codeword[2*i+1])
		if got, want := encoded.Tones[pos], grayMap[value]; got != want {
			t.Fatalf("data tone %d at position %d got %d, want %d", i, pos, got, want)
		}
	}
}

func TestEncodeStandardMessageRejectsUnsupportedInput(t *testing.T) {
	_, err := EncodeStandardMessage("THIS IS FREE TEXT")
	if !errors.Is(err, ErrUnsupportedStandardMessage) {
		t.Fatalf("error got %v, want ErrUnsupportedStandardMessage", err)
	}
	if errors.Is(err, ft8.ErrUnsupportedStandardMessage) {
		t.Fatal("FT4 encoder leaked the FT8 sentinel")
	}
}

func TestFT4ProtocolConstants(t *testing.T) {
	if SlotSamples != 90000 || SamplesPerSymbol != 576 {
		t.Fatalf("slot/symbol samples got %d/%d", SlotSamples, SamplesPerSymbol)
	}
	if DataSymbols != 87 || SyncSymbols != 16 || ToneSymbols != 103 {
		t.Fatalf("symbol counts got data=%d sync=%d tones=%d", DataSymbols, SyncSymbols, ToneSymbols)
	}
	if WaveformSamples != 60480 || math.Abs(WaveformSeconds-5.04) > 1e-12 {
		t.Fatalf("waveform geometry got samples=%d seconds=%v", WaveformSamples, WaveformSeconds)
	}
	if math.Abs(ToneSpacingHz-20.833333333333332) > 1e-12 || math.Abs(SignalWidthHz-83.33333333333333) > 1e-12 {
		t.Fatalf("tone geometry got spacing=%v width=%v", ToneSpacingHz, SignalWidthHz)
	}
	if math.Abs(SyncStartSeconds-0.5) > 1e-12 || math.Abs(NominalStartSeconds-0.452) > 1e-12 {
		t.Fatalf("nominal timing got waveform=%v sync=%v", NominalStartSeconds, SyncStartSeconds)
	}
}
