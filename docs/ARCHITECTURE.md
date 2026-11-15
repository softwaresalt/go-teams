---
title: Architecture and Domain Map
description: Domain map, enforced dependency direction, runtime flows, and security invariants for go-teams (teams-cli)
date: 2026-11-14
---

## System overview

`go-teams` builds the `teams-cli` executable: a single static Go 1.26 binary
that drives a host Chromium-based browser (Chrome, Edge, or Brave) over the
Chrome DevTools Protocol using `chromedp` and `gobwas/ws`. It joins Microsoft
Teams meetings, records live captions, exports finite Meeting Recap
transcripts with the AI Summary, optionally captures meeting or recap audio as
MP3, and writes everything to local Markdown files. It needs no external
drivers and no FFmpeg.

The executable has no server component and persists nothing except the files
it writes: transcripts, optional MP3 files, the browser profile directory, and
the optional YAML configuration file. For an approachable setup walkthrough and
the complete command, flag, and configuration reference, see the
[user guide](USER_GUIDE.md); the [README](../README.md) is the cover page. For
the full blueprint, see
[Go-TeamsTranscriptCLI-Architecture.md](product-specs/Go-TeamsTranscriptCLI-Architecture.md).
For graded quality per domain, see [QUALITY_SCORE.md](QUALITY_SCORE.md).

## Domain map

The module contains one entrypoint and ten packages under `internal/`.

| Package | Responsibility | Key exported surface |
|---|---|---|
| `cmd/teams-cli` | Process entrypoint; wires signal handling (`os/signal`) to the root command | `main` |
| `internal/cli` | Cobra commands (`captions`, `join`, `auth`, `schedule`, `generate-config`, `update`, root), capture flag binding, output target resolution, batch queue runner, root help handler (`installRootHelpHandler`: placeholder/unknown-argument `Note:` banner with credential-safe redaction) | `NewRootCommand` |
| `internal/config` | YAML configuration model, defaults, key aliases, validation, atomic save, commented template generation (`template.go`) | `Config`, `Load`, `Save`, `DefaultConfig`, `TemplateYAML`, `ResolveTemplatePath`, `WriteTemplate` |
| `internal/teams` | Meeting URL allowlist validation, join URL mutation, URL redaction | `ValidateMeetingURL`, `MutateJoinURL`, `RedactURL`, `RedactArgs` |
| `internal/browser` | `chromedp` allocator and stealth flags, pre-join state machine, live caption observer, Meeting Recap export, AI Summary extraction, audio capture | `Runner`, `Meeting`, `RecapMeeting`, `Options`, `NewLiveMeeting` |
| `internal/capture` | Capture session that connects a meeting to a transcript writer; output path resolution, filename templates, owner-only file creation | `Session`, `SessionConfig`, `Output`, `FormatTranscriptFilename` |
| `internal/transcript` | Caption event model, caption deduplication, Markdown rendering with text sanitization | `CaptionEvent`, `Deduplicator`, `Writer`, `Metadata` |
| `internal/audio` | PCM to MP3 encoding (`shine-mp3`) | `Encoder`, `NewEncoder` |
| `internal/scheduler` | Cron schedule construction for the `schedule` daemon | `Entry`, `JobFunc`, `Build` |
| `internal/updater` | Release lookup, checksum verification, executable swap, inter-process locking | `Updater`, `Config`, `Result`, sentinel errors |
| `internal/version` | Build metadata and SemVer 2.0.0 normalization and comparison | `Version`, `Normalize`, `Compare`, `IsNewer` |

## Dependency direction

Imports flow downward only. A package may import packages in the layers below
it and must not import sideways within its own layer or upward.

```text
cmd/teams-cli
    |
    v
internal/cli
    |-- internal/capture --+-- internal/browser --+-- internal/audio
    |                      |                      |-- internal/teams
    |                      |                      '-- internal/transcript
    |                      '-- internal/transcript
    |-- internal/browser   (direct use for auth, runner, and meeting setup)
    |-- internal/updater ----- internal/version
    |-- internal/config
    |-- internal/scheduler
    |-- internal/teams
    '-- internal/version
```

Read as layers:

| Layer | Packages | May import |
|---|---|---|
| Entrypoint | `cmd/teams-cli` | `internal/cli` |
| Command wiring | `internal/cli` | Every package below it |
| Adapters | `internal/capture`, `internal/browser`, `internal/updater` | Leaf packages; `capture` also imports `browser` |
| Leaves | `internal/audio`, `internal/config`, `internal/scheduler`, `internal/teams`, `internal/transcript`, `internal/version` | Standard library and third-party modules only |

Rules the layout enforces:

* Leaf packages never import another `internal/` package. `config` knows
  nothing about the browser, and `transcript` knows nothing about Teams.
* `internal/cli` is the only package that imports `config`, `scheduler`, and
  `updater`, and the only one that reads build metadata from `version` for the
  CLI itself (`updater` imports `version` for comparisons). It translates the
  loaded configuration and flags into the plain option structs that adapters
  consume (`browser.Options`, `capture.SessionConfig`, `updater.Config`), so
  adapters never read the config file.
* `internal/capture` depends on `browser` and `transcript`; `browser` never
  imports `capture`. Capture-specific behavior such as file naming and
  permissions stays out of the browser layer.
* `internal/updater` is independent of the capture path. Its only internal
  dependency is `internal/version`.
* Tests substitute behavior through the `deps` struct in `internal/cli`
  (`loadConfig`, `newRunner`, `newMeeting`, `newSession`, `runUpdate`), not by
  importing production adapters into leaf packages.

## Runtime flows

### Interactive auth

`teams-cli auth` opens a visible browser with a persistent profile directory
(`--profile-dir` or the configured `profile_dir`). Initial authentication (and
cold-profile bootstrap when the profile does not yet contain a Teams session)
requires `default_account` in `config.yaml` or `--account` on the command line
(`browser.ErrAccountRequired` when missing); `ChromeRunner` polls for the
Microsoft Entra "Pick an account" interstitial (`#tilesHolder`) and clicks the
single unambiguously matching account tile automatically. You complete any
password or MFA prompts manually and the session cookies stay in that profile.
Headless runs later reuse the profile; they can auto-select a cached account
tile when `default_account` / `--account` is configured, but cannot complete
password entry, MFA, or lobby admission themselves.

### Live join and captions

1. `internal/cli` loads the config, applies flag overrides (`--mode`,
   `--duration`, `--idle-timeout`, `--ai-summary-delay`, `--no-ai-summary`,
   `--lobby-timeout`), validates the URL with `teams.ValidateMeetingURL`, and
   mutates it with `teams.MutateJoinURL`.
2. `internal/browser` allocates Chromium and walks the pre-join state machine
   within any configured `LobbyTimeout` (`ErrLobbyTimeout` on expiration):
   when `DefaultAccount` is configured and an Entra account picker appears, it
   clicks the matching account tile (`selectEntraAccountJS`), then dismisses
   the launcher interstitial, mutes the microphone, injects the bot name for
   guests, and waits through lobby admission.
3. A `MutationObserver` in the page reports caption rows (emitting both
   throttled 1000 ms interim activity heartbeats and settled/punctuated
   utterances). A `payloadPump` forwards them to the Go side, where
   `transcript.Deduplicator` collapses rolling updates into settled caption
   events and notifies `capture.Session` of caption activity. Meanwhile, a
   background DOM poller checks `callEndedProbeJS()` every 2 seconds for
   in-browser call-end teardown (`#hangup-button` loss or post-call banner).
4. `capture.Session` writes each event through `transcript.Writer` to the
   resolved Markdown file (using `SessionConfig.MeetingTitle` as the fallback
   title when the DOM supplies none). When `Duration` is set, `capture.Session`
   treats it as the nominal baseline window, extending into overtime while
   caption mutations or PCM voice energy (`PCMRMS >= 300.0`) continue and
   stopping cleanly once neither signal occurs for `IdleTimeout` (default `3m`).
5. With `--audio` (or by default in `ephemeral-live` mode), the page's
   `getDisplayMedia` stream delivers PCM that `browser.HasVoiceActivity`
   measures for speech activity and `audio.Encoder` writes to an `.mp3` next to
   the transcript.
6. For `live-with-summary` captures, `internal/cli` closes Chromium immediately
   when the live call ends (releasing the `UserDataDir` profile), appends a
   crash-safe `"<url>" --append-ai-summary "<transcript.md>" --not-before <RFC3339>`
   entry to `queue_file` / `transcription_queue.txt`, and runs an
   `ai_summary_delay` timer (default `30m`) that reopens the meeting in Recap
   mode (`ForceRecap: true`), appends `# AI Summary` via
   `capture.AppendAISummaryToFile`, and marks the queue line `::`.

### Finite Meeting Recap export

For a Meeting Recap URL the browser opens the recap page and exports a finite
transcript instead of listening live:

1. Transcript entries come from the `.ms-List` virtual list through React fiber
   or DOM extraction, restricted to the tab and frame that belong to the
   requested recap.
2. The AI Summary is read by intercepting the clipboard write of the Copy
   control, retried up to 20 times with 500 ms between attempts. Transcript
   export still succeeds when no summary appears.
3. With audio enabled, the recording is decoded through Web Audio over CDP and
   encoded to MP3, but only after the recording identity is verified against
   the requested recap and size and duration limits pass.
4. `capture.Session` writes the transcript, a Markdown horizontal rule, and the
   summary, then the process exits.

### Batch queue

`teams-cli --queue <file>` (`-q`) runs `runQueueFile` in `internal/cli`:

1. The queue file is read once; blank lines and `::` lines are skipped, and an
   optional leading `teams-cli` token is stripped. Lines carrying a future
   `--not-before <RFC3339>` timestamp are skipped without error and left
   uncommented until due.
2. Each due line becomes its own root command invocation built with
   `executeQueueLine`, with `inheritedQueueFlags` prepended. Those flags
   forward the outer capture flags (config path, output settings, template,
   profile, bot name, category, browser path, default account, headless,
   audio, mode, duration, idle timeout, AI summary delay, lobby timeout) but
   never the outer `--url`, `--queue`, `--append-ai-summary`, or `--not-before`.
   When a due line carries `--append-ai-summary <transcript.md>`, `runCapture`
   dispatches to `runAppendAISummary` (`ForceRecap: true`) and appends
   `# AI Summary` in place via `capture.AppendAISummaryToFile`.
3. After a line succeeds it is commented out by prepending `::` under
   `queueFileMu` after re-reading the file on disk so concurrent deferred
   appends are never clobbered. `writeQueueLines` writes the new content to a
   temporary file in the queue directory and renames it over the queue file;
   if creating the temporary file or the rename fails, it falls back to an
   in-place `os.WriteFile` with `0o600` permissions.
4. Failures on one line are logged and collected, and later lines still run.
   Queue logs print URLs through `teams.RedactArgs`.

### Cron schedule

`teams-cli schedule` builds a `robfig/cron` instance from the `meetings`
entries through `scheduler.Build` / `scheduler.BuildWithClock`, supporting both
recurring 5-field `cron` expressions and one-shot `at` timestamps (RFC 3339 or
local `YYYY-MM-DD HH:MM`, scheduled via a single-fire `cron.Schedule` adapter
and skipped on startup when already in the past). Each job runs with a nominal
duration (`meetings[].duration` -> `config.duration` -> default `1h`) and post-duration
`idle_timeout` (default `3m`), forwarding `e.Name` as the fallback
`MeetingTitle`.

All configured meetings share one Chromium `UserDataDir` profile, so
`scheduleJobWithSummaryWG` in `internal/cli/schedule.go` serializes active
browser sessions with an `atomic.Bool` busy gate. As soon as a live call and
its Chromium process finish, `onLiveCallFinished` releases the `busy` gate and
detaches any post-call `ai_summary_delay` timer onto a daemon `sync.WaitGroup`
so neither `busy` nor `cron.SkipIfStillRunning` blocks subsequent scheduled
meetings during the 30-minute AI summary wait. On startup, `schedule` also runs
a non-blocking sweep for due `--append-ai-summary` entries in `queue_file` /
`transcription_queue.txt`.

### Self-update

`teams-cli update` calls `updater.Update`:

1. Resolve the release from GitHub, require `https` (or loopback for tests),
   and cap metadata, checksum, and binary sizes.
2. Download `SHA256SUMS` and the platform binary, and verify the digest before
   touching the installed file.
3. Acquire the inter-process lock `<exe>.lock` (and, for a `.old` target, the
   ordered chain of overlapping locks), then verify launch and installed
   identity under the lock.
4. Rename the current binary to `<exe>.old`, move the verified binary into
   place, and restore `.old` automatically if the final move fails.
5. Release the lock through the token-verified `<exe>.lock.ctl` protocol.

The protocol details are in the
[self-update design doc](design-docs/2026-11-11-self-update-and-interprocess-locking.md).

### Configuration template generation

`teams-cli generate-config [output-dir]` (aliases `init-config`, `gen-config`)
never loads a configuration file and ignores the root `--config` flag:

1. `internal/cli` folds the optional positional argument into a directory and
   filename. An explicit `--output-dir` wins; a positional `.yaml` or `.yml`
   path that is not an existing directory names the file itself.
2. `config.ResolveTemplatePath` expands the directory like other configuration
   paths and validates the filename, rejecting anything that is not a plain
   name inside that directory with `ErrInvalidConfigName`.
3. `config.WriteTemplate` writes `config.TemplateYAML()`, a fully commented file
   that is valid as generated. Without `--force` it opens the target with
   `O_EXCL` and returns `ErrConfigExists` for an existing file. With `--force`
   it replaces the file atomically through a sibling temporary file.
4. The resolved path is printed to stdout. `config.example.yaml` is kept
   identical to `TemplateYAML()` by a test.

## Security and isolation invariants

* **Meeting URL allowlist.** `teams.ValidateMeetingURL` runs before any browser
  launches. It requires `https` and a Microsoft Teams, SharePoint, or Microsoft
  365 host, allows `http` or `https` only for loopback fixtures, and rejects
  embedded credentials. It checks the initial URL only; in-browser redirects
  are not intercepted.
* **Redaction.** `teams.RedactURL` and `teams.RedactArgs` reduce URLs to
  `scheme://host/path` in navigation errors, logs, queue output, and the
  `**Source:**` link in Markdown transcript headers (with `transcript.Writer`
  additionally enforcing a standard-library `net/url` `scheme://host/path`
  allowlist that strips userinfo, query parameters, fragments, and SharePoint
  `/:` sharing tokens) so passcodes, userinfo, and fragments never reach logs
  or saved transcripts. When Microsoft Entra account auto-selection
  (`default_account` / `--account`) logs a tile selection,
  `redactAccountIdentifier` masks the configured UPN or display name
  (`a***@contoso.com` or `A***`) so raw account identifiers never appear in
  logs, and `selectEntraAccountJS` serializes the target string with
  `json.Marshal` to prevent DOM/script injection.
* **Participant text.** `transcript.Writer` defuses Markdown and bare-URL
  autolink syntax in captions and metadata fields so remote participants cannot
  inject live links or structure into the transcript file.
* **Output permissions.** `internal/capture` creates new directories with
  `0o700` and new transcript and MP3 files with `0o600`. Pre-existing
  directories and explicitly overwritten files keep their existing filesystem
  permissions; the code does not tighten them. On POSIX platforms,
  `internal/capture/output_unix_test.go` verifies these modes under
  `syscall.Umask(0)`, including preservation of a pre-existing directory's mode.
  `config.WriteTemplate` (`generate-config`) also creates missing parent
  directories with `0o700` and generated configuration files with `0o600`,
  using `O_EXCL` unless `--force` is set, and rejects filenames that could
  escape the target directory or collide with Windows reserved device names
  with `ErrInvalidConfigName`. The `0o700` and `0o600` modes are enforced on
  POSIX platforms; on Windows the files inherit the access control list of the
  parent folder.
* **Recap identity isolation.** Recap export is bound to the requested recap's
  tab, frame, and recording identity. Audio whose identity cannot be verified
  is rejected rather than captured.
* **Browser sandbox.** The Chromium sandbox stays on by default, including in
  direct-exec launches. `--no-sandbox` is added only when
  `Options.DisableSandbox` is set or `TEAMS_DISABLE_SANDBOX` is `1` or `true`.
* **Updater fail-closed behavior.** Updates require a matching `SHA256SUMS`
  entry, bounded downloads, and the `<exe>.lock` protocol. A lock is never
  evicted automatically, and the updater refuses to run when the executable
  identity changed or a downgrade is attempted without `--force`.
* **Tracked-file hygiene.** `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`
  and the CI secret-hygiene step scan the git-tracked regular text files
  reported by `git ls-files`, so no live passcode or token is committed. Both
  skip binary files (a NUL byte in the first 8000 bytes) and files larger than
  `8 MiB`, and neither follows symlinks. They flag tracked local `.env`
  filenames (`.env`, `.env.*`, `*.env`, except `.env.example`), Teams
  passcodes (`?p=` or `&p=` values of eight or more characters on Teams meeting
  URLs, excluding obvious synthetic placeholders), `SKYPE-TOKEN` assigned a
  JWT value, and `Bearer eyJ...` JWTs. Documentation examples use synthetic
  values only.

## Keeping this document current

Update this file in the same pull request as any change that adds or removes an
`internal/` package, changes an import direction, or alters a runtime flow or
security invariant. Graduated design decisions live in
[`design-docs/`](design-docs/).
