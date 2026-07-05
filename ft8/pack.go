// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"fmt"
	"strings"
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

	n58a, firstType4 := packType4Call(fields[0], false)
	n58b, secondType4 := packType4Call(fields[1], false)
	if firstType4 == secondType4 {
		return out, false
	}

	n58 := n58b
	hashCallText := fields[0]
	iflip := 0
	if firstType4 {
		n58 = n58a
		hashCallText = fields[1]
		iflip = 1
	}

	hashCallText, ok := normalizeHashCallToken(hashCallText)
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
	if strings.Contains(call, "/") {
		return slashCallOK(call)
	}
	if len(call) <= 4 {
		return false
	}
	return hasLetter(call) && hasDigit(call)
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

func callAffixOK(s string) bool {
	if len(s) < 1 || len(s) > 4 || isReservedCallToken(s) {
		return false
	}
	for i := range s {
		if !isLetter(s[i]) && !isDigit(s[i]) {
			return false
		}
	}
	return hasLetter(s) || hasDigit(s)
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
	if len(call) >= 2 && call[0] == '<' && call[len(call)-1] == '>' {
		call = strings.TrimSpace(call[1 : len(call)-1])
	}
	if len(call) < 3 || call == "..." || strings.Contains(call, "<") || strings.Contains(call, ">") ||
		isReservedCallToken(call) || !base38TextOK(call) {
		return "", false
	}
	if _, _, _, ok := pack28WithSuffix(call); ok {
		return call, true
	}
	if !type4CallOK(call, false) {
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
	var message91 [91]int8
	copy(message91[:77], message77[:])
	crc := crc14ForMessage(message77)
	for i := 0; i < 14; i++ {
		message91[77+i] = int8((crc >> uint(13-i)) & 1)
	}
	return encode17491NoCRC(message91)
}

func crc14ForMessage(message77 [77]int8) int {
	target := crc14RemainderMessage77(message77)
	return solveCRC14(target)
}

func crc14RemainderMessage77(message77 [77]int8) int {
	var state uint16
	for i := 0; i < 15; i++ {
		state = (state << 1) | uint16(message77[i]&1)
	}
	for i := 0; i <= 81; i++ {
		pos := i + 14
		var bit int8
		if pos < 77 {
			bit = message77[pos]
		}
		state = crc14Step(state, bit)
	}
	return int(state >> 1)
}

var crc14EncodeRows = initCRC14EncodeRows()

func initCRC14EncodeRows() [14]uint16 {
	var rows [14]uint16
	for col := 0; col < 14; col++ {
		var bits [96]int8
		bits[82+col] = 1
		rem := crc14Remainder(bits[:])
		for row := 0; row < 14; row++ {
			if rem&(1<<uint(13-row)) != 0 {
				rows[row] |= 1 << uint(13-col)
			}
		}
	}
	return rows
}

func solveCRC14(target int) int {
	rows := crc14EncodeRows
	var rhs [14]int
	for row := 0; row < 14; row++ {
		rhs[row] = (target >> uint(13-row)) & 1
	}

	for col := 0; col < 14; col++ {
		mask := uint16(1 << uint(13-col))
		pivot := -1
		for row := col; row < 14; row++ {
			if rows[row]&mask != 0 {
				pivot = row
				break
			}
		}
		if pivot < 0 {
			panic("crc14: singular generator submatrix")
		}
		if pivot != col {
			rows[col], rows[pivot] = rows[pivot], rows[col]
			rhs[col], rhs[pivot] = rhs[pivot], rhs[col]
		}
		for row := 0; row < 14; row++ {
			if row == col || rows[row]&mask == 0 {
				continue
			}
			rows[row] ^= rows[col]
			rhs[row] ^= rhs[col]
		}
	}

	crc := 0
	for row := 0; row < 14; row++ {
		crc |= rhs[row] << uint(13-row)
	}
	return crc
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
