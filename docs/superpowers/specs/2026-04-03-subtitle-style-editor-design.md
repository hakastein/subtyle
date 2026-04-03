# Subtitle Style Editor — Design Spec

## Overview

Desktop application for visually editing subtitle styles across an entire folder of video/subtitle files. Built with Go + Wails v2 (backend) and Vue 3 + TypeScript (frontend). Target platform: Windows amd64. Development environment: WSL2 Linux.

Users work with anime/media folders containing video files (.mkv, .mp4, etc.) and ASS/SSA subtitle files. Subtitles may also be embedded inside MKV containers. The app scans the folder, presents a tree of files and their styles, and lets the user edit styles with real-time preview. Changes are applied across multiple files via multi-select. Saving to disk is explicit (Ctrl+S).

## Tech Stack

- **Backend:** Go + Wails v2
- **Frontend:** Vue 3 (Composition API, `<script setup lang="ts">`), TypeScript
- **Subtitle parsing:** `github.com/asticode/go-astisub`
- **Video frame extraction:** ffmpeg (system PATH or auto-downloaded)
- **State management:** Pinia
- **UI components:** Naive UI (default theme, no customization)
- **i18n:** vue-i18n (EN + RU, system locale detection)
- **Project serialization:** Go `encoding/gob`

## Supported Formats

Only subtitle formats with named styles and visual attributes:
- ASS / SSA (primary format)
- Any other go-astisub-supported formats that contain style information

SRT and similar text-only formats — out of scope.

## UI Layout: Three Columns

```
┌─────────────────────────────────────────────────────────┐
│  Toolbar: [Open] [Save] [Undo] [Redo]         [EN|RU]  │
├──────────┬──────────────────────────┬───────────────────┤
│ Files &  │                          │   Style Editor    │
│ Styles   │      Preview Area        │                   │
│ (tree)   │   (CSS overlay + ffmpeg  │   Font name       │
│          │    rendered frame)        │   Font size       │
│ ▼ ep01   │                          │   B I U S         │
│   Default│                          │   Colors (4)      │
│   Signs  │                          │   Outline/Shadow  │
│ ▶ ep02   │                          │   Scale X/Y       │
│ ▶ ep03   │  ┌────────────────────┐  │   Spacing/Angle   │
│ 🎬 ep04  │  │ Subtitle text here │  │   Alignment (3×3) │
│          │  └────────────────────┘  │   Margins L/R/V   │
│          ├──────────────────────────┤                   │
│          │ Timeline: |  | |   | |  │                   │
│          │           ◀ ▶           │                   │
└──────────┴──────────────────────────┴───────────────────┘
```

### Left Panel: Files & Styles Tree
- Tree grouped by file. Expand file → see its styles.
- Each style shows: font name, font size, primary color, bold/italic, outline — inline in the tree.
- MKV files with embedded ASS tracks shown with 🎬 icon.
- Multi-select works cross-file (Ctrl+Click, Shift+Click).

### Center: Preview Area
- Video frame with subtitle overlay.
- Dual-mode preview:
  - **CSS preview** (instant): `<div>` styled to approximate ASS style, updates reactively on any change.
  - **FFmpeg preview** (accurate): kicks in after 500ms debounce, replaces CSS preview with actual rendered frame.
  - While ffmpeg is working, CSS preview remains visible (no blank state).
- Timeline bar below the frame showing all events for the selected style as vertical markers.
- Click marker → extract frame at that timecode.
- ◀ ▶ buttons to navigate between events.

### Right Panel: Style Editor
Fields:
- Font name (text input / dropdown)
- Font size (number input)
- Bold, Italic, Underline, Strikeout (toggle buttons)
- Primary color, Secondary color, Outline color, Back color (color pickers with alpha — Naive UI)
- Outline width, Shadow depth (number inputs)
- Scale X, Scale Y (number inputs, %)
- Spacing, Angle (number inputs)
- Alignment (3×3 grid, ASS numpad layout)
- Margins: L, R, V (number inputs)

When multiple styles selected: changing any field applies to ALL selected styles. Single undo entry for the batch.

## Architecture: Hybrid State Management

**Go** = source of truth for file I/O, ffmpeg operations, project serialization.
**Pinia** = working copy cache + undo stack for instant UI response.
Sync via debounced Wails calls.

### Go Backend Packages

```
internal/
  ffmpeg/       # find/download ffmpeg, extract frames, list/extract subtitle tracks
  parser/       # parse ASS/SSA via go-astisub, map to DTOs, write modified files
  editor/       # apply style mutations, generate temp ASS for ffmpeg preview
  project/      # gob serialization, autosave, session restore
  preview/      # orchestrate CSS preview data + ffmpeg frame extraction
  scan/         # scan folder: .ass files + MKV with ASS tracks, match video↔subs by name
  i18n/         # detect system locale, provide to frontend
```

### Domain DTOs (internal/parser)

Clean structs, not go-astisub types:

```go
type SubtitleStyle struct {
    Name            string
    FontName        string
    FontSize        float64
    Bold            bool
    Italic          bool
    Underline       bool
    Strikeout       bool
    PrimaryColour   Color  // RGBA
    SecondaryColour Color
    OutlineColour   Color
    BackColour      Color
    Outline         float64
    Shadow          float64
    ScaleX          float64
    ScaleY          float64
    Spacing         float64
    Angle           float64
    Alignment       int    // ASS numpad: 1-9
    MarginL         int
    MarginR         int
    MarginV         int
}

type Color struct {
    R, G, B, A uint8
}

type SubtitleEvent struct {
    StyleName string
    StartTime time.Duration
    EndTime   time.Duration
    Text      string
}

type SubtitleFile struct {
    ID       string           // unique identifier
    Path     string           // filesystem path
    Source   string           // "external" or "embedded"
    TrackID  int              // for embedded: stream index
    Styles   []SubtitleStyle
    Events   []SubtitleEvent
}

type FolderScanResult struct {
    Files []ScannedFile
}

type ScannedFile struct {
    Path       string
    VideoPath  string           // matched video file (if found)
    Type       string           // "external" or "embedded"
    Tracks     []TrackInfo      // for embedded: available ASS/SSA tracks
}

type TrackInfo struct {
    Index    int
    Language string
    Title    string
}
```

## Data Flows

### Open Folder
1. User clicks "Open Folder" → native folder dialog
2. Go `scan/` scans folder:
   - Finds `.ass`/`.ssa` files
   - For `.mkv`/`.mp4`/`.avi`/`.mov`/`.webm`: runs `ffmpeg -i` to list tracks, filters ASS/SSA only
   - Matches video↔subtitle files by base name
3. Returns `FolderScanResult` to frontend
4. Pinia stores the file tree
5. User expands a file → Go `parser/` parses the `.ass` → returns `SubtitleFile` with styles + events
6. For embedded tracks: user selects which tracks to extract → ffmpeg extracts to temp `.ass` → parse

### Edit Style
1. User changes a style field (e.g., fontSize 48 → 52)
2. Pinia: updates style in store, pushes undo entry
3. CSS preview updates instantly (Vue reactivity)
4. Debounce 500ms → Wails call to Go `editor/`:
   - Generates temp `.ass` with current styles
   - Calls ffmpeg to extract frame with subtitles overlay
   - Emits Wails event `preview:frame` with base64 PNG
5. Frontend replaces CSS preview with ffmpeg frame
6. On next edit → CSS preview returns instantly → ffmpeg restarts

### Save (Ctrl+S)
1. Pinia sends all modified styles to Go
2. Go `parser/` writes changes back to original `.ass` files
3. For embedded (extracted from MKV): saves as `filename.[modified].ass` next to video
4. Go `project/` marks project as clean (`Dirty = false`)
5. Frontend shows "Saved" notification

### Autosave
1. On any change (debounce 2s): Pinia sends state snapshot to Go
2. Go `project/` serializes to gob:
   - Folder path, file list, original + modified styles, undo/redo stacks, UI state
3. Saves to `%APPDATA%/subtitles-editor/autosave.gob`

### Session Restore
1. On startup: check `autosave.gob` exists and `Dirty == true`
2. If yes → dialog: "Unsaved work from [SavedAt]. Continue?"
3. Yes → restore full state including undo stack
4. No → delete `autosave.gob`, clean start

## Undo/Redo

Lives in Pinia. Each entry:

```typescript
interface UndoEntry {
  id: number
  description: string
  changes: Array<{
    fileID: string
    styleName: string
    field: string
    oldValue: StyleFieldValue
    newValue: StyleFieldValue
  }>
}
```

- Batch edit (change fontSize for 5 styles across files) = one entry with 5 items in `changes[]`.
- Ctrl+Z undoes all `changes[]` in the entry. Ctrl+Y / Ctrl+Shift+Z redoes.
- Undo/redo triggers CSS preview instantly + ffmpeg with debounce.
- Stack is unbounded within a session.
- Serialized in gob for autosave, restored on session resume.

## FFmpeg Management

### Discovery & Installation
1. On startup: `exec.LookPath("ffmpeg")`
2. If found → use system ffmpeg
3. If not → download static build for Windows amd64
4. Store in `%APPDATA%/subtitles-editor/bin/ffmpeg.exe`
5. Download progress → Wails event → frontend notification with progress bar

### FFmpeg Operations

All via `exec.CommandContext(ctx, ...)` in goroutines:

1. **Extract frame with subtitles:**
   ```
   ffmpeg -ss <time> -i <video> -vf "subtitles=<temp.ass>" -frames:v 1 -f image2 pipe:1
   ```
   Result: PNG via stdout → base64 → Wails event

2. **List tracks in container:**
   ```
   ffmpeg -i <video>
   ```
   Parse stderr for `Stream #0:N: Subtitle: ass`

3. **Extract subtitle track:**
   ```
   ffmpeg -i <video> -map 0:s:<N> -c:s copy <output.ass>
   ```

- Cancel previous unfinished call via `context.Cancel()` when user navigates to a different frame.
- FFmpeg errors → Wails event → user-friendly message in frontend.

## CSS Preview: ASS → CSS Mapping

| ASS Property | CSS Equivalent |
|---|---|
| FontName | `font-family` |
| FontSize | `font-size` (scaled to video dimensions) |
| PrimaryColour (`&HAABBGGRR`) | `color: rgba(R,G,B,A)` |
| OutlineColour + Outline | `-webkit-text-stroke` |
| Shadow + BackColour | `text-shadow` |
| Bold | `font-weight: bold` |
| Italic | `font-style: italic` |
| Underline | `text-decoration: underline` |
| Strikeout | `text-decoration: line-through` |
| Alignment (1-9 numpad) | CSS positioning (9 zones) |
| MarginL/R/V | `padding` |
| ScaleX/ScaleY | `transform: scale()` |
| Spacing | `letter-spacing` |
| Angle | `transform: rotate()` |

## i18n

- `vue-i18n` with two locale files: `frontend/src/i18n/en.json`, `frontend/src/i18n/ru.json`
- System locale detected by Go (Windows registry / env) → passed to frontend on init
- Language toggle in toolbar → saves choice to localStorage
- All user-facing strings go through `$t()` / `t()` — no hardcoded text in templates

## Project Structure

```
cmd/
  app/
    main.go                    # Wails app entry, bind Go structs
internal/
  ffmpeg/
    ffmpeg.go                  # find, download, manage ffmpeg binary
    extract.go                 # frame extraction, track listing, track extraction
  parser/
    types.go                   # domain DTOs (SubtitleStyle, SubtitleEvent, etc.)
    parser.go                  # parse ASS/SSA → DTOs
    writer.go                  # write DTOs → ASS/SSA files
  editor/
    editor.go                  # apply style changes, generate temp ASS
  project/
    project.go                 # gob serialization, autosave, restore
  preview/
    preview.go                 # orchestrate preview frame generation
  scan/
    scan.go                    # folder scanning, file matching, track discovery
  i18n/
    i18n.go                    # system locale detection
frontend/
  src/
    components/
      FileTree.vue             # left panel: file/style tree
      StyleEditor.vue          # right panel: style editing fields
      PreviewArea.vue          # center: video frame + CSS overlay
      Timeline.vue             # timeline bar with event markers
      Toolbar.vue              # top bar: actions + language toggle
    stores/
      project.ts               # main Pinia store: files, styles, selection
      undo.ts                  # undo/redo stack
      preview.ts               # preview state, current frame
    views/
      MainView.vue             # three-column layout
      RestoreDialog.vue        # session restore prompt
    services/
      scan.ts                  # Wails bindings: folder scan
      parser.ts                # Wails bindings: parse/save files
      editor.ts                # Wails bindings: apply changes, get preview
      ffmpeg.ts                # Wails bindings: ffmpeg status
      project.ts               # Wails bindings: autosave/restore
    i18n/
      en.json
      ru.json
      index.ts                 # vue-i18n setup
    App.vue
    main.ts
```

## Error Handling

- **Go:** All errors wrapped with context via `fmt.Errorf("...: %w", err)`. No bare `panic()`.
- **Frontend:** All Wails calls wrapped in typed service functions. Errors caught and displayed as Naive UI notifications — never swallowed silently.
- **FFmpeg failures:** Shown as non-blocking notifications. Preview falls back to CSS-only mode if ffmpeg is unavailable.

## Out of Scope

- SRT and other text-only subtitle formats
- Video playback (play/pause) — static frames only
- Muxing subtitles back into MKV containers
- Platforms other than Windows amd64
- Multiple simultaneous projects / tabs
