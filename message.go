package ft8x

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

// ────────────────────────────────────────────────────────────────────────────
// Constants for callsign encoding
// ────────────────────────────────────────────────────────────────────────────

const (
	ntokens = 2063592 // DE=0, QRZ=1, CQ=2, CQ_NNN=3..1002, CQ_LLLL=1003..2063591
	max22   = 4194304 // 2^22
)

// charset for pack28/unpack28
const (
	a1 = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ" // 37 chars
	a2 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"  // 36 chars
	a3 = "0123456789"                            // 10 chars
	a4 = " ABCDEFGHIJKLMNOPQRSTUVWXYZ"           // 27 chars
)

// hashCharset is the 38-character set used in ihashcall.
const hashCharset = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ/"

// ────────────────────────────────────────────────────────────────────────────
// Call-sign hashing (ihashcall)
// ────────────────────────────────────────────────────────────────────────────

// HashCall computes the m-bit hash of a callsign string c (max 11 chars).
// Equivalent to function ihashcall(c0,m) in packjt77.f90.
func HashCall(c string, m int) int {
	c = fmt.Sprintf("%-11s", c)
	if len(c) > 11 {
		c = c[:11]
	}
	var n8 int64
	for i := 0; i < 11; i++ {
		j := strings.IndexByte(hashCharset, c[i])
		if j < 0 {
			j = 0
		}
		n8 = 38*n8 + int64(j)
	}
	// ishft(47055833459 * n8, m-64)
	result := int64(47055833459) * n8
	shift := m - 64
	if shift >= 0 {
		return int(result << uint(shift))
	}
	return int(result >> uint(-shift))
}

// ────────────────────────────────────────────────────────────────────────────
// pack28 / unpack28: encode/decode a 28-bit callsign field
// ────────────────────────────────────────────────────────────────────────────

// callBase is the offset for standard callsigns: NTOKENS + MAX22.
const callBase = ntokens + max22 // 6257896

// Pack28 packs a callsign or special token into a 28-bit integer.
// Returns -1 if the callsign is not valid or cannot be represented.
// Port of subroutine pack28 from packjt77.f90.
func Pack28(c13 string) int {
	c13 = strings.ToUpper(strings.TrimRight(c13, " "))

	// Work-around: Swaziland 3DA0 prefix.
	if len(c13) >= 4 && c13[:4] == "3DA0" {
		c13 = "3D0" + c13[4:]
	}
	// Work-around: Guinea 3X prefix.
	if len(c13) >= 3 && c13[:2] == "3X" && c13[2] >= 'A' && c13[2] <= 'Z' {
		c13 = "Q" + c13[2:]
	}

	// Special tokens.
	switch {
	case c13 == "DE":
		return 0
	case c13 == "QRZ":
		return 1
	case c13 == "CQ":
		return 2
	}

	// CQ_NNN (frequency) or CQ_LLLL (area designator).
	if len(c13) > 3 && c13[:3] == "CQ_" {
		suffix := c13[3:]
		// Numeric: CQ_160 etc.
		nnum, nlet := 0, 0
		for _, ch := range suffix {
			if ch >= '0' && ch <= '9' {
				nnum++
			} else if ch >= 'A' && ch <= 'Z' {
				nlet++
			}
		}
		if nnum == 3 && nlet == 0 {
			var nqsy int
			fmt.Sscanf(suffix, "%d", &nqsy)
			return 3 + nqsy
		}
		if nlet >= 1 && nlet <= 4 && nnum == 0 {
			s := fmt.Sprintf("%4s", suffix)
			m := 0
			for _, ch := range s {
				j := 0
				if ch >= 'A' && ch <= 'Z' {
					j = int(ch-'A') + 1
				}
				m = 27*m + j
			}
			return 3 + 1000 + m
		}
	}

	// Hash-encoded call <callsign>.
	if len(c13) > 0 && c13[0] == '<' {
		inner := strings.TrimRight(c13[1:], ">")
		inner = strings.TrimRight(inner, " ")
		n22 := HashCall(inner, 22)
		if n22 < 0 {
			n22 = -n22
		}
		n22 &= (max22 - 1)
		return ntokens + n22
	}

	// Standard callsign: right-justify to 6 characters (Fortran ADJUSTR),
	// then encode as [a1][a2][a3][a4][a4][a4].
	return packCallsign(c13)
}

// packCallsign encodes a standard callsign string into a 28-bit integer.
// Returns -1 if the callsign cannot be encoded.
// The callsign is right-justified to 6 characters (Fortran ADJUSTR) so that
// the digit falls at position 2 (0-indexed).
func packCallsign(callsign string) int {
	callsign = strings.ToUpper(strings.TrimRight(callsign, " "))

	// Right-justify to exactly 6 characters (ADJUSTR equivalent).
	s := fmt.Sprintf("%6s", callsign)
	if len(s) > 6 {
		s = s[:6]
	}

	i1 := strings.IndexByte(a1, s[0])
	i2 := strings.IndexByte(a2, s[1])
	i3 := strings.IndexByte(a3, s[2])
	i4 := strings.IndexByte(a4, s[3])
	i5 := strings.IndexByte(a4, s[4])
	i6 := strings.IndexByte(a4, s[5])

	if i1 < 0 || i2 < 0 || i3 < 0 || i4 < 0 || i5 < 0 || i6 < 0 {
		return -1
	}

	// Fortran formula: n28 = NTOKENS+MAX22 + 36*10*27*27*27*i1 + 10*27*27*27*i2 +
	//                        27*27*27*i3 + 27*27*i4 + 27*i5 + i6
	n := i1*7085880 + i2*196830 + i3*19683 + i4*729 + i5*27 + i6
	return callBase + n
}

// Unpack28 decodes a 28-bit integer into a callsign string.
// Port of subroutine unpack28 from packjt77.f90.
func Unpack28(n28 int) (string, bool) {
	if n28 == 0 {
		return "DE", true
	}
	if n28 == 1 {
		return "QRZ", true
	}
	if n28 == 2 {
		return "CQ", true
	}
	if n28 >= 3 && n28 <= 1002 {
		return fmt.Sprintf("CQ_%03d", n28-3), true
	}
	if n28 >= 1003 && n28 < ntokens {
		m := n28 - 1003
		c4 := [4]byte{}
		for i := 3; i >= 0; i-- {
			j := m % 27
			m /= 27
			if j == 0 {
				c4[i] = ' '
			} else {
				c4[i] = 'A' + byte(j-1)
			}
		}
		return "CQ_" + strings.TrimLeft(string(c4[:]), " "), true
	}
	if n28 >= ntokens && n28 < callBase {
		// Hash-encoded non-standard call (22-bit hash).
		return "<...>", true
	}

	// Standard callsign: n28 = callBase + i1*7085880 + i2*196830 + i3*19683 + i4*729 + i5*27 + i6
	if n28 < callBase {
		return "", false
	}
	n := n28 - callBase

	i6 := n % 27
	n /= 27
	i5 := n % 27
	n /= 27
	i4 := n % 27
	n /= 27
	i3 := n % 10
	n /= 10
	i2 := n % 36
	n /= 36
	i1 := n % 37

	if i1 >= len(a1) || i2 >= len(a2) || i3 >= len(a3) || i4 >= len(a4) || i5 >= len(a4) || i6 >= len(a4) {
		return "", false
	}

	call := []byte{a1[i1], a2[i2], a3[i3], a4[i4], a4[i5], a4[i6]}
	// ADJUSTL equivalent: strip leading spaces (from right-justified encoding),
	// then trim any trailing spaces.
	return strings.TrimSpace(string(call)), true
}

// ────────────────────────────────────────────────────────────────────────────
// Grid locator encoding/decoding
// ────────────────────────────────────────────────────────────────────────────

const maxGrid4 = 32400 // 180 * 180

// PackGrid4 packs a 4-character Maidenhead grid into a 15-bit integer.
// Valid range: AA00 to RR99 (18×18×10×10=32400 grids).
func PackGrid4(grid string) (int, bool) {
	if len(grid) < 4 {
		return 0, false
	}
	g := strings.ToUpper(grid[:4])
	if g[0] < 'A' || g[0] > 'R' || g[1] < 'A' || g[1] > 'R' ||
		g[2] < '0' || g[2] > '9' || g[3] < '0' || g[3] > '9' {
		return 0, false
	}
	n := (int(g[0]-'A')*18+int(g[1]-'A'))*100 +
		int(g[2]-'0')*10 + int(g[3]-'0')
	return n, true
}

// UnpackGrid4 decodes a 15-bit integer into a 4-character grid locator.
func UnpackGrid4(n int) (string, bool) {
	if n < 0 || n >= maxGrid4 {
		return "", false
	}
	i4 := n % 10
	n /= 10
	i3 := n % 10
	n /= 10
	i2 := n % 18
	i1 := n / 18
	grid := []byte{
		byte('A' + i1),
		byte('A' + i2),
		byte('0' + i3),
		byte('0' + i4),
	}
	return string(grid), true
}

// ────────────────────────────────────────────────────────────────────────────
// Free text packing (up to 13 characters, 71 bits)
// ────────────────────────────────────────────────────────────────────────────

const freeTextCharset = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ+-./?~" // 42 chars → fits in 6 bits (2^6=64)

// PackFreeText packs up to 13 characters of free text into 71 bits.
// The result is stored in out[0:70] (bit string, 0-indexed).
func PackFreeText(text string) ([71]int8, bool) {
	var bits [71]int8
	text = strings.ToUpper(fmt.Sprintf("%-13s", text))
	if len(text) > 13 {
		text = text[:13]
	}
	var val uint64
	for i := 0; i < 13; i++ {
		j := strings.IndexByte(freeTextCharset, text[i])
		if j < 0 {
			j = 0 // map unknown to space
		}
		val = val*42 + uint64(j)
	}
	// Pack 71 bits (42^13 < 2^71).
	for i := 70; i >= 0; i-- {
		bits[i] = int8(val & 1)
		val >>= 1
	}
	return bits, true
}

// UnpackFreeText decodes 71 bits into a free-text string.
func UnpackFreeText(bits [71]int8) string {
	var val uint64
	for _, b := range bits {
		val = val*2 + uint64(b&1)
	}
	text := make([]byte, 13)
	for i := 12; i >= 0; i-- {
		j := val % 42
		val /= 42
		text[i] = freeTextCharset[j]
	}
	return strings.TrimRight(string(text), " ")
}

// ────────────────────────────────────────────────────────────────────────────
// Unpack77: decode a 77-bit message string into human-readable text
// ────────────────────────────────────────────────────────────────────────────

// ARRL section abbreviations (1-indexed, 86 entries).
var arrlSec = [...]string{
	"", // 1-indexed
	"AB", "AK", "AL", "AR", "AZ", "BC", "CO", "CT", "DE", "EB",
	"EMA", "ENY", "EPA", "EWA", "GA", "GH", "IA", "ID", "IL", "IN",
	"KS", "KY", "LA", "LAX", "NS", "MB", "MDC", "ME", "MI", "MN",
	"MO", "MS", "MT", "NC", "ND", "NE", "NFL", "NH", "NL", "NLI",
	"NM", "NNJ", "NNY", "TER", "NTX", "NV", "OH", "OK", "ONE", "ONN",
	"ONS", "OR", "ORG", "PAC", "PR", "QC", "RI", "SB", "SC", "SCV",
	"SD", "SDG", "SF", "SFL", "SJV", "SK", "SNJ", "STX", "SV", "TN",
	"UT", "VA", "VI", "VT", "WCF", "WI", "WMA", "WNY", "WPA", "WTX",
	"WV", "WWA", "WY", "DX", "PE", "NB",
}

// Unpack77 decodes the 77-bit message c77 (string of '0'/'1' characters)
// into a human-readable message string.
//
// Returns (message, success).
// Port of subroutine unpack77 from wsjt-wsjtx/lib/77bit/packjt77.f90.
func Unpack77(c77 string) (string, bool) {
	if len(c77) != 77 {
		return "bad c77 length", false
	}
	for _, ch := range c77 {
		if ch != '0' && ch != '1' {
			return "failed unpack", false
		}
	}

	// Read i3 (bits 74..76) and n3 (bits 71..73).
	n3 := readBits(c77, 71, 3)
	i3 := readBits(c77, 74, 3)

	switch {
	case i3 == 0 && n3 == 0:
		// 0.0 Free text
		return unpack77_00(c77)

	case i3 == 0 && n3 == 1:
		// 0.1 DXpedition mode: call1 RR73; call2 <hashed> -11
		return unpack77_01(c77)

	case i3 == 0 && n3 == 2:
		return "", false // not implemented / reserved

	case i3 == 0 && (n3 == 3 || n3 == 4):
		// 0.3 / 0.4 ARRL Field Day
		return unpack77_03(c77, n3)

	case i3 == 0 && n3 == 5:
		// 0.5 Telemetry (18 hex digits)
		return unpack77_05(c77)

	case i3 == 0 && n3 == 6:
		// 0.6 WSPR-compatible messages
		return unpack77_06(c77)

	case i3 == 0 && n3 > 6:
		return "", false

	case i3 == 1 || i3 == 2:
		// Type 1 (standard) or Type 2 (EU VHF /P form)
		return unpack77_1(c77, i3)

	case i3 == 3:
		// Type 3 ARRL RTTY Roundup
		return unpack77_3(c77)

	case i3 == 4:
		// Type 4: one nonstandard + one hashed call
		return unpack77_4(c77)

	case i3 == 5:
		// Type 5 EU VHF contest
		return unpack77_5(c77)

	default:
		return "", false
	}
}

// readBits reads nbits bits starting at position pos in the bit string s.
func readBits(s string, pos, nbits int) int {
	v := 0
	for i := 0; i < nbits; i++ {
		v <<= 1
		if s[pos+i] == '1' {
			v |= 1
		}
	}
	return v
}

// unpack77_00 decodes free text (type 0.0).
func unpack77_00(c77 string) (string, bool) {
	var bits [71]int8
	for i := 0; i < 71; i++ {
		bits[i] = int8(c77[i] - '0')
	}
	msg := UnpackFreeText(bits)
	if len(strings.TrimSpace(msg)) == 0 {
		return "", false
	}
	return strings.TrimRight(msg, " "), true
}

// unpack77_01 decodes DXpedition mode (type 0.1).
// Format: call1 RR73; call2 <hashed> -11   (28+28+10+5 = 71 bits used)
func unpack77_01(c77 string) (string, bool) {
	n28a := readBits(c77, 0, 28)
	n28b := readBits(c77, 28, 28)
	n10 := readBits(c77, 56, 10)
	n5 := readBits(c77, 66, 5)

	irpt := 2*n5 - 30
	call1, ok1 := Unpack28(n28a)
	call2, ok2 := Unpack28(n28b)
	if !ok1 || !ok2 || n28a <= 2 || n28b <= 2 {
		return "", false
	}
	crpt := fmt.Sprintf("%+d", irpt)
	_ = n10 // hash lookup would go here; not implemented without call-cache
	call3 := "<...>"
	msg := fmt.Sprintf("%s RR73; %s %s %s", call1, call2, call3, crpt)
	return strings.TrimRight(msg, " "), true
}

// unpack77_03 decodes ARRL Field Day (type 0.3 / 0.4).
func unpack77_03(c77 string, n3 int) (string, bool) {
	n28a := readBits(c77, 0, 28)
	n28b := readBits(c77, 28, 28)
	ir := readBits(c77, 56, 1)
	intx := readBits(c77, 57, 4)
	nclass := readBits(c77, 61, 3)
	isec := readBits(c77, 64, 7)

	call1, ok1 := Unpack28(n28a)
	call2, ok2 := Unpack28(n28b)
	if !ok1 || !ok2 {
		return "", false
	}
	if isec < 1 || isec > len(arrlSec)-1 {
		return "", false
	}
	ntx := intx + 1
	if n3 == 4 {
		ntx += 16
	}
	classChar := string(rune('A' + nclass))
	sec := strings.TrimRight(arrlSec[isec], " ")
	var msg string
	if ir == 0 {
		msg = fmt.Sprintf("%s %s %d%s %s", call1, call2, ntx, classChar, sec)
	} else {
		msg = fmt.Sprintf("%s %s R%d%s %s", call1, call2, ntx, classChar, sec)
	}
	return msg, true
}

// unpack77_05 decodes telemetry (type 0.5).
func unpack77_05(c77 string) (string, bool) {
	// 71 bits interpreted as three 24-bit integers (23+24+24).
	n1 := readBits(c77, 0, 23)
	n2 := readBits(c77, 23, 24)
	n3 := readBits(c77, 47, 24)
	msg := fmt.Sprintf("%06X%06X%06X", n1, n2, n3)
	msg = strings.TrimLeft(msg, "0")
	if msg == "" {
		msg = "0"
	}
	return msg, true
}

// unpack77_06 decodes WSPR-compatible messages (type 0.6).
func unpack77_06(c77 string) (string, bool) {
	// Bits 47-49 determine subtype.
	j48 := int(c77[47] - '0')
	j49 := int(c77[48] - '0')
	j50 := int(c77[49] - '0')

	var itype int
	if j50 == 1 {
		itype = 2
	} else if j49 == 0 {
		itype = 1
	} else if j48 == 0 {
		itype = 3
	} else {
		return "", false
	}

	switch itype {
	case 1:
		// WSPR Type 1: call grid dBm
		n28 := readBits(c77, 0, 28)
		igrid4 := readBits(c77, 28, 15)
		idbm := readBits(c77, 43, 5)
		idbm = int(math.Round(float64(idbm) * 10.0 / 3.0))
		call, ok := Unpack28(n28)
		if !ok {
			return "", false
		}
		grid, gok := UnpackGrid4(igrid4)
		if !gok {
			return "", false
		}
		return fmt.Sprintf("%s %s %d", call, grid, idbm), true

	case 2:
		// WSPR Type 2: call/pfx or call/sfx dBm (not fully decoded here)
		return "", false

	case 3:
		// WSPR Type 3: hashed call grid6 (not fully decoded here)
		return "", false
	}
	return "", false
}

// unpack77_1 decodes standard type-1 (or type-2 /P) messages.
// Format: call1[/R] call2[/R] grid4|RRR|RR73|73|report   (28+1+28+1+1+15+3 = 77)
func unpack77_1(c77 string, i3 int) (string, bool) {
	n28a := readBits(c77, 0, 28)
	ipa := readBits(c77, 28, 1)
	n28b := readBits(c77, 29, 28)
	ipb := readBits(c77, 57, 1)
	ir := readBits(c77, 58, 1)
	igrid4 := readBits(c77, 59, 15)
	// i3 is in bits 74..76

	call1, ok1 := Unpack28(n28a)
	call2, ok2 := Unpack28(n28b)
	if !ok1 || !ok2 {
		return "", false
	}

	// Append /R or /P suffix.
	if ipa == 1 && len(call1) >= 3 {
		if i3 == 1 {
			call1 = call1 + "/R"
		} else {
			call1 = call1 + "/P"
		}
	}
	if ipb == 1 && len(call2) >= 3 {
		if i3 == 1 {
			call2 = call2 + "/R"
		} else {
			call2 = call2 + "/P"
		}
	}

	// Replace CQ_ with CQ .
	call1 = strings.Replace(call1, "CQ_", "CQ ", 1)

	if igrid4 <= maxGrid4 {
		grid, gok := UnpackGrid4(igrid4)
		if !gok {
			return "", false
		}
		if ir == 0 {
			if call1 == "CQ" && ir == 1 {
				return "", false
			}
			return fmt.Sprintf("%s %s %s", call1, call2, grid), true
		}
		return fmt.Sprintf("%s %s R %s", call1, call2, grid), true
	}

	// Not a grid: special report codes.
	irpt := igrid4 - maxGrid4
	switch irpt {
	case 1:
		return fmt.Sprintf("%s %s", call1, call2), true
	case 2:
		return fmt.Sprintf("%s %s RRR", call1, call2), true
	case 3:
		return fmt.Sprintf("%s %s RR73", call1, call2), true
	case 4:
		return fmt.Sprintf("%s %s 73", call1, call2), true
	default:
		if irpt >= 5 {
			isnr := irpt - 35
			if isnr > 50 {
				isnr -= 101
			}
			crpt := fmt.Sprintf("%+d", isnr)
			if ir == 0 {
				return fmt.Sprintf("%s %s %s", call1, call2, crpt), true
			}
			return fmt.Sprintf("%s %s R%s", call1, call2, crpt), true
		}
	}
	return "", false
}

// unpack77_3 decodes ARRL RTTY Roundup (type 3).
func unpack77_3(c77 string) (string, bool) {
	itu := readBits(c77, 0, 1)
	n28a := readBits(c77, 1, 28)
	n28b := readBits(c77, 29, 28)
	ir := readBits(c77, 57, 1)
	irpt := readBits(c77, 58, 3)
	nexch := readBits(c77, 61, 13)
	// i3 in bits 74..76

	call1, ok1 := Unpack28(n28a)
	call2, ok2 := Unpack28(n28b)
	if !ok1 || !ok2 {
		return "", false
	}

	crpt := fmt.Sprintf("5%d9", irpt+2)
	prefix := ""
	if itu == 1 {
		prefix = "TU; "
	}
	rStr := ""
	if ir == 1 {
		rStr = "R "
	}

	if nexch > 8000 {
		imult := nexch - 8000
		if imult < 1 || imult > 171 {
			return "", false
		}
		mult := rttyMult[imult-1]
		return fmt.Sprintf("%s%s %s %s%s %s", prefix, call1, call2, rStr, crpt, mult), true
	}
	nserial := nexch
	if nserial < 1 {
		return "", false
	}
	return fmt.Sprintf("%s%s %s %s%s %04d", prefix, call1, call2, rStr, crpt, nserial), true
}

// unpack77_4 decodes type-4 (one nonstandard call + one hashed call).
func unpack77_4(c77 string) (string, bool) {
	n12 := readBits(c77, 0, 12)
	n58 := readBits64(c77, 12, 58)
	iflip := readBits(c77, 70, 1)
	nrpt := readBits(c77, 71, 2)
	icq := readBits(c77, 73, 1)

	// Decode the 58-bit nonstandard callsign (up to 11 chars from 38-char set).
	charset38 := " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ/"
	c11 := make([]byte, 11)
	n := n58
	for i := 10; i >= 0; i-- {
		j := n % 38
		n /= 38
		c11[i] = charset38[j]
	}
	nonstdCall := strings.TrimLeft(string(c11), " ")

	_ = n12 // hash lookup not implemented without call cache
	hashedCall := "<...>"

	var call1, call2 string
	if iflip == 0 {
		call1 = hashedCall
		call2 = nonstdCall
	} else {
		call1 = nonstdCall
		call2 = hashedCall
	}

	if icq == 1 {
		return "CQ " + call2, true
	}
	switch nrpt {
	case 0:
		return fmt.Sprintf("%s %s", call1, call2), true
	case 1:
		return fmt.Sprintf("%s %s RRR", call1, call2), true
	case 2:
		return fmt.Sprintf("%s %s RR73", call1, call2), true
	case 3:
		return fmt.Sprintf("%s %s 73", call1, call2), true
	}
	return "", false
}

// unpack77_5 decodes EU VHF contest (type 5).
func unpack77_5(c77 string) (string, bool) {
	n12 := readBits(c77, 0, 12)
	n22 := readBits(c77, 12, 22)
	ir := readBits(c77, 34, 1)
	irpt := readBits(c77, 35, 3)
	iserial := readBits(c77, 38, 11)
	_ = readBits(c77, 49, 25) // igrid6

	_ = n12
	_ = n22
	call1 := "<...>"
	call2 := "<...>"

	nrs := 52 + irpt
	rStr := ""
	if ir == 1 {
		rStr = "R "
	}
	return fmt.Sprintf("%s %s %s%02d%04d", call1, call2, rStr, nrs, iserial), true
}

// readBits64 reads nbits bits from position pos in the bit string s,
// returning a 64-bit integer.
func readBits64(s string, pos, nbits int) int {
	v := 0
	for i := 0; i < nbits; i++ {
		v <<= 1
		if s[pos+i] == '1' {
			v |= 1
		}
	}
	return v
}

// ────────────────────────────────────────────────────────────────────────────
// Message bits → c77 string helpers
// ────────────────────────────────────────────────────────────────────────────

// BitsToC77 converts a 77-element int8 array (values 0/1) to a 77-char string.
func BitsToC77(bits [77]int8) string {
	b := make([]byte, 77)
	for i, v := range bits {
		if v != 0 {
			b[i] = '1'
		} else {
			b[i] = '0'
		}
	}
	return string(b)
}

// C77ToBits converts a 77-char bit string to a [77]int8.
func C77ToBits(c77 string) ([77]int8, bool) {
	var bits [77]int8
	if len(c77) != 77 {
		return bits, false
	}
	for i, ch := range c77 {
		switch ch {
		case '0':
			bits[i] = 0
		case '1':
			bits[i] = 1
		default:
			return bits, false
		}
	}
	return bits, true
}

// IsUpperAlnum checks if all runes in s are upper-case ASCII letters or digits.
func IsUpperAlnum(s string) bool {
	for _, r := range s {
		if !unicode.IsUpper(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// rttyMult is the list of ARRL RTTY Roundup multipliers (states/provinces).
var rttyMult = [...]string{
	"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "FL", "GA",
	"HI", "ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD",
	"MA", "MI", "MN", "MS", "MO", "MT", "NE", "NV", "NH", "NJ",
	"NM", "NY", "NC", "ND", "OH", "OK", "OR", "PA", "RI", "SC",
	"SD", "TN", "TX", "UT", "VT", "VA", "WA", "WV", "WI", "WY",
	"NB", "NS", "QC", "ON", "MB", "SK", "AB", "BC", "NWT", "NF",
	"LB", "NU", "YT", "PEI", "DC", "DR", "FR", "GD", "GR", "OV",
	"ZH", "ZL",
	"X01", "X02", "X03", "X04", "X05", "X06", "X07", "X08", "X09", "X10",
	"X11", "X12", "X13", "X14", "X15", "X16", "X17", "X18", "X19", "X20",
	"X21", "X22", "X23", "X24", "X25", "X26", "X27", "X28", "X29", "X30",
	"X31", "X32", "X33", "X34", "X35", "X36", "X37", "X38", "X39", "X40",
	"X41", "X42", "X43", "X44", "X45", "X46", "X47", "X48", "X49", "X50",
	"X51", "X52", "X53", "X54", "X55", "X56", "X57", "X58", "X59", "X60",
	"X61", "X62", "X63", "X64", "X65", "X66", "X67", "X68", "X69", "X70",
	"X71", "X72", "X73", "X74", "X75", "X76", "X77", "X78", "X79", "X80",
	"X81", "X82", "X83", "X84", "X85", "X86", "X87", "X88", "X89", "X90",
	"X91", "X92", "X93", "X94", "X95", "X96", "X97", "X98", "X99",
}
