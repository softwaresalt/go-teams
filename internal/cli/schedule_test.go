package cli

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/config"
	"github.com/softwaresalt/go-teams/internal/scheduler"
)

func TestScheduleJobRunsCapture(t *testing.T) {
	var meetingCalls int32
	var gotURL string
	sess := &fakeSession{}
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, url string) (browser.Meeting, context.Context, func(), error) {
			atomic.AddInt32(&meetingCalls, 1)
			gotURL = url
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	var busy atomic.Bool
	job := scheduleJob(context.Background(), d, config.Config{}, &busy)
	job(scheduler.Entry{Name: "m1", URL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc"})

	if got := atomic.LoadInt32(&meetingCalls); got != 1 {
		t.Fatalf("expected one capture, got %d", got)
	}
	if gotURL == "" {
		t.Fatal("meeting url was not passed to capture")
	}
	if busy.Load() {
		t.Fatal("busy flag should be released after the job returns")
	}
}

func TestScheduleJobBusySkipsOverlappingEntry(t *testing.T) {
	var meetingCalls int32
	ran := make(chan struct{})
	block := make(chan struct{})
	sess := &fakeSession{ran: ran, block: block}
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			atomic.AddInt32(&meetingCalls, 1)
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	var busy atomic.Bool
	job := scheduleJob(context.Background(), d, config.Config{}, &busy)

	go job(scheduler.Entry{Name: "m1", URL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_one"})
	<-ran // first capture is active and holds the shared profile

	// A distinct entry firing while a capture is active must be skipped, not
	// launched concurrently against the shared profile.
	job(scheduler.Entry{Name: "m2", URL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_two"})
	if got := atomic.LoadInt32(&meetingCalls); got != 1 {
		t.Fatalf("overlapping distinct entry should be skipped; launched %d times, want 1", got)
	}

	close(block) // let the first capture finish

	deadline := time.After(2 * time.Second)
	for busy.Load() {
		select {
		case <-deadline:
			t.Fatal("busy flag not released after the active capture finished")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestScheduleJobInvalidURLNotBusy(t *testing.T) {
	sess := &fakeSession{}
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			t.Fatal("newMeeting must not be called for an invalid url")
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	var busy atomic.Bool
	job := scheduleJob(context.Background(), d, config.Config{}, &busy)
	job(scheduler.Entry{Name: "bad", URL: "ftp://example.com"})
	if busy.Load() {
		t.Fatal("busy must not be held after an invalid url is rejected")
	}
}
