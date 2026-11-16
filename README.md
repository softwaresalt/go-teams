# go-teams

`go-teams` is a standalone Go command-line tool that captures Microsoft Teams
live captions and Meeting Recaps and saves them as local Markdown files. It
compiles to a single static binary and drives your installed Chromium-based
browser directly over the Chrome DevTools Protocol through
[`chromedp`](https://github.com/chromedp/chromedp), so it needs no external
drivers or FFmpeg.

> [!TIP]
> New here? Follow the [Quick start](#quick-start) below, then read the
> step-by-step [user guide](docs/USER_GUIDE.md).

## Why go-teams exists

`go-teams` is built for legitimate context capture in AI agentic work
environments. It does not circumvent meeting security: it works only in
meetings and Meeting Recaps that you are already authorized to attend or view,
signed in as yourself.

Use it to capture requirements and action items, to record the impact that a
team or an organization's employees had on an outcome (evidence for your own
self-assessments), and to build a personal or team communications library that
an AI assistant can synthesize into summaries, plans, and next steps. Capture
only meetings you are authorized to attend, and follow your organization's
recording and consent policies.

## Key features

* Live captions capture to Markdown
* Meeting Recap and AI Summary export
* Companion MP3 audio recording
* Scheduled and protocol-driven capture (`auto`, `ephemeral-live`,
  `live-with-summary`, `past-recap`)
* Batch queue processing (`--queue`)
* Templated YAML configuration generation (`go-teams generate-config`)
* Verified cross-platform self-update (`go-teams update`)

## Quick start

Build `go-teams` (Go 1.26 or later, plus Chrome, Edge, or Brave installed):

```powershell
go build -o .\bin\go-teams.exe .\cmd\go-teams
```

Then follow three steps:

1. Create a commented configuration file:

   ```powershell
   .\bin\go-teams.exe generate-config
   ```

2. Sign in interactively once to cache your browser session:

   ```powershell
   .\bin\go-teams.exe auth --account alice@contoso.com
   ```

3. Capture live captions, or export a Meeting Recap:

   ```powershell
   .\bin\go-teams.exe captions "<meeting-or-recap-url>"
   ```

Transcripts are written to the `output_dir` you set in the generated
`config.yaml`. The [user guide](docs/USER_GUIDE.md) explains each step in detail.

## Commands

| Command | Purpose |
|---|---|
| `go-teams [meeting-url] [output-markdown-path]` | Capture directly |
| `go-teams captions [meeting-url] [output-markdown-path]` | Capture live captions or a Meeting Recap |
| `go-teams join <meeting-url> [output-markdown-path]` | Join and capture (also accepts `--url`) |
| `go-teams --queue <file>` | Run every meeting in a batch queue file (also `-q`) |
| `go-teams auth` | Sign in interactively and cache the session |
| `go-teams schedule` | Run as a daemon, joining meetings on a cron schedule |
| `go-teams generate-config` | Write a fully commented YAML configuration file |
| `go-teams update` | Update to the latest GitHub Release |
| `go-teams --version` | Print the installed version |

The [user guide](docs/USER_GUIDE.md#commands) lists every command, and
[Compiled executable and explicit output file](docs/USER_GUIDE.md#compiled-executable-and-explicit-output-file)
documents the capture flags.

Every command has detailed built-in help with examples. Run
`go-teams <subcommand> --help` or `go-teams help <subcommand>`, for example:

```text
go-teams join --help
go-teams schedule --help
go-teams help captions
```

Name a real subcommand. Passing the literal placeholder `[command]` (or any
other word that is not a subcommand) prints a short note followed by the root
help instead of subcommand help.

## Key rules at a glance

### Output target precedence

The most specific output setting wins: the positional output argument, then
`--output-file`, `--output`, `--output-dir`, `--output-folder`, and finally the
config keys `output_file`, `output_folder`, and `output_dir`. A directory flag
clears any `output_file` from the config. See the full rules in the
[user guide](docs/USER_GUIDE.md#output-target-precedence).

### Self-update

`go-teams update` replaces the running executable with a GitHub Release build
after verifying its SHA-256 digest against the release `SHA256SUMS`. Updates
take an inter-process lock next to the executable. See
[Self-update](docs/USER_GUIDE.md#self-update) for flags, locking, and errors.

#### Manual lock and backup recovery

Recover by hand only when no `go-teams update` is running and a lock error
persists: restore `<exe>` from `<exe>.old` if it is missing, delete `<exe>.lock`
and any `<exe>.lock.ctl`, then rerun `go-teams update`. The
[full procedure](docs/USER_GUIDE.md#manual-lock-and-backup-recovery) explains
each step and its warnings.

## Configuration

`go-teams` reads a YAML file from `--config` / `-f`, then `./config.yaml`,
then `config.yaml` next to the executable, then the per-user config directory.
Create a commented template with `go-teams generate-config`; the same template
ships as [`config.example.yaml`](config.example.yaml). The
[user guide](docs/USER_GUIDE.md#configuration) documents every key and alias.

Command-line flags override top-level `config.yaml` settings. In
`go-teams schedule`, global `schedule` flags are applied to the base
configuration first, and the per-meeting `meetings:` settings in `config.yaml`
then override those flags for that entry. Base `ephemeral-live` normalization
(`ai_summary_delay: 0s`, `capture_audio: true`) also runs when the global
`schedule` flags are applied, so changing a meeting entry's `mode` away from an
`ephemeral-live` base requires explicit per-meeting `ai_summary_delay` or
`capture_audio` values to override the normalized base. See
[Configuration](docs/USER_GUIDE.md#configuration) and
[Built-in daemon scheduling](docs/USER_GUIDE.md#built-in-daemon-scheduling-go-teams-schedule)
for the full precedence rules.

## Documentation

* [`docs/USER_GUIDE.md`](docs/USER_GUIDE.md): step-by-step setup, complete
  command and flag reference, and the YAML configuration manual
* [`config.example.yaml`](config.example.yaml): commented configuration template
* [`.env.example`](.env.example): tracked synthetic template for gitignored
  `.env.local.test` integration and E2E test environment overrides
* [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md): package map and dependency
  direction
* [`docs/QUALITY_SCORE.md`](docs/QUALITY_SCORE.md): per-package quality grades
* [`docs/product-specs/Go-TeamsTranscriptCLI-Architecture.md`](docs/product-specs/Go-TeamsTranscriptCLI-Architecture.md):
  full architectural blueprint
* [Capture design](docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md):
  how live captions and Meeting Recaps are captured

## Development

```bash
go build ./...        # Build
go test ./...         # Run unit tests
go vet ./...          # Static analysis
gofmt -l .            # Format check
golangci-lint run     # Lint
```

Integration and E2E test setup (including copying [`.env.example`](.env.example)
to gitignored `.env.local.test`) is covered in the
[user guide](docs/USER_GUIDE.md#development).
