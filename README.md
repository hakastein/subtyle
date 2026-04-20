# Subtyle

> **⚠️ 100% vibe-coded.** Not a single line of this code was written by a human. A human has not even *looked* at the code — not the Go backend, not the Vue frontend, not this README. Everything was produced by an LLM, steered entirely through natural-language prompts. Use at your own risk.

**Batch style editor for ASS / SSA subtitles.**

You downloaded a season of anime. The subtitles work fine, but the *styles* are ugly — tiny font, bad color, awful outline, wrong position. You want to restyle all 12 episodes the same way without opening each file one by one. That's what Subtyle is for.

It does **not** edit subtitle text, timing, karaoke, or typesetting. If you need those, use [Aegisub](https://aegisub.org/). Subtyle is the narrower, dumber, faster tool for one specific job.

## What it does

- Scans a folder for `.ass` / `.ssa` subtitle files and video files
- Extracts embedded ASS tracks directly from MKV / MP4 containers — no `mkvextract` required
- Lets you edit **only styles**: font, size, primary/secondary/outline/shadow colors, bold/italic, alignment, margins, border style
- Renders a live preview frame from the actual video with your styles applied (via `ffmpeg` + `libass`)
- Applies your styles across every file in the folder and saves them in one click
- Bundles `ffmpeg` on demand — downloads it on first launch if you don't already have one

## Why not Aegisub?

Aegisub is the gold standard for fansubbing. It is a full subtitle editor — timing, typesetting, karaoke templating, translation assist, frame-by-frame scrubbing, the works. Subtyle is **not trying to replace it** and never will.

Subtyle exists because Aegisub is clumsy for one specific use case: *"I just want these 12 files to look the same, and I want that to take 30 seconds."*

|                                                     | Aegisub                | Subtyle              |
|-----------------------------------------------------|------------------------|----------------------|
| Edit event text / timing / karaoke / typesetting    | ✅ (the whole point)   | ❌ (out of scope)    |
| **Batch-apply styles to a whole folder**            | ❌ (one file at a time)| ✅                   |
| Extract embedded ASS from MKV / MP4                 | via external `mkvextract` | ✅ (built-in)     |
| Preview styles on a real video frame                | ✅                     | ✅                   |
| Scope                                               | professional editor    | single-purpose tool  |
| Learning curve                                      | steep                  | basically none       |

**TL;DR:** Aegisub is Photoshop. Subtyle is the "change font across the whole folder" button that Photoshop doesn't have.

## Install

Grab the latest release from the [Releases page](../../releases):

- **Windows** — `subtyle-windows-amd64.exe` — this is the primary target and the only build that gets tested.
- **Linux** — `subtyle-linux-amd64` — built by the same CI pipeline but **not tested**. It compiles. Whether it runs is between you and `libwebkit2gtk`.

`ffmpeg` is downloaded automatically on first launch if it's not already on your system.

## Build from source

Requires Go 1.25+, Node.js 20+, and [Wails v2](https://wails.io/).

```bash
wails build -platform windows/amd64   # Windows
wails build -platform linux/amd64     # Linux (see caveat above)
```

Output lands in `build/bin/`.

On Linux you'll also need `libgtk-3-dev` and `libwebkit2gtk-4.0-dev` (or `-4.1-dev` with the appropriate Wails build tag).

## Status

Actively used by the author on Windows. Everything else is best-effort. No roadmap, no promises, no support SLA. If something breaks and you fix it, open a PR — the LLM will probably be the one reading it.
