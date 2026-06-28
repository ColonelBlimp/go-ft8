// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"strconv"
	"strings"
)

var arrlFieldDaySections = []string{
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

func pack77ARRLFieldDayMessage(fields []string) ([77]int8, bool) {
	var out [77]int8
	if len(fields) != 4 && len(fields) != 5 {
		return out, false
	}
	if len(fields) == 5 && fields[2] != "R" {
		return out, false
	}
	if !callOK(fields[0]) || !callOK(fields[1]) {
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

	exchange := fields[len(fields)-2]
	ntx, nclass, ok := parseARRLFieldDayExchange(exchange)
	if !ok {
		return out, false
	}
	isec, ok := arrlFieldDaySectionIndex(fields[len(fields)-1])
	if !ok {
		return out, false
	}

	ir := 0
	if len(fields) == 5 {
		ir = 1
	}
	n3 := 3
	intx := ntx - 1
	if intx >= 16 {
		n3 = 4
		intx = ntx - 17
	}

	writeBits(out[:], 0, 28, n28a)
	writeBits(out[:], 28, 28, n28b)
	writeBits(out[:], 56, 1, ir)
	writeBits(out[:], 57, 4, intx)
	writeBits(out[:], 61, 3, nclass)
	writeBits(out[:], 64, 7, isec)
	writeBits(out[:], 71, 3, n3)
	writeBits(out[:], 74, 3, 0)
	return out, true
}

func unpackARRLFieldDay(bits []int8, n3 int, hashes *hashTable) (string, bool) {
	n28a := readBits(bits, 0, 28)
	n28b := readBits(bits, 28, 28)
	ir := readBits(bits, 56, 1)
	intx := readBits(bits, 57, 4)
	nclass := readBits(bits, 61, 3)
	isec := readBits(bits, 64, 7)

	if isec < 1 || isec > len(arrlFieldDaySections) {
		return "", false
	}
	call1, ok1 := unpack28WithHashes(n28a, hashes)
	call2, ok2 := unpack28WithHashes(n28b, hashes)
	if !ok1 || !ok2 || n28a <= 2 || n28b <= 2 {
		return "", false
	}

	ntx := intx + 1
	if n3 == 4 {
		ntx += 16
	}
	exchange := strconv.Itoa(ntx) + string(byte('A'+nclass))
	section := arrlFieldDaySections[isec-1]
	if ir == 1 {
		return call1 + " " + call2 + " R " + exchange + " " + section, true
	}
	return call1 + " " + call2 + " " + exchange + " " + section, true
}

func parseARRLFieldDayExchange(token string) (int, int, bool) {
	token = strings.ToUpper(strings.TrimSpace(token))
	if len(token) < 2 || len(token) > 3 {
		return 0, 0, false
	}
	class := token[len(token)-1]
	if class < 'A' || class > 'H' {
		return 0, 0, false
	}
	number := token[:len(token)-1]
	if !allDigits(number) {
		return 0, 0, false
	}
	ntx, err := strconv.Atoi(number)
	if err != nil || ntx < 1 || ntx > 32 {
		return 0, 0, false
	}
	return ntx, int(class - 'A'), true
}

func arrlFieldDaySectionIndex(section string) (int, bool) {
	section = strings.ToUpper(strings.TrimSpace(section))
	if len(section) < 2 || len(section) > 3 {
		return 0, false
	}
	for i, candidate := range arrlFieldDaySections {
		if section == candidate {
			return i + 1, true
		}
	}
	return 0, false
}
