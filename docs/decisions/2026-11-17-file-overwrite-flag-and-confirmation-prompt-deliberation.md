---
title: File overwrite configuration, --overwrite CLI flag, and interactive (Y)es | (N)o confirmation prompt
description: Deliberation on stash entry DF20598F for covering feature 034-F and shipment 034-S
topic: File Overwrite Configuration, --overwrite CLI Flag, and Interactive (Y)es | (N)o Confirmation Prompt (034-S)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 023-DL
feature: 034-F
shipment: 034-S
source_stash:
  - DF20598F
linked_artifacts:
  - docs/plans/2026-11-17-file-overwrite-flag-and-confirmation-prompt-decided-plan.md
  - docs/archive/plans/2026-11-17/2026-11-17-file-overwrite-flag-and-confirmation-prompt-plan.md
  - .backlogit/archive/023-DL.md
  - .backlogit/archive/034-F.md
  - .backlogit/archive/034-S.md
date: 2026-11-17
status: accepted
tags:
  - config
  - cli
  - capture
  - scheduler
  - overwrite
  - prompt
---

## Problem Frame

In `go-teams`, capture output file creation in `setupOutput` (`internal/capture/output.go`) currently has two divergent and rigid behaviors when target transcript (`.md`) or audio (`.mp3`) files already exist on disk:

1. **Directory mode (`OutputConfig.Dir`, default `<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md`)**:
   * `setupOutput` opens both `mdPath` and `mp3Path` with `os.O_CREATE | os.O_WRONLY | os.O_EXCL`, failing immediately with `os.ErrExist` if either file already exists, with no configuration option, CLI flag, or interactive confirmation prompt to replace an existing same-day capture.
   * Furthermore, if `mdPath` does not exist and is newly created, but `mp3Path` already exists on disk, `setupOutput` removes the newly created `mdPath` on rollback; however, if `mdPath` pre-existed and were opened with truncation or if an error occurred during `WriteHeader`, `Flush`, or `mp3File` creation, calling `os.Remove(mdPath)` unconditionally would delete a pre-existing file. Similarly, in `Session.Run` (`internal/capture/session.go`), the deferred cleanup on `hadAudioErr` removes `out.AudioPath` if `info.Size() == 0`, which could delete a pre-existing 0-byte audio file unless pre-existing state is tracked (`!out.AudioExisted`).
2. **Explicit file mode (`OutputConfig.FilePath`, set via positional `arg[1]`, `--output-file`, `--output <file.md>`, or `output_file` in `config.yaml`)**:
   * `setupOutput` unconditionally sets `openFlg = os.O_CREATE | os.O_WRONLY | os.O_TRUNC`, silently truncating and overwriting any existing `.md` and `.mp3` files without checking `overwrite` configuration or prompting the operator.

Stash entry **`DF20598F`** (`priority: high`, `kind: feature`) requires unifying file overwrite handling across configuration, capture session/output, scheduler, and CLI:

1. **Configuration & Template (`internal/config/config.go`, `internal/config/template.go`, `config.example.yaml`)**:
   * Add `Overwrite bool` (`yaml:"overwrite,omitempty"`, default `false` in `DefaultConfig()`) to `config.Config`, with `file_overwrite` supported as a YAML alias in `config.Load` (`overwrite` taking precedence over `file_overwrite` when both are specified).
   * Add `Overwrite *bool` (`yaml:"overwrite,omitempty"`, plus `file_overwrite` alias support) to `config.MeetingSchedule` so scheduled `meetings:` entries can override the global `overwrite` setting per meeting.
   * Document `# overwrite: false` and `#   Alias: file_overwrite` in `templateYAML` (`internal/config/template.go`) and `config.example.yaml` under `# Where transcripts are saved` (and `# overwrite: false` in the `# meetings:` example block), keeping `TemplateYAML()` and `config.example.yaml` identical.
2. **Capture Output, Session & Scheduler (`internal/capture/output.go`, `internal/capture/session.go`, `internal/scheduler/schedule.go`)**:
   * Define sentinel error `var ErrOverwriteDeclined = errors.New("capture: file overwrite declined")` in `internal/capture/output.go`.
   * Add `Overwrite bool` and `ConfirmOverwrite func(paths []string) (bool, error)` to `capture.OutputConfig` and `capture.SessionConfig`, wired from `Session.Run` into `outCfg`.
   * Track `AudioExisted bool` on `capture.Output` (and `mdExisted` / `mp3Existed` in `setupOutput`) so pre-existing files are never deleted on rollback (`WriteHeader`, `Flush`, or `mp3Path` open error) or on deferred 0-byte audio cleanup in `Session.Run` (`!out.AudioExisted`).
   * Enforce a uniform overwrite policy in `setupOutput` across both `Dir` and `FilePath` modes: inspect `mdPath` and (when `!cfg.CaptionsOnly`) `mp3Path` via `os.Stat` after the path-collision and `os.SameFile` checks; reject directory targets immediately (`capture: output path is a directory: ...`); if any target file exists and `!cfg.Overwrite`, require a non-nil `cfg.ConfirmOverwrite` callback (returning an error wrapping `os.ErrExist` if `nil`, wrapping the callback error if non-nil, or wrapping `ErrOverwriteDeclined` if `!approved` — all before opening, truncating, or deleting any file); and open existing approved files with `os.O_CREATE | os.O_WRONLY | os.O_TRUNC` while opening non-existing files with `os.O_CREATE | os.O_WRONLY | os.O_EXCL`.
   * Add `Overwrite *bool` to `scheduler.Entry` (`internal/scheduler/schedule.go`).
3. **CLI Flag & Interactive `(Y)es | (N)o` Prompt (`internal/cli/root.go`, `internal/cli/join.go`, `internal/cli/schedule.go`)**:
   * Bind `--overwrite` on root, `join`, `captions` (`captureFlags` in `internal/cli/join.go`), and `schedule` (`scheduleFlags` in `internal/cli/schedule.go`), apply it in `applyCaptureFlags` and `applyScheduleFlags`, forward it in `inheritedQueueFlags` (`--overwrite` or `--overwrite=false` when changed), and propagate `Overwrite` in `buildScheduleEntryConfig` and `buildSchedulerEntryFromConfig`.
   * Implement `promptOverwriteApproval(in io.Reader, out io.Writer, paths []string) (bool, error)` writing `"File already exists: %s. Overwrite? (Y)es | (N)o: "` to `out` (defaulting to `os.Stderr` / `cmd.ErrOrStderr()`) and reading one line from `in` (defaulting to `os.Stdin` / `cmd.InOrStdin()`), accepting case-insensitive `"y"`, `"yes"`, or `"(y)es"` as approval and treating `"n"`, `"no"`, `"(n)o"`, `""`, or any other input as decline (`false, nil`).
4. **Tests & Documentation (`internal/config/config_test.go`, `internal/capture/output_test.go`, `internal/cli/join_test.go`, `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`)**:
   * Comprehensive unit tests across `internal/config`, `internal/capture`, and `internal/cli` plus documentation updates in `docs/USER_GUIDE.md` and `docs/ARCHITECTURE.md`.

### P-021 C5 and C6 Triage Record

* `DF20598F` does not carry the `DEFERRED SCOPE EXPANSION` marker; it is a high-priority feature entry (`kind: feature`, `priority: high`).
* **Obligation (A) Unconditional duplicate detection**: Executed across `.backlogit/stash.jsonl`, `.backlogit/archive/`, and `.backlogit/queue/`. Outcome: **CLEAN DUPLICATE SCAN** for `DF20598F`. No other active or archived stash entry or backlog item addresses `overwrite` / `file_overwrite` configuration, `--overwrite` CLI flags, or interactive `(Y)es | (N)o` overwrite confirmation prompts for capture output files.
* **Obligation (B) Late-identifier reconciliation**: Not triggered (no `N/A` deferred-expansion source refs).

### Scope Boundaries

* **In scope**:
  * **U1 (`034.001-T` — `code` domain: `internal/config/config.go`, `internal/config/template.go`, `config.example.yaml`, `Size: S`, `Complexity: low`, 3 files)**:
    * Add `Overwrite bool` (`yaml:"overwrite,omitempty"`) to `config.Config` and `Overwrite *bool` (`yaml:"overwrite,omitempty"`) to `config.MeetingSchedule`.
    * Support `file_overwrite` alias at both top-level (`aliases.Overwrite *bool`, `aliases.FileOverwrite *bool`) and per-meeting (`aliases.Meetings[].Overwrite *bool`, `aliases.Meetings[].FileOverwrite *bool`) in `config.Load`.
    * Document `# overwrite: false` and `#   Alias: file_overwrite` in `templateYAML` (`internal/config/template.go`) and `config.example.yaml` (and `# overwrite: false` in the `# meetings:` example block).
  * **U2 (`034.002-T` — `code` domain: `internal/capture/output.go`, `internal/capture/session.go`, `internal/scheduler/schedule.go`, `Size: M`, `Complexity: medium`, depends on `034.001-T`, 3 files)**:
    * Define `ErrOverwriteDeclined` in `internal/capture/output.go`, add `Overwrite bool` and `ConfirmOverwrite func(paths []string) (bool, error)` to `OutputConfig` and `SessionConfig`, add `AudioExisted bool` to `Output`, enforce uniform pre-open existence checks and `ConfirmOverwrite` gating in `setupOutput` while preserving pre-existing files byte-for-byte when declined/unconfirmed (or when pre-open directory/stat checks fail) and never deleting (`os.Remove`) pre-existing files on post-open rollback (`!mdExisted`) or 0-byte audio error (`!out.AudioExisted`), check `isShutdown(ctx, ctx.Err())` in `Session.Run` before `setupOutput`, and add `Overwrite *bool` to `scheduler.Entry` in `internal/scheduler/schedule.go`.
  * **U3 (`034.003-T` — `code` domain: `internal/cli/root.go`, `internal/cli/join.go`, `internal/cli/schedule.go`, `Size: S`, `Complexity: low`, depends on `034.001-T` and `034.002-T`, 3 files)**:
    * Bind `--overwrite` on `captureFlags` (`internal/cli/join.go`) and `scheduleFlags` (`internal/cli/schedule.go`), apply it in `applyCaptureFlags` and `applyScheduleFlags`, forward it in `inheritedQueueFlags`, propagate `Overwrite` in `buildScheduleEntryConfig` and `buildSchedulerEntryFromConfig`, and implement `promptOverwriteApproval` wired via `d.promptIn` / `d.promptOut` on `deps` (bound from `cmd.InOrStdin()` and `cmd.ErrOrStderr()` when `!isInsideQueueExecution(cmd.Context())` so outer command streams are preserved across `--queue` execution while keeping `config.Config` free of runtime I/O fields).
  * **U4 (`034.004-T` — `test` domain: `internal/config/config_test.go`, `internal/capture/output_test.go`, `internal/cli/join_test.go`, `Size: S`, `Complexity: low`, depends on `034.001-T`, `034.002-T`, `034.003-T`, 3 files)**:
    * Add unit tests in `internal/config/config_test.go`, `internal/capture/output_test.go`, and `internal/cli/join_test.go` covering config/alias/meeting parsing, `setupOutput` uniform overwrite/prompt/decline/rollback preservation and `Session.Run` `AudioExisted` preservation, and CLI `--overwrite`, `inheritedQueueFlags`, schedule precedence, and interactive `(Y)es | (N)o` prompt handling.
  * **U5 (`034.005-T` — `docs` domain: `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`, `Size: XS`, `Complexity: trivial`, depends on `034.001-T`, `034.002-T`, `034.003-T`, 2 files)**:
    * Document `--overwrite`, `overwrite` / `file_overwrite` (including `meetings[].overwrite`), and the interactive `(Y)es | (N)o` confirmation prompt in `docs/USER_GUIDE.md` and `docs/ARCHITECTURE.md`.
* **Out of scope**:
  * All other active stash entries (`9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, `3F6BA819`, `39450846`, `7883544E`, `59545A24`, `C3460FDC`, `4AB8EAEE`, `E03CB3C3`, `4C40AF2B`, `BA069506`, `196AB9E4`, `69AB7ACC`, `0DE2E900`), which remain untouched in `active` stash state.
  * Note on `docs/CLI_REFERENCE.md`: the stash mentions `docs/USER_GUIDE.md, docs/CLI_REFERENCE.md if applicable`; `go-teams` consolidates its CLI reference inside `docs/USER_GUIDE.md` and `docs/ARCHITECTURE.md` (there is no `docs/CLI_REFERENCE.md` file in the repository).

## Research Findings

### Prior Learnings (`docs/compound/`)

* **`docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`**: `internal/config/template_test.go` verifies that `config.example.yaml` matches `TemplateYAML()` byte-for-byte (after CRLF normalization) and uses reflection on `Config{}` and `MeetingSchedule{}` to verify that every `yaml` struct tag key is documented in `TemplateYAML()` and that uncommenting all canonical keys and the `# meetings:` block produces a valid `Config`.
* **`docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`**: User-facing CLI prompts and error messages must only echo local filesystem paths (`existingPaths`) and never echo unredacted meeting URLs or passcodes.

### Codebase Investigation

1. **`internal/config/config.go` & `internal/config/template.go`**:
   * `Config` and `MeetingSchedule` use `gopkg.in/yaml.v3` struct tags and a secondary `aliases` struct in `config.Load` to resolve alias precedence (`overwrite` over `file_overwrite`).
   * `template_test.go` defines `canonicalTemplateKeys` and `documentedTemplateKeys` and runs `uncommentCanonical(tpl)` which uncomments `# meetings:` and all `# <canonical_key>:` lines. Adding `# overwrite: false` and `#   Alias: file_overwrite` under `# Where transcripts are saved` and `# overwrite: false` in the `# meetings:` example block satisfies `TestTemplateYAMLContentAndLoad` (`documents every Config and MeetingSchedule yaml key` and `uncommented canonical examples load and validate`) and `TestExampleConfigMatchesTemplateYAML` without modifying `template_test.go`.
2. **`internal/capture/output.go` & `internal/capture/session.go`**:
   * In `setupOutput`, `explicitFile := cfg.FilePath` currently sets `openFlg = os.O_CREATE | os.O_WRONLY | os.O_TRUNC` while `Dir` mode sets `openFlg = os.O_CREATE | os.O_WRONLY | os.O_EXCL`.
   * Unifying both modes requires checking `os.Stat(mdPath)` and (when `!cfg.CaptionsOnly`) `os.Stat(mp3Path)` after the path-collision and `os.SameFile` checks:
     * If `err == nil && info.IsDir()`, fail immediately (`capture: output path is a directory: %s`).
     * Collect `existingPaths` in deterministic order (`mdPath` if `mdExisted`, then `mp3Path` if `!cfg.CaptionsOnly && mp3Existed`).
     * When `len(existingPaths) > 0 && !cfg.Overwrite`:
       * If `cfg.ConfirmOverwrite == nil`, return `fmt.Errorf("capture: target file already exists (use --overwrite or overwrite: true): %s: %w", existingPaths[0], os.ErrExist)` without opening, truncating, or deleting any file.
       * Otherwise call `approved, err := cfg.ConfirmOverwrite(existingPaths)`. If `err != nil`, return `fmt.Errorf("capture: confirm overwrite: %w", err)`; if `!approved`, return `fmt.Errorf("%w: %s", ErrOverwriteDeclined, strings.Join(existingPaths, ", "))` — both before opening or mutating any file on disk.
     * Open `mdPath` with `os.O_CREATE|os.O_WRONLY|os.O_TRUNC` if `mdExisted` (when overwrite is permitted), else `os.O_CREATE|os.O_WRONLY|os.O_EXCL`. On subsequent `WriteHeader`, `Flush`, or `mp3Path` open failure, only call `_ = os.Remove(mdPath)` when `!mdExisted`.
     * Open `mp3Path` (when `!cfg.CaptionsOnly`) with `os.O_CREATE|os.O_WRONLY|os.O_TRUNC` if `mp3Existed` (when overwrite is permitted), else `os.O_CREATE|os.O_WRONLY|os.O_EXCL`, and record `AudioExisted: mp3Existed` on `Output`.
     * In `Session.Run`'s deferred cleanup, only remove a 0-byte `out.AudioPath` on `hadAudioErr` when `!out.AudioExisted`.
3. **`internal/cli/root.go`, `internal/cli/join.go`, `internal/cli/schedule.go`**:
   * `root`, `join`, and `captions` all share `captureFlags` (`bindCaptureFlags` and `applyCaptureFlags` in `internal/cli/join.go`) and forward batch queue flags via `inheritedQueueFlags`.
   * `schedule` uses `scheduleFlags` (`bindScheduleFlags` and `applyScheduleFlags` in `internal/cli/schedule.go`) and merges per-meeting `MeetingSchedule` / `scheduler.Entry` settings in `buildSchedulerEntryFromConfig` and `buildScheduleEntryConfig`.
   * Passing `cmd.InOrStdin()` and `cmd.ErrOrStderr()` (via `PromptIn io.Reader` and `PromptOut io.Writer` `yaml:"-"` fields on `config.Config` populated in `applyCaptureFlags` / `applyScheduleFlags`, falling back to `os.Stdin` and `os.Stderr` when `nil`) allows `defaultDeps().newSession` to wire `ConfirmOverwrite` cleanly without changing the `deps.newSession` function signature (`func(cfg config.Config, meetingURL string) meetingSession`) used across existing CLI tests.

## Options Evaluated

### Option A: Single Unified `overwrite` Config/Flag (`file_overwrite` Alias + Per-Meeting Override) and Interactive `(Y)es | (N)o` Prompt Across Both `Dir` and `FilePath` Modes (Recommended)

* **Description**:
  * Implement a single boolean `Overwrite` setting (`overwrite` / `file_overwrite` in YAML, `--overwrite` on CLI, `Overwrite *bool` on `MeetingSchedule` and `scheduler.Entry`), uniform existence checking and `ConfirmOverwrite` prompting in `setupOutput`, pre-existing file preservation on decline and rollback (`!mdExisted`, `!out.AudioExisted`), and `promptOverwriteApproval` on `cmd.InOrStdin()` / `cmd.ErrOrStderr()`.
* **Pros**:
  * Eliminates accidental silent truncation in `FilePath` mode while enabling controlled or interactive overwrite in `Dir` mode.
  * Preserves pre-existing `.md` and `.mp3` files untouched when declined (`ErrOverwriteDeclined`), when `ConfirmOverwrite == nil` in non-interactive library calls, or when a subsequent setup/audio error triggers rollback.
  * Keeps `deps.newSession` signature unchanged by carrying `PromptIn` / `PromptOut` (`yaml:"-"`) on `config.Config`.
* **Cons**:
  * Updates existing test `TestSetupOutputExplicitFilePathCreatesAndOverwritesMarkdownFile` in `internal/capture/output_test.go` to reflect that `FilePath` mode now requires `Overwrite: true` or `ConfirmOverwrite` approval when the destination file already exists.
* **Effort estimate**: Medium (5 atomic, width-isolated tasks: `S`, `M`, `S`, `S`, `XS`).
* **Fit**: 100% alignment with stash `DF20598F` and architecture boundaries.

### Option B: Separate `--overwrite-transcript` and `--overwrite-audio` Flags and Config Keys

* **Description**: Provide independent overwrite flags and prompts for `.md` and `.mp3` files.
* **Pros**: Fine-grained control if an operator wants to overwrite only one artifact of a capture pair.
* **Cons**: Violates the stash requirement for a single file overwrite configuration and CLI flag (`overwrite` / `--overwrite`) and complicates the interactive prompt UX for a single capture session.

### Option C: Prompt Only in `Dir` Mode and Keep Silent Truncation in `FilePath` Mode

* **Description**: Leave `FilePath` mode unconditionally truncating existing files and only check `Overwrite` / `ConfirmOverwrite` when `FilePath == ""`.
* **Pros**: No change to `FilePath` behavior.
* **Cons**: Leaves explicit `-o existing.md` invocations vulnerable to silent data loss when `--overwrite` was not set and violates the uniform overwrite policy required by `DF20598F`.

## Trade-off Comparison

| Criterion | Option A (Unified `overwrite` + `(Y)es \| (N)o` Prompt) | Option B (Separate `.md` / `.mp3` Flags) | Option C (`Dir`-Only Prompt, Silent `FilePath` Truncation) |
|---|---|---|---|
| Protects existing `.md` and `.mp3` files in both `Dir` and `FilePath` modes | Yes | Yes | No (silent truncation in `FilePath` mode) |
| Matches stash `DF20598F` single-flag / single-config contract | Yes | No | No |
| Preserves pre-existing files on decline and rollback (`!mdExisted`, `!out.AudioExisted`) | Yes | Partial | No |
| Configuration and CLI ergonomics | Simple (`--overwrite`, `overwrite: true`) | Verbose (2 flags, 2 config keys) | Inconsistent across modes |

## Decision

We select **Option A**: promote stash entry **`DF20598F`** to covering feature **`034-F`** and decompose it into five width-isolated tasks (`034.001-T`..`034.005-T`) assembled into shipment **`034-S`**, sequenced in the ready set after `033-S` and before `010-S` (`033-S -> 034-S -> 010-S`).

## Rejected Alternatives

* **Option B** was rejected because a capture session produces a coupled transcript + audio pair for the same meeting, and stash `DF20598F` explicitly specifies a single `overwrite` / `--overwrite` boolean setting and a single prompt listing all existing target paths.
* **Option C** was rejected because silent truncation in `FilePath` mode when `Overwrite` is `false` risks unintentional loss of existing transcripts or recordings.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Risk — Deleting a pre-existing `.md` or `.mp3` file on partial setup failure or 0-byte audio error**:
  * **Mitigation**: Track `mdExisted` and `mp3Existed` (`Output.AudioExisted`) before opening files in `setupOutput`. On `WriteHeader`, `Flush`, or `mp3Path` open failure, only call `os.Remove(mdPath)` when `!mdExisted`. In `Session.Run`'s deferred cleanup, only remove a 0-byte `out.AudioPath` on `hadAudioErr` when `!out.AudioExisted`.
* **Risk — `TestTemplateYAMLContentAndLoad` or `TestExampleConfigMatchesTemplateYAML` failing in `internal/config/template_test.go`**:
  * **Mitigation**: Keep `internal/config/template.go` (`templateYAML`) and `config.example.yaml` byte-for-byte identical (with LF line endings), document `# overwrite: false` and `#   Alias: file_overwrite` under `# Where transcripts are saved`, and include `# overwrite: false` in the `# meetings:` example block so reflection on `Config` and `MeetingSchedule` finds every YAML key and `uncommentCanonical` validates cleanly.
* **Risk — Blocking indefinitely on `os.Stdin` in unit tests that do not set `ConfirmOverwrite`**:
  * **Mitigation**: In `setupOutput`, when `cfg.ConfirmOverwrite == nil` and `!cfg.Overwrite`, fail immediately with `os.ErrExist` without reading `os.Stdin`. In CLI `defaultDeps().newSession`, wire `ConfirmOverwrite` using `cfg.PromptIn` (`cmd.InOrStdin()`) and `cfg.PromptOut` (`cmd.ErrOrStderr()`), which handle `io.EOF` cleanly (returning `false, nil` when empty or declined).
