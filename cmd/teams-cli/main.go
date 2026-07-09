// Command teams-cli is the go-teams command-line entrypoint for autonomous
// Microsoft Teams live-transcription and audio capture.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/softwaresalt/go-teams/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := cli.NewRootCommand()
	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
