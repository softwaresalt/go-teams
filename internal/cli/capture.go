package cli

import (
	"context"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/config"
)

// meetingSession runs a capture session against a joined meeting. It is
// satisfied by *capture.Session and replaced with a fake in unit tests.
type meetingSession interface {
	Run(ctx context.Context, m browser.Meeting) error
}

// runCapture builds a browser-backed meeting, guarantees its teardown, and runs
// the capture session for meetingURL. The deferred cleanup runs on success,
// session error, and context cancellation, so no Chromium process is leaked.
func runCapture(ctx context.Context, d *deps, cfg config.Config, meetingURL string) error {
	meeting, browserCtx, cleanup, err := d.newMeeting(ctx, cfg, meetingURL)
	if err != nil {
		return err
	}
	defer cleanup()
	return d.newSession(cfg, meetingURL).Run(browserCtx, meeting)
}
