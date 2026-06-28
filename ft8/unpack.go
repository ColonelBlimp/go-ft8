// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"fmt"
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
}

func unpack77FromCodewordWithHashes(cw [174]int8, hashes *hashTable) (string, bool) {
	return unpack77WithHashes(cw[:77], hashes)
}

func unpack77WithHashes(bits []int8, hashes *hashTable) (string, bool) {
	if len(bits) < 77 {
		return "", false
	}
	i3 := readBits(bits, 74, 3)
	n3 := readBits(bits, 71, 3)

	switch {
	case i3 == 0 && n3 == 0:
		return unpackFreeText(bits[:71])
	case i3 == 0 && (n3 == 3 || n3 == 4):
		return unpackARRLFieldDay(bits, n3, hashes)
	case i3 == 1 || i3 == 2:
		return unpackStandard(bits, i3, hashes)
	case i3 == 4:
		return unpackType4(bits, hashes)
	default:
		return "", false
	}
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
	out := make([]byte, 13)
	for i := 0; i < 13; i++ {
		v := readBits(bits, i*5, 5)
		if v >= len(alphabet) {
			return "", false
		}
		out[i] = alphabet[v]
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

func (h *hashTable) Lookup22(hash int) string {
	for _, entry := range h.calls22 {
		if entry.hash == hash {
			return entry.call
		}
	}
	return ""
}

func hashCall(call string, bits int) int {
	const alphabet = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ/"
	call = strings.ToUpper(strings.TrimSpace(call))
	var n uint64
	// FT8 callsign hashes operate on an 11-character window, padded with
	// spaces or truncated to match the WSJT-X packing convention.
	for i := 0; i < 11; i++ {
		ch := byte(' ')
		if i < len(call) {
			ch = call[i]
		}
		idx := strings.IndexByte(alphabet, ch)
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

func formatReport(snr int) string {
	if snr >= 0 {
		return fmt.Sprintf("+%02d", snr)
	}
	return fmt.Sprintf("-%02d", -snr)
}

func unpackBase38Call(n uint64) string {
	const alphabet = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ/"
	out := make([]byte, 11)
	for i := 10; i >= 0; i-- {
		out[i] = alphabet[n%38]
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
