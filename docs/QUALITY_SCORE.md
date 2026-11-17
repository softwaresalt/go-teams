---
title: Quality Grades by Domain
description: Honest quality grades, test coverage tiers, and live-verification gaps for each go-teams package
date: 2026-11-16
---

## How to read this document

Grades follow the rule in
[architecture-doc.instructions.md](../.github/instructions/architecture-doc.instructions.md):
a grade is honest or it is useless. Each grade reflects the evidence in this
repository, not the effort spent.

| Grade | Meaning |
|---|---|
| A | Behavior is covered by hermetic tests, error paths and security invariants are exercised, documentation matches the code, and no known verification gap remains for the domain's own responsibility |
| B | Solid tests and documentation, with a named verification gap that the repository cannot close on its own (for example a live tenant) |
| C | Known weaknesses in tests, documentation, or design; flagged for improvement |

Statement coverage in the table below was measured with `go test -cover` on a
Windows development host on 2026-11-12; the `internal/cli` and `internal/config` rows
were re-measured on 2026-11-14 after `024-S`. Treat the numbers as a point-in-time
signal, not a target; CI reports its own figure with
`go test -race -coverprofile=coverage.out ./...`.

## Grades by package

| Domain | Grade | Coverage | Rationale |
|---|---|---|---|
| `internal/version` | A | 97.8% | Pure functions with table tests for SemVer 2.0.0 normalization, precedence, and build metadata; no I/O |
| `internal/teams` | A | 93.8% | Allowlist, userinfo rejection, join URL mutation, and redaction are table-tested, including malformed inputs that must never leak userinfo; the redaction contract is exercised by `internal/cli` queue tests as well |
| `internal/scheduler` | A | 93.5% | Cron and one-shot `at` (`ParseAt`, `BuildWithClock`, `oneShotSchedule`) construction and validation error paths are covered |
| `internal/transcript` | A | 91.2% | Deduplication, ordering, and Markdown sanitization are covered with deterministic fixtures |
| `internal/audio` | A | 87.5% | Encoder output is pinned by a golden digest, and sibling tests parse MPEG-1 Layer III frame headers to count frames. The grade covers the encoder only; real Teams audio fidelity is a browser-domain gap |
| `internal/updater` | A | 84.5% | Fail-closed lock protocol, identity guards, checksum verification, and rollback are tested with injected clocks, process liveness, and rename failures. The Windows rename window is exercised on `windows-latest` in CI (`test-windows`) as well as on Windows development hosts |
| `internal/capture` | A | 84.8% | Output path resolution, filename templates, activity-aware nominal duration/overtime/idle shutdown, call-end early stop, fallback `MeetingTitle`, and idempotent `AppendAISummaryToFile` are covered. `internal/capture/output_unix_test.go` asserts the `0o700` (`dirPerm`) directory and `0o600` (`filePerm`) file creation modes under `syscall.Umask(0)`, the preservation of a pre-existing directory's mode, and `AppendAISummaryToFile` mode preservation, so the owner-only guarantee has a POSIX regression test. CI consolidates Go test execution onto `windows-latest` (`test-windows`), where `//go:build unix` tests do not execute at runtime; they remain type-checked in CI via `GOOS=darwin GOARCH=amd64 go vet ./...` and runnable locally via the `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c` cross-compile recipe |
| `internal/cli` | A | 80.6% | Flag binding, the `generate-config` subcommand (aliases, positional file-versus-directory handling, `--force` collision, and invalid-name rejection, with the root `--config` flag proven unused), capture protocols, Idea 2C delayed AI summary timer + inter-process locked queue fallback, `--not-before` gating, schedule overrides, output precedence, and flag inheritance are covered through the injected `deps`; the compiled-binary harness adds an end-to-end check |
| `internal/config` | A | 81.6% | Load, `TemplateYAML` / `ResolveTemplatePath` / `WriteTemplate` (template parses before and after uncommenting, filename traversal rejection, `O_EXCL` and atomic `--force` writes, owner-only modes, and `config.example.yaml` parity), capture modes, lifecycle durations, per-meeting overrides (`cron` vs `at`), `default_account` (`entra_account` / `account`) and output alias precedence (`output` versus `output_file` versus `output_folder` versus `default_path`), `headless` / `headed` and `audio` pointer combinations, `expandUserPath`, `defaultProfileDir`, atomic `Save`, and validation are table-tested, and the tracked-file secret scan lives here |
| `internal/browser` | B | 71.6% | Synthetic-page and unit tests cover the pre-join machine, lobby timeout (`ErrLobbyTimeout`), `ForceRecap`, PCM RMS voice activity (`PCMRMS`/`HasVoiceActivity`), interim caption heartbeats, call-end probes (`shouldNotifyCallEnded`), recap export, and sandbox flags. The Teams DOM changes without notice, and no automated test runs against a real tenant (queued in `010-S` and `029-S`) |
| `cmd/go-teams` | B | none | Thin `main` with signal wiring and no tests of its own; exercised only through the compiled-binary harness |

Documentation grade for the repository as a whole is B: the README cover page,
the [user guide](USER_GUIDE.md), [ARCHITECTURE.md](ARCHITECTURE.md), and the
graduated design docs describe the shipped behavior, but no per-package GoDoc audit has been done and
`docs/product-specs/` holds two specification documents (the architecture
blueprint and the scheduled live-call capture requirements).

## Test coverage tiers

Verification is layered. Each tier answers a different question, and a tier
that skips on a given host does not prove anything for that host.

### Tier 1: unit and concurrency suite

`go test -race -coverprofile=coverage.out ./...` runs every hermetic test with
the race detector. Both `.github/workflows/ci.yml` and
`.github/workflows/release.yml` run the consolidated quality-gate suite on
`windows-latest` (`test-windows`), covering `go build`, `go vet ./...`,
`go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`,
`golangci-lint run` (`install-mode: goinstall`), the non-skipped
`TestChromeMeetingEndToEnd` browser E2E check, and
`go test -race -coverprofile=coverage.out ./...`. The Windows updater
rename-window tests run on `windows-latest`, while the `//go:build unix` mode
tests in `internal/capture/output_unix_test.go` remain available via
`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/capture`. Run the
suite with `-p 1` on memory-constrained hosts, because parallel package builds
can exhaust the Windows commit limit.

### Tier 2: synthetic browser and compiled-binary end to end

* `TestChromeMeetingEndToEnd` and `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd`
  in `internal/browser/meeting_e2e_test.go` drive a real Chromium against
  simulated Teams single-page apps served over `httptest`. Together they cover
  the sign-in wait, the launcher interstitial, pre-join (mute, bot name, Join),
  lobby admission, the caption menu, the live caption `MutationObserver`
  (including interim activity heartbeats), in-browser `OnCallEnded` DOM teardown
  detection, and `ForceRecap` + `AISummaryOnly` extraction. They fall back to
  gitignored root `.env.local.test` (seeded from `.env.example`) when
  environment variables such as `TEAMS_BROWSER_PATH` are not already set in the
  process environment, and skip unless the host is Windows and a Chromium-based
  browser is installed (`ResolveExecPath("")` must find one). The `test-windows`
  job in `ci.yml` and `release.yml` runs `TestChromeMeetingEndToEnd` in a
  dedicated step that fails if the test skips and runs the full `go test -race ./...`
  suite on `windows-latest`.
* `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd` in
  `internal/cli/e2e_test.go` runs `go-teams join --mode live-with-summary`
  end-to-end across two headless Chromium sessions on Windows (`go test -race ./...`),
  verifying scheduled join, overtime caption continuation past `--duration`,
  in-browser call-end silence shutdown, `transcription_queue.txt` persistence,
  and delayed Recap `--append-ai-summary` enrichment + `::` queue completion.
* `TestCompiledBinaryCapturesRecapToExplicitOutput` in
  `internal/cli/binary_automation_test.go` runs a freshly built `go-teams`
  executable against a synthetic Meeting Recap page and checks the transcript
  and AI Summary output across the root, `captions`, short-flag, `join`,
  output-file, and output-directory invocation forms. It loads gitignored root
  `.env.local.test` (seeded from `.env.example`) when `GO_TEAMS_BINARY` is not
  already set in the process environment, and skips unless `GO_TEAMS_BINARY`
  points at a built executable; CI does not set that variable, so this test
  remains a local check.

### Tier 3: deterministic golden MP3 regression

`TestEncoderDeterministicGoldenReference` in `internal/audio/mp3_test.go`
encodes a fixed PCM signal twice and requires byte-identical output that
matches an inline golden reference: total length, SHA-256 digest, and the
leading and trailing bytes. It avoids zero-slack buffers so it stays
`checkptr`-safe under `go test -race`. Any encoder or dependency change that
alters the bitstream fails this test and forces a deliberate golden update.

### Tier 4: tracked-file secret scan

`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` in
`internal/config/gitignore_test.go` scans every git-tracked regular text file
reported by `git ls-files`, skipping binary files and files larger than
`8 MiB`. It flags tracked local `.env` filenames, Teams passcodes (`?p=` or
`&p=` values on Teams meeting URLs), `SKYPE-TOKEN` JWT assignments, and
`Bearer eyJ...` JWTs. It reads the git index, so new files must be staged with
`git add` before the test can see them. CI runs a separate secret-hygiene step
with the same rules and the same `8 MiB` limit on tracked files, including on
documentation-only pull requests.

### Tier 5: integration-tagged live harness

Tests behind the `integration` build tag (including `meeting_integration_test.go`,
`scheduled_lifecycle_integration_test.go`, and `scheduled_integration_test.go`
for `010-S` and `029-S`) launch a real browser and, with `TEAMS_MEETING_URL`
and `TEAMS_RECAP_URL`, a real meeting or Recap. Test helpers load gitignored
root `.env.local.test` (seeded from `.env.example`) when environment variables
are not already set in the process environment. They run only when
`GO_TEAMS_INTEGRATION` is set (and not `"0"` or `"false"`). CI compiles them
with `go vet -tags=integration ./...` to prevent bit-rot but never runs them.

## Live-verification gaps

These gaps are known and unresolved. Tiers 1 to 4 cannot close them.

| Gap | Impact | Tracking |
|---|---|---|
| No automated run against a real Microsoft Teams tenant: login, MFA, tenant policy, lobby, overtime continuation, call-end teardown, and caption DOM selectors are verified only against synthetic pages | A Teams UI change can break capture without failing any test | `010-S`, `029-S` |
| Real Meeting Recap completeness and delayed Idea 2C AI Summary availability are unverified | Recap export can miss entries or the summary in production | `010-S`, `029-S` |
| Real meeting audio fidelity (levels, sync, long recordings) is unverified; only the encoder is pinned by golden output | MP3 output may differ from the meeting audio in ways tests cannot see | `010-S` |
| The `darwin/amd64` and `windows/arm64` release binaries are cross-compiled on `ubuntu-latest` at release time and never execute a native test suite; only `windows/amd64` (`windows-latest`) runs the native test suite in CI (with `GOOS=darwin GOARCH=amd64 go vet ./...` type-checking `unix`/`darwin` files), and `//go:build unix` permission tests in `internal/capture/output_unix_test.go` do not execute at runtime in CI after `032-S` | Platform-specific regressions on cross-compiled or POSIX-only targets surface only after release or during local `GOOS=linux` cross-compile checks | `023-S`, `032-S` |

## Improvement roadmap

1. Complete `010-S` and `029-S` live-tenant runs to qualify live capture,
   scheduled overtime/call-end shutdown, delayed Recap AI Summary enrichment,
   and audio against an authorized tenant, then upgrade `internal/browser`
   toward A.
2. ~~Add table cases to `internal/config` for alias and precedence interactions
   and raise its coverage above 80%.~~ Completed in `021-S` (83.3% statement
   coverage, Grade A).
3. ~~Add a POSIX-only test that asserts the `0o700` and `0o600` modes created by
   `internal/capture`, which would lift that package to A.~~ Completed in
   `023-S` (`internal/capture/output_unix_test.go`, Grade A).
4. ~~Add a Windows runner to CI so the Windows-only tests stop depending on
   developer hosts.~~ Completed in `023-S` (`test-windows` on `windows-latest`
   in `ci.yml` and `release.yml`).
5. Re-grade this table at each post-merge closure that changes a package's
   behavior or tests.
