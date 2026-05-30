// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

func clampFFTBinRange(firstBin, lastBin, bins int) (int, int) {
	if firstBin < 0 {
		firstBin = 0
	}
	if lastBin >= bins {
		lastBin = bins - 1
	}
	if lastBin < firstBin {
		return 0, -1
	}
	return firstBin, lastBin
}
