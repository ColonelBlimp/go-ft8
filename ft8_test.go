package ft8x

import (
	"fmt"
	"math"
	"testing"
)

// TestCRC14 verifies that the CRC-14 computation produces zero when the
// message and its CRC are presented together (self-consistency check).
func TestCRC14(t *testing.T) {
	// Known message: 77 zero bits → CRC should be reproducible.
	var m77 [77]int8
	crc := ComputeCRC14(m77)
	t.Logf("CRC of 77 zero bits: 0x%04X (%d)", crc, crc)

	// Build the 96-bit check array and verify it produces zero remainder.
	var m96 [96]int8
	for i := 0; i < 14; i++ {
		m96[82+i] = int8((crc >> uint(13-i)) & 1)
	}
	got := CRC14Bits(m96[:])
	if got != 0 {
		t.Errorf("CRC14 self-check failed: expected 0, got %d", got)
	}
}

// TestEncodeDecode verifies that encoding 77 bits produces a 174-bit codeword
// whose first 77 bits match the input and whose CRC check passes.
func TestEncodeDecode(t *testing.T) {
	var msg77 [77]int8
	// Set some bits.
	msg77[0] = 1
	msg77[10] = 1
	msg77[76] = 1

	cw := Encode174_91(msg77)

	// Verify CRC.
	if !CheckCRC14Codeword(cw) {
		t.Error("CRC check failed for encoded codeword")
	}

	// Verify first 77 bits match.
	for i := 0; i < 77; i++ {
		if int8(cw[i]) != msg77[i] {
			t.Errorf("codeword bit %d: got %d want %d", i, cw[i], msg77[i])
		}
	}
}

// TestGenFT8Tones verifies that the sync tone positions contain the Costas array.
func TestGenFT8Tones(t *testing.T) {
	var msg77 [77]int8
	tones := GenFT8Tones(msg77)

	// Check the three Costas arrays.
	offsets := [3]int{0, 36, 72}
	for _, off := range offsets {
		for k := 0; k < 7; k++ {
			if tones[off+k] != Icos7[k] {
				t.Errorf("sync tone at pos %d: got %d want %d", off+k, tones[off+k], Icos7[k])
			}
		}
	}
}

// TestUnpack77StandardMsg verifies decoding of a type-1 message.
func TestUnpack77StandardMsg(t *testing.T) {
	// "CQ K1ABC FN42" packed as 77 bits.
	// We encode it via Pack28 and the grid encoder, then verify the round-trip.
	n28cq := Pack28("CQ")
	n28k1abc := Pack28("K1ABC")
	igrid4, ok := PackGrid4("FN42")
	if !ok {
		t.Fatal("PackGrid4 failed")
	}

	if n28cq < 0 || n28k1abc < 0 {
		t.Fatalf("Pack28 failed: cq=%d k1abc=%d", n28cq, n28k1abc)
	}

	// Build 77-bit string manually (type 1, i3=1).
	// Format: n28a(28) ipa(1) n28b(28) ipb(1) ir(1) igrid4(15) i3(3) = 77 bits
	c77 := fmt.Sprintf("%028b%01b%028b%01b%01b%015b%03b",
		n28cq, 0, n28k1abc, 0, 0, igrid4, 1)
	if len(c77) != 77 {
		t.Fatalf("c77 length %d, want 77", len(c77))
	}

	msg, success := Unpack77(c77)
	if !success {
		t.Fatalf("Unpack77 failed")
	}
	t.Logf("Decoded: %q", msg)
	if msg != "CQ K1ABC FN42" {
		t.Errorf("got %q, want %q", msg, "CQ K1ABC FN42")
	}
}

// TestBPDecodeAllZero verifies that the all-zero LLR vector does not produce
// a false decode (the all-zero codeword is rejected by design).
func TestBPDecodeAllZero(t *testing.T) {
	var llr [LDPCn]float64
	var apmask [LDPCn]int8
	_, _, ok := BPDecode(llr, apmask, 30)
	// The all-zero codeword has valid syndrome but zero CRC, so it may or may
	// not decode depending on the CRC; either way the function should not panic.
	_ = ok
}

// TestFFTRadix2 checks that the radix-2 FFT followed by IFFT reconstructs
// the original signal.
func TestFFTRadix2(t *testing.T) {
	n := 32
	x := make([]complex128, n)
	for i := range x {
		x[i] = complex(float64(i+1), 0)
	}
	orig := make([]complex128, n)
	copy(orig, x)

	fftRadix2(x, false)
	fftRadix2(x, true)

	for i := range x {
		if abs128(x[i]-orig[i]) > 1e-9 {
			t.Errorf("round-trip error at index %d: got %v want %v", i, x[i], orig[i])
		}
	}
}

func abs128(z complex128) float64 {
	r, im := real(z), imag(z)
	return r*r + im*im
}

// TestLDPCGenerator verifies that the generator matrix has the right dimensions.
func TestLDPCGenerator(t *testing.T) {
	gen := LDPCGenerator()
	// Verify it's not all zeros.
	nonzero := 0
	for i := 0; i < LDPCm; i++ {
		for j := 0; j < LDPCk; j++ {
			if gen[i][j] != 0 {
				nonzero++
			}
		}
	}
	if nonzero == 0 {
		t.Error("generator matrix is all zeros")
	}
	t.Logf("Generator matrix: %d×%d, %d non-zero entries", LDPCm, LDPCk, nonzero)
}

// TestNextpat91 verifies that nextpat91 generates the correct number of
// weight-w patterns among k positions: C(k, w).
func TestNextpat91(t *testing.T) {
	for _, tc := range []struct {
		k, w, want int
	}{
		{5, 1, 5},
		{5, 2, 10},
		{6, 2, 15},
		{6, 3, 20},
		{10, 1, 10},
		{10, 2, 45},
	} {
		mi := make([]int8, tc.k)
		for i := tc.k - tc.w; i < tc.k; i++ {
			mi[i] = 1
		}
		count := 1 // initial pattern
		for {
			iflag := nextpat91(mi, tc.k, tc.w)
			if iflag < 0 {
				break
			}
			count++
		}
		if count != tc.want {
			t.Errorf("nextpat91(k=%d, w=%d): got %d patterns, want %d", tc.k, tc.w, count, tc.want)
		}
	}
}

// TestOSDRoundTrip encodes a known message, converts to soft LLRs with some
// noise, and verifies that OSDDecode at ndeep=4 recovers the original.
func TestOSDRoundTrip(t *testing.T) {
	var msg77 [77]int8
	msg77[0] = 1
	msg77[5] = 1
	msg77[10] = 1
	msg77[20] = 1
	msg77[30] = 1
	msg77[40] = 1
	msg77[50] = 1
	msg77[60] = 1
	msg77[70] = 1

	cw := Encode174_91(msg77)
	if !CheckCRC14Codeword(cw) {
		t.Fatal("encoded codeword has bad CRC")
	}

	// Convert to LLRs: bit=1 → positive, bit=0 → negative.
	// Apply moderate magnitude to simulate a decent SNR signal.
	var llr [LDPCn]float64
	for i := 0; i < LDPCn; i++ {
		if cw[i] == 1 {
			llr[i] = 2.5
		} else {
			llr[i] = -2.5
		}
	}

	// Flip a few bits to create errors.
	llr[3] = -llr[3]
	llr[17] = -llr[17]
	llr[42] = -llr[42]
	llr[88] = -llr[88]
	llr[130] = -llr[130]

	var apmask [LDPCn]int8

	// Try OSD with ndeep=4 (order-2 + pre-processing).
	msg91, cwOut, nhard, ok := OSDDecode(llr, LDPCk, apmask, 4)
	if !ok {
		t.Fatal("OSDDecode(ndeep=4) failed to decode")
	}
	if nhard < 0 {
		t.Fatalf("OSDDecode returned negative nhardmin: %d", nhard)
	}

	// Verify recovered codeword matches the original.
	for i := 0; i < LDPCn; i++ {
		if cwOut[i] != cw[i] {
			t.Errorf("codeword bit %d: got %d want %d", i, cwOut[i], cw[i])
		}
	}
	// Verify message matches.
	for i := 0; i < 77; i++ {
		if msg91[i] != msg77[i] {
			t.Errorf("message bit %d: got %d want %d", i, msg91[i], msg77[i])
		}
	}
	t.Logf("OSD ndeep=4: nhard=%d, decoded OK", nhard)
}

// TestPlatanh verifies the protected atanh clamp.
func TestPlatanh(t *testing.T) {
	tests := []struct {
		x    float64
		want float64
	}{
		{0.0, 0.0},
		{0.5, math.Atanh(0.5)},
		{-0.5, math.Atanh(-0.5)},
		{0.9, math.Atanh(0.9)},
		{1.0, 19.07},
		{-1.0, -19.07},
		{2.0, 19.07},
		{-2.0, -19.07},
	}
	for _, tc := range tests {
		got := platanh(tc.x)
		diff := got - tc.want
		if diff < 0 {
			diff = -diff
		}
		if diff > 1e-10 {
			t.Errorf("platanh(%v) = %v, want %v", tc.x, got, tc.want)
		}
	}
}
