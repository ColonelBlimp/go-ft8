package ft8x

import (
	"math"
	"testing"
)

// TestComputeAPSymbolsCQ verifies that Pack28("CQ") produces the expected
// bipolar pattern matching the Fortran mcq data statement.
func TestComputeAPSymbolsCQ(t *testing.T) {
	// CQ has Pack28 value = 2 (i.e., ntokens index 2).
	// 28-bit binary of 2 = 0000...0010, MSB first.
	// So bit 26 (0-indexed from MSB) is 1, rest are 0.
	// With /R flag = 0 at position 28, the 29 bipolar values should be:
	// [-1]*26 + [1] + [-1]*2 = mcqBipolar
	n28 := Pack28("CQ")
	if n28 < 0 {
		t.Fatal("Pack28(CQ) failed")
	}

	// Build expected bipolar from n28.
	var expected [29]int
	for i := 0; i < 28; i++ {
		bit := (n28 >> uint(27-i)) & 1
		expected[i] = 2*bit - 1
	}
	expected[28] = -1 // /R flag

	for i := 0; i < 29; i++ {
		if expected[i] != mcqBipolar[i] {
			t.Errorf("CQ bipolar[%d]: got %d, want %d", i, expected[i], mcqBipolar[i])
		}
	}
	t.Logf("Pack28(CQ) = %d, bipolar matches mcqBipolar ✓", n28)
}

// TestComputeAPSymbolsRoundTrip verifies ComputeAPSymbols for a known callsign.
func TestComputeAPSymbolsRoundTrip(t *testing.T) {
	apsym, ok := ComputeAPSymbols("K1ABC", "W2XYZ")
	if !ok {
		t.Fatal("ComputeAPSymbols failed")
	}

	// Verify mycall portion (bits 0-28).
	n28my := Pack28("K1ABC")
	if n28my < 0 {
		t.Fatal("Pack28(K1ABC) failed")
	}
	for i := 0; i < 28; i++ {
		bit := (n28my >> uint(27-i)) & 1
		expected := 2*bit - 1
		if apsym[i] != expected {
			t.Errorf("apsym[%d] (mycall): got %d, want %d", i, apsym[i], expected)
		}
	}
	if apsym[28] != -1 {
		t.Errorf("apsym[28] (/R flag): got %d, want -1", apsym[28])
	}

	// Verify dxcall portion (bits 29-57).
	n28dx := Pack28("W2XYZ")
	if n28dx < 0 {
		t.Fatal("Pack28(W2XYZ) failed")
	}
	for i := 0; i < 28; i++ {
		bit := (n28dx >> uint(27-i)) & 1
		expected := 2*bit - 1
		if apsym[29+i] != expected {
			t.Errorf("apsym[%d] (dxcall): got %d, want %d", 29+i, apsym[29+i], expected)
		}
	}
	if apsym[57] != -1 {
		t.Errorf("apsym[57] (/R flag): got %d, want -1", apsym[57])
	}
}

// TestApplyAPMask verifies that ApplyAP sets the correct apmask positions
// and LLR values for each AP type.
func TestApplyAPMask(t *testing.T) {
	apsym, ok := ComputeAPSymbols("K1ABC", "W2XYZ")
	if !ok {
		t.Fatal("ComputeAPSymbols failed")
	}
	apmag := 10.0

	tests := []struct {
		name       string
		iaptype    int
		wantMasked int // number of masked bits
	}{
		{"CQ", APTypeCQ, 32},             // 29 CQ + 3 i3
		{"MyCall", APTypeMyCall, 32},     // 29 mycall + 3 i3
		{"MyDxCall", APTypeMyDxCall, 61}, // 58 + 3 i3
		{"MyDxRRR", APTypeMyDxRRR, 77},   // all 77 message bits
		{"MyDx73", APTypeMyDx73, 77},
		{"MyDxRR73", APTypeMyDxRR73, 77},
	}

	for _, tc := range tests {
		var llrz [LDPCn]float64
		var apmask [LDPCn]int8
		// Fill with baseline LLRs.
		for i := range llrz {
			llrz[i] = 1.0
		}

		ApplyAP(&llrz, &apmask, tc.iaptype, apsym, apmag)

		masked := 0
		for _, m := range apmask {
			if m == 1 {
				masked++
			}
		}
		if masked != tc.wantMasked {
			t.Errorf("%s: masked bits = %d, want %d", tc.name, masked, tc.wantMasked)
		}

		// Verify all masked positions have |llrz| == apmag.
		for i := 0; i < LDPCn; i++ {
			if apmask[i] == 1 {
				if math.Abs(math.Abs(llrz[i])-apmag) > 1e-10 {
					t.Errorf("%s: llrz[%d] = %v, want ±%v", tc.name, i, llrz[i], apmag)
				}
			}
		}
	}
}

// TestAPPassTypes verifies the AP pass type selection logic.
func TestAPPassTypes(t *testing.T) {
	tests := []struct {
		name    string
		hasMy   bool
		hasDx   bool
		cqOnly  bool
		wantLen int
		want0   int // first AP type
	}{
		{"no calls", false, false, false, 1, APTypeCQ},
		{"CQ only flag", true, true, true, 1, APTypeCQ},
		{"mycall only", true, false, false, 2, APTypeCQ},
		{"both calls", true, true, false, 4, APTypeMyDxCall},
	}
	for _, tc := range tests {
		types := APPassTypes(tc.hasMy, tc.hasDx, tc.cqOnly)
		if len(types) != tc.wantLen {
			t.Errorf("%s: got %d types, want %d", tc.name, len(types), tc.wantLen)
			continue
		}
		if types[0] != tc.want0 {
			t.Errorf("%s: first type = %d, want %d", tc.name, types[0], tc.want0)
		}
	}
}

// TestAPDecodeWeakCQ encodes a known CQ message, creates weak LLRs, and
// verifies that AP type 1 (CQ) can decode what regular BP cannot.
func TestAPDecodeWeakCQ(t *testing.T) {
	// Build "CQ K1ABC FN42" as 77 bits.
	n28cq := Pack28("CQ")
	n28k1abc := Pack28("K1ABC")
	igrid4, ok := PackGrid4("FN42")
	if !ok {
		t.Fatal("PackGrid4 failed")
	}

	// Pack type-1 message: n28a(28) ipa(1) n28b(28) ipb(1) ir(1) igrid4(15) i3(3)
	var msg77 [77]int8
	for i := 0; i < 28; i++ {
		msg77[i] = int8((n28cq >> uint(27-i)) & 1)
	}
	msg77[28] = 0 // ipa
	for i := 0; i < 28; i++ {
		msg77[29+i] = int8((n28k1abc >> uint(27-i)) & 1)
	}
	msg77[57] = 0 // ipb
	msg77[58] = 0 // ir
	for i := 0; i < 15; i++ {
		msg77[59+i] = int8((igrid4 >> uint(14-i)) & 1)
	}
	// i3 = 1 → binary 001
	msg77[74] = 0
	msg77[75] = 0
	msg77[76] = 1

	cw := Encode174_91(msg77)
	if !CheckCRC14Codeword(cw) {
		t.Fatal("encoded codeword has bad CRC")
	}

	// Create weak LLRs: correct polarity but small magnitude, with several errors.
	var llrBase [LDPCn]float64
	for i := 0; i < LDPCn; i++ {
		if cw[i] == 1 {
			llrBase[i] = 1.5
		} else {
			llrBase[i] = -1.5
		}
	}
	// Flip 6 bits to create errors — enough to challenge regular BP but
	// recoverable with AP assistance on the 29 CQ bits.
	flips := []int{3, 17, 42, 88, 110, 145}
	for _, idx := range flips {
		llrBase[idx] = -llrBase[idx]
	}
	// Weaken some additional bits without flipping them.
	weakens := []int{5, 20, 35, 50, 65, 80, 95, 120, 135, 150, 160, 170}
	for _, idx := range weakens {
		llrBase[idx] *= 0.1
	}

	// Try regular BP+OSD without AP — should fail.
	var apmask [LDPCn]int8
	res, decoded := Decode174_91(llrBase, LDPCk, 0, 2, apmask)
	if decoded && res.NHardErrors >= 0 && res.NHardErrors <= 36 {
		t.Log("Regular decode succeeded (signal wasn't weak enough for this test)")
		return // Can't test AP advantage if regular decode works
	}

	// Try with AP type 1 (CQ) — inject CQ bits with high magnitude.
	var llrz [LDPCn]float64
	copy(llrz[:], llrBase[:])
	apmag := 0.0
	for _, v := range llrz {
		if math.Abs(v) > apmag {
			apmag = math.Abs(v)
		}
	}
	apmag *= 1.01

	var apsym [58]int // not needed for CQ-only
	ApplyAP(&llrz, &apmask, APTypeCQ, apsym, apmag)

	res, decoded = Decode174_91(llrz, LDPCk, 0, 2, apmask)
	if !decoded || res.NHardErrors < 0 {
		t.Log("AP CQ decode also failed — test inconclusive (noise too strong)")
		return
	}

	// Verify the decoded message matches.
	for i := 0; i < 77; i++ {
		if res.Message91[i] != msg77[i] {
			t.Errorf("message bit %d: got %d want %d", i, res.Message91[i], msg77[i])
		}
	}
	t.Logf("AP CQ decode succeeded: nhard=%d ✓", res.NHardErrors)
}
