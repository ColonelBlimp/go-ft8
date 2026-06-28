// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestAPProfilesForceExpectedBits(t *testing.T) {
	cqDX := findAPProfile(t, ft8BroadAPProfiles, "cq-dx")
	wantCQDX, ok := pack28("CQ_DX")
	if !ok {
		t.Fatal("pack28(CQ_DX) failed")
	}
	if got := readBits(cqDX.bits[:], 0, 28); got != wantCQDX {
		t.Fatalf("cq-dx n28 got %d, want %d", got, wantCQDX)
	}
	if got := readBits(cqDX.mask[:], 0, 29); got != (1<<29)-1 {
		t.Fatalf("cq-dx mask[0:29] got %#x, want all known", got)
	}
	if got := readBits(cqDX.bits[:], 74, 3); got != 1 {
		t.Fatalf("cq-dx i3 got %d, want 1", got)
	}

	cqCOTA := findAPProfile(t, ft8BroadAPProfiles, "cq-cota")
	wantCOTA, ok := pack28("CQ_COTA")
	if !ok {
		t.Fatal("pack28(CQ_COTA) failed")
	}
	if got := readBits(cqCOTA.bits[:], 0, 28); got != wantCOTA {
		t.Fatalf("cq-cota n28 got %d, want %d", got, wantCOTA)
	}
	if got := readBits(cqCOTA.bits[:], 74, 3); got != 1 {
		t.Fatalf("cq-cota i3 got %d, want 1", got)
	}

	cqFD := findAPProfile(t, ft8BroadAPProfiles, "cq-fd")
	wantFD, ok := pack28("CQ_FD")
	if !ok {
		t.Fatal("pack28(CQ_FD) failed")
	}
	if got := readBits(cqFD.bits[:], 0, 28); got != wantFD {
		t.Fatalf("cq-fd n28 got %d, want %d", got, wantFD)
	}
	if got := readBits(cqFD.bits[:], 74, 3); got != 1 {
		t.Fatalf("cq-fd i3 got %d, want 1", got)
	}
}

func TestDecodeLLRPassAPDiagnosticsSuccess(t *testing.T) {
	llr := llrForStandardMessage(t, "CQ DX S56GD JN65", nil)
	cqDX := findAPProfile(t, ft8BroadAPProfiles, "cq-dx")
	var diagnostics DecodeDiagnostics

	decoded, ok := decodeLLRPass(&llr, &cqDX.mask, cqDX.name, cqDX.source, true, nil, normalizeDecoderOptions(DecoderOptions{}), &diagnostics)
	if !ok {
		t.Fatal("decodeLLRPass returned no AP decode")
	}
	if decoded.Text != "CQ DX S56GD JN65" {
		t.Fatalf("decoded text got %q, want CQ DX S56GD JN65", decoded.Text)
	}
	if diagnostics.LDPCAttempts != 1 || diagnostics.MetricLDPCAttempts != 0 || diagnostics.APAttempts != 1 {
		t.Fatalf("attempt counters got total=%d metric=%d ap=%d, want 1/0/1",
			diagnostics.LDPCAttempts, diagnostics.MetricLDPCAttempts, diagnostics.APAttempts)
	}
	if diagnostics.APAttemptsByProfile["cq-dx"] != 1 {
		t.Fatalf("cq-dx AP attempts got %d, want 1", diagnostics.APAttemptsByProfile["cq-dx"])
	}
	if diagnostics.APAttemptsBySource["cq-dx"] != 1 {
		t.Fatalf("cq-dx AP source attempts got %d, want 1", diagnostics.APAttemptsBySource["cq-dx"])
	}
	if diagnostics.APSuccesses != 1 || diagnostics.APSuccessesByProfile["cq-dx"] != 1 {
		t.Fatalf("AP successes got total=%d cq-dx=%d, want 1/1",
			diagnostics.APSuccesses, diagnostics.APSuccessesByProfile["cq-dx"])
	}
	if diagnostics.APRejectedAfterLDPC != 0 {
		t.Fatalf("APRejectedAfterLDPC got %d, want 0", diagnostics.APRejectedAfterLDPC)
	}
}

func TestDecodeLLRPassAPDiagnosticsRejectedAfterLDPC(t *testing.T) {
	mutate := func(bits *[77]int8) {
		bits[28] = 1
	}
	llr := llrForStandardMessage(t, "K1ABC W9XYZ RR73", mutate)
	standard := initStandardTypeAPProfile("standard", 1)
	var diagnostics DecodeDiagnostics

	_, ok := decodeLLRPass(&llr, &standard.mask, standard.name, standard.source, true, nil, normalizeDecoderOptions(DecoderOptions{}), &diagnostics)
	if ok {
		t.Fatal("decodeLLRPass accepted AP decode that should be message-filtered")
	}
	if diagnostics.LDPCAttempts != 1 || diagnostics.APAttempts != 1 {
		t.Fatalf("attempt counters got total=%d ap=%d, want 1/1",
			diagnostics.LDPCAttempts, diagnostics.APAttempts)
	}
	if diagnostics.RejectedMessageFilter != 1 {
		t.Fatalf("RejectedMessageFilter got %d, want 1", diagnostics.RejectedMessageFilter)
	}
	if diagnostics.APRejectedAfterLDPC != 1 || diagnostics.APRejectedAfterLDPCByProfile["standard"] != 1 {
		t.Fatalf("AP post-LDPC rejections got total=%d standard=%d, want 1/1",
			diagnostics.APRejectedAfterLDPC, diagnostics.APRejectedAfterLDPCByProfile["standard"])
	}
	if diagnostics.APRejectedAfterLDPCBySource["standard"] != 1 {
		t.Fatalf("standard AP source rejections got %d, want 1", diagnostics.APRejectedAfterLDPCBySource["standard"])
	}
	if diagnostics.APSuccesses != 0 {
		t.Fatalf("APSuccesses got %d, want 0", diagnostics.APSuccesses)
	}
}

func TestNormalizeAPCallHintsCopiesDeduplicatesAndCaps(t *testing.T) {
	hints := make([]APCallHint, 0, ft8MaxAPCallHints+4)
	hints = append(hints,
		APCallHint{Call: "k1abc", Source: "worked", Weight: 1},
		APCallHint{Call: "K1ABC", Source: "duplicate", Weight: 2},
		APCallHint{Call: "not a call", Source: "bad"},
	)
	for i := 0; i < ft8MaxAPCallHints+4; i++ {
		hints = append(hints, APCallHint{Call: testAPCall(i), Source: "bulk"})
	}
	normalized := normalizeAPCallHints(hints)
	if len(normalized) != ft8MaxAPCallHints {
		t.Fatalf("normalized hints got %d, want cap %d", len(normalized), ft8MaxAPCallHints)
	}
	if normalized[0].call != "K1ABC" || normalized[0].source != "worked" || normalized[0].weight != 1 {
		t.Fatalf("first hint got %+v, want normalized first K1ABC worked weight 1", normalized[0])
	}
	for _, hint := range normalized[1:] {
		if hint.call == "K1ABC" {
			t.Fatal("duplicate K1ABC was retained")
		}
	}
}

func TestDecoderSetAPCallHintsCopiesInput(t *testing.T) {
	decoder := NewDecoderWithOptions(DecoderOptions{APCallHints: []APCallHint{{Call: "K1ABC", Source: "ctor"}}})
	hints := []APCallHint{{Call: "W9XYZ", Source: "worked"}}
	decoder.SetAPCallHints(hints)
	hints[0] = APCallHint{Call: "BAD", Source: "mutated"}
	if len(decoder.options.apCallHints) != 1 {
		t.Fatalf("decoder normalized hints got %d, want 1", len(decoder.options.apCallHints))
	}
	if decoder.options.apCallHints[0].call != "W9XYZ" || decoder.options.apCallHints[0].source != "worked" {
		t.Fatalf("decoder hint got %+v, want W9XYZ worked", decoder.options.apCallHints[0])
	}
}

func TestSelectAPCallHintHypothesesPrefersMetricMatch(t *testing.T) {
	metrics := softMetrics{Single: llrForStandardMessage(t, "K1ABC W9XYZ RR73", nil)}
	hints := normalizeAPCallHints([]APCallHint{
		{Call: "N0BAD", Source: "worked"},
		{Call: "W9XYZ", Source: "worked"},
	})
	var diagnostics DecodeDiagnostics
	var selected [ft8MaxAPCallHypotheses]apHintSelection
	n := selectAPCallHintHypotheses(&metrics, hints, 1, &diagnostics, &selected)
	if n != 1 {
		t.Fatalf("selected hypotheses got %d, want 1", n)
	}
	if hints[selected[0].hint].call != "W9XYZ" || selected[0].field != apHintFieldCall2 {
		t.Fatalf("selected got hint=%+v field=%d, want W9XYZ call2", hints[selected[0].hint], selected[0].field)
	}
	if diagnostics.APCallHints != 2 || diagnostics.APHintProfilesScored != 4 || diagnostics.APHintHypothesesSelected != 1 {
		t.Fatalf("hint diagnostics got hints=%d scored=%d selected=%d, want 2/4/1",
			diagnostics.APCallHints, diagnostics.APHintProfilesScored, diagnostics.APHintHypothesesSelected)
	}
}

func TestDecodeAPCallHintsDiagnosticsSuccess(t *testing.T) {
	metrics := softMetrics{Single: llrForStandardMessage(t, "K1ABC W9XYZ RR73", nil)}
	options := normalizeDecoderOptions(DecoderOptions{
		APCallHints:         []APCallHint{{Call: "W9XYZ", Source: "worked"}},
		MaxAPCallHypotheses: 1,
	})
	var diagnostics DecodeDiagnostics
	decoded, ok := decodeAPCallHints(&metrics, nil, options, &diagnostics)
	if !ok {
		t.Fatal("decodeAPCallHints returned no AP decode")
	}
	if decoded.Text != "K1ABC W9XYZ RR73" {
		t.Fatalf("decoded text got %q, want K1ABC W9XYZ RR73", decoded.Text)
	}
	if diagnostics.APAttemptsByProfile["hint-call2"] != 1 {
		t.Fatalf("hint-call2 AP attempts got %d, want 1", diagnostics.APAttemptsByProfile["hint-call2"])
	}
	if diagnostics.APAttemptsBySource["worked"] != 1 || diagnostics.APSuccessesBySource["worked"] != 1 {
		t.Fatalf("worked source attempts/successes got %d/%d, want 1/1",
			diagnostics.APAttemptsBySource["worked"], diagnostics.APSuccessesBySource["worked"])
	}
}

func findAPProfile(t *testing.T, profiles []apProfile, name string) *apProfile {
	t.Helper()
	for i := range profiles {
		if profiles[i].name == name {
			return &profiles[i]
		}
	}
	t.Fatalf("AP profile %q not found", name)
	return nil
}

func llrForStandardMessage(t *testing.T, text string, mutate func(*[77]int8)) [174]float64 {
	t.Helper()
	bits, ok := pack77StandardMessage(text)
	if !ok {
		t.Fatalf("pack77StandardMessage(%q) failed", text)
	}
	if mutate != nil {
		mutate(&bits)
	}
	codeword := encode17491(bits)
	var llr [174]float64
	for i, bit := range codeword {
		llr[i] = -ft8ScaleFac
		if bit == 1 {
			llr[i] = ft8ScaleFac
		}
	}
	return llr
}

func testAPCall(i int) string {
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	return "K" + string(letters[(i/26)%26]) + "1" + string(letters[i%26]) + string(letters[(i/676)%26]) + string(letters[(i/17576)%26])
}
