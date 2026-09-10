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
