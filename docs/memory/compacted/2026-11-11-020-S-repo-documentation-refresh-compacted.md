---
type: memory-compacted
date: 2026-11-11
shipment: 020-S
feature: 020-F
source:
  - docs/archive/memory/2026-11-11/020-S-repo-documentation-refresh-memory.md
  - docs/archive/memory/2026-11-11/020-S-stage-repo-documentation-refresh-memory.md
---

# Compacted memory - Shipment 020-S (repository documentation refresh and design graduation)

**Shipped**: PR #50, merge `480e7162851583fe684598e50d2c0b2bc0a45f7c`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship --sha`). Deliberation `009-DL` was retained. No Go
source changed.

## Decision and outcome

* `README.md` documents batch queue mode (`--queue` / `-q`, `::` as the only
  comment marker, `writeQueueLines`, `inheritedQueueFlags`), the capture flag and
  filename template reference, the eight-level output target precedence, the
  `018-S` update locking and manual recovery steps, the config key table, and all
  10 `internal/` packages.
* `docs/ARCHITECTURE.md` and `docs/QUALITY_SCORE.md` are new: domain map,
  import-derived dependency direction, runtime flows, security invariants, graded
  packages with measured coverage, five test tiers, and live-verification gaps
  (`010-S`).
* `docs/design-docs/` gained `2026-11-11-self-update-and-interprocess-locking.md`
  (`005-DL`, `007-DL`) and updated the browser caption and recap capture design doc
  for `002-DL` (URL allowlist and redaction) and `006-DL` (sandbox posture).

## Key reusable insights

* Docs follow the code. The task text named `#` queue comments and a golden-MP3
  test line number that the code did not support; the docs record what the code
  does. A review finding can also cite a symbol that does not exist
  (`browser.resolveProfileDir`); map it to the real code and record the
  discrepancy instead of documenting the symbol.
* No executable-adjacent or `bin\profile` profile search exists. The default
  profile is `%LOCALAPPDATA%\teams-cli\profile`, then `os.UserConfigDir()`. An
  empty `UserDataDir` behaves differently per launch path: the chromedp allocator
  uses a temporary profile, while the direct-exec headed launcher omits
  `--user-data-dir` so Chromium's own default profile applies.
* The scheduler busy gate is an `atomic.Bool` in `scheduleJob`
  (`internal/cli/schedule.go`), not in `internal/scheduler`.
* `release.yml` publishes only `windows/amd64` and `windows/arm64`, so
  `teams-cli update` returns `ErrAssetNotFound` elsewhere.
* The secret scan (test and CI step) reads tracked regular text files up to
  `8 MiB`, skips binary files, and ignores synthetic passcode placeholders.
* On this host, run `go test -cover` with `-p 1`; parallel package builds ran out
  of memory.
* Done tasks move to `.backlogit/archive/` while still declaring `status: done`;
  closure must classify them by declared status, not location.

## Review outcome

Local review: `READY`, `P0=0, P1=0` (single-agent declared degradation,
`DEGRADED_SINGLE_AGENT (non-consensus)`). The Orchestrator `rubber-duck` pass
raised six findings, fixed in `6be1121`. Copilot raised one in-scope thread
(empty `UserDataDir` behavior by launch path), fixed in `109008d` (P-021 C1),
replied to, and resolved. The `copilot-review` gate returned `SATISFIED`. CI ran
`Detect code changes` and `Detect CLI-affecting changes` and skipped
`Build, vet, lint & test` as expected for a docs-only PR. The merge used a merge
commit.

## Post-merge closure summary

`backlogit shipment ship` returned `archived_ids: [020.001-T, 020.002-T,
020.003-T, 020-F, 020-S]` and `returned_ids: []`; all three tasks kept
`parent_id: 020-F`. `009-DL` stayed in `.backlogit/queue/` as `queued`, unmoved.
Stash entry `6227A297` was already archived by Stage. Four follow-up stash
entries were created: `70ABBB2C` (POSIX permission test for `internal/capture`),
`A368D60E` (`internal/config` coverage), `D3A9CBD0` (Windows CI job), and
`EC6D1CAF` (non-Windows release assets). Stage or the operator may archive
`009-DL` under the post-closure rule once it has zero live governing references.
`010-S` stays queued and untouched. `stash@{0}` (`operator-option-a-and-scratch`)
was preserved.
