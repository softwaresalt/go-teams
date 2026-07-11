package cli

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/config"
)

// fakeSession is a test double for the capture session seam.
type fakeSession struct {
	runErr     error
	gotCtx     context.Context
	gotMeeting browser.Meeting
	ran        chan struct{} // signalled once when Run is entered
	block      chan struct{} // if non-nil, Run blocks until it is closed
	once       sync.Once
}

func (f *fakeSession) Run(ctx context.Context, m browser.Meeting) error {
	f.gotCtx = ctx
	f.gotMeeting = m
	if f.ran != nil {
		f.once.Do(func() { close(f.ran) })
	}
	if f.block != nil {
		<-f.block
	}
	return f.runErr
}

func TestRunCaptureCleanupOnSuccess(t *testing.T) {
	var cleanupCalls int32
	sess := &fakeSession{}
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			return nil, ctx, func() { atomic.AddInt32(&cleanupCalls, 1) }, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	if err := runCapture(context.Background(), d, config.Config{}, "url"); err != nil {
		t.Fatalf("runCapture: %v", err)
	}
	if got := atomic.LoadInt32(&cleanupCalls); got != 1 {
		t.Fatalf("cleanup called %d times, want 1", got)
	}
}

func TestRunCaptureCleanupOnError(t *testing.T) {
	var cleanupCalls int32
	boom := errors.New("session boom")
	sess := &fakeSession{runErr: boom}
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			return nil, ctx, func() { atomic.AddInt32(&cleanupCalls, 1) }, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	err := runCapture(context.Background(), d, config.Config{}, "url")
	if !errors.Is(err, boom) {
		t.Fatalf("runCapture error = %v, want %v", err, boom)
	}
	if got := atomic.LoadInt32(&cleanupCalls); got != 1 {
		t.Fatalf("cleanup called %d times on error, want 1", got)
	}
}

func TestRunCaptureNewMeetingErrorNoCleanup(t *testing.T) {
	var cleanupCalls int32
	boom := errors.New("alloc boom")
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			return nil, ctx, func() { atomic.AddInt32(&cleanupCalls, 1) }, boom
		},
		newSession: func(config.Config, string) meetingSession {
			t.Fatal("newSession must not be called when newMeeting fails")
			return nil
		},
	}
	err := runCapture(context.Background(), d, config.Config{}, "url")
	if !errors.Is(err, boom) {
		t.Fatalf("runCapture error = %v, want %v", err, boom)
	}
	if got := atomic.LoadInt32(&cleanupCalls); got != 0 {
		t.Fatalf("cleanup must not run on newMeeting failure, got %d", got)
	}
}

func TestRunCaptureCleanupOnCancellation(t *testing.T) {
	var cleanupCalls int32
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // caller cancellation (e.g. Ctrl+C) must still tear the browser down
	sess := &fakeSession{runErr: context.Canceled}
	d := &deps{
		newMeeting: func(mctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			return nil, mctx, func() { atomic.AddInt32(&cleanupCalls, 1) }, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	_ = runCapture(ctx, d, config.Config{}, "url")
	if got := atomic.LoadInt32(&cleanupCalls); got != 1 {
		t.Fatalf("cleanup must run on cancellation, got %d", got)
	}
}
