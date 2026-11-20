# go-teams Development Guidelines

Last updated: 2026-09-04

go-teams is a standalone Go CLI for autonomous Microsoft Teams live-transcription and audio capture via headless browser automation (chromedp).

## Technology Stack

| Layer           | Technology                | Notes                                 |
|-----------------|---------------------------|---------------------------------------|
| Language        | Go 1.26 | (Go 1.26; standard library first, minimal third-party dependencies)          |
| Build           | go            | `go build ./...`                   |
| Test            | go test           | `go test ./...`                    |
| Lint            | golangci-lint                | `golangci-lint run`                    |
| Format          | gofmt             | `gofmt -l .`                  |
| CI              | GitHub Actions           | GitHub Actions runs go build, go vet (including -tags=integration), golangci-lint, and go test -race with coverage. Formatting is NOT checked in CI — gofmt is enforced only by the local pre-push gate. Release and CLI-reference-drift workflows run separately.                          |
| Browser automation | chromedp | Pure-Go CDP client — no external driver downloads |
| CLI framework   | spf13/cobra | Command routing: `auth`, `join`, `schedule` |
| Scheduler       | robfig/cron/v3 | Cron-based daemon mode |
| Audio encoding  | braheezy/shine-mp3 | Pure-Go MP3 encoder |
| Assertions      | stretchr/testify | Unit and integration test assertions |

## Project Structure

```text
go-teams/
├── cmd/
│   └── go-teams/        # main package (Cobra root command)
├── internal/
│   ├── audio/           # pure-Go MP3 encoding, audio capture
│   ├── browser/         # chromedp allocation, stealth flags, FSM
│   ├── capture/         # capture orchestration
│   ├── cli/             # command wiring and flag parsing
│   ├── config/          # configuration loading and validation
│   ├── scheduler/       # cron-based daemon mode
│   ├── teams/           # Teams meeting join and auth flows
│   └── transcript/      # DOM caption interception, dedup, Markdown writer
├── docs/                # durable knowledge (design-docs, product-specs, plans, ...)
├── scripts/             # harness helper scripts
├── .backlogit/          # backlog workflow state
├── .github/             # agent harness
└── go.mod
```

## Commands

```bash
go build ./...              # Build
go test ./...               # Run all tests
golangci-lint run               # Lint
gofmt -l .             # Format check
gofmt -w .                  # Format fix
go vet ./...                # Static check
go test -tags=integration ./...   # Integration tests
```

## Code Style and Conventions

### Error Handling

Errors are returned, never panicked, from library code. Wrap with context using `fmt.Errorf("...: %w", err)`. Sentinel errors use `errors.Is`/`errors.As` comparisons.

### Naming

PascalCase (MixedCaps) for exported identifiers, camelCase for unexported identifiers; package names are short, lowercase, and free of underscores.

### Documentation

Godoc comments on every exported type, function, and package, starting with the identifier name.

### Testing

* TDD required: write tests first, verify they fail, then implement
* Test tiers in `co-located *_test.go files` directory:
Unit: `go test ./...` with table-driven tests co-located as `*_test.go`; Integration: `go test -tags=integration ./...` gated behind the `integration` build tag (Teams meeting join, chromedp session flows).

## Search Strategy

Use available workspace search tools before falling back to file-based search
(grep, glob, view). Indexed search returns precise results with minimal token
cost. File-based tools read raw content into the context window, consuming
tokens proportional to file size.

**Search tool preference order:**

1. When the `agent-engram` capability pack is enabled and reachable: `unified_search`, `query_memory`, `map_code`, `list_symbols`, `impact_analysis`, `query_graph`
2. Otherwise use workspace-indexed tools (if available): semantic search, symbol lookup, call graphs
3. File-based fallback: grep, glob, view — only when indexed results are insufficient

## Durable Knowledge Layout

| Path | Purpose |
|---|---|
| `docs/compound/` | Reusable learnings and hard-won fixes |
| `docs/plans/` | Implementation plans |
| `docs/decisions/` | Durable decisions and investigation outputs |
| `docs/memory/` | Session memory and checkpoints |
| `docs/closure/` | Review, runtime verification, and closure artifacts |
| `docs/design-docs/` | Graduated architecture and design rationale |
| `docs/product-specs/` | Product-oriented requirements |

## Session Memory Requirements

* Working agent sessions MUST persist output to `docs/memory/` before the session ends — do NOT rely on built-in AI assistant memory features, which write to their own managed locations.
* When the context window reaches approximately 65% capacity, checkpoint current work before continuing.
* For long sessions, save memory checkpoints after completing each phase or major task group.
* Content to capture: task IDs completed, files modified, decisions and rationale, failed approaches, open questions, and next steps.
* File convention: `docs/memory/{YYYY-MM-DD}/{descriptive-slug}-memory.md`
* Memory date folders (`docs/memory/{YYYY-MM-DD}/`) use the UTC date, and all recorded dates and times are UTC (`Z`); see `.github/instructions/output-timestamps.instructions.md`.
* After writing memory, invoke the **compact-context** skill to consolidate stale checkpoints and finalize decided-plans. This is a mandatory workflow step, not advisory.
* If context has grown from loading multiple skill definitions mid-session, consider invoking **compact-context** proactively before hitting hard thresholds.

## Foundational Protocols

| Protocol | Location | When |
|---|---|---|
| **Circuit Breaker** | `.github/instructions/circuit-breaker.instructions.md` | All retry loops and failure handling |
| **Concurrency Control** | `.github/instructions/concurrency.instructions.md` | Multi-agent or human+agent concurrent edits |
| **Skill Discovery** | `scripts/search.ps1` / `scripts/search.sh` | Finding capabilities by keyword (Primitive 6) |

## Optional Capability Packs

### agent-engram

When the workspace enabled the `agent-engram` capability pack:

* verify the engram daemon / MCP surface is reachable before depending on indexed lookup
* prefer engram tools for conceptual search, symbol discovery, call-graph lookup, impact analysis, and workspace-memory retrieval
* verify the workspace binding state before relying on results; if the daemon auto-binds the workspace, prefer status checks over repeated rebinding
* use `sync_workspace` or the equivalent freshness operation when code changed outside the expected indexing flow
* if semantic search is unavailable or degraded, fall back to `list_symbols` + `map_code` + `impact_analysis` before resorting to broad file scans
* treat `.engram/` generated artifacts as tool-managed state rather than files to hand-edit casually

### backlogit

When the workspace enabled the `backlogit` capability pack:

* verify the backlogit MCP / CLI surface is reachable before depending on queue, dependency, memory, or traceability operations
* prefer backlogit query operations for targeted state lookup instead of reading many backlog markdown files into context
* use backlogit queue and dependency operations when available rather than inferring execution order from prose alone
* write concise memory summaries and checkpoints through backlogit operations at task and session boundaries when supported
* append significant task comments and associate commits with task IDs for execution traceability when those operations are available
* if backlog content was edited outside the normal mutation flow, refresh the backlogit index before relying on query results

### kbdb

When the workspace enabled the `kbdb` capability pack:

* follow `.github/instructions/kbdb.instructions.md`
* verify the kbdb server / MCP surface is reachable and sources are indexed before depending on indexed documentation retrieval
* prefer `search_local_docs`, `search_semantic`, and `research_topic` for conceptual, API-oriented, or documentation questions before broad web search or raw file scans
* use `traverse_doc_links` to follow related documentation and `list_sources` to confirm indexed coverage
* if the server is unavailable or sources are not indexed, fall back to grep, glob, or direct file reading and note reduced confidence
* treat `.kbdb/` generated artifacts as tool-managed state rather than files to hand-edit casually

## Core Workflow Protocols

These protocols are part of the core harness and are always active; they are not capability packs. Each applies when its surface condition is met.

### browser-verification

When verifying browser-visible UI flows (core protocol):

* verify the target server or preview environment is reachable before launching browser work
* choose headed vs headless mode intentionally and record the reason
* derive browser routes from changed pages, components, or affected user journeys
* treat OAuth, email, SMS, payments, CAPTCHAs, or other external flows as explicit human checkpoints
* carry browser findings into runtime verification and operational closure rather than leaving them as informal notes

### continuous-learning

When recurring practice is observed across sessions (core protocol):

* store observation state under `.gsd/continuous-learning/`
* keep hook capture optional and environment-specific; manual capture is still valid
* use `observe` to capture recurring workflow signals, `learn` to infer instincts, and `evolve` to promote mature patterns into `learned-*` artifacts
* do not harden a rule into a learned instruction or skill until it has enough corroborating observations to justify the promotion
* treat learned artifacts as explicit repository knowledge rather than invisible prompt-only behavior

### strict-safety

For elevated-risk or destructive work (core protocol):

* follow `.github/instructions/strict-safety.instructions.md`
* express risky work as `ProposedAction` entries with `ActionRisk` and `ActionResult`
* require explicit approval before destructive actions and prefer approval for high-blast-radius actions
* keep risky action records visible in plan hardening, review, runtime verification, and operational closure

### release-observability

For runtime-affecting or deployment-sensitive release units (core protocol):

* follow `.github/instructions/release-observability.instructions.md`
* produce monitoring plans with SLIs, dashboards, baselines, and alert thresholds before merge
* complete pre-deploy audit checklists for runtime, migration, or rollout-risk changes
* define explicit post-deploy observation windows with owner and duration
* declare rollback triggers with named metrics and thresholds
* carry all release-observability artifacts into operational closure

### adversarial-review

When a review surfaces 3+ P0/P1 findings or touches security-sensitive work (core protocol):

* follow `.github/instructions/adversarial-review.instructions.md`
* escalate from standard review when 3+ P0/P1 findings appear or the work is security-sensitive
* dispatch parallel reviewer instances across different model tiers for cross-model diversity
* assemble consensus-weighted findings (HIGH / MEDIUM / LOW confidence)
* treat HIGH-confidence P0/P1 findings as gate-blocking
* feed remediation queue entries into backlog

## Remote Operator Integration

<!-- pack-scope:agent-engram:begin -->
### agent-engram

When `agent-engram` is available:

* Verify workspace binding before relying on indexed results.
* If the workspace is not bound or indexed, run `sync_workspace` or the workspace's equivalent freshness operation before searching.
* Fall back to grep, glob, or direct file reads only when indexed results are unavailable or insufficient.
<!-- pack-scope:agent-engram:end -->

## Backlog Workflow Expectations

When a backlog tool is active in the workspace:

* prefer queue-aware and dependency-aware operations over prose-only sequencing when the tool surface supports them
* use comments, checkpoints, and commit-tracking operations when they add traceability
* refresh the backlog index or query cache after out-of-band edits before trusting query results
* avoid inventing parallel markdown trackers outside the configured backlog tool surface

## Workspace-Local `.bin/` Tools and `.gsd/` Runtime Compatibility

Provision GSD and every enabled external tool under the repository-root
`.bin/` directory (`.bin/gsd` or `.bin/gsd.exe`, the `gsd-assets*.zip` bundle,
and executables such as backlogit, Engram, kbdb, and Bun). Invoke them by an
explicit workspace-relative path. Current GSD compatibility commands may also
mirror a runtime and `runtime-manifest.json` under `.gsd/`; treat that mirror as
runtime compatibility state, never as the preferred tool installation location
or as permission to use a shared user-home engine. The bundle is read directly
without unpacking `templates/` or `schemas/` into the repository.

Generated by gsd | Template: copilot-instructions.md.tmpl

<!-- engram:start -->
## Engram Agent Memory — GitHub Copilot Integration

Engram is running as an MCP server at `http://127.0.0.1:7437/mcp`.

### Available Tools

| Tool | Purpose |
|------|---------|
| `set_workspace` | Register this workspace at session start |
| `query_memory` | Retrieve stored context, tasks, and code knowledge |
| `create_task` | Create a new task in the workspace task list |
| `update_task` | Update task status or details |
| `map_code` | Index code files for semantic navigation |
| `unified_search` | Search across all content types |
| `query_changes` | Query git commit history by file, symbol, or date |

### Recommended Workflow

1. **Session start**: Call `set_workspace` with the current workspace path.
2. **Before coding**: Call `query_memory` to load relevant context.
3. **Task tracking**: Use `create_task` and `update_task` to record progress.
4. **Code navigation**: Use `map_code` and `unified_search` for codebase exploration.
5. **Change history**: Use `query_changes` to understand recent modifications.
<!-- engram:end -->
