---
title: Past-meeting recap reliability, deterministic Entra auth bootstrap, Edge CDP lifecycle, and transcript passcode redaction implementation plan
description: Implementation plan for 026-F / 026-S covering stash entries 52519E8D, 0544DBA7, 030CD519, 6B0E5F68, 1B1877DB, 54894478, and 2925E813
source: docs/decisions/2026-11-14-recap-auth-and-redaction-hardening-deliberation.md
deliberation_id: 015-DL
feature: 026-F
shipment: 026-S
source_stash:
  - 52519E8D
  - 0544DBA7
  - 030CD519
  - 6B0E5F68
  - 1B1877DB
  - 54894478
  - 2925E813
date: 2026-11-14
status: approved
tags:
  - browser
  - recap
  - auth
  - capture
  - transcript
  - security
  - cli
---

## Problem Frame

Following the accepted deliberation in [2026-11-14-recap-auth-and-redaction-hardening-deliberation.md](../decisions/2026-11-14-recap-auth-and-redaction-hardening-deliberation.md) (`015-DL`), seven active stash entries (`52519E8D`, `0544DBA7`, `030CD519`, `6B0E5F68`, `1B1877DB`, `54894478`, `2925E813`) are planned under covering feature **`026-F`** and shipment **`026-S`** (sequenced after `025-S`).

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Add `AISummaryOnly bool`, `DisableAISummary bool`, and `ErrAccountRequired` to `internal/browser/meeting.go`; in `joinHeadedWithDeadline` and `tryExtractRecapViaCDP` (`internal/browser/meeting_chrome.go`), skip `.ms-List` transcript early-return when `m.opts.AISummaryOnly` is `true` and only take the AI-summary-without-transcript short-circuit when `m.opts.AISummaryOnly` is `true`, so `--mode past-recap` (`ForceRecap: true, AISummaryOnly: false`) waits for `.ms-List` transcript entries (`len(entries) > 0`), and skip `captureAISummaryFromTargets` when `m.opts.DisableAISummary && !m.opts.AISummaryOnly` | `52519E8D`, `030CD519` | U1 (`026.001-T`) | `go test -race ./internal/browser/...` |
| R2: In `internal/browser/recap.go`, raise default Recap audio limits to support 1-hour+ meetings without exceeding V8's ~512 MiB string limit (`maxRecapAudioDurationSec = 4200`, `maxRecapAudioSourceBytes = 256 * 1024 * 1024`, `maxRecapAudioPCMBytes = 360 * 1024 * 1024`), bound normal DOM `evalInCDPTarget` message reads to 16 MiB while keeping the audio message limit for `captureRecapAudioFromTargets`, and update `extractRecapAudioJS` to use `new AudioContext({sampleRate: 44100})`, check resampled `outputLength * 2 > maxRecapAudioPCMBytes` (bounding `buffer.length` scaled by sampleRate), deduplicate and prioritize non-`blob:` media URLs ahead of `blob:` URLs, reset `identityMatched` to `baseIdentityMatched` per candidate, use `redirect: 'follow'` with post-redirect `response.url` identity verification and `mediaUrl: response.url \|\| candidateUrl`, and retry with `credentials: 'omit'` on network `TypeError` | `0544DBA7` | U1 (`026.001-T`) | `go test -race ./internal/browser/...` |
| R3: Enforce non-empty `DefaultAccount` (`ErrAccountRequired`) in `ChromeRunner.Authenticate` (`internal/browser/runner.go`) and on cold profiles (`!hasTeamsSessionInProfile`) in `Join` / `joinHeadedWithDeadline` (`internal/browser/meeting_chrome.go`); in `joinHeadedWithDeadline`, do not abort on `<-procDone` while `listCDPTargets(port)` succeeds; and in `launchCleanHeadedBrowser` (`internal/browser/recap.go`), declare `var wsURL string` before `cleanup`, send bounded `Browser.close` over `wsURL`, and on Windows terminate any surviving child browser process still serving CDP on `127.0.0.1:<port>` after verifying CDP liveness and browser image name | `1B1877DB` | U2 (`026.002-T`) | `go test -race ./internal/browser/...` |
| R4: In `internal/capture/session.go`, add `DisableAISummary bool` to `SessionConfig`; track `captionsErr` and `audioErr` separately under `mu`; write `# AI Summary` whenever `captionsErr == nil && !skipSyncSummary && !s.cfg.DisableAISummary` even if `audioErr != nil` (preserving `errors.Join`); and in the deferred cleanup strictly after `out.Close()`, remove `out.AudioPath` when `audioErr != nil` and file size is `0` bytes | `0544DBA7`, `030CD519` | U3 (`026.003-T`) | `go test -race ./internal/capture/...` |
| R5: In `setupOutput` (`internal/capture/output.go`), compute `headerStart` (`cfg.MeetingStart` when non-zero, else `now`) before calling `FormatTranscriptFilename(cfg.FilenameTemplate, cfg, headerStart, id)` and update `OutputConfig` GoDoc; redact fallback URLs in `sanitizeMeetingID` via `teams.RedactURL` so query secrets never enter `MeetingID` or filenames; redact `SourceURL` via `teams.RedactURL` in `internal/capture/output.go`; and in `internal/transcript/writer.go` (`sanitizeSourceURL`), enforce a stdlib-only `net/url` `scheme://host/path` allowlist (preserving leaf-package isolation) that strips userinfo, query parameters, fragments, and SharePoint `/:` sharing paths | `6B0E5F68`, `54894478` | U3 (`026.003-T`) | `go test -race ./internal/capture/... ./internal/transcript/...` |
| R6: In `internal/cli/root.go`, `internal/cli/capture.go`, and `internal/cli/auth.go`, wire `AISummaryOnly` and `DisableAISummary` (`isAISummaryDisabled(cfg)`), require `--account` or `default_account` in `newAuthCommand`, correct the `redactHelpArg` / `redactedHelpArg` doc comments (`2925E813`), and update `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` | `52519E8D`, `030CD519`, `1B1877DB`, `2925E813` | U4 (`026.004-T`) | `go test -race ./internal/cli/...` |

## Implementation Units

### U1 (`026.001-T`) — Separate `AISummaryOnly` from `ForceRecap`, honor `DisableAISummary` in browser Recap extraction, and harden Recap audio extraction for SharePoint redirects/MSE and 1-hour+ recordings

* **Domain:** code (`internal/browser`). **Posture:** test-first. **Size:** `M` | **Complexity:** `medium`.
* **Files:** `internal/browser/meeting.go`, `internal/browser/meeting_chrome.go`, `internal/browser/recap.go`, `internal/browser/recap_retry_test.go`.
* **What:**
  1. **Red phase (`internal/browser/recap_retry_test.go`)**: Write failing tests first for:
     * `ForceRecap: true, AISummaryOnly: false` waiting for `.ms-List` transcript entries even when the outer page already exposes an AI Summary, whereas `AISummaryOnly: true` skips `.ms-List` early return and returns once the AI Summary is copied.
     * `DisableAISummary: true` skipping `captureAISummaryFromTargets` during Recap transcript extraction.
     * `extractRecapAudioJS` preferring non-`blob:` candidate media URLs over `blob:` URLs, resetting `identityMatched` per candidate, following redirects (`redirect: 'follow'`) with post-redirect `response.url` identity verification, retrying with `credentials: 'omit'` when `credentials: 'include'` throws a `TypeError`, and accepting a 1-hour (`3600s`) 48 kHz recording under the updated defaults.
  2. In `internal/browser/meeting.go`:
     * Add `AISummaryOnly bool` and `DisableAISummary bool` to `MeetingOptions`, and export sentinel error `ErrAccountRequired = errors.New("browser: initial authentication requires --account or default_account in config.yaml")`.
  3. In `internal/browser/meeting_chrome.go`:
     * Treat `m.opts.ForceRecap || m.opts.AISummaryOnly` as implying Recap mode in `lobbyDeadline()`, `Join()`, `joinHeadedWithDeadline()`, and `awaitPreJoinOrAdmissionWithDeadline()`.
     * In `joinHeadedWithDeadline` and `tryExtractRecapViaCDP`, guard the `.ms-List` transcript extraction loop with `if !m.opts.AISummaryOnly` and call `captureAISummaryFromTargets(ctx, port, m.capturePageID)` inside it only when `!m.opts.DisableAISummary`.
     * Change the transcript-less AI-summary short-circuit to `if m.opts.AISummaryOnly && isOnTargetMeetingOrRecapURL(...)` so `--mode past-recap` (`ForceRecap: true, AISummaryOnly: false`) waits for `.ms-List` transcript entries (`len(entries) > 0`) before returning from `Join()`, while `runAppendAISummary` (`ForceRecap: true, AISummaryOnly: true`) polls until `m.aiSummary != ""` or deadline expiry.
  4. In `internal/browser/recap.go`:
     * Update default resource caps to `maxRecapAudioSourceBytes = 256 * 1024 * 1024` (256 MiB), `maxRecapAudioDurationSec = 4200` (70 minutes, covering 1-hour meetings + overtime), and `maxRecapAudioPCMBytes = 360 * 1024 * 1024` (360 MiB, whose ~480 MiB base64 representation stays safely below V8's ~512 MiB `String::kMaxLength`).
     * In `extractRecapAudioJS`, snapshot `const baseIdentityMatched = identityMatched;` after page-level checks, collect deduplicated candidate media URLs ordering non-`blob:` HTTP(S) URLs ahead of `blob:` URLs, reset `identityMatched = baseIdentityMatched` at the start of each candidate iteration, run `checkIdentity(candidateUrl)` and `if (enforceIdentity && !identityMatched) throw new Error('recap audio recording identity unverified')` outside the network-retry `try/catch`, fetch with `{credentials: 'include', redirect: 'follow'}` (falling back to `{credentials: 'omit', redirect: 'follow'}` on `TypeError`), verify `checkIdentity(response.url)` when `response.url` is non-empty, use `new AudioContext({sampleRate: 44100})`, check `outputLength * 2 > maxRecapAudioPCMBytes` (bounding `buffer.length * 2` scaled by `Math.max(1, buffer.sampleRate / 44100)`), and return `mediaUrl: response.url || candidateUrl`.

### U2 (`026.002-T`) — Enforce cold-profile Entra account requirement and harden Windows Edge child-process CDP lifecycle and cleanup

* **Domain:** code (`internal/browser`). **Posture:** test-first. **Size:** `M` | **Complexity:** `medium`.
* **Depends on:** `026.001-T` (`blocks`).
* **Files:** `internal/browser/runner.go`, `internal/browser/meeting_chrome.go`, `internal/browser/recap.go`, `internal/browser/runner_test.go`, `internal/browser/recap_retry_test.go`.
* **What:**
  1. **Red phase (`internal/browser/runner_test.go`, `internal/browser/recap_retry_test.go`)**: Write failing tests first for:
     * `ChromeRunner.Authenticate` returning `ErrAccountRequired` when `DefaultAccount` is empty.
     * `chromeMeeting.Join` / `joinHeadedWithDeadline` returning `ErrAccountRequired` on a cold profile (`!hasTeamsSessionInProfile`) when `DefaultAccount` is empty (updating `TestJoinHeadedPersistsLaunchedCDPPortForRecap` to supply `DefaultAccount: "presenter@example.com"`).
     * `joinHeadedWithDeadline` surviving an immediate parent stub process exit (`procDone` closed) while `listCDPTargets(port)` remains alive and extracting the Recap transcript, plus `parseNetstatListeningPID` unit tests.
  2. In `internal/browser/runner.go`:
     * In `ChromeRunner.Authenticate`, check `if strings.TrimSpace(r.options.DefaultAccount) == ""` and return `ErrAccountRequired` before launching the browser.
  3. In `internal/browser/meeting_chrome.go`:
     * In `joinHeadedWithDeadline`, after computing `coldProfile := !hasTeamsSessionInProfile(m.headedOpts.UserDataDir)`, if `coldProfile && m.defaultAccount() == ""` return `ErrAccountRequired` before calling `launchCleanHeadedBrowser` (and in `Join()`, if `m.headedOpts == nil && m.probePreJoinState == nil && m.userDataDir != "" && !hasTeamsSessionInProfile(m.userDataDir) && m.defaultAccount() == ""`, return `ErrAccountRequired`).
     * In `joinHeadedWithDeadline`, track `procExited := false` and `procWait := procDone`. In the top-of-loop `listCDPTargets(port)` error branch, check `procDone` via non-blocking `select` to update `procExited` and return `fmt.Errorf("browser: browser process exited before meeting admission: %w", err)` when `procExited` is `true`. In the bottom `select`, when `case <-procWait:` fires, set `procExited = true` and `procWait = nil`, then probe `if _, probeErr := listCDPTargets(port); probeErr != nil { return fmt.Errorf("browser: browser process exited before meeting admission") }`; if `listCDPTargets(port)` succeeds, log a debug/info message and continue polling.
  4. In `internal/browser/recap.go`:
     * Declare `var wsURL string` prior to `cleanup` in `launchCleanHeadedBrowser` so `cleanup` captures the populated WebSocket URL.
     * In `cleanup`, send bounded `Browser.close` over `wsURL` (when non-empty), terminate `cmd.Process`, and on Windows, if `wsURL != ""` and `isCDPPortStillAlive(port)` (verifying `http://127.0.0.1:<port>/json/version` still responds with a CDP `webSocketDebuggerUrl`), locate the listening PID via `parseNetstatListeningPID`, verify `isChromiumProcessImage(pid, execPath)`, and terminate it with `taskkill /T /F /PID <pid>` using bounded `exec.CommandContext`.

### U3 (`026.003-T`) — Preserve AI summary and remove 0-byte MP3 on audio failure, use `MeetingStart` in transcript filenames, and redact `SourceURL` in transcript headers

* **Domain:** code (`internal/capture`, `internal/transcript`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **Files:** `internal/capture/session.go`, `internal/capture/output.go`, `internal/transcript/writer.go`, `internal/capture/session_test.go`, `internal/capture/output_test.go`, `internal/transcript/writer_test.go`.
* **What:**
  1. **Red phase (`internal/capture/session_test.go`, `internal/capture/output_test.go`, `internal/transcript/writer_test.go`)**: Write failing tests first for:
     * `Session.Run` on a Recap where `CaptureAudio` returns an error: `# AI Summary` is still written to the `.md` transcript, the 0-byte `.mp3` file is removed on cleanup, and `Run` still returns the audio error.
     * `Session.Run` with `DisableAISummary: true`: `# AI Summary` is omitted from the `.md` transcript even when `m.AISummary()` is non-empty.
     * `setupOutput` with non-zero `MeetingStart` (e.g., `2026-11-13` captured on `2026-11-14`): `<YYYYMMDD>` and `<YYYY-MM-DD>` in the generated filename expand to `20261113` / `2026-11-13`.
     * `setupOutput`, `sanitizeMeetingID`, and `Writer.WriteHeader` stripping query parameters (`?p=SecretPasscode`, `driveId=...`, `sitePath=...`, `threadId=...`), userinfo, fragments, and SharePoint `/:` sharing paths from `**Source:**` and fallback `MeetingID` (updating `TestWriterHeader` in `internal/transcript/writer_test.go`).
  2. In `internal/capture/session.go`:
     * Add `DisableAISummary bool` to `SessionConfig`.
     * Declare `mu`, `firstErr`, `captionsErr`, and `audioErr` before the deferred `out.Close()` cleanup. In the deferred cleanup, after `out.Close()` finishes, read `audioErr` under `mu.Lock()`; if `audioErr != nil && out.AudioPath != ""`, check `os.Stat(out.AudioPath)` and remove the file (`_ = os.Remove(out.AudioPath) // Best-effort removal of empty 0-byte audio artifact after failed audio capture`) when `info.Size() == 0`.
     * When `isRecap` is `true`, do not cancel `runCtx` when `CaptureAudio` fails so `EnableCaptions` completes undisturbed. After `wg.Wait()`, write `# AI Summary` whenever `captionsErr == nil && !skipSyncSummary && !s.cfg.DisableAISummary`, joining any summary write/flush error with `result` via `errors.Join`.
  3. In `internal/capture/output.go`:
     * Update the GoDoc comments on `OutputConfig.Now` and `OutputConfig.MeetingStart`.
     * Compute `headerStart := now; if !cfg.MeetingStart.IsZero() { headerStart = cfg.MeetingStart }` before calling `FormatTranscriptFilename(cfg.FilenameTemplate, cfg, headerStart, id)`.
     * In `sanitizeMeetingID(rawURL)`, after checking `meetingIDToken` and `meetPathToken`, redact `rawURL` via `teams.RedactURL(rawURL)` (returning `"meeting"` if `RedactURL` returns `"[invalid-url]"`) before slugifying the fallback URL so query secrets never enter `MeetingID` or filenames.
     * Redact `SourceURL` via `redactSourceURL(cfg.MeetingURL)` (using `teams.RedactURL` and rejecting `"[invalid-url]"` or SharePoint `/:` sharing paths) before passing `transcript.Metadata` to `w.WriteHeader`.
  4. In `internal/transcript/writer.go`:
     * Update `sanitizeSourceURL(raw string) string` using only standard library `net/url` (preserving leaf-package isolation): require a valid `http` or `https` URL with a non-empty host, empty `Opaque`, no `@` in host, and no SharePoint `/:` sharing path segment; clear `User`, `RawQuery`, `ForceQuery`, `Fragment`, and `RawFragment`; and escape spaces and parentheses.

### U4 (`026.004-T`) — Wire `AISummaryOnly`, `DisableAISummary`, and `auth` `--account` validation in `internal/cli` and update `redactHelpArg` doc comment and docs

* **Domain:** code (`internal/cli`, `docs/`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `026.001-T`, `026.002-T`, `026.003-T` (`blocks`).
* **Files:** `internal/cli/root.go`, `internal/cli/capture.go`, `internal/cli/auth.go`, `internal/cli/root_test.go`, `internal/cli/capture_test.go`, `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`.
* **What:**
  1. **Red phase (`internal/cli/root_test.go`, `internal/cli/capture_test.go`)**: Write failing tests first for:
     * `teams-cli auth` returning `browser.ErrAccountRequired` when neither `--account` nor `default_account` is set (and update `validConfig()` in `internal/cli/root_test.go` to include `DefaultAccount: "user@example.com"` while testing `--account=""` rejection in `TestDefaultAccountFlagAndQueueForwarding`).
     * `defaultNewMeeting` and `defaultDeps().newSession` wiring `AISummaryOnly` and `DisableAISummary` for `--mode past-recap`, `--no-ai-summary`, `ai_summary_delay: 0s`, and `--append-ai-summary`.
  2. In `internal/cli/capture.go` and `internal/cli/root.go`:
     * Add `isAISummaryDisabled(c config.Config) bool` returning `true` when `strings.TrimSpace(c.AppendAISummary) == ""` and either `CaptureMode` is `ephemeral-live` or `c.AISummaryDelay` parses to a zero duration (`0`, `0s`, `0m`).
     * Wire `AISummaryOnly: strings.TrimSpace(c.AppendAISummary) != ""` and `DisableAISummary: isAISummaryDisabled(c)` into `browser.MeetingOptions` and `capture.SessionConfig`; in `runAppendAISummary`, set `recapCfg.AppendAISummary = transcriptPath` and `recapCfg.AISummaryDelay = ""`.
  3. In `internal/cli/auth.go`:
     * In `newAuthCommand`'s `RunE`, after `applyBrowserFlags(cmd, &cfg, flags)`, if `strings.TrimSpace(cfg.DefaultAccount) == ""` return `fmt.Errorf("cli: %w", browser.ErrAccountRequired)` before invoking `d.newRunner(cfg).Authenticate(cmd.Context())`, and update `Long` / `Example` help text.
  4. In `internal/cli/root.go`, `docs/ARCHITECTURE.md`, and `docs/USER_GUIDE.md`:
     * Update the `redactedHelpArg` and `redactHelpArg` doc comments (`2925E813`) to state that dispatch to `teams.RedactArgs` is by `http://` or `https://` prefix only (not URL validation), and update `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` to reflect required `--account` / `default_account` on initial auth and `**Source:**` URL redaction.

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Compliant — pure Go 1.26, no `unsafe`, explicit error wrapping with `fmt.Errorf("...: %w", err)` and sentinel `browser.ErrAccountRequired`; every discarded cleanup error carries an explanatory comment |
| II. Test-First Development | Compliant — each task specifies its Red-phase failing unit tests in `*_test.go` before production code |
| III. Workspace Isolation | Compliant — all paths remain inside `t.TempDir()` or configured output dirs |
| IV. CLI Workspace Containment | Compliant — no writes outside workspace root |
| V. Structured Observability | Compliant — `slog` logging with redacted URLs and account identifiers |
| VI. Single Responsibility | Compliant — zero new third-party dependencies; leaf-package isolation preserved (`internal/transcript` uses stdlib `net/url` only) |
| VII. Destructive Command Approval | Compliant — no destructive workspace operations; Windows child-browser cleanup verifies CDP port liveness and browser image name before `taskkill` |
| VIII. Safety Modes | Compliant — `Freeze-scope mode` active; strict-safety `ProposedAction` table below |
| IX. Git-Friendly Persistence | Compliant — Markdown transcript headers and backlog artifacts remain deterministic, human-readable, and LF-normalized |
| X. Agent Context Efficiency | Compliant — targeted queries and bounded file reads used throughout |
| XI. Merge Commit History Preservation | Compliant — PRs merge via `--merge` only (P-009) |

## Plan Hardening

`Requires plan hardening: yes`

### Safety Mode Posture

* **Mode:** `Freeze-scope mode` + `Strict-safety` (`internal/browser`, `internal/capture`, `internal/transcript`, `internal/cli`).
* **Strict-Safety `ProposedAction` Inventory:**

| Action ID | Summary | Targets | Change Kind | ActionRisk | Approval Required | ActionResult | Rollback |
|---|---|---|---|---|---|---|---|
| `PA-026-1` | Require `--account` / `default_account` on initial `auth` and cold-profile headed bootstrap, and keep headed browser loop alive across Windows `msedge.exe` stub exit while terminating the child listener on `<port>` on cleanup | `internal/cli/auth.go`, `internal/browser/runner.go`, `internal/browser/meeting_chrome.go`, `internal/browser/recap.go` | Auth & browser process lifecycle | `high` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U2/U4 commits |
| `PA-026-2` | Separate `AISummaryOnly` from `ForceRecap`, honor `DisableAISummary`, and harden Recap audio extraction + 0-byte `.mp3` cleanup | `internal/browser/meeting_chrome.go`, `internal/browser/recap.go`, `internal/capture/session.go`, `internal/cli/root.go`, `internal/cli/capture.go` | Runtime capture & Recap extraction | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U1/U3/U4 commits |
| `PA-026-3` | Strip all query parameters, fragments, and userinfo from Markdown `**Source:**` header links via `teams.RedactURL` and use `MeetingStart` for transcript filename dates | `internal/capture/output.go`, `internal/transcript/writer.go` | Security redaction & output naming | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U3 commit |

### Failure Modes and Guardrails

1. **Windows Edge child-process termination safety (`PA-026-1`)**:
   * Risk: Killing the wrong process on Windows when cleaning up `launchCleanHeadedBrowser`.
   * Guardrail: Only match TCP rows in `LISTENING` state whose local address is exactly `127.0.0.1:<port>` (the ephemeral port allocated by `launchCleanHeadedBrowser`), never `0.0.0.0` or any other port, and reject PID `<= 4`. Send CDP `Browser.close` over `wsURL` first so the browser exits gracefully before falling back to `taskkill` on the listening PID.
2. **Recap audio failure during `Session.Run` (`PA-026-2`)**:
   * Risk: A `CaptureAudio` failure in `Session.Run` cancels `runCtx` before `EnableCaptions` finishes writing Recap transcript entries, or skips `# AI Summary` and leaves a 0-byte `.mp3` file on disk.
   * Guardrail: Track `captionsErr` and `audioErr` separately; for Recap sessions (`isRecap == true`), do not cancel `EnableCaptions` when `CaptureAudio` fails; always write `# AI Summary` when `captionsErr == nil && !s.cfg.DisableAISummary`; and remove `out.AudioPath` after `out.Close()` only when `audioErr != nil` and the file size is `0` bytes.
3. **Post-redirect identity verification in `extractRecapAudioJS` (`PA-026-2`)**:
   * Risk: Allowing `redirect: 'follow'` in `fetch()` could follow a redirect to a mismatched recording URL without checking identity.
   * Guardrail: Immediately after `await fetch(...)` resolves, if `response.url` is non-empty, call `checkIdentity(response.url)` before reading `response.body`.

## Verification Gates

1. `go vet ./...`
2. `golangci-lint run`
3. `gofmt -l .` (verified LF-clean)
4. `go test -race ./...`
