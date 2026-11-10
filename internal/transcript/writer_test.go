package transcript

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteEventCollapsesLineBreaks(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	require.NoError(t, w.WriteEvent(CaptionEvent{
		Timestamp: time.Date(2026, 7, 9, 13, 6, 5, 0, time.UTC),
		Speaker:   "Bob",
		Text:      "hi\r\n# Injected",
	}))
	require.NoError(t, w.Flush())

	body := strings.TrimSuffix(buf.String(), "\n\n")
	// Only the single newline separating the speaker header from the blockquote line is allowed;
	// the embedded CR/LF inside the caption text must be collapsed.
	assert.Equal(t, 1, strings.Count(body, "\n"), "embedded newline must be collapsed")
	assert.NotContains(t, body, "\r", "embedded carriage return must be collapsed")
	assert.Equal(t, "**Bob** (13:06:05):\n> hi  \\# Injected", body)
}

func TestWriteEventDefusesBareAutolink(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	require.NoError(t, w.WriteEvent(CaptionEvent{
		Timestamp: time.Date(2026, 7, 9, 13, 6, 5, 0, time.UTC),
		Speaker:   "Bob",
		Text:      "visit http://evil.example and www.evil.example",
	}))
	require.NoError(t, w.Flush())
	out := buf.String()
	// The contiguous URL must be broken by a zero-width space after the scheme.
	assert.NotContains(t, out, "http://evil.example")
	assert.Contains(t, out, "http://\u200bevil.example")
	assert.Contains(t, out, "www.\u200bevil.example")
}

func TestWriteEventEscapesMarkdownInjection(t *testing.T) {
	ts := time.Date(2026, 7, 9, 13, 6, 5, 0, time.UTC)
	tests := []struct {
		name     string
		speaker  string
		text     string
		contains []string
		absent   []string
	}{
		{
			name:     "heading and newline in text",
			speaker:  "Bob",
			text:     "hi\n# Injected Heading",
			contains: []string{`\# Injected Heading`},
			absent:   []string{"\n# "},
		},
		{
			name:     "link in text",
			speaker:  "Bob",
			text:     "[click](http://evil)",
			contains: []string{`\[click\]`},
		},
		{
			name:     "html in text",
			speaker:  "Bob",
			text:     "<script>x</script>",
			contains: []string{`\<script\>`},
		},
		{
			name:     "emphasis in speaker",
			speaker:  "**Admin**",
			text:     "ok",
			contains: []string{`\*\*Admin\*\*`},
		},
		{
			name:    "carriage return does not break the line",
			speaker: "Bob",
			text:    "a\r\n- injected list",
			absent:  []string{"\r", "\n- "},
		},
		{
			name:     "strikethrough tilde escaped",
			speaker:  "Bob",
			text:     "~~redacted~~",
			contains: []string{`\~\~redacted\~\~`},
		},
		{
			name:     "bare email defused",
			speaker:  "Bob",
			text:     "mail support@paypa1.example please",
			contains: []string{"support@\u200bpaypa1.example"},
			absent:   []string{"support@paypa1.example"},
		},
		{
			name:     "unicode line separator collapsed",
			speaker:  "Bob",
			text:     "a\u2028# heading",
			contains: []string{`\# heading`},
			absent:   []string{"\u2028"},
		},
		{
			name:    "unicode paragraph separator collapsed",
			speaker: "Bob",
			text:    "a\u2029- item",
			absent:  []string{"\u2029"},
		},
		{
			name:     "leading thematic break escaped in blockquote",
			speaker:  "Bob",
			text:     "---",
			contains: []string{"> \\---"},
			absent:   []string{"> ---"},
		},
		{
			name:     "leading unordered list dash escaped in blockquote",
			speaker:  "Bob",
			text:     "- bullet item",
			contains: []string{"> \\- bullet item"},
			absent:   []string{"> - bullet item"},
		},
		{
			name:     "leading unordered list plus escaped in blockquote",
			speaker:  "Bob",
			text:     "+ plus item",
			contains: []string{"> \\+ plus item"},
			absent:   []string{"> + plus item"},
		},
		{
			name:     "leading ordered list period escaped in blockquote",
			speaker:  "Bob",
			text:     "1. first item",
			contains: []string{"> 1\\. first item"},
			absent:   []string{"> 1. first item"},
		},
		{
			name:     "leading ordered list paren escaped in blockquote",
			speaker:  "Bob",
			text:     "2) second item",
			contains: []string{"> 2\\) second item"},
			absent:   []string{"> 2) second item"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf)
			require.NoError(t, w.WriteEvent(CaptionEvent{Timestamp: ts, Speaker: tc.speaker, Text: tc.text}))
			require.NoError(t, w.Flush())
			out := buf.String()
			for _, c := range tc.contains {
				assert.Containsf(t, out, c, "output: %q", out)
			}
			for _, a := range tc.absent {
				assert.NotContainsf(t, out, a, "output: %q", out)
			}
		})
	}
}

func TestWriterHeader(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	start := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	require.NoError(t, w.WriteHeader(Metadata{
		MeetingID:    "19:meeting_abc",
		MeetingTitle: "NVD Pacific TZ Sync",
		Category:     "WSP",
		SourceURL:    "https://teams.microsoft.com/l/meetingrecap?threadId=1",
		StartTime:    start,
		Timezone:     "America/Los_Angeles",
		Attendees:    []string{"Alice Example", "Bob Example", "Charlie Example"},
	}))
	require.NoError(t, w.Flush())

	out := buf.String()
	assert.Contains(t, out, "# Meeting Attendees\n\n")
	assert.Contains(t, out, "**Total Attendees:** 3\n\n")
	assert.Contains(t, out, "**Meeting Start:** 2026-10-23 09:00 America/Los_Angeles\n\n")
	assert.Contains(t, out, "**Meeting:** NVD Pacific TZ Sync\n\n")
	assert.Contains(t, out, "**Category:** WSP\n\n")
	assert.Contains(t, out, "**Source:** [Teams meeting recap](https://teams.microsoft.com/l/meetingrecap?threadId=1)\n\n")
	assert.Contains(t, out, "## Attendee List\n\n- Alice Example\n- Bob Example\n- Charlie Example\n\n---\n\n## Transcript\n\n")
	assert.NotContains(t, out, "Speakers identified in the transcript")
	assert.NotContains(t, out, "generated_by:")
	assert.NotContains(t, out, "bot_name:")
}

func TestWriterEvent(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	ev := CaptionEvent{
		Timestamp: time.Date(2026, 7, 9, 13, 6, 5, 0, time.UTC),
		Speaker:   "Alice",
		Text:      "The architecture requires a standalone binary.",
	}
	require.NoError(t, w.WriteEvent(ev))
	require.NoError(t, w.Flush())

	assert.Equal(t,
		"**Alice** (13:06:05):\n> The architecture requires a standalone binary.\n\n",
		buf.String())
}

func TestWriterBuffersUntilFlush(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	require.NoError(t, w.WriteHeader(Metadata{MeetingID: "x", StartTime: time.Now(), BotName: "b"}))
	assert.Zero(t, buf.Len(), "small writes stay buffered until Flush")
	require.NoError(t, w.Flush())
	assert.Positive(t, buf.Len())
}

func TestWriteAISummaryFormatsCopiedTeamsSummary(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	rawCopied := "Generated by AI. Be sure to check for accuracy.\n" +
		"View the full meeting recap in Teams: https://teams.cloud.microsoft/l/meetingrecap?secret=123.\n\n" +
		"Meeting notes:\n" +
		"Scope Alignment: \n" +
		"The team aligned on the 200-hour package.\n" +
		"\tPhased Delivery: Start with low-risk items first.\n" +
		"Follow-up tasks:\n" +
		"Work Scope: \n" +
		"Prepare a written scope document. (Presenter)\n"

	require.NoError(t, w.WriteAISummary(rawCopied))
	require.NoError(t, w.Flush())

	out := buf.String()
	assert.Contains(t, out, "---\n\n# AI Summary\n\n")
	assert.Contains(t, out, "## Meeting notes")
	assert.Contains(t, out, "* **Scope Alignment:** The team aligned on the 200-hour package.")
	assert.Contains(t, out, "  * **Phased Delivery:** Start with low-risk items first.")
	assert.Contains(t, out, "## Follow-up tasks")
	assert.Contains(t, out, "* **Work Scope:** Prepare a written scope document. (Presenter)")
	assert.NotContains(t, out, "secret=123", "recap URL with query tokens should be stripped from AI summary")
	assert.NotContains(t, out, "\n\n\n", "markdown output must not contain multiple consecutive blank lines")
}
