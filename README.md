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

The browser capture MVP supports live captions, Meeting Recap transcripts,
and available Teams AI summaries. It supports headed sign-in and headless
automation with a previously authenticated browser profile. Teams DOM changes,
tenant permissions, admission, and recap availability can affect capture.
See the [capture design](docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md).

## Requirements

* Go 1.26 or later to build the executable
* A Chromium-based browser (Chrome, Edge, or Brave) installed on the host

## Commands

```text
teams-cli [meeting-url] [output-markdown-path]          Capture directly
teams-cli captions [meeting-url] [output-markdown-path] Capture live captions or a Meeting Recap
teams-cli join <meeting-url> [output-markdown-path]     Join and capture (also accepts --url)
teams-cli --queue <file>         Run every meeting listed in a batch queue file (also -q)
teams-cli auth                   Sign in interactively and cache the session
teams-cli schedule               Run as a daemon, joining meetings on a cron schedule
teams-cli update                 Update teams-cli to the latest GitHub Release
teams-cli --version              Print the installed version
```

### Quick Start (Interactive Live Captions MVP)

Run `teams-cli captions` (with an optional meeting link) to open a browser
window, sign in to Microsoft Teams manually, enter your call, and automatically
enable and record live captions to a local Markdown file:

```bash
# Open Teams in a browser window, sign in, join any call, and save captions to ./transcripts:
go run ./cmd/teams-cli captions --output-dir ./transcripts

# Or navigate directly to a specific Teams meeting URL:
go run ./cmd/teams-cli captions "https://teams.microsoft.com/meet/synthetic" --output-dir ./transcripts
```

### Compiled executable and explicit output file

Build on Windows, then pass both the input URL and destination file:

```powershell
go build -o .\bin\teams-cli.exe .\cmd\teams-cli
.\bin\teams-cli.exe "https://teams.microsoft.com/meet/synthetic" ".\transcripts\call.md"
.\bin\teams-cli.exe captions "https://teams.microsoft.com/meet/synthetic" ".\transcripts\call.md"
.\bin\teams-cli.exe -u "https://teams.microsoft.com/meet/synthetic" -o ".\transcripts\call.md"
```

These are synthetic URLs; replace them with a meeting or Meeting Recap URL you
are authorized to access. Quote URLs (especially those containing `&`) and
paths containing spaces. Absolute destinations such as
`C:\captures\call.md` are also accepted.

Flags for the root command, `captions`, and `join`:

* `--url`, `-u`: input meeting or Meeting Recap URL
* `--queue`, `-q`: batch queue file that lists one capture per line (see
  [Batch queue mode](#batch-queue-mode))
* `--output`, `-o`: Markdown file (`.md` or `.markdown`) or output directory
* `--output-file`: explicit Markdown destination file
* `--output-dir`: output directory, even if its name ends in `.md`
* `--output-folder`: default folder for generated transcript names; behaves
  like `--output-dir` with the lowest flag precedence
* `--category`, `-c`: optional meeting category written to the transcript
  header and used by the `<category>` filename placeholder
* `--filename-template`: filename pattern used when output goes to a directory
* `--name-template`: alias for `--filename-template`
* `--profile-dir`: browser user-data directory to persist login state across runs
* `--bot-name`: display name injected when joining as an unauthenticated guest
* `--browser-path`: explicit Chrome, Edge, or Brave executable path (also reads `TEAMS_BROWSER_PATH`)
* `--headed` / `--headless`: control whether the browser window is visible
* `--audio`: also capture meeting or recap audio to an `.mp3` file alongside the `.md` transcript
* `--recap-audio`: also capture Meeting Recap audio to an `.mp3` file alongside the `.md` transcript

Meeting Recap URLs write only the `.md` transcript by default. Enabling audio
with `--audio`, `--recap-audio`, or `capture_audio: true` (alias `audio: true`)
in the config file also writes a companion `.mp3` file next to the transcript,
extracted from the recording that belongs to the requested Recap. Recap audio
is rejected rather than captured when its recording identity cannot be verified
or when it exceeds the supported size or duration limits.

`--browser-path` and `--profile-dir` are also accepted by `auth` and `schedule`,
and `browser_path` in the config file applies to every command.

Parent output directories are created automatically. Explicit file destinations
are replaced on each successful capture admission; use unique paths if you
need to retain previous results. Directory output generates names from the
filename template and rejects collisions. An existing directory passed to
`--output` is always treated as a directory. Positional URL and destination
arguments override their corresponding flags.

### Output target precedence

When several output settings are present, the most specific one wins. From
highest to lowest precedence:

1. positional output argument (`arg[1]`)
2. `--output-file`
3. `--output`
4. `--output-dir`
5. `--output-folder`
6. config `output_file`
7. config `output_folder`
8. config `output_dir`

A directory-selecting flag (`--output-dir`, `--output-folder`, or an `--output`
value that is a directory) clears any `output_file` from the config file, so a
command-line directory always replaces a configured file target. Config
`output` and `default_path` follow the rules in [Configuration](#configuration).

### Filename templates

When output goes to a directory, the transcript file name comes from the
filename template. The default template is:

```text
<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md
```

Set a different pattern with `--filename-template` (or its alias
`--name-template`) or the `filename_template` config key. Supported
placeholders:

| Placeholder | Value |
|---|---|
| `<category>` | Value of `--category` / `category` |
| `<concat-meeting-name>` | Meeting title words joined without separators, with filler words dropped |
| `<meeting-name>` | Meeting title sanitized for use in a file name |
| `<meeting-id>` | Identifier derived from the meeting URL, used when the title is unavailable |
| `<YYYYMMDD>` | Capture date, for example `20261112` |
| `<YYYY-MM-DD>` | Capture date with dashes |
| `<HHMMSS>` | Capture time |
| `<timestamp>` | Full date and time stamp |

### Batch queue mode

`--queue <file>` (short form `-q`) captures several meetings in one run. It is
accepted on the root command, `captions`, and `join`:

```powershell
.\bin\teams-cli.exe --queue .\transcript-queue.txt --output-folder ".\transcripts" --headless
```

Each queue file line is one capture invocation:

```text
# Blank lines are ignored
"https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic-1" --category standup
teams-cli.exe captions "https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic-2" -o "C:\out\review.md"
:: "https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic-3"
```

* Empty lines and lines that start with `::` (after leading whitespace) are
  skipped. A leading `teams-cli` or `teams-cli.exe` token is stripped, so you
  can paste a full command line.
* Single quotes and double quotes group arguments that contain spaces. Windows
  backslash paths pass through unchanged; inside double quotes, `\"` produces a
  literal quote. An unterminated quote fails that line.
* A line cannot contain `--queue` or `-q`; nested queues are rejected.
* Every line runs as its own isolated invocation. A failing line is logged and
  skipped, the remaining lines still run, and all failures are returned
  together at the end.
* After a line succeeds, the CLI comments it out in place by prepending `::`,
  so rerunning the same queue file skips completed meetings. The update goes
  through `writeQueueLines` (`internal/cli/queue.go`), which writes a temporary
  file next to the queue file and renames it over the original. If the
  temporary file or rename fails, it falls back to an in-place `os.WriteFile`
  with `0o600` permissions. Line endings (LF or CRLF) are preserved.
* Capture flags on the outer command are forwarded to every line
  (`inheritedQueueFlags`): `--config`, `--output-folder`, `--output-dir`,
  `--output`, `--output-file`, `--filename-template` / `--name-template`,
  `--profile-dir`, `--bot-name`, `--category`, `--browser-path`, `--headless`,
  `--headed`, and `--audio` / `--recap-audio`. The outer `--url` and `--queue`
  are never forwarded. Flags written on a line come after the inherited ones
  and take precedence.
* Queue logs print every URL as `scheme://host/path` only. Query strings
  (including passcodes), userinfo, and fragments are removed by
  `teams.RedactArgs`.

Pointing every line at one explicit `--output-file` replaces that file on each
capture, so use `--output-folder` or per-line destinations for batches.

### Browser security posture

* Meeting URLs must use `https` and a Microsoft Teams, SharePoint, or Microsoft
  365 host (`teams.microsoft.com`, `teams.live.com`, `teams.cloud.microsoft`,
  `sharepoint.com`, `microsoft.com`, `office.com`, `sfx.ms`, `aka.ms`, and their
  subdomains). Loopback hosts (`localhost`, `127.0.0.1`, `::1`) may use `http`
  or `https` for local test fixtures. URLs with embedded credentials are
  rejected. The check runs before any browser launches and covers the initial
  URL only; redirects inside the browser are not intercepted.
* Passcodes (`?p=`), userinfo, query strings, and fragments are stripped from
  navigation errors and queue logs.
* The Chromium sandbox stays enabled. Set `TEAMS_DISABLE_SANDBOX=1` (or `true`)
  only in containers that cannot run it.

### Headless automation

Sign in using a visible capture session first and reuse that profile:

```powershell
.\bin\teams-cli.exe captions --profile-dir ".\bin\profile"
.\bin\teams-cli.exe captions --url "https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic" --output-file ".\transcripts\recap.md" --profile-dir ".\bin\profile" --headless
```

For zero-config runs, an existing profile next to the executable is preferred
over the working-directory `bin\profile`. Use an explicit `--profile-dir` for
scheduled automation. Do not run two browser instances against the same profile.
Headless mode cannot complete interactive login, MFA, or lobby admission for
you. Live capture continues until cancellation; recap capture exits after
exporting available transcript entries and the optional AI summary, separated
by a Markdown horizontal rule. AI summary discovery retries up to 20 times
with 500 ms between attempts; transcript export still succeeds if no summary
is available.

A `--config` flag overrides the config file path; see [Configuration](#configuration)
for the default discovery order.
When no config file exists on disk, `teams-cli` uses built-in defaults and opens
a visible browser window automatically.

### Self-update

`teams-cli update` replaces the running executable with a published GitHub
Release build from `softwaresalt/go-teams`:

```powershell
.\bin\teams-cli.exe --version                  # print the installed version
.\bin\teams-cli.exe update --check             # report whether a newer release exists
.\bin\teams-cli.exe update                     # install the latest release
.\bin\teams-cli.exe update --version v1.2.0    # install a specific release tag
.\bin\teams-cli.exe update --version v1.1.0 --force   # reinstall or downgrade
```

* `--check` only reports availability and never modifies the executable
* `--version <tag>` installs that release instead of the latest
* `--force` reinstalls the target version even when it is already current, and
  is required to downgrade to an older release

Every update downloads the release's `SHA256SUMS` asset and verifies the
platform binary's SHA-256 digest before touching the installed executable. A
missing `SHA256SUMS`, a missing entry for your platform, or a digest mismatch
aborts the update and leaves the current binary untouched. Downloads must use
`https`, and release metadata and assets are size-capped.

The replacement uses a rename sequence that also works for a running Windows
`.exe`: the current binary is renamed to `teams-cli.exe.old`, the verified
download is moved into place, and the `.old` file is restored automatically if
the final move fails. The `.old` file is removed after a successful update, or
on the next update if Windows still holds it open. The executable's directory
must be writable by the current user; a build installed under a privileged
folder such as `C:\Program Files` needs an elevated shell.

Local builds report the version `dev` and treat any release as an upgrade.
Release builds embed their version through the `.github/workflows/release.yml`
link flags.

#### Update locking and concurrent updates

Two `teams-cli update` processes cannot swap the same executable at once. Each
update takes an inter-process lock next to the executable before it modifies
anything:

* `<exe>.lock` is created with `O_CREATE|O_EXCL` and `0o600` permissions. It
  holds JSON owner metadata: `pid`, a random `token`, and `created_at`.
* A contender polls every 50 ms and gives up after 15 seconds with
  `ErrUpdateLocked`.
* The lock fails closed. A lock older than 2 minutes only changes the
  diagnostic message (an orphaned dead PID versus a live or reused PID);
  `teams-cli` never evicts or deletes another process's lock on its own.
* When the target path itself ends in `.old` (for example when you launch the
  backup copy), `acquireSwapLockChain` locks every overlapping path in order,
  shortest prefix first, so two updaters cannot deadlock or overlap. On Windows
  and macOS the `.old` suffix is matched case-insensitively.
* Executable names that end in `.lock` or `.lock.ctl` are rejected before
  any change, because a lock marker could otherwise overwrite a live
  executable.
* Releasing the lock takes a short-lived `<exe>.lock.ctl` control lock, rereads
  the lock file, and removes `<exe>.lock` only when the stored token matches.
  The release ignores cancellation of the update context
  (`context.WithoutCancel`) and waits at most 2 seconds, polling every 10 ms,
  so pressing Ctrl+C does not leave a stale lock behind.

While another update holds the lock, a process that finds the executable
missing waits (up to the same 15 seconds) for the swap to restore it. This
covers the brief window in which the running binary has been renamed to
`.old` and the new binary is not yet in place; symlinked install paths are
resolved first so the wait follows the real target.

Before replacing the binary, the updater also verifies identity under the
lock, so it never overwrites a binary it did not launch from:

* `ErrConcurrentUpdateModified` means the executable changed since the process
  started: it was deleted, replaced, or no longer matches the launch snapshot,
  the running image, or the `.old` provenance. Rerun `teams-cli update`.
* `ErrDowngrade` means the installed binary is already newer than the
  requested release. Pass `--force` to downgrade deliberately.
* `ErrUpdateLocked` means another update is in progress or left a lock
  behind. `ErrConcurrentUpdateModified` wraps it, so both satisfy
  `errors.Is(err, ErrUpdateLocked)`.

The installed binary's own version comes from running `<exe> --version` with a
30-second timeout and a 2-second output cleanup delay, so a stalled binary
cannot hold the lock indefinitely.

#### Manual lock and backup recovery

Only recover by hand when no `teams-cli update` is running and the lock error
persists. Follow these steps in order:

1. Confirm that every `teams-cli update` process has exited. Check the PID
   named in the error message; the operating system can reuse a PID after a
   crash, so a live PID does not by itself prove an update is running.
2. If `<exe>` is missing and `<exe>.old` exists, restore the backup by
   renaming `<exe>.old` back to `<exe>`.
3. Remove `<exe>.lock` and any `<exe>.lock.ctl`.
4. Run `teams-cli update` again.

> [!WARNING]
> Never delete `<exe>.lock` while an update might still be running. Removing
> the lock early lets a second process swap the executable concurrently.

## Configuration

The CLI reads a YAML configuration file. Pass `--config <path>` (`-f`) to use a
specific file. Without it, `teams-cli` uses the first file that exists:

1. `./config.yaml` in the working directory
2. `config.yaml` next to the executable
3. the per-user config directory: `%APPDATA%\teams-cli\config.yaml` on
   Windows, `~/.config/teams-cli/config.yaml` on Linux

When none exists, built-in defaults apply. Keys you omit keep their built-in
defaults. A sample file lives in [`config.example.yaml`](config.example.yaml).

```yaml
output_dir: /home/user/.config/teams-cli/transcripts
# output_file: /home/user/transcripts/standup.md   # fixed file path; takes precedence over output_dir
filename_template: "<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md"
category: standup
profile_dir: /home/user/.config/teams-cli/profile
browser_path: /usr/bin/google-chrome
bot_name: Transcription Agent
headless: true
capture_audio: false # true also writes an .mp3 for live meetings and Meeting Recaps
meetings:
  - name: standup
    url: https://teams.microsoft.com/l/meetup-join/...
    cron: "0 9 * * 1-5"
```

| Key | Alias | Meaning |
|---|---|---|
| `output_dir` | | Directory for generated transcript file names |
| `output_folder` | `default_path` | Directory-only aliases for `output_dir`; `output_folder` wins over `default_path` |
| `output` | | File-or-directory target (see below) |
| `output_file` | | Explicit Markdown file path; overrides directory naming |
| `filename_template` | `name_template` | Filename pattern for directory output |
| `meeting_url` | `url` | Meeting or Meeting Recap URL to capture |
| `category` | | Category for the transcript header and filename |
| `profile_dir` | | Browser user-data directory |
| `browser_path` | | Chrome, Edge, or Brave executable |
| `bot_name` | | Display name for guest joins |
| `headless` | `headed` | Browser visibility; `headed: true` forces a visible window |
| `capture_audio` | `audio` | Also write an `.mp3` next to the transcript |
| `meetings` | | Scheduled meetings (`name`, `url`, `cron`) used by `schedule` |

`output_folder` and `default_path` only ever set the output directory. The
separate `output` key accepts either a file or a directory. When `config.Load`
reads it, a value ending in `.md` or `.markdown` unconditionally sets
`output_file`, overriding any `output_file` in the same config file. Any
other value sets `output_dir`. Because `output` is read after the other
aliases, it also overrides `output_folder` and `default_path`. Validation
requires `output_dir`, `profile_dir`, and `bot_name` to be non-empty, and every
scheduled meeting needs a `url` and a `cron` expression.

Omitting both `headless` and `headed` opens a visible browser window, so
interactive sign-in works out of the box. Set `headless: true` for unattended
runs. Command-line flags override every config value (see
[Output target precedence](#output-target-precedence)).

## Project layout

```text
cmd/teams-cli/        CLI entrypoint
internal/audio/       PCM-to-MP3 encoder for meeting and recap audio
internal/browser/     chromedp allocator, stealth flags, live meeting and recap runner
internal/capture/     Capture session, output paths, filename templates, and file writes
internal/cli/         Cobra command wiring and batch queue runner
internal/config/      Configuration model and YAML persistence
internal/scheduler/   Cron schedule construction
internal/teams/       Teams meeting URL validation, mutation, and redaction
internal/transcript/  Caption deduplication and Markdown writer
internal/updater/     Release lookup, SHA-256 verification, locking, and executable swap
internal/version/     Build metadata and semantic version comparison
```

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the dependency direction
between these packages and [`docs/QUALITY_SCORE.md`](docs/QUALITY_SCORE.md) for
per-package quality grades.

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

The local synthetic-page end-to-end harness verifies browser caption delivery.
The opt-in compiled-executable harness also verifies recap and AI summary
output without connecting to a real tenant:

```powershell
go build -o .\bin\teams-cli.exe .\cmd\teams-cli
$env:GO_TEAMS_BINARY = (Resolve-Path .\bin\teams-cli.exe).Path
go test ./internal/cli -run TestCompiledBinaryCapturesRecapToExplicitOutput -count=1
```

Synthetic checks do not qualify real Teams login, tenant policy, audio fidelity,
or recap completeness. Those require an authorized operator-assisted live run.
