// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"fmt"
	"strings"

	"github.com/ColonelBlimp/go-ft8/internal/ftx77"
)

const ft8Base38Alphabet = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ/"

type standardCallSuffix int

const (
	standardCallNoSuffix standardCallSuffix = iota
	standardCallRover
	standardCallPortable
)

func pack77StandardMessage(msg string) ([77]int8, bool) {
	fields := strings.Fields(strings.ToUpper(msg))
	if bits, ok := pack77ARRLFieldDayMessage(fields); ok {
		return bits, true
	}
	if bits, ok := pack77Type4CompoundMessage(fields); ok {
		return bits, true
	}
	if len(fields) >= 3 && fields[0] == "CQ" {
		if _, _, _, ok := pack28WithSuffix(fields[2]); ok {
			normalized := make([]string, 0, len(fields)-1)
			normalized = append(normalized, "CQ_"+fields[1], fields[2])
			normalized = append(normalized, fields[3:]...)
			fields = normalized
		}
	}
	var out [77]int8
	if len(fields) < 2 || len(fields) > 4 {
		return out, false
	}

	n28a, ipa, suffixA, ok := pack28WithSuffix(fields[0])
	if !ok {
		return out, false
	}
	n28b, ipb, suffixB, ok := pack28WithSuffix(fields[1])
	if !ok {
		return out, false
	}
	i3 := 1
	hasPortable := suffixA == standardCallPortable || suffixB == standardCallPortable
	hasRover := suffixA == standardCallRover || suffixB == standardCallRover
	if hasPortable && hasRover {
		return out, false
	}
	if hasPortable {
		i3 = 2
	}

	ir := 0
	igrid4 := ft8MaxGrid4 + 1
	if len(fields) >= 3 {
		last := fields[len(fields)-1]
		if isGrid4(last) {
			grid, ok := grid4Index(last)
			if !ok {
				return out, false
			}
			igrid4 = grid
			if len(fields) == 4 && fields[2] == "R" {
				ir = 1
			}
		} else {
			irpt, reportIsResponse, ok := packReportToken(last)
			if !ok {
				return out, false
			}
			ir = 0
			if reportIsResponse {
				ir = 1
			}
			igrid4 = ft8MaxGrid4 + irpt
		}
	}

	writeBits(out[:], 0, 28, n28a)
	writeBits(out[:], 28, 1, ipa)
	writeBits(out[:], 29, 28, n28b)
	writeBits(out[:], 57, 1, ipb)
	writeBits(out[:], 58, 1, ir)
	writeBits(out[:], 59, 15, igrid4)
	writeBits(out[:], 74, 3, i3)
	return out, true
}

func pack77Type4CompoundMessage(fields []string) ([77]int8, bool) {
	var out [77]int8
	if len(fields) == 2 && fields[0] == "CQ" {
		n58, ok := packType4Call(fields[1], true)
		if !ok {
			return out, false
		}
		writeBits64(out[:], 12, 58, n58)
		writeBits(out[:], 73, 1, 1)
		writeBits(out[:], 74, 3, 4)
		return out, true
	}

	if len(fields) < 2 || len(fields) > 3 {
		return out, false
	}
	nrpt := 0
	if len(fields) == 3 {
		switch fields[2] {
		case "RRR":
			nrpt = 1
		case "RR73":
			nrpt = 2
		case "73":
			nrpt = 3
		default:
			return out, false
		}
	}

	// WSJT-X pack77_4 hashes a <bracketed> call unconditionally and spells
	// the other call in c58 with no structural checks; pack77_1 routes
	// bracket+slash pairs here precisely so suffixed calls like K1ABC/P
	// keep their suffix on the air. Without brackets exactly one call must
	// look nonstandard; unlike WSJT-X pack77_1 (which drops the prefix and
	// packs type 1) the full call is kept in c58, matching ft8_lib.
	bracketedFirst := isBracketedToken(fields[0])
	bracketedSecond := isBracketedToken(fields[1])
	var hashCallText, spelled string
	iflip := 0
	switch {
	case bracketedFirst && bracketedSecond:
		return out, false
	case bracketedFirst:
		hashCallText, spelled = fields[0], fields[1]
	case bracketedSecond:
		hashCallText, spelled = fields[1], fields[0]
		iflip = 1
	default:
		firstType4 := type4CallOK(fields[0], false)
		secondType4 := type4CallOK(fields[1], false)
		if firstType4 == secondType4 {
			return out, false
		}
		if firstType4 {
			hashCallText, spelled = fields[1], fields[0]
			iflip = 1
		} else {
			hashCallText, spelled = fields[0], fields[1]
		}
		// Without brackets the hashed side must be a standard call;
		// brackets assert the call is already known to receivers.
		if _, _, _, ok := pack28WithSuffix(hashCallText); !ok {
			return out, false
		}
	}

	if !type4SpelledCallOK(spelled) {
		return out, false
	}
	n58, ok := packBase38Call(spelled)
	if !ok {
		return out, false
	}
	hashCallText, ok = normalizeHashCallToken(hashCallText)
	if !ok {
		return out, false
	}

	writeBits(out[:], 0, 12, hashCall(hashCallText, 12))
	writeBits64(out[:], 12, 58, n58)
	writeBits(out[:], 70, 1, iflip)
	writeBits(out[:], 71, 2, nrpt)
	writeBits(out[:], 74, 3, 4)
	return out, true
}

func packBase38Call(call string) (uint64, bool) {
	call = strings.ToUpper(strings.TrimSpace(call))
	if call == "" || len(call) > 11 {
		return 0, false
	}
	// Type-4 c58 calls are right-justified in the 11-character base-38 field.
	call = strings.Repeat(" ", 11-len(call)) + call
	var n uint64
	for i := 0; i < 11; i++ {
		idx := strings.IndexByte(ft8Base38Alphabet, call[i])
		if idx < 0 {
			return 0, false
		}
		n = 38*n + uint64(idx)
	}
	return n, true
}

func packType4Call(call string, cq bool) (uint64, bool) {
	call = strings.ToUpper(strings.TrimSpace(call))
	if !type4CallOK(call, cq) {
		return 0, false
	}
	return packBase38Call(call)
}

func type4CallOK(call string, cq bool) bool {
	if len(call) < 3 || len(call) > 11 || isReservedCallToken(call) {
		return false
	}
	if cq && len(call) <= 4 {
		return false
	}
	if _, _, _, ok := pack28WithSuffix(call); ok {
		return false
	}
	if !base38TextOK(call) {
		return false
	}
	if cq {
		// WSJT-X pack77_4 applies no structural checks to a CQ'd
		// nonstandard call beyond its length, so "CQ YW18FIFA/P" and
		// "CQ MUSEUM" are valid.
		return true
	}
	if strings.Contains(call, "/") {
		return slashCallOK(call)
	}
	if len(call) <= 4 {
		return false
	}
	return hasLetter(call) && hasDigit(call)
}

// type4SpelledCallOK validates the call spelled out in c58 of a hashed-pair
// type-4 message. WSJT-X pack77_4 spells it with no structural checks, so
// only base-38 encodability and hash-table sanity apply. A bare standard
// call is still rejected: WSJT-X sends those as type 1.
func type4SpelledCallOK(call string) bool {
	if len(call) < 3 || len(call) > 11 || isReservedCallToken(call) || !base38TextOK(call) {
		return false
	}
	if _, ok := pack28(call); ok {
		return false
	}
	return true
}

func isBracketedToken(s string) bool {
	return len(s) >= 2 && s[0] == '<' && s[len(s)-1] == '>'
}

func slashCallOK(call string) bool {
	parts := strings.Split(call, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	if callOK(parts[0]) && callAffixOK(parts[1]) {
		return true
	}
	return callOK(parts[1]) && callAffixOK(parts[0])
}

// callAffixOK mirrors WSJT-X chkcall, which allows either side of a
// compound call up to 6 characters with no reserved-token screening
// ("R/KL7ABC" carries a legitimate Russian temporary prefix).
func callAffixOK(s string) bool {
	if len(s) < 1 || len(s) > 6 {
		return false
	}
	for i := range s {
		if !isLetter(s[i]) && !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func base38TextOK(s string) bool {
	if s == "" || len(s) > 11 {
		return false
	}
	for i := range s {
		if strings.IndexByte(ft8Base38Alphabet, s[i]) < 0 {
			return false
		}
	}
	return true
}

func isReservedCallToken(s string) bool {
	switch s {
	case "CQ", "DE", "QRZ", "R", "RRR", "RR73", "73":
		return true
	default:
		return false
	}
}

func hasLetter(s string) bool {
	for i := range s {
		if isLetter(s[i]) {
			return true
		}
	}
	return false
}

func hasDigit(s string) bool {
	for i := range s {
		if isDigit(s[i]) {
			return true
		}
	}
	return false
}

func normalizeHashCallToken(call string) (string, bool) {
	call = strings.ToUpper(strings.TrimSpace(call))
	if isBracketedToken(call) {
		call = strings.TrimSpace(call[1 : len(call)-1])
	}
	// Mirrors WSJT-X save_hash_call, which stores any base-38 token of at
	// least 3 characters verbatim without structural validation.
	if len(call) < 3 || call == "..." || strings.Contains(call, "<") || strings.Contains(call, ">") ||
		isReservedCallToken(call) || !base38TextOK(call) {
		return "", false
	}
	return call, true
}

func pack28WithSuffix(call string) (int, int, standardCallSuffix, bool) {
	call = strings.TrimSpace(strings.ToUpper(call))
	suffix := standardCallNoSuffix
	flag := 0
	switch {
	case strings.HasSuffix(call, "/P"):
		suffix = standardCallPortable
		flag = 1
		call = strings.TrimSuffix(call, "/P")
		if !callOK(call) {
			return 0, 0, standardCallNoSuffix, false
		}
	case strings.HasSuffix(call, "/R"):
		suffix = standardCallRover
		flag = 1
		call = strings.TrimSuffix(call, "/R")
		if !callOK(call) {
			return 0, 0, standardCallNoSuffix, false
		}
	}
	n28, ok := pack28(call)
	if !ok {
		return 0, 0, standardCallNoSuffix, false
	}
	return n28, flag, suffix, true
}

func pack28(call string) (int, bool) {
	call = strings.TrimSpace(strings.ToUpper(call))
	switch {
	case call == "DE":
		return 0, true
	case call == "QRZ":
		return 1, true
	case call == "CQ":
		return 2, true
	case strings.HasPrefix(call, "CQ_"):
		suffix := call[3:]
		if len(suffix) == 3 && allDigits(suffix) {
			var n int
			_, _ = fmt.Sscanf(suffix, "%d", &n)
			if n >= 0 && n <= 999 {
				return 3 + n, true
			}
		}
		if len(suffix) >= 1 && len(suffix) <= 4 && allLetters(suffix) {
			padded := strings.Repeat(" ", 4-len(suffix)) + suffix
			const c4 = " ABCDEFGHIJKLMNOPQRSTUVWXYZ"
			m := 0
			for i := 0; i < 4; i++ {
				j := strings.IndexByte(c4, padded[i])
				if j < 0 {
					return 0, false
				}
				m = 27*m + j
			}
			return 3 + 1000 + m, true
		}
		return 0, false
	}

	if !callOK(call) {
		return 0, false
	}
	n := len(call)
	area := -1
	for i := n - 1; i >= 1; i-- {
		if isDigit(call[i]) {
			area = i
			break
		}
	}
	if area != 1 && area != 2 {
		return 0, false
	}

	base := call
	if area == 1 {
		base = " " + call
	}
	if len(base) > 6 {
		return 0, false
	}
	base += strings.Repeat(" ", 6-len(base))

	const c1 = " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const c2 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const c3 = "0123456789"
	const c4 = " ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	i1 := strings.IndexByte(c1, base[0])
	i2 := strings.IndexByte(c2, base[1])
	i3 := strings.IndexByte(c3, base[2])
	i4 := strings.IndexByte(c4, base[3])
	i5 := strings.IndexByte(c4, base[4])
	i6 := strings.IndexByte(c4, base[5])
	if i1 < 0 || i2 < 0 || i3 < 0 || i4 < 0 || i5 < 0 || i6 < 0 {
		return 0, false
	}
	n28 := 36*10*27*27*27*i1 + 10*27*27*27*i2 + 27*27*27*i3 + 27*27*i4 + 27*i5 + i6
	return n28 + ft8NTokens + ft8Max22, true
}

func packReportToken(token string) (int, bool, bool) {
	switch token {
	case "RRR":
		return 2, false, true
	case "RR73":
		return 3, false, true
	case "73":
		return 4, false, true
	}
	response := false
	report := token
	if strings.HasPrefix(token, "R+") || strings.HasPrefix(token, "R-") {
		response = true
		report = token[1:]
	}
	if len(report) != 3 || report[0] != '+' && report[0] != '-' || !allDigits(report[1:]) {
		return 0, false, false
	}
	var snr int
	_, _ = fmt.Sscanf(report, "%d", &snr)
	if snr < -50 || snr > 49 {
		return 0, false, false
	}
	if snr >= -50 && snr <= -31 {
		snr += 101
	}
	return snr + 35, response, true
}

func encode17491(message77 [77]int8) [174]int8 {
	return ftx77.Encode17491(message77)
}

func crc14RemainderMessage77(message77 [77]int8) int {
	return ftx77.CRC14RemainderMessage77(message77)
}

func isGrid4(s string) bool {
	if len(s) != 4 {
		return false
	}
	return s[0] >= 'A' && s[0] <= 'R' && s[1] >= 'A' && s[1] <= 'R' &&
		isDigit(s[2]) && isDigit(s[3])
}

func grid4Index(grid string) (int, bool) {
	grid = strings.ToUpper(grid)
	if !isGrid4(grid) {
		return 0, false
	}
	return int(grid[0]-'A')*18*10*10 + int(grid[1]-'A')*10*10 + int(grid[2]-'0')*10 + int(grid[3]-'0'), true
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range s {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func allLetters(s string) bool {
	if s == "" {
		return false
	}
	for i := range s {
		if !isLetter(s[i]) {
			return false
		}
	}
	return true
}

func writeBits(bits []int8, start int, width int, value int) {
	writeBits64(bits, start, width, uint64(value))
}

func writeBits64(bits []int8, start int, width int, value uint64) {
	for i := 0; i < width; i++ {
		shift := uint(width - 1 - i)
		bits[start+i] = int8((value >> shift) & 1)
	}
}
