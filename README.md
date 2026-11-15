# go-teams

`teams-cli` is a standalone Go command-line tool that captures Microsoft Teams
live captions and Meeting Recaps and saves them as local Markdown files. It
compiles to a single static binary and drives your installed Chromium-based
browser directly over the Chrome DevTools Protocol through
[`chromedp`](https://github.com/chromedp/chromedp), so it needs no external
drivers or FFmpeg.

> [!TIP]
> New here? Follow the [Quick start](#quick-start) below, then read the
> step-by-step [user guide](docs/USER_GUIDE.md).

## Why teams-cli exists

`teams-cli` is built for legitimate context capture in AI agentic work
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
* Templated YAML configuration generation (`teams-cli generate-config`)
* Verified cross-platform self-update (`teams-cli update`)

## Quick start

Build `teams-cli` (Go 1.26 or later, plus Chrome, Edge, or Brave installed):

```powershell
go build -o .\bin\teams-cli.exe .\cmd\teams-cli
```

Then follow three steps:

1. Create a commented configuration file:

   ```powershell
   .\bin\teams-cli.exe generate-config
   ```

2. Sign in interactively once to cache your browser session:

   ```powershell
   .\bin\teams-cli.exe auth
   ```

3. Capture live captions, or export a Meeting Recap:

   ```powershell
   .\bin\teams-cli.exe captions "<meeting-or-recap-url>"
   ```

Transcripts are written to the `output_dir` you set in the generated
`config.yaml`. The [user guide](docs/USER_GUIDE.md) explains each step in detail.

## Commands

| Command | Purpose |
|---|---|
| `teams-cli [meeting-url] [output-markdown-path]` | Capture directly |
| `teams-cli captions [meeting-url] [output-markdown-path]` | Capture live captions or a Meeting Recap |
| `teams-cli join <meeting-url> [output-markdown-path]` | Join and capture (also accepts `--url`) |
| `teams-cli --queue <file>` | Run every meeting in a batch queue file (also `-q`) |
| `teams-cli auth` | Sign in interactively and cache the session |
| `teams-cli schedule` | Run as a daemon, joining meetings on a cron schedule |
| `teams-cli generate-config` | Write a fully commented YAML configuration file |
| `teams-cli update` | Update to the latest GitHub Release |
| `teams-cli --version` | Print the installed version |

The [user guide](docs/USER_GUIDE.md#commands) lists every command, and
[Compiled executable and explicit output file](docs/USER_GUIDE.md#compiled-executable-and-explicit-output-file)
documents the capture flags.

Every command has detailed built-in help with examples. Run
`teams-cli <subcommand> --help` or `teams-cli help <subcommand>`, for example:

```text
teams-cli join --help
teams-cli schedule --help
teams-cli help captions
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

`teams-cli update` replaces the running executable with a GitHub Release build
after verifying its SHA-256 digest against the release `SHA256SUMS`. Updates
take an inter-process lock next to the executable. See
[Self-update](docs/USER_GUIDE.md#self-update) for flags, locking, and errors.

#### Manual lock and backup recovery

Recover by hand only when no `teams-cli update` is running and a lock error
persists: restore `<exe>` from `<exe>.old` if it is missing, delete `<exe>.lock`
and any `<exe>.lock.ctl`, then rerun `teams-cli update`. The
[full procedure](docs/USER_GUIDE.md#manual-lock-and-backup-recovery) explains
each step and its warnings.

## Configuration

`teams-cli` reads a YAML file from `--config` / `-f`, then `./config.yaml`,
then `config.yaml` next to the executable, then the per-user config directory.
Create a commented template with `teams-cli generate-config`; the same template
ships as [`config.example.yaml`](config.example.yaml). The
[user guide](docs/USER_GUIDE.md#configuration) documents every key and alias.

Command-line flags override top-level `config.yaml` settings. In
`teams-cli schedule`, global `schedule` flags are applied to the base
configuration first, and the per-meeting `meetings:` settings in `config.yaml`
then override those flags for that entry. Base `ephemeral-live` normalization
(`ai_summary_delay: 0s`, `capture_audio: true`) also runs when the global
`schedule` flags are applied, so changing a meeting entry's `mode` away from an
`ephemeral-live` base requires explicit per-meeting `ai_summary_delay` or
`capture_audio` values to override the normalized base. See
[Configuration](docs/USER_GUIDE.md#configuration) and
[Built-in daemon scheduling](docs/USER_GUIDE.md#built-in-daemon-scheduling-teams-cli-schedule)
for the full precedence rules.

## Documentation

* [`docs/USER_GUIDE.md`](docs/USER_GUIDE.md): step-by-step setup, complete
  command and flag reference, and the YAML configuration manual
* [`config.example.yaml`](config.example.yaml): commented configuration template
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

Integration-test setup is covered in the
[user guide](docs/USER_GUIDE.md#development).
