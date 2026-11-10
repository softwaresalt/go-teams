package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCompiledBinaryCapturesRecapToExplicitOutput(t *testing.T) {
	binary := os.Getenv("GO_TEAMS_BINARY")
	if binary == "" {
		t.Skip("set GO_TEAMS_BINARY to a freshly built teams-cli executable")
	}
	binary, err := filepath.Abs(binary)
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, err := w.Write([]byte(`<!doctype html><html><body>
<div class="ms-List"><div class="ms-List-cell" data-transcript-entry-id="synthetic">
<span data-transcript-copy-field="speaker">Presenter</span>
<span data-transcript-copy-field="timestamp">PT3S</span>
<span class="entryText-synthetic">Synthetic compiled binary transcript.</span>
</div></div>
<button aria-label="Copy all"
 onclick="navigator.clipboard.writeText('Synthetic compiled binary AI summary.')">Copy all</button>
</body></html>`))
		if err != nil {
			t.Errorf("write fixture: %v", err)
		}
	}))
	defer srv.Close()

	for _, form := range []string{"root positional", "captions positional", "short flags", "join long flags", "output file", "output directory"} {
		t.Run(form, func(t *testing.T) {
			dir := t.TempDir()
			output := filepath.Join(dir, "nested", "result.md")
			configPath := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(configPath,
				[]byte("output_dir: unused\nprofile_dir: unused\nbot_name: Fixture Bot\n"), 0o600))
			url := srv.URL + "/l/meetingrecap"
			var args []string
			switch form {
			case "root positional":
				args = []string{url, output}
			case "captions positional":
				args = []string{"captions", url, output}
			case "short flags":
				args = []string{"-u", url, "-o", output}
			case "join long flags":
				args = []string{"join", "--url", url, "--output", output}
			case "output file":
				args = []string{"captions", "--url", url, "--output-file", output}
			case "output directory":
				output = filepath.Join(dir, "generated.md")
				args = []string{"captions", "--url", url, "--output-dir", output}
			}
			args = append(args, "--headless", "--profile-dir", filepath.Join(dir, "profile"),
				"--config", configPath)
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.Dir = dir
			log, err := cmd.CombinedOutput()
			require.NoError(t, err, "compiled capture failed: %s", log)
			if form == "output directory" {
				files, err := filepath.Glob(filepath.Join(output, "*.md"))
				require.NoError(t, err)
				require.Len(t, files, 1)
				output = files[0]
			}
			content, err := os.ReadFile(output)
			require.NoError(t, err)
			require.Contains(t, string(content), "**Presenter** (00:00:03):\n> Synthetic compiled binary transcript.")
			require.Contains(t, string(content), "\n---\n")
			require.Contains(t, string(content), "Synthetic compiled binary AI summary.")
		})
	}
}
