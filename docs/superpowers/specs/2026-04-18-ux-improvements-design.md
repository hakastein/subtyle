# UX Improvements Design Spec

## Overview

Five related UX improvements to the subtitle style editor:

1. **Translation-first selection** — replace the source dropdown with a visible translations list; episodes become a list with checkboxes below. Clicking a translation auto-checks its covered episodes. Ctrl+click adds more translations (union of episodes).
2. **Local progress bars** — show loading progress in the toolbar (scan), styles column header (load styles), and preview overlay (render).
3. **Preview rendering speed** — use ffmpeg double-seek, dynamic scale to preview pixel size, and a base-frame cache.
4. **Correct Windows locale detection** — use `GetUserDefaultLocaleName` via `golang.org/x/sys/windows` instead of env vars.
5. **Permanent status bar** — always-visible status bar at the bottom. Click to toggle the debug log. Shows ffmpeg download progress during initial setup.

**Scope:** All UX polish on top of the existing app. One spec, one plan.

---

## 1. Translation-first selection

### Data model

The `SourceType` concept is renamed and refined to `Translation`. A translation represents one logical subtitle line across all episodes (e.g., "Полные (Crunchyroll)" from embedded tracks, or `.rus.[Anku].ass` from external files).

```typescript
interface TranslationInstance {
  videoPath: string
  subtitlePath?: string    // external
  trackIndex?: number      // embedded
  trackTitle?: string      // embedded
}

interface Translation {
  key: string              // "ext:.rus.[Anku].ass" or "emb:Полные (Crunchyroll):rus"
  label: string            // display name
  kind: 'external' | 'embedded'
  perEpisode: Map<string, TranslationInstance>  // videoPath → instance
  coverageCount: number    // perEpisode.size
  totalEpisodes: number    // total videos in folder
}
```

### State (project store)

- `selectedTranslationKeys: string[]` — multi-select (ctrl+click).
- `episodeChecks: Map<string, boolean>` — per-video checkbox state. Replaces the old `fileChecks`.

### Interactions

- **Click on translation** → `selectedTranslationKeys = [key]`. Episode checks are overwritten: videos covered by this translation become checked, others unchecked.
- **Ctrl+Click on translation** → toggle in `selectedTranslationKeys`. After toggle, episode checks are reset to the union of covered videos across all currently selected translations (manual uncheck/check state is lost — simple model).
- **Manual episode checkbox toggle** → only modifies `episodeChecks`; does not affect `selectedTranslationKeys` and is not reconciled until the next translation click.

### Grouped styles computation

Iterate `selectedTranslationKeys` × `episodeChecks` (checked only). For each pair, resolve the matching `TranslationInstance`, load if needed (external parse or embedded extract), then aggregate styles by name. Each grouped style shows:
- Representative style (first loaded).
- Episode range label: `collapseRanges(instances.map(i => episode))`.
- Instance count.

Clicking a grouped style puts `selectedStyleKeys = instances.map(i => fileId::styleName)`. StyleEditor's existing multi-select behavior applies edits to all instances.

### UI layout

Left panel (520px total) split into two sub-columns:

| Sub-column | Width | Contents |
|---|---|---|
| 1 | 260px | **Top 40%:** Translations list (no dropdown) with `label` + `coverageCount/totalEpisodes`. **Bottom 60%:** Episodes list with checkboxes, episode badge, full filename. |
| 2 | 260px | Styles list (unchanged) with a progress bar in the header. |

---

## 2. Local progress bars

Three distinct progress streams, each driven by Wails events from the backend and rendered in its own UI location.

### Streams

| Stream | Where | Events | Payload |
|---|---|---|---|
| `scan` | Toolbar (inline, right of buttons) | `progress:scan` | `{stage, current, total, message}` |
| `load` | Styles column header | `progress:load` | `{current, total, message}` |
| `preview` | Preview area overlay (thin blue top bar) | `progress:preview` | `{busy: bool}` |

### Pinia store: `progressStore`

```typescript
interface ProgressState {
  scan: { active: boolean; message: string; current: number; total: number }
  load: { active: boolean; message: string; current: number; total: number }
  preview: { busy: boolean }
}
```

### Backend integration

- `ScanFolder` emits `progress:scan` with `{stage: "reading", message: "Reading directory"}`, then for each video file being probed: `{stage: "probing", current: i, total: N, message: basename}`.
- Frontend `loadSourceStyles` emits `progress:load` from the store as it iterates selected translation × episodes.
- `GeneratePreviewFrame` → `progress:preview {busy: true}` at start, `{busy: false}` at end.

### UI components

- `Toolbar.vue`: add a small inline progress area that shows `message` + indeterminate bar when `scan.active`.
- `FilePanel.vue` styles column header: thin blue bar filling `current/total` when `load.active`.
- `PreviewArea.vue`: top 3px blue bar with animated stripe when `preview.busy`.

---

## 3. Preview rendering optimization

### Problem

Current ffmpeg command: `-ss <target> -i <video> -vf "subtitles=..." -frames:v 1 ...`. With `-ss` after `-i` (required for subtitle timing), ffmpeg decodes from file start to the target time. For a 23-minute video, a frame at 21m takes many seconds.

### Solution: double-seek + dynamic scale

```
ffmpeg -ss <fast> -i <video> -ss <fine> -vf "scale=W:-1,subtitles='<temp.ass>'" \
       -frames:v 1 -f image2 pipe:1
```

Where:
- `<fast>` = `max(0, targetSec - 10)` → input seek to nearest keyframe before target.
- `<fine>` = `min(10, targetSec)` → output seek for precise positioning.
- `W` = width in pixels from the frontend preview container size.

This decodes only ~10 seconds of video instead of the whole file. Subtitles still match because output timestamps are preserved.

### Dynamic resolution

- `PreviewArea.vue` uses a `ResizeObserver` on `.frame-container`.
- Debounce 200ms → send `widthPx = containerWidth * devicePixelRatio` (rounded to even).
- `GeneratePreviewFrame(fileID, videoPath, styles, atMs, widthPx)` signature extended with `widthPx`.
- Go formats filter: `scale=<widthPx>:-1,subtitles='<escaped>'`.

### Base-frame cache

Caches the decoded base frame (no subtitles overlay) on disk. When the same frame is requested again (e.g., while editing styles), skip the video decode step.

**Two-pass rendering:**

```
# Pass 1 (first time, cache miss):
ffmpeg -ss <fast> -i <video> -ss <fine> -vf "scale=W:-1" \
       -frames:v 1 -f image2 <base.png>

# Pass 2 (every time, applies current styles):
ffmpeg -itsoffset <atSec> -loop 1 -i <base.png> -vf "subtitles='<temp.ass>'" \
       -t 1 -frames:v 1 -f image2 pipe:1
```

`-itsoffset <atSec>` shifts the still image's PTS forward so the subtitles filter matches the event timing.

**Cache location:** `%APPDATA%\subtitles-editor\preview-cache\`.
**Cache key:** `sha1(videoPath + "|" + atMs + "|" + widthPx).png`.
**Eviction:** LRU when total cache size > 100 MB. Runs after each write.

**On cache hit:** skip pass 1, run pass 2 directly on cached PNG.

### Expected speedup

- Double-seek alone: 5–10x faster.
- Base-frame cache (second render of same event): near-instant (pass 2 only, no video decode).
- Scale-down: 2–3x faster per pass.

Combined: editing styles at a fixed preview point goes from ~3 sec to ~0.3 sec.

---

## 4. Windows locale detection

### Current bug

`internal/i18n/i18n.go` only checks env vars `LANG`, `LC_ALL`, etc. On native Windows these are not set, so the function always returns `"en"`.

### Fix: platform-specific implementation

Split `i18n.go` into two files with build tags:

**`i18n_windows.go`** (`//go:build windows`):
- Import `golang.org/x/sys/windows`.
- Call `windows.GetUserDefaultLocaleName()` → returns BCP-47 string like `"ru-RU"` or `"en-US"`.
- Parse: split on `-`, take first segment, lower-case.
- Return `"ru"` if first segment is `ru`, else `"en"`.
- Fallback to env var logic if API call fails.

**`i18n_other.go`** (`//go:build !windows`):
- Keep existing env var logic as-is.

### go.mod

Add `golang.org/x/sys` (already a transitive dep via Wails; ensure it's direct).

---

## 5. Permanent status bar

### Design

Extract the status line from `DebugPanel.vue` header into a new `StatusBar.vue` component. Always mounted in `MainView`. The debug log remains as a collapsible panel that appears above the status bar when toggled.

### StatusBar contents

- **ffmpeg status**: `● ffmpeg ready` (green) / `● downloading 43%` (amber) / `● not ready` (red).
- **Counters**: `episodes: N/M | translations: K | styles: L groups | undo: U`.
- **Dirty indicator**: `● unsaved` (amber) when `projectStore.dirty`.
- **Right side**: `⌃ debug` label + chevron icon.

### Behavior

- Click anywhere on the status bar → toggle `debugStore.visible`.
- F12 keyboard shortcut → same toggle (existing behavior).
- When ffmpeg is downloading (`previewStore.ffmpegDownloading && !ffmpegReady`): show `● downloading <percent>%` instead of ready/not ready. Uses existing `ffmpegProgress` value (0-1).

### Debug log panel

- Hidden by default. When visible, mounts above status bar (max-height 200px, scrollable).
- Same log contents as current `DebugPanel.vue` body.
- Transition: slide up/down 200ms.

### MainView layout change

Bottom of main view:
```
<main-body>
  <FilePanel /> <PreviewArea /> <StyleEditor />
</main-body>
<DebugLog v-if="debugStore.visible" />
<StatusBar />
```

---

## Component and file changes

### New files

- `frontend/src/stores/progress.ts` — progress state for scan/load/preview streams.
- `frontend/src/components/StatusBar.vue` — always-visible status line.
- `frontend/src/components/DebugLog.vue` — log viewer (extracted from DebugPanel).
- `internal/i18n/i18n_windows.go` — Windows API locale detection.
- `internal/i18n/i18n_other.go` — existing env-var fallback.
- `internal/preview/cache.go` — base-frame cache with LRU eviction.

### Modified files

- `internal/ffmpeg/extract.go` — new `ExtractBaseFrame` and `OverlayFrame` methods, update `buildFrameArgs` for double-seek + scale.
- `internal/preview/preview.go` — use cache, two-pass rendering.
- `internal/scan/scan.go` — emit `progress:scan` events during folder probe.
- `app.go` — `GeneratePreviewFrame` signature adds `widthPx`; wire up progress events.
- `frontend/src/services/types.ts` — rename `SourceType` → `Translation`, update shape.
- `frontend/src/stores/project.ts` — translation-first state (`selectedTranslationKeys`, `episodeChecks`), new `translations` and `groupedStyles` computed.
- `frontend/src/components/FilePanel.vue` — two sub-column layout with translations list + episodes list; styles with progress bar.
- `frontend/src/components/PreviewArea.vue` — ResizeObserver, pass `widthPx` to backend, progress bar overlay.
- `frontend/src/components/Toolbar.vue` — inline scan progress.
- `frontend/src/components/DebugPanel.vue` — deleted (split into StatusBar + DebugLog).
- `frontend/src/views/MainView.vue` — new layout with StatusBar mounted at bottom.

### Deleted files

- `frontend/src/components/DebugPanel.vue` (replaced by StatusBar + DebugLog).

---

## Error handling

- ffmpeg cache write failures: log to debug, fall back to non-cached render.
- Locale detection failure on Windows: fall back to env var / `"en"` default.
- Progress events: dropping an event is harmless — UI just doesn't update that tick.
- Preview cache size calculation racing with writes: best-effort LRU, not exact.

## Testing

- `i18n_windows_test.go` — skipped on non-Windows; verifies parsing logic in isolation.
- `preview/cache_test.go` — round-trip cache write/read, LRU eviction under size pressure.
- `extract_test.go` — `buildBaseFrameArgs`, `buildOverlayArgs` command construction.
- Frontend: manual verification (no unit tests for UI interactions in this codebase).

## Out of scope

- Parallel translation loading (load all instances of a translation concurrently) — could help for slow ffmpeg extraction but adds complexity; revisit if needed.
- Cache sharing between base-frame and final-frame (they can differ by style modifications).
- Preview frame debounce tuning — keep existing 400ms debounce from previous spec.
