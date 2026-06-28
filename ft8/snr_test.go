// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestDecodedMessagesExposeSignalReportSNR(t *testing.T) {
	samples := loadCorpusWAV(t, "20m_slot1.wav")
	got := DecodeMessages(samples)
	if len(got) == 0 {
		t.Fatal("DecodeMessages returned no messages")
	}

	want := map[string]int{
		"CQ DX S56GD JN65": 4,
		"CQ A61FJ LL74":    -7,
		"CQ UB6OAK KN88":   -14,
	}
	wantReport := map[string]string{
		"CQ DX S56GD JN65": "+04",
		"CQ A61FJ LL74":    "-07",
		"CQ UB6OAK KN88":   "-14",
	}
	for _, msg := range got {
		snr, ok := want[msg.Text]
		if !ok {
			continue
		}
		if msg.SNR != snr {
			t.Fatalf("%s SNR got %d, want %d", msg.Text, msg.SNR, snr)
		}
		if report := msg.SignalReport(); report != wantReport[msg.Text] {
			t.Fatalf("%s SignalReport got %q, want %q", msg.Text, report, wantReport[msg.Text])
		}
		delete(want, msg.Text)
	}
	for text := range want {
		t.Fatalf("missing decoded message %q", text)
	}
}

func TestSignalReportClampsToEncodableRange(t *testing.T) {
	cases := []struct {
		snr  int
		want string
	}{
		{snr: -60, want: "-50"},
		{snr: -13, want: "-13"},
		{snr: 4, want: "+04"},
		{snr: 60, want: "+49"},
	}
	for _, tc := range cases {
		msg := DecodedMessage{SNR: tc.snr}
		if got := msg.SignalReport(); got != tc.want {
			t.Fatalf("SignalReport(%d) got %q, want %q", tc.snr, got, tc.want)
		}
	}
}

func TestEstimateSNRFallbackUsesOppositeEightToneBin(t *testing.T) {
	var tones [ft8Symbols]int
	var symbolPower [8][ft8Symbols]float64
	for sym := range tones {
		tones[sym] = 7
		symbolPower[7][sym] = 20
		symbolPower[3][sym] = 1
		symbolPower[4][sym] = 10
	}

	if got := estimateSNR(tones, symbolPower, 0); got != -1 {
		t.Fatalf("estimateSNR fallback got %d, want -1", got)
	}
}
