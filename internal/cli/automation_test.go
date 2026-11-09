package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/config"
)

func TestAutomationCaptureArguments(t *testing.T) {
	url := "https://teams.microsoft.com/meet/synthetic"
	for _, command := range []string{"", "captions", "join"} {
		for _, form := range []string{"positional", "short flags", "long flags", "file flag", "directory flag", "generic directory"} {
			t.Run(command+"/"+form, func(t *testing.T) {
				output := filepath.Join(t.TempDir(), "capture.md")
				if form == "generic directory" {
					output = filepath.Join(t.TempDir(), "new-output")
				}
				var got config.Config
				rec := &captureRecorder{}
				d := depsWithCapture(&fakeRunner{}, rec)
				d.loadConfig = func(string) (config.Config, error) {
					cfg := validConfig()
					cfg.OutputFile = "configured.md"
					return cfg, nil
				}
				d.newMeeting = func(ctx context.Context, cfg config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
					got = cfg
					return nil, ctx, func() {}, nil
				}
				var args []string
				if command != "" {
					args = append(args, command)
				}
				switch form {
				case "positional":
					args = append(args, url, output)
				case "short flags":
					args = append(args, "-u", url, "-o", output)
				case "long flags":
					args = append(args, "--url", url, "--output", output)
				case "file flag":
					args = append(args, "--url", url, "--output-file", output)
				case "directory flag":
					args = append(args, "--url", url, "--output-dir", output)
				case "generic directory":
					args = append(args, "--url", url, "--output", output)
				}
				args = append(args, "--headless")
				require.NoError(t, runCmd(context.Background(), d, args...))
				require.True(t, rec.called)
				require.True(t, got.Headless)
				if form == "directory flag" || form == "generic directory" {
					require.Equal(t, output, got.OutputDir)
					require.Empty(t, got.OutputFile, "an explicit directory must override a configured file, even with a .md suffix")
				} else {
					require.Equal(t, output, got.OutputFile)
				}
			})
		}
	}
}
