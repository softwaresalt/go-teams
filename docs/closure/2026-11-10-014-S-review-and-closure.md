---
title: Deterministic MP3 golden regression test and CI path-filter trigger hygiene review and operational closure
date: 2026-11-10
feature: 014-F
shipment: 014-S
branch: feat/014-s-deterministic-mp3-golden-regression-test-and-ci-path-filter-trigger-hygiene
status: ready
closure_status: READY
compaction_status: pending
merge_commit: pending
pr: pending
---

## Release scope

Shipment `014-S` (`014-F`, `014.001-T`, and `014.002-T`) turns two maintenance
items into automated guarantees:

* `014.001-T`: `TestEncoderDeterministicGoldenReference` in
  `internal/audio/mp3_test.go` synthesizes three whole frames of mono PCM with
  integer-only arithmetic (a triangle wave plus linear-congruential noise, so no
  floating-point variance across architectures), encodes it through two
  independent `NewEncoder` runs using uneven 1000-sample chunks plus `Flush`,
  and asserts byte-for-byte equality. It also pins the output against a golden
  reference: 1252 total bytes, SHA-256
  `8f48238a04612c14851b93c7cd0e56f29d91609c7090f1af269d8e23d25288b6`, and the
  leading and trailing 16 bytes. The test never builds a zero-slack
  (`cap == len`) stereo buffer, so it stays `checkptr`-safe under
  `go test -race`.
* `014.002-T`: `.github/workflows/ci.yml` drops `paths-ignore` from the `push`
  and `pull_request` triggers so `Detect code changes` (`changes`) runs and
  reports on every push and pull request, including docs and backlog-only
  changes. The `code` filter gains `'!.autoharness/**'`, and an unconditional
  `Secret hygiene check (tracked files)` step in the `changes` job scans
  `git ls-files -z` for tracked local env files (other than `.env.example`),
  live Teams passcode URLs, `SKYPE-TOKEN` credential values, and bearer JWTs.
  `test` (`Build, vet, lint & test`) stays gated on
  `needs.changes.outputs.code == 'true'`.

Source stash entries `FE245033` and `D3EFD224` (plus the `3084A83E` CI
companion) fed this scope under deliberation `003-DL`. Shipment `010-S` and
stash entries `DF1F461F` and `FC763C17` are out of scope and untouched.

## Review

Single-agent declared degradation (`TOOL_DEGRADED: subagent-dispatch`): the
multi-persona review rubric ran in-session against `git diff main...HEAD`,
including `ci-security.instructions.md` and `workflows.instructions.md` for the
workflow change.

| Area | Finding | Disposition |
|---|---|---|
| Correctness | Golden bytes are produced from integer-only PCM and the pure-Go shine-mp3 integer pipeline; three repeated `-race` runs matched the pinned values | No action |
| Correctness | The red phase failed only on placeholder constants, then passed once the verified values were pinned | No action |
| Security | Action SHA pins, `persist-credentials: false`, and `contents: read` permissions are unchanged; the new step uses no secrets and no `${{ }}` expression interpolation inside the script | No action |
| Security | The scan reports rule and path only (path passed through `repr`), so neither the matched secret nor a crafted path can reach the log or inject workflow commands | No action |
| Correctness | The step ran locally against 13 cases: tracked `.env` and `*.env` files, live passcode, `SKYPE-TOKEN` value, and bearer JWT fail; `.env.example`, synthetic or short passcodes, bare `SKYPE-TOKEN` key names, the word "Bearer", binary files, non-ASCII paths pass; an empty file list fails closed. The real repository (320 tracked files) exits 0 | No action |
| Maintainability | The credential rules now exist in the Go guardrail test and in the CI step, so a future rule change must touch both | Advisory P3 residual risk; see Follow-ups |
| Security | Copilot round 1 (thread `PRRT_kwDOTStvas62BtFt`): the CI passcode allowlist used `tok` and `12345678`, weaker than the Go guardrail markers, so docs-only PRs could miss credential-shaped URLs | Fixed in-scope (P-021 C1: completes the authorized rule mirroring); markers now identical to `syntheticPasscodeMarkers`, and both example passcodes are rejected locally |
| Scope | Both tasks stay inside their declared single file; no source, schema, or dependency change | No action |

Blocking findings: `P0=0, P1=0, P2=0, P3=1`. Outcome: `READY_WITH_FOLLOWUPS`.

## Follow-ups

* Residual-risk note (advisory P3): the CI secret-scan rules mirror
  `internal/config/gitignore_test.go`. Keep the two in sync when a rule changes.
  No stash entry is created because the duplication is the planned shape of
  `014.002-T`, which is constrained to a single workflow file.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Red then green | `TestEncoderDeterministicGoldenReference` failed on placeholder constants, then passed with pinned values under `go test -race -count=3 ./internal/audio/` | PASS |
| Format | `gofmt -l cmd internal` (0 files listed) | PASS |
| Static analysis | `go vet ./...` | PASS |
| Integration compilation | `go vet -tags=integration ./...` | PASS |
| Lint | `golangci-lint run`, CI-pinned v1.64.8 with `GOTOOLCHAIN=go1.26.0` | PASS |
| Race suite | `go test -race ./...` (all packages) | PASS |
| Full build | `go build ./...` and `go build -o .\bin\teams-cli.exe .\cmd\teams-cli` | PASS |
| Workflow structure | `ci.yml` parsed with a YAML loader: no `paths-ignore`, secret-hygiene step has no `if:`, `test` keeps its `code == 'true'` gate | PASS |
| Secret-scan script | Extracted step run on the repository and on 13 synthetic positive and negative repositories | PASS |
| Backlog integrity | `backlogit sync` and `backlogit doctor` | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Remove `paths-ignore` from `push` and `pull_request`, add `'!.autoharness/**'` to the code filter, and add an always-run tracked-file secret scan to `changes` | low | SHA pins, `persist-credentials: false`, least-privilege permissions, and the `test` job gate are preserved; the scan prints no secret values | applied |
| Add a deterministic golden-reference test for MP3 output | low | Test-only change; no production code touched | applied |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Golden MP3 regression test | READY | Any deliberate encoder or shine-mp3 change that alters output bytes must regenerate the pinned constants in the same change |
| CI trigger hygiene | READY | The `changes` job now runs on every push and pull request; a docs-only change runs only the cheap `changes` job and skips `test` |
| Secret hygiene in CI | READY | A failure of `Secret hygiene check (tracked files)` blocks merge on all changes |

## Monitoring and observation window

Owner: operator merging to `main`.
Duration: the next three pull requests after merge, at least one of which is
docs or backlog-only.

| SLI | Observation source | Baseline | Alert threshold |
|---|---|---|---|
| `Detect code changes` reports on docs-only PRs | GitHub Actions checks tab | Check appears and passes | Check missing on a docs-only PR |
| `Build, vet, lint & test` skipped on docs-only PRs | GitHub Actions checks tab | Job shows as skipped | Job runs on a docs-only PR |
| Secret hygiene false positives | `Secret hygiene check (tracked files)` log | `no findings` | Failure on a change that adds no credential-shaped content |
| Golden test stability | `go test -race` in the `test` job | Passes on every run | Any mismatch not caused by a deliberate encoder change |

## Rollback

Trigger rollback on secret-scan false positives that block unrelated work, or
on a golden-test mismatch that is not caused by an encoder change. Revert the
`014-S` merge commit on `main`. No runtime artifact or data migration is
involved.

## Handoff

Related [decision](../decisions/2026-11-10-audio-golden-test-and-ci-hygiene-deliberation.md)
and [plan](../plans/2026-11-10-audio-golden-test-and-ci-hygiene-plan.md).
