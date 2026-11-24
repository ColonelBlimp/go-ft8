// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "strings"

// CallsignHistory renders shared FT4/FT8 payloads with bounded callsign hash
// history. Its zero value is empty and ready for use. Each receiver stream
// should own its history; it is not safe for concurrent use or copying after use.
//
// History follows the FT8 decoder's learning rules: successfully parsed standard
// messages save the full second call (including /P or /R), and type 4 messages
// save the spelled-out call. Protocol tokens and invalid calls are not saved.
// Other full-call fields are not learned. Up to 1000 entries are kept, with the
// most recently saved match winning hash collisions.
type CallsignHistory struct {
	hashes hashTable
}

// ParsePayload77 renders a verified payload and learns calls only if parsing
// succeeds. Calls learned here are available to subsequent parses; earlier text
// is not re-rendered. An unresolved hash renders as <...>.
func (h *CallsignHistory) ParsePayload77(payload Payload77) (string, ParseStatus) {
	parsed := parsePayload77AndRemember(payload, &h.hashes)
	return parsed.Text, parsed.Status
}

// RememberCallsigns seeds rendering history with known calls. Calls are trimmed,
// uppercased, and may have one surrounding pair of angle brackets. Invalid or
// reserved tokens, non-ASCII input, embedded whitespace, and calls outside
// 3-11 base-38 characters are ignored. This validates protocol tokens, not
// whether a callsign has been issued.
//
// Seeds use the decoder's usual 1000-entry cap and recency/collision rules.
// They affect hash rendering only, without providing signal-decoding AP hints.
func (h *CallsignHistory) RememberCallsigns(calls ...string) {
	rememberCallsigns(&h.hashes, calls)
}

func rememberCallsigns(hashes *hashTable, calls []string) {
	for _, call := range calls {
		if normalized, ok := normalizeHistoryCallsign(call); ok {
			hashes.Save(normalized)
		}
	}
}

func normalizeHistoryCallsign(call string) (string, bool) {
	// Check before Unicode case folding: for example, long s (U+017F) must
	// not turn into an accepted ASCII S in an otherwise invalid callsign.
	for i := 0; i < len(call); i++ {
		if call[i] >= 0x80 {
			return "", false
		}
	}
	call, ok := normalizeHashCallToken(call)
	return call, ok && !strings.Contains(call, " ")
}

// CallsignHash returns the shared FT4/FT8 callsign hash for a width of 10, 12,
// or 22 bits. Calls use the same normalization and validation as
// CallsignHistory.RememberCallsigns. Invalid calls or widths return (0, false).
// A matching hash indicates a possible identity; collisions are possible.
func CallsignHash(call string, bits int) (int, bool) {
	if bits != 10 && bits != 12 && bits != 22 {
		return 0, false
	}
	call, ok := normalizeHistoryCallsign(call)
	if !ok {
		return 0, false
	}
	return hashCall(call, bits), true
}
