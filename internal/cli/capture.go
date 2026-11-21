package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/capture"
	"github.com/softwaresalt/go-teams/internal/config"
)

const (
	defaultAISummaryDelay         = 30 * time.Minute
	defaultRecapEnrichmentTimeout = 2 * time.Minute
	defaultQueueFileName          = "transcription_queue.txt"
)

// meetingSession runs a capture session against a joined meeting. It is
// satisfied by *capture.Session and replaced with a fake in unit tests.
type meetingSession interface {
	Run(ctx context.Context, m browser.Meeting) error
}

type sessionMetadataProvider interface {
	LastTranscriptPath() string
	WasRecap() bool
}

type captureHooks struct {
	onLiveCallFinished   func()
	scheduleAsyncSummary func(delay time.Duration, fn func())
	acquireProfile       func() (release func(), ok bool)
}

func parseOptionalDuration(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	return time.ParseDuration(trimmed)
}

func validateCaptureLifecycles(cfg config.Config) error {
	if mode := strings.TrimSpace(cfg.CaptureMode); mode != "" && !config.IsValidCaptureMode(mode) {
		return fmt.Errorf("cli: %w: %q", config.ErrInvalidCaptureMode, mode)
	}
	for _, item := range []struct {
		name string
		raw  string
	}{
		{"duration", cfg.Duration},
		{"idle_timeout", cfg.IdleTimeout},
		{"lobby_timeout", cfg.LobbyTimeout},
	} {
		if trimmed := strings.TrimSpace(item.raw); trimmed != "" {
			d, err := time.ParseDuration(trimmed)
			if err != nil || d <= 0 {
				return fmt.Errorf("cli: %s %q: %w", item.name, item.raw, config.ErrInvalidDuration)
			}
		}
	}
	if trimmed := strings.TrimSpace(cfg.AISummaryDelay); trimmed != "" {
		d, err := time.ParseDuration(trimmed)
		if err != nil || d < 0 {
			return fmt.Errorf("cli: ai_summary_delay %q: %w", cfg.AISummaryDelay, config.ErrInvalidDuration)
		}
	}
	return nil
}

func isAISummaryDisabled(c config.Config) bool {
	if strings.TrimSpace(c.AppendAISummary) != "" {
		return false
	}
	if c.NoAISummary || strings.EqualFold(strings.TrimSpace(c.CaptureMode), config.CaptureModeEphemeralLive) {
		return true
	}
	return false
}

func effectiveAISummaryDelay(cfg config.Config, isRecap bool) time.Duration {
	if isRecap || cfg.NoAISummary {
		return 0
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.CaptureMode))
	if mode == config.CaptureModePastRecap || mode == config.CaptureModeEphemeralLive {
		return 0
	}
	if raw := strings.TrimSpace(cfg.AISummaryDelay); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			return 0
		}
		return d
	}
	if mode == config.CaptureModeLiveWithSummary {
		return defaultAISummaryDelay
	}
	return 0
}

func resolveDeferredQueuePath(cfg config.Config) string {
	if q := strings.TrimSpace(cfg.QueueFile); q != "" {
		return q
	}
	return defaultQueueFileName
}

func runAppendAISummary(ctx context.Context, d *deps, cfg config.Config, meetingURL, transcriptPath string) error {
	recapCfg := cfg
	recapCfg.CaptureMode = config.CaptureModePastRecap
	recapCfg.AppendAISummary = transcriptPath
	recapCfg.AISummaryDelay = ""
	recapCfg.NoAISummary = false
	if strings.TrimSpace(recapCfg.LobbyTimeout) == "" {
		recapCfg.LobbyTimeout = defaultRecapEnrichmentTimeout.String()
	}
	meeting, browserCtx, cleanup, err := d.newMeeting(ctx, recapCfg, meetingURL)
	if err != nil {
		return fmt.Errorf("cli: create meeting for ai summary: %w", err)
	}
	defer cleanup()
	if err := meeting.Join(browserCtx); err != nil {
		return fmt.Errorf("cli: join recap for ai summary: %w", err)
	}
	sp, ok := meeting.(browser.AISummaryProvider)
	if !ok {
		return errors.New("cli: meeting does not provide AI summary")
	}
	summary := strings.TrimSpace(sp.AISummary())
	if summary == "" {
		return errors.New("cli: AI summary not yet available")
	}
	if err := capture.AppendAISummaryToFile(transcriptPath, summary); err != nil {
		return err
	}
	slog.Info("cli: appended delayed AI summary to transcript", "path", transcriptPath)
	return nil
}

// runCapture builds a browser-backed meeting, guarantees its teardown, and runs
// the capture session for meetingURL. The deferred cleanup runs on success,
// session error, and context cancellation, so no Chromium process is leaked.
func runCapture(ctx context.Context, d *deps, cfg config.Config, meetingURL string) error {
	return runCaptureWithHooks(ctx, d, cfg, meetingURL, captureHooks{})
}

func runCaptureWithHooks(ctx context.Context, d *deps, cfg config.Config, meetingURL string, hooks captureHooks) error {
	if nbRaw := strings.TrimSpace(cfg.NotBefore); nbRaw != "" {
		nb, err := time.Parse(time.RFC3339, nbRaw)
		if err != nil {
			return fmt.Errorf("cli: invalid --not-before timestamp %q: %w", nbRaw, err)
		}
		if time.Now().Before(nb) {
			return errQueueEntryNotYetDue
		}
	}
	if err := validateCaptureLifecycles(cfg); err != nil {
		if hooks.onLiveCallFinished != nil {
			hooks.onLiveCallFinished()
		}
		return err
	}
	if targetMD := strings.TrimSpace(cfg.AppendAISummary); targetMD != "" {
		if cfg.NoAISummary && isInsideQueueExecution(ctx) {
			return errQueueEntryAISummaryDisabled
		}
		if hooks.onLiveCallFinished != nil {
			defer hooks.onLiveCallFinished()
		}
		return runAppendAISummary(ctx, d, cfg, meetingURL, targetMD)
	}

	var (
		transcriptPath string
		wasRecap       bool
	)
	err := func() error {
		meeting, browserCtx, cleanup, createErr := d.newMeeting(ctx, cfg, meetingURL)
		if createErr != nil {
			return fmt.Errorf("cli: create meeting: %w", createErr)
		}
		defer cleanup()
		sess := d.newSession(cfg, meetingURL)
		if runErr := sess.Run(browserCtx, meeting); runErr != nil {
			return fmt.Errorf("cli: capture session: %w", runErr)
		}
		if smp, ok := sess.(sessionMetadataProvider); ok {
			transcriptPath = smp.LastTranscriptPath()
			wasRecap = smp.WasRecap()
		} else if rm, ok := meeting.(browser.RecapMeeting); ok && rm.IsRecap() {
			wasRecap = true
		}
		if transcriptPath == "" && strings.TrimSpace(cfg.OutputFile) != "" {
			transcriptPath = strings.TrimSpace(cfg.OutputFile)
		}
		return nil
	}()
	if hooks.onLiveCallFinished != nil {
		hooks.onLiveCallFinished()
	}
	if err != nil {
		return err
	}

	delay := effectiveAISummaryDelay(cfg, wasRecap)
	if wasRecap || delay <= 0 || strings.TrimSpace(transcriptPath) == "" {
		return nil
	}

	queuePath := resolveDeferredQueuePath(cfg)
	if queuePath != "" {
		if qErr := appendDeferredAISummaryQueueLineWithConfig(queuePath, meetingURL, transcriptPath, time.Now().Add(delay), cfg); qErr != nil {
			slog.Warn("cli: failed to persist deferred AI summary queue entry", "queue", queuePath, "err", qErr)
		}
	}

	execEnrichment := func() {
		if hooks.acquireProfile != nil {
			release, ok := hooks.acquireProfile()
			if !ok {
				slog.Warn("cli: skipping in-process AI summary fetch because profile is busy; entry remains in queue file", "transcript", transcriptPath)
				return
			}
			defer release()
		}
		if appErr := runAppendAISummary(ctx, d, cfg, meetingURL, transcriptPath); appErr != nil {
			slog.Warn("cli: delayed AI summary fetch did not succeed; entry remains in queue file", "transcript", transcriptPath, "err", appErr)
			return
		}
		if queuePath != "" {
			if mErr := markDeferredAISummaryQueueLineDone(queuePath, transcriptPath); mErr != nil {
				slog.Warn("cli: failed to mark deferred AI summary queue entry done", "queue", queuePath, "err", mErr)
			}
		}
	}

	if hooks.scheduleAsyncSummary != nil {
		hooks.scheduleAsyncSummary(delay, execEnrichment)
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil
	case <-timer.C:
		execEnrichment()
		return nil
	}
}
