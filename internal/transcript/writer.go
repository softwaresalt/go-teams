package transcript

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode"
)

// markdownEscaper backslash-escapes the runes that let participant-controlled
// caption text inject Markdown or HTML structure. Backslash is listed first so
// it is not double-escaped by the single-pass replacer.
var markdownEscaper = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	"*", `\*`,
	"_", `\_`,
	"[", `\[`,
	"]", `\]`,
	"<", `\<`,
	">", `\>`,
	"#", `\#`,
	"|", `\|`,
	"~", `\~`,
)

// autolinkDefuser matches the scheme/prefix that triggers GitHub-Flavored
// Markdown bare URL autolinking. A zero-width space is inserted after the match
// so a participant-supplied URL renders as inert text rather than a clickable
// link.
var autolinkDefuser = regexp.MustCompile(`(?i)(https?://|ftp://|www\.)`)

// emailDefuser matches GFM bare email autolinks. A zero-width space is inserted
// after the "@" so a participant-supplied address does not render as a clickable
// mailto link.
var emailDefuser = regexp.MustCompile(`([A-Za-z0-9._%+\-]+)@([A-Za-z0-9.\-]+\.[A-Za-z]{2,})`)

// controlToSpace maps control characters and Unicode line/paragraph separators
// to a space; all other runes pass through. This prevents participant text from
// breaking the transcript's line structure.
func controlToSpace(r rune) rune {
	if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
		return ' '
	}
	return r
}

// sanitizeField neutralizes participant-controlled text (speaker names and
// caption text come from meeting participants) for safe inclusion in the
// Markdown transcript: control characters and line/paragraph separators are
// replaced with spaces so they cannot break the line structure,
// Markdown-structural runes are backslash-escaped, and bare URL and email
// autolinks are defused. Ordinary words and sentence punctuation are unchanged.
func sanitizeField(s string) string {
	s = strings.Map(controlToSpace, s)
	s = markdownEscaper.Replace(s)
	s = autolinkDefuser.ReplaceAllString(s, "${1}\u200b")
	return emailDefuser.ReplaceAllString(s, "${1}@\u200b${2}")
}

// Metadata is the header information written as YAML frontmatter at the top of
// a transcript file.
type Metadata struct {
	MeetingID string
	StartTime time.Time
	BotName   string
}

const headerTemplate = `---
meeting_id: {{ printf "%q" .MeetingID }}
date: {{ .Date }}
start_time: "{{ .StartTime }}"
generated_by: teams-cli
bot_name: {{ printf "%q" .BotName }}
---

# Transcript

`

var tmpl = template.Must(template.New("header").Parse(headerTemplate))

// Writer streams caption events to an underlying writer as Markdown, using a
// buffered writer to keep the memory footprint low regardless of meeting
// length.
type Writer struct {
	bw *bufio.Writer
}

// NewWriter wraps w in a buffered transcript Writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{bw: bufio.NewWriter(w)}
}

// WriteHeader renders the YAML frontmatter header from meta.
func (w *Writer) WriteHeader(meta Metadata) error {
	data := struct {
		MeetingID string
		Date      string
		StartTime string
		BotName   string
	}{
		MeetingID: meta.MeetingID,
		Date:      meta.StartTime.Format("2006-01-02"),
		StartTime: meta.StartTime.Format("15:04:05"),
		BotName:   meta.BotName,
	}
	if err := tmpl.Execute(w.bw, data); err != nil {
		return fmt.Errorf("transcript: write header: %w", err)
	}
	return nil
}

// WriteEvent appends a formatted caption event to the transcript. The
// participant-controlled speaker and text are sanitized to prevent Markdown or
// HTML injection into the rendered transcript.
func (w *Writer) WriteEvent(ev CaptionEvent) error {
	line := fmt.Sprintf("**%s (%s):** %s\n\n",
		sanitizeField(ev.Speaker), ev.Timestamp.Format("15:04:05"), sanitizeField(ev.Text))
	if _, err := w.bw.WriteString(line); err != nil {
		return fmt.Errorf("transcript: write event: %w", err)
	}
	return nil
}

// Flush writes any buffered data to the underlying writer.
func (w *Writer) Flush() error {
	if err := w.bw.Flush(); err != nil {
		return fmt.Errorf("transcript: flush: %w", err)
	}
	return nil
}
