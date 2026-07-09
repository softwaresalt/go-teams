package scheduler

import (
	"testing"

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
