package capture

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var safeName = regexp.MustCompile(`^[a-z0-9-]+$`)

func TestSanitizeMeetingID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "teams join url extracts meeting token",
			url:  "https://teams.microsoft.com/l/meetup-join/19%3ameeting_AbC123XyZ%40thread.v2/0?context=x",
			want: "meeting-abc123xyz",
		},
		{
			name: "teams meet url extracts meet id and omits passcode",
			url:  "https://teams.microsoft.com/meet/234567890?p=SecretPasscode123",
			want: "meet-234567890",
		},
		{
			name: "empty url falls back to meeting",
			url:  "",
			want: "meeting",
		},
		{
			name: "only unsafe characters falls back to meeting",
			url:  "://@@@",
			want: "meeting",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeMeetingID(tc.url)
			if got != tc.want {
				t.Fatalf("sanitizeMeetingID(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestSanitizeMeetingIDAlwaysFilesystemSafe(t *testing.T) {
	urls := []string{
		"https://teams.microsoft.com/l/meetup-join/19:meeting_LongOne@thread.v2/0",
		"../../etc/passwd",
		"C:\\Windows\\System32",
		strings.Repeat("meeting_", 40),
	}
	for _, u := range urls {
		got := sanitizeMeetingID(u)
		if !safeName.MatchString(got) {
			t.Errorf("sanitizeMeetingID(%q) = %q is not filesystem-safe", u, got)
		}
		if strings.Contains(got, "..") || strings.ContainsAny(got, `/\`) {
			t.Errorf("sanitizeMeetingID(%q) = %q contains path traversal", u, got)
		}
		if len(got) > 64 {
			t.Errorf("sanitizeMeetingID(%q) length %d exceeds 64", u, len(got))
		}
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 7, 10, 15, 4, 5, 0, time.UTC)
}

func TestSetupOutputCreatesPairWithHeader(t *testing.T) {
	dir := t.TempDir()
	out, err := setupOutput(OutputConfig{
		Dir:          dir,
		MeetingURL:   "https://teams.microsoft.com/l/meetup-join/19%3ameeting_AbC123%40thread.v2/0",
		MeetingTitle: "NVD Pacific TZ Sync",
		Category:     "WSP",
		BotName:      "Transcription Agent",
		Now:          fixedTime(),
	})
	if err != nil {
		t.Fatalf("setupOutput: %v", err)
	}
	t.Cleanup(func() { _ = out.Close() })

	wantBase := "WSP-NVDPacificTZSync-transcript-20260710"
	if filepath.Base(out.TranscriptPath) != wantBase+".md" {
		t.Errorf("transcript path = %q, want base %q.md", out.TranscriptPath, wantBase)
	}
	if filepath.Base(out.AudioPath) != wantBase+".mp3" {
		t.Errorf("audio path = %q, want base %q.mp3", out.AudioPath, wantBase)
	}
	// Both files must live inside the output dir.
	for _, p := range []string{out.TranscriptPath, out.AudioPath} {
		if filepath.Dir(p) != dir {
			t.Errorf("path %q is not inside output dir %q", p, dir)
		}
		if _, statErr := os.Stat(p); statErr != nil {
			t.Errorf("expected file %q to exist: %v", p, statErr)
		}
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	data, err := os.ReadFile(out.TranscriptPath)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	header := string(data)
	for _, want := range []string{"# Meeting Attendees", "**Total Attendees:**", "**Meeting Start:** 2026-07-10 15:04", "**Meeting:** NVD Pacific TZ Sync", "**Category:** WSP", "## Attendee List", "## Transcript"} {
		if !strings.Contains(header, want) {
			t.Errorf("transcript header missing %q; got:\n%s", want, header)
		}
	}
	for _, unwanted := range []string{"bot_name:", "generated_by:"} {
		if strings.Contains(header, unwanted) {
			t.Errorf("transcript header should not contain %q; got:\n%s", unwanted, header)
		}
	}
}

func TestConcatMeetingNameAndDefaultTemplate(t *testing.T) {
	ts := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		category string
		title    string
		tmpl     string
		want     string
	}{
		{
			name:     "category and mixed-case title",
			category: "WSP",
			title:    "NVD Pacific TZ Sync",
			want:     "WSP-NVDPacificTZSync-transcript-20261109.md",
		},
		{
			name:     "category and uppercase title with acronyms",
			category: "WSP",
			title:    "FL DOE ENGINEER OPTIONS",
			want:     "WSP-FLDOEEngineerOptions-transcript-20261109.md",
		},
		{
			name:     "omits articles of speech and filler words",
			category: "WSP",
			title:    "Review of the Architecture and Design for a Cloud Migration in the Portal",
			want:     "WSP-ReviewArchitectureDesignCloudMigrationPortal-transcript-20261109.md",
		},
		{
			name:     "empty category omits leading hyphen",
			category: "",
			title:    "FL DOE ENGINEER OPTIONS",
			want:     "FLDOEEngineerOptions-transcript-20261109.md",
		},
		{
			name:     "custom filename template",
			category: "ACME",
			title:    "Weekly Sync for the Platform",
			tmpl:     "<YYYY-MM-DD>-<category>-<concat-meeting-name>.md",
			want:     "2026-11-09-ACME-WeeklySyncPlatform.md",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatTranscriptFilename(tc.tmpl, OutputConfig{
				Category:     tc.category,
				MeetingTitle: tc.title,
			}, ts, "meeting-fallback")
			if got != tc.want {
				t.Fatalf("FormatTranscriptFilename() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSetupOutputRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	cfg := OutputConfig{
		Dir:        dir,
		MeetingURL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_X%40thread.v2/0",
		BotName:    "Bot",
		Now:        fixedTime(),
	}
	first, err := setupOutput(cfg)
	if err != nil {
		t.Fatalf("first setupOutput: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })

	// A second capture with the same id and timestamp must not silently
	// truncate the first capture's files; it must fail loudly instead.
	if _, err := setupOutput(cfg); err == nil {
		t.Fatal("expected setupOutput to refuse overwriting an existing capture, got nil error")
	}
}

func TestSetupOutputCaptionsOnlyRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	cfg := OutputConfig{
		Dir:          dir,
		MeetingURL:   "https://teams.microsoft.com/l/meetup-join/19%3ameeting_X%40thread.v2/0",
		MeetingTitle: "NVD Pacific TZ Sync",
		Category:     "WSP",
		BotName:      "Bot",
		Now:          fixedTime(),
		CaptionsOnly: true,
	}
	first, err := setupOutput(cfg)
	if err != nil {
		t.Fatalf("first CaptionsOnly setupOutput: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })

	if _, err := setupOutput(cfg); err == nil {
		t.Fatal("expected CaptionsOnly directory capture to refuse overwriting an existing transcript, got nil error")
	}
}

func TestSetupOutputCreatesMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does", "not", "exist", "yet")
	out, err := setupOutput(OutputConfig{
		Dir:        dir,
		MeetingURL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_X%40thread.v2/0",
		BotName:    "Bot",
		Now:        fixedTime(),
	})
	if err != nil {
		t.Fatalf("setupOutput into nonexistent dir: %v", err)
	}
	t.Cleanup(func() { _ = out.Close() })
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Fatalf("output dir was not created: %v", statErr)
	}
}

func TestSetupOutputCaptionsOnlyCreatesMarkdownOnly(t *testing.T) {
	dir := t.TempDir()
	out, err := setupOutput(OutputConfig{
		Dir:          dir,
		MeetingURL:   "https://teams.microsoft.com/meet/999888777?p=secret",
		BotName:      "Bot",
		Now:          fixedTime(),
		CaptionsOnly: true,
	})
	if err != nil {
		t.Fatalf("setupOutput CaptionsOnly: %v", err)
	}
	t.Cleanup(func() { _ = out.Close() })

	if out.TranscriptPath == "" {
		t.Fatal("expected non-empty TranscriptPath")
	}
	if out.AudioPath != "" {
		t.Fatalf("expected empty AudioPath when CaptionsOnly=true, got %q", out.AudioPath)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".md") {
		t.Fatalf("expected only one .md file in output dir, got %v", entries)
	}
}

func TestSetupOutputExplicitFilePathCreatesAndOverwritesMarkdownFile(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "nested", "custom-transcript.md")

	out1, err := setupOutput(OutputConfig{
		FilePath:     targetFile,
		MeetingURL:   "https://teams.microsoft.com/meet/999888777?p=secret",
		BotName:      "Bot",
		Now:          fixedTime(),
		CaptionsOnly: true,
	})
	if err != nil {
		t.Fatalf("first setupOutput with FilePath: %v", err)
	}
	if out1.TranscriptPath != targetFile {
		t.Fatalf("TranscriptPath = %q, want %q", out1.TranscriptPath, targetFile)
	}
	if err := out1.Close(); err != nil {
		t.Fatalf("close first output: %v", err)
	}

	// Second run to the same explicit FilePath (automation re-run) must succeed.
	out2, err := setupOutput(OutputConfig{
		FilePath:     targetFile,
		MeetingURL:   "https://teams.microsoft.com/meet/999888777?p=secret",
		BotName:      "Bot",
		Now:          fixedTime(),
		CaptionsOnly: true,
	})
	if err != nil {
		t.Fatalf("second setupOutput with FilePath: %v", err)
	}
	if err := out2.Close(); err != nil {
		t.Fatalf("close second output: %v", err)
	}
}

func TestSetupOutputRejectsHardLinkedTranscriptAndAudioAlias(t *testing.T) {
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "call.md")
	mp3File := filepath.Join(dir, "call.mp3")
	if err := os.WriteFile(mdFile, []byte("original"), 0o600); err != nil {
		t.Fatalf("write mdFile: %v", err)
	}
	if err := os.Link(mdFile, mp3File); err != nil {
		t.Skipf("hard links not supported on this filesystem: %v", err)
	}

	_, err := setupOutput(OutputConfig{
		FilePath:     mdFile,
		MeetingURL:   "https://teams.microsoft.com/meet/999888777",
		BotName:      "Bot",
		Now:          fixedTime(),
		CaptionsOnly: false,
	})
	if err == nil {
		t.Fatal("expected setupOutput to reject aliased transcript and audio files")
	}
	// Original file content must not have been truncated.
	got, readErr := os.ReadFile(mdFile)
	if readErr != nil {
		t.Fatalf("read mdFile: %v", readErr)
	}
	if string(got) != "original" {
		t.Fatalf("expected aliased file to remain untouched, got %q", string(got))
	}
}
