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

	"github.com/softwaresalt/go-teams/internal/teams"
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
	// ErrOverwriteDeclined is returned when one or more target output files
	// already exist, Overwrite is false, and the operator declines the
	// overwrite confirmation prompt.
	ErrOverwriteDeclined = errors.New("capture: file overwrite declined")

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
	// Now is the capture start time used as the fallback for output file
	// naming and the transcript header start time when MeetingStart is zero; a
	// zero value defaults to time.Now.
	Now time.Time
	// MeetingStart is the optional discovered meeting start time written to
	// the transcript header (**Meeting Start:**) and used for date/time
	// placeholders in output file naming when non-zero.
	MeetingStart time.Time
	// CaptionsOnly skips creating the MP3 audio output file when only live
	// captions are being captured to Markdown.
	CaptionsOnly bool
	// Overwrite permits replacing existing transcript and audio files without
	// prompting. When false and a target file already exists, setupOutput
	// invokes ConfirmOverwrite (or fails with os.ErrExist when nil).
	Overwrite bool
	// ConfirmOverwrite prompts the operator before overwriting existing target
	// files when Overwrite is false. It receives the existing paths in
	// deterministic order (mdPath, then mp3Path).
	ConfirmOverwrite func(paths []string) (bool, error)
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
	// AudioExisted reports whether AudioPath already existed before setupOutput
	// opened it, so failed-capture cleanup never deletes a pre-existing file.
	AudioExisted bool

	mdFile  *os.File
	mp3File *os.File
}

// setupOutput creates the output directory and files, writes the transcript
// header, and returns the open Output. Both Dir and FilePath modes enforce the
// same overwrite policy: existing files require Overwrite or ConfirmOverwrite
// approval before any file is opened or truncated.
func setupOutput(cfg OutputConfig) (*Output, error) {
	return setupOutputWithFlushHook(cfg, nil)
}

func setupOutputWithFlushHook(cfg OutputConfig, flushHeader func(*transcript.Writer) error) (*Output, error) {
	now := cfg.Now
	if now.IsZero() {
		now = time.Now()
	}
	headerStart := now
	if !cfg.MeetingStart.IsZero() {
		headerStart = cfg.MeetingStart
	}
	id := sanitizeMeetingID(cfg.MeetingURL)
	mdFilename := FormatTranscriptFilename(cfg.FilenameTemplate, cfg, headerStart, id)
	base := strings.TrimSuffix(mdFilename, filepath.Ext(mdFilename))

	explicitFile := cfg.FilePath
	var (
		mdPath  string
		mp3Path string
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

	checkTarget := func(p string) (os.FileInfo, bool, error) {
		info, statErr := os.Lstat(p)
		if statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, false, fmt.Errorf("capture: output path must not be a symlink: %s", p)
			}
			if info.IsDir() {
				return nil, false, fmt.Errorf("capture: output path is a directory: %s", p)
			}
			if !info.Mode().IsRegular() {
				return nil, false, fmt.Errorf("capture: output path is not a regular file: %s", p)
			}
			// Eagerly populate platform file identity (e.g. Windows volume/index IDs
			// loaded lazily inside os.SameFile) before ConfirmOverwrite runs.
			_ = os.SameFile(info, info)
			return info, true, nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return nil, false, fmt.Errorf("capture: stat %s: %w", p, statErr)
		}
		return nil, false, nil
	}

	mdPreInfo, mdExisted, err := checkTarget(mdPath)
	if err != nil {
		return nil, err
	}
	var (
		mp3PreInfo os.FileInfo
		mp3Existed bool
	)
	if !cfg.CaptionsOnly {
		mp3PreInfo, mp3Existed, err = checkTarget(mp3Path)
		if err != nil {
			return nil, err
		}
	}

	var existingPaths []string
	if mdExisted {
		existingPaths = append(existingPaths, mdPath)
	}
	if !cfg.CaptionsOnly && mp3Existed {
		existingPaths = append(existingPaths, mp3Path)
	}

	if len(existingPaths) > 0 && !cfg.Overwrite {
		if cfg.ConfirmOverwrite == nil {
			return nil, fmt.Errorf("capture: target file already exists: %s: %w", existingPaths[0], os.ErrExist)
		}
		approved, confirmErr := cfg.ConfirmOverwrite(existingPaths)
		if confirmErr != nil {
			return nil, fmt.Errorf("capture: confirm overwrite: %w", confirmErr)
		}
		if !approved {
			return nil, fmt.Errorf("%w: %s", ErrOverwriteDeclined, strings.Join(existingPaths, ", "))
		}
	}

	openOutputFileNoTrunc := func(p string, preInfo os.FileInfo, existed bool) (*os.File, error) {
		if !existed {
			return os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_EXCL, filePerm)
		}
		lstatInfo, lErr := os.Lstat(p)
		if lErr != nil {
			return nil, lErr
		}
		if lstatInfo.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("output path must not be a symlink: %s", p)
		}
		if lstatInfo.IsDir() || !lstatInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("output path is not a regular file: %s", p)
		}
		if !os.SameFile(preInfo, lstatInfo) {
			return nil, fmt.Errorf("output path changed before overwrite: %s", p)
		}
		f, openErr := os.OpenFile(p, os.O_RDWR, filePerm)
		if openErr != nil {
			return nil, openErr
		}
		fdInfo, fStatErr := f.Stat()
		if fStatErr != nil || !os.SameFile(preInfo, fdInfo) || !os.SameFile(lstatInfo, fdInfo) {
			_ = f.Close()
			if fStatErr != nil {
				return nil, fStatErr
			}
			return nil, fmt.Errorf("output file identity changed before truncation: %s", p)
		}
		return f, nil
	}

	backupToTemp := func(f *os.File, p string) (string, error) {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		tmp, err := os.CreateTemp(filepath.Dir(p), ".go-teams-backup-*")
		if err != nil {
			return "", err
		}
		tmpPath := tmp.Name()
		if _, copyErr := io.Copy(tmp, f); copyErr != nil {
			_ = tmp.Close()        // best-effort close on backup copy failure; copyErr is returned
			_ = os.Remove(tmpPath) // best-effort cleanup of incomplete temp backup
			return "", copyErr
		}
		if closeErr := tmp.Close(); closeErr != nil {
			_ = os.Remove(tmpPath) // best-effort cleanup of unclosed temp backup
			return "", closeErr
		}
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			_ = os.Remove(tmpPath) // best-effort cleanup when rewind fails
			return "", seekErr
		}
		return tmpPath, nil
	}

	mdFile, err := openOutputFileNoTrunc(mdPath, mdPreInfo, mdExisted)
	if err != nil {
		return nil, fmt.Errorf("capture: create transcript %s: %w", mdPath, err)
	}
	var mp3File *os.File
	if !cfg.CaptionsOnly {
		mp3File, err = openOutputFileNoTrunc(mp3Path, mp3PreInfo, mp3Existed)
		if err != nil {
			// Close mdFile before truncating it so an audio open failure leaves
			// any pre-existing mdPath byte-for-byte untouched.
			_ = mdFile.Close() // best-effort close on audio open failure; err is returned
			if !mdExisted {
				_ = os.Remove(mdPath) // best-effort removal of newly created empty transcript
			}
			return nil, fmt.Errorf("capture: create audio %s: %w", mp3Path, err)
		}
	}

	var (
		mdBackup, mp3Backup       string
		mdTruncated, mp3Truncated bool
	)
	cleanupBackups := func() {
		if mdBackup != "" {
			_ = os.Remove(mdBackup) // best-effort removal of unneeded temp backup
			mdBackup = ""
		}
		if mp3Backup != "" {
			_ = os.Remove(mp3Backup) // best-effort removal of unneeded temp backup
			mp3Backup = ""
		}
	}
	rollbackPair := func(cause error) error {
		var restoreErrs []error
		if mdTruncated {
			if rErr := restoreFromTemp(mdFile, mdBackup); rErr != nil {
				restoreErrs = append(restoreErrs, fmt.Errorf("restore transcript (backup preserved at %s): %w", mdBackup, rErr))
				mdBackup = "" // preserve backup on disk for manual recovery
			} else {
				mdBackup = ""
			}
		}
		_ = mdFile.Close() // best-effort close on rollback; primary cause is returned
		if !mdExisted {
			_ = os.Remove(mdPath) // best-effort removal of newly created transcript
		}
		if mp3File != nil {
			if mp3Truncated {
				if rErr := restoreFromTemp(mp3File, mp3Backup); rErr != nil {
					restoreErrs = append(restoreErrs, fmt.Errorf("restore audio (backup preserved at %s): %w", mp3Backup, rErr))
					mp3Backup = "" // preserve backup on disk for manual recovery
				} else {
					mp3Backup = ""
				}
			}
			_ = mp3File.Close() // best-effort close on rollback; primary cause is returned
			if !mp3Existed {
				_ = os.Remove(mp3Path) // best-effort removal of newly created audio file
			}
		}
		cleanupBackups()
		if len(restoreErrs) > 0 {
			return errors.Join(append([]error{cause}, restoreErrs...)...)
		}
		return cause
	}

	if mdExisted {
		mdBackup, err = backupToTemp(mdFile, mdPath)
		if err != nil {
			return nil, rollbackPair(fmt.Errorf("capture: backup transcript %s: %w", mdPath, err))
		}
	}
	if mp3File != nil && mp3Existed {
		mp3Backup, err = backupToTemp(mp3File, mp3Path)
		if err != nil {
			return nil, rollbackPair(fmt.Errorf("capture: backup audio %s: %w", mp3Path, err))
		}
	}

	if mdExisted {
		if truncErr := mdFile.Truncate(0); truncErr != nil {
			return nil, rollbackPair(fmt.Errorf("capture: truncate transcript %s: %w", mdPath, truncErr))
		}
		mdTruncated = true
	}
	w := transcript.NewWriter(mdFile)
	if err := w.WriteHeader(transcript.Metadata{
		MeetingID:    id,
		MeetingTitle: cfg.MeetingTitle,
		Category:     cfg.Category,
		SourceURL:    redactSourceURL(cfg.MeetingURL),
		StartTime:    headerStart,
		Timezone:     cfg.Timezone,
		Attendees:    cfg.Attendees,
		BotName:      cfg.BotName,
	}); err != nil {
		// Best-effort rollback of newly created files or restoration of
		// pre-existing files from backup.
		return nil, rollbackPair(fmt.Errorf("capture: write transcript header: %w", err))
	}
	// Flush the header immediately so a physical write failure (e.g. a full
	// filesystem) surfaces here and triggers rollback, rather than being
	// deferred inside the bufio buffer until the first caption is written.
	flushErr := w.Flush()
	if flushErr == nil && flushHeader != nil {
		flushErr = flushHeader(w)
	}
	if flushErr != nil {
		return nil, rollbackPair(fmt.Errorf("capture: flush transcript header: %w", flushErr))
	}
	if mp3File != nil && mp3Existed {
		if truncErr := mp3File.Truncate(0); truncErr != nil {
			return nil, rollbackPair(fmt.Errorf("capture: truncate audio %s: %w", mp3Path, truncErr))
		}
		mp3Truncated = true
	}
	cleanupBackups()
	if cfg.CaptionsOnly {
		return &Output{
			Transcript:     w,
			Audio:          io.Discard,
			TranscriptPath: mdPath,
			mdFile:         mdFile,
		}, nil
	}
	return &Output{
		Transcript:     w,
		Audio:          mp3File,
		TranscriptPath: mdPath,
		AudioPath:      mp3Path,
		AudioExisted:   mp3Existed,
		mdFile:         mdFile,
		mp3File:        mp3File,
	}, nil
}

// restoreFromTemp restores f byte-for-byte from backupPath and removes
// backupPath only after Sync succeeds. On any error, backupPath is preserved on
// disk so the caller can surface its path for recovery.
func restoreFromTemp(f *os.File, backupPath string) error {
	if f == nil || backupPath == "" {
		return nil
	}
	src, openErr := os.Open(backupPath)
	if openErr != nil {
		return openErr
	}
	if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
		_ = src.Close() // best-effort close of read-only backup handle; seekErr is returned
		return seekErr
	}
	if truncErr := f.Truncate(0); truncErr != nil {
		_ = src.Close() // best-effort close of read-only backup handle; truncErr is returned
		return truncErr
	}
	if _, copyErr := io.Copy(f, src); copyErr != nil {
		_ = src.Close() // best-effort close of read-only backup handle; copyErr is returned
		return copyErr
	}
	if syncErr := f.Sync(); syncErr != nil {
		_ = src.Close() // best-effort close of read-only backup handle; syncErr is returned
		return syncErr
	}
	if closeErr := src.Close(); closeErr != nil {
		return closeErr
	}
	_ = os.Remove(backupPath) // best-effort removal after verified restore+sync; original file is already intact
	return nil
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

func hasSharePointSharingPath(raw string) bool {
	cur := raw
	for range 3 {
		lower := strings.ToLower(cur)
		if strings.Contains(lower, "/:") || strings.Contains(lower, "/%3a") ||
			strings.Contains(lower, "%2f:") || strings.Contains(lower, "%2f%3a") {
			return true
		}
		next, err := url.PathUnescape(cur)
		if err != nil || next == cur {
			break
		}
		cur = next
	}
	return false
}

func redactSourceURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	redacted := teams.RedactURL(trimmed)
	if redacted == "" || redacted == "[invalid-url]" || hasSharePointSharingPath(redacted) {
		return ""
	}
	return redacted
}

func extractThreadIDMeetingToken(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	for _, rawVal := range u.Query()["threadId"] {
		if tok := meetingIDToken.FindString(rawVal); tok != "" {
			return tok
		}
	}
	if idx := strings.Index(u.Fragment, "?"); idx >= 0 {
		if fragVals, fErr := url.ParseQuery(u.Fragment[idx+1:]); fErr == nil {
			for _, rawVal := range fragVals["threadId"] {
				if tok := meetingIDToken.FindString(rawVal); tok != "" {
					return tok
				}
			}
		}
	}
	return ""
}

// sanitizeMeetingID derives a filesystem-safe identifier from a Teams meeting
// URL. It prefers the embedded "meeting_..." token when present, lowercases the
// value, replaces every unsafe run with a single hyphen, and caps the length.
// The result always matches [a-z0-9-]+ and never contains path separators or
// "..", so it is safe to join onto the output directory.
func sanitizeMeetingID(rawURL string) string {
	if hasSharePointSharingPath(rawURL) {
		return "meeting"
	}
	redacted := redactSourceURL(rawURL)
	decodedPath := redacted
	if unescaped, err := url.PathUnescape(redacted); err == nil {
		decodedPath = unescaped
	}
	var candidate string
	if token := meetingIDToken.FindString(decodedPath); token != "" {
		candidate = token
	} else if m := meetPathToken.FindStringSubmatch(decodedPath); len(m) == 2 {
		candidate = "meet-" + m[1]
	} else if token := extractThreadIDMeetingToken(rawURL); token != "" {
		candidate = token
	} else if redacted != "" {
		candidate = redacted
	} else if token := meetingIDToken.FindString(rawURL); token != "" && !strings.ContainsAny(rawURL, "?#@") {
		candidate = token
	} else {
		return "meeting"
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

// AppendAISummaryToFile appends the formatted AI Summary section to an
// existing Markdown transcript file without overwriting preceding content. It
// is idempotent when the file already contains an AI Summary heading.
func AppendAISummaryToFile(transcriptPath, summary string) error {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return errors.New("capture: ai summary is empty")
	}
	if strings.TrimSpace(transcriptPath) == "" {
		return errors.New("capture: transcript path must not be empty")
	}
	absPath, err := filepath.Abs(transcriptPath)
	if err != nil {
		return fmt.Errorf("capture: resolve transcript path %s: %w", transcriptPath, err)
	}
	existingBytes, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("capture: read transcript %s: %w", absPath, err)
	}
	existing := strings.ReplaceAll(string(existingBytes), "\r\n", "\n")
	if strings.Contains(existing, "\n# AI Summary\n") ||
		strings.Contains(existing, "\n## AI Summary\n") ||
		strings.HasPrefix(existing, "# AI Summary\n") ||
		strings.HasPrefix(existing, "## AI Summary\n") {
		return nil
	}
	f, err := os.OpenFile(absPath, os.O_WRONLY|os.O_APPEND, filePerm)
	if err != nil {
		return fmt.Errorf("capture: open transcript %s for append: %w", absPath, err)
	}
	w := transcript.NewWriter(f)
	if err := w.WriteAISummary(trimmed); err != nil {
		_ = f.Close()
		return fmt.Errorf("capture: write ai summary to %s: %w", absPath, err)
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		return fmt.Errorf("capture: flush ai summary to %s: %w", absPath, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("capture: sync transcript %s: %w", absPath, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("capture: close transcript %s: %w", absPath, err)
	}
	return nil
}
