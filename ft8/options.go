// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

const (
	ft8DefaultMaxAPCallHypotheses = 2
	ft8MaxAPCallHypotheses        = 8
	ft8MaxAPCallHints             = 200
)

// APCallHint describes one upstream-ranked callsign hint for a-priori decode.
//
// The decoder copies, normalizes, deduplicates, and caps hints, but it does not
// rank them by logbook policy. Callers should pass hints in priority order.
type APCallHint struct {
	// Call is a callsign encodable as a standard FT8 28-bit callsign token.
	Call string
	// Weight is optional caller policy metadata retained for future scoring.
	Weight float64
	// Source labels where the hint came from, such as "recent" or "worked".
	Source string
}

// DecoderOptions controls non-default decoder behavior.
//
// The zero value is strict mode and preserves the package defaults used by
// DecodeMessages and NewDecoder.
type DecoderOptions struct {
	// SyncMin overrides the candidate sync threshold when positive.
	SyncMin float64
	// MaxCandidates overrides the candidate cap when positive.
	MaxCandidates int
	// MinFreqHz overrides the lower search frequency when positive.
	MinFreqHz int
	// MaxFreqHz overrides the upper search frequency when positive.
	MaxFreqHz int
	// Blocks optionally adds alternate 3456-sample block counts to search.
	// The zero value searches the strict 50-block receive slot.
	Blocks []int
	// LLRWinsorFactor caps absolute LLR values to factor*medianAbs when positive.
	LLRWinsorFactor float64
	// HardSyncMin overrides the refined hard-sync gate when positive.
	HardSyncMin int
	// CostasMinWins rejects candidates with fewer Costas anchor wins when positive.
	CostasMinWins int
	// CostasMinGeo rejects candidates below the geometric Costas tone ratio when positive.
	CostasMinGeo float64
	// CostasMinBlock rejects candidates below the weakest-block Costas tone ratio when positive.
	CostasMinBlock float64
	// EnableOSD enables the experimental OSD-2/MRB fallback after BP misses.
	EnableOSD bool
	// EnableBroadAP enables additional experimental a-priori profiles beyond
	// the default CQ profile. It increases decode work and is intended for deep
	// searches.
	EnableBroadAP bool
	// APCallHints supplies upstream-ranked callsign hints for bounded BP-only
	// AP hypotheses. The decoder copies, normalizes, deduplicates, and caps
	// hints at 200 entries without applying logbook policy ranking.
	APCallHints []APCallHint
	// MaxAPCallHypotheses caps AP call-hint hypotheses tried per candidate when
	// positive. The default is 2 and the maximum accepted by checked APIs is 8.
	MaxAPCallHypotheses int
}

// DeepDecoderOptions returns an experimental configuration that searches
// alternate receive-window lengths, uses a lower sync threshold, and enables
// the OSD fallback. It is intended for maximum-recall scans, not strict oracle
// parity mode.
func DeepDecoderOptions() DecoderOptions {
	return DecoderOptions{
		SyncMin:       1.4,
		MaxCandidates: 300,
		HardSyncMin:   6,
		Blocks:        []int{50, 41, 43},
		EnableOSD:     true,
		EnableBroadAP: true,
	}
}

type decodeOptions struct {
	syncMin             float64
	maxCandidates       int
	minFreqHz           int
	maxFreqHz           int
	blocks              [4]int
	blockCount          int
	llrWinsorFactor     float64
	hardSyncMin         int
	costasMinWins       int
	costasMinGeo        float64
	costasMinBlock      float64
	enableOSD           bool
	enableBroadAP       bool
	apCallHints         []apCallHint
	maxAPCallHypotheses int
}

func normalizeDecoderOptions(o DecoderOptions) decodeOptions {
	out := decodeOptions{
		syncMin:       ft8DefaultSyncMin,
		maxCandidates: ft8DefaultMaxCand,
		minFreqHz:     ft8DefaultMinFreq,
		maxFreqHz:     ft8DefaultMaxFreq,
		blocks:        [4]int{50},
		blockCount:    1,
		hardSyncMin:   8,
	}
	if o.SyncMin > 0 {
		out.syncMin = o.SyncMin
	}
	if o.MaxCandidates > 0 {
		out.maxCandidates = o.MaxCandidates
	}
	if o.MinFreqHz > 0 {
		out.minFreqHz = o.MinFreqHz
	}
	if o.MaxFreqHz > 0 {
		out.maxFreqHz = o.MaxFreqHz
	}
	if len(o.Blocks) > 0 {
		out.blocks, out.blockCount = normalizeBlockCounts(o.Blocks)
	}
	if o.LLRWinsorFactor > 0 {
		out.llrWinsorFactor = o.LLRWinsorFactor
	}
	if o.HardSyncMin > 0 {
		out.hardSyncMin = o.HardSyncMin
	}
	if o.CostasMinWins > 0 {
		out.costasMinWins = o.CostasMinWins
	}
	if o.CostasMinGeo > 0 {
		out.costasMinGeo = o.CostasMinGeo
	}
	if o.CostasMinBlock > 0 {
		out.costasMinBlock = o.CostasMinBlock
	}
	out.enableOSD = o.EnableOSD
	out.enableBroadAP = o.EnableBroadAP
	out.apCallHints = normalizeAPCallHints(o.APCallHints)
	out.maxAPCallHypotheses = ft8DefaultMaxAPCallHypotheses
	if o.MaxAPCallHypotheses > 0 {
		out.maxAPCallHypotheses = o.MaxAPCallHypotheses
		if out.maxAPCallHypotheses > ft8MaxAPCallHypotheses {
			out.maxAPCallHypotheses = ft8MaxAPCallHypotheses
		}
	}
	return out
}

func normalizeBlockCounts(blocks []int) ([4]int, int) {
	var out [4]int
	n := 0
	for _, blocks := range blocks {
		if blocks <= 0 || blocks*3456 > 180000 || n == len(out) {
			continue
		}
		seen := false
		for i := 0; i < n; i++ {
			if out[i] == blocks {
				seen = true
				break
			}
		}
		if seen {
			continue
		}
		out[n] = blocks
		n++
	}
	if n == 0 {
		out[0] = 50
		n = 1
	}
	return out, n
}

func decoderOptionsEmpty(o DecoderOptions) bool {
	return o.SyncMin == 0 &&
		o.MaxCandidates == 0 &&
		o.MinFreqHz == 0 &&
		o.MaxFreqHz == 0 &&
		len(o.Blocks) == 0 &&
		o.LLRWinsorFactor == 0 &&
		o.HardSyncMin == 0 &&
		o.CostasMinWins == 0 &&
		o.CostasMinGeo == 0 &&
		o.CostasMinBlock == 0 &&
		!o.EnableOSD &&
		!o.EnableBroadAP &&
		len(o.APCallHints) == 0 &&
		o.MaxAPCallHypotheses == 0
}
