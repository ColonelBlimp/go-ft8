// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"os"
	"path/filepath"
	"testing"
)

func corpusTruthFiles(t testing.TB) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(corpusTestdataDir(), "*.truth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Skip("no truth fixtures")
	}
	return matches
}

func corpusWAVFiles(t testing.TB) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(corpusTestdataDir(), "*.wav"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Skip("no WAV fixtures")
	}
	return matches
}

func corpusPath(name string) string {
	return filepath.Join(corpusTestdataDir(), name)
}

func loadCorpusWAV(t testing.TB, name string) []int16 {
	t.Helper()
	samples, err := loadWAV(corpusPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("missing WAV fixture %s", name)
		}
		t.Fatal(err)
	}
	return samples
}

func corpusTestdataDir() string {
	for _, dir := range []string{"testdata", filepath.Join("..", "testdata")} {
		if matches, _ := filepath.Glob(filepath.Join(dir, "*.truth.json")); len(matches) > 0 {
			return dir
		}
		if matches, _ := filepath.Glob(filepath.Join(dir, "*.wav")); len(matches) > 0 {
			return dir
		}
	}
	return "testdata"
}
