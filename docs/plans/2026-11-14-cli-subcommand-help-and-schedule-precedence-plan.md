---
title: Comprehensive CLI subcommand help, root help placeholder guidance, and schedule flag precedence implementation plan
description: Implementation plan for 025-F / 025-S adding installRootHelpHandler, comprehensive Long and Example help on all 7 CLI commands, unit tests in internal/cli/help_test.go, and schedule flag vs per-meeting config precedence documentation in docs/USER_GUIDE.md and README.md
source: docs/decisions/2026-11-14-cli-subcommand-help-and-schedule-precedence-deliberation.md
deliberation_id: 014-DL
feature: 025-F
shipment: 025-S
source_stash:
  - A52B6644
  - 11ED4180
date: 2026-11-14
status: approved
tags:
  - cli
  - help
  - schedule
  - docs
  - ux
---

## Problem Frame

Following the accepted deliberation in
[2026-11-14-cli-subcommand-help-and-schedule-precedence-deliberation.md](../decisions/2026-11-14-cli-subcommand-help-and-schedule-precedence-deliberation.md)
(`014-DL`), stash entries **`A52B6644`** (`bug`, `high`) and **`11ED4180`**
(`task`, `low`, `DEFERRED SCOPE EXPANSION` from `024-S`) are planned under
covering feature **`025-F`** and shipment **`025-S`** (sequenced after
`024-S`):

1. **Root `--help` placeholder/unknown-command trap and incomplete subcommand `--help` (`A52B6644`)**:
   * Because `newRootCommand` (`internal/cli/root.go`) sets
     `Use: "teams-cli [meeting-url] [output-markdown-path]"` with
     `Args: cobra.MaximumNArgs(2)`, running `teams-cli [command] --help`
     (copying the literal `[command]` token from Cobra's footer
     `Use "teams-cli [command] --help" for more information about a command.`)
     or `teams-cli <unknown-subcommand> --help` causes Cobra to match `root`
     with `args = ["[command]"]` and print the generic root help screen without
     any hint that `[command]` is not a subcommand.
   * `newJoinCommand` (`internal/cli/join.go`) defines neither `Long` nor
     `Example`, so `teams-cli join --help` prints only its 1-line `Short`
     summary and the flag list.
   * `newCaptionsCommand` (`internal/cli/join.go`), `newAuthCommand`
     (`internal/cli/auth.go`), `newScheduleCommand` (`internal/cli/schedule.go`),
     and `newUpdateCommand` (`internal/cli/update.go`) have only 1–2 sentence
     `Long` summaries and no `Example` field.
   * `newRootCommand` (`internal/cli/root.go`) and `newGenerateConfigCommand`
     (`internal/cli/generate_config.go`) inline short examples inside `Long`
     rather than populating Cobra's `Example` field, and `root` lacks a summary
     of the 4 capture protocols, config discovery order, and how to invoke
     per-subcommand help (`teams-cli <subcommand> --help`).
2. **`teams-cli schedule` global CLI flag vs per-meeting config override precedence (`11ED4180`)**:
   * In `internal/cli/schedule.go`, `newScheduleCommand` applies global CLI
     flags (`--mode`, `--duration`, `--idle-timeout`, `--ai-summary-delay`,
     `--no-ai-summary`, `--lobby-timeout`, `--audio` / `--recap-audio`,
     `--browser-path`, `--profile-dir`, `--account`, `--headless`, `--headed`)
     to the loaded base `config.Config` first (`applyScheduleFlags`), and when
     each scheduled meeting fires, `buildScheduleEntryConfig(base, e)` applies
     that entry's per-meeting settings from `meetings:` in `config.yaml`
     (`name`, `category`, `output_dir`, `capture_audio`/`audio`,
     `mode`/`capture_mode`, `duration`, `idle_timeout`, `ai_summary_delay`,
     `lobby_timeout`). Per-meeting settings in `config.yaml` therefore override
     global `teams-cli schedule` flags for that meeting.
   * Both `teams-cli schedule --help` and `docs/USER_GUIDE.md` (`## Configuration`
     and `### Built-in daemon scheduling (teams-cli schedule)`) / `README.md`
     must state this 3-tier precedence explicitly and qualify the guide's
     statement that command-line flags override every config value.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Install `installRootHelpHandler(root *cobra.Command)` in `internal/cli/root.go` wrapping `root.HelpFunc()` via `root.SetHelpFunc(...)` and wiring `root.InitDefaultHelpCmd()` so when `--help` or `help <arg>` is invoked on `root` (`cmd == root`) with parsed positional arguments `len(posArgs) > 0` (`cmd.Flags().Args()` when parsed, or non-flag tokens from `args`) and `teams.ValidateMeetingURL(posArgs[0]) != nil`, it writes an explicit guidance note formatting `teams.RedactArgs([]string{posArgs[0]})` to `cmd.OutOrStdout()` before rendering root help | `A52B6644` | U1 (`025.001-T`) | `go test -race ./internal/cli/...` (`TestRootHelpPlaceholderAndUnknownCommandGuidance`) |
| R2: Populate comprehensive `Long` descriptions and dedicated `Example` fields on all 7 commands (`root`, `join`, `captions`, `auth`, `schedule`, `update`, `generate-config`), including the `schedule` 3-tier precedence rule (`11ED4180`) and `ephemeral-live` protocol normalization in `newScheduleCommand` | `A52B6644`, `11ED4180` | U1 (`025.001-T`) | `go test -race ./internal/cli/...` (`TestAllSubcommandsProvideComprehensiveHelpAndExamples`) |
| R3: Qualify "Command-line flags override every config value" in `docs/USER_GUIDE.md` (`## Configuration` and `### Built-in daemon scheduling (teams-cli schedule)`) and `README.md` by documenting `teams-cli schedule`'s 3-tier precedence (`buildScheduleEntryConfig`) and `ephemeral-live` protocol normalization, and update help examples in `README.md` and `docs/USER_GUIDE.md` to show `teams-cli <subcommand> --help` | `11ED4180`, `A52B6644` | U2 (`025.002-T`) | `go test -race ./...` including `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |

## Implementation Units

### U1 (`025.001-T`) — Add root `--help` placeholder/unknown-command interceptor and comprehensive `Long`/`Example` help across all CLI commands with unit tests

* **Domain:** code (`internal/cli`). **Posture:** test-first. **Size:** `M` | **Complexity:** `low`.
* **What:**
  1. **Red phase (`internal/cli/help_test.go`)**: Create table-driven unit tests
     in `package cli` first and observe failure before modifying the command
     definitions:
     * `TestAllSubcommandsProvideComprehensiveHelpAndExamples`:
       - Table-test every command (`root` via `[]string{"--help"}`, and each
         subcommand `auth`, `captions`, `generate-config`, `join`, `schedule`,
         `update` via both `[]string{sub, "--help"}` and
         `[]string{"help", sub}`).
       - Assert `ExecuteContext` succeeds, writes to `cmd.OutOrStdout()`, is
         strictly longer and richer than the 1-line `Short` summary, contains
         an `Examples:` section (without a duplicate `Examples:\nExamples:`
         header), and contains command-specific behavioral, flag, and precedence
         phrases:
         * `root`: 4 capture protocols (`auto`, `ephemeral-live`,
           `live-with-summary`, `past-recap`), config discovery order, and
           `teams-cli join --help`.
         * `join`: `<meeting-url>` / `--url` / `meeting_url`, 4 capture
           protocols (`auto`, `ephemeral-live`, `live-with-summary`,
           `past-recap`), `--duration` overtime + `--idle-timeout` +
           `--lobby-timeout`, delayed AI Summary (`--ai-summary-delay`,
           `--no-ai-summary`, `--append-ai-summary`, `--not-before`), all 8
           output target precedence levels (`1. positional arg[1]`,
           `2. --output-file`, `3. --output`, `4. --output-dir`,
           `5. --output-folder`, `6. config output_file`,
           `7. config output_folder`, `8. config output_dir`), all 8 filename
           template placeholders (`<category>`, `<concat-meeting-name>`,
           `<meeting-name>`, `<meeting-id>`, `<YYYYMMDD>`, `<YYYY-MM-DD>`,
           `<HHMMSS>`, `<timestamp>`), and `--queue` (`::` comments).
         * `captions`: optional `[meeting-url]` defaulting to
           `https://teams.microsoft.com/v2/`, 4 capture protocols, all 8 output
           precedence levels, all 8 filename template placeholders, and
           `--queue`.
         * `auth`: interactive headed login + MFA caching in `--profile-dir`
           (`%LOCALAPPDATA%\teams-cli\profile`), `--account` /
           `default_account` Entra account tile selection, and profile
           locking/exclusivity.
         * `schedule`: 5-field `cron` and one-shot `at` (`RFC3339` or
           `YYYY-MM-DD HH:MM`), shared-profile atomic `busy` gate released when
           the live call ends before `--ai-summary-delay`, startup sweep of
           deferred `--append-ai-summary` entries in `queue_file`, and explicit
           3-tier precedence stating that `teams-cli schedule` applies
           command-line flags to the base configuration first
           (`applyScheduleFlags`) and then applies each `meetings:` entry's
           per-meeting settings (`buildScheduleEntryConfig`:
           `mode`/`capture_mode`, `duration`, `idle_timeout`,
           `ai_summary_delay`, `lobby_timeout`, `category`,
           `capture_audio`/`audio`, `output_dir`), so per-meeting settings in
           `config.yaml` override global `schedule` flags for that meeting
           (followed by `ephemeral-live` protocol normalization forcing
           `ai_summary_delay` to `0s` and defaulting `capture_audio` to `true`
           unless audio was explicitly set false on the base config or flag, in
           which case `capture_audio: true` on the entry overrides it).
         * `update`: `teams-cli-<goos>-<goarch>`, `SHA256SUMS`, `<exe>.old`,
           `<exe>.lock` / `<exe>.lock.ctl`, `--check`, `--version`, `--force`,
           and manual lock recovery.
         * `generate-config`: `init-config` / `gen-config` aliases,
           `--output-dir` (`-d`), `--name` (`-n`), `--force`, `.yaml`/`.yml`
           positional path handling, and ignoring root `--config` / `-f`.
     * `TestRootHelpPlaceholderAndUnknownCommandGuidance`:
       - Table-test `[]string{"[command]", "--help"}`,
         `[]string{"<command>", "--help"}`,
         `[]string{"unknown-subcmd", "--help"}`,
         `[]string{"--help", "[command]"}`,
         `[]string{"-f", "config.yaml", "[command]", "--help"}`,
         `[]string{"help", "[command]"}`, and
         `[]string{"help", "unknown-subcmd"}`: assert output starts with
         `Note: "<arg>" is not a subcommand or a valid Teams meeting URL. Replace [command] with one of the subcommands below (for example: teams-cli join --help, teams-cli captions --help, teams-cli schedule --help, teams-cli auth --help, teams-cli generate-config --help, or teams-cli update --help).`
         followed by the root help output.
       - Test rejected URL redaction (`[]string{"http://user:secret@teams.microsoft.com/meet/123?p=secretpass", "--help"}`):
         assert the `Note:` banner prints the redacted URL
         `"http://teams.microsoft.com/meet/123"` via `teams.RedactArgs` and
         never echoes `user:secret` or `secretpass` (Principle III).
       - Table-test `[]string{"--help"}` (no positional args),
         `[]string{"-f", "config.yaml", "--help"}` (flag + value + `--help`, no
         positional args),
         `[]string{"https://teams.microsoft.com/meet/synthetic", "--help"}`
         (valid Teams meeting URL), and
         `[]string{"generate-config", "./custom-dir", "--help"}` (subcommand
         with a non-URL positional argument exercising the `cmd == root` guard):
         assert output does **not** contain
         `is not a subcommand or a valid Teams meeting URL`.
  2. **Green phase (`internal/cli/root.go`, `join.go`, `auth.go`, `schedule.go`, `update.go`, `generate_config.go`)**:
     * In `internal/cli/root.go`:
       - Implement `installRootHelpHandler(root *cobra.Command)`:
         * Call `root.InitDefaultHelpCmd()` and wrap the `help` subcommand's
           `Run` function so when `cmd, rest, err := root.Find(args)` resolves
           to `cmd == root && err == nil && len(rest) > 0`, it calls
           `root.InitDefaultHelpFlag()` and `root.HelpFunc()(root, rest)`
           instead of `root.Help()` (which otherwise passes `[]string{}`).
         * Wrap `defaultHelp := root.HelpFunc()` via `root.SetHelpFunc`: when
           `cmd == root`, extract positional arguments (`posArgs := cmd.Flags().Args()`
           when `cmd.Flags().Parsed()` is true, or non-flag tokens from `args`
           when unparsed). When `len(posArgs) > 0 && teams.ValidateMeetingURL(posArgs[0]) != nil`,
           format `redacted := teams.RedactArgs([]string{posArgs[0]})` and write
           the `Note: %q is not a subcommand or a valid Teams meeting URL...`
           banner to `cmd.OutOrStdout()`:
           `_, _ = fmt.Fprintf(cmd.OutOrStdout(), ...)` (with an explicit
           Principle I comment on the informational help output write) before
           invoking `defaultHelp(cmd, args)`.
       - Expand `root.Long` and move examples into `root.Example` (with 2-space
         indentation and no leading `Examples:` label).
     * In `internal/cli/join.go`:
       - Populate `Long` and `Example` on `newJoinCommand` and expand `Long` +
         add `Example` on `newCaptionsCommand`.
     * In `internal/cli/auth.go`:
       - Expand `Long` and add `Example` on `newAuthCommand`.
     * In `internal/cli/schedule.go`:
       - Expand `Long` (including the explicit `11ED4180` 3-tier precedence
         section and `ephemeral-live` protocol normalization notes) and add
         `Example` on `newScheduleCommand`.
     * In `internal/cli/update.go`:
       - Expand `Long` and add `Example` on `newUpdateCommand`.
     * In `internal/cli/generate_config.go`:
       - Expand `Long` and move examples into `Example` on
         `newGenerateConfigCommand`.
* **Files (7 files within `internal/cli`, single `cli` domain — ~45m human-equivalent effort, 1 new helper function `installRootHelpHandler` + declarative `Long`/`Example` strings + 2 table-driven unit tests):**
  `internal/cli/root.go`, `internal/cli/join.go`, `internal/cli/auth.go`,
  `internal/cli/schedule.go`, `internal/cli/update.go`,
  `internal/cli/generate_config.go`, `internal/cli/help_test.go`.
* **Verify:** `go vet ./internal/cli/...`, `golangci-lint run`,
  `gofmt -l internal/cli`, `go test -race ./internal/cli/...`, and
  `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`.

### U2 (`025.002-T`) — Document `teams-cli schedule` per-meeting vs global flag precedence and subcommand `--help` usage in `docs/USER_GUIDE.md` and `README.md`

* **Domain:** docs (Markdown). **Posture:** migration-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `docs/USER_GUIDE.md`:
     * In `## Commands` (around line 222): Add a clear note and examples showing
       how to get detailed help for any subcommand with
       `teams-cli <subcommand> --help` or `teams-cli help <subcommand>` (for
       example `teams-cli join --help`, `teams-cli schedule --help`), warning
       against passing the literal placeholder `[command]`.
     * In `### Built-in daemon scheduling (teams-cli schedule)` (around line
       474): Add a dedicated paragraph / callout documenting the 3-tier
       configuration and flag precedence for `teams-cli schedule`:
       1. Top-level settings in `config.yaml` establish the baseline.
       2. Global command-line flags passed to `teams-cli schedule` (`--mode`,
          `--duration`, `--idle-timeout`, `--ai-summary-delay`,
          `--no-ai-summary`, `--lobby-timeout`, `--audio` / `--recap-audio`,
          `--browser-path`, `--profile-dir`, `--account`, `--headless`,
          `--headed`) are applied to that base configuration first
          (`applyScheduleFlags`), including base-level `ephemeral-live`
          normalization (which overwrites `ai_summary_delay` on the base config
          to `0s` and sets `capture_audio = true` unless audio was explicitly
          configured when the base mode is `ephemeral-live`).
       3. When each scheduled meeting fires, its per-meeting settings under
          `meetings:` (`mode`/`capture_mode`, `duration`, `idle_timeout`,
          `ai_summary_delay`, `lobby_timeout`, `category`,
          `capture_audio`/`audio`, `output_dir`) are applied on top of the
          flag-adjusted base config (`buildScheduleEntryConfig`), followed by
          entry-level `ephemeral-live` protocol normalization (which forces
          `ai_summary_delay` to `0s` when the effective entry mode is
          `ephemeral-live` and defaults `capture_audio` to `true` unless audio
          was explicitly set to `false` via `--audio=false` or top-level
          `capture_audio: false`, in which case setting `capture_audio: true` on
          the meeting entry overrides it). Because `applyScheduleFlags` already
          mutates the base config when the base mode is `ephemeral-live`,
          changing an individual meeting entry's `mode` (for example to
          `live-with-summary`) does not restore an earlier `ai_summary_delay` or
          `capture_audio` value from before base normalization — set explicit
          per-meeting `ai_summary_delay` and `capture_audio` values on that
          entry to override the normalized base. Therefore, per-meeting settings
          in `config.yaml` override global `schedule` CLI flags (such as
          `--audio=false` or `--no-ai-summary`) for that specific meeting, while
          daemon-wide browser/profile flags (`--browser-path`, `--profile-dir`,
          `--account`, `--headless`, `--headed`) apply to every scheduled
          meeting.
     * In `## Configuration` (around line 755): Qualify the sentence
       `Command-line flags override every config value (see Output target precedence).`
       so it accurately states that for single-capture commands (`root`, `join`,
       `captions`, `auth`) command-line flags override every top-level config
       value, whereas in `teams-cli schedule` global CLI flags override
       top-level `config.yaml` defaults first and per-meeting `meetings:`
       settings (`mode`/`capture_mode`, `duration`, `idle_timeout`,
       `ai_summary_delay`, `lobby_timeout`, `category`, `capture_audio`/`audio`,
       `output_dir`) then override those global CLI flags for that meeting (with
       a cross-link to
       `[Built-in daemon scheduling (teams-cli schedule)](#built-in-daemon-scheduling-teams-cli-schedule)`).
  2. In `README.md`:
     * In `## Commands`: Show how to inspect detailed per-command help and
       examples using `teams-cli <subcommand> --help` (for example
       `teams-cli join --help`, `teams-cli schedule --help`).
     * In `## Configuration`: Note that command-line flags override top-level
       `config.yaml` settings, and that in `teams-cli schedule` per-meeting
       `meetings:` settings override global `schedule` flags for that entry
       (linking to `docs/USER_GUIDE.md#configuration` and
       `docs/USER_GUIDE.md#built-in-daemon-scheduling-teams-cli-schedule`).
* **Files (2 files, single `docs` domain):** `docs/USER_GUIDE.md`, `README.md`.
* **Verify:** `go test -race ./...` (including
  `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and
  `TestExampleConfigMatchesTemplateYAML`).
* **Depends on:** U1 (`025.001-T`).

## Dependency Graph

```text
U1 (025.001-T: internal/cli/*.go help overhaul + installRootHelpHandler + help_test.go)
  └──> U2 (025.002-T: docs/USER_GUIDE.md + README.md schedule precedence & subcommand help docs)
```

## Decisions and Rationale

* **Wrap `root.HelpFunc()` and `help` subcommand via `installRootHelpHandler` using `cmd.Flags().Args()` and `teams.ValidateMeetingURL(posArgs[0]) != nil`**:
  Because Cobra's `Command.execute` runs `c.ParseFlags(a)` and then passes the
  raw unstripped slice `a` (including `"--help"`) to `HelpFunc(cmd, args)`,
  inspecting `cmd.Flags().Args()` when `cmd.Flags().Parsed()` is true (and
  forwarding `rest` from `root.Find(args)` in the `help` subcommand) cleanly
  extracts the user's non-flag positional token across both
  `teams-cli [command] --help` and `teams-cli help [command]`. Checking
  `cmd == root && len(posArgs) > 0 && teams.ValidateMeetingURL(posArgs[0]) != nil`
  and formatting `teams.RedactArgs([]string{posArgs[0]})` distinguishes
  placeholders (`[command]`, `<command>`) or unknown subcommands from a valid
  Teams meeting URL without ever echoing credentials or passcodes.
* **Use Cobra's native `Long` and `Example` fields on all 7 commands**:
  Cobra's default help template renders `Long` followed by `Usage:` and then
  `Examples:` when `Example` is non-empty, ensuring consistent formatting for
  both `teams-cli <subcommand> --help` and `teams-cli help <subcommand>`.
* **Document the existing 3-tier `schedule` precedence and `ephemeral-live` normalization rather than altering `buildScheduleEntryConfig`**:
  In `teams-cli schedule`, global CLI flags serve as daemon-wide defaults over
  top-level `config.yaml`, while per-meeting `meetings:` fields customize
  individual meetings in the roster. Documenting this 3-tier precedence and the
  `ephemeral-live` protocol normalization in `schedule --help`,
  `docs/USER_GUIDE.md`, and `README.md` resolves `11ED4180` without breaking
  existing multi-meeting configurations or tests.

## Risks and Caveats

* **Synthetic URLs only in help text, tests, and documentation**:
  `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` in
  `internal/config/gitignore_test.go` scans all git-tracked files for live Teams
  passcodes (`?p=` / `&p=`). Every example URL in `Long`, `Example`,
  `help_test.go`, `README.md`, and `docs/USER_GUIDE.md` must use synthetic URLs
  (`https://teams.microsoft.com/meet/synthetic...` or
  `https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic...`).
* **Cobra `cmd == root` check and `cmd.Flags().Args()` extraction inside `SetHelpFunc`**:
  Because `SetHelpFunc` on a parent command is inherited by child commands in
  Cobra and receives raw unstripped `args`, `installRootHelpHandler` MUST guard
  the guidance banner with `cmd == root` and inspect parsed positional
  arguments (`cmd.Flags().Args()`) so neither bare `teams-cli --help` nor valid
  positional arguments passed to subcommands (such as
  `teams-cli generate-config ./out --help`) ever trigger the root
  unknown-command note.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Standard library + Cobra only; explicit Principle I comment on `_, _ = fmt.Fprintf(cmd.OutOrStdout(), ...)` in `installRootHelpHandler`; clean `go vet`, `gofmt`, and `golangci-lint` |
| II: Test first | `internal/cli/help_test.go` (`025.001-T`) written in Red phase before updating command definitions, testing all 7 commands via `--help` and `help <subcommand>` plus placeholder/unknown-command interception, flag stripping, subcommand positional-arg pass-through, and rejected-URL redaction |
| III, IV: Secrets & containment | Reuses `teams.ValidateMeetingURL` and `teams.RedactArgs([]string{posArgs[0]})` so rejected URLs never echo credentials or passcodes; all example URLs are synthetic and verified by `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |
| V: Structured observability | Clear user-facing guidance note written to `cmd.OutOrStdout()` when an invalid subcommand or `[command]` placeholder is passed to `teams-cli <arg> --help` or `teams-cli help <arg>` |
| VI: Single responsibility | Two width-isolated tasks (`025.001-T` in `cli`, `025.002-T` in `docs`). *Heuristic exception justification (`025.001-T`)*: `025.001-T` touches 7 files in `internal/cli` (~45m human-equivalent effort) because each subcommand lives in its own file (`root.go`, `join.go`, `auth.go`, `schedule.go`, `update.go`, `generate_config.go`) and `TestAllSubcommandsProvideComprehensiveHelpAndExamples` in `help_test.go` tests all 7 commands together; splitting declarative `Long`/`Example` string updates across multiple tasks was rejected as artificial fragmentation that would prevent atomic test-first verification of the command tree |
| VII, VIII: Strict safety | CLI help contract refinement and documentation qualification classified in `## Plan Hardening` below under `Freeze-scope mode` (`010-S` and `3EB89E2D` untouched) |
| IX, X, XI: Persistence & merge history | Deliberation `014-DL` and this plan are recorded in `docs/`, and feature `025-F`, tasks `025.001-T`–`025.002-T`, and shipment `025-S` are materialized in `.backlogit/queue/` during `harvest` immediately following `plan-review` with full stash traceability (`A52B6644`, `11ED4180`); PR merge must use a merge commit (`gh pr merge --merge`, `P-009` / Principle XI) |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present (`ActionRisk: low`)**:
  expands the CLI `--help` output contract across all 7 commands, adds a root
  `--help` / `help <arg>` guidance banner when a non-URL positional argument is
  passed on `root`, and clarifies the `teams-cli schedule` flag vs per-meeting
  config precedence contract in CLI help and user documentation.
* Security, auth, permission, or compliance-sensitive behavior — **absent**:
  no authentication, browser, or filesystem permission logic is modified.
* Migration, backfill, destructive data/config action, or irreversible step —
  **absent**: additive help text and documentation updates only.
* External integration, operator checkpoint, or external dependency — **absent**:
  100% local CLI and documentation changes.
* High runtime, rollout, or rollback risk — **absent**: runtime capture,
  scheduling, config loading, and self-update execution paths are untouched.

Requires plan hardening: yes

## Plan Hardening

Hardening is applied under **`Freeze-scope mode`** (explicitly excluding queued
shipment `010-S` and stash entry `3EB89E2D`) because U1 refines the user-facing
CLI `--help` contract on the root command and all six subcommands, and U2
updates the documented configuration-precedence contract for
`teams-cli schedule` (`11ED4180`).

* **Instructions and learnings consulted**:
  * `.github/instructions/go.instructions.md`
  * `.github/instructions/markdown.instructions.md`
  * `.github/instructions/writing-style.instructions.md`
  * `.github/instructions/architecture-doc.instructions.md`
  * `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`

### Protected Invariants

1. **Subcommand Inheritance Guard (`cmd == root`) and Parsed Positional Argument Extraction (`cmd.Flags().Args()`) in `installRootHelpHandler`**:
   In Cobra, `root.SetHelpFunc(fn)` is inherited by child subcommands and
   receives the raw, unstripped argument slice `a` (which includes `"--help"`,
   `"-h"`, and any flags). `installRootHelpHandler` MUST check `cmd == root`,
   extract parsed positional arguments via `cmd.Flags().Args()` when
   `cmd.Flags().Parsed()` is true (or non-flag tokens from `args` when invoked
   via the wrapped `help` subcommand), and format
   `teams.RedactArgs([]string{posArgs[0]})` only when `len(posArgs) > 0 && teams.ValidateMeetingURL(posArgs[0]) != nil`.
   Invoking `teams-cli --help`, `teams-cli -f config.yaml --help`, or
   `teams-cli generate-config ./custom-dir --help` MUST NEVER emit the root
   unknown-command notice.
2. **Valid Meeting URL Pass-Through and Rejected URL Redaction on Root `--help`**:
   When `posArgs[0]` is a valid Teams meeting URL
   (`teams.ValidateMeetingURL(posArgs[0]) == nil`), `installRootHelpHandler`
   MUST NOT print the unknown-subcommand notice. When `posArgs[0]` is a rejected
   URL (for example `http://` on a remote host or a URL with embedded userinfo),
   `installRootHelpHandler` MUST pass `teams.RedactArgs([]string{posArgs[0]})`
   to `%q` so credentials and query passcodes are never echoed.
3. **100% Parity Between `buildScheduleEntryConfig` and Documented Precedence (`11ED4180`)**:
   Both `newScheduleCommand` (`internal/cli/schedule.go`) and `docs/USER_GUIDE.md`
   / `README.md` MUST list the exact per-meeting fields that override global
   `schedule` CLI flags in `buildScheduleEntryConfig`: `mode` (alias
   `capture_mode`), `duration`, `idle_timeout`, `ai_summary_delay`,
   `lobby_timeout`, `category`, `capture_audio` (alias `audio`), and
   `output_dir` (plus `name` setting `MeetingTitle`), followed by the
   `ephemeral-live` protocol normalization (forcing `ai_summary_delay` to `0s`
   and defaulting `capture_audio` to `true` unless audio was explicitly set on
   the base config/flags, which a per-meeting `capture_audio: true` overrides),
   while noting that browser/profile flags (`--browser-path`, `--profile-dir`,
   `--account`, `--headless`, `--headed`) apply across the daemon.
4. **Zero Live Passcodes or Tokens in Examples (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`)**:
   All example URLs in `internal/cli/*.go`, `internal/cli/help_test.go`,
   `docs/USER_GUIDE.md`, and `README.md` MUST use synthetic URLs.

### Strict-Safety Classification (`ProposedAction` / `ActionRisk` / `ActionResult`)

| ProposedAction | ActionRisk | Approval & Containment | Expected ActionResult |
|---|---|---|---|
| Wrap `root.HelpFunc()` and `help` subcommand via `installRootHelpHandler` in `internal/cli/root.go` and populate `Long` + `Example` across all 7 CLI commands (`025.001-T`) | `low` | Guarded by `cmd == root`, `cmd.Flags().Args()`, `teams.ValidateMeetingURL(posArgs[0]) != nil`, and `teams.RedactArgs`; does not alter `RunE` execution paths; verified by table-driven tests in `internal/cli/help_test.go` | Every command's `--help` and `help <cmd>` outputs comprehensive `Long` and `Examples:` sections, and `teams-cli [command] --help` / `teams-cli help [command]` prints the redacted guidance banner |
| Qualify `teams-cli schedule` global CLI flag vs per-meeting `meetings:` config precedence and subcommand `--help` examples in `docs/USER_GUIDE.md` and `README.md` (`025.002-T`) | `low` | Pure documentation update matching `applyScheduleFlags` + `buildScheduleEntryConfig` in `internal/cli/schedule.go`; verified by `go test -race ./...` | `docs/USER_GUIDE.md`, `README.md`, and `teams-cli schedule --help` state the identical 3-tier precedence and protocol-normalization rule |

### Runtime Verification and Operational Closure

* **Changed runtime surface**: CLI `--help` and `help <subcommand>` output on
  `teams-cli` (`root`, `join`, `captions`, `auth`, `schedule`, `update`,
  `generate-config`).
* **Runtime verification**:
  * Static analysis and lint: `go vet ./...`, `golangci-lint run`, and
    `gofmt -l .`.
  * Unit test suite: `go test -race ./...` (including
    `TestAllSubcommandsProvideComprehensiveHelpAndExamples`,
    `TestRootHelpPlaceholderAndUnknownCommandGuidance`, and
    `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`).
  * Compiled binary smoke check during Ship: build `bin/teams-cli.exe` and
    verify `teams-cli.exe [command] --help`, `teams-cli.exe join --help`, and
    `teams-cli.exe schedule --help`.
* **Operational closure expectations**:
  * **Healthy signal**: `go build ./...`, `go vet ./...`, `golangci-lint run`,
    `gofmt -l .`, and `go test -race ./...` pass locally and on CI
    (`ubuntu-latest` and `windows-latest`).
  * **Failure signal**: Any subcommand `--help` missing `Examples:` or printing
    the root unknown-command banner on valid subcommand invocations.
  * **Rollback trigger**: Regressions in CLI flag parsing or `--help` rendering.
  * **Rollback procedure**: Revert the `025-S` merge commit on `main` via a
    reviewed PR (`git revert -m 1 <merge-sha>`); changes are self-contained in
    `internal/cli/` help strings/handler and Markdown documentation.
  * **Post-merge context compaction (`P-020`)**: Execute `compact-context` and
    `compound-refresh` during Ship post-merge closure and record
    `compaction_status: done` in the `025-S` closure artifact.
  * **Owner**: Repository maintainer (`softwaresalt`).
  * **Validation window**: Next CLI invocation / CI run on PR merge.

### Review-Gate Capability Risks

* `plan-review` MUST emit literal `dispatch_mode:` and `decision:` markers in
  the appended `## Plan Review` section before `harvest` can proceed.
* Cross-model Anchor Reviewer (`Architecture Strategist` routed to
  `model_routing.anchor_review`: `gpt-6.1-sol`, `reasoning_effort: high`) plus
  always-on personas (`Constitution Reviewer`, `Go Reviewer`,
  `Scope Boundary Auditor`, `Learnings Researcher`) and `rubber-duck` critique
  are dispatched in parallel.

## Plan Review

* `dispatch_mode: multi-agent`
* `decision: PASS`
* **Gate decision and rationale**: **PASS** (after in-place remediation of all
  P1, P2, and P3 findings surfaced by the multi-agent review panel).
* **Plan hardening requirement**: Required (`Requires plan hardening: yes`) and
  satisfied by `## Plan Hardening` above.

### Persona Coverage

| Persona | Mode / Model | Findings Summary |
|---|---|---|
| Constitution Reviewer | Subagent (`gemini-4-argon`) | 0 P0, 0 P1, 4 P2, 4 P3 — all remediated in-place (`teams.RedactArgs` in `installRootHelpHandler`, subcommand positional-arg negative test case, `golangci-lint run` + secret-hygiene test in U1 `Verify`, `_, _ = fmt.Fprintf` Principle I comment, Principle VI 7-file justification, `Freeze-scope mode` + `P-020` compaction note) |
| Go Reviewer | Subagent (`gemini-4-argon`) | 0 P0, 1 P1, 2 P2, 1 P3 — all remediated in-place (use `cmd.Flags().Args()` for parsed positional args in `installRootHelpHandler` so `--help` is not treated as `args[0]`, wrap `help` subcommand via `root.InitDefaultHelpCmd()`, test subcommand positional args and flag-ordering combinations, format `Example` with 2-space indent and no duplicate `Examples:` header) |
| Scope Boundary Auditor | Subagent (`gemini-4-argon`) | 0 P0, 0 P1, 0 P2, 0 P3 — confirmed strict alignment to `A52B6644` and `11ED4180`, width isolation (`cli` vs `docs`), and zero overlap with `010-S` or `3EB89E2D` |
| Learnings Researcher | Subagent (`claude-haiku-5.5`) | 0 P0, 0 P1, 0 P2, 0 P3 — searched 9 files in `docs/compound/`; no prior conflicts |
| Architecture Strategist (Anchor) | Cross-model Anchor subagent (`gpt-6.1-sol`, `reasoning_effort: high`) | 0 P0, 1 P1, 1 P2 — all remediated in-place (extract parsed positional args via `cmd.Flags().Args()` and define `help <arg>` topic forwarding; document `ephemeral-live` protocol normalization alongside the 3-tier field-overlay precedence in `schedule --help`, `docs/USER_GUIDE.md`, and `README.md`) |
| Correctness Reviewer (`rubber-duck`) | Subagent (`gemini-4-argon`) | 0 P0, 1 P1, 2 P2, 1 P3 — all remediated in-place (`cmd.Flags().Args()` extraction, `help <arg>` forwarding, `ephemeral-live` audio/`ai_summary_delay` normalization details, and inline enumeration of all 8 output precedence levels in U1) |

### Findings and Remediations Applied

1. **[P1 — Remediated] Cobra `SetHelpFunc` raw `args` slice vs parsed positional arguments (`Go Reviewer`, `Architecture Strategist`, `Correctness Reviewer`)**:
   * *Issue*: Cobra's `Command.execute` runs `c.ParseFlags(a)` and then passes
     the raw unstripped argument slice `a` (which includes `"--help"`, `"-h"`,
     and any flags) to `c.HelpFunc()(c, a)`. Inspecting `args[0]` directly would
     treat `"--help"` as a positional argument on `teams-cli --help`.
     Additionally, `teams-cli help [command]` invokes `root.Help()` with an
     empty slice unless the `help` subcommand forwards unresolved root args.
   * *Remediation*: Updated R1, U1, Decisions, Risks, and Protected Invariant 1
     so `installRootHelpHandler` extracts `cmd.Flags().Args()` when
     `cmd.Flags().Parsed()` is true (falling back to non-flag tokens in `args`)
     and wires `root.InitDefaultHelpCmd()` so `teams-cli help [command]` and
     `teams-cli [command] --help` both emit the guidance banner while bare
     `teams-cli --help` and `teams-cli -f config.yaml --help` do not.
2. **[P2 — Remediated] Redact rejected URLs via `teams.RedactArgs` in `installRootHelpHandler` (`Constitution Reviewer`)**:
   * *Issue*: Formatting raw `posArgs[0]` with `%q` when
     `teams.ValidateMeetingURL(posArgs[0]) != nil` could echo userinfo
     credentials or query passcodes from a rejected URL.
   * *Remediation*: Specified `teams.RedactArgs([]string{posArgs[0]})` for the
     `%q` argument in `installRootHelpHandler` and added a rejected-URL
     redaction test case in `TestRootHelpPlaceholderAndUnknownCommandGuidance`.
3. **[P2 — Remediated] Document `ephemeral-live` protocol normalization alongside the 3-tier `schedule` field overlay (`Architecture Strategist`, `Correctness Reviewer`)**:
   * *Issue*: In `buildScheduleEntryConfig`, after per-meeting settings override
     global CLI flags, `ephemeral-live` mode forces `ai_summary_delay` to `0s`
     and defaults `capture_audio` to `true` only when neither the entry nor the
     base config/flags explicitly set audio (whereas an explicit
     `capture_audio: true` on the entry overrides `--audio=false`).
   * *Remediation*: Updated U1, U2, and Protected Invariant 3 so
     `schedule --help`, `docs/USER_GUIDE.md`, and `README.md` state both the
     3-tier field-overlay precedence and the `ephemeral-live` normalization rule.
4. **[P2/P3 — Remediated] Test coverage, linter gate, and Constitution Check completeness (`Constitution Reviewer`, `Go Reviewer`, `Correctness Reviewer`)**:
   * *Remediation*: Added `[]string{"generate-config", "./custom-dir", "--help"}`
     (testing the `cmd == root` guard), flag-ordering cases, `golangci-lint run`,
     `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`, inline enumeration of
     all 8 output target precedence levels, Principle VI 7-file justification,
     and `Freeze-scope mode` + `P-020` compaction notes.

