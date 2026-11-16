---
type: compound-learning
category: build-errors
date: 2026-11-16
tags: [cdp, websocket, audio, windows, ephemeral-ports, recap]
evidence: Shipment 031-S live TestMeetingCaptureAudio on 65-minute SharePoint Recap
---

# Chunked CDP Transfer and Persistent WebSocket Session for Large PCM Payloads

## Problem

When capturing 65 minutes (~3,900 seconds) of 44.1 kHz mono `int16` PCM audio
from `AudioContext.decodeAudioData` inside Chromium (`internal/browser/recap.go`),
the base64-encoded JSON array (`pcmChunks`) exceeds 440 MB:

1. Returning a single 440 MB string from `Runtime.evaluate(returnByValue: true)`
   stalls Chromium's V8 inspector / DevTools WebSocket transport for more than
   2m45s (`context deadline exceeded`), even when `decodeAudioData` and PCM
   conversion already finished in 37 seconds.
2. Pulling 80+ chunk batches sequentially via one-shot `ws.Dial` calls per batch
   on Windows triggers `connectex: Only one usage of each socket address
   (protocol/network address/port) is normally permitted` (`WSAEADDRINUSE`) as
   loopback ephemeral ports enter `TIME_WAIT`.

## Root Cause

* Chromium's `Runtime.evaluate` double-escapes and serializes `returnByValue`
  strings inside a monolithic CDP JSON envelope, hitting severe serialization
  latency on multi-hundred-megabyte payloads.
* Windows loopback TCP sockets linger in `TIME_WAIT` when dozens of short-lived
  CDP WebSocket connections are dialed back-to-back to `127.0.0.1:<port>`.

## Solution

1. **Chunked CDP pull (`transferPending`)**: When `totalPCMBytes > 4 MiB`,
   `extractRecapAudioJS` stores `{ chunks: pcmState.pcmChunks }` on
   `window.__goTeamsRecapAudioTransfer` and returns a `< 1 KB` metadata payload
   with `transferPending: true` and `chunkCount`. Small unit/E2E payloads
   (`<= 4 MiB`) continue returning `pcmChunks` inline so single-response mock
   fixtures remain compatible.
2. **Persistent `cdpEvalSession`**: `captureRecapAudioFromTargets` opens one
   WebSocket connection (`newCDPEvalSession`), pulls 64-chunk batches
   (`t.chunks.splice(0, 64)`) with incrementing request IDs, encodes each batch
   directly into `audio.NewEncoder`, returns an error if an empty batch arrives
   before `ChunkCount`, and deletes `window.__goTeamsRecapAudioTransfer` on all
   normal and early-exit paths (`cleanupPendingTransfer`).
