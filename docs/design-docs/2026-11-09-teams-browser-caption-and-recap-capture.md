---
title: Teams browser caption and recap capture
date: 2026-11-09
status: implemented
feature: 011-F
shipment: 011-S
---

## Architecture and dependency direction

`cmd/teams-cli` enters `internal/cli`, which resolves configuration and passes
browser and output settings into `internal/browser` and `internal/capture`.
Capture coordinates the `browser.Meeting` interface and writes through
`internal/transcript`; browser code does not own output files. URL mutation
belongs to `internal/teams`. Configuration persistence remains in
`internal/config`.

## Browser launch and authentication

The capture path discovers Chrome, Edge, or Brave, or uses `--browser-path`
and `TEAMS_BROWSER_PATH`. It launches a browser with a loopback CDP port,
optional user-data directory, and `--headless=new` only when requested.
Clean capture launch omits `AutomationControlled` and `--no-sandbox`.
This does not redefine the separate legacy authentication allocator.

Headed mode supports operator sign-in and meeting admission. Headless mode
requires an already usable profile and authorized access. A zero-config run
prefers an existing profile beside the executable, then working-directory
`bin\profile`; automation should supply `--profile-dir` explicitly.
Browser profiles contain credentials and must never be committed or shared.

## Live caption path

The join flow handles browser launcher pages, pre-join controls, lobby
admission, and active-call detection. Caption activation handles the direct
caption menu and the nested language-and-speech menu.

A JavaScript `MutationObserver` watches Teams caption rows. A settle timer
avoids exporting every incremental rewrite of the same utterance.
CDP binding events enter a bounded Go payload pump, followed by transcript
deduplication and a caption sink. The pump drains accepted events on shutdown;
queue overflow is logged and can drop fragments. Capture flushes each written
caption so output is visible during a live session.

## Meeting Recap path

CDP target discovery inspects both pages and out-of-process iframes, including
`xplatplugins.aspx`. Transcript extraction first seeks the `.ms-List` React
item collection to avoid limiting export to currently rendered virtualized
rows. A DOM fallback reads transcript cells, speaker names, timestamps, and
text. The fallback can be incomplete when virtualization hides entries.

ISO duration or colon-separated recording offsets become `HH:MM:SS` labels.
Empty transcript events are omitted. Recap captions are finite, so this path
exits after writing the extracted entries instead of waiting for cancellation.

## Teams AI summary

The page script selects Notes and AI notes when present, then invokes Copy all.
It intercepts `navigator.clipboard.writeText`, `navigator.clipboard.write`,
and copy-event plain text. This captures Teams-provided text; it does not
request a separately generated AI response.

Because the transcript iframe may load first, summary capture re-queries
`listCDPTargets(port)` on each of at most 20 attempts, separated by 500 ms.
Cancellation stops retry waiting. HTTP enumeration and individual CDP
evaluations retain their existing I/O timeouts, so this is an attempt bound,
not a strict ten-second wall-clock deadline.

If no nonempty summary is captured, the transcript still succeeds. When
available, `WriteAISummary` appends a horizontal divider and summary section.
Tenant policy, localization, and UI changes can prevent summary extraction.

## CLI and output contract

| Input | Meaning |
|---|---|
| `teams-cli "<url>" "<output.md>"` | Direct capture |
| `teams-cli captions "<url>" "<output.md>"` | Live or recap capture |
| `teams-cli join "<url>" "<output.md>"` | Explicit join/capture entrypoint |
| `-u/--url` | Input URL when positional URL is absent |
| `-o/--output` | File for `.md`/`.markdown`, otherwise directory |
| `--output-file` | Explicit transcript path |
| `--output-dir` | Directory regardless of extension |
| `--headless` | Headless clean-browser launch |

Positional inputs override flags. Output flags resolve as `--output-file`
over `--output` over `--output-dir`. Existing directory targets remain
directories. Explicit output selection overrides configured `output_file`.
Parent directories are created as needed.

Explicit file destinations are truncated after admission, including their
same-basename MP3 companion when audio is enabled. Reusing these paths is
destructive to prior results; unique per-run paths are recommended. Generated
names use exclusive creation. Failure rollback is best-effort, not a
transactional backup of replaced files.

## Validation and known limits

Unit checks cover URL/output argument routing, configuration precedence,
recap offsets, launch flags, transcript rendering, and delayed target discovery.
A synthetic browser harness exercises live captions; an opt-in compiled
executable harness verifies all output forms against a local recap fixture
from a different working directory.

These fixtures establish the executable contract, not production Teams DOM
compatibility, transcript completeness, authorization, or audio fidelity.
Live qualification and rollback ownership are defined in the
[closure record](../closure/2026-11-09-011-S-review-and-closure.md).
