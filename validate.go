package ft8x

import (
	"strings"
	"unicode"
)

// Plausibility filters for decoded FT8 messages.
//
// These filters reject false decodes by validating callsign structure
// (ITU format) and message plausibility.  They are applied after
// Unpack77 succeeds — a decoded message that unpacks correctly may still
// be implausible (e.g. "9AAAA 7BBBB +03" has syntactically valid n28
// values but no real callsign structure).

// PlausibleCallsign checks whether a callsign string has valid ITU
// amateur radio callsign structure compatible with the FT8 pack28
// encoding.
//
// Pack28 standard callsign rules (from WSJT-X packjt77.f90 lines 708–738):
//   - Find the last digit (the "call-area digit") — must be at position
//     2 or 3 (1-indexed) in the raw callsign
//   - Prefix (before the call-area digit) must contain at least one letter
//     and cannot be all digits
//   - Suffix (after the call-area digit) must be 1–3 letters
//   - Overall length 3–6 characters (letters and digits only)
//
// Special tokens (CQ, DE, QRZ, CQ_nnn, CQ_LLLL) and hash-encoded calls
// (<...>) are always considered plausible.
func PlausibleCallsign(call string) bool {
	call = strings.TrimSpace(call)
	if call == "" {
		return false
	}

	// Special tokens are always plausible.
	if call == "CQ" || call == "DE" || call == "QRZ" {
		return true
	}
	if strings.HasPrefix(call, "CQ ") || strings.HasPrefix(call, "CQ_") {
		return true
	}
	// Hash-encoded calls.
	if strings.HasPrefix(call, "<") && strings.HasSuffix(call, ">") {
		return true
	}

	// Strip /R or /P suffix for validation.
	base := call
	if strings.HasSuffix(base, "/R") || strings.HasSuffix(base, "/P") {
		base = base[:len(base)-2]
	}

	// Compound callsigns: PREFIX/CALL or CALL/SUFFIX (e.g., V4/SP9FIH,
	// ZL4XZ/VK).  Type-4 messages carry the full non-standard callsign.
	// Validate by checking that each part contains letters and digits and
	// at least one part looks like a plausible base callsign.
	if strings.Contains(base, "/") {
		parts := strings.SplitN(base, "/", 2)
		if len(parts) == 2 && len(parts[0]) >= 1 && len(parts[1]) >= 1 {
			// Accept if either part is a plausible standard callsign.
			if PlausibleCallsign(parts[0]) || PlausibleCallsign(parts[1]) {
				return true
			}
			// Also accept prefix/call patterns where the prefix is short
			// alphanumeric (1–4 chars) and the call part is a valid callsign.
			pOk := len(parts[0]) <= 4 && isAlphaNum(parts[0])
			sOk := len(parts[1]) <= 4 && isAlphaNum(parts[1])
			if pOk || sOk {
				return true
			}
		}
		return false
	}

	n := len(base)
	// Length check: standard callsigns are 3–6 characters.
	if n < 3 || n > 6 {
		return false
	}

	// Must contain only letters and digits.
	for _, ch := range base {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) {
			return false
		}
	}

	// Find the last digit — this is the call-area digit.
	// Matching Fortran packjt77.f90 lines 711–714:
	//   do i=n,2,-1
	//      if(is_digit(c13(i:i))) exit
	//   enddo
	//   iarea=i
	iarea := -1
	for i := n - 1; i >= 1; i-- { // 0-indexed; Fortran starts from n down to 2 (1-indexed)
		if base[i] >= '0' && base[i] <= '9' {
			iarea = i + 1 // convert to 1-indexed to match Fortran
			break
		}
	}
	if iarea < 0 {
		return false // no digit found
	}

	// Fortran validation (lines 725–726):
	// iarea must be 2 or 3 (1-indexed)
	if iarea < 2 || iarea > 3 {
		return false
	}

	// Count prefix digits and letters (before the call-area digit).
	npdig := 0
	nplet := 0
	for i := 0; i < iarea-1; i++ { // 0-indexed: positions 0..iarea-2
		if base[i] >= '0' && base[i] <= '9' {
			npdig++
		}
		if base[i] >= 'A' && base[i] <= 'Z' {
			nplet++
		}
	}

	// Count suffix letters (after the call-area digit).
	nslet := 0
	for i := iarea; i < n; i++ { // 0-indexed: positions iarea..n-1 (Fortran iarea+1..n)
		if base[i] >= 'A' && base[i] <= 'Z' {
			nslet++
		}
	}

	// Fortran rejection criteria (line 725–726):
	// if(nplet.eq.0 .or. npdig.ge.iarea-1 .or. nslet.gt.3)
	if nplet == 0 {
		return false // prefix must have at least one letter
	}
	if npdig >= iarea-1 {
		return false // prefix can't be all digits
	}
	if nslet > 3 {
		return false // suffix too long
	}
	if nslet == 0 {
		return false // suffix must have at least one letter
	}

	return true
}

// PlausibleMessage checks whether a decoded FT8 message is plausible.
// It validates the callsigns embedded in the message string.
//
// The function parses the message into fields and checks:
//   - Type-1 messages "CALL1 CALL2 GRID/RPT": both callsigns must be plausible
//   - Free text messages: always plausible (can't validate arbitrary text)
//   - Messages with <...> (hashed calls): always plausible
//
// Returns true if the message passes plausibility checks.
func PlausibleMessage(msg string) bool {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return false
	}

	fields := strings.Fields(msg)
	if len(fields) == 0 {
		return false
	}

	// Free-text messages (start with special chars or contain unusual tokens)
	// are hard to validate — accept them.
	// Also accept any message containing hashed calls <...>.
	if strings.Contains(msg, "<") {
		return true
	}

	// Single-field messages are unusual but allowed.
	if len(fields) == 1 {
		return true
	}

	// Two-field message: "CALL1 CALL2" (rare but valid for type-1 with irpt=1).
	// Three-field: "CALL1 CALL2 GRID/RPT" (most common type-1).
	// Four-field: "CALL1 CALL2 R GRID" or "CQ DX CALL2 GRID" etc.

	// Extract the first two fields that look like callsigns.
	// Skip "CQ", "CQ_xxx", "DE", "QRZ", "R", "RRR", "RR73", "73", signal reports.
	for _, f := range fields {
		if isReportOrToken(f) {
			continue
		}
		// This field should be a callsign — validate it.
		if !PlausibleCallsign(f) {
			return false
		}
	}

	return true
}

// isAlphaNum returns true if s contains only ASCII letters and digits.
func isAlphaNum(s string) bool {
	for _, ch := range s {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) {
			return false
		}
	}
	return len(s) > 0
}

// isReportOrToken returns true for FT8 message tokens that are not callsigns.
func isReportOrToken(s string) bool {
	// Standard tokens.
	switch s {
	case "CQ", "DE", "QRZ", "RRR", "RR73", "73", "R":
		return true
	}
	// CQ with suffix: "CQ_DX", "CQ_160", etc.  These appear in parsed
	// messages as "CQ DX" or "CQ 160".
	if strings.HasPrefix(s, "CQ") && len(s) > 2 {
		return true
	}
	// Signal reports: +NN, -NN, R+NN, R-NN.
	if len(s) >= 2 {
		start := s
		if s[0] == 'R' && len(s) >= 3 {
			start = s[1:]
		}
		if start[0] == '+' || start[0] == '-' {
			allDigits := true
			for _, ch := range start[1:] {
				if !unicode.IsDigit(ch) {
					allDigits = false
					break
				}
			}
			if allDigits && len(start) > 1 {
				return true
			}
		}
	}
	// Grid locator: 4 chars, AA00-RR99.
	if len(s) == 4 &&
		s[0] >= 'A' && s[0] <= 'R' &&
		s[1] >= 'A' && s[1] <= 'R' &&
		s[2] >= '0' && s[2] <= '9' &&
		s[3] >= '0' && s[3] <= '9' {
		return true
	}
	// Contest exchange fields (e.g. "1A", "2B", state codes).
	// These are hard to enumerate — for now, short all-alpha fields are tokens.
	if len(s) <= 3 {
		allAlpha := true
		for _, ch := range s {
			if !unicode.IsLetter(ch) {
				allAlpha = false
				break
			}
		}
		if allAlpha {
			return true
		}
	}
	return false
}
