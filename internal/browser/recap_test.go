package browser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRecapTimestamp(t *testing.T) {
	base := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"iso seconds only", "PT0M19S", "00:00:19"},
		{"iso minutes and seconds", "PT15M29S", "00:15:29"},
		{"iso hours minutes seconds", "PT1H2M3S", "01:02:03"},
		{"colon mm:ss", "3:07", "00:03:07"},
		{"colon hh:mm:ss", "01:12:45", "01:12:45"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseRecapTimestamp(tc.raw, base)
			require.True(t, ok)
			assert.Equal(t, tc.want, got.Format("15:04:05"))
		})
	}
}

func TestParseRecapItemsJSON(t *testing.T) {
	raw := `{
		"entries": [
			{"speaker": "John Seitzinger", "timestamp": "PT0M3S", "text": "", "type": "StartEvent"},
			{"speaker": "John Seitzinger", "timestamp": "PT0M19S", "text": "Good morning.", "type": "Text"},
			{"speaker": "Derek Williams", "timestamp": "PT0M20S", "text": "Hello there.", "type": "Text"}
		]
	}`
	entries, err := parseRecapEntriesPayload(raw)
	require.NoError(t, err)
	require.Len(t, entries, 2, "StartEvent with empty text should be filtered out")
	assert.Equal(t, "John Seitzinger", entries[0].Speaker)
	assert.Equal(t, "00:00:19", entries[0].Offset)
	assert.Equal(t, "Good morning.", entries[0].Text)
	assert.Equal(t, "Derek Williams", entries[1].Speaker)
	assert.Equal(t, "00:00:20", entries[1].Offset)
	assert.Equal(t, "Hello there.", entries[1].Text)
}

func TestCleanHeadedBrowserFlags(t *testing.T) {
	args := cleanHeadedBrowserArgs(9222, `/tmp/profile`, "https://teams.cloud.microsoft")
	for _, a := range args {
		assert.NotContains(t, a, "AutomationControlled",
			"headed browser must not pass --disable-blink-features=AutomationControlled")
		assert.NotContains(t, a, "no-sandbox",
			"headed browser must not disable the browser sandbox")
		assert.NotContains(t, a, "headless",
			"headed browser must not include --headless")
	}
	assert.Contains(t, args, "--remote-debugging-port=9222")
}

func TestCleanHeadlessBrowserFlags(t *testing.T) {
	args := cleanBrowserArgs(9222, `/tmp/profile`, true, "https://teams.cloud.microsoft")
	for _, a := range args {
		assert.NotContains(t, a, "AutomationControlled",
			"headless browser must not pass --disable-blink-features=AutomationControlled")
		assert.NotContains(t, a, "no-sandbox",
			"headless browser must not disable the browser sandbox")
	}
	assert.Contains(t, args, "--remote-debugging-port=9222")
	assert.Contains(t, args, "--headless=new")
}

func TestCopyAISummaryJSStructure(t *testing.T) {
	assert.Contains(t, copyAISummaryJS, `data-tid="Notes"`)
	assert.Contains(t, copyAISummaryJS, `data-tid="AINotes"`)
	assert.Contains(t, copyAISummaryJS, `aria-label="Copy all"`)
	assert.Contains(t, copyAISummaryJS, `navigator.clipboard`)
}
