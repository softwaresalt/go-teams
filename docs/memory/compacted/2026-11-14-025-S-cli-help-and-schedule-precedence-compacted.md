---
type: memory-compacted
date: 2026-11-14
shipment: 025-S
feature: 025-F
source:
  - docs/archive/memory/2026-11-14/stage-025-S-memory.md
  - docs/archive/memory/2026-11-14/025-S-ship-memory.md
---

# Compacted memory - Shipment 025-S (CLI subcommand help and schedule precedence)

**Shipped**: PR #64, merge `a5a889df22ad2782fc383cacfe17951ea5602b3d`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship 025-S --sha a5a889df22ad2782fc383cacfe17951ea5602b3d`);
deliberation `014-DL` archived via `backlogit archive 014-DL`.

## Decision and outcome

* `internal/cli`: every command has comprehensive `Long` and `Example` help;
  `installRootHelpHandler` prints a redacted `Note:` banner when `teams-cli
  [command] --help` or `teams-cli help [command]` receives a placeholder or
  unknown argument. `--help <subcommand>` renders that subcommand's help,
  `help -f cfg <topic>` skips flag values, and the intercepted help path shows
  `--version`.
* Banner redaction: only well-formed `http(s)://` inputs go through
  `teams.RedactArgs`; any other input carrying `@ ? #`, `://`, or an embedded
  `http(s):` token prints `[redacted]`
  (`docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`).
* Docs: README, `docs/USER_GUIDE.md`, `schedule --help`, and `update --help`
  document the three-tier schedule precedence (config, global flags with base
  `ephemeral-live` normalization, per-meeting entries), the busy gate, startup
  sweep, 15-second shutdown window, 1-hour default duration, and four-step
  manual update-lock recovery.

## Process notes

* Task commits `b8aee67` and `3a7a136` were left intact; review fixes landed as
  `14545c9` and the Copilot fix as `be92c2b`.
* Copilot found one in-scope credential-prefix leak that eight pre-PR reviewers
  missed; fixed, answered, and resolved, after which the P-018 gate returned
  `SATISFIED` with HEAD unchanged.
* CI: both `ubuntu-latest` and `windows-latest` jobs passed on each HEAD.
* `gh pr merge` must not run with `2>&1` in this PowerShell host.

## Open items

* None. Stash `3EB89E2D` (live-tenant validation from `022-S`) and shipment
  `010-S` stay queued for an attended session.

Full closure record: `docs/closure/2026-11-14-025-S-review-and-closure.md`.
