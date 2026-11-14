//go:build unix

package capture

import (
	"os"
	"path/filepath"
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

		assertMode(t, dir, dirPerm)
		assertMode(t, out.TranscriptPath, filePerm)
		assertMode(t, out.AudioPath, filePerm)
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

		assertMode(t, parent, dirPerm)
		assertMode(t, out.TranscriptPath, filePerm)
		assertMode(t, out.AudioPath, filePerm)
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
		assertMode(t, out.TranscriptPath, filePerm)
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

		assertMode(t, out.TranscriptPath, filePerm)
	})
}
