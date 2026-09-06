---
type: closure
date: 2026-09-05
shipment: 009-S
feature: 009-F
pr: 23
post_merge_pr: 24
merge_commit: 3d59b7d4204a788ee6e6d119db97751c24149c60
archive_commit: 12f0dc5cc36cee6cb3943e169768bb5ae072d59e
pre_fix_commit: 822915c
status: shipped
closure_status: READY
compaction_status: done
---

# Post-Merge Closure — Shipment 009-S: shine-mp3 -race safety via buffer slack

## Backfill note

This artifact is written retroactively. Shipment 009-S merged (PR #23, commit
`3d59b7d`) and was archived via the post-merge branch/PR flow (PR #24, commit
`12f0dc5`) on 2026-07-11/07-12, but the operational-closure artifact required
by the `pipeline-topology` predecessor-closure check
(`docs/closure/009-S-*-post-merge-closure.md` with a satisfied
`compaction_status`/`closure_status` frontmatter contract) was never produced
in that pass — only `docs/closure/2026-07-11-009-S-review.md` (pre-merge
adversarial review) and the archival commit existed. This backfill supplies
the missing releasability/closure evidence without altering any already-merged
history, code, or backlog state. No implementation work, no new shipment
claim, and no scope beyond 009-S is performed here.

**Revision note (2026-09-06)**: the first version of this artifact contained
five inaccuracies caught by Copilot review on PR #28 (unsupported
`closure_status: READY`, an unsafe/incorrect rollback command, an
oversold byte-identity claim, an inaccurate deployment statement, and an
incomplete CI baseline). This revision corrects all five with verified
evidence (commit diffs and CI run history) rather than restating the
original claims. See the corrected sections below.

## Released scope

Feature `009-F` ("Make shine-mp3 encoding -race safe without forking") and
task `009.001-T` ("Add encoder tail-slack capacity and re-enable -race
tests"), merged via PR #23 as merge commit `3d59b7d`. Resolved stash
`3E55DF0D`. Backlog archival completed via PR #24 (commit `12f0dc5`); verified
still intact at backfill time (`.backlogit/archive/009-S.md`,
`009-F.md`, `009.001-T.md` all present, `archived_status: shipped`/`done`,
no orphaned `.backlogit/queue/009*` entries, no working-tree deletions under
`.backlogit/archive/`).

## What shipped

- `internal/audio/mp3.go` (`encodeFrames`) — the interleaved PCM buffer is now
  allocated as `make([]int16, len(mono)*2, len(mono)*2+encoderTailSlack)`
  (`encoderTailSlack = 4`): **length unchanged, capacity only**, so
  shine-mp3's `windowFilterSubband` end-of-buffer read pointer (which advances
  one stride past the final sample without dereferencing it) stays inside the
  Go-allocation boundary.
- Removed the `raceDetectorEnabled` skip guards (in `internal/audio/mp3_test.go`)
  and the `internal/audio/race.go`/`norace.go` build-tag files; the encoder
  test suite now runs under `go test -race` unconditionally.

## Invariants to preserve

- `encoderTailSlack` must stay `>= 2` (proven minimum) for the fix to remain
  valid; the pinned rationale is version-scoped to shine-mp3 v0.1.0 per the
  009-S review's P2 resolution.
- The change is capacity-only (`len(mono)*2` is unchanged) and shine-mp3's
  `Write` loops on `len(data)`, never `cap(data)` — this is the *source-level*
  argument for why output is unaffected (see Validator evidence for what is
  and is not actually asserted by tests).
- The re-enabled `go test -race ./internal/audio/...` is the upgrade/regression
  gate — a shine-mp3 upgrade that moves the pointer further must fault this
  test, not silently reintroduce a checkptr fault.

## Validator evidence

No runtime-surface (live meeting / browser DOM) behavior changed in this
shipment — the fix is internal to Go-level buffer capacity, so
`runtime-verification` (live Teams DOM/audio probes) was correctly not
invoked for 009-S itself. Live end-to-end audio verification against a real
meeting remains tracked separately under stash `CC0A7034` / shipment `010-S`
(queued, out of scope for this closure).

**Byte-identity — corrected claim (Copilot finding, 2026-09-06)**: the
original text asserted output was "verified byte-identical by unit tests".
That is not accurate. What actually holds:

- *Source-level argument (not a test)*: the change only grows `cap(stereo)`,
  never `len(stereo)`; shine-mp3's `Write` iterates `len(data)`, never
  `cap(data)`, so the extra capacity is never read — by construction, no
  additional/different bytes reach the encoder.
- *What the tests actually assert*: `TestEncoderEncodesAllSamples` checks the
  encoded **frame count** matches the input sample count (no samples
  silently dropped); `TestEncoderProducesMP3Frames`,
  `TestEncoderHandlesNonFrameAlignedInput`, and
  `TestEncoderFlushEmitsRemainder` check buffer emission and frame-alignment
  behavior. **None of these perform a byte-for-byte / fixed-reference
  comparison of encoder output.** A future byte-equality regression test
  would need to be added to make "byte-identical" a tested (not just
  source-level) guarantee — tracked as a follow-up below, not claimed as
  already covered.

Pre-merge gates recorded in `docs/closure/2026-07-11-009-S-review.md`
(outcome `READY`, `P0=0, P1=0`):

| Check | Result | Evidence |
|---|---|---|
| `go build ./...` | pass | Local (review artifact) |
| `go vet ./...` (+`-tags=integration`) | pass | Local (review artifact) |
| `golangci-lint run` | pass (0 issues) | Local (review artifact) |
| `gofmt -l .` | clean | Local (review artifact) |
| `go test -race ./...` | pass | **CI-verified**: GitHub Actions run `29182178078` (`pull_request` event, head `b715b45`, the actual PR #23 tip) — job `Build, vet, lint & test` succeeded, which runs `go test -race -coverprofile=coverage.out ./...` per `.github/workflows/ci.yml` |
| Adversarial review (2 cross-model reviewers) | 0 P0/P1; 2 P2 + 1 P3 resolved (doc/durability only) | `docs/closure/2026-07-11-009-S-review.md` |

Re-verified at backfill time (2026-09-06, no code touched): `go vet ./...`
passes clean on `main` at current HEAD.

## Pre-deploy audits

- No feature flags, migrations, or rollout gates apply — this is a pure
  library-internal encoding fix with no config/schema surface.
- No cross-service boundary is affected (single Go binary, `internal/audio`
  package only).
- Rollback path: source-only targeted revert (see corrected procedure below);
  no data migration to reverse.

## Deployment / rollout path

**Corrected (Copilot finding, 2026-09-06)**: the original text said "shipped
to `main`" without distinguishing merge from distribution. This repository
has a separate tag-triggered release workflow
(`.github/workflows/release.yml`, triggers on `v*.*.*` tag pushes) that
builds and publishes binaries. As of this backfill, **no git tag and no
GitHub release exist in this repository at all** (`git tag --list` and
`gh release list` both empty) — no shipment, including 009-S, has been
distributed to end users via a tagged release yet.

Accurate statement: the change **merged to `main`** at commit `3d59b7d`
(2026-07-12) and has been present in every subsequent commit on `main` since.
It has **not** been distributed via a tagged release/binary, because no
tagged release of this repository has been cut yet.

## Post-deploy checks

- `go test -race ./internal/audio/...` continues to pass on `main` at HEAD
  post-merge (re-enabled by this change) — confirms the checkptr fault does
  not resurface. See Monitoring plan for the corrected CI-run evidence.
- No additional smoke check applies; there is no tagged release/deployment to
  smoke-test yet (see Deployment / rollout path above).

## Risky action record

None. No `ProposedAction`/approval-gated risky action was required — the
change is additive capacity-only and does not modify shine-mp3 itself
(rejected as too risky due to its `NOASSERTION` license; see the pre-merge
review's decision rationale).

## Healthy signal

`go test -race ./internal/audio/...` continues to pass on every subsequent
change to `internal/audio` or on any shine-mp3 dependency bump, as part of
the repository's standard `go test -race -coverprofile=coverage.out ./...`
CI step.

## Failure signal

A `-race` checkptr fault reappears in `internal/audio` tests (indicates a
shine-mp3 upgrade moved the internal read pointer further than the current
`encoderTailSlack = 4` margin covers), or a *future* byte-for-byte
fixed-reference regression test (not present today — see the byte-identity
correction above) fails.

## Monitoring plan

This is a CLI tool repository with no live dashboards/alerting/APM system.
Per `release-observability` guidance for a workspace without a monitoring
system, the monitoring plan is recorded as a manual, CI-driven checklist
rather than a dashboard.

**Corrected CI baseline (Copilot finding, 2026-09-06)**: the original text
claimed a continuously green baseline "since PR #23 merged 2026-07-12". That
is inaccurate. The actual CI history for `internal/audio` around this
shipment:

| Run | Event | Head | Result |
|---|---|---|---|
| `29182178078` | `pull_request` (PR #23 tip) | `b715b45` | **success** — `Build, vet, lint & test` (incl. `go test -race ./...`) passed before merge |
| `29182283079` | `push` to `main` (the merge itself) | `3d59b7d` | **failure** — but the failure is in the `Detect code changes` job's `dorny/paths-filter` step (a base-ref resolution bug on push events, unrelated to `internal/audio`); the `Build, vet, lint & test` job was **skipped**, not failed — no `-race` result was produced by this run at all |
| (gap: 2026-07-12 → 2026-09-04) | `push` to `main` | various | Every push-triggered CI run in this window failed the same way (paths-filter base-ref bug), so `-race` was **not CI-re-verified on `main`** during this gap — only the pre-merge `pull_request` run above provides CI evidence for that period |
| `33923304738` | `push` to `main` | `06cfece` | failure (pre-fix) |
| `33924752132` | `push` to `main` (merge of PR #26, the paths-filter fix, commit `59d57ad`) | `b910c71` | **success** — first push-to-`main` run since the merge to actually execute and pass `Build, vet, lint & test` (incl. `-race`) covering the current `internal/audio` code |

Net: the merge-time evidence for `-race` health is the pre-merge `pull_request`
run (`29182178078`), not the immediately-following push run (which never
executed the test job). Continuous push-to-`main` CI verification of
`-race` resumed 2026-09-04 (`33924752132`) after the unrelated paths-filter
fix, and has covered `internal/audio` since.

- **SLI / key metric**: CI result of `go test -race ./internal/audio/...`
  (via the `Build, vet, lint & test` job) on every push/PR touching
  `internal/audio` or `go.mod` (shine-mp3 version).
- **Dashboard / query**: GitHub Actions `Build, vet, lint & test` workflow run
  status for `main` and PRs touching `internal/audio/**`.
- **Baseline**: green as of `33924752132` (2026-09-04) and every
  `internal/audio`-affecting `pull_request` run since; the 2026-07-12 →
  2026-09-04 push-to-`main` gap above produced no CI signal either way for
  this package (unrelated infra bug, not a code regression).
- **Alert threshold**: any red `-race` run touching `internal/audio` or a
  shine-mp3 version bump in `go.mod`.
- **Owner**: Ship agent / repository maintainer reviewing the next PR that
  touches `internal/audio` or bumps shine-mp3.

## Rollback trigger

A `-race` checkptr fault or an MP3-output regression attributable to the
`encoderTailSlack` change, surfacing on `main` or in a subsequent PR's CI run.

## Rollback procedure

**Corrected (Copilot finding, 2026-09-06)**: the original text specified
`git revert 3d59b7d4204a788ee6e6d119db97751c24149c60`. This does not work as
written and would be unsafe even with a fix:

1. `3d59b7d` is a **merge commit** with two parents; `git revert` on a merge
   commit requires `-m <parent-number>` or it fails outright
   ("commit ... is a merge but no -m option was given").
2. Even `git revert -m 1 3d59b7d4204a788ee6e6d119db97751c24149c60` would
   revert **every** file PR #23 touched, not just the audio implementation:
   `.backlogit/archive/009-F.md`, `009.001-T.md`, `stash.jsonl`,
   `.backlogit/hooks_queue.jsonl`, `.backlogit/queue/009-S.md`,
   `docs/closure/2026-07-11-009-S-review.md`, and two
   `docs/compound/build-errors/*.md` files, in addition to
   `internal/audio/mp3.go` / `mp3_test.go`. Backlog/archive state has moved
   on substantially since (many further shipments archived through 2026-09),
   so reverting those backlog files now would corrupt current backlog state
   rather than cleanly undo the audio fix.

**Source-only targeted rollback** (the correct procedure): restore only the
audio implementation files to their pre-fix state at commit `822915c`
(`3d59b7d`'s first parent — the tip of `main` immediately before this
shipment merged), leaving all backlog/doc state untouched.

**Precondition (Copilot finding, 2026-09-06)**: the checkout commands below
overwrite tracked paths, and repository policy classifies an overwrite
without a clean-worktree guard or explicit approval as destructive. This
procedure MUST NOT be run against a dirty worktree and MUST NOT be run
without explicit operator approval of the specific rollback decision — both
are enforced as literal steps below, not left implicit in prose:

```bash
git status --short                    # MUST print nothing; if it prints
                                        # anything, STOP and resolve/stash the
                                        # uncommitted changes first -- do not
                                        # overwrite a dirty worktree
# STOP: do not run the checkout commands below without explicit operator
# approval of *this* rollback decision. This is a destructive overwrite of
# internal/audio/* and is never auto-executable by an agent.
git checkout 822915c -- internal/audio/mp3.go internal/audio/mp3_test.go
git checkout 822915c -- internal/audio/race.go internal/audio/norace.go
go build ./...
go vet ./...
go test ./internal/audio/...          # non-race: actually exercises encoder-output tests
go test -race ./internal/audio/...    # race: confirms the restored skip guards suppress checkptr
git add internal/audio/
git commit -m "revert(audio): restore pre-009-S encoder (see Rollback trigger above for the observed regression), ref docs/compound/build-errors/shine-mp3-race-checkptr.md"
```

**Corrected (Copilot finding, 2026-09-06)**: the restored pre-fix
`mp3_test.go` re-adds `if raceDetectorEnabled { t.Skip(...) }` guards on every
encoder-output test, so a `-race`-only validation run **skips all of them**
and proves nothing about MP3 output after rollback — it only confirms the
checkptr fault is suppressed again. The corrected validation runs **both**:
`go test ./internal/audio/...` (non-race — actually exercises
`TestEncoderProducesMP3Frames`, `TestEncoderEncodesAllSamples`,
`TestEncoderHandlesNonFrameAlignedInput`, `TestEncoderFlushEmitsRemainder`,
i.e. the real output-regression trigger from the Rollback trigger section
above) **and** `go test -race ./internal/audio/...` (race — confirms the
restored skip guards are in effect, i.e. the true pre-fix configuration) —
plus `go build ./...` / `go vet ./...` for the whole module before pushing.

## Validation window

Continuous (CI-gated) — see the corrected CI-run table under Monitoring
plan. No incident has been observed as of this backfill (2026-09-06).

## Owner

Ship agent, on behalf of the repository maintainer, for the CI-gated
validation window above.

## Source artifact cleanup

Already completed in PR #24 (2026-07-11/12): `.backlogit/archive/009-S.md`,
`009-F.md`, and `009.001-T.md` are present with `archived_status:
shipped`/`done`. Stash `3E55DF0D` (the source of this shipment) was resolved
at shipment time; no separate `source_stash_id`/`source_deliberation_id`
cleanup remained outstanding at backfill time. Re-verified clean at backfill:
no `.backlogit/queue/009*` entries, `git status -- ".backlogit/archive/"`
reports no deletions.

## Releasability evidence

| Requirement | Status | Evidence |
|---|---|---|
| Pre-merge review readiness | Satisfied | `docs/closure/2026-07-11-009-S-review.md` — `READY`, P0=0/P1=0 |
| Build/lint/test gates | Satisfied | CI run `29182178078` (PR #23 tip, `pull_request` event) — `Build, vet, lint & test` passed, incl. `go test -race ./...`; re-verified `go vet ./...` clean at backfill |
| Runtime verification | Not applicable | No runtime-surface change; see Validator evidence for the corrected (source-level, not test-asserted) byte-identity argument |
| Deployment/distribution | Not applicable — merged only | No tag/release exists for this repository yet; see Deployment / rollout path |
| Backlog archival | Satisfied | PR #24, commit `12f0dc5`; re-verified intact at backfill |
| Monitoring plan | Satisfied (manual/CI checklist) | See corrected Monitoring plan above (CI-run table) |
| Rollback plan | Satisfied | Source-only targeted procedure above (not a raw merge-commit revert) |
| P-020 compaction | Satisfied | `compaction_status: done`; `docs/memory/compacted/2026-07-11-009-S-mp3-race-safety-compacted.md` |

**Readiness: `READY`.** The five items above were factually corrected on
2026-09-06 following Copilot review; none of the corrections change the
underlying disposition — the code has been on `main` since 2026-07-12,
CI-verified `-race`-clean both before merge and continuously since
2026-09-04, and now has an accurate, executable rollback procedure. Of the
four items tracked under Remaining follow-ups below, the live-verification
follow-up (`CC0A7034`) is intentionally NOT a condition of *this* shipment's
releasability — it was scoped out of 009-S at plan time and is tracked under
its own shipment (`010-S`, queued, untouched by this backfill). The other
three (CI `paths-ignore` tuning, `D3EFD224`; shine-mp3 upstream fix;
`FE245033`) are likewise non-blocking residual items, not conditions of this
READY disposition.

## Remaining follow-ups (not blocking)

- Live end-to-end audio verification against a real Teams meeting — tracked
  under stash `CC0A7034` / shipment `010-S` (queued; out of scope here).
- **Added (Copilot finding, 2026-09-06)**: CI `pull_request` trigger
  `paths-ignore` tuning — tracked under stash `D3EFD224` (low priority,
  parked). Carried forward unchanged from the original 009-S review
  (`docs/closure/2026-07-11-009-S-review.md:48`) and the compacted memory
  (`docs/memory/compacted/2026-07-11-009-S-mp3-race-safety-compacted.md`);
  omitted from this artifact's first revision by oversight, not by
  disposition change. Not blocking: `required_status_checks=[]` today, so
  docs/backlog-only PRs merge clean without this fix; only becomes relevant
  if required checks are re-added (see the stash entry for the full
  conditional-fix rationale).
- shine-mp3 `unsafe`/`-race` upstream fix — tracked in
  `docs/compound/build-errors/shine-mp3-race-checkptr.md`.
- **New (from this backfill's corrections; captured per P-021 C2)**: no test
  in `internal/audio` performs a byte-for-byte / fixed-reference comparison
  of MP3 encoder output; the "byte-identical" property is currently a
  source-level argument only, not a tested one. Adding such a regression
  test would let a future shine-mp3 upgrade be verified byte-identical
  rather than merely frame-count-identical. This is out of scope for PR #28
  (which authorizes only the 009-S closure backfill, not new
  `internal/audio` test coverage) per P-021 C1, so it was captured as
  deferred-scope-expansion stash entry **`FE245033`** (six-field payload:
  expansion statement, C1 out-of-scope rationale, source refs — PR #28,
  review-thread `PRRT_kwDOTStvas6fpj0Y`, task `009.001-T`, feature `009-F`,
  shipment `009-S`, kind `task`, provisional priority `low`) rather than
  merely noted in prose, per P-021 C2/C3.
  **Known data-quality residual (Copilot finding, 2026-09-06, not fixed by
  Ship)**: the captured entry's `requires deliberation` field reads `NO`.
  Per P-021 C6, every `DEFERRED SCOPE EXPANSION` entry must route through
  the `deliberate` skill before planning regardless of apparent triviality,
  so this field should read `YES`. Ship cannot correct it: P-021 C5 grants
  Ship entry **creation only** for these captures — "Ship MUST NOT ...
  edit ... a captured entry afterwards" — so amending the field would
  itself be a P-021/P-010 violation. In practice this does not create a
  bypass risk: C6's routing requirement is unconditional and does not
  defer to this self-reported flag, so Stage's intake still routes
  `FE245033` through `deliberate` regardless of its stated value. Flagged
  here so Stage's intake triage corrects the field (Stage, unlike Ship, may
  edit stash entries) when it processes `FE245033`.
