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
	// DefaultFilenameTemplate is the default transcript filename pattern used
	// when only an output directory is provided.
	DefaultFilenameTemplate = "<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md"
)

var (
	meetingIDToken  = regexp.MustCompile(`meeting_[A-Za-z0-9]+`)
	meetPathToken   = regexp.MustCompile(`(?i)/meet/([A-Za-z0-9_-]+)`)
	unsafeIDChars   = regexp.MustCompile(`[^a-z0-9]+`)
	wordTokenRe     = regexp.MustCompile(`[A-Za-z0-9]+`)
	unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	multiHyphenRe   = regexp.MustCompile(`-+`)
)

// fillerWords is the set of English articles of speech, prepositions,
// conjunctions, auxiliaries, and conversational filler words omitted when
// constructing <concat-meeting-name>.
var fillerWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {},
	"and": {}, "or": {}, "but": {}, "nor": {}, "so": {}, "yet": {},
	"of": {}, "in": {}, "on": {}, "at": {}, "to": {}, "for": {}, "with": {}, "by": {}, "from": {},
	"about": {}, "as": {}, "into": {}, "like": {}, "through": {}, "after": {}, "over": {},
	"between": {}, "out": {}, "against": {}, "during": {}, "without": {}, "before": {},
	"under": {}, "around": {}, "among": {},
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "being": {},
	"have": {}, "has": {}, "had": {}, "do": {}, "does": {}, "did": {},
	"this": {}, "that": {}, "these": {}, "those": {}, "it": {}, "its": {},
	"uh": {}, "um": {}, "er": {}, "ah": {},
}

// OutputConfig configures creation of the transcript+audio file pair.
type OutputConfig struct {
	// Dir is the target directory for the output files when FilePath is not
	// set; it is created if absent.
	Dir string
	// FilePath is an optional explicit destination path for the Markdown
	// transcript file. When set, the transcript is written directly to
	// FilePath (creating parent directories as needed).
	FilePath string
	// FilenameTemplate is the pattern used to construct the transcript file
	// name inside Dir when FilePath is empty. An empty value defaults to
	// DefaultFilenameTemplate.
	FilenameTemplate string
	// MeetingURL is used to derive the sanitized meeting id in the file names
	// and the Source link in the transcript header.
	MeetingURL string
	// MeetingTitle is the human-readable meeting subject written to the header.
	MeetingTitle string
	// Category is an optional meeting category written to the header.
	Category string
	// Timezone is the IANA timezone name (e.g. "America/Los_Angeles") written
	// alongside the meeting start time.
	Timezone string
	// Attendees is the list of speaker display names written to the header.
	Attendees []string
	// BotName is retained for compatibility with existing callers.
	BotName string
	// Now is the capture start time used for output file naming and as the
	// fallback header start time; a zero value defaults to time.Now.
	Now time.Time
	// MeetingStart is the optional discovered meeting start time written to
	// the transcript header (**Meeting Start:**) when non-zero, keeping Now as
	// the per-capture timestamp for file naming.
	MeetingStart time.Time
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

// setupOutput creates the output directory and files, writes the transcript
// header, and returns the open Output. Generated names use O_EXCL to reject
// collisions; an explicit FilePath replaces the destination on each run.
func setupOutput(cfg OutputConfig) (*Output, error) {
	now := cfg.Now
	if now.IsZero() {
		now = time.Now()
	}
	id := sanitizeMeetingID(cfg.MeetingURL)
	mdFilename := FormatTranscriptFilename(cfg.FilenameTemplate, cfg, now, id)
	base := strings.TrimSuffix(mdFilename, filepath.Ext(mdFilename))

	explicitFile := cfg.FilePath
	var (
		mdPath  string
		mp3Path string
		openFlg = os.O_CREATE | os.O_WRONLY | os.O_EXCL
	)
	if explicitFile != "" {
		absFile, err := filepath.Abs(explicitFile)
		if err != nil {
			return nil, fmt.Errorf("capture: resolve output file %s: %w", explicitFile, err)
		}
		parentDir := filepath.Dir(absFile)
		if err := os.MkdirAll(parentDir, dirPerm); err != nil {
			return nil, fmt.Errorf("capture: create output dir %s: %w", parentDir, err)
		}
		mdPath = absFile
		ext := filepath.Ext(absFile)
		if ext != "" {
			mp3Path = strings.TrimSuffix(absFile, ext) + ".mp3"
		} else {
			mp3Path = absFile + ".mp3"
		}
		// When an explicit output file path is requested (automation mode),
		// truncate/overwrite if it already exists.
		openFlg = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	} else {
		dir, err := filepath.Abs(cfg.Dir)
		if err != nil {
			return nil, fmt.Errorf("capture: resolve output dir %s: %w", cfg.Dir, err)
		}
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return nil, fmt.Errorf("capture: create output dir %s: %w", dir, err)
		}
		mdPath = filepath.Join(dir, mdFilename)
		mp3Path = filepath.Join(dir, base+".mp3")
	}

	if !cfg.CaptionsOnly {
		if strings.EqualFold(mdPath, mp3Path) {
			return nil, fmt.Errorf("capture: transcript and audio paths must differ: %s", mdPath)
		}
		mdInfo, mdErr := os.Stat(mdPath)
		mp3Info, mp3Err := os.Stat(mp3Path)
		if mdErr == nil && mp3Err == nil && os.SameFile(mdInfo, mp3Info) {
			return nil, fmt.Errorf("capture: transcript and audio paths refer to the same file: %s", mdPath)
		}
	}

	mdFile, err := os.OpenFile(mdPath, openFlg, filePerm)
	if err != nil {
		return nil, fmt.Errorf("capture: create transcript %s: %w", mdPath, err)
	}
	headerStart := now
	if !cfg.MeetingStart.IsZero() {
		headerStart = cfg.MeetingStart
	}
	w := transcript.NewWriter(mdFile)
	if err := w.WriteHeader(transcript.Metadata{
		MeetingID:    id,
		MeetingTitle: cfg.MeetingTitle,
		Category:     cfg.Category,
		SourceURL:    cfg.MeetingURL,
		StartTime:    headerStart,
		Timezone:     cfg.Timezone,
		Attendees:    cfg.Attendees,
		BotName:      cfg.BotName,
	}); err != nil {
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
	mp3File, err := os.OpenFile(mp3Path, openFlg, filePerm)
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

// ConcatMeetingName builds the <concat-meeting-name> token by extracting
// alphanumeric words from title (or fallbackID when title is empty), omitting
// English articles of speech and filler words, and concatenating the remaining
// words without spaces or hyphens.
func ConcatMeetingName(title, fallbackID string) string {
	source := strings.TrimSpace(title)
	words := wordTokenRe.FindAllString(source, -1)
	if len(words) == 0 {
		source = strings.TrimSpace(fallbackID)
		words = wordTokenRe.FindAllString(source, -1)
	}
	if len(words) == 0 {
		return "Meeting"
	}

	filtered := make([]string, 0, len(words))
	for _, w := range words {
		if _, isFiller := fillerWords[strings.ToLower(w)]; !isFiller {
			filtered = append(filtered, w)
		}
	}
	if len(filtered) == 0 {
		filtered = words
	}

	allUpperTitle := source != "" && source == strings.ToUpper(source) && source != strings.ToLower(source)
	var b strings.Builder
	for _, w := range filtered {
		if allUpperTitle {
			if len(w) <= 3 {
				b.WriteString(strings.ToUpper(w))
			} else {
				b.WriteString(strings.ToUpper(w[:1]) + strings.ToLower(w[1:]))
			}
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	out := b.String()
	if len(out) > maxMeetingIDLen {
		out = out[:maxMeetingIDLen]
	}
	if out == "" {
		return "Meeting"
	}
	return out
}

// FormatTranscriptFilename expands tmpl (defaulting to
// DefaultFilenameTemplate) using the meeting's Category, ConcatMeetingName,
// and date/timestamp placeholders, returning a filesystem-safe .md filename.
func FormatTranscriptFilename(tmpl string, cfg OutputConfig, now time.Time, fallbackID string) string {
	pattern := strings.TrimSpace(tmpl)
	if pattern == "" {
		pattern = DefaultFilenameTemplate
	}
	if fallbackID == "" {
		fallbackID = sanitizeMeetingID(cfg.MeetingURL)
	}
	cat := sanitizeFilenameToken(cfg.Category)
	concatName := ConcatMeetingName(cfg.MeetingTitle, fallbackID)
	meetingName := sanitizeFilenameToken(cfg.MeetingTitle)
	if meetingName == "" {
		meetingName = fallbackID
	}
	dateYYYYMMDD := now.Format("20060102")
	dateISO := now.Format("2006-01-02")
	timeHHMMSS := now.Format("150405")
	tsFull := now.Format(timestampLayout)

	r := strings.NewReplacer(
		"<category>", cat,
		"<Category>", cat,
		"{{category}}", cat,
		"<concat-meeting-name>", concatName,
		"{{concat-meeting-name}}", concatName,
		"{{concat_meeting_name}}", concatName,
		"<meeting-name>", meetingName,
		"{{meeting-name}}", meetingName,
		"{{meeting_name}}", meetingName,
		"<meeting-id>", fallbackID,
		"{{meeting-id}}", fallbackID,
		"{{meeting_id}}", fallbackID,
		"<YYYYMMDD>", dateYYYYMMDD,
		"<yyyymmdd>", dateYYYYMMDD,
		"<date>", dateYYYYMMDD,
		"{{YYYYMMDD}}", dateYYYYMMDD,
		"{{yyyymmdd}}", dateYYYYMMDD,
		"{{date}}", dateYYYYMMDD,
		"<YYYY-MM-DD>", dateISO,
		"<yyyy-mm-dd>", dateISO,
		"{{YYYY-MM-DD}}", dateISO,
		"{{yyyy-mm-dd}}", dateISO,
		"<HHMMSS>", timeHHMMSS,
		"<hhmmss>", timeHHMMSS,
		"{{HHMMSS}}", timeHHMMSS,
		"{{hhmmss}}", timeHHMMSS,
		"<YYYYMMDD-HHMMSS>", dateYYYYMMDD+"-"+timeHHMMSS,
		"<timestamp>", tsFull,
		"{{timestamp}}", tsFull,
	)
	expanded := r.Replace(pattern)
	lower := strings.ToLower(expanded)
	switch {
	case strings.HasSuffix(lower, ".markdown"):
		expanded = expanded[:len(expanded)-len(".markdown")]
	case strings.HasSuffix(lower, ".md"):
		expanded = expanded[:len(expanded)-len(".md")]
	}
	stem := sanitizeFilenameToken(expanded)
	if stem == "" {
		stem = concatName + "-transcript-" + dateYYYYMMDD
	}
	return stem + ".md"
}

func sanitizeFilenameToken(raw string) string {
	cleaned := unsafeFileChars.ReplaceAllString(strings.TrimSpace(raw), "-")
	cleaned = strings.ReplaceAll(cleaned, "..", "-")
	cleaned = multiHyphenRe.ReplaceAllString(cleaned, "-")
	return strings.Trim(cleaned, "-_.")
}
