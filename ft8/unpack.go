// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"fmt"
	mathbits "math/bits"
	"strings"
)

const (
	ft8MaxGrid4 = 32400
	ft8NTokens  = 2063592
	ft8Max22    = 4194304
	ft8MaxHash  = 1000
)

type hashEntry struct {
	hash int
	call string
}

type hashTable struct {
	calls22 []hashEntry
	// frozen marks a read-only snapshot shared by concurrent candidate
	// decode workers: Save becomes a no-op so lookups stay race-free and
	// deterministic. The decode commit phase re-runs unpack against the
	// live table, so saves are never lost.
	frozen       bool
	pendingSaves *[]string
}

// frozenSnapshot returns a read-only copy of the table for concurrent
// workers. The copy keeps lookups deterministic regardless of worker
// scheduling; Save calls against it are ignored.
func (h *hashTable) frozenSnapshot() *hashTable {
	return &hashTable{
		calls22: append([]hashEntry(nil), h.calls22...),
		frozen:  true,
	}
}

// transactionalSnapshot shares immutable lookup state while collecting Save
// calls for a later commit. Candidate validation can therefore remain
// side-effect-free without copying the bounded hash history on every decode.
func (h *hashTable) transactionalSnapshot(pending *[]string) *hashTable {
	return &hashTable{calls22: h.calls22, pendingSaves: pending}
}

func unpack77FromCodewordWithHashes(cw [174]int8, hashes *hashTable) (string, bool) {
	return unpack77WithHashes(cw[:77], hashes)
}

func unpack77WithHashes(bits []int8, hashes *hashTable) (string, bool) {
	if len(bits) < 77 {
		return "", false
	}
	parsed := parsePayload77WithHashes(payload77FromBits(bits), hashes)
	return parsed.Text, parsed.Status == ParseStatusParsed
}

type payloadParseResult struct {
	Payload Payload77
	Text    string
	Type    MessageType
	Status  ParseStatus
}

func parsePayload77WithHashes(payload Payload77, hashes *hashTable) payloadParseResult {
	typeID := payload.MessageType()
	bitsArray := payload.int8Bits()
	bits := bitsArray[:]
	result := payloadParseResult{Payload: payload, Type: typeID}
	var ok bool

	switch {
	case typeID.I3 == 0 && typeID.N3 == 0:
		result.Text, ok = unpackFreeText(bits[:71])
	case typeID.I3 == 0 && typeID.N3 == 1:
		result.Text, ok = unpackDXpedition(bits, hashes)
	case typeID.I3 == 0 && (typeID.N3 == 3 || typeID.N3 == 4):
		result.Text, ok = unpackARRLFieldDay(bits, int(typeID.N3), hashes)
	case typeID.I3 == 0 && typeID.N3 == 5:
		result.Text, ok = unpackTelemetry(bits)
	case typeID.I3 == 0 && typeID.N3 == 6:
		result.Status = ParseStatusUnsupported
		return result
	case typeID.I3 == 0:
		result.Status = ParseStatusUnknownType
		return result
	case typeID.I3 == 1 || typeID.I3 == 2:
		result.Text, ok = unpackStandard(bits, int(typeID.I3), hashes)
	case typeID.I3 == 3:
		result.Text, ok = unpackARRLRTTY(bits, hashes)
	case typeID.I3 == 4:
		result.Text, ok = unpackType4(bits, hashes)
	case typeID.I3 == 5:
		result.Text, ok = unpackEUVHFContest(bits, hashes)
	default:
		result.Status = ParseStatusUnknownType
		return result
	}
	if ok {
		result.Status = ParseStatusParsed
	} else {
		result.Status = ParseStatusInvalid
	}
	return result
}

func unpackDXpedition(bits []int8, hashes *hashTable) (string, bool) {
	n28a := readBits(bits, 0, 28)
	n28b := readBits(bits, 28, 28)
	n10 := readBits(bits, 56, 10)
	n5 := readBits(bits, 66, 5)

	call1, ok1 := unpack28WithHashes(n28a, hashes)
	call2, ok2 := unpack28WithHashes(n28b, hashes)
	if !ok1 || !ok2 || n28a <= 2 || n28b <= 2 {
		return "", false
	}
	call3 := "<...>"
	if hashes != nil {
		if call := hashes.Lookup10(n10); call != "" {
			call3 = "<" + call + ">"
		}
	}
	report := formatReport(2*n5 - 30)
	return call1 + " RR73; " + call2 + " " + call3 + " " + report, true
}

func unpackTelemetry(bits []int8) (string, bool) {
	if len(bits) < 71 {
		return "", false
	}
	part1 := readBits(bits, 0, 23)
	part2 := readBits(bits, 23, 24)
	part3 := readBits(bits, 47, 24)
	text := fmt.Sprintf("%06X%06X%06X", part1, part2, part3)
	return strings.TrimLeft(text, "0"), true
}

func unpackARRLRTTY(bits []int8, hashes *hashTable) (string, bool) {
	itu := readBits(bits, 0, 1)
	n28a := readBits(bits, 1, 28)
	n28b := readBits(bits, 29, 28)
	ir := readBits(bits, 57, 1)
	irpt := readBits(bits, 58, 3)
	nexch := readBits(bits, 61, 13)

	call1, ok1 := unpack28WithHashes(n28a, hashes)
	call2, ok2 := unpack28WithHashes(n28b, hashes)
	if !ok1 || !ok2 {
		return "", false
	}

	var exchange string
	switch {
	case nexch >= 1 && nexch <= 7999:
		exchange = fmt.Sprintf("%04d", nexch)
	case nexch >= 8001:
		var ok bool
		exchange, ok = arrlRTTYMultiplier(nexch - 8000)
		if !ok {
			return "", false
		}
	default:
		return "", false
	}

	var text strings.Builder
	if itu != 0 {
		text.WriteString("TU; ")
	}
	text.WriteString(call1)
	text.WriteByte(' ')
	text.WriteString(call2)
	text.WriteByte(' ')
	if ir != 0 {
		text.WriteString("R ")
	}
	text.WriteString(fmt.Sprintf("5%d9", irpt+2))
	text.WriteByte(' ')
	text.WriteString(exchange)
	return text.String(), true
}

func unpackEUVHFContest(bits []int8, hashes *hashTable) (string, bool) {
	n12 := readBits(bits, 0, 12)
	n22 := readBits(bits, 12, 22)
	ir := readBits(bits, 34, 1)
	irpt := readBits(bits, 35, 3)
	serial := readBits(bits, 38, 11)
	gridIndex := readBits(bits, 49, 25)

	grid, ok := toGrid6(gridIndex)
	if !ok {
		return "", false
	}
	call1 := "<...>"
	call2 := "<...>"
	if hashes != nil {
		if call := hashes.Lookup12(n12); call != "" {
			call1 = "<" + call + ">"
		}
		if call := hashes.Lookup22(n22); call != "" {
			call2 = "<" + call + ">"
		}
	}
	exchange := fmt.Sprintf("%02d%04d", 52+irpt, serial)
	if ir != 0 {
		return call1 + " " + call2 + " R " + exchange + " " + grid, true
	}
	return call1 + " " + call2 + " " + exchange + " " + grid, true
}

func unpackStandard(bits []int8, i3 int, hashes *hashTable) (string, bool) {
	n28a := readBits(bits, 0, 28)
	ipa := readBits(bits, 28, 1)
	n28b := readBits(bits, 29, 28)
	ipb := readBits(bits, 57, 1)
	ir := readBits(bits, 58, 1)
	igrid4 := readBits(bits, 59, 15)

	call1, ok1 := unpack28WithHashes(n28a, hashes)
	call2, ok2 := unpack28WithHashes(n28b, hashes)
	if !ok1 || !ok2 {
		return "", false
	}
	call1 = normalizeCQ(call1)
	call2 = normalizeCQ(call2)
	call1 = addPortableSuffix(call1, ipa, i3)
	call2 = addPortableSuffix(call2, ipb, i3)
	if hashes != nil && !strings.Contains(call2, "<") {
		hashes.Save(call2)
	}

	if igrid4 <= ft8MaxGrid4 {
		grid, ok := toGrid4(igrid4)
		if !ok || strings.HasPrefix(call1, "CQ ") && ir == 1 {
			return "", false
		}
		if ir == 1 {
			return strings.TrimSpace(call1 + " " + call2 + " R " + grid), true
		}
		return strings.TrimSpace(call1 + " " + call2 + " " + grid), true
	}

	irpt := igrid4 - ft8MaxGrid4
	if strings.HasPrefix(call1, "CQ ") && irpt >= 2 {
		return "", false
	}
	switch irpt {
	case 1:
		return strings.TrimSpace(call1 + " " + call2), true
	case 2:
		return strings.TrimSpace(call1 + " " + call2 + " RRR"), true
	case 3:
		return strings.TrimSpace(call1 + " " + call2 + " RR73"), true
	case 4:
		return strings.TrimSpace(call1 + " " + call2 + " 73"), true
	default:
		if irpt < 5 {
			return "", false
		}
		isnr := irpt - 35
		if isnr > 50 {
			isnr -= 101
		}
		report := formatReport(isnr)
		if ir == 1 {
			return strings.TrimSpace(call1 + " " + call2 + " R" + report), true
		}
		return strings.TrimSpace(call1 + " " + call2 + " " + report), true
	}
}

func unpackType4(bits []int8, hashes *hashTable) (string, bool) {
	n12 := readBits(bits, 0, 12)
	n58 := readBits64(bits, 12, 58)
	iflip := readBits(bits, 70, 1)
	nrpt := readBits(bits, 71, 2)
	icq := readBits(bits, 73, 1)

	callHash := "<...>"
	if hashes != nil {
		if call := hashes.Lookup12(n12); call != "" {
			callHash = "<" + call + ">"
		}
	}
	callText := unpackBase38Call(n58)
	if callText == "" {
		return "", false
	}
	if hashes != nil {
		hashes.Save(callText)
	}

	call1, call2 := callHash, callText
	if iflip != 0 {
		call1, call2 = callText, callHash
	}
	if icq != 0 {
		return strings.TrimSpace("CQ " + call2), true
	}

	switch nrpt {
	case 0:
		return strings.TrimSpace(call1 + " " + call2), true
	case 1:
		return strings.TrimSpace(call1 + " " + call2 + " RRR"), true
	case 2:
		return strings.TrimSpace(call1 + " " + call2 + " RR73"), true
	case 3:
		return strings.TrimSpace(call1 + " " + call2 + " 73"), true
	default:
		return "", false
	}
}

func unpackFreeText(bits []int8) (string, bool) {
	const alphabet = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ+-./?"
	if len(bits) < 71 {
		return "", false
	}
	var high uint64
	for i := 0; i < 7; i++ {
		high = high<<1 | uint64(bits[i]&1)
	}
	var low uint64
	for i := 7; i < 71; i++ {
		low = low<<1 | uint64(bits[i]&1)
	}
	out := make([]byte, 13)
	for i := len(out) - 1; i >= 0; i-- {
		quotientHigh := high / uint64(len(alphabet))
		remainderHigh := high % uint64(len(alphabet))
		quotientLow, remainder := mathbits.Div64(remainderHigh, low, uint64(len(alphabet)))
		out[i] = alphabet[remainder]
		high, low = quotientHigh, quotientLow
	}
	msg := strings.TrimSpace(string(out))
	return msg, msg != ""
}

func unpack28WithHashes(n28 int, hashes *hashTable) (string, bool) {
	const cqAlphabet = " ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if n28 < ft8NTokens {
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
			n := n28 - 1003
			i1 := n / (27 * 27 * 27)
			n -= i1 * 27 * 27 * 27
			i2 := n / (27 * 27)
			n -= i2 * 27 * 27
			i3 := n / 27
			i4 := n - 27*i3
			token := string([]byte{cqAlphabet[i1], cqAlphabet[i2], cqAlphabet[i3], cqAlphabet[i4]})
			return "CQ_" + strings.TrimSpace(token), true
		default:
			return "<...>", true
		}
	}

	n28 -= ft8NTokens
	if n28 < ft8Max22 {
		if hashes != nil {
			if call := hashes.Lookup22(n28); call != "" {
				return "<" + call + ">", true
			}
		}
		return "<...>", true
	}

	n := n28 - ft8Max22
	if n < 0 {
		return "", false
	}
	const c1 = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const c2 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const c3 = "0123456789"
	const c4 = " ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	i1 := n / (36 * 10 * 27 * 27 * 27)
	n -= i1 * 36 * 10 * 27 * 27 * 27
	i2 := n / (10 * 27 * 27 * 27)
	n -= i2 * 10 * 27 * 27 * 27
	i3 := n / (27 * 27 * 27)
	n -= i3 * 27 * 27 * 27
	i4 := n / (27 * 27)
	n -= i4 * 27 * 27
	i5 := n / 27
	i6 := n - 27*i5
	if i1 < 0 || i1 >= len(c1) || i2 < 0 || i2 >= len(c2) || i3 < 0 || i3 >= len(c3) ||
		i4 < 0 || i4 >= len(c4) || i5 < 0 || i5 >= len(c4) || i6 < 0 || i6 >= len(c4) {
		return "", false
	}
	call := strings.TrimSpace(string([]byte{c1[i1], c2[i2], c3[i3], c4[i4], c4[i5], c4[i6]}))
	if !callOK(call) || strings.Contains(call, " ") {
		return "", false
	}
	return call, true
}

func (h *hashTable) Save(call string) {
	if h.pendingSaves != nil {
		*h.pendingSaves = append(*h.pendingSaves, call)
		return
	}
	if h.frozen {
		return
	}
	call = strings.TrimSpace(strings.Trim(call, "<>"))
	if call == "" || call == "..." || strings.Contains(call, "<") || len(call) < 3 {
		return
	}
	n22 := hashCall(call, 22)
	for i := range h.calls22 {
		if h.calls22[i].hash == n22 {
			entry := hashEntry{hash: n22, call: call}
			copy(h.calls22[1:i+1], h.calls22[:i])
			h.calls22[0] = entry
			return
		}
	}
	h.calls22 = append([]hashEntry{{hash: n22, call: call}}, h.calls22...)
	if len(h.calls22) > ft8MaxHash {
		h.calls22 = h.calls22[:ft8MaxHash]
	}
}

func (h *hashTable) Lookup12(hash int) string {
	if hash < 0 || hash >= 1<<12 {
		return ""
	}
	for _, entry := range h.calls22 {
		if hashCall(entry.call, 12) == hash {
			return entry.call
		}
	}
	return ""
}

func (h *hashTable) Lookup10(hash int) string {
	if hash < 0 || hash >= 1<<10 {
		return ""
	}
	for _, entry := range h.calls22 {
		if hashCall(entry.call, 10) == hash {
			return entry.call
		}
	}
	return ""
}

func (h *hashTable) Lookup22(hash int) string {
	for _, entry := range h.calls22 {
		if entry.hash == hash {
			return entry.call
		}
	}
	return ""
}

func hashCall(call string, bits int) int {
	call = strings.ToUpper(strings.TrimSpace(call))
	var n uint64
	// FT8 callsign hashes operate on an 11-character window, padded with
	// spaces or truncated to match the WSJT-X packing convention.
	for i := 0; i < 11; i++ {
		ch := byte(' ')
		if i < len(call) {
			ch = call[i]
		}
		idx := strings.IndexByte(ft8Base38Alphabet, ch)
		if idx < 0 {
			idx = 0
		}
		n = 38*n + uint64(idx)
	}
	return int((uint64(47055833459) * n) >> uint(64-bits))
}

func normalizeCQ(call string) string {
	if strings.HasPrefix(call, "CQ_") {
		return "CQ " + strings.TrimSpace(call[3:])
	}
	return call
}

func addPortableSuffix(call string, flag int, i3 int) string {
	if flag == 0 || strings.Contains(call, "<") || len(call) < 3 {
		return call
	}
	if i3 == 1 {
		return call + "/R"
	}
	if i3 == 2 {
		return call + "/P"
	}
	return call
}

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
	return string([]byte{byte('A' + j1), byte('A' + j2), byte('0' + j3), byte('0' + j4)}), true
}

func toGrid6(n int) (string, bool) {
	const maxGrid6 = 18*18*10*10*24*24 - 1
	if n < 0 || n > maxGrid6 {
		return "", false
	}
	j1 := n / (18 * 10 * 10 * 24 * 24)
	n -= j1 * 18 * 10 * 10 * 24 * 24
	j2 := n / (10 * 10 * 24 * 24)
	n -= j2 * 10 * 10 * 24 * 24
	j3 := n / (10 * 24 * 24)
	n -= j3 * 10 * 24 * 24
	j4 := n / (24 * 24)
	n -= j4 * 24 * 24
	j5 := n / 24
	j6 := n - j5*24
	return string([]byte{
		byte('A' + j1), byte('A' + j2), byte('0' + j3),
		byte('0' + j4), byte('A' + j5), byte('A' + j6),
	}), true
}

func formatReport(snr int) string {
	if snr >= 0 {
		return fmt.Sprintf("+%02d", snr)
	}
	return fmt.Sprintf("-%02d", -snr)
}

func unpackBase38Call(n uint64) string {
	out := make([]byte, 11)
	for i := 10; i >= 0; i-- {
		out[i] = ft8Base38Alphabet[n%38]
		n /= 38
	}
	return strings.TrimSpace(string(out))
}

func callOK(w string) bool {
	n := len(strings.TrimSpace(w))
	if n < 3 || w[0] == 'Q' {
		return false
	}
	i0 := -1
	for i := n - 1; i >= 0; i-- {
		if isDigit(w[i]) {
			i0 = i
			break
		}
	}
	if i0 != 1 && i0 != 2 {
		return false
	}

	prefix := w[:i0]
	suffix := w[i0+1 : n]
	lettersInPrefix := 0
	for i := 0; i < len(prefix); i++ {
		if isLetter(prefix[i]) {
			lettersInPrefix++
		}
	}
	if lettersInPrefix == 0 {
		return false
	}
	for i := 0; i < len(suffix); i++ {
		if !isLetter(suffix[i]) {
			return false
		}
	}
	return true
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isLetter(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

func readBits(bits []int8, start int, width int) int {
	v := 0
	for i := 0; i < width; i++ {
		v = v*2 + int(bits[start+i]&1)
	}
	return v
}

func readBits64(bits []int8, start int, width int) uint64 {
	var v uint64
	for i := 0; i < width; i++ {
		v = v*2 + uint64(bits[start+i]&1)
	}
	return v
}
