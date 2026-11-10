---
title: "MVP Live Captions Browser Capture Session Memory"
date: "2026-11-08"
branch: "feat/mvp-live-captions"
---

## Summary

Implemented a working MVP in Go on branch `feat/mvp-live-captions` that opens a
browser window, waits for manual user authentication and Teams call entry,
navigates the call interface to enable live captions, and streams finalized
captions in real time to a local Markdown file.

## Files Modified

* `internal/teams/url.go`, `internal/teams/url_test.go`: preserved `/meet/` and
  general Teams URLs without forcing legacy `type=meetup-join` query parameters
* `internal/browser/allocator.go`, `internal/browser/allocator_test.go`: added
  `ExecPath` and `ResolveExecPath` to auto-discover Chrome, Edge (`msedge.exe`),
  Chromium, or Brave across Windows, macOS, and Linux
* `internal/browser/prejoin.go`, `internal/browser/prejoin_test.go`,
  `internal/browser/meeting_chrome.go`: added `PreJoinState` polling that waits
  for user authentication, clicks `joinOnWeb` on launcher interstitials, reads
  the boolean `checked` property on the v2 mute switch, and detects active calls
  via `#hangup-button`
* `internal/browser/captions.go`, `internal/browser/captions_test.go`,
  `internal/transcript/caption.go`, `internal/transcript/caption_test.go`:
  updated live caption menu selectors for both guest and full Teams v2 clients,
  prevented multi-block same-speaker ping-pong duplication via per-element
  tracking, and added settled-utterance ingestion (`IngestSettled`)
* `internal/capture/output.go`, `internal/capture/output_test.go`,
  `internal/capture/session.go`, `internal/capture/session_test.go`: added
  `CaptionsOnly` mode, real-time `Flush()` on every caption write, and `/meet/`
  ID sanitization that strips passcode tokens from filenames
* `internal/config/config.go`, `internal/cli/root.go`, `internal/cli/join.go`,
  `internal/cli/root_test.go`: added `teams-cli captions [meeting-url]`, CLI
  flags (`--output-dir`, `--profile-dir`, `--bot-name`, `--browser-path`,
  `--headed`, `--headless`, `--audio`), and zero-config headed fallback
* `internal/browser/meeting_e2e_test.go`, `internal/cli/e2e_test.go`,
  `README.md`: added end-to-end Chromium/Edge tests and usage documentation

## Verification

* `gofmt -l .`: clean
* `go build ./...`: clean
* `go vet ./...` and `go vet -tags=integration ./...`: clean
* `golangci-lint run --no-config -E misspell ./...`: 0 issues
* `go test -race ./...`: all unit and end-to-end browser tests pass
* `GO_TEAMS_INTEGRATION=1 go test -tags=integration ./internal/browser/ -run TestChromeRunnerLaunch`: pass
