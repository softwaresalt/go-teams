package transcript

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
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

// validTimezoneRe matches standard IANA timezone identifiers (such as
// "America/Los_Angeles" or "UTC") so their internal underscore is preserved
// verbatim without Markdown backslash-escaping.
var validTimezoneRe = regexp.MustCompile(`^[A-Za-z0-9_+-]+(?:/[A-Za-z0-9_+-]+)*$`)

var (
	blockquoteBulletOrRuleRe = regexp.MustCompile(`^(\s*)([-+=])`)
	blockquoteOrderedListRe  = regexp.MustCompile(`^(\s*\d+)([.)])(\s|$)`)
)

func sanitizeBlockquoteText(s string) string {
	s = sanitizeField(s)
	s = blockquoteBulletOrRuleRe.ReplaceAllString(s, `${1}\${2}`)
	return blockquoteOrderedListRe.ReplaceAllString(s, `${1}\${2}${3}`)
}

func sanitizeTimezone(tz string) string {
	trimmed := strings.TrimSpace(tz)
	if validTimezoneRe.MatchString(trimmed) {
		return trimmed
	}
	return sanitizeField(trimmed)
}

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

// Metadata is the header information written at the top of a transcript file.
type Metadata struct {
	MeetingID    string
	MeetingTitle string
	Category     string
	SourceURL    string
	StartTime    time.Time
	Timezone     string
	Attendees    []string
	BotName      string
}

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

// WriteHeader renders the Meeting Attendees and Transcript header section from
// meta.
func (w *Writer) WriteHeader(meta Metadata) error {
	title := strings.TrimSpace(meta.MeetingTitle)
	if title == "" {
		title = strings.TrimSpace(meta.MeetingID)
	}
	if title == "" {
		title = "Teams Meeting"
	}

	tz := strings.TrimSpace(meta.Timezone)
	if tz == "" && !meta.StartTime.IsZero() && meta.StartTime.Location() != nil {
		locName := meta.StartTime.Location().String()
		if locName != "" && locName != "Local" {
			tz = locName
		}
	}
	startStr := meta.StartTime.Format("2006-01-02 15:04")
	if tz != "" {
		startStr = startStr + " " + sanitizeTimezone(tz)
	}

	var sb strings.Builder
	sb.WriteString("# Meeting Attendees\n\n")
	sb.WriteString(fmt.Sprintf("**Total Attendees:** %d\n\n", len(meta.Attendees)))
	sb.WriteString(fmt.Sprintf("**Meeting Start:** %s\n\n", startStr))
	sb.WriteString(fmt.Sprintf("**Meeting:** %s\n\n", sanitizeField(title)))
	if strings.TrimSpace(meta.Category) != "" {
		sb.WriteString(fmt.Sprintf("**Category:** %s\n\n", sanitizeField(strings.TrimSpace(meta.Category))))
	}
	if src := sanitizeSourceURL(meta.SourceURL); src != "" {
		sb.WriteString(fmt.Sprintf("**Source:** [Teams meeting recap](%s)\n\n", src))
	}
	sb.WriteString("## Attendee List\n\n")
	for _, attendee := range meta.Attendees {
		name := strings.TrimSpace(attendee)
		if name == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s\n", sanitizeField(name)))
	}
	if len(meta.Attendees) > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString("---\n\n")
	sb.WriteString("## Transcript\n\n")

	if _, err := w.bw.WriteString(sb.String()); err != nil {
		return fmt.Errorf("transcript: write header: %w", err)
	}
	return nil
}

func sanitizeSourceURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	cleaned := strings.Map(controlToSpace, trimmed)
	u, err := url.Parse(cleaned)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Opaque != "" {
		return ""
	}
	if u.Hostname() == "" || strings.Contains(u.Host, "@") || strings.Contains(u.Path, "/:") {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	out := u.String()
	out = strings.ReplaceAll(out, " ", "%20")
	out = strings.ReplaceAll(out, ")", "%29")
	return out
}

// WriteEvent appends a formatted caption event to the transcript. The
// participant-controlled speaker and text are sanitized to prevent Markdown or
// HTML injection into the rendered transcript.
func (w *Writer) WriteEvent(ev CaptionEvent) error {
	line := fmt.Sprintf("**%s** (%s):\n> %s\n\n",
		sanitizeField(ev.Speaker), ev.Timestamp.Format("15:04:05"), sanitizeBlockquoteText(ev.Text))
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
