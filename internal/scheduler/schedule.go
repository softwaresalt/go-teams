// Package scheduler builds cron schedules that drive automated Teams meeting
// joins.
package scheduler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

// ErrNoEntries is returned when a schedule has no meetings configured.
var ErrNoEntries = errors.New("scheduler: no meeting entries configured")

// Entry pairs a meeting URL with the cron expression or one-shot timestamp
// that triggers a join, plus optional per-entry capture overrides.
type Entry struct {
	Name           string
	URL            string
	Cron           string
	At             string
	Mode           string
	Duration       time.Duration
	IdleTimeout    time.Duration
	AISummaryDelay *time.Duration
	LobbyTimeout   time.Duration
	Category       string
	CaptureAudio   *bool
	OutputDir      string
	ValidateURL    func(string) error
}

// JobFunc is invoked when an entry's cron or one-shot trigger fires.
type JobFunc func(Entry)

// ParseAt parses a one-shot timestamp in RFC 3339 or local "2006-01-02 15:04"
// format using loc (defaulting to time.Local when loc is nil).
func ParseAt(raw string, loc *time.Location) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, errors.New("scheduler: empty at timestamp")
	}
	if loc == nil {
		loc = time.Local
	}
	if ts, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return ts, nil
	}
	ts, err := time.ParseInLocation("2006-01-02 15:04", trimmed, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("scheduler: invalid at timestamp %q (expected RFC3339 or YYYY-MM-DD HH:MM): %w", raw, err)
	}
	return ts, nil
}

func validateBasicHTTPURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("url must use http or https with a non-empty host")
	}
	return nil
}

// Validate checks that the entry has a valid URL, non-negative durations, and
// exactly one trigger specification (Cron or At).
func (e Entry) Validate() error {
	if strings.TrimSpace(e.URL) == "" {
		return fmt.Errorf("scheduler: entry %q: missing url", e.Name)
	}
	if e.ValidateURL != nil {
		if err := e.ValidateURL(e.URL); err != nil {
			return fmt.Errorf("scheduler: entry %q: invalid url: %w", e.Name, err)
		}
	} else if err := validateBasicHTTPURL(e.URL); err != nil {
		return fmt.Errorf("scheduler: entry %q: invalid url: %w", e.Name, err)
	}
	if e.Duration < 0 {
		return fmt.Errorf("scheduler: entry %q: duration must not be negative", e.Name)
	}
	if e.IdleTimeout < 0 {
		return fmt.Errorf("scheduler: entry %q: idle_timeout must not be negative", e.Name)
	}
	if e.LobbyTimeout < 0 {
		return fmt.Errorf("scheduler: entry %q: lobby_timeout must not be negative", e.Name)
	}
	if e.AISummaryDelay != nil && *e.AISummaryDelay < 0 {
		return fmt.Errorf("scheduler: entry %q: ai_summary_delay must not be negative", e.Name)
	}
	hasCron := strings.TrimSpace(e.Cron) != ""
	hasAt := strings.TrimSpace(e.At) != ""
	if hasCron == hasAt {
		return fmt.Errorf("scheduler: entry %q: must specify exactly one of cron or at", e.Name)
	}
	if hasCron {
		if _, err := cron.ParseStandard(e.Cron); err != nil {
			return fmt.Errorf("scheduler: entry %q: invalid cron %q: %w", e.Name, e.Cron, err)
		}
		return nil
	}
	if _, err := ParseAt(e.At, time.Local); err != nil {
		return fmt.Errorf("scheduler: entry %q: %w", e.Name, err)
	}
	return nil
}

type oneShotSchedule struct {
	at    time.Time
	fired atomic.Bool
}

func (s *oneShotSchedule) Next(t time.Time) time.Time {
	if s.fired.Load() || t.After(s.at) {
		return time.Time{}
	}
	return s.at
}

// Build validates entries and constructs a cron scheduler that invokes job for
// each entry on its schedule. Additional cron options (for example a job
// wrapper chain) may be supplied via opts. The returned scheduler is not
// started; call Start on it to begin dispatching. Build returns an error if
// there are no entries or if any entry is invalid.
func Build(entries []Entry, job JobFunc, opts ...cron.Option) (*cron.Cron, error) {
	return BuildWithClock(entries, job, time.Now, opts...)
}

// BuildWithClock validates entries and constructs a cron scheduler using the
// provided clock for evaluating one-shot At triggers.
func BuildWithClock(entries []Entry, job JobFunc, now func() time.Time, opts ...cron.Option) (*cron.Cron, error) {
	if len(entries) == 0 {
		return nil, ErrNoEntries
	}
	if now == nil {
		now = time.Now
	}
	c := cron.New(opts...)
	currentNow := now()
	loc := currentNow.Location()
	if loc == nil {
		loc = time.Local
	}
	for _, e := range entries {
		if err := e.Validate(); err != nil {
			return nil, err
		}
		entryCopy := e
		if strings.TrimSpace(e.Cron) != "" {
			if _, err := c.AddFunc(e.Cron, func() { job(entryCopy) }); err != nil {
				return nil, fmt.Errorf("scheduler: add entry %q: %w", e.Name, err)
			}
			continue
		}
		targetTime, err := ParseAt(e.At, loc)
		if err != nil {
			return nil, fmt.Errorf("scheduler: entry %q: %w", e.Name, err)
		}
		if !targetTime.After(currentNow) {
			slog.Info("scheduler: skipping past one-shot meeting", "name", e.Name, "at", e.At)
			continue
		}
		sched := &oneShotSchedule{at: targetTime}
		c.Schedule(sched, cron.FuncJob(func() {
			if sched.fired.CompareAndSwap(false, true) {
				job(entryCopy)
			}
		}))
	}
	return c, nil
}
