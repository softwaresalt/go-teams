package browser

import (
	"fmt"
	"sync"
	"testing"
)

func TestPayloadPumpProcessesAllInOrder(t *testing.T) {
	var got []string
	p := newPayloadPump(8, func(raw string) { got = append(got, raw) })
	want := []string{"a", "b", "c", "d"}
	for _, s := range want {
		if !p.offer(s) {
			t.Fatalf("offer(%q) rejected", s)
		}
	}
	p.stop()
	if len(got) != len(want) {
		t.Fatalf("processed %d payloads, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order mismatch at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

// TestPayloadPumpDrainsBufferedBeforeStopReturns proves the producer barrier:
// stop must not return until every buffered payload has been processed.
func TestPayloadPumpDrainsBufferedBeforeStopReturns(t *testing.T) {
	release := make(chan struct{})
	var mu sync.Mutex
	got := 0
	p := newPayloadPump(16, func(string) {
		<-release
		mu.Lock()
		got++
		mu.Unlock()
	})
	for i := 0; i < 5; i++ {
		if !p.offer("x") {
			t.Fatalf("offer %d rejected", i)
		}
	}
	close(release)
	p.stop()

	mu.Lock()
	n := got
	mu.Unlock()
	if n != 5 {
		t.Fatalf("stop returned before draining: processed %d, want 5", n)
	}
}

func TestPayloadPumpRejectsOfferAfterStop(t *testing.T) {
	p := newPayloadPump(4, func(string) {})
	p.stop()
	if p.offer("late") {
		t.Fatal("offer after stop should be rejected")
	}
}

// TestPayloadPumpConcurrentOffersNoLoss exercises the barrier under concurrent
// producers (run with -race): every accepted payload must be delivered.
func TestPayloadPumpConcurrentOffersNoLoss(t *testing.T) {
	var mu sync.Mutex
	got := make(map[string]bool)
	p := newPayloadPump(256, func(raw string) {
		mu.Lock()
		got[raw] = true
		mu.Unlock()
	})
	const n = 100
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p.offer(fmt.Sprintf("m%d", i))
		}(i)
	}
	wg.Wait()
	p.stop()

	mu.Lock()
	c := len(got)
	mu.Unlock()
	if c != n {
		t.Fatalf("lost payloads: delivered %d, want %d", c, n)
	}
}
