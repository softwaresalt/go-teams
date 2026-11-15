---
title: CLI subcommand help and schedule precedence documentation review and operational closure
date: 2026-11-14
feature: 025-F
shipment: 025-S
branch: feat/025-s-comprehensive-cli-subcommand-help-root-help-placeholder-guidance-and-schedule-flag-precedence-documentation
status: closed
closure_status: READY
compaction_status: done
merge_commit: a5a889df22ad2782fc383cacfe17951ea5602b3d
pull_request: 64
---

## Release scope

Shipment `025-S` (`025-F`, `025.001-T`, `025.002-T`) delivers deliberation
`014-DL` (source stash `A52B6644` and `11ED4180`): comprehensive help for every
`teams-cli` command, guidance for a mistyped or placeholder argument, and
documentation of the schedule flag precedence.

* `025.001-T` (`internal/cli/*.go`, `internal/cli/help_test.go`, commit `b8aee67`):
  * Adds comprehensive `Long` and `Example` help to the root, `join`, `captions`,
    `auth`, `schedule`, `update`, and `generate-config` commands.
  * Adds `installRootHelpHandler`, which prints a redacted `Note:` banner before
    root help when `teams-cli [command] --help` or `teams-cli help [command]`
    receives a placeholder or unknown argument.
* `025.002-T` (`README.md`, `docs/USER_GUIDE.md`, commit `3a7a136`):
  * Documents the three-tier schedule precedence (top-level config, global
    `schedule` flags with base `ephemeral-live` normalization, per-meeting
    `meetings:` entries) and qualifies the statement that flags override every
    config value.

## Review

Multi-persona, cross-model adversarial and `rubber-duck` review of
`main..0b05c492e6299f8e914945ad200ae229f29adfeb` (eight reviewers) returned
`0 P0, 0 P1`, followed by GitHub Copilot review of PR #64 (P-018
`copilot-review` gate `SATISFIED`).

| Area | Finding | Disposition |
|---|---|---|
| CLI (Architecture, Go) | Intercepted `help [command]` path omitted `--version` | Fixed in-scope (P-021 C1); commit `14545c9` |
| CLI (`rubber-duck`, Security) | Schemeless and protocol-relative URL credentials echoed in the banner | Fixed in-scope (P-021 C1); commit `14545c9`, hardened in `be92c2b` (see Copilot row) |
| CLI (`rubber-duck`, Correctness) | `--help <subcommand>` and `help -f cfg <topic>` mis-identified the topic | Fixed in-scope (P-021 C1): `findDirectSubcommand`, flag-value skipping; commit `14545c9` |
| CLI help (`rubber-duck`, Coupling) | `update --help` lock recovery listed fewer steps than README and USER_GUIDE | Fixed in-scope (P-021 C1); commit `14545c9` |
| CLI help (Correctness) | `schedule --help` omitted required entry fields and startup-sweep behavior | Fixed in-scope (P-021 C1); commit `14545c9` |
| Docs (Scope, Coupling) | README and USER_GUIDE lacked the base-normalization caveat, busy gate, sweep, shutdown window, default duration | Fixed in-scope (P-021 C1); commit `14545c9` |
| Copilot (`PRRT_kwDOTStvas63lNfV`, `internal/cli/root.go`) | Banner could leak password prefixes (`//user:secret?pw@host` became `//user:secret`; an embedded `https:` kept a prefix) | Fixed in-scope (P-021 C1): original argument inspected first; inputs with an `http://` or `https://` prefix go through `RedactArgs` (prefix dispatch, not URL validation), and other inputs with `@ ? #`, `://`, or `http(s):` print `[redacted]`; regression tests added; commit `be92c2b`; replied citing the commit; thread resolved |

Blocking findings: `P0=0, P1=0`. Outcome: `READY`.

### Review record

| Field | Value |
|---|---|
| Reviewed diff identity | `main..be92c2b98ab2ad94a72de955d58ad8e9157900cc` (PR #64) |
| Reviewed HEAD | `be92c2b98ab2ad94a72de955d58ad8e9157900cc` |
| `dispatch_mode` | `multi-agent` |
| P-018 `copilot-review` gate | First run `UNRESOLVED_THREADS` (one thread); after the fix, reply, and resolution, `SATISFIED` (exit 0). HEAD did not advance afterward |

## Follow-ups

* Stash `2925E813` (deferred scope expansion, provisional priority `low`, raised by
  Copilot thread `PRRT_kwDOTStvas63lUy4` on closure PR #65): correct the
  `redactHelpArg` doc comment in `internal/cli/root.go`, which says "well-formed"
  although dispatch is by `http://`/`https://` prefix only. Stage triages it.

## Post-merge closure

| Field | Value |
|---|---|
| Feature PR | #64, merge commit `a5a889df22ad2782fc383cacfe17951ea5602b3d`, merge commit strategy (P-009) |
| Merge confirmation | `state: MERGED` (`2026-11-15T01:49:14Z`); `git merge-base --is-ancestor` against `origin/main` returned 0 |
| Topology gate | `autoharness gate pipeline-topology --mode agent --shipment 025-S --phase lifecycle --json` passed (exit 0, `BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`) on `post-merge/025-s-comprehensive-cli-subcommand-help-root-help-placeholder-guidance-and-schedule-flag-precedence-documentation` immediately before the cascade |
| Close path | P-015 verified fully-covered-root cascade (`classify_shipment_close_path` returned `CASCADE`; `025-F` is a root feature and its only descendants `025.001-T` and `025.002-T` are manifest members; backlogit `1.12.0` is on the verified `1.12` engine line) |
| Command | `backlogit shipment ship 025-S --sha a5a889df22ad2782fc383cacfe17951ea5602b3d` |
| `archived_ids` | `025.001-T`, `025.002-T`, `025-F`, `025-S` (equal to `required_ids`; no unexpected artifact) |
| `returned_ids` | empty |
| `parent_id` preservation | `025.001-T` and `025.002-T` retained `parent_id: 025-F` |
| Final records | `025-S`: `archived_status: shipped`; `025-F`: `archived_status: done`; tasks `status: archived` |
| Archive integrity (P-007) | No archive file reported as deleted |

### Source artifact cleanup

| Artifact | Disposition | Result |
|---|---|---|
| Stashes `A52B6644`, `11ED4180` | Already archived by Stage at harvest; absent from the active stash | skipped |
| Deliberation `014-DL` | `retained_description_mention`; status `queued` with zero live references in `.backlogit/queue/`; archived via `backlogit archive 014-DL` | archived |

### Compaction (P-020)

`compact-context` ran at closure. It wrote
`docs/memory/compacted/2026-11-14-025-S-cli-help-and-schedule-precedence-compacted.md`
and moved the verbose session memory to
`docs/archive/memory/2026-11-14/stage-025-S-memory.md` and
`docs/archive/memory/2026-11-14/025-S-ship-memory.md`. `compound-refresh` kept all
nine existing learnings and added
`docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`.
`docs/ARCHITECTURE.md` now lists the root help handler. `compaction_status` is `done`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Static analysis | `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `gofmt -l .` (clean) | PASS |
| Full test suite | `go test -race ./...` on Windows | PASS |
| Secret hygiene | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| CI `ubuntu-latest` | `Build, vet, lint & test`, GitHub Actions run `43664442369` (PR #64, HEAD `be92c2b`) | PASS |
| CI `windows-latest` | `Build, vet & test (Windows)`, same run | PASS |
| Backlog integrity | `backlogit sync` (157 artifacts indexed) and `backlogit doctor` ("No issues found.") | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Change help rendering and add a banner that echoes a rejected argument | low | Credential-bearing inputs are suppressed, not truncated; regression tests assert secrets never appear | applied and verified |
| Cascade-close `025-S` with `backlogit shipment ship` | medium | Classified as a P-015 fully-covered-root close; `archived_ids`, `returned_ids`, `parent_id`, and the archive-deletion guard verified afterward | applied and verified |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Healthy signal | READY | `go build ./...` and `go test -race ./...` pass locally; both CI jobs green on PR #64; `teams-cli --help`, `teams-cli help <command>`, and `teams-cli [command] --help` render the expected help |
| Failure signal | READY | A help command panics or prints a credential-bearing argument, or `--help` output omits the documented precedence |
| Monitoring plan | READY | After the next tagged release, run `teams-cli schedule --help` and `teams-cli [command] --help` from the published binary and confirm the banner and precedence text |
| Rollback trigger | READY | Help output regresses or leaks an argument |
| Rollback procedure | READY | Revert merge commit `a5a889df22ad2782fc383cacfe17951ea5602b3d` with `git revert -m 1 a5a889df22ad2782fc383cacfe17951ea5602b3d` through a reviewed PR; the change touches help text, documentation, and the help renderer only, so no data migration is needed |
| Owner | READY | Repository maintainer cutting the next release |
| Validation window | READY | The next tagged release |

## Dark Mode Execution Summary (P-017)

* **Gates passed**:
  * `pipeline-topology` (`lifecycle` on the closure branch) - exit 0
  * Local Review Readiness (`P-014`) - `LOCAL_REVIEW_READY` for HEAD `be92c2b98ab2ad94a72de955d58ad8e9157900cc` (`READY`, `0 P0, 0 P1`)
  * Copilot Review Completion Gate (`P-018`) - `SATISFIED` (exit 0) with HEAD unchanged at the last mile
  * Merge Strategy (`P-009`) - `gh pr merge 64 --merge --delete-branch` succeeded without admin fallback (`DARK_MODE_MERGE_AUTHORIZED`)
  * Context Compaction (`P-020`) - `compaction_status: done`

## Handoff

Related [decision](../decisions/2026-11-14-cli-subcommand-help-and-schedule-precedence-deliberation.md)
(`014-DL`), [plan](../plans/2026-11-14-cli-subcommand-help-and-schedule-precedence-plan.md),
and the [quality score](../QUALITY_SCORE.md). Compaction (P-020) completed at
post-merge closure; see the compacted
[memory summary](../memory/compacted/2026-11-14-025-S-cli-help-and-schedule-precedence-compacted.md).
