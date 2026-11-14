package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntryValidate(t *testing.T) {
	tests := []struct {
		name    string
		entry   Entry
		wantErr bool
	}{
		{"valid", Entry{Name: "a", URL: "https://x", Cron: "0 9 * * 1-5"}, false},
		{"missing url", Entry{Name: "a", Cron: "* * * * *"}, true},
		{"invalid cron", Entry{Name: "a", URL: "https://x", Cron: "not a cron"}, true},
		{"too many fields", Entry{Name: "a", URL: "https://x", Cron: "* * * * * *"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildNoEntries(t *testing.T) {
	_, err := Build(nil, func(Entry) {})
	assert.ErrorIs(t, err, ErrNoEntries)
}

func TestBuildInvalidEntry(t *testing.T) {
	_, err := Build([]Entry{{Name: "bad", URL: "https://x", Cron: "bogus"}}, func(Entry) {})
	assert.Error(t, err)
}

func TestBuildRegistersEntries(t *testing.T) {
	entries := []Entry{
		{Name: "a", URL: "https://x", Cron: "* * * * *"},
		{Name: "b", URL: "https://y", Cron: "0 9 * * 1-5"},
	}
	c, err := Build(entries, func(Entry) {})
	require.NoError(t, err)
	assert.Len(t, c.Entries(), 2)
}

func TestBuildJobDispatch(t *testing.T) {
	var called []Entry
	entries := []Entry{{Name: "standup", URL: "https://x", Cron: "* * * * *"}}
	c, err := Build(entries, func(e Entry) { called = append(called, e) })
	require.NoError(t, err)
	require.Len(t, c.Entries(), 1)

	// Invoke the wrapped job directly to verify the entry is passed through.
	c.Entries()[0].Job.Run()
	require.Len(t, called, 1)
	assert.Equal(t, "standup", called[0].Name)
}

func TestParseAtAndOneShotSchedule(t *testing.T) {
	loc := time.FixedZone("PST", -8*3600)
	ts, err := ParseAt("2026-11-15 14:30", loc)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 15, 14, 30, 0, 0, loc), ts)

	tsRFC, err := ParseAt("2026-11-15T14:30:00-08:00", nil)
	require.NoError(t, err)
	assert.True(t, ts.Equal(tsRFC))

	_, err = ParseAt("", loc)
	assert.Error(t, err)
	_, err = ParseAt("not-a-timestamp", loc)
	assert.Error(t, err)

	now := time.Date(2026, 11, 15, 14, 0, 0, 0, loc)
	var fired []string
	c, err := BuildWithClock([]Entry{
		{Name: "past", URL: "https://teams.microsoft.com/meet/1", At: "2026-11-15 13:00"},
		{Name: "future", URL: "https://teams.microsoft.com/meet/2", At: "2026-11-15T14:30:00-08:00", Duration: 45 * time.Minute},
	}, func(e Entry) { fired = append(fired, e.Name) }, func() time.Time { return now })
	require.NoError(t, err)
	require.Len(t, c.Entries(), 1)

	entry := c.Entries()[0]
	assert.True(t, ts.Equal(entry.Schedule.Next(now)))
	entry.Job.Run()
	entry.Job.Run() // second invocation must be a no-op
	assert.Equal(t, []string{"future"}, fired)
	assert.True(t, entry.Schedule.Next(now).IsZero())

	negDelay := -1 * time.Second
	invalidEntries := []Entry{
		{Name: "both", URL: "https://x", Cron: "* * * * *", At: "2026-11-15 14:30"},
		{Name: "neither", URL: "https://x"},
		{Name: "bad-at", URL: "https://x", At: "invalid"},
		{Name: "bad-url-scheme", URL: "ftp://x", Cron: "* * * * *"},
		{Name: "custom-url-validator", URL: "https://x", Cron: "* * * * *", ValidateURL: func(string) error { return assert.AnError }},
		{Name: "neg-duration", URL: "https://x", Cron: "* * * * *", Duration: -1 * time.Minute},
		{Name: "neg-idle", URL: "https://x", Cron: "* * * * *", IdleTimeout: -1 * time.Minute},
		{Name: "neg-lobby", URL: "https://x", Cron: "* * * * *", LobbyTimeout: -1 * time.Minute},
		{Name: "neg-delay", URL: "https://x", Cron: "* * * * *", AISummaryDelay: &negDelay},
	}
	for _, ie := range invalidEntries {
		t.Run(ie.Name, func(t *testing.T) {
			assert.Error(t, ie.Validate())
		})
	}
}
