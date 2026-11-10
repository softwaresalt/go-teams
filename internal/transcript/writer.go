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
	}{
		MeetingID: meta.MeetingID,
		Date:      meta.StartTime.Format("2006-01-02"),
		StartTime: meta.StartTime.Format("15:04:05"),
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

// WriteAISummary formats and appends the Teams AI Summary (Meeting notes and
// Follow-up tasks copied from the Recap AI summary tab) to the Markdown
// transcript.
func (w *Writer) WriteAISummary(raw string) error {
	formatted := formatAISummaryMarkdown(raw)
	if formatted == "" {
		return nil
	}
	if _, err := w.bw.WriteString(formatted); err != nil {
		return fmt.Errorf("transcript: write ai summary: %w", err)
	}
	return nil
}

func formatAISummaryMarkdown(raw string) string {
	trimmedRaw := strings.TrimSpace(raw)
	if trimmedRaw == "" {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(trimmedRaw, "\r\n", "\n"), "\n")
	var sb strings.Builder
	sb.WriteString("---\n\n# AI Summary\n\n")

	ensureBlankLine := func() {
		s := sb.String()
		switch {
		case strings.HasSuffix(s, "\n\n"):
		case strings.HasSuffix(s, "\n"):
			sb.WriteString("\n")
		default:
			sb.WriteString("\n\n")
		}
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "View the full meeting recap in Teams:") {
			continue
		}
		if strings.EqualFold(trimmed, "Meeting notes:") {
			ensureBlankLine()
			sb.WriteString("## Meeting notes\n\n")
			continue
		}
		if strings.EqualFold(trimmed, "Follow-up tasks:") {
			ensureBlankLine()
			sb.WriteString("## Follow-up tasks\n\n")
			continue
		}
		if strings.HasPrefix(line, "\t") {
			if idx := strings.Index(trimmed, ":"); idx > 0 {
				title := strings.TrimSpace(trimmed[:idx])
				body := strings.TrimSpace(trimmed[idx+1:])
				sb.WriteString("  * **" + sanitizeField(title) + ":** " + sanitizeField(body) + "\n")
			} else {
				sb.WriteString("  * " + sanitizeField(trimmed) + "\n")
			}
			continue
		}
		if strings.HasSuffix(trimmed, ":") && i+1 < len(lines) {
			nextRaw := strings.TrimRight(lines[i+1], " \r")
			nextTrimmed := strings.TrimSpace(nextRaw)
			if nextTrimmed != "" &&
				!strings.HasPrefix(nextRaw, "\t") &&
				!strings.EqualFold(nextTrimmed, "Meeting notes:") &&
				!strings.EqualFold(nextTrimmed, "Follow-up tasks:") {
				heading := strings.TrimSuffix(trimmed, ":")
				sb.WriteString("* **" + sanitizeField(heading) + ":** " + sanitizeField(nextTrimmed) + "\n")
				i++
				continue
			}
		}
		ensureBlankLine()
		sb.WriteString(fmt.Sprintf("%s\n\n", sanitizeField(trimmed)))
	}
	return sb.String()
}

// Flush writes any buffered data to the underlying writer.
func (w *Writer) Flush() error {
	if err := w.bw.Flush(); err != nil {
		return fmt.Errorf("transcript: flush: %w", err)
	}
	return nil
}
