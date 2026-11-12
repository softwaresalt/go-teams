---
title: Repository documentation refresh and knowledge graduation decision
description: Deliberation on updating README.md, adding docs/ARCHITECTURE.md and docs/QUALITY_SCORE.md, and graduating 013-S..018-S design decisions into docs/design-docs/
topic: Update and complete go-teams repository documentation (6227A297)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 009-DL
feature: 020-F
shipment: 020-S
source_stash:
  - 6227A297
linked_artifacts:
  - docs/plans/2026-11-11-repo-documentation-refresh-plan.md
  - .backlogit/queue/009-DL.md
  - .backlogit/queue/020-F.md
  - .backlogit/queue/020-S.md
date: 2026-11-11
status: accepted
tags:
  - docs
  - architecture
  - quality-score
  - design-docs
  - readme
---

## Problem Frame

Stash entry **`6227A297`** (`feature`, `medium`) requests:

> Update and complete go-teams repository documentation: refresh README.md
> CLI/config/update-locking/layout reference, add docs/ARCHITECTURE.md and
> docs/QUALITY_SCORE.md mapped in AGENTS.md, and graduate 013-S..018-S design
> decisions into docs/design-docs/.

Across shipments `011-S` through `019-S`, `go-teams` (`teams-cli`) shipped
significant runtime, security, and operational capabilities whose durable
documentation has drifted or remains ungraduated:

1. **`README.md` operator reference drift**:
   * Batch queue mode (`--queue` / `-q <file>` in `internal/cli/queue.go` and
     `internal/cli/join.go`) is shown in a single `--help` example on `root.go`
     but is missing from `README.md` (`## Commands`, flag reference, and batch
     execution/commenting/redaction behavior: `writeQueueLines` in
     `internal/cli/queue.go:200` attempts an atomic temp-file rename and falls
     back to an in-place `os.WriteFile` if the rename fails,
     `inheritedQueueFlags` forwards outer capture flags except `--url` and
     `--queue`, and `teams.RedactArgs` redacts query strings, userinfo, and
     fragments).
   * Capture flags `--category` (`-c`), `--filename-template` (and alias
     `--name-template`), and `--output-folder`, along with the complete output
     target precedence (positional output argument `arg[1]` > `--output-file` >
     `--output` > `--output-dir` > `--output-folder` > config `output_file` >
     config `output_folder` > config `output_dir`) and the default filename
     template `<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md`, are
     implemented in `internal/cli/join.go`, `internal/config/config.go`, and
     `internal/capture/output.go` but omitted from `README.md`.
   * `### Self-update` (`teams-cli update`) describes the `016-S` `.old` rename
     sequence but omits the `018-S` inter-process lock protocol (`<exe>.lock`
     with `O_CREATE|O_EXCL`, `0o600`, JSON owner metadata `pid`/`token`/`created_at`,
     15s wait timeout / 50ms poll, fail-closed no-auto-eviction policy, ordered
     prefix lock chain `acquireSwapLockChain` for `.old` targets, `<exe>.lock.ctl`
     non-cancelable token-verified release, pre-swap `statExecutable` wait across
     the transient `.old` rename window, launch/installed-version identity
     guards `ErrConcurrentUpdateModified` / `ErrDowngrade` / `ErrUpdateLocked`,
     and safe manual operator lock/backup recovery preconditions).
   * `## Configuration` omits `output_folder` (and `default_path`) as the
     directory-only alias for `output_dir`, the YAML `output` file-or-directory
     target (`internal/config/config.go:192-198`, where `config.Load` treats
     `.md`/`.markdown` values in `output` as `OutputFile` — overriding
     `output_file` when `output_file` is unset — and other values as
     `OutputDir`), `output_file`, `filename_template` (and `name_template`
     alias), `category`, `browser_path`, and `audio` alias details.
   * `## Project layout` lists 8 of the 10 `internal/` packages, omitting
     `internal/audio/` and `internal/capture/`.
2. **Missing progressive-disclosure maps (`docs/ARCHITECTURE.md` and `docs/QUALITY_SCORE.md`)**:
   * `AGENTS.md` (`## Repository Knowledge (Progressive Disclosure)`) and
     `.github/instructions/architecture-doc.instructions.md` both signpost
     `docs/ARCHITECTURE.md` and `docs/QUALITY_SCORE.md`, neither of which exists
     in `docs/`.
   * `docs/ARCHITECTURE.md` must accurately document the 10 `internal/`
     packages, dependency direction, runtime flows (including batch `--queue`
     `writeQueueLines` in `internal/cli/queue.go:200` atomic temp-file rename
     with in-place `os.WriteFile` fallback and `inheritedQueueFlags` forwarding
     outer capture flags except `--url` and `--queue`), and security/isolation
     invariants (including that `internal/capture` creates *new* directories
     with `0o700` and *new* transcript/MP3 files with `0o600`, while
     pre-existing directories and explicitly overwritten files retain their
     existing filesystem permissions).
   * `docs/QUALITY_SCORE.md` must accurately document per-domain quality grades
     and test coverage tiers, referencing the exact test names and paths
     `TestChromeMeetingEndToEnd` (`internal/browser/meeting_e2e_test.go:25`),
     `TestCompiledBinaryCapturesRecapToExplicitOutput`
     (`internal/cli/binary_automation_test.go:16`),
     `TestEncoderDeterministicGoldenReference` (`internal/audio/mp3_test.go`,
     which verifies inline golden SHA-256 digests and MP3 frame headers), and
     `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` (`internal/config`,
     which inspects `git ls-files`).
3. **Ungraduated design decisions (`013-S`, `016-S`, `017-S`, `018-S`)**:
   * `docs/design-docs/` contains `0001-cli-foundation.md` (`001-S`) and
     `2026-11-09-teams-browser-caption-and-recap-capture.md` (`011-S`, partially
     updated in `015-S`).
   * The self-update and inter-process lock architecture from `016-S` (`005-DL`)
     and `018-S` (`007-DL`) has no design doc in `docs/design-docs/`.
   * `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`
     does not yet reflect the `013-S` (`002-DL`) URL host allowlist, userinfo
     rejection, and error/log query/passcode redaction in `internal/teams` and
     `internal/browser`, nor the `017-S` (`006-DL`) `cleanBrowserArgsForOptions`
     contract honoring `Options.DisableSandbox` and `TEAMS_DISABLE_SANDBOX=1|true`
     in direct-exec browser launches while keeping the Chromium sandbox enabled
     by default.

## Post-Closure Deliberation Hygiene Record (`007-DL` & `008-DL`)

Per the `019.002-T` post-closure Stage archival rule in
`.github/skills/shipment-reconcile/SKILL.md` and
`docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`:

* Covering shipments `018-S` and `019-S` are in `.backlogit/archive/` with
  `archived_status: shipped`, and covering features `018-F` and `019-F` are in
  `.backlogit/archive/` with `archived_status: done`.
* Both `007-DL` and `008-DL` had `status: queued` in `.backlogit/queue/` with
  zero remaining live governing/dependency references in `.backlogit/queue/` or
  `.backlogit/stash.jsonl`.
* Stage archived both orphaned deliberations (`backlogit archive 007-DL` and
  `backlogit archive 008-DL`) at session start.

## Options Evaluated

### Option A: Single monolithic documentation task

Bundle all `README.md`, `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`, and
`docs/design-docs/*.md` edits into a single task under `020-F`.

* **Pros**: Single task item to track.
* **Cons**: Touches 5 files across three distinct documentation surfaces
  (user-facing CLI reference, progressive-disclosure architecture/quality maps,
  and graduated design records), violating the 2-Hour Rule constraint (`< 3`
  files per task).
* **Effort**: medium | **Fit**: low.

### Option B: Three independent, width-isolated `docs` tasks touching disjoint file sets (chosen)

Decompose `020-F` into three atomic, width-isolated `docs` tasks (`<= 2` files
each) with zero overlapping file paths and no intra-feature `blocks` dependency
edges:

1. **`020.001-T` (`README.md`)**: User-facing CLI, batch queue
   (`writeQueueLines` in `internal/cli/queue.go:200` atomic temp-file rename
   with in-place `os.WriteFile` fallback, `inheritedQueueFlags` forwarding outer
   capture flags except `--url` and `--queue`, `teams.RedactArgs` redaction),
   complete capture flag and output target precedence (`arg[1]` >
   `--output-file` > `--output` > `--output-dir` > `--output-folder` > config
   `output_file` > config `output_folder` > config `output_dir`), self-update
   lock/recovery, configuration (specifying `output_folder` and `default_path`
   as directory-only aliases for `output_dir` separately from the YAML `output`
   file-or-directory target in `internal/config/config.go:192-198`, where
   `config.Load` treats `.md`/`.markdown` values in `output` as `OutputFile` —
   overriding `output_file` when `output_file` is unset — and other values as
   `OutputDir`), and project layout reference (1 file).
2. **`020.002-T` (`docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`)**:
   Top-level domain map, dependency direction, runtime flows (including batch
   `--queue` `writeQueueLines` in `internal/cli/queue.go:200` atomic rename with
   `os.WriteFile` fallback and `inheritedQueueFlags` excluding `--url` and
   `--queue`), security/isolation invariants (including `internal/capture`
   creating *new* directories with `0o700` and *new* transcript/MP3 files with
   `0o600` while pre-existing directories and explicitly overwritten files
   retain their existing permissions), per-package quality grades, test coverage
   tiers (`TestChromeMeetingEndToEnd` in
   `internal/browser/meeting_e2e_test.go:25`,
   `TestCompiledBinaryCapturesRecapToExplicitOutput` in
   `internal/cli/binary_automation_test.go:16`,
   `TestEncoderDeterministicGoldenReference` in `internal/audio/mp3_test.go`,
   `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` in `internal/config`),
   and explicit live-verification gaps (`010-S`) (2 files).
3. **`020.003-T` (`docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`, `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`)**:
   Graduate `016-S` + `018-S` self-update and inter-process locking design
   decisions and update the browser caption/recap capture design doc for
   `013-S` + `017-S` (2 files).

* **Pros**: Every task touches `<= 2` files within a single domain (`docs`),
  strictly satisfies the 2-Hour Rule, and avoids artificial sequential blocking
  because the target file sets are completely disjoint.
* **Cons**: None.
* **Effort**: low | **Fit**: high.

## Trade-off Comparison

| Criterion | Option A (Monolithic 1-task sweep) | Option B (3 disjoint width-isolated `docs` tasks) |
|---|---|---|
| File count per task (2-Hour Rule `< 3` files) | 5 files (violates `< 3` rule) | 1 file, 2 files, 2 files (compliant) |
| Parallelism / execution independence | Coupled | Fully independent (disjoint file sets, no intra-feature `blocks` edges) |
| Alignment with `architecture-doc.instructions.md` | Partial | Exact separation of `README.md`, `ARCHITECTURE.md` + `QUALITY_SCORE.md`, and `docs/design-docs/` |

## Decision

We select **Option B** and decompose the work into feature **`020-F`**
(*Update and complete go-teams repository documentation and graduate 013-S..018-S design decisions*)
and shipment **`020-S`** (chained after shipped `019-S` via
`backlogit dep add 020-S 019-S --type blocks`) with three independent `docs`
tasks (`020.001-T`, `020.002-T`, `020.003-T`), each sized `S` with complexity
`low`.

## Rejected Alternatives

* **Option A (Single monolithic documentation task)**: Rejected because modifying
  5 files in a single task exceeds the 2-Hour Rule's `< 3` files limit.
* **Adding sequential `blocks` edges between `020.001-T`, `020.002-T`, and `020.003-T`**:
  Rejected because the three tasks modify completely disjoint Markdown files and
  have no code or artifact build dependency on one another.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Accidental secret/token pattern trigger in documentation examples**:
  Mitigated by using clearly synthetic URLs (`https://teams.microsoft.com/meet/synthetic`,
  `https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic`), staging
  any new files with `git add` first (because the test inspects `git ls-files`),
  and running
  `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`
  as part of verification.
* **Markdown style compliance**: Mitigated by following
  `.github/instructions/markdown.instructions.md` and
  `.github/instructions/writing-style.instructions.md` (YAML frontmatter with
  `title:` where appropriate, ATX headings without H1 when `title:` is in
  frontmatter, `*` unordered lists, fenced code blocks with language tags, and
  single trailing newline).
