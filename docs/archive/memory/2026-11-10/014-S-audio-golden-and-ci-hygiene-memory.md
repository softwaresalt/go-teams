---
title: Ship session memory for 014-S audio golden test and CI hygiene
date: 2026-11-10
shipment: 014-S
feature: 014-F
agent: ship
status: in-progress
---

## Completed

* `014.001-T`: `TestEncoderDeterministicGoldenReference` and helpers in
  `internal/audio/mp3_test.go`, commit `e98057d9497cea91c70b89cf854f5fd28cac5bee`.
* `014.002-T`: `.github/workflows/ci.yml` trigger and secret-scan change, commit
  `0cce11c5afb2fb272139c066be4d70d49d53f6f9`.

## Decisions and rationale

* Golden PCM is integer-only (triangle plus LCG noise) instead of `sineMono`, so
  the pinned bytes do not depend on `math.Sin` or floating-point fusion on other
  architectures. The plan named `sineMono`, but the plan's own risk section
  called out cross-platform float determinism.
* `Encoder` has no `Close`; the test uses `WritePCM` chunks plus `Flush`.
* The CI scan is an inline `python3` heredoc because the task is bounded to one
  file. It mirrors the rules and synthetic markers of
  `internal/config/gitignore_test.go` and prints rule and path only.
* Commit scopes use `config` for the workflow change because `ci` is not an
  allowed scope.
* `compaction_status` starts `pending` in the closure artifact and is set to
  `done` only after P-020 compaction runs post-merge.

## Environment notes

* Backlogit moved done tasks to `.backlogit/archive/` immediately, so the
  manifest tasks are pre-archived before safe close. This is a move, not a
  deletion.
* Pinned lint binary: `bin/lint-v1/golangci-lint.exe` (v1.64.8) with
  `GOTOOLCHAIN=go1.26.0`.

## Failed approaches

None.

## Next steps

* Push the branch, open the feature PR, wait for CI and Copilot review, merge
  with a merge commit.
* Run post-merge closure on `post-merge/014-deterministic-mp3-golden-regression-test-and-ci-path-filter-trigger-hygiene`.
