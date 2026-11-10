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

	captionsOnly := s.cfg.CaptionsOnly
	if rm, ok := m.(browser.RecapMeeting); ok && rm.IsRecap() {
		captionsOnly = true
	}

	outCfg := OutputConfig{
		Dir:              s.cfg.OutputDir,
		FilePath:         s.cfg.OutputFile,
		FilenameTemplate: s.cfg.FilenameTemplate,
		MeetingURL:       s.cfg.MeetingURL,
		Category:         s.cfg.Category,
		BotName:          s.cfg.BotName,
		Now:              s.cfg.Now,
		CaptionsOnly:     captionsOnly,
	}
	if mp, ok := m.(browser.MetadataProvider); ok {
		meta := mp.MeetingMetadata()
		outCfg.MeetingTitle = meta.Title
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
		ts := s.now()
		if idx := strings.Index(speaker, browser.RecapOffsetSeparator); idx >= 0 {
			offsetStr := speaker[idx+len(browser.RecapOffsetSeparator):]
			speaker = speaker[:idx]
			if parsedTS, ok := browser.ParseRecapTimestamp(offsetStr, ts); ok {
				ts = parsedTS
			}
		} else {
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
		if e := m.EnableCaptions(runCtx, sink); e != nil && !isShutdown(ctx, e) {
			setErr(fmt.Errorf("capture: captions: %w", e))
		}
	}()
	if !captionsOnly {
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
	if result == nil {
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
