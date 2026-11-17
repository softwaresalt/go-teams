package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// defaultTemplateName is the configuration filename used when none is given.
	defaultTemplateName = "config.yaml"

	// templateDirMode is the permission mode for directories created by
	// WriteTemplate (owner-only).
	templateDirMode os.FileMode = 0o700
	// templateFileMode is the permission mode for configuration files written
	// by WriteTemplate (owner-only).
	templateFileMode os.FileMode = 0o600
)

var (
	// ErrConfigExists indicates that WriteTemplate refused to overwrite an
	// existing configuration file because force was not set.
	ErrConfigExists = errors.New("config: configuration file already exists")
	// ErrInvalidConfigName indicates that a generated configuration filename
	// or path is empty, escapes its directory, or otherwise unsafe.
	ErrInvalidConfigName = errors.New("config: invalid configuration filename")
)

// templateYAML is the fully commented configuration template. Lines of the
// form "# key: value" (and the "# meetings:" example block) are canonical
// examples that become valid YAML when the leading "# " is removed. Aliases
// and alternatives are documented as prose so that activating every canonical
// example never produces conflicting settings.
const templateYAML = `# go-teams configuration file
#
# Every setting below is turned off (commented out), so this file is valid as
# generated and go-teams falls back to its built-in defaults. To change a
# setting, remove the leading "# " from its line and edit the value. Lines that
# start with "#   Alias:" or "#   Alternative:" are notes, not settings.
#
# Where go-teams looks for this file (first match wins):
#   1. the path given with --config / -f
#   2. config.yaml in the current directory
#   3. config.yaml next to the go-teams executable
#   4. the per-user default location (for example
#      %APPDATA%\go-teams\config.yaml on Windows)
#
# Paths accept %LOCALAPPDATA%-style variables, $ENV_VAR variables, and a
# leading ~ for your home folder. In double-quoted YAML text, write a backslash
# as two backslashes (\\).
#
# Defaults when nothing below is changed:
#   * transcripts are saved under your per-user go-teams\transcripts folder
#   * a visible browser window opens (set headless: true to run in the background)
#   * the bot name is "Transcription Agent"

# ---------------------------------------------------------------------------
# Where transcripts are saved
# ---------------------------------------------------------------------------

# Folder where transcripts are written when you do not give an explicit .md
# file on the command line.
#   Alias: output_folder, default_path
# output_dir: "C:\\Transcripts\\Team"

# Alternative to output_dir: one fixed Markdown file path instead of a folder
# plus a naming template. Do not use it together with output_dir.
#   Alternative: output_file: "C:\\Transcripts\\Team\\meeting.md"
#   Alias: output (a path ending in .md or .markdown is treated as output_file,
#   anything else as output_dir)

# Naming pattern for files written into output_dir. Placeholders:
#   <category>            your category value (dropped cleanly when empty)
#   <concat-meeting-name> meeting title with filler words removed, words joined
#   <meeting-name>        meeting title with words joined by hyphens
#   <meeting-id>          cleaned-up Teams meeting identifier
#   <YYYYMMDD>            capture date, for example 20261109
#   <YYYY-MM-DD>          capture date, for example 2026-11-09
#   <HHMMSS>              capture time, for example 080000
#   <timestamp>           capture timestamp
#   Alias: name_template
# filename_template: "<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md"

# Label for a customer, project, or team. It appears in the transcript header
# and fills the <category> placeholder.
# category: "TEAM"

# Replace existing transcript (.md) and audio (.mp3) files without prompting
# when true; when false (the default), go-teams prompts "(Y)es | (N)o" before
# overwriting an existing file.
#   Alias: file_overwrite
# overwrite: false

# ---------------------------------------------------------------------------
# What to capture
# ---------------------------------------------------------------------------

# Default meeting or Meeting Recap URL, used when you do not pass one on the
# command line.
#   Alias: url
# meeting_url: "https://teams.microsoft.com/meet/synthetic-meeting"

# Capture protocol:
#   auto              Meeting Recap links are exported, live links are captured
#                     as they happen (default)
#   ephemeral-live    live captions and MP3 audio with no AI Summary wait
#   live-with-summary live capture plus the AI Summary fetched after ai_summary_delay
#   past-recap        finite export of a finished meeting's transcript and summary
#   Alias: mode
# capture_mode: "auto"

# Record the meeting audio to an MP3 file next to the transcript.
#   Alias: audio
# capture_audio: false

# Timing. Durations use Go syntax such as 90s, 5m, or 1h30m.
#   duration          planned live capture length before inactivity checks begin
#   idle_timeout      stop after this long without captions or audio
#   ai_summary_delay  wait before fetching the post-call AI Summary (0s is allowed)
#   lobby_timeout     give up if you are not admitted from the lobby in this time
# duration: "1h"
# idle_timeout: "3m"
# ai_summary_delay: "30m"
# lobby_timeout: "5m"

# Queue file where deferred --append-ai-summary entries are written. "go-teams
# schedule" sweeps it at startup. Defaults to transcription_queue.txt.
# queue_file: "C:\\Transcripts\\Team\\transcription_queue.txt"

# ---------------------------------------------------------------------------
# Browser and sign-in
# ---------------------------------------------------------------------------

# Browser profile folder that keeps your Microsoft 365 sign-in between runs.
# Treat it like a password: never share or commit it.
# profile_dir: "%LOCALAPPDATA%\\go-teams\\profile"

# Path to Microsoft Edge or Google Chrome. Edge is found automatically on
# Windows when this is not set.
# browser_path: "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe"

# Microsoft Entra account (email or display name) to pick automatically on the
# account chooser screen.
#   Alias: entra_account, account
# default_account: "alice@contoso.com"

# Name shown only when joining as an unauthenticated guest.
# bot_name: "Transcription Agent"

# true runs the browser in the background (use after your first interactive
# sign-in with go-teams auth); false shows the browser window.
#   Alias: headed (headed: true forces a visible window)
# headless: true

# ---------------------------------------------------------------------------
# Scheduled meetings for "go-teams schedule"
# ---------------------------------------------------------------------------

# Each meeting needs a url and exactly one trigger: cron (repeats) or at (once).
#   cron  five-field cron expression, for example "0 9 * * 1-5" (09:00 weekdays)
#   at    one-time local time "2026-11-15 14:00" or an RFC 3339 timestamp
# Optional per-meeting settings: mode (alias: capture_mode), duration,
# idle_timeout, ai_summary_delay, lobby_timeout, category, capture_audio
# (alias: audio), overwrite (alias: file_overwrite), and output_dir. They
# override the global settings above.
# meetings:
#   - name: "Daily Standup"
#     url: "https://teams.microsoft.com/meet/synthetic-standup"
#     cron: "0 9 * * 1-5"
#     mode: "live-with-summary"
#     duration: "30m"
#     idle_timeout: "3m"
#     ai_summary_delay: "30m"
#     lobby_timeout: "5m"
#     category: "TEAM"
#     capture_audio: false
#     overwrite: false
#   - name: "Architecture Review"
#     url: "https://teams.microsoft.com/meet/synthetic-review"
#     at: "2026-11-15 14:00"
#     mode: "ephemeral-live"
#     duration: "1h"
#     idle_timeout: "3m"
#     category: "ARCH"
#     capture_audio: true
#     output_dir: "C:\\Transcripts\\Architecture"
`

// TemplateYAML returns a complete configuration file in which every option
// supported by Config and MeetingSchedule is commented out and explained in
// plain language. The text is valid as generated; removing the leading "# "
// from a "# key: value" line activates that setting.
func TemplateYAML() string {
	return templateYAML
}

// ResolveTemplatePath returns the destination path for a generated
// configuration file. An empty (or whitespace-only) outputDir means the current
// directory; otherwise it is expanded like other configuration paths. An empty
// name means "config.yaml". A name that is not a plain filename inside
// outputDir (separators, drive letters, alternate data streams, traversal, or
// no stem) is rejected with ErrInvalidConfigName, and ".yaml" is appended when
// name has no .yaml or .yml extension.
func ResolveTemplatePath(outputDir, name string) (string, error) {
	dir := "."
	if trimmed := strings.TrimSpace(outputDir); trimmed != "" {
		dir = expandUserPath(trimmed)
	}
	if name == "" {
		name = defaultTemplateName
	}
	fileName, err := validateTemplateName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// ExpandPath expands user and environment path prefixes (%VAR%, $VAR, and a
// leading ~) in raw using the same rules as other configuration paths, after
// trimming surrounding whitespace.
func ExpandPath(raw string) string {
	return expandUserPath(strings.TrimSpace(raw))
}

// isWindowsReservedName reports whether name, compared case-insensitively on
// the stem before the first ".", is a Windows reserved device name (CON, PRN,
// AUX, NUL, COM1-COM9, LPT1-LPT9). Such names cannot be created as ordinary
// files on Windows, so they are rejected on every platform for portability.
func isWindowsReservedName(name string) bool {
	stem, _, _ := strings.Cut(name, ".")
	switch strings.ToUpper(strings.TrimSpace(stem)) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	}
	return false
}

// validateTemplateName checks that name is a bare filename and returns it
// trimmed, with a .yaml extension appended when it has no YAML extension.
func validateTemplateName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	ext := filepath.Ext(trimmed)
	switch {
	case trimmed == "", trimmed == ".", trimmed == "..",
		strings.Contains(trimmed, ".."),
		strings.ContainsAny(trimmed, `/\:`),
		strings.HasSuffix(trimmed, "."),
		filepath.VolumeName(trimmed) != "",
		strings.TrimSpace(strings.TrimSuffix(trimmed, ext)) == "",
		isWindowsReservedName(trimmed):
		return "", fmt.Errorf("config: filename %q: %w", name, ErrInvalidConfigName)
	}
	if !strings.EqualFold(ext, ".yaml") && !strings.EqualFold(ext, ".yml") {
		trimmed += ".yaml"
	}
	return trimmed, nil
}

// WriteTemplate writes TemplateYAML to path, creating missing parent
// directories with mode 0o700 and the file with mode 0o600. Without force an
// existing file is never touched and the returned error wraps ErrConfigExists.
// With force the file is replaced atomically through a temporary file in the
// same directory.
func WriteTemplate(path string, force bool) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("config: write template: empty path: %w", ErrInvalidConfigName)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, templateDirMode); err != nil {
		return fmt.Errorf("config: create dir %s: %w", dir, err)
	}
	content := []byte(TemplateYAML())
	if force {
		return replaceTemplate(dir, path, content)
	}
	return createTemplate(path, content)
}

// createTemplate writes content to a new file at path, failing with
// ErrConfigExists when the file already exists.
func createTemplate(path string, content []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, templateFileMode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("config: write template %s: %w", path, ErrConfigExists)
		}
		return fmt.Errorf("config: write template %s: %w", path, err)
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()       // best-effort close on the failure path; the write error is the one returned
		_ = os.Remove(path) // best-effort removal of the partial file we created
		return fmt.Errorf("config: write template %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()       // best-effort close on the failure path; the sync error is the one returned
		_ = os.Remove(path) // best-effort removal of the partial file we created
		return fmt.Errorf("config: sync template %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path) // best-effort removal of the partial file we created
		return fmt.Errorf("config: close template %s: %w", path, err)
	}
	return nil
}

// replaceTemplate atomically replaces path with content using a sibling
// temporary file in dir.
func replaceTemplate(dir, path string, content []byte) error {
	tmp, err := os.CreateTemp(dir, ".config-template-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("config: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if err := tmp.Chmod(templateFileMode); err != nil {
		_ = tmp.Close()        // best-effort close; the chmod error is returned
		_ = os.Remove(tmpName) // best-effort temp cleanup
		return fmt.Errorf("config: chmod temp file: %w", err)
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()        // best-effort close on the failure path; the write error is the one returned
		_ = os.Remove(tmpName) // best-effort temp cleanup
		return fmt.Errorf("config: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()        // best-effort close on the failure path; the sync error is the one returned
		_ = os.Remove(tmpName) // best-effort temp cleanup
		return fmt.Errorf("config: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName) // best-effort temp cleanup
		return fmt.Errorf("config: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName) // best-effort temp cleanup
		return fmt.Errorf("config: replace %s: %w", path, err)
	}
	return nil
}
