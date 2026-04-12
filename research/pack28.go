// pack28.go — Callsign packing for the research package.
//
// Port of pack28 from wsjt-wsjtx/lib/77bit/packjt77.f90 lines 621–751,
// ihashcall from lines 64–79, and stdcall from lib/qra/q65/q65_set_list.f90.
//
// Pure port — no production dependency.

package research

import "strings"

const (
	packNTOKENS = 2063592
	packMAX22   = 4194304
)

// Character sets for standard callsign encoding (packjt77.f90 lines 636-639).
var (
	packA1 = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ" // 37 chars
	packA2 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"  // 36 chars
	packA3 = "0123456789"                            // 10 chars
	packA4 = " ABCDEFGHIJKLMNOPQRSTUVWXYZ"           // 27 chars
)

// ihashcall computes a hash of a callsign for non-standard call encoding.
//
// Port of integer function ihashcall from packjt77.f90 lines 64–79.
//
// Parameters:
//   - callsign: up to 11 characters
//   - m: number of hash bits (10, 12, or 22)
func ihashcall(callsign string, m int) int {
	c := " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ/"
	// Pad or truncate to 11 characters
	cs := callsign
	for len(cs) < 11 {
		cs += " "
	}
	cs = cs[:11]

	var n8 uint64
	for i := 0; i < 11; i++ {
		j := strings.IndexByte(c, cs[i])
		if j < 0 {
			j = 0
		}
		n8 = 38*n8 + uint64(j)
	}
	// Fortran: ihashcall = ishft(47055833459_8 * n8, m-64)
	// This is equivalent to (47055833459 * n8) >> (64 - m)
	result := (47055833459 * n8) >> uint(64-m)
	return int(result)
}

// stdcall checks if a callsign has standard ITU format.
//
// Port of subroutine stdcall from wsjt-wsjtx/lib/qra/q65/q65_set_list.f90.
//
// Standard format: prefix (1-2 letters + optional digit) + area digit + suffix (1-3 letters).
func stdcall(callsign string) bool {
	cs := strings.TrimSpace(callsign)
	n := len(cs)
	if n < 2 || n > 6 {
		return false
	}

	// Find rightmost digit (call area)
	iarea := -1
	for i := n - 1; i >= 1; i-- {
		if cs[i] >= '0' && cs[i] <= '9' {
			iarea = i
			break
		}
	}
	if iarea < 0 {
		return false
	}

	// Count digits and letters before call area
	npdig := 0
	nplet := 0
	for i := 0; i < iarea; i++ {
		if cs[i] >= '0' && cs[i] <= '9' {
			npdig++
		}
		if cs[i] >= 'A' && cs[i] <= 'Z' {
			nplet++
		}
	}

	// Count letters in suffix
	nslet := 0
	for i := iarea + 1; i < n; i++ {
		if cs[i] >= 'A' && cs[i] <= 'Z' {
			nslet++
		}
	}

	// Fortran: if(iarea.lt.2 .or. iarea.gt.3 .or. nplet.eq.0 .or.
	//              npdig.ge.iarea-1 .or. nslet.gt.3) std=.false.
	// Note: Fortran iarea is 1-based, Go iarea is 0-based
	iarea1 := iarea + 1 // convert to 1-based for comparison
	if iarea1 < 2 || iarea1 > 3 || nplet == 0 || npdig >= iarea1-1 || nslet > 3 {
		return false
	}

	return true
}

// pack28 encodes a callsign into a 28-bit integer.
//
// Port of subroutine pack28 from packjt77.f90 lines 621–751.
//
// Handles:
//   - Special tokens: DE, QRZ, CQ, CQ_nnn, CQ_AAAA
//   - Standard callsigns: 6-char packed encoding
//   - Non-standard callsigns: 22-bit hash
func pack28(callsign string) int {
	cs := strings.TrimSpace(strings.ToUpper(callsign))

	// Special tokens (packjt77.f90 lines 652-696)
	if cs == "DE" {
		return 0 & ((1 << 28) - 1)
	}
	if cs == "QRZ" {
		return 1 & ((1 << 28) - 1)
	}
	if cs == "CQ" {
		return 2 & ((1 << 28) - 1)
	}

	// CQ_nnn or CQ_AAAA
	if strings.HasPrefix(cs, "CQ_") || strings.HasPrefix(cs, "CQ ") {
		rest := cs[3:]
		n := len(rest)
		if n >= 1 && n <= 4 {
			nlet := 0
			nnum := 0
			for _, c := range rest {
				if c >= 'A' && c <= 'Z' {
					nlet++
				}
				if c >= '0' && c <= '9' {
					nnum++
				}
			}
			if nnum == 3 && nlet == 0 && n == 3 {
				nqsy := 0
				for _, c := range rest {
					nqsy = nqsy*10 + int(c-'0')
				}
				return (3 + nqsy) & ((1 << 28) - 1)
			}
			if nlet >= 1 && nlet <= 4 && nnum == 0 {
				// Right-justify in 4-char field
				c4 := rest
				for len(c4) < 4 {
					c4 = " " + c4
				}
				m := 0
				for i := 0; i < 4; i++ {
					j := 0
					if c4[i] >= 'A' && c4[i] <= 'Z' {
						j = int(c4[i]-'A') + 1
					}
					m = 27*m + j
				}
				return (3 + 1000 + m) & ((1 << 28) - 1)
			}
		}
	}

	// Check for standard callsign
	if !stdcall(cs) {
		// Non-standard: 22-bit hash
		n22 := ihashcall(cs, 22)
		return (packNTOKENS + n22) & ((1 << 28) - 1)
	}

	// Standard callsign encoding (packjt77.f90 lines 708-748)
	n := len(cs)

	// Find call area (rightmost digit)
	iarea := -1
	for i := n - 1; i >= 1; i-- {
		if cs[i] >= '0' && cs[i] <= '9' {
			iarea = i
			break
		}
	}

	// Normalize to 6-character callsign
	// Fortran: if(iarea.eq.2) callsign=' '//c13(1:5)   (1-based iarea)
	//          if(iarea.eq.3) callsign=c13(1:6)
	var call6 string
	iarea1 := iarea + 1 // 1-based
	if iarea1 == 2 {
		call6 = " " + cs
		if len(call6) > 6 {
			call6 = call6[:6]
		}
	} else {
		call6 = cs
	}
	for len(call6) < 6 {
		call6 += " "
	}
	call6 = call6[:6]

	i1 := strings.IndexByte(packA1, call6[0])
	i2 := strings.IndexByte(packA2, call6[1])
	i3 := strings.IndexByte(packA3, call6[2])
	i4 := strings.IndexByte(packA4, call6[3])
	i5 := strings.IndexByte(packA4, call6[4])
	i6 := strings.IndexByte(packA4, call6[5])

	if i1 < 0 || i2 < 0 || i3 < 0 || i4 < 0 || i5 < 0 || i6 < 0 {
		// Invalid character — fall back to hash
		n22 := ihashcall(cs, 22)
		return (packNTOKENS + n22) & ((1 << 28) - 1)
	}

	n28 := 36*10*27*27*27*i1 + 10*27*27*27*i2 + 27*27*27*i3 + 27*27*i4 + 27*i5 + i6
	n28 += packNTOKENS + packMAX22

	return n28 & ((1 << 28) - 1)
}
