// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"strconv"
	"strings"
)

// ARRLFieldDaySection is a canonical ARRL/RAC Field Day section code.
type ARRLFieldDaySection string

// String returns the canonical section code.
func (section ARRLFieldDaySection) String() string {
	return string(section)
}

const (
	ARRLFieldDaySectionAB  ARRLFieldDaySection = "AB"
	ARRLFieldDaySectionAK  ARRLFieldDaySection = "AK"
	ARRLFieldDaySectionAL  ARRLFieldDaySection = "AL"
	ARRLFieldDaySectionAR  ARRLFieldDaySection = "AR"
	ARRLFieldDaySectionAZ  ARRLFieldDaySection = "AZ"
	ARRLFieldDaySectionBC  ARRLFieldDaySection = "BC"
	ARRLFieldDaySectionCO  ARRLFieldDaySection = "CO"
	ARRLFieldDaySectionCT  ARRLFieldDaySection = "CT"
	ARRLFieldDaySectionDE  ARRLFieldDaySection = "DE"
	ARRLFieldDaySectionEB  ARRLFieldDaySection = "EB"
	ARRLFieldDaySectionEMA ARRLFieldDaySection = "EMA"
	ARRLFieldDaySectionENY ARRLFieldDaySection = "ENY"
	ARRLFieldDaySectionEPA ARRLFieldDaySection = "EPA"
	ARRLFieldDaySectionEWA ARRLFieldDaySection = "EWA"
	ARRLFieldDaySectionGA  ARRLFieldDaySection = "GA"
	ARRLFieldDaySectionGH  ARRLFieldDaySection = "GH"
	ARRLFieldDaySectionIA  ARRLFieldDaySection = "IA"
	ARRLFieldDaySectionID  ARRLFieldDaySection = "ID"
	ARRLFieldDaySectionIL  ARRLFieldDaySection = "IL"
	ARRLFieldDaySectionIN  ARRLFieldDaySection = "IN"
	ARRLFieldDaySectionKS  ARRLFieldDaySection = "KS"
	ARRLFieldDaySectionKY  ARRLFieldDaySection = "KY"
	ARRLFieldDaySectionLA  ARRLFieldDaySection = "LA"
	ARRLFieldDaySectionLAX ARRLFieldDaySection = "LAX"
	ARRLFieldDaySectionNS  ARRLFieldDaySection = "NS"
	ARRLFieldDaySectionMB  ARRLFieldDaySection = "MB"
	ARRLFieldDaySectionMDC ARRLFieldDaySection = "MDC"
	ARRLFieldDaySectionME  ARRLFieldDaySection = "ME"
	ARRLFieldDaySectionMI  ARRLFieldDaySection = "MI"
	ARRLFieldDaySectionMN  ARRLFieldDaySection = "MN"
	ARRLFieldDaySectionMO  ARRLFieldDaySection = "MO"
	ARRLFieldDaySectionMS  ARRLFieldDaySection = "MS"
	ARRLFieldDaySectionMT  ARRLFieldDaySection = "MT"
	ARRLFieldDaySectionNC  ARRLFieldDaySection = "NC"
	ARRLFieldDaySectionND  ARRLFieldDaySection = "ND"
	ARRLFieldDaySectionNE  ARRLFieldDaySection = "NE"
	ARRLFieldDaySectionNFL ARRLFieldDaySection = "NFL"
	ARRLFieldDaySectionNH  ARRLFieldDaySection = "NH"
	ARRLFieldDaySectionNL  ARRLFieldDaySection = "NL"
	ARRLFieldDaySectionNLI ARRLFieldDaySection = "NLI"
	ARRLFieldDaySectionNM  ARRLFieldDaySection = "NM"
	ARRLFieldDaySectionNNJ ARRLFieldDaySection = "NNJ"
	ARRLFieldDaySectionNNY ARRLFieldDaySection = "NNY"
	ARRLFieldDaySectionTER ARRLFieldDaySection = "TER"
	ARRLFieldDaySectionNTX ARRLFieldDaySection = "NTX"
	ARRLFieldDaySectionNV  ARRLFieldDaySection = "NV"
	ARRLFieldDaySectionOH  ARRLFieldDaySection = "OH"
	ARRLFieldDaySectionOK  ARRLFieldDaySection = "OK"
	ARRLFieldDaySectionONE ARRLFieldDaySection = "ONE"
	ARRLFieldDaySectionONN ARRLFieldDaySection = "ONN"
	ARRLFieldDaySectionONS ARRLFieldDaySection = "ONS"
	ARRLFieldDaySectionOR  ARRLFieldDaySection = "OR"
	ARRLFieldDaySectionORG ARRLFieldDaySection = "ORG"
	ARRLFieldDaySectionPAC ARRLFieldDaySection = "PAC"
	ARRLFieldDaySectionPR  ARRLFieldDaySection = "PR"
	ARRLFieldDaySectionQC  ARRLFieldDaySection = "QC"
	ARRLFieldDaySectionRI  ARRLFieldDaySection = "RI"
	ARRLFieldDaySectionSB  ARRLFieldDaySection = "SB"
	ARRLFieldDaySectionSC  ARRLFieldDaySection = "SC"
	ARRLFieldDaySectionSCV ARRLFieldDaySection = "SCV"
	ARRLFieldDaySectionSD  ARRLFieldDaySection = "SD"
	ARRLFieldDaySectionSDG ARRLFieldDaySection = "SDG"
	ARRLFieldDaySectionSF  ARRLFieldDaySection = "SF"
	ARRLFieldDaySectionSFL ARRLFieldDaySection = "SFL"
	ARRLFieldDaySectionSJV ARRLFieldDaySection = "SJV"
	ARRLFieldDaySectionSK  ARRLFieldDaySection = "SK"
	ARRLFieldDaySectionSNJ ARRLFieldDaySection = "SNJ"
	ARRLFieldDaySectionSTX ARRLFieldDaySection = "STX"
	ARRLFieldDaySectionSV  ARRLFieldDaySection = "SV"
	ARRLFieldDaySectionTN  ARRLFieldDaySection = "TN"
	ARRLFieldDaySectionUT  ARRLFieldDaySection = "UT"
	ARRLFieldDaySectionVA  ARRLFieldDaySection = "VA"
	ARRLFieldDaySectionVI  ARRLFieldDaySection = "VI"
	ARRLFieldDaySectionVT  ARRLFieldDaySection = "VT"
	ARRLFieldDaySectionWCF ARRLFieldDaySection = "WCF"
	ARRLFieldDaySectionWI  ARRLFieldDaySection = "WI"
	ARRLFieldDaySectionWMA ARRLFieldDaySection = "WMA"
	ARRLFieldDaySectionWNY ARRLFieldDaySection = "WNY"
	ARRLFieldDaySectionWPA ARRLFieldDaySection = "WPA"
	ARRLFieldDaySectionWTX ARRLFieldDaySection = "WTX"
	ARRLFieldDaySectionWV  ARRLFieldDaySection = "WV"
	ARRLFieldDaySectionWWA ARRLFieldDaySection = "WWA"
	ARRLFieldDaySectionWY  ARRLFieldDaySection = "WY"
	ARRLFieldDaySectionDX  ARRLFieldDaySection = "DX"
	ARRLFieldDaySectionPE  ARRLFieldDaySection = "PE"
	ARRLFieldDaySectionNB  ARRLFieldDaySection = "NB"
)

var arrlFieldDaySections = []ARRLFieldDaySection{
	ARRLFieldDaySectionAB, ARRLFieldDaySectionAK, ARRLFieldDaySectionAL, ARRLFieldDaySectionAR,
	ARRLFieldDaySectionAZ, ARRLFieldDaySectionBC, ARRLFieldDaySectionCO, ARRLFieldDaySectionCT,
	ARRLFieldDaySectionDE, ARRLFieldDaySectionEB, ARRLFieldDaySectionEMA, ARRLFieldDaySectionENY,
	ARRLFieldDaySectionEPA, ARRLFieldDaySectionEWA, ARRLFieldDaySectionGA, ARRLFieldDaySectionGH,
	ARRLFieldDaySectionIA, ARRLFieldDaySectionID, ARRLFieldDaySectionIL, ARRLFieldDaySectionIN,
	ARRLFieldDaySectionKS, ARRLFieldDaySectionKY, ARRLFieldDaySectionLA, ARRLFieldDaySectionLAX,
	ARRLFieldDaySectionNS, ARRLFieldDaySectionMB, ARRLFieldDaySectionMDC, ARRLFieldDaySectionME,
	ARRLFieldDaySectionMI, ARRLFieldDaySectionMN, ARRLFieldDaySectionMO, ARRLFieldDaySectionMS,
	ARRLFieldDaySectionMT, ARRLFieldDaySectionNC, ARRLFieldDaySectionND, ARRLFieldDaySectionNE,
	ARRLFieldDaySectionNFL, ARRLFieldDaySectionNH, ARRLFieldDaySectionNL, ARRLFieldDaySectionNLI,
	ARRLFieldDaySectionNM, ARRLFieldDaySectionNNJ, ARRLFieldDaySectionNNY, ARRLFieldDaySectionTER,
	ARRLFieldDaySectionNTX, ARRLFieldDaySectionNV, ARRLFieldDaySectionOH, ARRLFieldDaySectionOK,
	ARRLFieldDaySectionONE, ARRLFieldDaySectionONN, ARRLFieldDaySectionONS, ARRLFieldDaySectionOR,
	ARRLFieldDaySectionORG, ARRLFieldDaySectionPAC, ARRLFieldDaySectionPR, ARRLFieldDaySectionQC,
	ARRLFieldDaySectionRI, ARRLFieldDaySectionSB, ARRLFieldDaySectionSC, ARRLFieldDaySectionSCV,
	ARRLFieldDaySectionSD, ARRLFieldDaySectionSDG, ARRLFieldDaySectionSF, ARRLFieldDaySectionSFL,
	ARRLFieldDaySectionSJV, ARRLFieldDaySectionSK, ARRLFieldDaySectionSNJ, ARRLFieldDaySectionSTX,
	ARRLFieldDaySectionSV, ARRLFieldDaySectionTN, ARRLFieldDaySectionUT, ARRLFieldDaySectionVA,
	ARRLFieldDaySectionVI, ARRLFieldDaySectionVT, ARRLFieldDaySectionWCF, ARRLFieldDaySectionWI,
	ARRLFieldDaySectionWMA, ARRLFieldDaySectionWNY, ARRLFieldDaySectionWPA, ARRLFieldDaySectionWTX,
	ARRLFieldDaySectionWV, ARRLFieldDaySectionWWA, ARRLFieldDaySectionWY, ARRLFieldDaySectionDX,
	ARRLFieldDaySectionPE, ARRLFieldDaySectionNB,
}

// ARRLFieldDaySections returns the canonical ARRL/RAC Field Day section codes.
func ARRLFieldDaySections() []ARRLFieldDaySection {
	out := make([]ARRLFieldDaySection, len(arrlFieldDaySections))
	copy(out, arrlFieldDaySections)
	return out
}

// ParseARRLFieldDaySection normalizes and validates an ARRL/RAC Field Day
// section code.
func ParseARRLFieldDaySection(section string) (ARRLFieldDaySection, bool) {
	normalized := ARRLFieldDaySection(strings.ToUpper(strings.TrimSpace(section)))
	for _, candidate := range arrlFieldDaySections {
		if normalized == candidate {
			return candidate, true
		}
	}
	return "", false
}

// ValidARRLFieldDaySection reports whether section is a supported ARRL/RAC
// Field Day section code.
func ValidARRLFieldDaySection(section string) bool {
	_, ok := ParseARRLFieldDaySection(section)
	return ok
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
	section := arrlFieldDaySections[isec-1].String()
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
	normalized, ok := ParseARRLFieldDaySection(section)
	if !ok {
		return 0, false
	}
	for i, candidate := range arrlFieldDaySections {
		if normalized == candidate {
			return i + 1, true
		}
	}
	return 0, false
}
