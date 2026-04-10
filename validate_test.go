package ft8x

import "testing"

// TestPlausibleCallsignValid verifies that known valid callsigns pass.
func TestPlausibleCallsignValid(t *testing.T) {
	valid := []string{
		// Standard callsigns.
		"K1ABC", "W2XYZ", "VE3ABC", "JA1XYZ", "DL5ABC",
		"G4ABC", "F5XYZ", "OH2XYZ", "UA3LAR", "SV2SIH",
		"A61CK", "HZ1TT", "ES2AJ", "PV8AJ", "KB7THX",
		"WB9VGJ", "VE1WT", "K4GBI", "KI8JP", "W3DQS",
		"UA1CEI", "RU1AB", "RA1OHX", "LU3DXU", "RA6ABC",
		"RV6ASU", "VK3ZSJ", "YO8RQP", "ZS4AW", "Z62NS",
		"TN8GD", "R1QD", "KB2ELA", "UY7VV", "KE6SU",
		"HA5LB", "5B4AMX",
		// With suffix.
		"K1ABC/R", "W2XYZ/P",
		// 3-char callsign.
		"K1A", "W2X",
		// Special tokens.
		"CQ", "DE", "QRZ",
		"CQ DX", "CQ_160", "CQ_EU",
		// Hash-encoded.
		"<...>", "<K1ABC>",
	}
	for _, call := range valid {
		if !PlausibleCallsign(call) {
			t.Errorf("PlausibleCallsign(%q) = false, want true", call)
		}
	}
}

// TestPlausibleCallsignInvalid verifies that implausible callsigns are rejected.
func TestPlausibleCallsignInvalid(t *testing.T) {
	invalid := []string{
		"",
		"AB",       // too short
		"ABCDEFGH", // too long
		"AAAAAA",   // no digit
		"123456",   // no letter
		"A!B1C",    // invalid character
	}
	for _, call := range invalid {
		if PlausibleCallsign(call) {
			t.Errorf("PlausibleCallsign(%q) = true, want false", call)
		}
	}
}

// TestPlausibleMessageValid verifies known valid messages pass.
func TestPlausibleMessageValid(t *testing.T) {
	valid := []string{
		"CQ K1ABC FN42",
		"SV2SIH ES2AJ -16",
		"VE1WT K4GBI 73",
		"CQ PV8AJ FJ92",
		"KB7THX WB9VGJ RR73",
		"A61CK UA1CEI KP50",
		"A61CK W3DQS -12",
		"HZ1TT RU1AB R-10",
		"<...> RA1OHX KP91",
		"<...> LU3DXU GF05",
		"CQ ZS4AW KG31",
		"CQ SV0TPN KM28",
		"CQ Z62NS KN02",
		"VK3ZSJ YO8RQP KN37",
		"R1QD KB2ELA -12",
		"UY7VV KE6SU DM14",
		"CQ TN8GD JI75",
		"HA5LB 5B4AMX RR73",
	}
	for _, msg := range valid {
		if !PlausibleMessage(msg) {
			t.Errorf("PlausibleMessage(%q) = false, want true", msg)
		}
	}
}

// TestPlausibleMessageInvalid verifies that implausible messages are rejected.
func TestPlausibleMessageInvalid(t *testing.T) {
	invalid := []string{
		"",
		"AAAAAA BBBBBB FN42", // no digits in "callsigns"
		"123456 K1ABC FN42",  // all-digit first field with no letter prefix
	}
	for _, msg := range invalid {
		if PlausibleMessage(msg) {
			t.Errorf("PlausibleMessage(%q) = true, want false", msg)
		}
	}
}

// TestPlausibleCallsignWSJTXCapture verifies that all callsigns from our
// WSJT-X reference decodes pass the plausibility filter.
func TestPlausibleCallsignWSJTXCapture(t *testing.T) {
	// All callsigns appearing in our capture reference sets.
	calls := []string{
		"SV2SIH", "ES2AJ", "VE1WT", "K4GBI", "KI8JP",
		"PV8AJ", "KB7THX", "WB9VGJ", "A61CK", "UA1CEI",
		"W3DQS", "HZ1TT", "RU1AB", "UA3LAR", "RA1OHX",
		"LU3DXU", "RA6ABC", "RV6ASU",
		"HA5LB", "5B4AMX", "ZS4AW", "SV0TPN", "Z62NS",
		"VK3ZSJ", "YO8RQP", "R1QD", "KB2ELA", "UY7VV",
		"KE6SU", "TN8GD",
	}
	for _, call := range calls {
		if !PlausibleCallsign(call) {
			t.Errorf("PlausibleCallsign(%q) = false — WSJT-X reference callsign rejected!", call)
		}
	}
}
