// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math/rand"
	"testing"
)

func TestCRC14FastMatchesSlow(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for n := 0; n < 1000; n++ {
		var bits [96]int8
		for i := range bits {
			bits[i] = int8(rng.Intn(2))
		}
		if got, want := crc14Remainder(bits[:]), crc14RemainderSlow(bits[:]); got != want {
			t.Fatalf("crc14Remainder mismatch: got %d want %d", got, want)
		}

		var msg [77]int8
		copy(msg[:], bits[:77])
		var m96 [96]int8
		copy(m96[:77], msg[:])
		if got, want := crc14RemainderMessage77(msg), crc14RemainderSlow(m96[:]); got != want {
			t.Fatalf("crc14RemainderMessage77 mismatch: got %d want %d", got, want)
		}

		var cw [174]int8
		copy(cw[:77], bits[:77])
		copy(cw[77:91], bits[82:96])
		if got, want := crc14OK(&cw), crc14RemainderSlow(bits[:]) == 0; got != want {
			t.Fatalf("crc14OK mismatch: got %v want %v", got, want)
		}
	}
}

func BenchmarkCRC14Remainder(b *testing.B) {
	var bits [96]int8
	for i := range bits {
		bits[i] = int8(i & 1)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = crc14Remainder(bits[:])
	}
}

func BenchmarkCRC14OK(b *testing.B) {
	var msg [77]int8
	for i := range msg {
		msg[i] = int8((i / 3) & 1)
	}
	cw := encode17491(msg)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = crc14OK(&cw)
	}
}

func crc14RemainderSlow(bits []int8) int {
	poly := [15]int8{1, 1, 0, 0, 1, 1, 1, 0, 1, 0, 1, 0, 1, 1, 1}
	var r [15]int8
	copy(r[:], bits[:15])

	for i := 0; i <= len(bits)-15; i++ {
		r[14] = bits[i+14]
		first := r[0]
		if first != 0 {
			for j := 0; j < 15; j++ {
				r[j] = (r[j] + poly[j]) & 1
			}
		}
		first = r[0]
		for j := 0; j < 14; j++ {
			r[j] = r[j+1]
		}
		r[14] = first
	}

	out := 0
	for i := 0; i < 14; i++ {
		out = out*2 + int(r[i])
	}
	return out
}
