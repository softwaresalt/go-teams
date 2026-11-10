package capture

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/transcript"
)

type captionPair struct {
	speaker string
	text    string
}

// fakeMeeting is a test double for browser.Meeting. It records call order and
// drives caption/audio methods deterministically.
type fakeMeeting struct {
	mu sync.Mutex

	joinErr   error
	audioErr  error
	aiSummary string
	isRecap   bool

	events []captionPair

	order          []string
	emitted        chan struct{} // closed after events are delivered
	audioStarted   chan struct{} // closed when CaptureAudio begins
	audioReturnNow bool          // if true, CaptureAudio returns audioErr without blocking
}

func (f *fakeMeeting) AISummary() string {
	return f.aiSummary
}

func (f *fakeMeeting) IsRecap() bool {
	return f.isRecap
}

func (f *fakeMeeting) mark(s string) {
	f.mu.Lock()
	f.order = append(f.order, s)
	f.mu.Unlock()
}

func (f *fakeMeeting) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.order...)
}

func (f *fakeMeeting) Join(ctx context.Context) error {
	f.mark("join")
	return f.joinErr
}

func (f *fakeMeeting) EnableCaptions(ctx context.Context, sink browser.CaptionSink) error {
	f.mark("captions")
	for _, e := range f.events {
		sink(e.speaker, e.text)
	}
	if f.emitted != nil {
		close(f.emitted)
	}
	if f.isRecap {
		return nil
	}
	<-ctx.Done()
	return nil
}

func (f *fakeMeeting) CaptureAudio(ctx context.Context, w io.Writer) error {
	f.mark("audio")
	if f.audioStarted != nil {
		close(f.audioStarted)
	}
	if f.audioReturnNow {
		return f.audioErr
	}
	<-ctx.Done()
	return nil
}

func newTestSession(dir string) *Session {
	s := NewSession(SessionConfig{
		OutputDir:  dir,
		MeetingURL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_Test%40thread.v2/0",
		BotName:    "Bot",
		Now:        fixedTime(),
	})
	s.now = func() time.Time { return fixedTime() }
	return s
}

func TestSessionRunJoinBeforeOutputAndCapture(t *testing.T) {
	dir := t.TempDir()
	s := newTestSession(dir)
	fm := &fakeMeeting{
		events:  []captionPair{{"Alice", "Hello there."}},
		emitted: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Run(ctx, fm) }()

	<-fm.emitted // captions delivered => join already succeeded and output ready
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	calls := fm.calls()
	if len(calls) == 0 || calls[0] != "join" {
		t.Fatalf("expected join first, got %v", calls)
	}
	// Transcript should contain the emitted caption.
	entries, _ := os.ReadDir(dir)
	var md string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			b, _ := os.ReadFile(dir + string(os.PathSeparator) + e.Name())
			md = string(b)
		}
	}
	if !strings.Contains(md, "Alice") || !strings.Contains(md, "Hello there.") {
		t.Errorf("transcript missing caption; got:\n%s", md)
	}
}

func TestSessionRunJoinFailureNoOutput(t *testing.T) {
	dir := t.TempDir()
	s := newTestSession(dir)
	boom := errors.New("boom join")
	fm := &fakeMeeting{joinErr: boom}

	err := s.Run(context.Background(), fm)
	if !errors.Is(err, boom) {
		t.Fatalf("Run error = %v, want wrapping %v", err, boom)
	}
	if calls := fm.calls(); len(calls) != 1 || calls[0] != "join" {
		t.Fatalf("capture should not start after join failure, calls=%v", calls)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("no output files expected on join failure, got %d", len(entries))
	}
}

func TestSessionRunJoinCancelledGraceful(t *testing.T) {
	dir := t.TempDir()
	s := newTestSession(dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fm := &fakeMeeting{joinErr: context.Canceled}

	err := s.Run(ctx, fm)
	if err != nil {
		t.Fatalf("cancelled join should be graceful, got %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("no output files expected on cancelled join, got %d", len(entries))
	}
}

func TestSessionRunWriteErrorPropagatesAndCancels(t *testing.T) {
	boom := errors.New("disk full")
	s := NewSession(SessionConfig{OutputDir: t.TempDir(), MeetingURL: "x", BotName: "b"})
	// Inject an output whose transcript writes to a failing writer.
	s.setupOutput = func(OutputConfig) (*Output, error) {
		return &Output{
			Transcript: transcript.NewWriter(failWriter{err: boom}),
			Audio:      io.Discard,
		}, nil
	}
	s.now = func() time.Time { return fixedTime() }

	// A caption large enough to bypass bufio buffering and hit the writer.
	fm := &fakeMeeting{
		events:  []captionPair{{"Bob", strings.Repeat("x", 8192) + "."}},
		emitted: make(chan struct{}),
	}

	err := s.Run(context.Background(), fm)
	if !errors.Is(err, boom) {
		t.Fatalf("write error should propagate, got %v", err)
	}
}

func TestSessionRunCancelDuringCaptureGraceful(t *testing.T) {
	dir := t.TempDir()
	s := newTestSession(dir)
	fm := &fakeMeeting{audioStarted: make(chan struct{})}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Run(ctx, fm) }()

	<-fm.audioStarted
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("cancel during capture should be graceful, got %v", err)
	}
	// Output files should have been created (join succeeded).
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("expected transcript+audio files, got %d", len(entries))
	}
}

func TestSessionRunCaptureAudioErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	s := newTestSession(dir)
	boom := errors.New("audio device lost")
	fm := &fakeMeeting{audioErr: boom, audioReturnNow: true}

	err := s.Run(context.Background(), fm)
	if !errors.Is(err, boom) {
		t.Fatalf("audio error should propagate, got %v", err)
	}
}

func TestSessionRunCaptionsOnlySkipsAudioAndFlushesImmediately(t *testing.T) {
	dir := t.TempDir()
	s := NewSession(SessionConfig{
		OutputDir:    dir,
		MeetingURL:   "https://teams.microsoft.com/meet/123456?p=token",
		BotName:      "Bot",
		Now:          fixedTime(),
		CaptionsOnly: true,
	})
	s.now = fixedTime

	fm := &fakeMeeting{
		events:  []captionPair{{"Alice", "Real-time flushed caption."}},
		emitted: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Run(ctx, fm) }()

	<-fm.emitted
	// Before cancel() is called, the caption must already be flushed to disk.
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 markdown file while session is active, got %v (err=%v)", entries, err)
	}
	liveBytes, err := os.ReadFile(dir + string(os.PathSeparator) + entries[0].Name())
	if err != nil {
		t.Fatalf("read live transcript: %v", err)
	}
	if !strings.Contains(string(liveBytes), "Real-time flushed caption.") {
		t.Fatalf("expected caption flushed to disk before session close, got:\n%s", string(liveBytes))
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, call := range fm.calls() {
		if call == "audio" {
			t.Fatalf("CaptureAudio must not be called when CaptionsOnly is true; calls=%v", fm.calls())
		}
	}
}

func TestSessionRunAppendsAISummaryWhenProvided(t *testing.T) {
	dir := t.TempDir()
	s := NewSession(SessionConfig{
		OutputDir:    dir,
		MeetingURL:   "https://teams.microsoft.com/meet/123456?p=token",
		BotName:      "Bot",
		Now:          fixedTime(),
		CaptionsOnly: true,
	})
	s.now = fixedTime

	fm := &fakeMeeting{
		events:    []captionPair{{"Alice", "First utterance."}},
		aiSummary: "Meeting notes:\nKey Topic:\nDiscussed key items.\n",
		emitted:   make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Run(ctx, fm) }()

	<-fm.emitted
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 markdown file, got %v", entries)
	}
	b, err := os.ReadFile(dir + string(os.PathSeparator) + entries[0].Name())
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	md := string(b)
	if !strings.Contains(md, "# AI Summary") || !strings.Contains(md, "Discussed key items.") {
		t.Fatalf("expected appended AI Summary in markdown, got:\n%s", md)
	}
}

func TestSessionRunRecapModeSkipsAudioEvenWhenCaptionsOnlyFalse(t *testing.T) {
	dir := t.TempDir()
	s := NewSession(SessionConfig{
		OutputDir:    dir,
		MeetingURL:   "https://teams.cloud.microsoft/l/meetingrecap?threadId=1",
		BotName:      "Bot",
		Now:          fixedTime(),
		CaptionsOnly: false, // Even if audio was requested, Recap mode must skip live audio capture.
	})
	s.now = fixedTime

	fm := &fakeMeeting{
		isRecap:        true,
		events:         []captionPair{{"Alice", "Recap line."}},
		aiSummary:      "Meeting notes:\nScope:\nAgreed on scope.\n",
		audioErr:       errors.New("audio should not be called in recap mode"),
		audioReturnNow: true,
	}

	if err := s.Run(context.Background(), fm); err != nil {
		t.Fatalf("Run in recap mode failed: %v", err)
	}
	for _, call := range fm.calls() {
		if call == "audio" {
			t.Fatalf("CaptureAudio must not be called in recap mode; calls=%v", fm.calls())
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".md") {
		t.Fatalf("expected only 1 .md file in recap mode, got %v", entries)
	}
}

// failWriter fails every Write with err.
type failWriter struct{ err error }

func (f failWriter) Write(_ []byte) (int, error) { return 0, f.err }
