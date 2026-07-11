package browser

import (
	"context"
	"testing"
)

func TestDrainWorkerDrainsBufferedPayloadsOnCancel(t *testing.T) {
	payloads := make(chan string, 8)
	for _, p := range []string{"a", "b", "c"} {
		payloads <- p
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled: the worker must still drain the buffered tail

	var got []string
	drainWorker(ctx, payloads, func(raw string) bool {
		got = append(got, raw)
		return true
	})
	if len(got) != 3 {
		t.Fatalf("drainWorker dropped buffered tail payloads: got %v, want 3", got)
	}
}

func TestDrainWorkerProcessesThenDrains(t *testing.T) {
	payloads := make(chan string, 8)
	ctx, cancel := context.WithCancel(context.Background())

	got := make(chan string, 8)
	done := make(chan struct{})
	go func() {
		drainWorker(ctx, payloads, func(raw string) bool {
			got <- raw
			return true
		})
		close(done)
	}()

	payloads <- "x"
	payloads <- "y"
	payloads <- "z"
	cancel()
	<-done
	close(got)

	n := 0
	for range got {
		n++
	}
	if n != 3 {
		t.Fatalf("expected all 3 payloads processed, got %d", n)
	}
}

func TestDrainWorkerStopsEarly(t *testing.T) {
	payloads := make(chan string, 8)
	for _, p := range []string{"ok", "stop", "never"} {
		payloads <- p
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var got []string
	drainWorker(ctx, payloads, func(raw string) bool {
		got = append(got, raw)
		return raw != "stop"
	})
	if len(got) != 2 || got[len(got)-1] != "stop" {
		t.Fatalf("expected early stop after 'stop', got %v", got)
	}
}
