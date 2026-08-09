// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "fmt"

var arrlRTTYNamedMultipliers = [...]string{
	"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "FL", "GA",
	"HI", "ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD",
	"MA", "MI", "MN", "MS", "MO", "MT", "NE", "NV", "NH", "NJ",
	"NM", "NY", "NC", "ND", "OH", "OK", "OR", "PA", "RI", "SC",
	"SD", "TN", "TX", "UT", "VT", "VA", "WA", "WV", "WI", "WY",
	"NB", "NS", "QC", "ON", "MB", "SK", "AB", "BC", "NWT", "NF",
	"LB", "NU", "YT", "PEI", "DC", "DR", "FR", "GD", "GR", "OV",
	"ZH", "ZL",
}

func arrlRTTYMultiplier(index int) (string, bool) {
	if index >= 1 && index <= len(arrlRTTYNamedMultipliers) {
		return arrlRTTYNamedMultipliers[index-1], true
	}
	x := index - len(arrlRTTYNamedMultipliers)
	if x >= 1 && x <= 99 {
		return fmt.Sprintf("X%02d", x), true
	}
	return "", false
}
