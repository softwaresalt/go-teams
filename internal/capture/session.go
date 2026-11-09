package capture

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/transcript"
)

// SessionConfig configures a capture Session.
type SessionConfig struct {
	// OutputDir is where the transcript and audio files are written.
	OutputDir string
	// MeetingURL is the mutated Teams meeting URL (used for output naming).
	MeetingURL string
	// BotName is written into the transcript frontmatter header.
	BotName string
	// Now is the capture start time used for output naming; a zero value
	// defaults to time.Now at setup.
	Now time.Time
	// CaptionsOnly records only the live captions to Markdown and skips tab
	// audio capture.
	CaptionsOnly bool
}

// Session orchestrates a single meeting capture end to end: join, then
// concurrent caption and audio recording until the context is cancelled.
type Session struct {
	cfg         SessionConfig
	setupOutput func(OutputConfig) (*Output, error)
	now         func() time.Time
}

// NewSession returns a Session that writes output under cfg.OutputDir.
func NewSession(cfg SessionConfig) *Session {
	return &Session{
		cfg:         cfg,
		setupOutput: setupOutput,
		now:         time.Now,
	}
}

// Run joins the meeting and then records captions and audio concurrently until
// ctx is cancelled. Output files are created only after Join succeeds, so a
// failed admission leaves no artifacts. A context-cancellation error caused by
// ctx (normal shutdown or meeting end) is reported as graceful completion; the
// first genuine caption-write, caption, or audio error is returned instead.
func (s *Session) Run(ctx context.Context, m browser.Meeting) (err error) {
	if joinErr := m.Join(ctx); joinErr != nil {
		if isShutdown(ctx, joinErr) {
			return nil
		}
		return fmt.Errorf("capture: join: %w", joinErr)
	}

	out, err := s.setupOutput(OutputConfig{
		Dir:          s.cfg.OutputDir,
		MeetingURL:   s.cfg.MeetingURL,
		BotName:      s.cfg.BotName,
		Now:          s.cfg.Now,
		CaptionsOnly: s.cfg.CaptionsOnly,
	})
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if out.TranscriptPath != "" {
		slog.Info("capture: saving transcript", "path", out.TranscriptPath)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu       sync.Mutex
		firstErr error
	)
	setErr := func(e error) {
		if e == nil {
			return
		}
		mu.Lock()
		if firstErr == nil {
			firstErr = e
		}
		mu.Unlock()
		cancel()
	}

	sink := func(speaker, text string) {
		slog.Info("capture: caption", "speaker", speaker, "text", text)
		if writeErr := out.Transcript.WriteEvent(transcript.CaptionEvent{
			Timestamp: s.now(),
			Speaker:   speaker,
			Text:      text,
		}); writeErr != nil {
			setErr(fmt.Errorf("capture: transcript write: %w", writeErr))
			return
		}
		if flushErr := out.Transcript.Flush(); flushErr != nil {
			setErr(fmt.Errorf("capture: transcript flush: %w", flushErr))
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if e := m.EnableCaptions(runCtx, sink); e != nil && !isShutdown(ctx, e) {
			setErr(fmt.Errorf("capture: captions: %w", e))
		}
	}()
	if !s.cfg.CaptionsOnly {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := m.CaptureAudio(runCtx, out.Audio); e != nil && !isShutdown(ctx, e) {
				setErr(fmt.Errorf("capture: audio: %w", e))
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	result := firstErr
	mu.Unlock()
	return result
}

// isShutdown reports whether err is a context-cancellation error caused by ctx
// being cancelled (graceful shutdown or meeting end) or by the user closing the
// browser window rather than a genuine capture failure.
func isShutdown(ctx context.Context, err error) bool {
	if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return true
	}
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "target closed") {
		return true
	}
	return false
}
