# go-teams Development Guidelines

Last updated: 2026-07-08

go-teams is a standalone Go CLI for autonomous Microsoft Teams live-transcription
and audio capture via headless browser automation (chromedp).

## Technology Stack

| Layer           | Technology                | Notes                                   |
|-----------------|---------------------------|------------------------------------------|
| Language        | Go 1.23+                  | module not yet initialized — recommend `go mod init` targeting Go 1.23+ |
| Build           | go                        | `go build ./...`                         |
| Test            | go test                   | `go test ./...`                          |
| Lint            | golangci-lint             | `golangci-lint run`                      |
| Format          | gofmt                     | `gofmt -l .`                             |
| CI              | GitHub Actions            | No workflow yet configured; add one once go.mod exists (build, vet, lint, test matrix) |
| Browser automation | chromedp (planned)     | Pure-Go CDP client — no external driver downloads |
| CLI framework   | spf13/cobra (planned)     | Command routing: `auth`, `join`, `schedule` |
| Scheduler       | robfig/cron/v3 (planned)  | Cron-based daemon mode |

## Project Structure

```text
go-teams/
├── cmd/
│   └── teams-cli/        # main package (Cobra root command) — not yet created
├── internal/
│   ├── browser/          # chromedp allocation, stealth flags, FSM
│   ├── auth/              # UserDataDir session persistence, auth command
│   ├── transcript/        # DOM caption interception, dedup, Markdown writer
│   ├── audio/              # pure-Go MP3 encoding, audio capture
│   └── scheduler/          # cron-based daemon mode
├── docs/
│   ├── design-docs/
│   └── product-specs/
├── .backlogit/            # backlog workflow state
├── .github/                # agent harness (this install)
└── go.mod                 # not yet created
```

## Commands

```bash
go build ./...              # Build
go test ./...                # Run all tests
golangci-lint run             # Lint
gofmt -l .                     # Format check
gofmt -w .                     # Format fix
go vet ./...                   # Static check
```

## Code Style and Conventions

### Error Handling

Errors are returned, never panicked, from library code. Wrap with context
using `fmt.Errorf("...: %w", err)`. Sentinel errors use `errors.Is`/`errors.As`
comparisons.

### Naming

PascalCase for exported identifiers, camelCase for unexported identifiers;
package names are short, lowercase, and free of underscores.

### Documentation

Godoc comments on every exported type, function, and package, starting with
the identifier name.

### Testing

* TDD required: write tests first, verify they fail, then implement
* Test tiers:
  * Unit: `go test ./...` — table-driven tests co-located as `*_test.go`
  * Integration: `go test -tags=integration ./...` — gated behind the `integration` build tag (Teams meeting join, chromedp session flows)

## Search Strategy

Use available workspace search tools before falling back to file-based search
(grep, glob, view). Indexed search returns precise results with minimal token
cost. File-based tools read raw content into the context window, consuming
tokens proportional to file size.

**Search tool preference order:**

1. When the `agent-engram` capability pack is enabled and reachable: `unified_search`, `query_memory`, `map_code`, `list_symbols`, `impact_analysis`, `query_graph`
2. When the `graphtor-docs` capability pack is enabled and reachable: `search_local_docs`, `search_semantic`, `research_topic` for documentation, API references, and concept research
3. Otherwise use workspace-indexed tools (if available): semantic search, symbol lookup, call graphs
4. File-based fallback: grep, glob, view — only when indexed results are insufficient

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
* After writing memory, invoke the **compact-context** skill to consolidate stale checkpoints and finalize decided-plans. This is a mandatory workflow step, not advisory.
* If context has grown from loading multiple skill definitions mid-session, consider invoking **compact-context** proactively before hitting hard thresholds.

## Foundational Protocols

| Protocol | Location | When |
|---|---|---|
| **Circuit Breaker** | `.github/instructions/circuit-breaker.instructions.md` | All retry loops and failure handling |
| **Concurrency Control** | `.github/instructions/concurrency.instructions.md` | Multi-agent or human+agent concurrent edits |
| **Skill Discovery** | `scripts/search.ps1` / `scripts/search.sh` | Finding capabilities by keyword (Primitive 6) |

## Optional Capability Packs

### agent-intercom

When the workspace enabled the `agent-intercom` capability pack:

* verify the intercom server / tool surface is reachable before depending on remote approval or operator steering
* call heartbeat / ping at session start and keep it alive during long-running work
* broadcast major workflow transitions so the operator can observe planning, build, review, verification, and closure progress
* route destructive terminal commands and destructive file operations through the intercom approval workflow
* use transmit / standby flows when blocked on operator clarification or when intentionally pausing for instructions
* if the intercom service is unreachable, warn that remote visibility is degraded and avoid pretending approval or operator awareness exists

> No intercom MCP server is currently configured in this workspace's `.mcp.json`. This overlay is installed proactively (per the `full` preset) and activates automatically once one is added.

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

### browser-verification

When the workspace enabled the `browser-verification` capability pack:

* verify the target server or preview environment is reachable before launching browser work
* choose headed vs headless mode intentionally and record the reason
* derive browser routes from changed pages, components, or affected user journeys
* treat OAuth, email, SMS, payments, CAPTCHAs, or other external flows as explicit human checkpoints
* carry browser findings into runtime verification and operational closure rather than leaving them as informal notes

> This pack verifies the harness's own web-facing surfaces (if any). It is
> distinct from this project's chromedp automation product code, which is
> reviewed by the go-reviewer and concurrency-reviewer personas.

### graphtor-docs

When the workspace enabled the `graphtor-docs` capability pack:

* verify the graphtor-docs server is reachable (`get_status`) once per session
* prefer `search_local_docs`, `search_semantic`, and `research_topic` over broad web or filesystem search for domain concepts and API references
* fall back to grep/view over `docs/` only when the server is unavailable

### continuous-learning

When the workspace enabled the `continuous-learning` capability pack:

* store observation state under `.autoharness/continuous-learning/`
* keep hook capture optional and environment-specific; manual capture is still valid
* use `observe` to capture recurring workflow signals, `learn` to infer instincts, and `evolve` to promote mature patterns into `learned-*` artifacts
* do not harden a rule into a learned instruction or skill until it has enough corroborating observations to justify the promotion
* treat learned artifacts as explicit repository knowledge rather than invisible prompt-only behavior

### strict-safety

When the workspace enabled the `strict-safety` capability pack:

* follow `.github/instructions/strict-safety.instructions.md`
* express risky work as `ProposedAction` entries with `ActionRisk` and `ActionResult`
* require explicit approval before destructive actions and prefer approval for high-blast-radius actions
* keep risky action records visible in plan hardening, review, runtime verification, and operational closure
* the planned Teams auth flow (interactive login/MFA, persisted `UserDataDir` session state) is security-sensitive — classify changes to it as `ActionRisk: high` by default

### release-observability

When the workspace enabled the `release-observability` capability pack:

* follow `.github/instructions/release-observability.instructions.md`
* produce monitoring plans with SLIs, dashboards, baselines, and alert thresholds before merge
* complete pre-deploy audit checklists for changes with runtime, migration, or rollout risk
* define explicit post-deploy observation windows with owner and duration
* declare rollback triggers with named metrics and thresholds
* carry all release-observability artifacts into operational closure

### adversarial-review

When the workspace enabled the `adversarial-review` capability pack:

* follow `.github/instructions/adversarial-review.instructions.md`
* escalate from standard review when 3+ P0/P1 findings appear or the work is security-sensitive
* dispatch parallel reviewer instances across different model tiers for cross-model diversity
* assemble consensus-weighted findings (HIGH / MEDIUM / LOW confidence)
* treat HIGH-confidence P0/P1 findings as gate-blocking
* feed remediation queue entries into backlog

## Remote Operator Integration

### agent-intercom

When `agent-intercom` is available:

* Call `ping` at the start of any multi-step session to confirm liveness.
* Broadcast progress at meaningful phase transitions — do not broadcast every trivial step.
* Route approval for destructive actions through the intercom approval workflow before executing.
* If intercom becomes unreachable mid-task, warn that operator visibility is degraded and continue only with safe, non-destructive work.

The `ping-loop.prompt.md` prompt is available in `.github/prompts/` for sustained heartbeat sessions when the pack is installed.

### agent-engram

When `agent-engram` is available:

* Verify workspace binding before relying on indexed results.
* If the workspace is not bound or indexed, run `sync_workspace` or the workspace's equivalent freshness operation before searching.
* Fall back to grep, glob, or direct file reads only when indexed results are unavailable or insufficient.

## Backlog Workflow Expectations

When a backlog tool is active in the workspace:

* prefer queue-aware and dependency-aware operations over prose-only sequencing when the tool surface supports them
* use comments, checkpoints, and commit-tracking operations when they add traceability
* refresh the backlog index or query cache after out-of-band edits before trusting query results
* avoid inventing parallel markdown trackers outside the configured backlog tool surface

This workspace uses **backlogit** at `.backlogit/`. See
`.github/instructions/backlogit.instructions.md`,
`.github/instructions/backlogit-sql-schema.instructions.md`, and
`.github/instructions/backlogit-yaml-header-tooling.instructions.md` for
tool-specific query, queue, checkpoint, and frontmatter details.

Generated by autoharness | Template: copilot-instructions.md.tmpl

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