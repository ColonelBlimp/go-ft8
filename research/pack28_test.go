// pack28_test.go — Tests for pack28, ihashcall, stdcall, and ComputeAPSymbols.
//
// Verifies faithful port from wsjt-wsjtx/lib/77bit/packjt77.f90 and
// wsjt-wsjtx/lib/ft8/ft8apset.f90.

package research

import (
	"fmt"
	"strings"
	"testing"
)

// ── stdcall tests ───────────────────────────────────────────────────────────

func TestStdcall(t *testing.T) {
	// Standard callsigns (should return true)
	stdCalls := []string{
		"K1ABC", "W3DQS", "VE1WT", "UA1CEI", "SV2SIH",
		"A61CK", "HA5LB", "ZS4AW", "JR3UIC", "SP7IIT",
		"KB3Z", "NH6D", "R7HL", "VK5BU",
	}
	for _, cs := range stdCalls {
		if !stdcall(cs) {
			t.Errorf("stdcall(%q) = false, want true", cs)
		}
	}

	// Non-standard callsigns (should return false)
	nonStd := []string{
		"", "A", "AB", // too short after trim
		"3DA0XYZ", // Swaziland prefix (>6 chars without /)
		"ABCDEFG", // no digit
		"123ABC",  // too many leading digits
		"K1ABCD",  // suffix too long (4 letters)
	}
	for _, cs := range nonStd {
		if stdcall(cs) {
			t.Errorf("stdcall(%q) = true, want false", cs)
		}
	}
}

// ── pack28 tests ────────────────────────────────────────────────────────────

func TestPack28SpecialTokens(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"DE", 0},
		{"QRZ", 1},
		{"CQ", 2},
	}
	for _, tt := range tests {
		got := pack28(tt.input)
		if got != tt.want {
			t.Errorf("pack28(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestPack28StandardCalls(t *testing.T) {
	// For a standard call, the Fortran algorithm produces:
	// n28 = 36*10*27*27*27*i1 + 10*27*27*27*i2 + 27*27*27*i3 + 27*27*i4 + 27*i5 + i6 + NTOKENS + MAX22
	//
	// Using the alphabet tables:
	//   a1 = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ" (index 0-36)
	//   a2 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"   (index 0-35)
	//   a3 = "0123456789"                              (index 0-9)
	//   a4 = " ABCDEFGHIJKLMNOPQRSTUVWXYZ"             (index 0-26)

	tests := []struct {
		call string
		// Expected n28 computed manually from the Fortran algorithm
		want int
	}{
		// K1ABC: iarea=2(1-based), call6=" K1ABC"
		// i1=idx(a1,' ')=0, i2=idx(a2,'K')=20, i3=idx(a3,'1')=1
		// i4=idx(a4,'A')=1, i5=idx(a4,'B')=2, i6=idx(a4,'C')=3
		// n = 0 + 10*27^3*20 + 27^3*1 + 27^2*1 + 27*2 + 3 = 3956769
		// n28 = 3956769 + 2063592 + 4194304 = 10214665
		{"K1ABC", 36*10*27*27*27*0 + 10*27*27*27*20 + 27*27*27*1 + 27*27*1 + 27*2 + 3 + packNTOKENS + packMAX22},
		// W3DQS: iarea=2(1-based), call6=" W3DQS"
		// i1=idx(a1,' ')=0, i2=idx(a2,'W')=32, i3=idx(a3,'3')=3
		// i4=idx(a4,'D')=4, i5=idx(a4,'Q')=17, i6=idx(a4,'S')=19
		{"W3DQS", 36*10*27*27*27*0 + 10*27*27*27*32 + 27*27*27*3 + 27*27*4 + 27*17 + 19 + packNTOKENS + packMAX22},
		// VE1WT: iarea=3(1-based), call6="VE1WT "
		// i1=idx(a1,'V')=32, i2=idx(a2,'E')=14, i3=idx(a3,'1')=1
		// i4=idx(a4,'W')=23, i5=idx(a4,'T')=20, i6=idx(a4,' ')=0
		{"VE1WT", 36*10*27*27*27*32 + 10*27*27*27*14 + 27*27*27*1 + 27*27*23 + 27*20 + 0 + packNTOKENS + packMAX22},
	}

	for _, tt := range tests {
		got := pack28(tt.call)
		if got != tt.want {
			t.Errorf("pack28(%q) = %d, want %d (diff=%d)", tt.call, got, tt.want, got-tt.want)
		}
	}
}

// TestPack28RoundTrip verifies that pack28 → bits → Unpack77 recovers the
// original callsign. This is the strongest correctness test: it exercises
// pack28's output against the independently ported Unpack77.
func TestPack28RoundTrip(t *testing.T) {
	// Standard callsigns only — special tokens (CQ, DE, QRZ) use a
	// different message format (n28<NTOKENS) and need special handling
	// in the c77 layout that this simple test doesn't cover.
	calls := []string{
		"K1ABC", "W3DQS", "VE1WT", "UA1CEI", "SV2SIH",
		"A61CK", "HA5LB", "ZS4AW", "JR3UIC", "SP7IIT",
		"KB3Z", "NH6D", "R7HL", "VK5BU", "PV8AJ",
	}

	for _, call1 := range calls {
		for _, call2 := range []string{"K1ABC", "W3DQS"} {
			if call1 == call2 {
				continue
			}

			// Build a type-1 message: call1 call2 RRR
			n28a := pack28(call1)
			n28b := pack28(call2)
			ipa := 0
			ipb := 0
			ir := 0
			irpt := 2 // RRR
			igrid4 := 32400 + irpt
			i3 := 1

			// Write 77-bit c77 string matching Fortran format:
			// n28a(28) ipa(1) n28b(28) ipb(1) ir(1) igrid4(15) i3(3)
			var bits [77]byte
			for i := 0; i < 28; i++ {
				bits[i] = byte((n28a >> uint(27-i)) & 1)
			}
			bits[28] = byte(ipa)
			for i := 0; i < 28; i++ {
				bits[29+i] = byte((n28b >> uint(27-i)) & 1)
			}
			bits[57] = byte(ipb)
			bits[58] = byte(ir)
			for i := 0; i < 15; i++ {
				bits[59+i] = byte((igrid4 >> uint(14-i)) & 1)
			}
			for i := 0; i < 3; i++ {
				bits[74+i] = byte((i3 >> uint(2-i)) & 1)
			}

			c77 := ""
			for _, b := range bits {
				c77 += fmt.Sprintf("%d", b)
			}

			msg, ok := Unpack77(c77)
			if !ok {
				t.Errorf("Unpack77 failed for %s %s RRR (c77=%s)", call1, call2, c77)
				continue
			}

			msg = strings.TrimSpace(msg)
			// Check that call1 appears in the unpacked message
			if !strings.Contains(msg, strings.TrimSpace(call1)) {
				t.Errorf("pack28 round-trip: packed %q %q RRR → %q (missing call1)",
					call1, call2, msg)
			}
		}
	}
}

// ── ihashcall tests ─────────────────────────────────────────────────────────

func TestIhashcall(t *testing.T) {
	// Verify determinism and range
	h10 := ihashcall("K1ABC", 10)
	h12 := ihashcall("K1ABC", 12)
	h22 := ihashcall("K1ABC", 22)

	if h10 < 0 || h10 >= 1024 {
		t.Errorf("ihashcall(K1ABC, 10) = %d, out of range [0, 1023]", h10)
	}
	if h12 < 0 || h12 >= 4096 {
		t.Errorf("ihashcall(K1ABC, 12) = %d, out of range [0, 4095]", h12)
	}
	if h22 < 0 || h22 >= 4194304 {
		t.Errorf("ihashcall(K1ABC, 22) = %d, out of range [0, 4194303]", h22)
	}

	// Same input → same output
	if ihashcall("K1ABC", 10) != h10 {
		t.Error("ihashcall is not deterministic")
	}

	// Different inputs → (very likely) different outputs
	h2 := ihashcall("W3DQS", 22)
	if h2 == h22 {
		t.Error("ihashcall collision for K1ABC and W3DQS (unlikely)")
	}
}

// ── ComputeAPSymbols tests ──────────────────────────────────────────────────

func TestComputeAPSymbolsSentinels(t *testing.T) {
	// Empty mycall → sentinel
	apsym := ComputeAPSymbols("", "K1ABC")
	if apsym[0] != 99 {
		t.Errorf("ComputeAPSymbols(\"\", K1ABC)[0] = %d, want 99 (sentinel)", apsym[0])
	}

	// Short mycall → sentinel
	apsym = ComputeAPSymbols("AB", "K1ABC")
	if apsym[0] != 99 {
		t.Errorf("ComputeAPSymbols(\"AB\", K1ABC)[0] = %d, want 99 (sentinel)", apsym[0])
	}

	// Valid mycall, empty hiscall → hiscall sentinel
	apsym = ComputeAPSymbols("K1ABC", "")
	if apsym[0] == 99 {
		t.Error("ComputeAPSymbols(K1ABC, \"\")[0] = 99, want valid (not sentinel)")
	}
	if apsym[29] != 99 {
		t.Errorf("ComputeAPSymbols(K1ABC, \"\")[29] = %d, want 99 (sentinel)", apsym[29])
	}

	// Both valid → no sentinels
	apsym = ComputeAPSymbols("K1ABC", "W3DQS")
	if apsym[0] == 99 {
		t.Error("ComputeAPSymbols(K1ABC, W3DQS)[0] = 99, want valid")
	}
	if apsym[29] == 99 {
		t.Error("ComputeAPSymbols(K1ABC, W3DQS)[29] = 99, want valid")
	}
}

func TestComputeAPSymbolsValues(t *testing.T) {
	// All non-sentinel values must be ±1
	apsym := ComputeAPSymbols("K1ABC", "W3DQS")
	for i, v := range apsym {
		if v != 1 && v != -1 {
			t.Errorf("apsym[%d] = %d, want ±1", i, v)
		}
	}

	// ipa bit (index 28) should be -1 (bit=0 → 2*0-1=-1)
	if apsym[28] != -1 {
		t.Errorf("apsym[28] (ipa) = %d, want -1", apsym[28])
	}
	// ipb bit (index 57) should be -1
	if apsym[57] != -1 {
		t.Errorf("apsym[57] (ipb) = %d, want -1", apsym[57])
	}
}

func TestComputeAPSymbolsConsistentWithPack28(t *testing.T) {
	// Verify that the first 28 bits of apsym match pack28(mycall)
	mycall := "K1ABC"
	hiscall := "W3DQS"
	apsym := ComputeAPSymbols(mycall, hiscall)

	n28a := pack28(mycall)
	for i := 0; i < 28; i++ {
		bit := (n28a >> uint(27-i)) & 1
		expected := 2*bit - 1
		if apsym[i] != expected {
			t.Errorf("apsym[%d] = %d, want %d (from pack28 bit %d of n28=%d)",
				i, apsym[i], expected, bit, n28a)
		}
	}

	// Verify bits 29-56 match pack28(hiscall)
	n28b := pack28(hiscall)
	for i := 0; i < 28; i++ {
		bit := (n28b >> uint(27-i)) & 1
		expected := 2*bit - 1
		if apsym[29+i] != expected {
			t.Errorf("apsym[%d] = %d, want %d (from pack28 bit %d of n28=%d)",
				29+i, apsym[29+i], expected, bit, n28b)
		}
	}
}

// TestComputeAPSymbolsRoundTrip verifies that the apsym bits, when converted
// back to a c77 string and unpacked, produce the original callsigns.
func TestComputeAPSymbolsRoundTrip(t *testing.T) {
	mycall := "VE1WT"
	hiscall := "K1ABC"
	apsym := ComputeAPSymbols(mycall, hiscall)

	// Build a type-1 c77 from the apsym bits + RRR payload + i3=1
	var bits [77]byte
	for i := 0; i < 58; i++ {
		if apsym[i] == 1 {
			bits[i] = 1
		} else {
			bits[i] = 0
		}
	}
	// ir=0, irpt=2(RRR), igrid4=32402, i3=1
	ir := 0
	igrid4 := 32402
	i3val := 1
	bits[58] = byte(ir)
	for i := 0; i < 15; i++ {
		bits[59+i] = byte((igrid4 >> uint(14-i)) & 1)
	}
	for i := 0; i < 3; i++ {
		bits[74+i] = byte((i3val >> uint(2-i)) & 1)
	}

	c77 := ""
	for _, b := range bits {
		c77 += fmt.Sprintf("%d", b)
	}

	msg, ok := Unpack77(c77)
	if !ok {
		t.Fatalf("Unpack77 failed for apsym round-trip (c77=%s)", c77)
	}

	msg = strings.TrimSpace(msg)
	if !strings.Contains(msg, mycall) {
		t.Errorf("apsym round-trip: expected %q in message, got %q", mycall, msg)
	}
	if !strings.Contains(msg, hiscall) {
		t.Errorf("apsym round-trip: expected %q in message, got %q", hiscall, msg)
	}
	if !strings.Contains(msg, "RRR") {
		t.Errorf("apsym round-trip: expected RRR in message, got %q", msg)
	}
	t.Logf("Round-trip: %s %s RRR → c77 → %q", mycall, hiscall, msg)
}
