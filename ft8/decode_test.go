// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import "testing"

func TestWinsorizeLLRUsesEvenMedianAverage(t *testing.T) {
	var llr [174]float64
	llr[0] = 1
	llr[1] = -3

	winsorizeLLR(&llr, 1)

	if llr[0] != 1 {
		t.Fatalf("llr[0] got %v, want 1", llr[0])
	}
	if llr[1] != -2 {
		t.Fatalf("llr[1] got %v, want -2", llr[1])
	}
}

func TestRejectedCandidateDoesNotMutateHashTable(t *testing.T) {
	bits, ok := pack77StandardMessage("K1ABC W9XYZ/R -10")
	if !ok {
		t.Fatal("failed to pack filtered /R message")
	}
	var hashes hashTable
	_, status := unpackCandidateMessage(encode17491(bits), &hashes)
	if status != candidateMessageFiltered {
		t.Fatalf("status got %v, want candidateMessageFiltered", status)
	}
	if len(hashes.calls22) != 0 {
		t.Fatalf("filtered candidate retained %d hash entries, want 0", len(hashes.calls22))
	}
}

func TestAcceptedCandidateCommitsHashTable(t *testing.T) {
	bits, ok := pack77StandardMessage("K1ABC W9XYZ -10")
	if !ok {
		t.Fatal("failed to pack accepted message")
	}
	var hashes hashTable
	msg, status := unpackCandidateMessage(encode17491(bits), &hashes)
	if status != candidateMessageAccepted || msg != "K1ABC W9XYZ -10" {
		t.Fatalf("candidate got status=%v msg=%q", status, msg)
	}
	if got := hashes.Lookup22(hashCall("W9XYZ", 22)); got != "W9XYZ" {
		t.Fatalf("committed hash got %q, want W9XYZ", got)
	}
}

func TestAcceptedCandidateDoesNotMutateFrozenHashTable(t *testing.T) {
	bits, ok := pack77StandardMessage("K1ABC W9XYZ -10")
	if !ok {
		t.Fatal("failed to pack accepted message")
	}
	var hashes hashTable
	frozen := hashes.frozenSnapshot()
	_, status := unpackCandidateMessage(encode17491(bits), frozen)
	if status != candidateMessageAccepted {
		t.Fatalf("status got %v, want candidateMessageAccepted", status)
	}
	if len(frozen.calls22) != 0 {
		t.Fatalf("worker snapshot retained %d hash entries, want 0", len(frozen.calls22))
	}
}
