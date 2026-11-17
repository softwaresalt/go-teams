package capture

import (
	"context"
	"errors"
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
		{
			name: "recap url without meeting token strips query secrets before slugifying",
			url:  "https://teams.cloud.microsoft/l/meetingrecap?driveId=SecretDrive999&sitePath=SecretSite&p=SecretPasscode123",
			want: "https-teams-cloud-microsoft-l-meetingrecap",
		},
		{
			name: "passcode containing meeting_ prefix in query or fragment never leaks into meeting id",
			url:  "https://teams.live.com/meet/234567890?p=meeting_SecretPasscode123#access_token=meeting_FragSecret",
			want: "meet-234567890",
		},
		{
			name: "percent-encoded SharePoint sharing link falls back to meeting",
			url:  "https://tenant-my.sharepoint.com/%3Av%3A/g/personal/user/EabcSecretSharingToken",
			want: "meeting",
		},
		{
			name: "SharePoint sharing link containing meeting_ substring never leaks token",
			url:  "https://tenant.sharepoint.com/:v:/g/personal/user/Eabcmeeting_PrivateToken",
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
	if err := os.WriteFile(targetFile, []byte("keep first file"), 0o600); err != nil {
		t.Fatalf("seed existing targetFile: %v", err)
	}

	// Second run without Overwrite or ConfirmOverwrite must refuse with os.ErrExist
	// and preserve the existing file untouched.
	_, err = setupOutput(OutputConfig{
		FilePath:     targetFile,
		MeetingURL:   "https://teams.microsoft.com/meet/999888777?p=secret",
		BotName:      "Bot",
		Now:          fixedTime(),
		CaptionsOnly: true,
	})
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("second setupOutput without Overwrite err = %v, want wrapping os.ErrExist", err)
	}
	preserved, readErr := os.ReadFile(targetFile)
	if readErr != nil {
		t.Fatalf("read preserved targetFile: %v", readErr)
	}
	if string(preserved) != "keep first file" {
		t.Fatalf("existing file modified on refusal: %q", string(preserved))
	}

	// Second run with Overwrite: true must succeed and replace the file.
	out2, err := setupOutput(OutputConfig{
		FilePath:     targetFile,
		MeetingURL:   "https://teams.microsoft.com/meet/999888777?p=secret",
		BotName:      "Bot",
		Now:          fixedTime(),
		CaptionsOnly: true,
		Overwrite:    true,
	})
	if err != nil {
		t.Fatalf("second setupOutput with Overwrite=true: %v", err)
	}
	if err := out2.Close(); err != nil {
		t.Fatalf("close second output: %v", err)
	}
	replaced, readErr := os.ReadFile(targetFile)
	if readErr != nil {
		t.Fatalf("read replaced targetFile: %v", readErr)
	}
	if !strings.Contains(string(replaced), "# Meeting Attendees") {
		t.Fatalf("expected overwritten transcript header, got %q", string(replaced))
	}
}

func TestSetupOutputOverwriteAndConfirmPrompt(t *testing.T) {
	url := "https://teams.microsoft.com/meet/999888777?p=secret"

	t.Run("Overwrite true replaces existing files in both Dir and FilePath modes without prompting", func(t *testing.T) {
		for _, useFilePath := range []bool{false, true} {
			dir := t.TempDir()
			cfg := OutputConfig{
				MeetingURL: url,
				BotName:    "Bot",
				Now:        fixedTime(),
				Overwrite:  true,
				ConfirmOverwrite: func([]string) (bool, error) {
					t.Fatal("ConfirmOverwrite must not be called when Overwrite is true")
					return false, nil
				},
			}
			if useFilePath {
				cfg.FilePath = filepath.Join(dir, "explicit.md")
			} else {
				cfg.Dir = dir
			}
			first, err := setupOutput(cfg)
			if err != nil {
				t.Fatalf("first setupOutput: %v", err)
			}
			mdPath, mp3Path := first.TranscriptPath, first.AudioPath
			if err := first.Close(); err != nil {
				t.Fatalf("close first: %v", err)
			}
			if err := os.WriteFile(mdPath, []byte("old md"), 0o600); err != nil {
				t.Fatalf("seed md: %v", err)
			}
			if err := os.WriteFile(mp3Path, []byte("old mp3"), 0o600); err != nil {
				t.Fatalf("seed mp3: %v", err)
			}

			second, err := setupOutput(cfg)
			if err != nil {
				t.Fatalf("second setupOutput with Overwrite=true: %v", err)
			}
			if err := second.Close(); err != nil {
				t.Fatalf("close second: %v", err)
			}
			mdBytes, _ := os.ReadFile(mdPath)
			if !strings.Contains(string(mdBytes), "# Meeting Attendees") {
				t.Fatalf("expected overwritten markdown header, got %q", string(mdBytes))
			}
			mp3Bytes, _ := os.ReadFile(mp3Path)
			if len(mp3Bytes) != 0 {
				t.Fatalf("expected truncated mp3 file, got %d bytes", len(mp3Bytes))
			}
		}
	})

	t.Run("ConfirmOverwrite receives deterministic existingPaths and approval overwrites", func(t *testing.T) {
		dir := t.TempDir()
		targetMD := filepath.Join(dir, "call.md")
		targetMP3 := filepath.Join(dir, "call.mp3")
		if err := os.WriteFile(targetMD, []byte("existing md"), 0o600); err != nil {
			t.Fatalf("seed md: %v", err)
		}
		if err := os.WriteFile(targetMP3, []byte("existing mp3"), 0o600); err != nil {
			t.Fatalf("seed mp3: %v", err)
		}

		var gotPaths []string
		out, err := setupOutput(OutputConfig{
			FilePath:   targetMD,
			MeetingURL: url,
			BotName:    "Bot",
			Now:        fixedTime(),
			Overwrite:  false,
			ConfirmOverwrite: func(paths []string) (bool, error) {
				gotPaths = append([]string(nil), paths...)
				return true, nil
			},
		})
		if err != nil {
			t.Fatalf("setupOutput with approval: %v", err)
		}
		_ = out.Close()
		if len(gotPaths) != 2 || gotPaths[0] != targetMD || gotPaths[1] != targetMP3 {
			t.Fatalf("gotPaths = %v, want [%s, %s]", gotPaths, targetMD, targetMP3)
		}
	})

	t.Run("ConfirmOverwrite decline returns ErrOverwriteDeclined and preserves files byte-for-byte", func(t *testing.T) {
		dir := t.TempDir()
		targetMD := filepath.Join(dir, "call.md")
		targetMP3 := filepath.Join(dir, "call.mp3")
		if err := os.WriteFile(targetMD, []byte("keep md"), 0o600); err != nil {
			t.Fatalf("seed md: %v", err)
		}
		if err := os.WriteFile(targetMP3, []byte("keep mp3"), 0o600); err != nil {
			t.Fatalf("seed mp3: %v", err)
		}

		_, err := setupOutput(OutputConfig{
			FilePath:   targetMD,
			MeetingURL: url,
			BotName:    "Bot",
			Now:        fixedTime(),
			Overwrite:  false,
			ConfirmOverwrite: func([]string) (bool, error) {
				return false, nil
			},
		})
		if !errors.Is(err, ErrOverwriteDeclined) {
			t.Fatalf("err = %v, want wrapping ErrOverwriteDeclined", err)
		}
		mdBytes, _ := os.ReadFile(targetMD)
		mp3Bytes, _ := os.ReadFile(targetMP3)
		if string(mdBytes) != "keep md" || string(mp3Bytes) != "keep mp3" {
			t.Fatalf("declined overwrite modified files: md=%q mp3=%q", string(mdBytes), string(mp3Bytes))
		}
	})

	t.Run("ConfirmOverwrite error preserves existing files", func(t *testing.T) {
		dir := t.TempDir()
		targetMD := filepath.Join(dir, "call.md")
		if err := os.WriteFile(targetMD, []byte("keep md"), 0o600); err != nil {
			t.Fatalf("seed md: %v", err)
		}
		boom := errors.New("prompt io error")
		_, err := setupOutput(OutputConfig{
			FilePath:     targetMD,
			MeetingURL:   url,
			BotName:      "Bot",
			Now:          fixedTime(),
			CaptionsOnly: true,
			ConfirmOverwrite: func([]string) (bool, error) {
				return false, boom
			},
		})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want wrapping %v", err, boom)
		}
		mdBytes, _ := os.ReadFile(targetMD)
		if string(mdBytes) != "keep md" {
			t.Fatalf("existing file modified on prompt error: %q", string(mdBytes))
		}
	})

	t.Run("pre-open directory check leaves pre-existing mdPath untouched and post-open rollback preserves mdExisted", func(t *testing.T) {
		dir := t.TempDir()
		targetMD := filepath.Join(dir, "call.md")
		targetMP3Dir := filepath.Join(dir, "call.mp3")
		if err := os.WriteFile(targetMD, []byte("pre-existing md"), 0o600); err != nil {
			t.Fatalf("seed md: %v", err)
		}
		if err := os.MkdirAll(targetMP3Dir, 0o700); err != nil {
			t.Fatalf("mkdir mp3: %v", err)
		}

		promptCalled := false
		_, err := setupOutput(OutputConfig{
			FilePath:   targetMD,
			MeetingURL: url,
			BotName:    "Bot",
			Now:        fixedTime(),
			Overwrite:  false,
			ConfirmOverwrite: func([]string) (bool, error) {
				promptCalled = true
				return true, nil
			},
		})
		if err == nil || !strings.Contains(err.Error(), "directory") {
			t.Fatalf("expected directory error, got %v", err)
		}
		if promptCalled {
			t.Fatal("ConfirmOverwrite must not be called when mp3Path is a directory")
		}
		mdBytes, _ := os.ReadFile(targetMD)
		if string(mdBytes) != "pre-existing md" {
			t.Fatalf("pre-existing md modified when mp3Path is a directory: %q", string(mdBytes))
		}

		// When mdPath pre-existed and Overwrite is true, a post-open mp3 failure
		// must NOT os.Remove(mdPath).
		_, err = setupOutput(OutputConfig{
			FilePath:   targetMD,
			MeetingURL: url,
			BotName:    "Bot",
			Now:        fixedTime(),
			Overwrite:  true,
		})
		if err == nil {
			t.Fatal("expected error when mp3Path is a directory")
		}
		if _, statErr := os.Stat(targetMD); statErr != nil {
			t.Fatalf("pre-existing mdPath must not be deleted on post-open rollback: %v", statErr)
		}

		// When mdPath did NOT pre-exist, a post-open mp3 failure removes newly created mdPath.
		freshDir := t.TempDir()
		freshMD := filepath.Join(freshDir, "fresh.md")
		freshMP3Dir := filepath.Join(freshDir, "fresh.mp3")
		if err := os.MkdirAll(freshMP3Dir, 0o700); err != nil {
			t.Fatalf("mkdir fresh mp3: %v", err)
		}
		if _, err := setupOutput(OutputConfig{
			FilePath:   freshMD,
			MeetingURL: url,
			BotName:    "Bot",
			Now:        fixedTime(),
			Overwrite:  true,
		}); err == nil {
			t.Fatal("expected error when fresh mp3Path is a directory")
		}
		if _, statErr := os.Stat(freshMD); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("newly created mdPath should be removed on post-open rollback, statErr=%v", statErr)
		}
	})

	t.Run("Session.Run 0-byte audio cleanup preserves pre-existing mp3 and removes newly created 0-byte mp3", func(t *testing.T) {
		// 1. Pre-existing 0-byte mp3 (AudioExisted == true) must be preserved on audioErr.
		dir := t.TempDir()
		mdPath := filepath.Join(dir, "existing.md")
		mp3Path := filepath.Join(dir, "existing.mp3")
		if err := os.WriteFile(mdPath, []byte("old"), 0o600); err != nil {
			t.Fatalf("seed md: %v", err)
		}
		if err := os.WriteFile(mp3Path, nil, 0o600); err != nil {
			t.Fatalf("seed empty mp3: %v", err)
		}
		s := NewSession(SessionConfig{
			OutputFile: mdPath,
			MeetingURL: url,
			BotName:    "Bot",
			Now:        fixedTime(),
			Overwrite:  true,
		})
		s.now = fixedTime
		audioBoom := errors.New("audio failed")
		fm := &fakeMeeting{
			audioErr:       audioBoom,
			audioReturnNow: true,
		}
		if err := s.Run(context.Background(), fm); !errors.Is(err, audioBoom) {
			t.Fatalf("Run err = %v, want %v", err, audioBoom)
		}
		if _, statErr := os.Stat(mp3Path); statErr != nil {
			t.Fatalf("pre-existing 0-byte audio file must be preserved when AudioExisted=true: %v", statErr)
		}

		// 2. Newly created 0-byte mp3 (AudioExisted == false) must be removed on audioErr.
		dir2 := t.TempDir()
		mdPath2 := filepath.Join(dir2, "new.md")
		mp3Path2 := filepath.Join(dir2, "new.mp3")
		s2 := NewSession(SessionConfig{
			OutputFile: mdPath2,
			MeetingURL: url,
			BotName:    "Bot",
			Now:        fixedTime(),
		})
		s2.now = fixedTime
		fm2 := &fakeMeeting{
			audioErr:       audioBoom,
			audioReturnNow: true,
		}
		if err := s2.Run(context.Background(), fm2); !errors.Is(err, audioBoom) {
			t.Fatalf("Run err = %v, want %v", err, audioBoom)
		}
		if _, statErr := os.Stat(mp3Path2); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("newly created 0-byte audio file should be removed on audioErr, statErr=%v", statErr)
		}
	})
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

func TestSetupOutputUsesMeetingStartForFilenameAndRedactsSourceURL(t *testing.T) {
	dir := t.TempDir()
	captureNow := time.Date(2026, 11, 14, 18, 30, 0, 0, time.UTC)
	meetingStart := time.Date(2026, 11, 13, 9, 15, 0, 0, time.UTC)

	out, err := setupOutput(OutputConfig{
		Dir:              dir,
		FilenameTemplate: "<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md",
		MeetingURL:       "https://teams.live.com/meet/9367043528176?p=SuperSecretPasscode999#fragToken=abc",
		MeetingTitle:     "Prior Day Architecture Review",
		Category:         "ENG",
		Now:              captureNow,
		MeetingStart:     meetingStart,
		CaptionsOnly:     true,
	})
	if err != nil {
		t.Fatalf("setupOutput: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	wantFilename := "ENG-PriorDayArchitectureReview-transcript-20261113.md"
	if filepath.Base(out.TranscriptPath) != wantFilename {
		t.Fatalf("filename = %q, want %q (using MeetingStart date 20261113, not captureNow 20261114)", filepath.Base(out.TranscriptPath), wantFilename)
	}

	data, err := os.ReadFile(out.TranscriptPath)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "**Meeting Start:** 2026-11-13 09:15 UTC") {
		t.Errorf("expected Meeting Start 2026-11-13 09:15 UTC in header, got:\n%s", content)
	}
	if !strings.Contains(content, "**Source:** [Teams meeting recap](https://teams.live.com/meet/9367043528176)") {
		t.Errorf("expected redacted Source URL in header, got:\n%s", content)
	}
	for _, secret := range []string{"SuperSecretPasscode999", "p=", "fragToken"} {
		if strings.Contains(content, secret) {
			t.Errorf("transcript header leaked %q:\n%s", secret, content)
		}
	}
}
