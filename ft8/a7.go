// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"strings"
)

// Decoder retains hash and A7 hint state across adjacent FT8 receive slots for
// one receiver stream.
//
// Decoder instances are not safe for concurrent use by multiple goroutines.
type Decoder struct {
	seq        int
	history    [2][]a7Hint
	hashes     hashTable
	options    decodeOptions
	rawOptions DecoderOptions
}

// NewDecoder returns a stateful FT8 decoder for one receiver stream using
// strict-mode default options.
func NewDecoder() *Decoder {
	return NewDecoderWithOptions(DecoderOptions{})
}

// NewDecoderWithOptions returns a stateful FT8 decoder using the supplied
// options.
//
// The zero-value options preserve strict-mode behavior. Options are normalized
// for permissive decode methods; DecodeMessagesChecked validates the original
// options supplied here before advancing decoder state.
func NewDecoderWithOptions(options DecoderOptions) *Decoder {
	return &Decoder{
		options:    normalizeDecoderOptions(options),
		rawOptions: options,
	}
}

// DecodeMessages decodes one 15-second FT8 slot from 12 kHz mono signed-16-bit
// PCM samples, using state retained from prior calls for hash and A7 hints.
//
// This is the permissive stateful API. Use DecodeMessagesWithReport for
// diagnostics or DecodeMessagesChecked when caller input and options should be
// validated before decoder state advances.
func (d *Decoder) DecodeMessages(iwave []int16) []DecodedMessage {
	// FT8 alternates transmit periods, so A7 hints are keyed by parity and come
	// from the previous slot with the same even/odd cadence.
	hints := d.history[d.seq]
	out := decodeMessagesCore(iwave, hints, &d.hashes, d.options)
	d.history[d.seq] = collectA7Hints(out)
	d.seq ^= 1
	return out
}

// DecodeMessagesWithReport decodes one slot using this decoder's retained
// hash/history state and returns aggregate diagnostics.
//
// This method is permissive and advances decoder state just like DecodeMessages.
// Use DecodeMessagesChecked to reject invalid input or options before state is
// updated.
func (d *Decoder) DecodeMessagesWithReport(iwave []int16) DecodeReport {
	// FT8 alternates transmit periods, so A7 hints are keyed by parity and come
	// from the previous slot with the same even/odd cadence.
	hints := d.history[d.seq]
	report := decodeMessagesReportCore(iwave, hints, &d.hashes, d.options)
	d.history[d.seq] = collectA7Hints(report.Messages)
	d.seq ^= 1
	return report
}

// DecodeMessagesChecked validates input and decoder options, then decodes one
// slot using this decoder's retained hash/history state.
//
// Invalid input or options return an error and do not advance decoder state. A
// valid slot with no decoded messages returns an empty report and a nil error.
func (d *Decoder) DecodeMessagesChecked(iwave []int16) (DecodeReport, error) {
	if err := validateDecodeRequest(iwave, d.rawOptions); err != nil {
		return DecodeReport{Diagnostics: newDecodeDiagnostics(iwave)}, err
	}
	return d.DecodeMessagesWithReport(iwave), nil
}

// DecodeStructured decodes one slot using this decoder's configured mode and
// returns mode-labeled results. Stateful decoders do not run an additional deep
// pass; use the package-level DecodeStructured for strict+deep comparison.
func (d *Decoder) DecodeStructured(iwave []int16) DecodeResult {
	messages := d.DecodeMessages(iwave)
	result := DecodeResult{
		Strict:   messages,
		Messages: make([]StructuredMessage, 0, len(messages)),
	}
	for _, msg := range messages {
		copyMsg := msg
		result.Messages = append(result.Messages, StructuredMessage{
			DecodedMessage: msg,
			Mode:           DecodeModeStrict,
			Strict:         true,
			StrictCopy:     &copyMsg,
		})
	}
	return result
}

type a7Hint struct {
	Call1  string
	Call2  string
	Grid   string
	FreqHz float64
	DTSec  float64
}

func collectA7Hints(messages []DecodedMessage) []a7Hint {
	out := make([]a7Hint, 0, len(messages))
	seen := make(map[string]bool)
	for _, msg := range messages {
		hint, ok := parseA7Hint(msg)
		if !ok {
			continue
		}
		key := hint.Call1 + "\x00" + hint.Call2 + "\x00" + hint.Grid
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, hint)
	}
	return out
}

func parseA7Hint(msg DecodedMessage) (a7Hint, bool) {
	text := strings.ToUpper(strings.TrimSpace(msg.Text))
	if strings.Contains(text, "/") || strings.Contains(text, "<") {
		return a7Hint{}, false
	}
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return a7Hint{}, false
	}

	var hint a7Hint
	hint.FreqHz = msg.FreqHz
	hint.DTSec = msg.DTSec
	if fields[0] == "CQ" {
		if len(fields) >= 4 && isDirectedCQToken(fields[1]) && callOK(fields[2]) {
			hint.Call1 = "CQ " + fields[1]
			hint.Call2 = fields[2]
			if isGrid4(fields[len(fields)-1]) {
				hint.Grid = fields[len(fields)-1]
			}
			return hint, true
		}
		if callOK(fields[1]) {
			hint.Call1 = "CQ"
			hint.Call2 = fields[1]
			if isGrid4(fields[len(fields)-1]) {
				hint.Grid = fields[len(fields)-1]
			}
			return hint, true
		}
		return a7Hint{}, false
	}

	if !callOK(fields[0]) || !callOK(fields[1]) {
		return a7Hint{}, false
	}
	hint.Call1 = fields[0]
	hint.Call2 = fields[1]
	if isGrid4(fields[len(fields)-1]) {
		hint.Grid = fields[len(fields)-1]
	}
	return hint, true
}

func isDirectedCQToken(s string) bool {
	if len(s) == 3 && allDigits(s) {
		return true
	}
	return len(s) >= 1 && len(s) <= 4 && allLetters(s)
}

func decodeA7Hints(dd []float32, hints []a7Hint, seen map[string]bool) []DecodedMessage {
	if len(hints) == 0 {
		return nil
	}
	ds := newDownsampler()
	cache := make(map[string][174]int8)
	out := make([]DecodedMessage, 0)
	recompute := true
	for _, hint := range hints {
		cand := candidate{FreqHz: hint.FreqHz, DTSec: hint.DTSec, Sync: 99}
		analysis := analyzeCandidateWithDownsampler(dd, ds, cand, recompute)
		recompute = false
		decoded, ok := decodeA7Candidate(&analysis, hint, cache)
		if !ok || seen[decoded.Text] {
			continue
		}
		seen[decoded.Text] = true
		out = append(out, DecodedMessage{
			Text:           decoded.Text,
			FreqHz:         analysis.Refined.FreqHz,
			DTSec:          analysis.Refined.DTSec - 0.5,
			Sync:           analysis.Refined.Sync,
			HardSync:       analysis.Refined.HardSync,
			CostasGeo:      analysis.Refined.CostasGeo,
			CostasMinBlock: analysis.Refined.CostasMinBlock,
			Blocks:         50,
			HardErrors:     decoded.Result.HardErrors,
			DMin:           decoded.Result.DMin,
		})
	}
	return out
}

func decodeA7Candidate(analysis *candidateAnalysis, hint a7Hint, cache map[string][174]int8) (candidateDecode, bool) {
	if analysis.Refined.HardSync <= 6 {
		return candidateDecode{}, false
	}

	passes := [4][174]float64{
		scaleLLR(analysis.Metrics.Single),
		scaleLLR(analysis.Metrics.Double),
		scaleLLR(analysis.Metrics.Triple),
		scaleLLR(analysis.Metrics.Normed),
	}

	bestDistance := math.Inf(1)
	secondDistance := math.Inf(1)
	bestText := ""
	bestResult := ldpcResult{}
	seenMessages := make(map[string]bool)

	for _, text := range a7CandidateMessages(hint) {
		if seenMessages[text] {
			continue
		}
		seenMessages[text] = true
		cw, ok := cachedA7Codeword(text, cache)
		if !ok {
			continue
		}
		for passIndex := range passes {
			llr := &passes[passIndex]
			d := softDistance(cw, llr)
			if d >= bestDistance {
				if d < secondDistance {
					secondDistance = d
				}
				continue
			}
			secondDistance = bestDistance
			bestDistance = d
			bestText = text
			bestResult = ldpcResult{
				Codeword:   cw,
				HardErrors: hardErrors(cw, llr),
				DMin:       d,
			}
			copy(bestResult.Message91[:], cw[:91])
		}
	}

	if bestText == "" || bestDistance > 100 {
		return candidateDecode{}, false
	}
	if !math.IsInf(secondDistance, 1) && bestDistance > 0 && secondDistance/bestDistance < 1.3 {
		return candidateDecode{}, false
	}
	if strings.HasPrefix(bestText, "CQ ") && hint.Grid == "" {
		return candidateDecode{}, false
	}
	return candidateDecode{Text: bestText, Result: bestResult}, true
}

func scaleLLR(metric [174]float64) [174]float64 {
	var out [174]float64
	for i, v := range metric {
		out[i] = ft8ScaleFac * v
	}
	return out
}

func cachedA7Codeword(text string, cache map[string][174]int8) ([174]int8, bool) {
	if cw, ok := cache[text]; ok {
		return cw, true
	}
	bits, ok := pack77StandardMessage(text)
	if !ok {
		return [174]int8{}, false
	}
	cw := encode17491(bits)
	cache[text] = cw
	return cw, true
}

func a7CandidateMessages(hint a7Hint) []string {
	base := strings.TrimSpace(hint.Call1 + " " + hint.Call2)
	out := make([]string, 0, 206)
	out = append(out, base)
	out = append(out, base+" RRR")
	out = append(out, base+" RR73")
	out = append(out, base+" 73")

	if callOK(hint.Call2) {
		cq := "CQ " + hint.Call2
		if strings.HasPrefix(hint.Call1, "CQ ") {
			cq = hint.Call1 + " " + hint.Call2
		}
		if hint.Grid != "" {
			cq += " " + hint.Grid
		}
		out = append(out, cq)
	}
	if hint.Grid != "" && callOK(hint.Call2) {
		out = append(out, base+" "+hint.Grid)
	}

	for snr := -50; snr <= 49; snr++ {
		report := formatReport(snr)
		out = append(out, base+" "+report)
		out = append(out, base+" R"+report)
	}
	return out
}
