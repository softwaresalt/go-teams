---
title: go-teams user guide
description: Step-by-step setup, configuration, and complete command reference for go-teams
---

This guide takes you from a fresh install to your first captured meeting, then
documents every command, flag, and configuration key. If you are new to
`go-teams`, read the first four sections in order. If you already use it, jump
to the section you need through the contents list below.

* [Why go-teams exists](#why-go-teams-exists)
* [Before you begin](#before-you-begin)
* [Getting started](#getting-started)
* [Generating a configuration file](#generating-a-configuration-file)
* [Commands](#commands)
* [Current capabilities and limits](#current-capabilities-and-limits)
* [Quick Start (Interactive Live Captions MVP)](#quick-start-interactive-live-captions-mvp)
* [Compiled executable and explicit output file](#compiled-executable-and-explicit-output-file)
* [Output target precedence](#output-target-precedence)
* [Filename templates](#filename-templates)
* [Batch queue mode](#batch-queue-mode)
* [Scheduled and protocol-driven capture](#scheduled-and-protocol-driven-capture)
* [Browser security posture](#browser-security-posture)
* [Headless automation](#headless-automation)
* [Self-update](#self-update)
* [Configuration](#configuration)
* [Project layout](#project-layout)
* [Development](#development)

## Why go-teams exists

`go-teams` is built for legitimate context capture in AI agentic work
environments. It is not a way around meeting security. It works only inside
meetings and Meeting Recaps that you are already authorized to attend or view,
using your own signed-in Microsoft account.

People who work with AI assistants need the substance of meetings in a form
those assistants can read. `go-teams` saves what was said as local Markdown so
you can:

* capture requirements and decisions while they are fresh
* collect action items and owners without retyping notes
* record the impact that a team or an organization's employees had on an
  outcome, as evidence for your own self-assessments
* build a personal or team library of communications that an AI assistant can
  later synthesize into summaries, plans, and next steps

> [!IMPORTANT]
> Capture only meetings you are authorized to attend, and follow your
> organization's recording, consent, and data-retention policies. Transcript
> files can contain sensitive business information, so store them accordingly.

## Before you begin

You need:

* a Windows, macOS, or Linux computer
* a Chromium-based browser (Microsoft Edge, Google Chrome, or Brave) installed
  on that computer
* a Microsoft 365 account that can attend the meetings you want to capture
* Go 1.26 or later, only if you build `go-teams` from source instead of using
  a published release build

## Getting started

This walkthrough takes about ten minutes. Commands are shown for Windows
PowerShell. On macOS and Linux, use the same commands with `/` in paths and
without the `.exe` suffix.

### Step 1: Get the executable

Build `go-teams` from a copy of the source code:

```powershell
go build -o .\bin\go-teams.exe .\cmd\go-teams
```

The rest of this guide writes `go-teams` for the executable. Use the full path
(for example `.\bin\go-teams.exe`) unless you place the file in a folder on
your `PATH`. Confirm that it runs:

```powershell
.\bin\go-teams.exe --version
```

### Step 2: Create your configuration file

A configuration file lets you set your transcript folder, your Microsoft
account, and your preferred defaults once, instead of typing them each time.
Generate a ready-to-edit file in the current folder:

```powershell
.\bin\go-teams.exe generate-config
```

The command prints the path of the file it wrote, `config.yaml` by default.
Every setting in the file is turned off and explained in plain language. See
[Generating a configuration file](#generating-a-configuration-file) for all
options.

### Step 3: Choose your settings

Open `config.yaml` in any text editor. To turn a setting on, delete the `# `
at the start of its line and change the value. A good first set is:

```yaml
output_dir: "C:\\Transcripts\\Team"
default_account: "synthetic-user@example.com"
category: "TEAM"
```

Save the file. Leave every other line commented out until you need it.

> [!NOTE]
> Until you set `output_dir`, transcripts go to the per-user default folder
> (`%APPDATA%\go-teams\transcripts` on Windows). Until you turn on
> `headless: true`, `go-teams` opens a visible browser window, which is what
> you want for your first sign-in.

### Step 4: Sign in once

Run the sign-in command (using `default_account` from `config.yaml` or passing
`--account`). A browser window opens so you can sign in to Microsoft 365,
including any multi-factor prompt:

```powershell
.\bin\go-teams.exe auth --account synthetic-user@example.com
```

`--account` (or `default_account` in `config.yaml`) is required for `auth` and
whenever a cold profile without an existing Teams session is bootstrapped. Your
sign-in is saved in a dedicated browser profile folder, so later runs do not
ask again. Treat that folder like a password and never share it.

### Step 5: Capture a meeting

To record live captions, start the capture and then join your call in the
browser window that opens:

```powershell
.\bin\go-teams.exe captions
```

To capture a specific meeting or to export a finished meeting's Meeting Recap,
pass its link in quotes:

```powershell
.\bin\go-teams.exe captions "<meeting-or-recap-url>"
```

When the capture ends, `go-teams` writes a Markdown transcript to your
`output_dir`. Open it in any editor, or hand it to your AI assistant.

### Step 6: Run unattended (optional)

After one successful interactive sign-in, you can capture without a visible
window. Uncomment `headless: true` in `config.yaml`, or add `--headless` to the
command. To capture many meetings in one run, see [Batch queue mode](#batch-queue-mode).
To capture on a schedule, see
[Scheduled and protocol-driven capture](#scheduled-and-protocol-driven-capture).

## Generating a configuration file

`go-teams generate-config` (aliases `init-config` and `gen-config`) writes a
fully commented YAML configuration file. Every option that `go-teams` reads is
included, turned off, with a plain-language explanation and an example value.

```text
go-teams generate-config [output-dir] [--output-dir <dir>] [--name <file>] [--force]
```

| Flag or argument | Meaning |
|---|---|
| `[output-dir]` | Optional folder for the file, or a full `.yaml` / `.yml` file path |
| `--output-dir`, `-d` | Folder for the generated file (default `.`); wins over `[output-dir]` |
| `--name`, `-n` | Filename inside the folder (default `config.yaml`); `.yaml` is added when the name has no `.yaml` or `.yml` extension |
| `--force` | Replace an existing file at the destination |

Examples:

```powershell
# ./config.yaml in the current folder
go-teams generate-config

# a named file in a specific folder
go-teams generate-config -d "%LOCALAPPDATA%\go-teams" -n wsp

# a full file path as the positional argument
go-teams generate-config C:\teams\team.yaml
```

Rules to know:

* The command prints the path it wrote and creates missing folders. On Linux
  and macOS, new folders get mode `0o700` and the file gets mode `0o600`, so
  only you can read them. On Windows these modes do not apply; the new folder
  and file inherit the access control list of the parent folder.
* An existing file is never overwritten. The command stops with an error
  unless you pass `--force`, which replaces the file atomically.
* A positional argument that ends in `.yaml` or `.yml` is treated as the full
  file path, unless it is an existing folder or you also pass `--output-dir` or
  `--name`.
* The filename must be a plain name. Names that contain path separators, `..`,
  a drive letter or colon, no name before the extension, or a Windows reserved
  device name such as `NUL`, `CON`, `COM1`, or `LPT1` are rejected.
* The root `--config` / `-f` flag is not read by this command. The destination
  is set only by the options above.
* The generated file is valid as written. With every setting commented out,
  `go-teams` uses its built-in defaults: the per-user transcript folder and a
  visible browser window, until you uncomment `output_dir` or `headless: true`.
* A copy of the same template ships in the repository as
  [`config.example.yaml`](../config.example.yaml).

To use a generated file that is not in a default location, pass it with
`--config`. See [Configuration](#configuration) for the lookup order.

## Commands

```text
go-teams [meeting-url] [output-markdown-path]          Capture directly
go-teams captions [meeting-url] [output-markdown-path] Capture live captions or a Meeting Recap
go-teams join <meeting-url> [output-markdown-path]     Join and capture (also accepts --url)
go-teams --queue <file>         Run every meeting listed in a batch queue file (also -q)
go-teams auth                   Sign in interactively and cache the session
go-teams schedule               Run as a daemon, joining meetings on a cron schedule
go-teams generate-config        Write a fully commented YAML configuration file template
go-teams update                 Update go-teams to the latest GitHub Release
go-teams --version              Print the installed version
```

Every command has detailed built-in help with examples. Run
`go-teams <subcommand> --help` or `go-teams help <subcommand>`:

```text
go-teams join --help
go-teams captions --help
go-teams schedule --help
go-teams auth --help
go-teams generate-config --help
go-teams update --help
go-teams help schedule
```

The `[command]` shown in the `help` usage text is a placeholder, not a literal
argument. Passing it (or any other word that is not a subcommand or a valid
Teams meeting URL) prints a one-line note followed by the root help. The note
redacts any URL it echoes back.

## Current capabilities and limits

The browser capture MVP supports live captions, Meeting Recap transcripts,
and available Teams AI summaries. It supports headed sign-in and headless
automation with a previously authenticated browser profile. Teams DOM changes,
tenant permissions, admission, and recap availability can affect capture.
See the [capture design](design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md).

## Quick Start (Interactive Live Captions MVP)

Run `go-teams captions` (with an optional meeting link) to open a browser
window, sign in to Microsoft Teams manually, enter your call, and automatically
enable and record live captions to a local Markdown file:

```bash
# Open Teams in a browser window, sign in, join any call, and save captions to ./transcripts:
go run ./cmd/go-teams captions --account synthetic-user@example.com --output-dir ./transcripts

# Or navigate directly to a specific Teams meeting URL:
go run ./cmd/go-teams captions "https://teams.microsoft.com/meet/synthetic" --account synthetic-user@example.com --output-dir ./transcripts
```

## Compiled executable and explicit output file

Build on Windows, then pass both the input URL and destination file:

```powershell
go build -o .\bin\go-teams.exe .\cmd\go-teams
.\bin\go-teams.exe "https://teams.microsoft.com/meet/synthetic" ".\transcripts\call.md"
.\bin\go-teams.exe captions "https://teams.microsoft.com/meet/synthetic" ".\transcripts\call.md"
.\bin\go-teams.exe -u "https://teams.microsoft.com/meet/synthetic" -o ".\transcripts\call.md"
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
* `--account`: default Microsoft Entra account (UPN/email or display name) to auto-select on the Entra account picker interstitial
* `--headed` / `--headless`: control whether the browser window is visible
* `--audio`: also capture meeting or recap audio to an `.mp3` file alongside the `.md` transcript
* `--recap-audio`: also capture Meeting Recap audio to an `.mp3` file alongside the `.md` transcript
* `--mode`: meeting capture protocol (`auto`, `ephemeral-live`, `live-with-summary`, or `past-recap`)
* `--duration`: nominal scheduled live capture window (for example `30m`, `1h`, `1h30m`) before post-duration inactivity monitoring begins
* `--idle-timeout`: silence/inactivity grace period after `--duration` or in-browser call-end teardown before stopping live capture (default `3m`)
* `--ai-summary-delay`: delay after a live call ends before reopening the meeting in Recap mode to fetch and append `# AI Summary` (default `30m` for `live-with-summary`); a value of `0s` disables deferred post-call queueing and fetching without performing immediate enrichment, but does not suppress an already-available Recap's immediate `# AI Summary`
* `--no-ai-summary`: disable AI Summary capture for both Meeting Recaps and
  post-call enrichment, independently of `--ai-summary-delay`; an explicit
  `--append-ai-summary` request remains a deliberate one-shot summary override
* `--lobby-timeout`: maximum time to wait in pre-join or the meeting lobby before aborting
* `--overwrite`: overwrite existing `.md` transcript and `.mp3` audio files without prompting (default `false` prompts `(Y)es | (N)o`)
* `--append-ai-summary`: existing Markdown transcript file to append the Meeting Recap AI Summary into without overwriting live captions
* `--not-before`: RFC 3339 timestamp before which a queued `--append-ai-summary` entry is skipped and left uncommented

Meeting Recap URLs (including direct SharePoint and OneDrive `stream.aspx`
recording URLs such as
`https://<tenant>.sharepoint.com/personal/<user>/_layouts/15/stream.aspx?id=/personal/<user>/Documents/Recordings/<file>.mp4`)
write only the `.md` transcript and `# AI Summary` by default. On direct
`stream.aspx` pages, `go-teams` activates the SharePoint `Transcript` pane and
triggers the SharePoint Copilot `"Recap the meeting"` summary flow automatically.
Enabling audio with `--audio`, `--recap-audio`, or `capture_audio: true` (alias
`audio: true`) in the config file also writes a companion `.mp3` file next to
the transcript, extracted from the recording that belongs to the requested
Recap. For view-only SharePoint Meeting Recaps and `stream.aspx` recordings
where direct file downloads are blocked (`isDownloadBlocked=true`), `go-teams`
captures the in-page `streamembed.aspx` or `stream.aspx` audio-only stream at
high speed (`14x-100x` real-time) while stripping the video track. Before
initial stream activity, playback setup re-clicks `"Play video"` every
`1000ms` and safely follows replacement `<video>` elements; direct fetches of
non-blob candidates are skipped. Across the 12 Path A workers, per-worker
`inFlightSourceBytes` reservations enforce the shared `256 MiB` source cap.
Path decoding is case-insensitive and single-pass for `fileUrl` and `?id=`
recording identities, while a retryable blocked-stream target error receives at
most one attempt-0 retry. `PerformanceResourceTiming` discovery and
`g_fileInfo.EncKey` apply to entries that are visible from the page window:
`window.performance.getEntriesByType('resource')` does not observe a
`oneDrive.transcode` request initiated solely in a dedicated Web Worker when
its entry remains isolated on the Worker's local performance timeline. Worker
requests require dedicated Worker CDP `Network` / `Target.setAutoAttach`
instrumentation. Stream target selection skips non-stream parent targets, and
delayed-mount discovery does not depend on `xplatplugins.aspx`. Audio is
encoded to MP3 in pure Go without external tools. Recap audio is rejected
rather than captured when its recording identity cannot be verified or when it
exceeds the supported size or duration limits.

`--browser-path`, `--profile-dir`, and `--account` are also accepted by `auth`
and `schedule`, and `schedule` additionally binds `--audio` / `--recap-audio`,
`--overwrite`, `--headless`, `--headed`, `--mode`, `--duration`,
`--idle-timeout`, `--ai-summary-delay`, `--no-ai-summary`, and
`--lobby-timeout`. `browser_path` and `default_account` in the config file
apply to every command.

Parent output directories are created automatically. Both directory mode and
explicit file mode protect existing `.md` and `.mp3` files by default
(`overwrite: false`): when one or more target files already exist on disk,
`go-teams` prompts on standard error (`File already exists: <paths>` followed
by `Overwrite? (Y)es | (N)o: `) and aborts with `capture: file overwrite
declined` without touching any existing file when declined (including empty
input or EOF in non-interactive runs). Pass `--overwrite` or set
`overwrite: true` (alias `file_overwrite: true`) to replace existing files
without prompting. An existing directory passed to `--output` is always treated
as a directory. Positional URL and destination arguments override their
corresponding flags.

## Output target precedence

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

## Filename templates

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
| `<YYYYMMDD>` | Meeting start date (or capture date when unavailable), for example `20261112` |
| `<YYYY-MM-DD>` | Meeting start date (or capture date when unavailable) with dashes |
| `<HHMMSS>` | Meeting start time (or capture time when unavailable) |
| `<timestamp>` | Full date and time stamp |

## Batch queue mode

`--queue <file>` (short form `-q`) captures several meetings in one run. It is
accepted on the root command, `captions`, and `join`:

```powershell
.\bin\go-teams.exe --queue .\transcript-queue.txt --output-folder ".\transcripts" --headless
```

Each queue file line is one capture invocation:

```text
:: Lines that start with :: are comments or completed entries
"https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic-1" --category standup
go-teams.exe captions "https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic-2" -o "C:\out\review.md"
::"https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic-3"
```

* Empty lines and lines that start with `::` (after leading whitespace) are
  skipped. `::` is the only comment marker; a line starting with `#` is parsed
  as arguments. A leading `go-teams` or `go-teams.exe` token (as well as a
  legacy `teams-cli` or `teams-cli.exe` token) is stripped across platforms, so
  you can paste a full command line and existing queue files continue to work
  unchanged.
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
  `--profile-dir`, `--bot-name`, `--category`, `--browser-path`, `--account`,
  `--headless`, `--headed`, `--audio` / `--recap-audio`, `--overwrite`, `--mode`,
  `--duration`, `--idle-timeout`, `--ai-summary-delay`, `--no-ai-summary`, and
  `--lobby-timeout`. The outer `--url`, `--queue`, `--append-ai-summary`, and
  `--not-before` are never forwarded. Flags written on a line come after the
  inherited ones, so repeating the same flag on a line overrides the inherited
  value.
* Deferred AI Summary enrichment lines (`"<url>" --append-ai-summary "<path.md>" --not-before <RFC3339>`)
  whose `--not-before` timestamp is still in the future are skipped without
  error and left uncommented so a later `--queue` or `schedule` run executes
  them once due. When the timestamp has passed, `go-teams` opens the meeting
  in Recap mode, appends `# AI Summary` to `<path.md>`, and prefixes the line
  with `::`.
* [Output target precedence](#output-target-precedence) still applies after the
  outer and per-line flags are merged. An inherited outer `--output-file`
  therefore outranks a per-line `--output` or `--output-dir`; only a per-line
  `--output-file` or a positional output argument (`arg[1]`) overrides it.
  Pass `--output-dir` or `--output-folder`, not `--output-file`, as the outer
  flag with `--queue`.
* Queue logs print every URL as `scheme://host/path` only. Query strings
  (including passcodes), userinfo, and fragments are removed by
  `teams.RedactArgs`.

Pointing every line at one explicit `--output-file` targets the same file on
each capture (prompting `(Y)es | (N)o` unless `--overwrite` or
`overwrite: true` is set), so use `--output-folder` or per-line destinations for
batches.

## Scheduled and protocol-driven capture

`go-teams` classifies every capture into one of four protocols (`--mode` /
`capture_mode` / `meetings[].mode`):

* `ephemeral-live`: real-time live caption and MP3 audio capture (`capture_audio`
  defaults to `true` unless `--audio=false` is passed) with no post-call AI
  Summary wait (`ai_summary_delay: 0s`); it disables AI Summary capture for
  both Recap surfaces and post-call enrichment, and is designed for meetings
  that leave no post-call Recap or recording.
* `live-with-summary`: real-time caption and optional audio capture during the
  live call, immediate flush of `.md` and `.mp3` and browser teardown when the
  call ends (releasing the Chromium `UserDataDir` profile immediately), a
  crash-safe pending entry in `transcription_queue.txt` (`queue_file`), and an
  in-process timer (`--ai-summary-delay`, default `30m`) that reopens the
  meeting in Recap mode, appends `# AI Summary` to the saved Markdown file, and
  marks the queue line `::`. When `--ai-summary-delay=0s` (or
  `ai_summary_delay: "0s"`), `delay <= 0` returns before deferred queueing or
  fetching, so it disables delayed post-call AI Summary work and does not
  perform immediate post-call enrichment.
* `past-recap`: finite post-meeting Recap export (transcript + immediate AI
  Summary + optional MP3) with no live-call wait. `ai_summary_delay: "0s"` does
  not suppress this immediate Recap Summary; use `--no-ai-summary` or
  `Config.NoAISummary` to disable it.
* `auto` (default): selects `past-recap` for Recap views and live capture
  otherwise. Its zero-delay behavior follows the selected surface: live capture
  suppresses deferred queueing and fetching, while an available Recap keeps the
  immediate `# AI Summary` unless `--no-ai-summary` is set.

### Activity-aware nominal duration, overtime, and idle shutdown

`--duration` (or `duration` in `config.yaml`) defines the **nominal baseline
window**, not a blind hard cutoff:

1. If the browser detects in-browser call-end teardown (`#hangup-button` absent
   and the Teams post-call screen visible, or toolbar loss for 3 consecutive
   polls), capture stops as soon as caption and voice activity remain silent for
   `--idle-timeout` (default `3m`).
2. When a live call reaches `--duration` while conversation is still active,
   `go-teams` continues capturing in **overtime** as long as live caption DOM
   mutations (including throttled 1000 ms interim heartbeats) or PCM voice audio
   energy (`PCMRMS >= 300.0`) is observed.
3. Once `--duration` has elapsed and neither caption nor voice activity occurs
   for `--idle-timeout` (default `3m`), `go-teams` shuts down cleanly and
   flushes the `.md` and `.mp3` files.

### Built-in daemon scheduling (`go-teams schedule`)

Configure recurring 5-field `cron:` expressions or one-time `at:` timestamps
(RFC 3339 or local `YYYY-MM-DD HH:MM`) in `config.yaml`:

```yaml
output_dir: C:\Transcripts\WSP
profile_dir: "%LOCALAPPDATA%\\go-teams\\profile"
default_account: synthetic-user@example.com
headless: true
queue_file: C:\Transcripts\WSP\transcription_queue.txt
meetings:
  - name: "Daily Engineering Standup"
    url: "https://teams.microsoft.com/meet/synthetic-standup"
    cron: "0 9 * * 1-5"
    mode: live-with-summary
    duration: 30m
    idle_timeout: 3m
    ai_summary_delay: 30m
    lobby_timeout: 5m
    category: "ENG"
    capture_audio: false
  - name: "Vendor Architecture Review"
    url: "https://teams.microsoft.com/meet/synthetic-vendor"
    at: "2026-11-15 14:00"
    mode: ephemeral-live
    duration: 1h
    idle_timeout: 3m
    category: "ARCH"
    capture_audio: true
```

Run the schedule daemon:

```powershell
.\bin\go-teams.exe schedule --config .\config.yaml --headless
```

Every `meetings:` entry forwards `name` as the fallback meeting title for
`<concat-meeting-name>` and the Markdown header when the live DOM does not
supply a richer title. Past `at:` timestamps are logged and skipped on startup,
and future `at:` timestamps fire once at the target time without repeating.

All scheduled meetings share one browser profile behind an atomic `busy` gate.
An entry that overlaps a running capture is skipped and logged. The gate
releases as soon as the live call ends, before any delayed
`--ai-summary-delay` AI Summary fetch, so the next meeting is not blocked. When
`queue_file` is configured, the daemon sweeps it at startup and runs pending
entries, skipping `--not-before` entries whose timestamp is still in the
future. Global `schedule --no-ai-summary` is inherited by ordinary queued
captures and skips deferred `--append-ai-summary` entries, without preventing
other eligible queue work. On shutdown the daemon allows a 15-second graceful
window. When
`overwrite` is `false` (the default) and a target file already exists, any
interactive `(Y)es | (N)o` overwrite confirmation prompt is bounded to 15
seconds so an unattended schedule releases the `busy` gate (a closed stdin
stream declines immediately). A scheduled entry without a `duration` uses a
nominal 1-hour duration.

#### Schedule configuration and flag precedence

`go-teams schedule` resolves each meeting's settings in three tiers, from
lowest to highest precedence:

1. Top-level settings in `config.yaml` establish the baseline.
2. Global flags passed to `go-teams schedule` (`--mode`, `--duration`,
   `--idle-timeout`, `--ai-summary-delay`, `--no-ai-summary`,
   `--lobby-timeout`, `--audio` / `--recap-audio`, `--overwrite`,
   `--browser-path`, `--profile-dir`, `--account`, `--headless`, `--headed`) are
   applied to that base configuration first (`applyScheduleFlags`). When the
   base mode is `ephemeral-live`, this step also normalizes the base:
   `ai_summary_delay` becomes `0s`, and `capture_audio` defaults to `true`
   unless audio was configured explicitly.
3. When a scheduled meeting fires, its per-meeting settings under `meetings:`
   (`mode` / `capture_mode`, `duration`, `idle_timeout`, `ai_summary_delay`,
   `lobby_timeout`, `category`, `capture_audio` / `audio`, `overwrite` /
   `file_overwrite`, and `output_dir`) are applied on top of the flag-adjusted
   base (`buildScheduleEntryConfig`). The entry is then normalized again: when
   its effective mode is `ephemeral-live`, `ai_summary_delay` becomes `0s`, and
   `capture_audio` defaults to `true` unless audio was explicitly set to `false`
   through `--audio=false` or top-level `capture_audio: false`. Setting
   `capture_audio: true` on the meeting entry overrides that explicit `false`.

As a result, ordinary per-meeting settings in `config.yaml` override the
corresponding global `schedule` flags. The global `schedule --no-ai-summary`
flag is an exception: it remains active on scheduled entries, including
`mode: past-recap` and `ai_summary_delay: 0s`, unless a positive per-meeting
`ai_summary_delay` enables that entry's summary path. The `ephemeral-live`
protocol disables AI Summary capture regardless of delay. The daemon-wide
browser and profile flags (`--browser-path`, `--profile-dir`,
`--account`, `--headless`, and `--headed`) apply to every scheduled meeting.

> [!IMPORTANT]
> Base normalization runs before any entry is built. If the base mode is
> `ephemeral-live`, changing one entry's `mode` (for example to
> `live-with-summary`) does not restore an earlier `ai_summary_delay` or
> `capture_audio` value from before normalization. Set explicit per-meeting
> `ai_summary_delay` and `capture_audio` values on that entry to override the
> normalized base.

### Windows Task Scheduler one-shot recipes (`go-teams join --duration`)

You can also schedule one-shot captures directly with Windows Task Scheduler
without running a persistent daemon:

```powershell
# PowerShell: register a one-time scheduled task for a 45-minute live call
$action = New-ScheduledTaskAction -Execute "C:\Tools\go-teams\go-teams.exe" `
  -Argument 'join "https://teams.microsoft.com/meet/synthetic" --mode ephemeral-live --duration 45m --idle-timeout 3m --lobby-timeout 5m --category ENG --audio --headless'
$trigger = New-ScheduledTaskTrigger -Once -At "2026-11-15 14:00"
Register-ScheduledTask -TaskName "TeamsCapture-ArchReview" -Action $action -Trigger $trigger
```

Or with `schtasks.exe`:

```powershell
schtasks /Create /SC ONCE /TN "TeamsCapture-ArchReview" /ST 14:00 /SD 11/15/2026 `
  /TR "\"C:\Tools\go-teams\go-teams.exe\" join \"https://teams.microsoft.com/meet/synthetic\" --duration 45m --audio --headless"
```

## Browser security posture

* Meeting URLs must use `https` and a Microsoft Teams, SharePoint, or Microsoft
  365 host (`teams.microsoft.com`, `teams.live.com`, `teams.cloud.microsoft`,
  `sharepoint.com`, `microsoft.com`, `office.com`, `sfx.ms`, `aka.ms`, and their
  subdomains). Loopback hosts (`localhost`, `127.0.0.1`, `::1`) may use `http`
  or `https` for local test fixtures. URLs with embedded credentials are
  rejected. The check runs before any browser launches and covers the initial
  URL only; redirects inside the browser are not intercepted.
* Passcodes (`?p=`), userinfo, query strings, fragments, and SharePoint `/:`
  sharing tokens are stripped from navigation errors, queue logs, and the
  `**Source:**` link in Markdown transcript headers (`scheme://host/path` only).
* The Chromium sandbox stays enabled. Set `TEAMS_DISABLE_SANDBOX=1` (or `true`)
  only in containers that cannot run it.

## Headless automation

Sign in using a visible capture session first and reuse that profile:

```powershell
.\bin\go-teams.exe captions --account synthetic-user@example.com --profile-dir ".\bin\profile"
.\bin\go-teams.exe captions --url "https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic" --output-file ".\transcripts\recap.md" --profile-dir ".\bin\profile" --headless
```

The default profile directory comes from `config.DefaultConfig()`. It is
`%LOCALAPPDATA%\go-teams\profile` on Windows when `LOCALAPPDATA` is set, and
otherwise `go-teams/profile` under `os.UserConfigDir()` (for example
`~/.config/go-teams/profile` on Linux). When a canonical per-user profile directory (`go-teams\profile` or
`teams-cli\profile` under `APPDATA`, `os.UserConfigDir()`, or `LOCALAPPDATA`)
has no persisted Teams session on disk (and neither does its same-parent
`teams-cli` sibling), `go-teams` searches those per-user config bases in order
(`APPDATA`, `os.UserConfigDir()`, `LOCALAPPDATA` — preferring
`go-teams\profile` over `teams-cli\profile` within each base) and selects the
first non-repository directory containing a persisted Teams session. The code does not search
for a profile next to the executable or in `bin\profile`; the `.\bin\profile`
paths above are explicit `--profile-dir` values. When `UserDataDir` is empty,
as with a direct library caller that bypasses `DefaultConfig`, the result
depends on the launch path. The chromedp allocator paths pass no user-data directory, so chromedp
uses a temporary profile that does not keep your sign-in. The direct-exec
headed launcher omits `--user-data-dir`, so Chromium uses its own default
profile, which may retain credentials. Use an explicit `--profile-dir` for
scheduled automation. Do not run two browser instances against the same profile.
Headless mode cannot complete interactive login, MFA, or lobby admission for
you. Live capture continues until cancellation; recap capture exits after
exporting available transcript entries and the optional AI summary, separated
by a Markdown horizontal rule. AI summary discovery retries up to 60 times
with 500 ms between attempts; transcript export still succeeds if no summary
is available.

A `--config` flag overrides the config file path; see [Configuration](#configuration)
for the default discovery order.
When no config file exists on disk, `go-teams` uses built-in defaults and opens
a visible browser window automatically.

## Self-update

`go-teams update` replaces the running executable with a published GitHub
Release build from `softwaresalt/go-teams`:

```powershell
.\bin\go-teams.exe --version                  # print the installed version
.\bin\go-teams.exe update --check             # report whether a newer release exists
.\bin\go-teams.exe update                     # install the latest release
.\bin\go-teams.exe update --version v1.2.0    # install a specific release tag
.\bin\go-teams.exe update --version v1.1.0 --force   # reinstall or downgrade
```

* `internal/updater` resolves release assets named
  `go-teams-<goos>-<goarch>` (with an `.exe` suffix on Windows), and
  `.github/workflows/release.yml` builds and publishes CGO-free
  (`CGO_ENABLED=0`) binaries for `windows/amd64`
  (`go-teams-windows-amd64.exe`), `windows/arm64`
  (`go-teams-windows-arm64.exe`), and `darwin/amd64` (`go-teams-darwin-amd64`),
  together with `SHA256SUMS`. The release jobs cross-compile on `ubuntu-latest`
  after the Windows quality gate (`test-windows`) passes, and publication fails
  if `SHA256SUMS` lacks any of those three platform entries. Platforms outside
  those three published targets (`windows/amd64`, `windows/arm64`,
  `darwin/amd64`) return `ErrAssetNotFound`
* `--check` only reports availability and never modifies the executable
* `--version <tag>` installs that release instead of the latest
* `--force` reinstalls the target version even when it is already current, and
  is required to downgrade to an older release

> [!NOTE]
> **Upgrading from an older `teams-cli` binary:** Pre-rename `teams-cli` builds
> look up `teams-cli-<goos>-<goarch>[.exe]` release assets, so running
> `teams-cli update` against a release that publishes `go-teams-<goos>-<goarch>[.exe]`
> fails closed with `ErrAssetNotFound`. Perform a one-time binary download from
> GitHub Releases or run `go install github.com/softwaresalt/go-teams/cmd/go-teams@latest`
> to switch to `go-teams`; subsequent updates work normally via `go-teams update`.

Every update downloads the release's `SHA256SUMS` asset and verifies the
platform binary's SHA-256 digest before touching the installed executable. A
missing `SHA256SUMS`, a missing entry for your platform, or a digest mismatch
aborts the update and leaves the current binary untouched. Downloads must use
`https`, and release metadata and assets are size-capped.

The replacement uses a rename sequence that also works for a running Windows
`.exe`: the current binary is renamed to `go-teams.exe.old`, the verified
download is moved into place, and the `.old` file is restored automatically if
the final move fails. The `.old` file is removed after a successful update, or
on the next update if Windows still holds it open. The executable's directory
must be writable by the current user; a build installed under a privileged
folder such as `C:\Program Files` needs an elevated shell.

Local builds report the version `dev` and treat any release as an upgrade.
Release builds embed their version through the `.github/workflows/release.yml`
link flags.

### Update locking and concurrent updates

Two `go-teams update` processes cannot swap the same executable at once. Each
update takes an inter-process lock next to the executable before it modifies
anything:

* `<exe>.lock` is created with `O_CREATE|O_EXCL` and `0o600` permissions. It
  holds JSON owner metadata: `pid`, a random `token`, and `created_at`.
* A contender polls every 50 ms and gives up after 15 seconds with
  `ErrUpdateLocked`.
* The lock fails closed. A lock older than 2 minutes only changes the
  diagnostic message (an orphaned dead PID versus a live or reused PID);
  `go-teams` never evicts or deletes another process's lock on its own.
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
lock, so it never overwrites a binary it did not launch from. `--force` skips
these identity checks but still takes the lock, because it exists for
deliberate reinstalls and downgrades:

* `ErrConcurrentUpdateModified` means the executable changed since the process
  started: it was deleted, replaced, or no longer matches the launch snapshot,
  the running image, or the `.old` provenance. Rerun `go-teams update`.
* `ErrDowngrade` means the installed binary is already newer than the
  requested release. Pass `--force` to downgrade deliberately.
* `ErrUpdateLocked` means another update is in progress or left a lock
  behind. `ErrConcurrentUpdateModified` wraps it, so both satisfy
  `errors.Is(err, ErrUpdateLocked)`.

The installed binary's own version comes from running `<exe> --version` with a
30-second timeout and a 2-second output cleanup delay, so a stalled binary
cannot hold the lock indefinitely.

### Manual lock and backup recovery

Only recover by hand when no `go-teams update` is running and the lock error
persists. Follow these steps in order:

1. Confirm that every `go-teams update` process has exited. Check the PID
   named in the error message; the operating system can reuse a PID after a
   crash, so a live PID does not by itself prove an update is running.
2. If `<exe>` is missing and `<exe>.old` exists, restore the backup by
   renaming `<exe>.old` back to `<exe>`.
3. Remove `<exe>.lock` and any `<exe>.lock.ctl`.
4. Run `go-teams update` again.

> [!WARNING]
> Never delete `<exe>.lock` while an update might still be running. Removing
> the lock early lets a second process swap the executable concurrently.

## Configuration

The CLI reads a YAML configuration file. Pass `--config <path>` (`-f`) to use a
specific file. Without it, `go-teams` uses the first file that exists:

1. `./config.yaml` in the working directory
2. `config.yaml` next to the executable
3. the per-user config directory: `%APPDATA%\go-teams\config.yaml` on
   Windows, `~/.config/go-teams/config.yaml` on Linux

When none exists, built-in defaults apply. Keys you omit keep their built-in
defaults. Run [`go-teams generate-config`](#generating-a-configuration-file)
to create a fully commented starting file; the same template ships as
[`config.example.yaml`](../config.example.yaml).

> [!NOTE]
> **Migrating an existing `teams-cli` configuration and browser profile:** When
> upgrading from `teams-cli`, rename or move `%APPDATA%\teams-cli` to
> `%APPDATA%\go-teams` and `%LOCALAPPDATA%\teams-cli\profile` to
> `%LOCALAPPDATA%\go-teams\profile` (or `~/.config/teams-cli` to
> `~/.config/go-teams` on Linux). Update any explicit `profile_dir`,
> `output_dir`, or `queue_file` paths in `config.yaml`, any `--config` or
> `--profile-dir` flags in scheduled tasks or scripts, and any pending
> `--append-ai-summary` lines in `transcription_queue.txt` that reference the
> old `teams-cli` directory names. Verify that `go-teams` authenticates and any
> pending queue entries complete before deleting the legacy directories.
> Alternatively, run `go-teams auth` to create a fresh profile under `go-teams`
> and securely delete the orphaned `teams-cli\profile` folder once pending work
> has finished so cached Microsoft 365 session tokens are not left behind on
> disk.

```yaml
output_dir: /home/user/.config/go-teams/transcripts
# output_file: /home/user/transcripts/standup.md   # fixed file path; takes precedence over output_dir
filename_template: "<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md"
category: standup
profile_dir: /home/user/.config/go-teams/profile
browser_path: /usr/bin/google-chrome
default_account: synthetic-user@example.com
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
| `default_account` | `entra_account`, `account` | Microsoft Entra account (UPN/email or display name) to auto-select on the account picker interstitial (`default_account` > `entra_account` > `account`) |
| `bot_name` | | Display name for guest joins |
| `headless` | `headed` | Browser visibility; `headed: true` forces a visible window |
| `capture_audio` | `audio` | Also write an `.mp3` next to the transcript |
| `overwrite` | `file_overwrite` | Replace existing `.md` and `.mp3` files without prompting (default `false` prompts `(Y)es \| (N)o`; `overwrite` wins over `file_overwrite`) |
| `capture_mode` | `mode` | Capture protocol: `auto`, `ephemeral-live`, `live-with-summary`, or `past-recap` |
| `duration` | | Nominal live capture window (for example `30m`, `1h`) before post-duration idle monitoring |
| `idle_timeout` | | Post-duration or post-teardown silence window before stopping capture (default `3m`) |
| `ai_summary_delay` | | Post-call delay before reopening Recap to append `# AI Summary` (default `30m` for `live-with-summary`); `0s` / `delay <= 0` disables delayed queueing and fetching on live calls without immediate enrichment, but preserves immediate Recap export unless `--no-ai-summary` disables it |
| `lobby_timeout` | | Maximum pre-join / lobby wait window before aborting |
| `queue_file` | | Path to `transcription_queue.txt` used to persist deferred `--append-ai-summary` entries |
| `meetings` | | Scheduled meetings (`name`, `url`, `cron` or `at`, plus optional `mode`/`capture_mode`, `duration`, `idle_timeout`, `ai_summary_delay`, `lobby_timeout`, `category`, `capture_audio`/`audio`, `overwrite`/`file_overwrite`, `output_dir`) |

`output_folder` and `default_path` only ever set the output directory. The
separate `output` key accepts either a file or a directory. When `config.Load`
reads it, a value ending in `.md` or `.markdown` unconditionally sets
`output_file`, overriding any `output_file` in the same config file. Any
other value sets `output_dir`. Because `output` is read after the other
aliases, it also overrides `output_folder` and `default_path`. Validation
requires `output_dir`, `profile_dir`, and `bot_name` to be non-empty, and every
scheduled meeting needs a `url` and either a `cron` expression or an `at`
timestamp, but not both.

Omitting both `headless` and `headed` opens a visible browser window, so
interactive sign-in works out of the box. Set `headless: true` for unattended
runs. For the single-capture commands (the root command, `join`, `captions`,
and `auth`), command-line flags override every top-level config value (see
[Output target precedence](#output-target-precedence)). `go-teams schedule` is
the exception: global `schedule` flags override top-level `config.yaml`
defaults first, and each `meetings:` entry's per-meeting settings (`mode` /
`capture_mode`, `duration`, `idle_timeout`, `ai_summary_delay`,
`lobby_timeout`, `category`, `capture_audio` / `audio`, `overwrite` /
`file_overwrite`, and `output_dir`) then override those flags for that
meeting. See
[Built-in daemon scheduling (go-teams schedule)](#built-in-daemon-scheduling-go-teams-schedule).

## Project layout

```text
cmd/go-teams/         CLI entrypoint
internal/audio/       PCM-to-MP3 encoder for meeting and recap audio
internal/browser/     chromedp allocator, stealth flags, live meeting and recap runner
internal/capture/     Capture session, output paths, filename templates, and file writes
internal/cli/         Cobra command wiring, generate-config, and batch queue runner
internal/config/      Configuration model, YAML persistence, and commented template generation
internal/scheduler/   Cron schedule construction
internal/teams/       Teams meeting URL validation, mutation, and redaction
internal/transcript/  Caption deduplication and Markdown writer
internal/updater/     Release lookup, SHA-256 verification, locking, and executable swap
internal/version/     Build metadata and semantic version comparison
```

See [`ARCHITECTURE.md`](ARCHITECTURE.md) for the dependency direction
between these packages and [`docs/QUALITY_SCORE.md`](QUALITY_SCORE.md) for
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
launch a browser in unattended CI.

Integration and end-to-end test helpers automatically load a gitignored
`.env.local.test` file at the repository root (when present) for any test
variable not already set in the process environment. Copy the tracked
[`.env.example`](../.env.example) template to get started:

```bash
cp .env.example .env.local.test
```

Process environment variables always take precedence over `.env.local.test`
across both primary and alias names, and unedited template placeholders from
`.env.example` (`synthetic-placeholder` values and `synthetic-user@example.com`)
are ignored. Never commit `.env.local.test` or real credentials.

```bash
# Compile the harness (CI runs this to prevent bit-rot):
go vet -tags=integration ./...

# Browser launch smoke test (requires a Chromium-based browser):
GO_TEAMS_INTEGRATION=1 go test -tags=integration ./internal/browser/ -run TestChromeRunnerLaunch

# Live meeting contract tests (currently red — pending live-capture work):
GO_TEAMS_INTEGRATION=1 TEAMS_MEETING_URL="https://teams.microsoft.com/l/meetup-join/..." \
  go test -tags=integration ./internal/browser/
```

| Env var | Alias | Purpose |
|---|---|---|
| `GO_TEAMS_INTEGRATION` | | Set to `1` (or any value other than `""`, `"0"`, or `"false"`) to enable integration tests |
| `TEAMS_ACCOUNT` | `TEAMS_TEST_ACCOUNT` | Microsoft Entra account (UPN/email or display name) passed as `DefaultAccount` in browser integration tests |
| `TEAMS_MEETING_URL` | `TEAMS_TEST_MEETING_URL` | Joinable Teams meeting URL for the live meeting tests |
| `TEAMS_RECAP_URL` | `TEAMS_TEST_RECAP_URL` | Teams Meeting Recap URL consumed by `TestMeetingLiveRecapAISummaryEnrichment` (`internal/browser`) and `TestLiveTenantDelayedAISummaryQueueReplay` (`internal/cli`) |
| `TEAMS_PROFILE_DIR` | `TEAMS_TEST_PROFILE_DIR` | Authenticated Chromium profile directory for live integration tests; expands `%VAR%`, `$VAR`, and `~` in a single pass, resolves relative paths from the repository root, and falls back from `go-teams` to an existing `teams-cli` profile with a persisted Teams session |
| `TEAMS_BROWSER_PATH` | `TEAMS_TEST_BROWSER_PATH` | Chromium/Edge/Chrome executable path or bare binary name; expands `%VAR%`, `$VAR`, and `~` before resolving relative paths from the repository root |
| `GO_TEAMS_BINARY` | | Compiled `go-teams` executable path for `TestCompiledBinaryCapturesRecapToExplicitOutput`; expands `%VAR%`, `$VAR`, and `~`, resolves relative paths from the repository root, and falls back from `teams-cli.exe` to `go-teams.exe` when present in the same directory |
| `TEAMS_DISABLE_SANDBOX` | | Set to `1` or `true` in container environments without user namespaces where Chromium requires `--no-sandbox` |

The local synthetic-page end-to-end harness verifies browser caption delivery.
The opt-in compiled-executable harness also verifies recap and AI summary
output without connecting to a real tenant:

```powershell
go build -o .\bin\go-teams.exe .\cmd\go-teams
$env:GO_TEAMS_BINARY = (Resolve-Path .\bin\go-teams.exe).Path
go test ./internal/cli -run TestCompiledBinaryCapturesRecapToExplicitOutput -count=1
```

Synthetic checks do not qualify real Teams login, tenant policy, audio fidelity,
or recap completeness. Those require an authorized operator-assisted live run.

#### Live-tenant scheduled call and Recap AI Summary validation

Shipment `029-S` adds dedicated `-tags=integration` harnesses in
`internal/browser/scheduled_lifecycle_integration_test.go` and
`internal/cli/scheduled_integration_test.go` to validate scheduled meeting join,
overtime continuation past nominal duration, in-browser call-end silence
shutdown, and Idea 2C delayed AI Summary enrichment against an authorized
Microsoft Teams tenant.

**Prerequisite setup (`.env.local.test`, gitignored):**

* `GO_TEAMS_INTEGRATION="1"`
* `TEAMS_MEETING_URL`: joinable meeting link for an active scheduled test call (or a `/l/meetingrecap` URL for finite Recap transcript/summary verification)
* `TEAMS_RECAP_URL`: Meeting Recap URL carrying a `threadId` or `meetingrecap`
  route (required when `TEAMS_MEETING_URL` uses `/meet/<id>?p=<passcode>`)
* `TEAMS_PROFILE_DIR`: authenticated browser profile directory (for example
  `"%APPDATA%\go-teams\profile"` or gitignored `./bin/teams-profile`; profiles
  inside the repository root must be git-ignored, while external profiles
  outside the workspace root are permitted when they do not contain the
  repository root; falls back to `teams-cli` when `go-teams` has no persisted
  Teams session yet)
* `TEAMS_BROWSER_PATH`: path or binary name for Edge/Chrome/Chromium
* `TEAMS_ACCOUNT`: Microsoft Entra UPN or display name used for account selection

**Run commands:**

```bash
# Browser-level scheduled overtime and call-end teardown:
go test -tags=integration -v ./internal/browser/... -run TestMeetingLiveScheduledOvertimeAndCallEnd

# Browser-level ForceRecap + AISummaryOnly extraction:
go test -tags=integration -v ./internal/browser/... -run TestMeetingLiveRecapAISummaryEnrichment

# CLI-level scheduled join (--mode live-with-summary), overtime, call-end shutdown, and queue write:
go test -tags=integration -v ./internal/cli/... -run TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown

# CLI-level delayed AI Summary queue replay (--queue):
go test -tags=integration -v ./internal/cli/... -run TestLiveTenantDelayedAISummaryQueueReplay
```

**Operator human checkpoints:**

1. **Organizer start and lobby admission**: start the Teams meeting from an
   organizer client and admit the `Transcription Agent` session from the lobby
   within the `2m` lobby timeout.
2. **Overtime speech/caption stimulus and call-end transition**: speak or
   generate live captions past the `15s` nominal duration into the `15s`
   overtime window, then hang up from the organizer client (or remain silent for
   `--idle-timeout`) so `OnCallEnded` / `Session.Run` self-terminates cleanly.
3. **Post-call Recap AI Summary window**: Microsoft Teams typically publishes the
   Meeting Recap AI Summary `5-30m` after the call ends; either wait for Recap
   publication or point `TEAMS_RECAP_URL` at an already-published Meeting Recap
   for immediate queue replay verification.

**URL shape rules for Idea 2C Recap enrichment:**

* Classic `/l/meetup-join/19%3ameeting_<redacted-id>@thread.v2/0?...` URLs embed
  `threadId`, so `deriveRecapURLFromMeetingURL` derives the
  `/l/meetingrecap?threadId=...` route automatically during delayed enrichment.
* Modern `/meet/<redacted-id>?p=<redacted-passcode>` URLs do not embed
  `threadId` in the URL query string, so Recap enrichment tests require
  `TEAMS_RECAP_URL` to be set alongside `TEAMS_MEETING_URL`.
