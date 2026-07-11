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
		Dir:        dir,
		MeetingURL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_AbC123%40thread.v2/0",
		BotName:    "Transcription Agent",
		Now:        fixedTime(),
	})
	if err != nil {
		t.Fatalf("setupOutput: %v", err)
	}
	t.Cleanup(func() { _ = out.Close() })

	wantBase := "meeting-abc123-20260710-150405.000"
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
	for _, want := range []string{"meeting_id:", "meeting-abc123", "bot_name:", "Transcription Agent", "generated_by: teams-cli"} {
		if !strings.Contains(header, want) {
			t.Errorf("transcript header missing %q; got:\n%s", want, header)
		}
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
