---
title: Comprehensive CLI subcommand help, root help placeholder guidance, and schedule flag precedence decision
description: Deliberation on grouping stash entries A52B6644 and 11ED4180 into feature 025-F and shipment 025-S
topic: CLI Subcommand Help Overhaul, Root Help Placeholder Guidance, and Schedule Precedence Documentation (A52B6644 + 11ED4180)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 014-DL
feature: 025-F
shipment: 025-S
source_stash:
  - A52B6644
  - 11ED4180
linked_artifacts:
  - docs/plans/2026-11-14-cli-subcommand-help-and-schedule-precedence-plan.md
  - .backlogit/queue/014-DL.md
  - .backlogit/queue/025-F.md
  - .backlogit/queue/025-S.md
date: 2026-11-14
status: accepted
tags:
  - cli
  - help
  - schedule
  - docs
  - ux
---

## Problem Frame

This deliberation covers the contextual grouping of two active stash entries into
a single cohesive release unit under covering feature **`025-F`** and shipment
**`025-S`** (sequenced after `024-S`):

1. **`A52B6644`** (`bug`, `high`):
   > Fix CLI subcommand `--help` output so every command (`root`, `join`,
   > `captions`, `auth`, `schedule`, `update`, `generate-config`) provides
   > comprehensive `Long` descriptions, detailed behavioral/flag/precedence
   > notes, and concrete `Examples` when invoked with `teams-cli <command> --help`
   > or `teams-cli help <command>`, and detect when a user passes a literal
   > placeholder like `[command]` or an unknown subcommand name to
   > `teams-cli <arg> --help` so it guides them to valid subcommand names
   > instead of silently printing generic root help.
2. **`11ED4180`** (`task`, `low`, `DEFERRED SCOPE EXPANSION` from `024-S`):
   > DEFERRED SCOPE EXPANSION. Document that `teams-cli schedule` applies global
   > CLI flags first and then per-meeting settings (`buildScheduleEntryConfig`,
   > `internal/cli/schedule.go`), so per-meeting `capture_audio`, `mode`, and
   > `ai_summary_delay` override global flags such as `--audio=false` or
   > `--no-ai-summary`, and qualify the guide statement that command-line flags
   > override every config value. Out of scope (P-021 C1): the `024-S`
   > authorized change only restructures existing `README` content into
   > `USER_GUIDE.md` and adds `generate-config` docs; the sentence predates the
   > shipment and corrects `schedule`-command precedence, a different contract
   > surface that needs code-verified new documentation. Source refs: task
   > `024.003-T`, feature `024-F`, shipment `024-S`, PR `#61`, review thread
   > `PRRT_kwDOTStvas63i5Fq` (comment `4575633022`).

### Why These Entries Belong Together

* **Unified CLI help and documentation contract surface**: Both `A52B6644` and
  `11ED4180` address how operators discover and understand `teams-cli`
  subcommand behavior, flag interactions, and configuration override rules.
* **Shared `schedule` precedence contract**: Expanding `newScheduleCommand`'s
  `Long` and `Example` text in `internal/cli/schedule.go` (`A52B6644`) requires
  stating the exact precedence between global `teams-cli schedule` flags and
  per-meeting `meetings:` entries in `config.yaml` (`11ED4180`). Shipping both
  entries together ensures that `teams-cli schedule --help`, `docs/USER_GUIDE.md`,
  and `README.md` state the identical, code-verified precedence rule in one pull
  request.

### Explicit Scope Boundary (Excluded Items)

* Queued shipment **`010-S`** (`010-F`, `001-DL`, `010.001-T`..`010.009-T`) and
  stash entry **`3EB89E2D`** (live-tenant validation follow-up from `022-S`)
  remain **explicitly out of scope** and untouched because they require a live
  Microsoft 365 tenant and interactive operator attendance.

## P-021 C5 and C6 Triage Record

* **Precedence classification (`11ED4180`)**: Stash entry `11ED4180` carries the
  literal `DEFERRED SCOPE EXPANSION` marker as its opening token. Under P-021 C6
  and the Stage Step 1 precedence rule, this marker forces routing through the
  `deliberate` skill before planning regardless of the entry's `kind: task`,
  provisional `low` priority, or capture-time `Requires deliberation: no` note.
  Stage re-prioritizes `11ED4180` to ship alongside `high`-priority `A52B6644`
  in `025-S`.
* **Source reference provenance (`11ED4180`)**:
  * Originating task ID: `024.003-T`
  * Originating feature ID: `024-F`
  * Originating shipment ID: `024-S`
  * Originating pull request: `PR #61`
  * Originating review thread ID: `PRRT_kwDOTStvas63i5Fq` (comment `4575633022`)
* **Obligation (A) Unconditional duplicate detection**: Executed across all
  active stash entries (`A52B6644`, `11ED4180`, `3EB89E2D`), `.backlogit/queue/`,
  and `.backlogit/archive/`. No duplicate entry describes either the CLI
  `--help` overhaul (`A52B6644`) or the `teams-cli schedule` per-meeting config
  vs global CLI flag precedence (`11ED4180`).
  * Recorded outcome: **CLEAN DUPLICATE SCAN** for both `11ED4180` and
    `A52B6644`.
* **Obligation (B) Late-identifier reconciliation**: Checked all five source-ref
  fields on `11ED4180` (`task 024.003-T`, `feature 024-F`, `shipment 024-S`,
  `PR #61`, `review thread PRRT_kwDOTStvas63i5Fq`). None of the fields is `N/A`;
  all five concrete identifiers were populated at Ship's C2 capture time and
  match `docs/closure/2026-11-14-024-S-review-and-closure.md`.
  * Recorded outcome: **All source-ref identifiers present at capture;
    reconciliation is an idempotent no-op with all five identifiers verified and
    carried forward.**

## Research Findings

### Root Cause 1: Why `teams-cli [command] --help` and `teams-cli <unknown> --help` Printed Generic Root Help

In `internal/cli/root.go`, `newRootCommand` configures the root command to
support direct capture via up to two positional arguments:

```go
root := &cobra.Command{
    Use:  "teams-cli [meeting-url] [output-markdown-path]",
    Args: cobra.MaximumNArgs(2),
    // ...
}
```

When Cobra renders root `--help`, its default footer prints:

```text
Use "teams-cli [command] --help" for more information about a command.
```

Because `root` has both subcommands (`auth`, `captions`, `generate-config`,
`join`, `schedule`, `update`) and `Args: cobra.MaximumNArgs(2)`, Cobra's
`Command.Find(args)` treats any first token that does not match a known
subcommand name (such as the literal string `"[command]"`, `"<command>"`, or a
mistyped subcommand like `"sched"`) as a positional argument (`[meeting-url]`)
for `root` rather than an unknown subcommand error. When `--help` or `-h` is
also present on the command line:

* Cobra resolves the target command to `root` with `args = ["[command]"]`.
* Cobra invokes `root.HelpFunc()(root, args)` and prints the generic root help
  output without any diagnostic or hint that `"[command]"` was not a subcommand.
* A user who copies `teams-cli [command] --help` literally or mistypes a
  subcommand name sees the exact same root help screen and concludes that
  per-command `--help` does not work.

### Root Cause 2: Subcommands Missing Detailed `Long` Descriptions and `Example` Fields

Inspecting every command constructor in `internal/cli/`:

* **`newJoinCommand` (`internal/cli/join.go`)**: Sets `Use` and `Short` only;
  it has **no `Long` field and no `Example` field at all**. Running
  `teams-cli join --help` or `teams-cli help join` prints only the 1-line
  `Short` summary (`Join a Teams meeting and capture its live captions to a Markdown file`)
  followed by the flag list, which is identical to `root`'s capture flag list.
* **`newCaptionsCommand` (`internal/cli/join.go`)**: Has a single-sentence
  `Long` field and **no `Example` field**. It does not explain how `captions`
  differs from `join` (specifically that `[meeting-url]` is optional on
  `captions` and defaults to opening `https://teams.microsoft.com/v2/` for
  interactive call/recap selection), nor does it document the 4 capture
  protocols, output target precedence, filename templates, or `--queue` mode.
* **`newAuthCommand` (`internal/cli/auth.go`)**: Has a 2-sentence `Long` field
  and **no `Example` field**. It does not explain the default `--profile-dir`
  location (`%LOCALAPPDATA%\teams-cli\profile` on Windows,
  `~/.config/teams-cli/profile` on Linux), automatic Microsoft Entra account
  picker selection (`--account` / `default_account`), or single-profile
  exclusivity.
* **`newScheduleCommand` (`internal/cli/schedule.go`)**: Has a 2-sentence `Long`
  field and **no `Example` field**. It omits the 5-field `cron` vs one-shot `at`
  (`RFC3339` or local `YYYY-MM-DD HH:MM`) trigger rules, the startup sweep of
  deferred `--append-ai-summary` entries in `queue_file`, the timing of shared
  profile `busy` gate release (released as soon as the live call ends, before
  `--ai-summary-delay` elapses), and the precedence order between global
  `schedule` flags and per-meeting `meetings:` settings in `config.yaml`.
* **`newUpdateCommand` (`internal/cli/update.go`)**: Has a 2-sentence `Long`
  field and **no `Example` field**. It omits the 6 published platform targets
  (`teams-cli-<goos>-<goarch>[.exe]`), `<exe>.lock` / `<exe>.lock.ctl`
  inter-process locking, `--check` / `--version` / `--force` semantics, and
  manual lock recovery steps.
* **`newGenerateConfigCommand` (`internal/cli/generate_config.go`)** and
  **`newRootCommand` (`internal/cli/root.go`)**: Embed a short `Examples:` block
  inline inside `Long` rather than using Cobra's dedicated `Example` field, and
  `root` does not summarize the 4 capture protocols, config discovery order, or
  how to request per-subcommand help (`teams-cli <subcommand> --help`).

### Root Cause 3: `teams-cli schedule` Flag vs Per-Meeting Config Precedence (`11ED4180`)

In `internal/cli/schedule.go`:

1. `newScheduleCommand` loads `cfg` via `loadConfigFrom(d, *configPath)` and
   immediately applies global CLI flags to `cfg` via
   `applyScheduleFlags(cmd, &cfg, flags)` (lines 250-254).
2. When a scheduled meeting fires, `scheduleJobWithSummaryWG` calls
   `entryCfg := buildScheduleEntryConfig(cfg, e)` (line 201).
3. `buildScheduleEntryConfig(base config.Config, e scheduler.Entry)` starts from
   `entryCfg := base` (which already includes the global CLI flags applied by
   `applyScheduleFlags`) and then overwrites `entryCfg` with every non-empty or
   non-nil per-meeting field from `e` (populated from `cfg.Meetings[i]` by
   `buildSchedulerEntryFromConfig`):
   * `e.Name` -> `entryCfg.MeetingTitle`
   * `e.Category` -> `entryCfg.Category`
   * `e.OutputDir` -> `entryCfg.OutputDir`
   * `e.CaptureAudio != nil` (`capture_audio` or `audio` on the `meetings:`
     entry) -> `entryCfg.CaptureAudio = *e.CaptureAudio` and
     `entryCfg.ExplicitCaptureAudio = true`
   * `e.Mode != ""` (`mode` or `capture_mode` on the `meetings:` entry) ->
     `entryCfg.CaptureMode = strings.ToLower(strings.TrimSpace(e.Mode))`
   * `e.Duration > 0` -> `entryCfg.Duration = e.Duration.String()` (otherwise if
     `entryCfg.Duration == ""`, defaults to `1h`)
   * `e.IdleTimeout > 0` -> `entryCfg.IdleTimeout = e.IdleTimeout.String()`
     (otherwise if `entryCfg.IdleTimeout == ""`, defaults to `3m`)
   * `e.AISummaryDelay != nil` -> `entryCfg.AISummaryDelay = (*e.AISummaryDelay).String()`
   * `e.LobbyTimeout > 0` -> `entryCfg.LobbyTimeout = e.LobbyTimeout.String()`
   * Plus the `ephemeral-live` post-adjustment: if `entryCfg.CaptureMode == "ephemeral-live"`,
     `entryCfg.AISummaryDelay` is forced to `"0s"`, and if `e.CaptureAudio == nil && !entryCfg.ExplicitCaptureAudio`,
     `entryCfg.CaptureAudio` defaults to `true`.

Consequently, for `teams-cli schedule`, command-line flags (`--mode`,
`--duration`, `--idle-timeout`, `--ai-summary-delay`, `--no-ai-summary`,
`--lobby-timeout`, `--audio` / `--recap-audio`) override top-level `config.yaml`
defaults, **and per-meeting settings under `meetings:` in `config.yaml` override
both top-level config values and global `teams-cli schedule` CLI flags for that
specific meeting** (while browser/profile flags `--browser-path`,
`--profile-dir`, `--account`, `--headless`, and `--headed`, which have no
per-meeting keys in `MeetingSchedule`, apply to all scheduled meetings).
Meanwhile, `docs/USER_GUIDE.md` line 755 currently states without qualification:
`Command-line flags override every config value (see Output target precedence).`

## Options Evaluated

### Option A: Wrap `root.SetHelpFunc` and `help` Subcommand for Non-URL Positional Args on Root Help + Populate Structured `Long` and `Example` on All Commands + Qualify `schedule` Precedence in `schedule --help`, `docs/USER_GUIDE.md`, and `README.md` (Recommended)

* **Description**:
  1. In `internal/cli/root.go`, add `installRootHelpHandler(root *cobra.Command)`
     wrapping `root.HelpFunc()` via `root.SetHelpFunc(...)` and wiring Cobra's
     built-in `help` subcommand (`root.InitDefaultHelpCmd()`). Because Cobra's
     `Command.execute` passes the raw unstripped argument slice `a` (which still
     contains `"--help"`, `"-h"`, and any flags) to `HelpFunc(cmd, args)` after
     running `cmd.ParseFlags(a)`, `installRootHelpHandler` extracts the parsed
     positional arguments via `cmd.Flags().Args()` when `cmd.Flags().Parsed()` is
     true (falling back to non-flag arguments in `args` when invoked via
     `teams-cli help <arg>`). When `cmd == root`, `len(posArgs) > 0`, and
     `teams.ValidateMeetingURL(posArgs[0]) != nil` (for example
     `teams-cli [command] --help`, `teams-cli <command> --help`,
     `teams-cli unknown-subcmd --help`, or `teams-cli help [command]`), it writes
     an explicit guidance notice to `cmd.OutOrStdout()` using
     `teams.RedactArgs([]string{posArgs[0]})` (so rejected URLs with credentials
     or passcodes are redacted per Principle III while non-URL tokens print
     verbatim) before calling the default help function:
     `Note: %q is not a subcommand or a valid Teams meeting URL. Replace [command] with one of the subcommands below (for example: teams-cli join --help, teams-cli captions --help, teams-cli schedule --help, teams-cli auth --help, teams-cli generate-config --help, or teams-cli update --help).\n\n`
  2. Populate comprehensive, structured `Long` and `Example` fields on all seven
     commands (`root`, `join`, `captions`, `auth`, `schedule`, `update`,
     `generate-config`), including the exact `schedule` precedence rule in
     `newScheduleCommand`.
  3. Update `docs/USER_GUIDE.md` (`## Configuration` and
     `### Built-in daemon scheduling (teams-cli schedule)`) and `README.md` to
     document the 3-tier `schedule` precedence rule (top-level `config.yaml` <
     global `teams-cli schedule` CLI flags < per-meeting `meetings:` settings in
     `config.yaml`) and show concrete `teams-cli <subcommand> --help` examples.
* **Pros**:
  * Preserves `teams-cli <meeting-url> [output-markdown-path]` direct capture on
    `root` (`cobra.MaximumNArgs(2)`) with zero breaking changes.
  * Gives immediate, actionable guidance when a user runs
    `teams-cli [command] --help` or mistypes a subcommand name with `--help`.
  * Uses Cobra's native `Long` and `Example` fields consistently across every
    command so both `teams-cli <subcommand> --help` and
    `teams-cli help <subcommand>` render thorough documentation and copy-pasteable
    examples.
  * Aligns `internal/cli/schedule.go`, `docs/USER_GUIDE.md`, and `README.md` on
    the exact `buildScheduleEntryConfig` contract.
* **Cons**:
  * Slightly increases help text volume in `internal/cli/*.go` (mitigated by
    clear section headings and focused unit tests).
* **Effort estimate**: Low-Medium (2 focused tasks: `cli` and `docs`).
* **Fit**: 100% alignment with `A52B6644` and `11ED4180`.

### Option B: Remove Positional URL Capture from `root` (`cobra.NoArgs`) So Cobra Rejects Unknown Subcommands Directly

* **Description**: Change `root.Args` from `cobra.MaximumNArgs(2)` to
  `cobra.NoArgs` so `teams-cli <meeting-url>` is no longer valid on `root` and
  all captures must use `teams-cli join` or `teams-cli captions`.
* **Pros**:
  * Cobra's built-in unknown-command handling would trigger on
    `teams-cli unknown`.
* **Cons**:
  * Breaks backward compatibility with the primary root invocation
    `teams-cli [meeting-url] [output-markdown-path]` documented in `README.md`,
    `docs/USER_GUIDE.md`, and `internal/cli/queue.go` batch queue lines.
  * Even with `cobra.NoArgs`, Cobra's `--help` flag short-circuits before `Args`
    validation on `root` when `Find` falls back to `root`.
* **Effort estimate**: Medium (breaking change).
* **Fit**: Rejected — violates existing CLI contract.

### Option C: Change `buildScheduleEntryConfig` So Global `schedule` Flags Override Per-Meeting `meetings:` Config Settings

* **Description**: Invert `buildScheduleEntryConfig` in
  `internal/cli/schedule.go` so passing `--audio=false` or `--mode` on
  `teams-cli schedule` overrides explicit per-meeting `capture_audio` or `mode`
  fields in `config.yaml`.
* **Pros**:
  * Would make "CLI flags override every config value" literally true without
    qualification.
* **Cons**:
  * Defeats the purpose of per-meeting overrides in a multi-meeting daemon roster
    (where global `schedule` flags set daemon-wide defaults such as `--headless`
    or a baseline `--duration`, while individual `meetings:` entries customize
    `mode: ephemeral-live` or `capture_audio: true` for specific calls).
  * Breaks existing unit tests in `internal/cli/schedule_test.go` that codify
    per-meeting override behavior and contradicts the scope of `11ED4180` (which
    explicitly requests documenting the existing `buildScheduleEntryConfig`
    behavior).
* **Effort estimate**: Medium (breaking behavioral change).
* **Fit**: Rejected — contradicts `11ED4180` and breaks multi-meeting daemon
  ergonomics.

## Trade-off Comparison

| Criterion | Option A (`SetHelpFunc` + Full `Long`/`Example` + Document `schedule` Precedence) | Option B (Drop Root Positional Args) | Option C (Invert `schedule` Precedence in Code) |
|---|---|---|---|
| Backward compatibility | Preserved (`root` positional URLs and `schedule` per-meeting overrides intact) | Broken (`teams-cli <url>` fails) | Broken (per-meeting overrides clobbered by daemon flags) |
| Fixes `teams-cli [command] --help` trap | Yes (`installRootHelpHandler` checks `ValidateMeetingURL(args[0]) != nil`) | Partial | No |
| Comprehensive subcommand `--help` | Yes (`Long` + `Example` on all 7 commands) | Yes | Partial |
| Resolves `11ED4180` accurately | Yes (code-verified 3-tier precedence documented in CLI help and guide) | No | No (changes runtime contract instead) |
| Complexity & risk | Low | Medium-High | Medium-High |

## Decision

Adopt **Option A**, decomposed into two width-isolated tasks under feature
**`025-F`** and shipment **`025-S`** (sequenced after `024-S`):

1. **Unit 1 (`025.001-T` — `cli` domain, size `M`, complexity `low`, test-first)**:
   * **Root help interceptor (`installRootHelpHandler` in `internal/cli/root.go`)**:
     Wrap `root.HelpFunc()` via `root.SetHelpFunc(...)` and wire
     `root.InitDefaultHelpCmd()` so when `--help` or `help <arg>` is invoked on
     `root` (`cmd == root`) with parsed positional arguments `len(posArgs) > 0`
     and `teams.ValidateMeetingURL(posArgs[0]) != nil`, it writes a clear
     guidance note at the top of `cmd.OutOrStdout()` (formatting
     `teams.RedactArgs([]string{posArgs[0]})` via `%q`) before calling the
     underlying help function:
     `Note: %q is not a subcommand or a valid Teams meeting URL. Replace [command] with one of the subcommands below (for example: teams-cli join --help, teams-cli captions --help, teams-cli schedule --help, teams-cli auth --help, teams-cli generate-config --help, or teams-cli update --help).\n\n`
   * **Comprehensive `Long` and `Example` on every command**:
     * `root` (`internal/cli/root.go`): Summarize direct capture vs subcommands,
       the 4 capture protocols (`auto`, `ephemeral-live`, `live-with-summary`,
       `past-recap`), config file discovery order (`--config` / `-f` >
       `./config.yaml` > `<exe-dir>/config.yaml` > user config dir), and how to
       view detailed per-subcommand help (`teams-cli <subcommand> --help`, e.g.,
       `teams-cli join --help`), plus a dedicated `Example` field.
     * `join` (`internal/cli/join.go`): Add comprehensive `Long` and `Example`
       covering required `<meeting-url>` (or `--url` / `meeting_url` in config),
       the 4 capture protocols (`--mode`), activity-aware `--duration` +
       voice/caption overtime continuation + `--idle-timeout` +
       `--lobby-timeout`, delayed post-call AI Summary (`--ai-summary-delay`,
       `--no-ai-summary`, `--append-ai-summary`, `--not-before`), all 8 levels of
       output target precedence (`arg[1]` > `--output-file` > `--output` >
       `--output-dir` > `--output-folder` > config `output_file` >
       `output_folder` > `output_dir`), all 8 filename template placeholders
       (`<category>`, `<concat-meeting-name>`, `<meeting-name>`, `<meeting-id>`,
       `<YYYYMMDD>`, `<YYYY-MM-DD>`, `<HHMMSS>`, `<timestamp>`), `--queue` batch
       mode (`::` comments, forwarded flags), and copy-pasteable examples (live
       join, Recap export, timed Windows Task Scheduler one-shot, and batch
       queue).
     * `captions` (`internal/cli/join.go`): Expand `Long` and add `Example`
       explaining how `captions` differs from `join` (`[meeting-url]` is optional
       and defaults to opening `https://teams.microsoft.com/v2/` for interactive
       call/recap selection when omitted), plus the 4 capture protocols, output
       target precedence, filename template placeholders, `--queue` batch mode,
       and concrete examples.
     * `auth` (`internal/cli/auth.go`): Expand `Long` and add `Example`
       explaining interactive headed sign-in + MFA caching in `--profile-dir`
       (`%LOCALAPPDATA%\teams-cli\profile` default on Windows,
       `~/.config/teams-cli/profile` on Linux), automatic Microsoft Entra
       account tile selection (`--account` / `default_account`), profile
       exclusivity/locking across concurrent browser sessions, and concrete
       examples.
     * `schedule` (`internal/cli/schedule.go`): Expand `Long` and add `Example`
       covering recurring 5-field `cron` and one-shot `at` (`RFC3339` or local
       `YYYY-MM-DD HH:MM`) triggers in `config.yaml`, the shared-profile atomic
       `busy` gate (released as soon as the live call ends, before
       `--ai-summary-delay`), startup sweep of deferred `--append-ai-summary`
       entries in `queue_file`, and the explicit 3-tier precedence rule from
       `11ED4180`: `teams-cli schedule` applies command-line flags to the base
       configuration first (`applyScheduleFlags`) and then applies each
       `meetings:` entry's per-meeting settings (`buildScheduleEntryConfig`:
       `mode`/`capture_mode`, `duration`, `idle_timeout`, `ai_summary_delay`,
       `lobby_timeout`, `category`, `capture_audio`/`audio`, `output_dir`), so
       per-meeting settings in `config.yaml` override global `schedule` flags for
       that meeting.
     * `update` (`internal/cli/update.go`): Expand `Long` and add `Example`
       covering GitHub Releases asset lookup (`teams-cli-<goos>-<goarch>[.exe]`
       across `{linux, darwin, windows} x {amd64, arm64}`), `SHA256SUMS`
       verification before binary swap, `<exe>.old` rollback backup,
       `<exe>.lock` / `<exe>.lock.ctl` inter-process locking, `--check`,
       `--version <tag>`, `--force` (reinstall/downgrade), and manual lock
       recovery steps.
     * `generate-config` (`internal/cli/generate_config.go`): Move examples into
       the dedicated `Example` field and ensure `Long` and `Example` are equally
       thorough and consistent with the other subcommands (covering aliases
       `init-config`/`gen-config`, destination resolution precedence, `.yaml`/`.yml`
       positional file target vs existing directory, `--force` atomic overwrite,
       and ignoring root `--config` / `-f`).
   * **Unit tests (`internal/cli/help_test.go`)**:
     * Verify `<subcommand> --help` and `help <subcommand>` for every subcommand
       (`auth`, `captions`, `generate-config`, `join`, `schedule`, `update`) and
       `--help` on `root` print command-specific `Long` details and `Examples:`
       that are strictly distinct from and richer than the 1-line `Short`
       summary.
     * Verify `teams-cli [command] --help`, `teams-cli <command> --help`, and
       `teams-cli unknown-cmd --help` emit the explicit guidance note while
       `teams-cli --help` (no positional args) and
       `teams-cli https://teams.microsoft.com/meet/synthetic --help` (valid
       Teams meeting URL) do not emit the unknown-command note.
2. **Unit 2 (`025.002-T` — `docs` domain, size `S`, complexity `low`, depends on `025.001-T`)**:
   * Update `docs/USER_GUIDE.md` (`## Configuration` and
     `### Built-in daemon scheduling (teams-cli schedule)`) and `README.md` to
     resolve `11ED4180`: qualify the statement that command-line flags override
     every config value by documenting that `teams-cli schedule` applies global
     CLI flags to the base config first and then applies each `meetings:` entry's
     per-meeting settings (`mode`/`capture_mode`, `duration`, `idle_timeout`,
     `ai_summary_delay`, `lobby_timeout`, `category`, `capture_audio`/`audio`,
     `output_dir`), so per-meeting overrides in `config.yaml` take precedence
     over global `schedule` flags for that entry.
   * Update `README.md` and `docs/USER_GUIDE.md` command sections to show
     concrete per-subcommand help invocations (`teams-cli <subcommand> --help`,
     e.g., `teams-cli join --help`, `teams-cli schedule --help`) rather than
     ambiguous `teams-cli [command] --help` prose.

## Rejected Alternatives

* **Option B (Remove positional arguments from `root`)**: Rejected because
  `teams-cli <meeting-url> [output-markdown-path]` is a primary supported CLI
  invocation mode across user documentation and batch queue files.
* **Option C (Change `buildScheduleEntryConfig` so global flags override per-meeting config)**:
  Rejected because per-meeting overrides in `config.yaml` are intentionally more
  specific than daemon-wide default flags on `teams-cli schedule`, and `11ED4180`
  specifically tracks documenting that contract accurately.

## Unresolved Questions

None. All CLI command signatures, flag bindings, and `buildScheduleEntryConfig`
precedence rules are verified directly against `internal/cli/` and
`internal/config/`.

## Risks and Mitigations

* **Risk: Live passcode/token leak test failure (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`)**:
  * *Mitigation*: Every example URL in `Long`, `Example`, `internal/cli/help_test.go`,
    `README.md`, and `docs/USER_GUIDE.md` must use synthetic URLs
    (`https://teams.microsoft.com/meet/synthetic...` or
    `https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic...`).
* **Risk: Root help interceptor triggering on valid meeting URLs when `--help` is appended**:
  * *Mitigation*: `installRootHelpHandler` checks
    `teams.ValidateMeetingURL(args[0]) != nil` before printing the guidance note,
    so a valid Teams URL passed alongside `--help` prints normal root help
    without a false-positive warning.
