---
title: Repository documentation refresh and knowledge graduation implementation plan
description: Implementation plan for 020-F / 020-S updating README.md, adding docs/ARCHITECTURE.md and docs/QUALITY_SCORE.md, and graduating 013-S..018-S design decisions into docs/design-docs/
source: docs/decisions/2026-11-11-repo-documentation-refresh-deliberation.md
deliberation_id: 009-DL
feature: 020-F
shipment: 020-S
date: 2026-11-11
status: approved
tags:
  - docs
  - architecture
  - quality-score
  - design-docs
  - readme
---

## Problem Frame

Following the accepted deliberation in
[2026-11-11-repo-documentation-refresh-deliberation.md](../decisions/2026-11-11-repo-documentation-refresh-deliberation.md)
(`009-DL`), stash entry **`6227A297`** is planned under feature **`020-F`** and
shipment **`020-S`** (sequenced after `019-S` via
`backlogit dep add 020-S 019-S --type blocks`):

1. **User-facing CLI & configuration reference (`README.md`)**: Update `README.md`
   so every shipped CLI flag, batch queue capability (`--queue` / `-q`),
   `018-S` inter-process self-update lock/recovery protocol, YAML configuration
   field, and `internal/` package directory is accurately documented.
2. **Progressive-disclosure maps (`docs/ARCHITECTURE.md` and `docs/QUALITY_SCORE.md`)**:
   Create both files signposted in `AGENTS.md` and
   `.github/instructions/architecture-doc.instructions.md` to document the 10
   `internal/` packages, enforced dependency direction, runtime flows,
   security/isolation invariants, honest quality grades per domain, test coverage
   tiers, and explicit live-verification gaps (`010-S`).
3. **Knowledge graduation (`docs/design-docs/`)**: Graduate the `016-S` (`005-DL`)
   and `018-S` (`007-DL`) self-update and inter-process locking design decisions
   into `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`,
   and update `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`
   to cover the `013-S` (`002-DL`) URL allowlist/redaction and `017-S` (`006-DL`)
   direct-exec browser sandbox controls.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Update `README.md` with batch queue mode (`--queue` / `-q`), complete capture flag & filename template reference, `018-S` self-update `<exe>.lock` / `<exe>.lock.ctl` & manual recovery reference, complete YAML config fields, and all 10 `internal/` packages | `6227A297` | U1 (`020.001-T`) | `git add` + `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |
| R2: Create `docs/ARCHITECTURE.md` and `docs/QUALITY_SCORE.md` fulfilling the progressive-disclosure signposts in `AGENTS.md` and `architecture-doc.instructions.md` | `6227A297` | U2 (`020.002-T`) | Markdown structure check + `git add` + `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |
| R3: Create `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md` (`016-S` + `018-S`) and update `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md` (`013-S` + `017-S`) | `6227A297` | U3 (`020.003-T`) | Markdown structure check + `git add` + `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |

## Implementation Units

### U1 (`020.001-T`) — Refresh `README.md` CLI, batch queue, self-update locking, configuration, and project layout reference

* **Domain:** docs (Markdown). **Posture:** documentation update. **Size:** `S` | **Complexity:** `low`.
* **What:**
  Update `README.md` to accurately cover all shipped capabilities:
  1. **Batch queue mode**: Document `--queue` / `-q <file>` in `## Commands` and
     a dedicated subsection: one CLI capture invocation per non-empty, non-`::`
     line (optional leading `teams-cli` / `teams-cli.exe` stripped), single and
     double quote handling with Windows backslash preservation, rejection of
     nested `--queue` / `-q` flags, per-line isolated execution that comments
     out each completed line in-place by prepending `::` (attempting an atomic
     temp-file rename via `writeQueueLines` in `internal/cli/queue.go:200` and
     falling back to an in-place `os.WriteFile` if the rename fails),
     inheritance of outer capture flags except `--url` and `--queue`
     (`inheritedQueueFlags`), and URL query/userinfo/fragment redaction
     (`teams.RedactArgs`) in queue logs and error messages.
  2. **Complete capture flag and output naming reference**: On root, `captions`,
     and `join`, add `--queue` (`-q`), `--category` (`-c`),
     `--filename-template` (and alias `--name-template`), and `--output-folder`
     alongside existing flags; document complete output target precedence
     (positional output argument `arg[1]` > `--output-file` > `--output` >
     `--output-dir` > `--output-folder` > config `output_file` > config
     `output_folder` > config `output_dir`) and the default transcript filename
     pattern `<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md` (plus
     supported placeholders `<category>`, `<concat-meeting-name>`,
     `<meeting-name>`, `<meeting-id>`, `<YYYYMMDD>`, `<YYYY-MM-DD>`, `<HHMMSS>`,
     `<timestamp>`).
  3. **Complete `### Self-update` (`teams-cli update`)**: Document the `018-S`
     inter-process `<exe>.lock` protocol (`O_CREATE|O_EXCL`, `0o600`, JSON owner
     metadata `pid`, `token`, `created_at`, 15s wait timeout / 50ms poll,
     2-minute stale diagnostic classification without automatic lock eviction,
     and ordered shortest-prefix-first lock chain `acquireSwapLockChain` when a
     target path ends in `.old`), `<exe>.lock.ctl` non-cancelable
     (`context.WithoutCancel`, 2s wait / 10ms poll) token-verified release,
     pre-swap `statExecutable` / `targetPath` symlink wait across the transient
     `.old` rename window, launch and installed-version identity guards
     (`ErrConcurrentUpdateModified`, `ErrDowngrade`, `ErrUpdateLocked`), and
     safe manual operator lock/backup recovery preconditions (confirm all
     `teams-cli update` processes have exited; if `<exe>` is missing and
     `<exe>.old` exists, restore `<exe>.old` to `<exe>`; then remove
     `<exe>.lock` and any `<exe>.lock.ctl`).
  4. **Complete `## Configuration`**: Expand the YAML example and field
     reference to cover `output_dir`, `output_folder` (and `default_path`) as
     the directory-only alias for `output_dir`, the YAML `output`
     file-or-directory target (`internal/config/config.go:192-198`, where in
     `config.Load` a `.md`/`.markdown` value in `output` unconditionally sets
     `OutputFile` — overriding `output_file` if both are present in
     `config.yaml` — and any non-`.md`/`.markdown` value in `output` sets
     `OutputDir`), `output_file`, `filename_template` (and `name_template`
     alias), `meeting_url` (and `url` alias), `category`, `profile_dir`,
     `browser_path`, `bot_name`, `headless` (and `headed` alias),
     `capture_audio` (and `audio` alias), and `meetings`, plus default config
     discovery order (`./config.yaml`, `<exe_dir>/config.yaml`, user config
     directory `%APPDATA%\teams-cli\config.yaml` / `~/.config/teams-cli/config.yaml`).
  5. **Complete `## Project layout`**: Include all 10 `internal/` packages
     (`internal/audio/`, `internal/browser/`, `internal/capture/`,
     `internal/cli/`, `internal/config/`, `internal/scheduler/`,
     `internal/teams/`, `internal/transcript/`, `internal/updater/`,
     `internal/version/`) alongside `cmd/teams-cli/`.
* **Files (1 file):** `README.md`.
* **Verify:** Stage `README.md` with `git add` (required because the secret-scan
  test inspects `git ls-files`) and run
  `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`;
  confirm Markdown conventions are satisfied.
* **Depends on:** none.

### U2 (`020.002-T`) — Add `docs/ARCHITECTURE.md` and `docs/QUALITY_SCORE.md` progressive-disclosure maps

* **Domain:** docs (Markdown). **Posture:** documentation creation. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. Create `docs/ARCHITECTURE.md` with YAML frontmatter (`title: Architecture and Domain Map`)
     fulfilling the signpost in `AGENTS.md` and
     `.github/instructions/architecture-doc.instructions.md`:
     * System overview of `go-teams` (`teams-cli`) as a single static Go 1.26
       CLI binary driving Chromium over CDP (`chromedp` + `gobwas/ws`).
     * Explicit domain map across `cmd/teams-cli` and all 10 `internal/`
       packages (`cli`, `config`, `teams`, `capture`, `browser`, `transcript`,
       `audio`, `scheduler`, `updater`, `version`) with package responsibilities
       and key exported interfaces/types.
     * Enforced dependency direction diagram and layering rules (leaf packages
       `version`, `teams`, `audio`, `transcript`, `config`, `scheduler`; middle
       adapters `browser`, `capture`, `updater`; top-level command wiring `cli`
       and entrypoint `cmd/teams-cli`).
     * Key runtime flows: (a) interactive `auth`, (b) live meeting `join` /
       `captions` (pre-join FSM, `MutationObserver` + `payloadPump` +
       `Deduplicator`, `getDisplayMedia` PCM -> `audio.Encoder`), (c) finite
       Meeting Recap export (`.ms-List` React fiber / DOM extraction, AI Summary
       clipboard interception with 20-attempt retry, identity-verified Web Audio
       decode -> MP3), (d) batch `--queue` execution with `::` in-place
       completion marking (attempting an atomic temp-file rename via
       `writeQueueLines` in `internal/cli/queue.go:200` and falling back to an
       in-place `os.WriteFile` if the rename fails) and outer capture flag
       inheritance except `--url` and `--queue` (`inheritedQueueFlags`), (e)
       cron `schedule` daemon, and (f) self-update (`update` with `SHA256SUMS`
       verification, `<exe>.lock` / `<exe>.lock.ctl` coordination, and `.old`
       rollback).
     * Security and isolation invariants: meeting URL host allowlist and
       credential rejection (`teams.ValidateMeetingURL`), URL redaction in
       errors/logs (`teams.RedactURL`, `teams.RedactArgs`), participant text
       Markdown/autolink sanitization (`transcript.Writer`), owner-only output
       permissions on newly created artifacts (`internal/capture` creates *new*
       directories with `0o700` and *new* transcript/MP3 files with `0o600`,
       while pre-existing directories and explicitly overwritten files retain
       their existing filesystem permissions), Recap tab/frame and recording
       identity isolation, Chromium sandbox default-on posture, and fail-closed
       updater verification/locking.
  2. Create `docs/QUALITY_SCORE.md` with YAML frontmatter (`title: Quality Grades by Domain`)
     fulfilling the signpost in `AGENTS.md` and
     `.github/instructions/architecture-doc.instructions.md`:
     * Grading rubric (`A`, `B`, `C`) aligned with
       `architecture-doc.instructions.md`.
     * Per-domain/package quality table across `internal/version`,
       `internal/teams`, `internal/config`, `internal/transcript`,
       `internal/audio`, `internal/scheduler`, `internal/updater`,
       `internal/capture`, `internal/cli`, and `internal/browser` (`A` for pure
       and hermetically verified packages; `B` for `internal/browser` due to
       pending live Microsoft Teams tenant qualification in `010-S`).
     * Test coverage tiers: (1) unit & concurrency `-race` suite, (2) synthetic
       browser & compiled-binary E2E harness (`TestChromeMeetingEndToEnd` in
       `internal/browser/meeting_e2e_test.go:25`,
       `TestCompiledBinaryCapturesRecapToExplicitOutput` in
       `internal/cli/binary_automation_test.go:16`), (3) deterministic golden MP3
       regression (`TestEncoderDeterministicGoldenReference` in
       `internal/audio/mp3_test.go`, which verifies inline golden SHA-256
       digests and MP3 frame headers), (4) tracked-file secret scan
       (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` in
       `internal/config`, inspecting `git ls-files`), and (5)
       `integration`-tagged live browser/meeting harness.
     * Explicit live-verification gaps and improvement roadmap referencing
       queued shipment `010-S`.
* **Files (2 files):** `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`.
* **Verify:** Stage `docs/ARCHITECTURE.md` and `docs/QUALITY_SCORE.md` with
  `git add` (required because the secret-scan test inspects `git ls-files`) and
  run
  `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`;
  confirm Markdown conventions are satisfied.
* **Depends on:** none.

### U3 (`020.003-T`) — Graduate `016-S` & `018-S` self-update locking design doc and update `011-S` browser capture design doc for `013-S` & `017-S`

* **Domain:** docs (Markdown). **Posture:** knowledge graduation. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. Create `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`
     with YAML frontmatter (`title`, `date: 2026-11-11`, `status: implemented`,
     `feature: 016-F, 018-F`, `shipment: 016-S, 018-S`, `deliberation: 005-DL, 007-DL`)
     graduating the durable architectural decisions from `016-S` and `018-S`:
     * Build metadata and SemVer 2.0.0 normalization/precedence in
       `internal/version` (`dev` upgrade posture, `+build` metadata handling).
     * GitHub Releases discovery, HTTPS + loopback-only transport and redirect
       re-validation, signed-URL query-token redaction, bounded downloads
       (`4 MiB` metadata, `1 MiB` checksums, `256 MiB` binary), and mandatory
       `SHA256SUMS` verification before rename.
     * Windows-safe `.old` rename sequence and automatic rollback on swap
       failure.
     * Inter-process lock protocol: `<exe>.lock` (`O_CREATE|O_EXCL`, `0o600`,
       JSON `lockOwner` with `pid`, 128-bit `token`, `created_at`, 15s wait /
       50ms poll, 2m stale diagnostic threshold, fail-closed no-auto-eviction
       policy), `<exe>.lock.ctl` non-cancelable (`context.WithoutCancel`, 2s
       wait / 10ms poll) token-verified release, ordered shortest-prefix-first
       lock chain (`acquireSwapLockChain`) for `.old`-suffixed targets, and
       reserved marker suffix rejection (`errReservedMarkerName`).
     * Pre-swap `statExecutable` and `targetPath` symlink resolution across the
       transient `.old` rename window.
     * Launch-snapshot, running-image (`/proc/self/exe` / `os.Executable`),
       `.old` `os.Args[0]` provenance, and installed-binary `--version` probe
       (30s timeout, 2s `WaitDelay`) identity guards returning
       `ErrConcurrentUpdateModified`, `ErrDowngrade`, or `ErrUpdateLocked`.
     * Operator recovery preconditions and diagnostic contract.
  2. Update `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`
     to incorporate the graduated decisions from `013-S` (`002-DL`) and `017-S`
     (`006-DL`):
     * Frontmatter metadata updated to list features `011-F, 013-F, 015-F, 017-F`
       and shipments `011-S, 013-S, 015-S, 017-S`.
     * Meeting URL validation and redaction (`013-S`): `teams.ValidateMeetingURL`
       enforced before browser allocation (`https` required on allowlisted
       Microsoft Teams / SharePoint / M365 host suffixes, `http`/`https` allowed
       on loopback `localhost`/`127.0.0.1`/`::1` for fixtures, embedded userinfo
       rejected; initial-URL ingress boundary), and `teams.RedactURL` /
       `teams.RedactArgs` stripping userinfo, query strings (`?p=` passcodes),
       and fragments from navigation errors and batch queue logs.
     * Browser sandbox posture (`017-S`): both `AllocatorOptions` /
       `StealthFlags` and direct-exec `cleanBrowserArgsForOptions` keep the
       Chromium sandbox enabled by default and emit `--no-sandbox` and
       `--disable-setuid-sandbox` (before the trailing `initialURL` in direct
       exec) only when `Options.DisableSandbox` is true or
       `TEAMS_DISABLE_SANDBOX` is `1` or `true`.
* **Files (2 files):**
  `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`,
  `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`.
* **Verify:** Stage both design docs with `git add` (required because the
  secret-scan test inspects `git ls-files`) and run
  `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`;
  confirm Markdown conventions are satisfied.
* **Depends on:** none.

## Dependency Graph

```text
020-F (Repository Documentation Update and Knowledge Graduation)
  ├── U1 (020.001-T: README.md reference update)          [independent]
  ├── U2 (020.002-T: docs/ARCHITECTURE.md + QUALITY_SCORE.md) [independent]
  └── U3 (020.003-T: docs/design-docs/ graduation)        [independent]
```

* No intra-feature `blocks` edges are added between `020.001-T`, `020.002-T`,
  and `020.003-T` because their file sets are completely disjoint.
* Shipment `020-S` is chained after `019-S` via
  `backlogit dep add 020-S 019-S --type blocks`.

## Decisions and Rationale

* **Disjoint width-isolated `docs` tasks**: Splitting by artifact purpose
  (`README.md`, `ARCHITECTURE.md` + `QUALITY_SCORE.md`, and `docs/design-docs/`)
  keeps every task at `<= 2` files (`Size: S`, `Complexity: low`) and allows
  independent verification.
* **Honest quality grading**: `docs/QUALITY_SCORE.md` grades `internal/browser`
  as `B` (and all pure/hermetically tested packages as `A`) to reflect that live
  Microsoft Teams tenant DOM verification remains queued in `010-S`.

## Risks and Caveats

* **Secret scan guardrails**: Documentation examples must never include real or
  realistic passcode tokens (`?p=<non-synthetic>`) that would trip
  `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`. Run
  `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`
  after staging any new files with `git add` (because the test inspects
  `git ls-files`).

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Documentation-only scope; zero Go production code changes |
| II: Test first | Existing test suite (`go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` after `git add`, plus `go test -race ./...`) verifies repository integrity |
| III, IV: Secrets & containment | All files reside under workspace root (`README.md`, `docs/`); synthetic URLs only |
| VI: Single responsibility | Three `docs`-domain tasks (`1`, `2`, and `2` files respectively) with disjoint file paths |
| IX, X, XI: Persistence & merge history | Deliberation `009-DL`, plan, feature `020-F`, tasks `020.001-T`..`020.003-T`, and shipment `020-S` (`blocks 019-S`) recorded |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **absent**: documents already-shipped
  CLI flags, config fields, and internal packages without altering any behavior.
* Security, auth, permission, or compliance-sensitive behavior — **absent**:
  pure Markdown documentation update.
* Migration, backfill, destructive data/config action, or irreversible step —
  **absent**: additive/updated documentation files only.
* External integration, operator checkpoint, or external dependency —
  **absent**.
* High runtime, rollout, or rollback risk — **absent**: zero runtime impact.

Requires plan hardening: no

## Runtime Verification and Closure

* **Verification**:
  * Stage new and modified files with `git add` (required because
    `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` inspects `git ls-files`)
    and run
    `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`
  * `go test -race ./...`
  * `go vet ./...`
  * `golangci-lint run`
  * `gofmt -l .`
* **Operational closure**:
  * Pure documentation release unit (`020-S`); rollback is a standard `git revert`
    of the `020-S` merge commit if any documentation inaccuracy is discovered.

## Plan Review

dispatch_mode: single-agent-declared-degradation
decision: PASS

* **Capability declaration (P-012)**:
  `TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`
  (inline rubric evaluation across all applicable personas).

| Persona | Mode | Findings | Verdict |
|---|---|---|---|
| Constitution Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — All 3 tasks obey the 2-hour rule (`1`, `2`, and `2` files), width isolation (`docs` domain), and workspace containment. | PASS |
| Scope Boundary Auditor (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Bounded to `6227A297`; does not touch `010-S` / `010-F` / `001-DL` / `010.001-T`..`010.009-T` or any Go source files. | PASS |
| Learnings Researcher (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Incorporates `019.002-T` post-closure deliberation hygiene (`007-DL` and `008-DL` archived) and `shine-mp3` / updater learnings in architecture and design docs. | PASS |
| Architecture Strategist (cross-model rubric) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`, and `docs/design-docs/` follow `architecture-doc.instructions.md` progressive disclosure and graduation rules. | PASS |

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
feature `020-F` and shipment `020-S`.
