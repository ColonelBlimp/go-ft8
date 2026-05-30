// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"fmt"
	"strings"
)

func pack77StandardMessage(msg string) ([77]int8, bool) {
	fields := strings.Fields(strings.ToUpper(msg))
	if len(fields) >= 3 && fields[0] == "CQ" {
		if _, ok := pack28(fields[2]); ok {
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

	n28a, ok := pack28(fields[0])
	if !ok {
		return out, false
	}
	n28b, ok := pack28(fields[1])
	if !ok {
		return out, false
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
	writeBits(out[:], 28, 1, 0)
	writeBits(out[:], 29, 28, n28b)
	writeBits(out[:], 57, 1, 0)
	writeBits(out[:], 58, 1, ir)
	writeBits(out[:], 59, 15, igrid4)
	writeBits(out[:], 74, 3, 1)
	return out, true
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
	for i := 0; i < width; i++ {
		shift := uint(width - 1 - i)
		bits[start+i] = int8((value >> shift) & 1)
	}
}
