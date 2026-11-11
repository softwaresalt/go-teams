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
* `--output`, `-o`: Markdown file (`.md` or `.markdown`) or output directory
* `--output-file`: explicit Markdown destination file
* `--output-dir`: output directory, even if its name ends in `.md`
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
need to retain previous results. Directory output generates timestamped names
and rejects collisions. An existing directory passed to `--output` is always
treated as a directory. Positional URL and destination arguments override
their corresponding flags; output flag precedence is `--output-file`,
then `--output`, then `--output-dir`.

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

A `--config` flag overrides the config file path, which defaults to the
per-user configuration directory (for example `~/.config/teams-cli/config.yaml`).
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

## Configuration

The CLI reads a YAML configuration file:

```yaml
output_dir: /home/user/.config/teams-cli/transcripts
profile_dir: /home/user/.config/teams-cli/profile
bot_name: Transcription Agent
headless: true
capture_audio: false # true also writes an .mp3 for live meetings and Meeting Recaps
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
internal/updater/     Release lookup, SHA-256 verification, and executable swap
internal/version/     Build metadata and semantic version comparison
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
