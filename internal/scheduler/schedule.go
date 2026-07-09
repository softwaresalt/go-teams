// Package scheduler builds cron schedules that drive automated Teams meeting
// joins.
package scheduler

import (
	"errors"
	"fmt"

	"github.com/robfig/cron/v3"
)

// ErrNoEntries is returned when a schedule has no meetings configured.
var ErrNoEntries = errors.New("scheduler: no meeting entries configured")

// Entry pairs a meeting URL with the cron expression that triggers a join.
type Entry struct {
	Name string
	URL  string
	Cron string
}

// JobFunc is invoked when an entry's cron trigger fires.
type JobFunc func(Entry)

// Validate parses the entry's cron expression with the standard 5-field cron
// parser and checks that a URL is present.
func (e Entry) Validate() error {
	if e.URL == "" {
		return fmt.Errorf("scheduler: entry %q: missing url", e.Name)
	}
	if _, err := cron.ParseStandard(e.Cron); err != nil {
		return fmt.Errorf("scheduler: entry %q: invalid cron %q: %w", e.Name, e.Cron, err)
	}
	return nil
}

// Build validates entries and constructs a cron scheduler that invokes job for
// each entry on its schedule. The returned scheduler is not started; call Start
// on it to begin dispatching. Build returns an error if there are no entries or
// if any entry is invalid.
func Build(entries []Entry, job JobFunc) (*cron.Cron, error) {
	if len(entries) == 0 {
		return nil, ErrNoEntries
	}
	c := cron.New()
	for _, e := range entries {
		if err := e.Validate(); err != nil {
			return nil, err
		}
		if _, err := c.AddFunc(e.Cron, func() { job(e) }); err != nil {
			return nil, fmt.Errorf("scheduler: add entry %q: %w", e.Name, err)
		}
	}
	return c, nil
}
