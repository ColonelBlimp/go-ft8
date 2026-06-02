// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"strings"
)

const (
	apHintFieldCall1 = iota
	apHintFieldCall2
)

const (
	ft8APHintKnownBits     = 32
	ft8APHintMinMatches    = 24
	ft8APHintMinScoreRatio = 0.35
)

type apCallHint struct {
	call   string
	source string
	n28    int
	weight float64
}

type apHintSelection struct {
	score   float64
	hint    int
	field   int
	metric  int
	matches int
}

func copyAPCallHints(hints []APCallHint) []APCallHint {
	if len(hints) == 0 {
		return nil
	}
	out := make([]APCallHint, len(hints))
	copy(out, hints)
	return out
}

func normalizeAPCallHints(hints []APCallHint) []apCallHint {
	if len(hints) == 0 {
		return nil
	}
	out := make([]apCallHint, 0, min(len(hints), ft8MaxAPCallHints))
	seen := make(map[int]bool, min(len(hints), ft8MaxAPCallHints))
	for _, hint := range hints {
		call := strings.ToUpper(strings.TrimSpace(hint.Call))
		n28, ok := pack28(call)
		if !ok || seen[n28] {
			continue
		}
		source := strings.TrimSpace(hint.Source)
		if source == "" {
			source = "hint"
		}
		weight := hint.Weight
		if math.IsNaN(weight) || math.IsInf(weight, 0) {
			weight = 0
		}
		out = append(out, apCallHint{
			call:   call,
			source: source,
			n28:    n28,
			weight: weight,
		})
		seen[n28] = true
		if len(out) == ft8MaxAPCallHints {
			break
		}
	}
	return out
}

func selectAPCallHintHypotheses(metrics *softMetrics, hints []apCallHint, maxHypotheses int, diagnostics *DecodeDiagnostics, out *[ft8MaxAPCallHypotheses]apHintSelection) int {
	if len(hints) == 0 || maxHypotheses <= 0 {
		return 0
	}
	if maxHypotheses > ft8MaxAPCallHypotheses {
		maxHypotheses = ft8MaxAPCallHypotheses
	}
	scored := len(hints) * 2
	topLen := 0
	belowThreshold := 0
	for hintIndex, hint := range hints {
		for field := 0; field < 2; field++ {
			selection, ok := scoreAPCallHintHypothesis(metrics, hintIndex, hint.n28, field)
			if !ok {
				belowThreshold++
				continue
			}
			topLen = insertAPHintSelection(out[:], topLen, maxHypotheses, selection)
		}
	}
	if diagnostics != nil {
		diagnostics.recordAPHintScored(len(hints), scored, topLen, belowThreshold)
	}
	return topLen
}

func scoreAPCallHintHypothesis(metrics *softMetrics, hintIndex int, n28 int, field int) (apHintSelection, bool) {
	singleScore, singleMatches := scoreAPCallHintMetric(&metrics.Single, n28, field)
	tripleScore, tripleMatches := scoreAPCallHintMetric(&metrics.Triple, n28, field)
	selection := apHintSelection{
		score:   singleScore,
		hint:    hintIndex,
		field:   field,
		metric:  0,
		matches: singleMatches,
	}
	if tripleScore > singleScore || tripleScore == singleScore && tripleMatches > singleMatches {
		selection.score = tripleScore
		selection.metric = 1
		selection.matches = tripleMatches
	}
	if selection.matches < ft8APHintMinMatches || selection.score < ft8APHintMinScoreRatio {
		return apHintSelection{}, false
	}
	return selection, true
}

func insertAPHintSelection(top []apHintSelection, topLen int, maxLen int, selection apHintSelection) int {
	if topLen == 0 {
		top[0] = selection
		return 1
	}
	if topLen == maxLen && !apHintSelectionBetter(selection, top[topLen-1]) {
		return topLen
	}
	pos := topLen
	if pos >= maxLen {
		pos = maxLen - 1
	} else {
		topLen++
	}
	for pos > 0 && apHintSelectionBetter(selection, top[pos-1]) {
		top[pos] = top[pos-1]
		pos--
	}
	top[pos] = selection
	return topLen
}

func apHintSelectionBetter(a, b apHintSelection) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	if a.matches != b.matches {
		return a.matches > b.matches
	}
	if a.hint != b.hint {
		return a.hint < b.hint
	}
	return a.field < b.field
}

func scoreAPCallHintMetric(metric *[174]float64, n28 int, field int) (float64, int) {
	sum := 0.0
	sumAbs := 0.0
	matches := 0
	start := 0
	flag := 28
	if field == apHintFieldCall2 {
		start = 29
		flag = 57
	}
	for i := 0; i < 28; i++ {
		shift := uint(27 - i)
		bit := int8((n28 >> shift) & 1)
		sum, sumAbs, matches = scoreAPKnownBit(metric[start+i], bit, sum, sumAbs, matches)
	}
	sum, sumAbs, matches = scoreAPKnownBit(metric[flag], 0, sum, sumAbs, matches)
	sum, sumAbs, matches = scoreAPKnownBit(metric[74], 0, sum, sumAbs, matches)
	sum, sumAbs, matches = scoreAPKnownBit(metric[75], 0, sum, sumAbs, matches)
	sum, sumAbs, matches = scoreAPKnownBit(metric[76], 1, sum, sumAbs, matches)
	if sumAbs == 0 {
		return 0, matches
	}
	return sum / sumAbs, matches
}

func scoreAPKnownBit(value float64, bit int8, sum float64, sumAbs float64, matches int) (float64, float64, int) {
	sign := -1.0
	if bit == 1 {
		sign = 1
	}
	contribution := sign * value
	if contribution >= 0 {
		matches++
	}
	return sum + contribution, sumAbs + math.Abs(value), matches
}

func apHintProfile(hint apCallHint, field int) apProfile {
	profile := initStandardTypeAPProfile("hint-call1", 1)
	if field == apHintFieldCall2 {
		profile = initStandardTypeAPProfile("hint-call2", 1)
		setAPBits(&profile, 29, 28, hint.n28)
		setAPBits(&profile, 57, 1, 0)
	} else {
		setAPBits(&profile, 0, 28, hint.n28)
		setAPBits(&profile, 28, 1, 0)
	}
	profile.source = hint.source
	return profile
}

func apHintMetric(metrics *softMetrics, selection apHintSelection) *[174]float64 {
	if selection.metric == 1 {
		return &metrics.Triple
	}
	return &metrics.Single
}
