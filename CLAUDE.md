# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

A small Go CLI that synthesizes Subnautica PDA-style voice alerts. It calls Streamlabs' free Polly proxy to get TTS audio, decodes the MP3, applies a modulation chain to give the voice its characteristic "PDA" timbre, then plays it through the speakers. (The Polly call is the TTS engine — it is the only remaining Streamlabs dependency; there is no Streamlabs socket/event integration anymore.)

## Commands

Build the binary (output: `pda.exe` on Windows):
```
go build -o pda.exe
```

Run directly. There are no flags:
```
go run . "All systems nominal"   # speak that text through the speakers, then exit
go run .                         # no arguments: listen mode — HTTP TTS server on :8787
```

In listen mode, POST a plaintext body to `http://localhost:8787/tts` to have it spoken.

There are no tests, lint config, or other tooling in this repo.

## Architecture

`main` (`main.go`) dispatches on argument presence only: any args → `speak` the joined text; no args → `runServer` (listen mode). The synthesis pipeline is linear, each step a top-level function:

1. **`requestPolly`** — POSTs to `https://streamlabs.com/polly/speak` with the `Amy` voice. The endpoint requires the `Referer: https://streamlabs.com` header or it rejects the request. Returns a `speak_url` pointing at the generated MP3.
2. **`httpGet` → `decodeMP3`** — `github.com/hajimehoshi/go-mp3` always emits 16-bit signed LE stereo; `decodeMP3` averages the two channels into mono float64 in `[-1, 1]`.
3. **`process`** — the sound design, applied in order:
   - Comb filter with 5 ms delay (`y[n] = x[n] - x[n-d]`) — produces the metallic notch.
   - Single-pole 250 Hz highpass — thins out low end.
   - Short slap echo: 2 ms delay at 0.75 gain.
   - Clip guard rescales to 0.99 if the peak exceeds 1.0.
   Delay lengths are in milliseconds and converted to samples using the decoded sample rate, so the effect is sample-rate-agnostic.
4. **`toPCM`** — float64 → int16 with hard clipping.
5. **`play`** — plays the PCM via `github.com/ebitengine/oto/v3` (one process-wide Oto context, lazily bound to the first sample rate seen).

`server.go` is listen mode: an HTTP server on `:8787` whose `/tts` handler pushes request bodies onto a buffered channel drained by `ttsWorker`, which calls `speak` serially.

The `*.mp3` files at the repo root (`alassmells.mp3`, `save.mp3`, `test.mp3`) are sample outputs, not fixtures consumed by the code.
