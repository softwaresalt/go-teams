package capture

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
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

type coordinatedRecapMeeting struct {
	fakeMeeting
	waitForAudio    bool
	captionsStarted chan struct{}
	audioStarted    chan struct{}
}

func (m *coordinatedRecapMeeting) EnableCaptions(ctx context.Context, sink browser.CaptionSink) error {
	close(m.captionsStarted)
	if m.waitForAudio {
		select {
		case <-m.audioStarted:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.fakeMeeting.EnableCaptions(ctx, sink)
}

func (m *coordinatedRecapMeeting) CaptureAudio(ctx context.Context, w io.Writer) error {
	m.mark("audio")
	close(m.audioStarted)
	select {
	case <-m.captionsStarted:
	case <-ctx.Done():
		return ctx.Err()
	}
	_, err := w.Write([]byte("synthetic mp3"))
	return err
}

func TestSessionRunRecapModeHonorsCaptionsOnlyFlag(t *testing.T) {
	for _, tc := range []struct {
		name         string
		captionsOnly bool
		wantAudio    bool
	}{
		{name: "default transcript only", captionsOnly: true},
		{name: "opted in audio", wantAudio: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			s := NewSession(SessionConfig{
				OutputDir: dir, MeetingURL: "https://teams.cloud.microsoft/l/meetingrecap?threadId=1",
				BotName: "Bot", Now: fixedTime(), CaptionsOnly: tc.captionsOnly,
			})
			s.now = fixedTime
			m := &coordinatedRecapMeeting{
				fakeMeeting: fakeMeeting{
					isRecap:   true,
					events:    []captionPair{{speaker: "Alice", text: "Recap line."}},
					aiSummary: "Synthetic meeting summary",
				},
				waitForAudio:    tc.wantAudio,
				captionsStarted: make(chan struct{}),
				audioStarted:    make(chan struct{}),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := s.Run(ctx, m); err != nil {
				t.Fatalf("run recap: %v", err)
			}
			audioCalled := false
			for _, call := range m.calls() {
				audioCalled = audioCalled || call == "audio"
			}
			if audioCalled != tc.wantAudio {
				t.Errorf("CaptureAudio called = %v, want %v", audioCalled, tc.wantAudio)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read outputs: %v", err)
			}
			var hasMD, hasMP3 bool
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".md") {
					hasMD = true
					b, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
					if readErr != nil {
						t.Fatalf("read transcript: %v", readErr)
					}
					if !strings.Contains(string(b), "Recap line.") || !strings.Contains(string(b), "Synthetic meeting summary") {
						t.Errorf("transcript lacks recap or summary: %s", b)
					}
				}
				if strings.HasSuffix(entry.Name(), ".mp3") {
					hasMP3 = true
					b, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
					if readErr != nil {
						t.Fatalf("read audio: %v", readErr)
					}
					if string(b) != "synthetic mp3" {
						t.Errorf("unexpected audio output: %q", b)
					}
				}
			}
			if !hasMD || hasMP3 != tc.wantAudio {
				t.Errorf("outputs: markdown=%v mp3=%v, want markdown=true mp3=%v", hasMD, hasMP3, tc.wantAudio)
			}
		})
	}
}

// failWriter fails every Write with err.
type failWriter struct{ err error }

func (f failWriter) Write(_ []byte) (int, error) { return 0, f.err }
