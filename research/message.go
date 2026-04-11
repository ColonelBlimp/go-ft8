// message.go — Message unpacking for the research package.
//
// Port of subroutine unpack77, unpack28, unpacktext77, to_grid4,
// to_grid6, to_grid from wsjt-wsjtx/lib/77bit/packjt77.f90.
//
// Ported directly from the Fortran source — no production ft8x dependency.
// Hash tables (hash10/hash12/hash22) are not maintained; hashed callsigns
// appear as "<...>" in decoded messages.

package research

import (
	"fmt"
	"strings"
)

// Character sets used by unpack28 (matching packjt77.f90 lines 763–766).
const (
	c1set  = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ/"      // 37 chars: callsign char 1
	c2set  = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"        // 36 chars: callsign char 2
	c3set  = "0123456789"                                  // 10 chars: callsign char 3
	c4set  = " ABCDEFGHIJKLMNOPQRSTUVWXYZ"                 // 27 chars: callsign chars 4–6
	a2set  = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"        // 36 chars: WSPR prefix/suffix
	c38set = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ/"      // 38 chars: non-standard call
	c42set = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ+-./?/" // free text — 42 chars
)

// Note: c42set has 43 bytes but the Fortran only indexes 1..42 from
// " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ+-./?". The trailing '/' above
// is position 37 in c38set. For free text, the correct 42-char set is:
const freeTextChars = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ+-./?"

// Key constants from packjt77.f90.
const (
	ntokens  = 2063592
	max22    = 4194304
	maxgrid4 = 32400
	nzzz     = 46656 // 36^3
)

// ARRL sections (packjt77.f90 lines 230–239).
var csec = [86]string{
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

// US/Canadian state/province multipliers (packjt77.f90 lines 240–258).
var cmult = [171]string{
	"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "FL", "GA",
	"HI", "ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD",
	"MA", "MI", "MN", "MS", "MO", "MT", "NE", "NV", "NH", "NJ",
	"NM", "NY", "NC", "ND", "OH", "OK", "OR", "PA", "RI", "SC",
	"SD", "TN", "TX", "UT", "VT", "VA", "WA", "WV", "WI", "WY",
	"NB", "NS", "QC", "ON", "MB", "SK", "AB", "BC", "NWT", "NF",
	"LB", "NU", "YT", "PEI", "DC", "DR", "FR", "GD", "GR", "OV",
	"ZH", "ZL", "X01", "X02", "X03", "X04", "X05", "X06", "X07", "X08",
	"X09", "X10", "X11", "X12", "X13", "X14", "X15", "X16", "X17", "X18",
	"X19", "X20", "X21", "X22", "X23", "X24", "X25", "X26", "X27", "X28",
	"X29", "X30", "X31", "X32", "X33", "X34", "X35", "X36", "X37", "X38",
	"X39", "X40", "X41", "X42", "X43", "X44", "X45", "X46", "X47", "X48",
	"X49", "X50", "X51", "X52", "X53", "X54", "X55", "X56", "X57", "X58",
	"X59", "X60", "X61", "X62", "X63", "X64", "X65", "X66", "X67", "X68",
	"X69", "X70", "X71", "X72", "X73", "X74", "X75", "X76", "X77", "X78",
	"X79", "X80", "X81", "X82", "X83", "X84", "X85", "X86", "X87", "X88",
	"X89", "X90", "X91", "X92", "X93", "X94", "X95", "X96", "X97", "X98",
	"X99",
}

// ────────────────────────────────────────────────────────────────────────────
// BitsToC77 converts 77 int8 bits (0/1) to a 77-character string of '0'/'1'.
// ────────────────────────────────────────────────────────────────────────────

func BitsToC77(bits [77]int8) string {
	var b [77]byte
	for i, v := range bits {
		b[i] = '0' + byte(v&1)
	}
	return string(b[:])
}

// ────────────────────────────────────────────────────────────────────────────
// readBits extracts nbits from c77 starting at 0-indexed position pos.
// ────────────────────────────────────────────────────────────────────────────

func readBits(c77 string, pos, nbits int) int64 {
	var v int64
	for i := 0; i < nbits; i++ {
		v = (v << 1) | int64(c77[pos+i]-'0')
	}
	return v
}

// ────────────────────────────────────────────────────────────────────────────
// Unpack77 decodes a 77-character binary string into a human-readable message.
//
// Port of subroutine unpack77 from wsjt-wsjtx/lib/77bit/packjt77.f90
// lines 200–619.  nrx is hardcoded to 1 (received message).
// Hash tables are not maintained — hashed callsigns appear as "<...>".
// ────────────────────────────────────────────────────────────────────────────

func Unpack77(c77 string) (string, bool) {
	if len(c77) != 77 {
		return "", false
	}
	// Validate all chars are '0' or '1' (packjt77.f90 lines 288–294).
	for i := 0; i < 77; i++ {
		if c77[i] != '0' && c77[i] != '1' {
			return "", false
		}
	}

	// Extract i3 and n3 (packjt77.f90 line 296).
	// Fortran: read(c77(72:77),'(2b3)') n3,i3
	n3 := int(readBits(c77, 71, 3))
	i3 := int(readBits(c77, 74, 3))

	var msg string
	ok := true

	switch {

	// ── i3=0, n3=0: Free text ──────────────────────────────────────────
	case i3 == 0 && n3 == 0:
		msg = unpacktext77(c77[:71])
		msg = strings.TrimRight(msg, " ")
		msg = strings.TrimLeft(msg, " ")
		if msg == "" {
			return "", false
		}

	// ── i3=0, n3=1: DXpedition mode ────────────────────────────────────
	case i3 == 0 && n3 == 1:
		n28a := int(readBits(c77, 0, 28))
		n28b := int(readBits(c77, 28, 28))
		n10 := int(readBits(c77, 56, 10))
		n5 := int(readBits(c77, 66, 5))
		irpt := 2*n5 - 30
		crpt := formatReport(irpt)
		call1, ok1 := unpack28(n28a)
		if !ok1 || n28a <= 2 {
			ok = false
		}
		call2, ok2 := unpack28(n28b)
		if !ok2 || n28b <= 2 {
			ok = false
		}
		call3 := "<...>" // hash10 lookup — not maintained
		_ = n10
		msg = call1 + " RR73; " + call2 + " " + call3 + " " + crpt

	// ── i3=0, n3=2: unused ─────────────────────────────────────────────
	case i3 == 0 && n3 == 2:
		ok = false

	// ── i3=0, n3=3/4: ARRL Field Day ───────────────────────────────────
	case i3 == 0 && (n3 == 3 || n3 == 4):
		n28a := int(readBits(c77, 0, 28))
		n28b := int(readBits(c77, 28, 28))
		ir := int(readBits(c77, 56, 1))
		intx := int(readBits(c77, 57, 4))
		nclass := int(readBits(c77, 61, 3))
		isec := int(readBits(c77, 64, 7))
		if isec < 1 || isec > 86 {
			ok = false
			isec = 1
		}
		call1, ok1 := unpack28(n28a)
		if !ok1 || n28a <= 2 {
			ok = false
		}
		call2, ok2 := unpack28(n28b)
		if !ok2 || n28b <= 2 {
			ok = false
		}
		ntx := intx + 1
		if n3 == 4 {
			ntx += 16
		}
		cntx := fmt.Sprintf("%2d", ntx) + string(rune('A'+nclass))
		sec := csec[isec-1] // 1-indexed in Fortran

		if ir == 0 && ntx < 10 {
			msg = call1 + " " + call2 + cntx + " " + sec
		} else if ir == 1 && ntx < 10 {
			msg = call1 + " " + call2 + " R" + cntx + " " + sec
		} else if ir == 0 && ntx >= 10 {
			msg = call1 + " " + call2 + " " + cntx + " " + sec
		} else {
			msg = call1 + " " + call2 + " R " + cntx + " " + sec
		}

	// ── i3=0, n3=5: Telemetry (18 hex) ─────────────────────────────────
	case i3 == 0 && n3 == 5:
		n1 := readBits(c77, 0, 23)
		n2 := readBits(c77, 23, 24)
		n3v := readBits(c77, 47, 24)
		hex := fmt.Sprintf("%06X%06X%06X", n1, n2, n3v)
		msg = strings.TrimLeft(hex, "0")
		if msg == "" {
			msg = "0"
		}

	// ── i3=0, n3=6: WSPR ───────────────────────────────────────────────
	case i3 == 0 && n3 == 6:
		j48 := int(readBits(c77, 47, 1))
		j49 := int(readBits(c77, 48, 1))
		j50 := int(readBits(c77, 49, 1))

		var itype int
		if j50 == 1 {
			itype = 2
		} else if j49 == 0 {
			itype = 1
		} else if j48 == 0 {
			itype = 3
		} else {
			ok = false
			itype = -1
		}

		switch itype {
		case 1: // WSPR Type 1
			n28 := int(readBits(c77, 0, 28))
			igrid4 := int(readBits(c77, 28, 15))
			idbm := int(readBits(c77, 43, 5))
			idbm = int(float64(idbm)*10.0/3.0 + 0.5)
			if idbm < 0 || idbm > 60 {
				ok = false
			}
			call1, ok1 := unpack28(n28)
			if !ok1 {
				ok = false
			}
			grid4, okg := toGrid4(igrid4)
			if !okg {
				ok = false
			}
			msg = call1 + " " + grid4 + " " + strings.TrimLeft(fmt.Sprintf("%3d", idbm), " ")

		case 2: // WSPR Type 2
			n28 := int(readBits(c77, 0, 28))
			npfx := int(readBits(c77, 28, 16))
			idbm := int(readBits(c77, 44, 5))
			idbm = int(float64(idbm)*10.0/3.0 + 0.5)
			if idbm < 0 || idbm > 60 {
				ok = false
			}
			call1, ok1 := unpack28(n28)
			if !ok1 {
				ok = false
			}
			crpt := strings.TrimLeft(fmt.Sprintf("%3d", idbm), " ")

			if npfx < nzzz {
				// Prefix
				cpfx := wspr2Prefix(npfx)
				msg = cpfx + "/" + call1 + " " + crpt
			} else {
				// Suffix
				cpfx, oksfx := wspr2Suffix(npfx - nzzz)
				if !oksfx {
					ok = false
					return "", false
				}
				msg = call1 + "/" + cpfx + " " + crpt
			}

		case 3: // WSPR Type 3
			n22 := int(readBits(c77, 0, 22))
			igrid6 := int(readBits(c77, 22, 25))
			n28 := n22 + ntokens
			call1, ok1 := unpack28(n28)
			if !ok1 {
				ok = false
			}
			grid6, okg := toGrid(igrid6)
			if !okg {
				ok = false
			}
			msg = call1 + " " + grid6
		}

	// ── i3=0, n3>6: undefined ──────────────────────────────────────────
	case i3 == 0 && n3 > 6:
		ok = false

	// ── i3=1 or i3=2: Standard message / EU VHF ────────────────────────
	case i3 == 1 || i3 == 2:
		n28a := int(readBits(c77, 0, 28))
		ipa := int(readBits(c77, 28, 1))
		n28b := int(readBits(c77, 29, 28))
		ipb := int(readBits(c77, 57, 1))
		ir := int(readBits(c77, 58, 1))
		igrid4 := int(readBits(c77, 59, 15))

		call1, ok1 := unpack28(n28a)
		if !ok1 {
			ok = false
		}
		call2, ok2 := unpack28(n28b)
		if !ok2 {
			ok = false
		}

		// Replace CQ_ with CQ (packjt77.f90 line 472).
		if strings.HasPrefix(call1, "CQ_") {
			call1 = "CQ " + call1[3:]
		}

		// Append /R or /P suffix (packjt77.f90 lines 473–488).
		if !strings.Contains(call1, "<") {
			i := strings.Index(call1, " ")
			if i < 0 {
				i = len(call1)
			}
			if i >= 3 && ipa == 1 && i3 == 1 {
				call1 = call1[:i] + "/R" + call1[i:]
			}
			if i >= 3 && ipa == 1 && i3 == 2 {
				call1 = call1[:i] + "/P" + call1[i:]
			}
		}
		if !strings.Contains(call2, "<") {
			i := strings.Index(call2, " ")
			if i < 0 {
				i = len(call2)
			}
			if i >= 3 && ipb == 1 && i3 == 1 {
				call2 = call2[:i] + "/R" + call2[i:]
			}
			if i >= 3 && ipb == 1 && i3 == 2 {
				call2 = call2[:i] + "/P" + call2[i:]
			}
		}

		call1 = strings.TrimSpace(call1)
		call2 = strings.TrimSpace(call2)

		if igrid4 <= maxgrid4 {
			// Grid locator (packjt77.f90 lines 489–494).
			grid4, okg := toGrid4(igrid4)
			if !okg {
				ok = false
			}
			if ir == 0 {
				msg = call1 + " " + call2 + " " + grid4
			} else {
				msg = call1 + " " + call2 + " R " + grid4
			}
			if strings.HasPrefix(msg, "CQ ") && ir == 1 {
				ok = false
			}
		} else {
			// Special report/signoff (packjt77.f90 lines 496–509).
			irpt := igrid4 - maxgrid4
			switch irpt {
			case 1:
				msg = call1 + " " + call2
			case 2:
				msg = call1 + " " + call2 + " RRR"
			case 3:
				msg = call1 + " " + call2 + " RR73"
			case 4:
				msg = call1 + " " + call2 + " 73"
			default:
				if irpt >= 5 {
					isnr := irpt - 35
					if isnr > 50 {
						isnr = isnr - 101
					}
					crpt := formatReport(isnr)
					if ir == 0 {
						msg = call1 + " " + call2 + " " + crpt
					} else {
						msg = call1 + " " + call2 + " R" + crpt
					}
				}
			}
			if strings.HasPrefix(msg, "CQ ") && irpt >= 2 {
				ok = false
			}
		}

	// ── i3=3: ARRL RTTY Contest ─────────────────────────────────────────
	case i3 == 3:
		itu := int(readBits(c77, 0, 1))
		n28a := int(readBits(c77, 1, 28))
		n28b := int(readBits(c77, 29, 28))
		ir := int(readBits(c77, 57, 1))
		irpt := int(readBits(c77, 58, 3))
		nexch := int(readBits(c77, 61, 13))

		crpt := fmt.Sprintf("5%d9", irpt+2)
		call1, ok1 := unpack28(n28a)
		if !ok1 {
			ok = false
		}
		call2, ok2 := unpack28(n28b)
		if !ok2 {
			ok = false
		}

		imult := 0
		nserial := 0
		if nexch > 8000 {
			imult = nexch - 8000
		}
		if nexch < 8000 {
			nserial = nexch
		}

		prefix := ""
		if itu == 1 {
			prefix = "TU; "
		}
		rstr := " "
		if ir == 1 {
			rstr = " R "
		}

		if imult >= 1 && imult <= 171 {
			msg = prefix + call1 + " " + call2 + rstr + crpt + " " + cmult[imult-1]
		} else if nserial >= 1 && nserial <= 7999 {
			cserial := fmt.Sprintf("%04d", nserial)
			msg = prefix + call1 + " " + call2 + rstr + crpt + " " + cserial
		}

	// ── i3=4: Non-standard callsign ─────────────────────────────────────
	case i3 == 4:
		n12 := int(readBits(c77, 0, 12))
		n58 := readBits(c77, 12, 58)
		iflip := int(readBits(c77, 70, 1))
		nrpt := int(readBits(c77, 71, 2))
		icq := int(readBits(c77, 73, 1))

		// Decode 11-character callsign from n58 (packjt77.f90 lines 557–561).
		var c11 [11]byte
		for i := 10; i >= 0; i-- {
			j := int(n58 % 38)
			c11[i] = c38set[j]
			n58 /= 38
		}
		call11 := strings.TrimSpace(string(c11[:]))

		// hash12 lookup — not maintained.
		call3 := "<...>"
		_ = n12

		var call1, call2 string
		if iflip == 0 {
			call1 = call3
			call2 = call11
		} else {
			call1 = call11
			call2 = call3
		}

		if icq == 0 {
			switch nrpt {
			case 0:
				msg = call1 + " " + call2
			case 1:
				msg = call1 + " " + call2 + " RRR"
			case 2:
				msg = call1 + " " + call2 + " RR73"
			case 3:
				msg = call1 + " " + call2 + " 73"
			}
		} else {
			msg = "CQ " + call2
		}

	// ── i3=5: EU VHF contest ────────────────────────────────────────────
	case i3 == 5:
		n12 := int(readBits(c77, 0, 12))
		n22 := int(readBits(c77, 12, 22))
		ir := int(readBits(c77, 34, 1))
		irpt := int(readBits(c77, 35, 3))
		iserial := int(readBits(c77, 38, 11))
		igrid6 := int(readBits(c77, 49, 25))

		if igrid6 < 0 || igrid6 > 18662399 {
			return "", false
		}

		call1 := "<...>" // hash12 lookup
		_ = n12
		call2 := "<...>" // hash22 lookup
		_ = n22

		nrs := 52 + irpt
		cexch := fmt.Sprintf("%2d%04d", nrs, iserial)
		grid6, okg := toGrid6(igrid6)
		if !okg {
			ok = false
		}
		if ir == 0 {
			msg = call1 + " " + call2 + " " + cexch + " " + grid6
		} else {
			msg = call1 + " " + call2 + " R " + cexch + " " + grid6
		}

	// ── i3 >= 6: undefined ──────────────────────────────────────────────
	default:
		ok = false
	}

	// Final check (packjt77.f90 line 616).
	if strings.HasPrefix(msg, "CQ <") {
		ok = false
	}

	return strings.TrimSpace(msg), ok
}

// ────────────────────────────────────────────────────────────────────────────
// unpack28 decodes a 28-bit encoded callsign field.
//
// Port of subroutine unpack28 from packjt77.f90 lines 754–826.
// ────────────────────────────────────────────────────────────────────────────

func unpack28(n28 int) (string, bool) {
	if n28 < ntokens {
		// Special tokens (packjt77.f90 lines 771–793).
		switch {
		case n28 == 0:
			return "DE", true
		case n28 == 1:
			return "QRZ", true
		case n28 == 2:
			return "CQ", true
		case n28 <= 1002:
			return fmt.Sprintf("CQ_%03d", n28-3), true
		case n28 <= 532443:
			// CQ with 4-char alpha directed call (packjt77.f90 lines 781–793).
			n := n28 - 1003
			i1 := n / (27 * 27 * 27)
			n -= 27 * 27 * 27 * i1
			i2 := n / (27 * 27)
			n -= 27 * 27 * i2
			i3 := n / 27
			i4 := n - 27*i3
			s := string([]byte{c4set[i1], c4set[i2], c4set[i3], c4set[i4]})
			s = strings.TrimLeft(s, " ")
			return "CQ_" + s, true
		default:
			return "<...>", false
		}
	}

	n := n28 - ntokens
	if n < max22 {
		// 22-bit hash — no hash table maintained.
		return "<...>", true
	}

	// Standard callsign (packjt77.f90 lines 804–818).
	n = n - max22
	i1 := n / (36 * 10 * 27 * 27 * 27)
	n -= 36 * 10 * 27 * 27 * 27 * i1
	i2 := n / (10 * 27 * 27 * 27)
	n -= 10 * 27 * 27 * 27 * i2
	i3 := n / (27 * 27 * 27)
	n -= 27 * 27 * 27 * i3
	i4 := n / (27 * 27)
	n -= 27 * 27 * i4
	i5 := n / 27
	i6 := n - 27*i5

	if i1 >= len(c1set) || i2 >= len(c2set) || i3 >= len(c3set) ||
		i4 >= len(c4set) || i5 >= len(c4set) || i6 >= len(c4set) {
		return "", false
	}

	s := string([]byte{c1set[i1], c2set[i2], c3set[i3], c4set[i4], c4set[i5], c4set[i6]})
	s = strings.TrimLeft(s, " ")

	// Validate: no embedded spaces (packjt77.f90 lines 820–824).
	trimmed := strings.TrimRight(s, " ")
	if strings.Contains(trimmed, " ") {
		return "", false
	}
	return trimmed, true
}

// ────────────────────────────────────────────────────────────────────────────
// unpacktext77 decodes a 71-bit free-text message.
//
// Port of subroutine unpacktext77 from packjt77.f90 lines 1461–1482.
// Uses multi-precision short division (mp_short_div) to extract characters.
// ────────────────────────────────────────────────────────────────────────────

func unpacktext77(c71 string) string {
	// Parse 71 bits into 9 bytes: first byte gets 7 bits, remaining 8 get 8 each.
	var b [9]byte
	for i := 0; i < 7; i++ {
		b[0] = (b[0] << 1) | (c71[i] - '0')
	}
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			b[1+i] = (b[1+i] << 1) | (c71[7+i*8+j] - '0')
		}
	}

	// Repeatedly divide by 42 to extract 13 characters (packjt77.f90 lines 1475–1479).
	result := make([]byte, 13)
	for i := 12; i >= 0; i-- {
		rem := 0
		for j := 0; j < 9; j++ {
			v := rem*256 + int(b[j])
			b[j] = byte(v / 42)
			rem = v % 42
		}
		result[i] = freeTextChars[rem]
	}
	return string(result)
}

// ────────────────────────────────────────────────────────────────────────────
// Grid locator helpers
// ────────────────────────────────────────────────────────────────────────────

// toGrid4 converts an integer to a 4-character Maidenhead grid locator.
// Port of subroutine to_grid4 from packjt77.f90 lines 1553–1575.
func toGrid4(n int) (string, bool) {
	j1 := n / (18 * 10 * 10)
	if j1 < 0 || j1 > 17 {
		return "", false
	}
	n -= j1 * 18 * 10 * 10
	j2 := n / (10 * 10)
	if j2 < 0 || j2 > 17 {
		return "", false
	}
	n -= j2 * 10 * 10
	j3 := n / 10
	if j3 < 0 || j3 > 9 {
		return "", false
	}
	j4 := n - j3*10
	if j4 < 0 || j4 > 9 {
		return "", false
	}
	g := string([]byte{
		byte('A' + j1),
		byte('A' + j2),
		byte('0' + j3),
		byte('0' + j4),
	})
	return g, true
}

// toGrid6 converts an integer to a 6-character Maidenhead grid locator.
// Port of subroutine to_grid6 from packjt77.f90 lines 1577–1607.
func toGrid6(n int) (string, bool) {
	j1 := n / (18 * 10 * 10 * 24 * 24)
	if j1 < 0 || j1 > 17 {
		return "", false
	}
	n -= j1 * 18 * 10 * 10 * 24 * 24
	j2 := n / (10 * 10 * 24 * 24)
	if j2 < 0 || j2 > 17 {
		return "", false
	}
	n -= j2 * 10 * 10 * 24 * 24
	j3 := n / (10 * 24 * 24)
	if j3 < 0 || j3 > 9 {
		return "", false
	}
	n -= j3 * 10 * 24 * 24
	j4 := n / (24 * 24)
	if j4 < 0 || j4 > 9 {
		return "", false
	}
	n -= j4 * 24 * 24
	j5 := n / 24
	if j5 < 0 || j5 > 23 {
		return "", false
	}
	j6 := n - j5*24
	if j6 < 0 || j6 > 23 {
		return "", false
	}
	g := string([]byte{
		byte('A' + j1), byte('A' + j2),
		byte('0' + j3), byte('0' + j4),
		byte('A' + j5), byte('A' + j6),
	})
	return g, true
}

// toGrid converts an integer to a 4- or 6-character Maidenhead grid (WSPR Type 3).
// Port of subroutine to_grid from packjt77.f90 lines 1609–1643.
func toGrid(n int) (string, bool) {
	j1 := n / (18 * 10 * 10 * 25 * 25)
	if j1 < 0 || j1 > 17 {
		return "", false
	}
	n -= j1 * 18 * 10 * 10 * 25 * 25
	j2 := n / (10 * 10 * 25 * 25)
	if j2 < 0 || j2 > 17 {
		return "", false
	}
	n -= j2 * 10 * 10 * 25 * 25
	j3 := n / (10 * 25 * 25)
	if j3 < 0 || j3 > 9 {
		return "", false
	}
	n -= j3 * 10 * 25 * 25
	j4 := n / (25 * 25)
	if j4 < 0 || j4 > 9 {
		return "", false
	}
	n -= j4 * 25 * 25
	j5 := n / 25
	if j5 < 0 || j5 > 24 {
		return "", false
	}
	j6 := n - j5*25
	if j6 < 0 || j6 > 24 {
		return "", false
	}
	g := string([]byte{
		byte('A' + j1), byte('A' + j2),
		byte('0' + j3), byte('0' + j4),
	})
	if j5 != 24 || j6 != 24 {
		g += string([]byte{byte('A' + j5), byte('A' + j6)})
	}
	return g, true
}

// ────────────────────────────────────────────────────────────────────────────
// WSPR prefix/suffix helpers
// ────────────────────────────────────────────────────────────────────────────

// wspr2Prefix decodes a WSPR Type 2 prefix from a base-36 number.
// Port of packjt77.f90 lines 414–422.
func wspr2Prefix(npfx int) string {
	var cpfx [3]byte
	for i := 2; i >= 0; i-- {
		j := npfx % 36
		cpfx[i] = a2set[j]
		npfx /= 36
		if npfx == 0 {
			break
		}
	}
	return strings.TrimLeft(string(cpfx[:]), "\x00")
}

// wspr2Suffix decodes a WSPR Type 2 suffix.
// Port of packjt77.f90 lines 426–439.
func wspr2Suffix(npfx int) (string, bool) {
	if npfx <= 35 {
		return string(a2set[npfx]), true
	} else if npfx <= 1295 {
		c1 := a2set[npfx/36]
		c2 := a2set[npfx%36]
		return string([]byte{c1, c2}), true
	} else if npfx <= 12959 {
		c1 := a2set[npfx/360]
		c2 := a2set[(npfx/10)%36]
		c3 := a2set[npfx%10]
		return string([]byte{c1, c2, c3}), true
	}
	return "", false
}

// formatReport formats an SNR report as ±DD string.
// Port of Fortran: write(crpt,'(i3.2)') irpt; if(crpt(1:1).eq.' ') crpt(1:1)='+'
func formatReport(irpt int) string {
	if irpt >= 0 {
		return fmt.Sprintf("+%02d", irpt)
	}
	return fmt.Sprintf("-%02d", -irpt)
}
