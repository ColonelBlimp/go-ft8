// osd_binary_test.go — Feed exact Fortran float32 bmet values (from binary dump)
// into Go's LDPC decoder to isolate whether the OSD algorithm or the LLR values
// are the cause of the decode gap.

package research

import (
	"encoding/binary"
	"math"
	"os"
	"strings"
	"testing"
)

func TestOSDBinaryFortranLLR(t *testing.T) {
	const bmetPath = "../bmet_cand9.bin"
	if _, err := os.Stat(bmetPath); os.IsNotExist(err) {
		t.Skipf("Fortran binary bmet not found: %s (run dump_llr_bin first)", bmetPath)
	}

	data, err := os.ReadFile(bmetPath)
	if err != nil {
		t.Fatalf("read bmet: %v", err)
	}
	// 4 arrays × 174 × 4 bytes = 2784 bytes
	if len(data) != 4*174*4 {
		t.Fatalf("unexpected bmet size: %d (expected %d)", len(data), 4*174*4)
	}

	var bmet [4][174]float32
	for p := 0; p < 4; p++ {
		for i := 0; i < 174; i++ {
			offset := (p*174 + i) * 4
			bits := binary.LittleEndian.Uint32(data[offset : offset+4])
			bmet[p][i] = math.Float32frombits(bits)
		}
	}

	names := []string{"bmeta", "bmetb", "bmetc", "bmetd"}
	t.Logf("Fortran binary bmet values loaded (bit-exact float32)")
	for p := 0; p < 4; p++ {
		t.Logf("  %s[0:5] = [%.8f, %.8f, %.8f, %.8f, %.8f]",
			names[p], bmet[p][0], bmet[p][1], bmet[p][2], bmet[p][3], bmet[p][4])
	}

	// Feed exact Fortran LLRs into Go decoder
	t.Logf("")
	t.Logf("Decode with bit-exact Fortran LLRs (maxosd=0, norder=2):")
	for p := 0; p < 4; p++ {
		var llrz [LDPCn]float64
		var apmask [LDPCn]int8
		for i := 0; i < 174; i++ {
			llrz[i] = ScaleFac * float64(bmet[p][i])
		}
		res, ok := Decode174_91(llrz, LDPCk, 0, 2, apmask)
		if ok {
			var msg77 [77]int8
			copy(msg77[:], res.Message91[:77])
			c77 := BitsToC77(msg77)
			msg, _ := Unpack77(c77)
			t.Logf("  %s: DECODE OK  nhard=%d  msg=%q", names[p], res.NHardErrors, strings.TrimSpace(msg))
		} else {
			t.Logf("  %s: DECODE FAILED (nhard=%d)", names[p], res.NHardErrors)
		}
	}

	// Also try with float32 LLR preservation (no float64 promotion of ScaleFac*bmet)
	t.Logf("")
	t.Logf("Decode with ScaleFac applied in float32:")
	sf32 := float32(ScaleFac)
	for p := 0; p < 4; p++ {
		var llrz [LDPCn]float64
		var apmask [LDPCn]int8
		for i := 0; i < 174; i++ {
			llrz[i] = float64(sf32 * bmet[p][i])
		}
		res, ok := Decode174_91(llrz, LDPCk, 0, 2, apmask)
		if ok {
			var msg77 [77]int8
			copy(msg77[:], res.Message91[:77])
			c77 := BitsToC77(msg77)
			msg, _ := Unpack77(c77)
			t.Logf("  %s: DECODE OK  nhard=%d  msg=%q", names[p], res.NHardErrors, strings.TrimSpace(msg))
		} else {
			t.Logf("  %s: DECODE FAILED (nhard=%d)", names[p], res.NHardErrors)
		}
	}
}
