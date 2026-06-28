// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"sort"
)

type candidate struct {
	FreqHz float64
	DTSec  float64
	Sync   float64
}

func findCandidates(dd []float32, minFreqHz, maxFreqHz int, syncMin float64, qsoFreqHz int, maxCandidates int) []candidate {
	const (
		maxPreCand = 1000
		maxLag     = 62
		narrowLag  = 13
	)

	df := float64(wantSampleRate) / float64(ft8NFFT1)
	tstep := float64(ft8Step) / float64(wantSampleRate)
	stepsPerSymbol := ft8SamplesPerSymbol / ft8Step
	freqOversample := ft8NFFT1 / ft8SamplesPerSymbol
	startStep := int(0.5 / tstep)

	firstBin := max(1, nint(float64(minFreqHz)/df))
	lastBin := nint(float64(maxFreqHz) / df)
	if lastBin+freqOversample*6 > ft8NBins {
		lastBin = ft8NBins - freqOversample*6
	}
	if firstBin > lastBin {
		return nil
	}
	if !hasNonZeroSample(dd) {
		return nil
	}
	toneLastBin := lastBin + freqOversample*6

	scratch := ft8SpectraScratchPool.Get().(*ft8SpectraScratch)
	spec := scratch.computeRange(dd, firstBin, toneLastBin)
	defer ft8SpectraScratchPool.Put(scratch)

	width := lastBin - firstBin + 1
	scratch.ensureFinder(lastBin+1, width, maxPreCand)
	jpeak := scratch.jpeak[:lastBin+1]
	jpeakWide := scratch.jpeakWide[:lastBin+1]
	red := scratch.red[:lastBin+1]
	redWide := scratch.redWide[:lastBin+1]
	toneSumStride := ft8NSymbolSpectra + 1
	toneSum := scratch.toneSum[:width*toneSumStride]
	for i := firstBin; i <= lastBin; i++ {
		row := toneSum[(i-firstBin)*toneSumStride:]
		for m := 1; m <= ft8NSymbolSpectra; m++ {
			var sum float64
			for k := 0; k <= 6; k++ {
				sum += spec[i+freqOversample*k][m]
			}
			row[m] = sum
		}
	}

	for i := firstBin; i <= lastBin; i++ {
		row := toneSum[(i-firstBin)*toneSumStride:]
		bestNarrowLag := -narrowLag
		bestWideLag := -maxLag
		var bestNarrow, bestWide float64
		for lag := -maxLag; lag <= maxLag; lag++ {
			var sigA, sigB, sigC float64
			var allA, allB, allC float64
			for n := 0; n < 7; n++ {
				m := lag + startStep + stepsPerSymbol*n
				bin := i + freqOversample*ft8Costas[n]
				if m >= 1 && m <= ft8NSymbolSpectra {
					sigA += spec[bin][m]
					allA += row[m]
				}

				mb := m + stepsPerSymbol*36
				sigB += spec[bin][mb]
				allB += row[mb]

				mc := m + stepsPerSymbol*72
				if mc <= ft8NSymbolSpectra {
					sigC += spec[bin][mc]
					allC += row[mc]
				}
			}

			syncABC := syncRatio(sigA+sigB+sigC, allA+allB+allC)
			syncBC := syncRatio(sigB+sigC, allB+allC)
			sync := math.Max(syncABC, syncBC)
			if lag == -maxLag || sync > bestWide {
				bestWideLag = lag
				bestWide = sync
			}
			if lag >= -narrowLag && lag <= narrowLag && (lag == -narrowLag || sync > bestNarrow) {
				bestNarrowLag = lag
				bestNarrow = sync
			}
		}
		jpeak[i], red[i] = bestNarrowLag, bestNarrow
		jpeakWide[i], redWide[i] = bestWideLag, bestWide
	}

	redOrder := sortedBinOrder(scratch.redOrder[:width], red, firstBin, lastBin)
	redWideOrder := sortedBinOrder(scratch.redWideOrder[:width], redWide, firstBin, lastBin)
	percentile := nint(0.40 * float64(width))
	if percentile < 1 {
		return nil
	}

	base := red[redOrder[percentile-1]]
	if base <= 0 || math.IsNaN(base) {
		base = 1
	}
	baseWide := redWide[redWideOrder[percentile-1]]
	if baseWide <= 0 || math.IsNaN(baseWide) {
		baseWide = 1
	}
	for i := firstBin; i <= lastBin; i++ {
		red[i] /= base
		redWide[i] /= baseWide
	}

	pre := scratch.pre[:0]
	for idx := width - 1; idx >= 0 && len(pre) < maxPreCand; idx-- {
		bin := redOrder[idx]
		if red[bin] >= syncMin && !math.IsNaN(red[bin]) {
			pre = append(pre, candidate{
				FreqHz: float64(bin) * df,
				DTSec:  (float64(jpeak[bin]) - 0.5) * tstep,
				Sync:   red[bin],
			})
		}
		if jpeakWide[bin] == jpeak[bin] || len(pre) >= maxPreCand {
			continue
		}
		if redWide[bin] >= syncMin && !math.IsNaN(redWide[bin]) {
			pre = append(pre, candidate{
				FreqHz: float64(bin) * df,
				DTSec:  (float64(jpeakWide[bin]) - 0.5) * tstep,
				Sync:   redWide[bin],
			})
		}
	}

	for i := range pre {
		for j := 0; j < i; j++ {
			fdiff := pre[i].FreqHz - pre[j].FreqHz
			tdiff := math.Abs(pre[i].DTSec - pre[j].DTSec)
			if math.Abs(fdiff) < 4.0 && tdiff < 0.04 {
				if pre[i].Sync >= pre[j].Sync {
					pre[j].Sync = 0
				} else {
					pre[i].Sync = 0
				}
			}
		}
	}

	order := sortedCandidateOrder(scratch.candOrder[:len(pre)], pre)
	out := make([]candidate, 0, min(maxCandidates, len(pre)))
	for i := range pre {
		if math.Abs(pre[i].FreqHz-float64(qsoFreqHz)) <= 10 && pre[i].Sync >= syncMin {
			out = append(out, pre[i])
			pre[i].Sync = 0
			if len(out) >= maxCandidates {
				return out
			}
		}
	}
	for idx := len(order) - 1; idx >= 0; idx-- {
		c := pre[order[idx]]
		if c.Sync >= syncMin {
			c.FreqHz = math.Abs(c.FreqHz)
			out = append(out, c)
			if len(out) >= maxCandidates {
				break
			}
		}
	}
	return out
}

func (s *ft8SpectraScratch) ensureFinder(binLen, width, maxPreCand int) {
	if cap(s.jpeak) < binLen {
		s.jpeak = make([]int, binLen)
		s.jpeakWide = make([]int, binLen)
		s.red = make([]float64, binLen)
		s.redWide = make([]float64, binLen)
	}
	if cap(s.redOrder) < width {
		s.redOrder = make([]int, width)
		s.redWideOrder = make([]int, width)
	}
	toneSumLen := width * (ft8NSymbolSpectra + 1)
	if cap(s.toneSum) < toneSumLen {
		s.toneSum = make([]float64, toneSumLen)
	}
	preCap := min(maxPreCand, 2*width)
	if cap(s.pre) < preCap {
		s.pre = make([]candidate, 0, preCap)
	}
	if cap(s.candOrder) < preCap {
		s.candOrder = make([]int, preCap)
	}
}

func syncRatio(signal, allTones float64) float64 {
	noise := (allTones - signal) / 6.0
	if noise <= 0 {
		return 0
	}
	return signal / noise
}

func hasNonZeroSample(samples []float32) bool {
	for _, sample := range samples {
		if sample != 0 {
			return true
		}
	}
	return false
}

func sortedBinOrder(order []int, values []float64, first, last int) []int {
	for i := range order {
		order[i] = first + i
	}
	sort.Slice(order, func(i, j int) bool {
		return values[order[i]] < values[order[j]]
	})
	return order
}

func sortedCandidateOrder(order []int, values []candidate) []int {
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		return values[order[i]].Sync < values[order[j]].Sync
	})
	return order
}
