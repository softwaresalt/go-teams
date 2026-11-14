package capture

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/transcript"
)

const (
	// DefaultIdleTimeout is the default inactivity grace period after nominal
	// Duration or in-browser call-end teardown before stopping live capture.
	DefaultIdleTimeout = 3 * time.Minute
	// DefaultPollInterval is the default watchdog check interval during live
	// capture.
	DefaultPollInterval = 250 * time.Millisecond
)

// SessionConfig configures a capture Session.
type SessionConfig struct {
	// OutputDir is where the transcript and audio files are written when
	// OutputFile is not set.
	OutputDir string
	// OutputFile is an optional explicit destination path for the Markdown
	// transcript file.
	OutputFile string
	// FilenameTemplate is the naming pattern used when writing to OutputDir.
	FilenameTemplate string
	// MeetingURL is the mutated Teams meeting URL (used for output naming).
	MeetingURL string
	// MeetingTitle is an optional fallback meeting title used for filename and
	// header generation when the DOM does not supply a richer title.
	MeetingTitle string
	// Category is an optional meeting category written to the transcript header.
	Category string
	// BotName is written into the transcript header when applicable.
	BotName string
	// Now is the capture start time used for output naming; a zero value
	// defaults to time.Now at setup.
	Now time.Time
	// CaptionsOnly records only the live captions to Markdown and skips tab
	// audio capture.
	CaptionsOnly bool
	// Mode is the capture protocol ("auto", "ephemeral-live",
	// "live-with-summary", or "past-recap").
	Mode string
	// Duration is the nominal scheduled capture window before post-duration
	// inactivity monitoring begins.
	Duration time.Duration
	// IdleTimeout is the post-duration or post-teardown silence window before
	// stopping live capture.
	IdleTimeout time.Duration
	// PollInterval overrides DefaultPollInterval when > 0.
	PollInterval time.Duration
}

// Session orchestrates a single meeting capture end to end: join, then
// concurrent caption and audio recording until the context is cancelled.
type Session struct {
	cfg                SessionConfig
	setupOutput        func(OutputConfig) (*Output, error)
	now                func() time.Time
	mu                 sync.Mutex
	lastActivity       time.Time
	callEnded          bool
	callEndedAt        time.Time
	lastTranscriptPath string
	wasRecap           bool
}

// LastTranscriptPath returns the resolved Markdown transcript path from the
// most recent Run invocation.
func (s *Session) LastTranscriptPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastTranscriptPath
}

// WasRecap reports whether the most recent Run invocation operated on a finite
// Meeting Recap rather than a live call.
func (s *Session) WasRecap() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wasRecap
}

// RecordCaptionActivity records live caption activity at the current session
// clock time.
func (s *Session) RecordCaptionActivity() {
	now := s.now()
	s.mu.Lock()
	if now.After(s.lastActivity) {
		s.lastActivity = now
	}
	s.mu.Unlock()
}

// RecordAudioActivity records live PCM voice audio activity at the current
// session clock time.
func (s *Session) RecordAudioActivity() {
	now := s.now()
	s.mu.Lock()
	if now.After(s.lastActivity) {
		s.lastActivity = now
	}
	s.mu.Unlock()
}

// RecordCallEnded records an in-browser call-end teardown signal at the
// current session clock time.
func (s *Session) RecordCallEnded() {
	now := s.now()
	s.mu.Lock()
	if !s.callEnded {
		s.callEnded = true
		s.callEndedAt = now
	}
	s.mu.Unlock()
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
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
	if am, ok := m.(browser.ActivityMonitor); ok {
		am.SetActivityCallbacks(s.RecordCaptionActivity, s.RecordAudioActivity, s.RecordCallEnded)
	}
	if joinErr := m.Join(ctx); joinErr != nil {
		if isShutdown(ctx, joinErr) {
			return nil
		}
		return fmt.Errorf("capture: join: %w", joinErr)
	}

	isRecap := false
	if rm, ok := m.(browser.RecapMeeting); ok && rm.IsRecap() {
		isRecap = true
	}
	startTime := s.now()
	s.mu.Lock()
	s.wasRecap = isRecap
	if s.lastActivity.IsZero() || startTime.After(s.lastActivity) {
		s.lastActivity = startTime
	}
	s.mu.Unlock()

	captionsOnly := s.cfg.CaptionsOnly

	outCfg := OutputConfig{
		Dir:              s.cfg.OutputDir,
		FilePath:         s.cfg.OutputFile,
		FilenameTemplate: s.cfg.FilenameTemplate,
		MeetingURL:       s.cfg.MeetingURL,
		MeetingTitle:     s.cfg.MeetingTitle,
		Category:         s.cfg.Category,
		BotName:          s.cfg.BotName,
		Now:              s.cfg.Now,
		CaptionsOnly:     captionsOnly,
	}
	if mp, ok := m.(browser.MetadataProvider); ok {
		meta := mp.MeetingMetadata()
		if strings.TrimSpace(meta.Title) != "" {
			outCfg.MeetingTitle = meta.Title
		}
		outCfg.Timezone = meta.Timezone
		outCfg.Attendees = meta.Attendees
		if !meta.StartTime.IsZero() {
			outCfg.MeetingStart = meta.StartTime
		}
	}

	out, err := s.setupOutput(outCfg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.lastTranscriptPath = out.TranscriptPath
	s.mu.Unlock()
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
		mu              sync.Mutex
		firstErr        error
		watchdogStopped atomic.Bool
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

	if !isRecap {
		effectiveIdleTimeout := s.cfg.IdleTimeout
		if effectiveIdleTimeout <= 0 {
			effectiveIdleTimeout = DefaultIdleTimeout
		}
		pollInterval := s.cfg.PollInterval
		if pollInterval <= 0 {
			pollInterval = DefaultPollInterval
		}
		go func() {
			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-runCtx.Done():
					return
				case <-ticker.C:
				}
				now := s.now()
				s.mu.Lock()
				lastAct := s.lastActivity
				ended := s.callEnded
				endedAt := s.callEndedAt
				s.mu.Unlock()

				if ended {
					ref := maxTime(endedAt, lastAct)
					if now.Sub(ref) >= effectiveIdleTimeout {
						slog.Info("capture: call ended and idle timeout elapsed; stopping capture")
						watchdogStopped.Store(true)
						cancel()
						return
					}
				}
				if s.cfg.Duration > 0 && now.Sub(startTime) >= s.cfg.Duration {
					if now.Sub(lastAct) >= effectiveIdleTimeout {
						slog.Info("capture: nominal duration and idle timeout elapsed; stopping capture")
						watchdogStopped.Store(true)
						cancel()
						return
					}
				}
			}
		}()
	}

	sink := func(speaker, text string) {
		ts := s.now()
		if idx := strings.Index(speaker, browser.RecapOffsetSeparator); idx >= 0 {
			offsetStr := speaker[idx+len(browser.RecapOffsetSeparator):]
			speaker = speaker[:idx]
			if parsedTS, ok := browser.ParseRecapTimestamp(offsetStr, ts); ok {
				ts = parsedTS
			}
		} else {
			s.RecordCaptionActivity()
			slog.Info("capture: caption", "speaker", speaker, "text", text)
		}
		if writeErr := out.Transcript.WriteEvent(transcript.CaptionEvent{
			Timestamp: ts,
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
		if e := m.EnableCaptions(runCtx, sink); e != nil && !isShutdown(ctx, e) && !(watchdogStopped.Load() && isShutdown(runCtx, e)) {
			setErr(fmt.Errorf("capture: captions: %w", e))
		}
	}()
	if !captionsOnly {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := m.CaptureAudio(runCtx, out.Audio); e != nil && !isShutdown(ctx, e) && !(watchdogStopped.Load() && isShutdown(runCtx, e)) {
				setErr(fmt.Errorf("capture: audio: %w", e))
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	result := firstErr
	mu.Unlock()
	if result == nil {
		skipSyncSummary := !isRecap && strings.EqualFold(strings.TrimSpace(s.cfg.Mode), "ephemeral-live")
		if !skipSyncSummary {
			if sp, ok := m.(browser.AISummaryProvider); ok {
				if summary := sp.AISummary(); summary != "" {
					if writeErr := out.Transcript.WriteAISummary(summary); writeErr != nil {
						return fmt.Errorf("capture: write ai summary: %w", writeErr)
					}
					if flushErr := out.Transcript.Flush(); flushErr != nil {
						return fmt.Errorf("capture: flush ai summary: %w", flushErr)
					}
					slog.Info("capture: appended AI summary to transcript")
				}
			}
		}
		if out.TranscriptPath != "" {
			slog.Info("capture: transcript complete", "path", out.TranscriptPath)
		}
	}
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
