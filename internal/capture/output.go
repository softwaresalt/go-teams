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
	// filePerm keeps each transcript/audio file owner-only (they may contain
	// confidential meeting content). dirPerm applies only when this code creates
	// the output directory; a pre-existing directory keeps its own permissions.
	dirPerm  = 0o700
	filePerm = 0o600
	// timestampLayout includes milliseconds so concurrent captures that share a
	// sanitized meeting id do not collide on the same output file name.
	timestampLayout = "20060102-150405.000"
)

var (
	meetingIDToken = regexp.MustCompile(`meeting_[A-Za-z0-9]+`)
	meetPathToken  = regexp.MustCompile(`(?i)/meet/([A-Za-z0-9_-]+)`)
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
	// CaptionsOnly skips creating the MP3 audio output file when only live
	// captions are being captured to Markdown.
	CaptionsOnly bool
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
// writes the transcript frontmatter header, and returns the open Output. Files
// are created with O_EXCL so an existing file is never silently truncated; a
// name collision (same meeting id in the same millisecond) surfaces as an error
// instead of corrupting a concurrent capture. On any error every file and
// handle created so far is rolled back so no partial artifacts remain (the pair
// is created atomically).
func setupOutput(cfg OutputConfig) (*Output, error) {
	now := cfg.Now
	if now.IsZero() {
		now = time.Now()
	}
	id := sanitizeMeetingID(cfg.MeetingURL)
	base := fmt.Sprintf("%s-%s", id, now.Format(timestampLayout))

	dir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("capture: resolve output dir %s: %w", cfg.Dir, err)
	}
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("capture: create output dir %s: %w", dir, err)
	}
	mdPath := filepath.Join(dir, base+".md")
	mp3Path := filepath.Join(dir, base+".mp3")

	mdFile, err := os.OpenFile(mdPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, filePerm)
	if err != nil {
		return nil, fmt.Errorf("capture: create transcript %s: %w", mdPath, err)
	}
	w := transcript.NewWriter(mdFile)
	if err := w.WriteHeader(transcript.Metadata{MeetingID: id, StartTime: now, BotName: cfg.BotName}); err != nil {
		// Best-effort rollback of the partial transcript; the header-write
		// error is the failure worth surfacing to the caller.
		_ = mdFile.Close()
		_ = os.Remove(mdPath)
		return nil, fmt.Errorf("capture: write transcript header: %w", err)
	}
	// Flush the header immediately so a physical write failure (e.g. a full
	// filesystem) surfaces here and triggers rollback, rather than being
	// deferred inside the bufio buffer until the first caption is written.
	if err := w.Flush(); err != nil {
		_ = mdFile.Close()
		_ = os.Remove(mdPath)
		return nil, fmt.Errorf("capture: flush transcript header: %w", err)
	}
	if cfg.CaptionsOnly {
		return &Output{
			Transcript:     w,
			Audio:          io.Discard,
			TranscriptPath: mdPath,
			mdFile:         mdFile,
		}, nil
	}
	mp3File, err := os.OpenFile(mp3Path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, filePerm)
	if err != nil {
		// Best-effort rollback so a failed pair leaves no partial artifacts;
		// the audio-create error is the failure worth surfacing.
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
	} else if m := meetPathToken.FindStringSubmatch(candidate); len(m) == 2 {
		candidate = "meet-" + m[1]
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
