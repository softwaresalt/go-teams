// Package capture orchestrates a single live meeting capture: it joins a
// browser.Meeting, then records captions to a Markdown transcript and audio to
// an MP3 file until the supplied context is cancelled.
package capture

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/softwaresalt/go-teams/internal/transcript"
)

const (
	maxMeetingIDLen = 64
	dirPerm         = 0o755
	timestampLayout = "20060102-150405"
)

var (
	meetingIDToken = regexp.MustCompile(`meeting_[A-Za-z0-9]+`)
	unsafeIDChars  = regexp.MustCompile(`[^a-z0-9]+`)
)

// OutputConfig configures creation of the transcript+audio file pair.
type OutputConfig struct {
	// Dir is the target directory for the output files; it is created if absent.
	Dir string
	// MeetingURL is used to derive the sanitized meeting id in the file names.
	MeetingURL string
	// BotName is written into the transcript frontmatter header.
	BotName string
	// Now is the capture start time used for naming and the header; a zero
	// value defaults to time.Now.
	Now time.Time
}

// Output is the open transcript+audio file pair for a capture session.
type Output struct {
	// Transcript is the buffered Markdown transcript writer.
	Transcript *transcript.Writer
	// Audio is the destination for encoded MP3 bytes.
	Audio io.Writer
	// TranscriptPath and AudioPath are the absolute output file paths.
	TranscriptPath string
	AudioPath      string

	mdFile  *os.File
	mp3File *os.File
}

// setupOutput creates the output directory and the transcript+MP3 file pair,
// writes the transcript frontmatter header, and returns the open Output. On any
// error every file and handle created so far is rolled back so no partial
// artifacts remain (the pair is created atomically).
func setupOutput(cfg OutputConfig) (*Output, error) {
	now := cfg.Now
	if now.IsZero() {
		now = time.Now()
	}
	id := sanitizeMeetingID(cfg.MeetingURL)
	base := fmt.Sprintf("%s-%s", id, now.Format(timestampLayout))

	if err := os.MkdirAll(cfg.Dir, dirPerm); err != nil {
		return nil, fmt.Errorf("capture: create output dir %s: %w", cfg.Dir, err)
	}
	mdPath := filepath.Join(cfg.Dir, base+".md")
	mp3Path := filepath.Join(cfg.Dir, base+".mp3")

	mdFile, err := os.Create(mdPath)
	if err != nil {
		return nil, fmt.Errorf("capture: create transcript %s: %w", mdPath, err)
	}
	w := transcript.NewWriter(mdFile)
	if err := w.WriteHeader(transcript.Metadata{MeetingID: id, StartTime: now, BotName: cfg.BotName}); err != nil {
		_ = mdFile.Close()
		_ = os.Remove(mdPath)
		return nil, fmt.Errorf("capture: write transcript header: %w", err)
	}
	mp3File, err := os.Create(mp3Path)
	if err != nil {
		_ = mdFile.Close()
		_ = os.Remove(mdPath)
		return nil, fmt.Errorf("capture: create audio %s: %w", mp3Path, err)
	}
	return &Output{
		Transcript:     w,
		Audio:          mp3File,
		TranscriptPath: mdPath,
		AudioPath:      mp3Path,
		mdFile:         mdFile,
		mp3File:        mp3File,
	}, nil
}

// Close flushes the buffered transcript and closes both files. Errors from each
// step are joined so a failure on one file still attempts to close the other.
func (o *Output) Close() error {
	var errs []error
	if o.Transcript != nil {
		if err := o.Transcript.Flush(); err != nil {
			errs = append(errs, err)
		}
	}
	if o.mdFile != nil {
		if err := o.mdFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("capture: close transcript: %w", err))
		}
	}
	if o.mp3File != nil {
		if err := o.mp3File.Close(); err != nil {
			errs = append(errs, fmt.Errorf("capture: close audio: %w", err))
		}
	}
	return errors.Join(errs...)
}

// sanitizeMeetingID derives a filesystem-safe identifier from a Teams meeting
// URL. It prefers the embedded "meeting_..." token when present, lowercases the
// value, replaces every unsafe run with a single hyphen, and caps the length.
// The result always matches [a-z0-9-]+ and never contains path separators or
// "..", so it is safe to join onto the output directory.
func sanitizeMeetingID(rawURL string) string {
	candidate := rawURL
	if decoded, err := url.QueryUnescape(rawURL); err == nil {
		candidate = decoded
	}
	if token := meetingIDToken.FindString(candidate); token != "" {
		candidate = token
	}
	candidate = strings.ToLower(candidate)
	candidate = unsafeIDChars.ReplaceAllString(candidate, "-")
	candidate = strings.Trim(candidate, "-")
	if len(candidate) > maxMeetingIDLen {
		candidate = strings.Trim(candidate[:maxMeetingIDLen], "-")
	}
	if candidate == "" {
		return "meeting"
	}
	return candidate
}
