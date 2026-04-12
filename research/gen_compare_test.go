package research

import (
	"fmt"
	"os"
	"testing"
)

func TestGeneratorCompare(t *testing.T) {
	// Go generator
	gen := osdFullGenerator(LDPCk)

	// Dump row 1 and row 78 for comparison with Fortran
	var row1 string
	for c := 0; c < LDPCn; c++ {
		row1 += fmt.Sprintf("%d", gen[0][c])
	}
	t.Logf("Go row 1:  %s", row1)
	// Fortran row 1: 100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010100000101001010000100011011000110001110010000000101001011111101000011101010111000
	t.Logf("F77 row 1: 100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010100000101001010000100011011000110001110010000000101001011111101000011101010111000")

	if row1 != "100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010100000101001010000100011011000110001110010000000101001011111101000011101010111000" {
		t.Errorf("Row 1 MISMATCH")
	} else {
		t.Logf("Row 1 matches ✓")
	}

	// Write full generator to file for external comparison
	f, err := os.Create("/tmp/go_gen.txt")
	if err != nil {
		t.Fatal(err)
	}
	for r := 0; r < LDPCk; r++ {
		for c := 0; c < LDPCn; c++ {
			fmt.Fprintf(f, "%d", gen[r][c])
		}
		fmt.Fprintln(f)
	}
	f.Close()
	t.Logf("Full generator written to /tmp/go_gen.txt")

	// Also write Fortran generator using encode174_91NoCRC
	f2, err := os.Create("/tmp/go_gen_encode.txt")
	if err != nil {
		t.Fatal(err)
	}
	for r := 0; r < LDPCk; r++ {
		var msg [LDPCk]int8
		msg[r] = 1
		cw := encode174_91NoCRC(msg)
		for c := 0; c < LDPCn; c++ {
			fmt.Fprintf(f2, "%d", cw[c])
		}
		fmt.Fprintln(f2)
	}
	f2.Close()
	t.Logf("Encode-based generator written to /tmp/go_gen_encode.txt")

	// Compare them
	gen1, _ := os.ReadFile("/tmp/go_gen.txt")
	gen2, _ := os.ReadFile("/tmp/go_gen_encode.txt")
	if string(gen1) == string(gen2) {
		t.Logf("Both generators are IDENTICAL ✓")
	} else {
		t.Errorf("Generator matrices DIFFER — this is the OSD bug!")
		// Find first difference
		lines1 := splitLines(string(gen1))
		lines2 := splitLines(string(gen2))
		for i := 0; i < len(lines1) && i < len(lines2); i++ {
			if lines1[i] != lines2[i] {
				t.Logf("First diff at row %d:", i)
				t.Logf("  gen:    %s", lines1[i])
				t.Logf("  encode: %s", lines2[i])
				break
			}
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
