// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft4

import (
	"sync"
	"testing"
)

func TestStatelessDecodeConcurrentNoRace(t *testing.T) {
	encoded, err := EncodeStandardMessage("CQ 7Q5MLV KH67")
	if err != nil {
		t.Fatal(err)
	}
	samples := synthesizeOracleWave(encoded.Tones, 1500)
	const workers = 2

	var wg sync.WaitGroup
	errs := make(chan string, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			messages := DecodeMessages(samples)
			if len(messages) != 1 || messages[0].Text != encoded.Text {
				errs <- "DecodeMessages did not recover the test frame"
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestDecoderHistoryConcurrentStreamsNoRace(t *testing.T) {
	samples := historyWave(historyPayloads(t)["10"], 1500)
	var wg sync.WaitGroup
	for _, seed := range []string{"DX1ABC", "W9XYZ"} {
		wg.Go(func() {
			decoder := NewDecoder()
			decoder.RememberCallsigns(seed)
			want := "W1AAA RR73; N3CCC <...> -10"
			if seed == "DX1ABC" {
				want = "W1AAA RR73; N3CCC <DX1ABC> -10"
			}
			for range 2 {
				messages := decoder.DecodeMessages(samples)
				if len(messages) != 1 || messages[0].Text != want {
					t.Errorf("stream seeded with %q decoded %+v, want %q", seed, messages, want)
					return
				}
			}
		})
	}
	wg.Wait()
}
