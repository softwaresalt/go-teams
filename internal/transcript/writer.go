package transcript

import (
	"bufio"
	"fmt"
	"io"
	"text/template"
	"time"
)

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

// WriteEvent appends a formatted caption event to the transcript.
func (w *Writer) WriteEvent(ev CaptionEvent) error {
	line := fmt.Sprintf("**%s (%s):** %s\n\n",
		ev.Speaker, ev.Timestamp.Format("15:04:05"), ev.Text)
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
