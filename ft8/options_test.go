// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"os"
	"testing"
	"time"
)

func TestDeepDecoderOptionsEnableOSD(t *testing.T) {
	if normalizeDecoderOptions(DecoderOptions{}).enableOSD {
		t.Fatal("strict options unexpectedly enable OSD")
	}
	if !normalizeDecoderOptions(DeepDecoderOptions()).enableOSD {
		t.Fatal("deep options did not enable OSD")
	}
	if decoderOptionsEmpty(DecoderOptions{EnableOSD: true}) {
		t.Fatal("OSD-only options treated as empty")
	}
}

func TestDiagnosticOptionSweep(t *testing.T) {
	if os.Getenv("DECODE_DIAGNOSTICS") == "" {
		t.Skip("set DECODE_DIAGNOSTICS=1")
	}
	matches := corpusTruthFiles(t)

	type sweepCase struct {
		name    string
		options DecoderOptions
	}
	cases := []sweepCase{
		{name: "strict"},
		{name: "winsor-2.0", options: DecoderOptions{LLRWinsorFactor: 2.0}},
		{name: "winsor-2.5", options: DecoderOptions{LLRWinsorFactor: 2.5}},
		{name: "winsor-3.0", options: DecoderOptions{LLRWinsorFactor: 3.0}},
		{name: "winsor-4.0", options: DecoderOptions{LLRWinsorFactor: 4.0}},
		{name: "winsor-6.0", options: DecoderOptions{LLRWinsorFactor: 6.0}},
		{name: "loose-sync-1.6", options: DecoderOptions{SyncMin: 1.6, MaxCandidates: 1000}},
		{name: "loose-sync-1.6-winsor-2.5", options: DecoderOptions{SyncMin: 1.6, MaxCandidates: 1000, LLRWinsorFactor: 2.5}},
		{name: "loose-sync-1.6-winsor-4.0", options: DecoderOptions{SyncMin: 1.6, MaxCandidates: 1000, LLRWinsorFactor: 4.0}},
		{name: "loose-sync-1.4", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000}},
		{name: "loose-sync-1.4-winsor-2.5", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, LLRWinsorFactor: 2.5}},
		{name: "loose-sync-1.4-winsor-4.0", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, LLRWinsorFactor: 4.0}},
		{name: "hard-sync-7", options: DecoderOptions{HardSyncMin: 7}},
		{name: "hard-sync-6", options: DecoderOptions{HardSyncMin: 6}},
		{name: "sync-1.6-hard-7", options: DecoderOptions{SyncMin: 1.6, MaxCandidates: 1000, HardSyncMin: 7}},
		{name: "sync-1.4-hard-7", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7}},
		{name: "sync-1.4-hard-6", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 6}},
		{name: "sync-1.4-hard-7-winsor-4.0", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7, LLRWinsorFactor: 4.0}},
		{name: "sync-1.4-hard-7-geo-1.05", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7, CostasMinGeo: 1.05}},
		{name: "sync-1.4-hard-7-geo-1.10", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7, CostasMinGeo: 1.10}},
		{name: "sync-1.4-hard-7-minblk-0.80", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7, CostasMinBlock: 0.80}},
		{name: "sync-1.4-hard-7-minblk-0.90", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7, CostasMinBlock: 0.90}},
		{name: "sync-1.4-hard-7-wins-10", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7, CostasMinWins: 10}},
		{name: "blocks-50-47-41", options: DecoderOptions{Blocks: []int{50, 47, 41}}},
		{name: "wide-50-3500", options: DecoderOptions{MinFreqHz: 50, MaxFreqHz: 3500}},
		{name: "wide-blocks", options: DecoderOptions{MinFreqHz: 50, MaxFreqHz: 3500, Blocks: []int{50, 47, 41}}},
		{name: "deep-sync-1.4-hard-7-blocks", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7, Blocks: []int{50, 47, 41}}},
		{name: "deep-wide-blocks", options: DecoderOptions{MinFreqHz: 50, MaxFreqHz: 3500, SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7, Blocks: []int{50, 47, 41}}},
		{name: "deep-blocks-50-41-43-44", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7, Blocks: []int{50, 41, 43, 44}}},
		{name: "deep-blocks-50-41-43", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 6, Blocks: []int{50, 41, 43}}},
	}

	for _, tc := range cases {
		start := time.Now()
		hit, truth, output, extras, missing := scoreCorpusWithOptions(t, matches, tc.options)
		t.Logf("%-28s matched=%3d/%3d extras=%3d output=%3d elapsed=%s",
			tc.name, hit, truth, output-hit, output, time.Since(start).Round(time.Millisecond))
		if len(extras) > 0 {
			t.Logf("%-28s extras=%v", tc.name, extras)
		}
		if len(missing) > 0 {
			t.Logf("%-28s missing=%v", tc.name, missing)
		}
	}
}

func TestDiagnosticBlockSweep(t *testing.T) {
	if os.Getenv("DECODE_DIAGNOSTICS") == "" {
		t.Skip("set DECODE_DIAGNOSTICS=1")
	}
	matches := corpusTruthFiles(t)

	for _, hardSyncMin := range []int{7, 6} {
		seenExtras := make(map[string]bool)
		for blocks := 35; blocks <= 52; blocks++ {
			options := DecoderOptions{
				SyncMin:       1.4,
				MaxCandidates: 1000,
				HardSyncMin:   hardSyncMin,
				Blocks:        []int{blocks},
			}
			start := time.Now()
			hit, truth, output, extras, missing := scoreCorpusWithOptions(t, matches, options)
			fresh := 0
			for _, extra := range extras {
				if !seenExtras[extra] {
					seenExtras[extra] = true
					fresh++
				}
			}
			t.Logf("hard-%d block-%02d matched=%3d/%3d extras=%3d fresh=%2d output=%3d missing=%2d elapsed=%s",
				hardSyncMin, blocks, hit, truth, output-hit, fresh, output, len(missing), time.Since(start).Round(time.Millisecond))
			if len(extras) > 0 {
				t.Logf("hard-%d block-%02d extras=%v", hardSyncMin, blocks, extras)
			}
		}
		t.Logf("hard-%d unique block-sweep extras=%d", hardSyncMin, len(seenExtras))
	}
}

func TestDiagnosticDeepPresetSweep(t *testing.T) {
	if os.Getenv("DECODE_DIAGNOSTICS") == "" {
		t.Skip("set DECODE_DIAGNOSTICS=1")
	}
	matches := corpusTruthFiles(t)

	cases := []struct {
		name    string
		options DecoderOptions
	}{
		{name: "deep-preset", options: DeepDecoderOptions()},
		{name: "deep-hard-6-three-blocks", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 6, Blocks: []int{50, 41, 43}}},
		{name: "deep-hard-6", options: DecoderOptions{SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 6, Blocks: []int{50, 41, 43, 44}}},
		{name: "deep-sync-1.2", options: DecoderOptions{SyncMin: 1.2, MaxCandidates: 1000, HardSyncMin: 7, Blocks: []int{50, 41, 43, 44}}},
		{name: "deep-sync-1.2-hard-6", options: DecoderOptions{SyncMin: 1.2, MaxCandidates: 1000, HardSyncMin: 6, Blocks: []int{50, 41, 43, 44}}},
		{name: "deep-wide", options: DecoderOptions{MinFreqHz: 50, MaxFreqHz: 3500, SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 7, Blocks: []int{50, 41, 43, 44}}},
		{name: "deep-wide-hard-6", options: DecoderOptions{MinFreqHz: 50, MaxFreqHz: 3500, SyncMin: 1.4, MaxCandidates: 1000, HardSyncMin: 6, Blocks: []int{50, 41, 43, 44}}},
	}
	for _, tc := range cases {
		start := time.Now()
		hit, truth, output, extras, missing := scoreCorpusWithOptions(t, matches, tc.options)
		t.Logf("%-24s matched=%3d/%3d extras=%3d output=%3d missing=%2d elapsed=%s",
			tc.name, hit, truth, output-hit, output, len(missing), time.Since(start).Round(time.Millisecond))
		if len(extras) > 0 {
			t.Logf("%-24s extras=%v", tc.name, extras)
		}
		if len(missing) > 0 {
			t.Logf("%-24s missing=%v", tc.name, missing)
		}
	}
}

func TestDiagnosticDeepCandidateCapSweep(t *testing.T) {
	if os.Getenv("DECODE_DIAGNOSTICS") == "" {
		t.Skip("set DECODE_DIAGNOSTICS=1")
	}
	matches := corpusTruthFiles(t)

	for _, maxCandidates := range []int{200, 225, 250, 275, 300, 400, 500, 750, 1000} {
		options := DeepDecoderOptions()
		options.MaxCandidates = maxCandidates
		start := time.Now()
		hit, truth, output, extras, missing := scoreCorpusWithOptions(t, matches, options)
		t.Logf("cap-%04d matched=%3d/%3d extras=%3d output=%3d missing=%2d elapsed=%s",
			maxCandidates, hit, truth, output-hit, output, len(missing), time.Since(start).Round(time.Millisecond))
		if len(extras) > 0 {
			t.Logf("cap-%04d extras=%v", maxCandidates, extras)
		}
		if len(missing) > 0 {
			t.Logf("cap-%04d missing=%v", maxCandidates, missing)
		}
	}
}

func scoreCorpusWithOptions(t *testing.T, truthPaths []string, options DecoderOptions) (hit, truth, output int, extras, missing []string) {
	t.Helper()
	for _, truthPath := range truthPaths {
		tf := readTruth(t, truthPath)
		samples := loadCorpusWAV(t, tf.WAV)
		want := make(map[string]bool)
		for _, sig := range tf.Signals {
			want[normalizeTruthText(sig.Text)] = true
		}
		got := DecodeMessagesWithOptions(samples, options)
		found := make(map[string]bool)
		for _, msg := range got {
			if want[msg.Text] {
				hit++
				found[msg.Text] = true
			} else {
				extras = append(extras, tf.WAV+": "+msg.Text)
			}
		}
		for text := range want {
			if !found[text] {
				missing = append(missing, tf.WAV+": "+text)
			}
		}
		truth += len(want)
		output += len(got)
	}
	return hit, truth, output, extras, missing
}
