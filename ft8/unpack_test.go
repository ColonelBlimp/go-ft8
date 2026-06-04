// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"fmt"
	"testing"
)

func TestHashTableLookup12UsesRollingHistory(t *testing.T) {
	older, newer, hash := findHash12Collision(t)
	var hashes hashTable
	hashes.Save(older)
	if got := hashes.Lookup12(hash); got != older {
		t.Fatalf("initial Lookup12 got %q, want %q", got, older)
	}

	hashes.Save(newer)
	if got := hashes.Lookup12(hash); got != newer {
		t.Fatalf("collision Lookup12 got %q, want newest %q", got, newer)
	}

	for i := 0; containsHashCall(hashes.calls22, older) || containsHashCall(hashes.calls22, newer); i++ {
		if i > 20000 {
			t.Fatal("failed to age colliding calls out of hash table")
		}
		call := hashTestCall(10000 + i)
		if hashCall(call, 12) == hash {
			continue
		}
		hashes.Save(call)
	}
	if got := hashes.Lookup12(hash); got != "" {
		t.Fatalf("aged Lookup12 got %q, want empty", got)
	}
}

func findHash12Collision(t *testing.T) (string, string, int) {
	t.Helper()
	seen := make(map[int]string)
	for i := 0; i < 20000; i++ {
		call := hashTestCall(i)
		hash := hashCall(call, 12)
		if prev, ok := seen[hash]; ok && prev != call {
			return prev, call, hash
		}
		seen[hash] = call
	}
	t.Fatal("failed to find deterministic 12-bit hash collision")
	return "", "", 0
}

func hashTestCall(i int) string {
	return fmt.Sprintf("K%05d", i)
}

func containsHashCall(entries []hashEntry, call string) bool {
	for _, entry := range entries {
		if entry.call == call {
			return true
		}
	}
	return false
}
