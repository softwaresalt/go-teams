# go-teams

`go-teams` is a standalone Go CLI that joins Microsoft Teams meetings through a
headless Chromium browser, captures live captions, and persists transcripts to
local Markdown files. It compiles to a single static binary and drives the host
browser directly over the Chrome DevTools Protocol via
[`chromedp`](https://github.com/chromedp/chromedp), so it needs no external
drivers or FFmpeg.

The full architectural blueprint lives in
[`docs/product-specs/Go-TeamsTranscriptCLI-Architecture.md`](docs/product-specs/Go-TeamsTranscriptCLI-Architecture.md).

## Status

This is the foundation release: the CLI skeleton, configuration, URL handling,
caption deduplication, Markdown rendering, cron scheduling, and browser
allocator flags are implemented and unit-tested. Live meeting automation (the
pre-join finite state machine, caption/audio JavaScript payloads, and MP3
encoding) is scaffolded behind interfaces and lands in follow-on releases.

## Requirements

- Go 1.26 or later (driven by the `chromedp` dependency)
- A Chromium-based browser (Chrome, Edge, or Brave) installed on the host

## Commands

```text
teams-cli auth               Sign in interactively and cache the session
teams-cli join <meeting-url> Join a meeting immediately from a URL
teams-cli schedule           Run as a daemon, joining meetings on a cron schedule
```

A `--config` flag overrides the config file path, which defaults to the
per-user configuration directory (for example `~/.config/teams-cli/config.yaml`).

## Configuration

The CLI reads a YAML configuration file:

```yaml
output_dir: /home/user/.config/teams-cli/transcripts
profile_dir: /home/user/.config/teams-cli/profile
bot_name: Transcription Agent
headless: true
meetings:
  - name: standup
    url: https://teams.microsoft.com/l/meetup-join/...
    cron: "0 9 * * 1-5"
```

## Project layout

```text
cmd/teams-cli/        CLI entrypoint
internal/cli/         Cobra command wiring
internal/config/      Configuration model and YAML persistence
internal/teams/       Teams meeting URL mutation
internal/transcript/  Caption deduplication and Markdown writer
internal/scheduler/   Cron schedule construction
internal/browser/     chromedp allocator, stealth flags, and runner
```

## Development

```bash
go build ./...        # Build
go test ./...         # Run unit tests
go vet ./...          # Static analysis
gofmt -l .            # Format check
golangci-lint run     # Lint
```

### Integration tests

Live browser and meeting behavior is covered by an `integration`-tagged harness.
The tests skip by default and only run when explicitly enabled, so they never
launch a browser in unattended CI:

```bash
# Compile the harness (CI runs this to prevent bit-rot):
go vet -tags=integration ./...

# Browser launch smoke test (requires a Chromium-based browser):
GO_TEAMS_INTEGRATION=1 go test -tags=integration ./internal/browser/ -run TestChromeRunnerLaunch

# Live meeting contract tests (currently red — pending live-capture work):
GO_TEAMS_INTEGRATION=1 TEAMS_MEETING_URL="https://teams.microsoft.com/l/meetup-join/..." \
  go test -tags=integration ./internal/browser/
```

| Env var | Purpose |
|---|---|
| `GO_TEAMS_INTEGRATION` | Set to any non-empty value to enable integration tests |
| `TEAMS_MEETING_URL` | A joinable Teams meeting URL for the live meeting tests |

The `Meeting` contract (`Join`, `EnableCaptions`, `CaptureAudio`) is scaffolded
and returns `ErrNotImplemented`; the live meeting tests are red until the
live-capture release units implement it.
