---
title: Quality Grades by Domain
description: Honest quality grades, test coverage tiers, and live-verification gaps for each go-teams package
date: 2026-11-11
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
Windows development host on 2026-11-12. Treat the numbers as a point-in-time
signal, not a target; CI reports its own figure with
`go test -race -coverprofile=coverage.out ./...`.

## Grades by package

| Domain | Grade | Coverage | Rationale |
|---|---|---|---|
| `internal/version` | A | 97.8% | Pure functions with table tests for SemVer 2.0.0 normalization, precedence, and build metadata; no I/O |
| `internal/teams` | A | 93.8% | Allowlist, userinfo rejection, join URL mutation, and redaction are table-tested, including malformed inputs that must never leak userinfo; the redaction contract is exercised by `internal/cli` queue tests as well |
| `internal/scheduler` | A | 93.3% | Small surface; cron construction and error paths are covered |
| `internal/transcript` | A | 91.2% | Deduplication, ordering, and Markdown sanitization are covered with deterministic fixtures |
| `internal/audio` | A | 87.5% | Encoder output is pinned by a golden digest, and sibling tests parse MPEG-1 Layer III frame headers to count frames. The grade covers the encoder only; real Teams audio fidelity is a browser-domain gap |
| `internal/updater` | A | 84.5% | Fail-closed lock protocol, identity guards, checksum verification, and rollback are tested with injected clocks, process liveness, and rename failures. The Windows rename window is exercised only on Windows hosts (see gaps) |
| `internal/capture` | B | 81.7% | Output path resolution, filename templates, and collision handling are covered, and the `0o700` / `0o600` creation modes are set by the `dirPerm` and `filePerm` constants. No test asserts the resulting mode bits, so the owner-only guarantee rests on code review rather than a regression test |
| `internal/cli` | A | 82.0% | Flag binding, output precedence, queue parsing, in-place completion marking, and flag inheritance are covered through the injected `deps`; the compiled-binary harness adds an end-to-end check |
| `internal/config` | B | 69.7% | Load, alias handling, and validation are tested and the tracked-file secret scan lives here. Coverage is the lowest of the leaf packages, and the alias interplay (`output` versus `output_file`, `headless` default) is subtle enough that it deserves more table cases |
| `internal/browser` | B | 72.9% | Synthetic-page tests cover the pre-join machine, caption observation, recap export, and sandbox flags. The Teams DOM changes without notice, and no automated test runs against a real tenant (queued in `010-S`) |
| `cmd/teams-cli` | B | none | Thin `main` with signal wiring and no tests of its own; exercised only through the compiled-binary harness |

Documentation grade for the repository as a whole is B: the README,
[ARCHITECTURE.md](ARCHITECTURE.md), and the graduated design docs describe the
shipped behavior, but no per-package GoDoc audit has been done and
`docs/product-specs/` still holds a single blueprint document.

## Test coverage tiers

Verification is layered. Each tier answers a different question, and a tier
that skips on a given host does not prove anything for that host.

### Tier 1: unit and concurrency suite

`go test -race ./...` runs every hermetic test with the race detector. This is
the CI gate (`.github/workflows/ci.yml`) alongside `go vet ./...`,
`go vet -tags=integration ./...`, and `golangci-lint run`. Run it with
`-p 1` on memory-constrained hosts, because parallel package builds can exhaust
the Windows commit limit.

### Tier 2: synthetic browser and compiled-binary end to end

* `TestChromeMeetingEndToEnd` in `internal/browser/meeting_e2e_test.go:25`
  drives a real Chromium against a simulated Teams single-page app served over
  `httptest`. It covers the sign-in wait, the launcher interstitial, pre-join
  (mute, bot name, Join), lobby admission, the caption menu, and the live
  caption `MutationObserver`. It skips unless the host is Windows and a
  Chromium-based browser is installed.
* `TestCompiledBinaryCapturesRecapToExplicitOutput` in
  `internal/cli/binary_automation_test.go:16` runs a freshly built
  `teams-cli` executable against a synthetic Meeting Recap page and checks the
  transcript and AI Summary output across the root, `captions`, short-flag,
  `join`, output-file, and output-directory invocation forms. It skips unless
  `GO_TEAMS_BINARY` points at a built executable.

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

Tests behind the `integration` build tag launch a real browser and, with
`TEAMS_MEETING_URL`, a real meeting. They run only when `GO_TEAMS_INTEGRATION`
is set. CI compiles them with `go vet -tags=integration ./...` to prevent
bit-rot but never runs them.

## Live-verification gaps

These gaps are known and unresolved. Tiers 1 to 4 cannot close them.

| Gap | Impact | Tracking |
|---|---|---|
| No automated run against a real Microsoft Teams tenant: login, MFA, tenant policy, lobby, and caption DOM selectors are verified only against synthetic pages | A Teams UI change can break capture without failing any test | `010-S` |
| Real Meeting Recap completeness and AI Summary availability are unverified | Recap export can miss entries or the summary in production | `010-S` |
| Real meeting audio fidelity (levels, sync, long recordings) is unverified; only the encoder is pinned by golden output | MP3 output may differ from the meeting audio in ways tests cannot see | `010-S` |
| CI runs on `ubuntu-latest` only, and `TestChromeMeetingEndToEnd` skips on non-Windows hosts | The Windows browser path and the Windows `.old` rename window in `internal/updater` are exercised only on developer machines | Open; consider a Windows CI job |
| No test asserts the `0o700` directory and `0o600` file modes that `internal/capture` requests; the constants are the only evidence, and Windows ACLs do not map to POSIX mode bits | The owner-only output guarantee is unproven by tests | Open; add a POSIX-only mode assertion test |

## Improvement roadmap

1. Complete `010-S` to qualify live capture, recap, and audio against an
   authorized tenant, then upgrade `internal/browser` toward A.
2. Add table cases to `internal/config` for alias and precedence interactions
   and raise its coverage above 80%.
3. Add a POSIX-only test that asserts the `0o700` and `0o600` modes created by
   `internal/capture`, which would lift that package to A.
4. Add a Windows runner to CI so the Windows-only tests stop depending on
   developer hosts.
5. Re-grade this table at each post-merge closure that changes a package's
   behavior or tests.
