// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"sync"
	"testing"
)

func TestStatelessDecodeConcurrentNoRace(t *testing.T) {
	samples := loadCorpusWAV(t, "20m_slot2.wav")
	const workers = 2

	var wg sync.WaitGroup
	errs := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := DecodeMessages(samples); len(got) == 0 {
				errs <- "DecodeMessages returned no messages"
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatal(err)
	}
}
