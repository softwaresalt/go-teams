//go:build unix

package capture

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// withZeroUmask runs fn with the process umask cleared so the kernel does not
// mask the permission bits requested by the code under test. syscall.Umask
// mutates process-wide state, so tests using this helper must not call
// t.Parallel.
func withZeroUmask(t *testing.T, fn func()) {
	t.Helper()
	oldMask := syscall.Umask(0)
	defer syscall.Umask(oldMask)
	fn()
}

// assertMode fails the test when the file or directory at path does not carry
// exactly the wanted permission bits.
func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("mode of %s = %#o, want %#o", path, got, want)
	}
}

// TestPermissionConstantsAreOwnerOnly pins the contract values so an accidental
// loosening of dirPerm or filePerm fails immediately rather than only when the
// derived modes are compared against the same constants.
func TestPermissionConstantsAreOwnerOnly(t *testing.T) {
	if dirPerm != 0o700 {
		t.Errorf("dirPerm = %#o, want 0o700", dirPerm)
	}
	if filePerm != 0o600 {
		t.Errorf("filePerm = %#o, want 0o600", filePerm)
	}
}

func TestSetupOutputUnixPermissionsDirMode(t *testing.T) {
	withZeroUmask(t, func() {
		dir := filepath.Join(t.TempDir(), "nested", "captures")
		out, err := setupOutput(OutputConfig{
			Dir:        dir,
			MeetingURL: "https://teams.microsoft.com/meet/234567890",
		})
		if err != nil {
			t.Fatalf("setupOutput: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		assertMode(t, dir, 0o700)
		assertMode(t, filepath.Dir(dir), 0o700)
		assertMode(t, out.TranscriptPath, 0o600)
		assertMode(t, out.AudioPath, 0o600)
	})
}

func TestSetupOutputUnixPermissionsExplicitFilePath(t *testing.T) {
	withZeroUmask(t, func() {
		parent := filepath.Join(t.TempDir(), "explicit-parent")
		out, err := setupOutput(OutputConfig{
			FilePath:   filepath.Join(parent, "call.md"),
			MeetingURL: "https://teams.microsoft.com/meet/234567890",
		})
		if err != nil {
			t.Fatalf("setupOutput: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		assertMode(t, parent, 0o700)
		assertMode(t, out.TranscriptPath, 0o600)
		assertMode(t, out.AudioPath, 0o600)
	})
}

func TestSetupOutputUnixPreservesPreExistingDirPermissions(t *testing.T) {
	withZeroUmask(t, func() {
		preExistingDir := filepath.Join(t.TempDir(), "pre-existing")
		if err := os.Mkdir(preExistingDir, 0o750); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}
		out, err := setupOutput(OutputConfig{
			Dir:          preExistingDir,
			MeetingURL:   "https://teams.microsoft.com/meet/234567890",
			CaptionsOnly: true,
		})
		if err != nil {
			t.Fatalf("setupOutput: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		assertMode(t, preExistingDir, 0o750)
		assertMode(t, out.TranscriptPath, 0o600)
	})
}

func TestAppendAISummaryToFileUnixPreservesFilePermissions(t *testing.T) {
	withZeroUmask(t, func() {
		out, err := setupOutput(OutputConfig{
			Dir:          t.TempDir(),
			MeetingURL:   "https://teams.microsoft.com/meet/234567890",
			CaptionsOnly: true,
		})
		if err != nil {
			t.Fatalf("setupOutput: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		if err := AppendAISummaryToFile(out.TranscriptPath, "Key decisions made."); err != nil {
			t.Fatalf("AppendAISummaryToFile: %v", err)
		}

		assertMode(t, out.TranscriptPath, 0o600)

		content, err := os.ReadFile(out.TranscriptPath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if !strings.Contains(string(content), "# AI Summary") || !strings.Contains(string(content), "Key decisions made.") {
			t.Errorf("transcript missing AI summary content:\n%s", content)
		}
	})
}

func TestAppendAISummaryToFileUnixPreservesExistingLooserMode(t *testing.T) {
	withZeroUmask(t, func() {
		out, err := setupOutput(OutputConfig{
			Dir:          t.TempDir(),
			MeetingURL:   "https://teams.microsoft.com/meet/234567890",
			CaptionsOnly: true,
		})
		if err != nil {
			t.Fatalf("setupOutput: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if err := os.Chmod(out.TranscriptPath, 0o640); err != nil {
			t.Fatalf("Chmod: %v", err)
		}

		if err := AppendAISummaryToFile(out.TranscriptPath, "Key decisions made."); err != nil {
			t.Fatalf("AppendAISummaryToFile: %v", err)
		}

		assertMode(t, out.TranscriptPath, 0o640)

		content, err := os.ReadFile(out.TranscriptPath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if !strings.Contains(string(content), "# AI Summary") || !strings.Contains(string(content), "Key decisions made.") {
			t.Errorf("transcript missing AI summary content:\n%s", content)
		}
	})
}
