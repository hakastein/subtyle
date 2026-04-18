# UX Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver five related UX improvements: translation-first selection, local progress bars, faster preview rendering with cache, correct Windows locale detection, and a permanent status bar.

**Architecture:** Mostly frontend + Go backend tweaks. Uses existing Wails event system for progress streams. Adds base-frame cache in appdata. Platform-specific locale detection via Go build tags.

**Tech Stack:** Go 1.22+, Wails v2, `golang.org/x/sys/windows`, Vue 3 Composition API, Pinia, Naive UI.

**Spec:** `docs/superpowers/specs/2026-04-18-ux-improvements-design.md`

---

## File Map

### Go Backend

| File | Responsibility |
|---|---|
| `internal/i18n/i18n.go` | Deleted, replaced by platform-specific files |
| `internal/i18n/i18n_windows.go` (new) | Windows locale via `GetUserDefaultLocaleName` |
| `internal/i18n/i18n_other.go` (new) | Env-var fallback for non-Windows |
| `internal/i18n/i18n_test.go` (new) | Parsing logic tests |
| `internal/ffmpeg/extract.go` | Double-seek args, scale, ExtractBaseFrame, OverlayFrame |
| `internal/ffmpeg/extract_test.go` | Test new arg builders |
| `internal/preview/cache.go` (new) | SHA1-keyed base-frame cache with LRU eviction |
| `internal/preview/cache_test.go` (new) | Cache round-trip + eviction tests |
| `internal/preview/preview.go` | Two-pass generation using cache |
| `internal/scan/scan.go` | Emit progress events during folder probe |
| `app.go` | `GeneratePreviewFrame` adds `widthPx`; progress events plumbing; cache wiring |

### Vue Frontend

| File | Responsibility |
|---|---|
| `frontend/src/stores/progress.ts` (new) | Progress state for scan/load/preview |
| `frontend/src/components/StatusBar.vue` (new) | Permanent bottom status line |
| `frontend/src/components/DebugLog.vue` (new) | Collapsible log (extracted from DebugPanel) |
| `frontend/src/components/DebugPanel.vue` | Deleted |
| `frontend/src/components/FilePanel.vue` | Refactor: translations list + episodes list |
| `frontend/src/components/Toolbar.vue` | Inline scan progress |
| `frontend/src/components/PreviewArea.vue` | ResizeObserver, widthPx pipeline, top progress bar |
| `frontend/src/services/types.ts` | Add `Translation` type |
| `frontend/src/services/parser.ts` | `generatePreviewFrame` signature adds `widthPx` |
| `frontend/src/services/editor.ts` | Same widthPx param |
| `frontend/src/stores/project.ts` | Translation-first state, `selectedTranslationKeys`, `episodeChecks` |
| `frontend/src/views/MainView.vue` | Mount StatusBar + DebugLog |
| `frontend/src/wails.d.ts` | Update method signatures |

---

### Task 1: Windows Locale Detection

**Files:**
- Delete: `internal/i18n/i18n.go`
- Create: `internal/i18n/i18n_windows.go`
- Create: `internal/i18n/i18n_other.go`
- Create: `internal/i18n/i18n_test.go`

- [ ] **Step 1: Write test for shared parsing logic**

Create `internal/i18n/i18n_test.go`:
```go
package i18n

import "testing"

func TestNormalizeBCP47(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ru-RU", "ru"},
		{"en-US", "en"},
		{"RU", "ru"},
		{"ru_RU.UTF-8", "ru"},
		{"en_US.UTF-8", "en"},
		{"", "en"},
		{"de-DE", "en"}, // unsupported → default
		{"ru", "ru"},
	}

	for _, tt := range tests {
		got := normalizeBCP47(tt.input)
		if got != tt.want {
			t.Errorf("normalizeBCP47(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Delete old file**

Run:
```bash
rm internal/i18n/i18n.go
```

- [ ] **Step 3: Create shared helper file**

Create `internal/i18n/i18n_other.go`:
```go
//go:build !windows

package i18n

import (
	"os"
	"strings"
)

// DetectLocale checks environment variables and returns "ru" or "en".
func DetectLocale() string {
	for _, envVar := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		val := os.Getenv(envVar)
		if val == "" {
			continue
		}
		if loc := normalizeBCP47(val); loc != "" {
			return loc
		}
	}
	return "en"
}

// normalizeBCP47 takes a locale string (e.g. "ru-RU", "ru_RU.UTF-8", "RU")
// and returns "ru" or "en".
func normalizeBCP47(s string) string {
	if s == "" {
		return "en"
	}
	lower := strings.ToLower(s)
	for _, sep := range []string{"-", "_", "."} {
		if idx := strings.Index(lower, sep); idx >= 0 {
			lower = lower[:idx]
		}
	}
	if lower == "ru" {
		return "ru"
	}
	return "en"
}
```

- [ ] **Step 4: Create Windows-specific file**

Create `internal/i18n/i18n_windows.go`:
```go
//go:build windows

package i18n

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// DetectLocale uses Windows GetUserDefaultLocaleName, with env-var fallback.
func DetectLocale() string {
	if loc, err := getWindowsLocale(); err == nil {
		if normalized := normalizeBCP47(loc); normalized != "" {
			return normalized
		}
	}
	// Fallback to env vars (WSL, etc.)
	for _, envVar := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		val := os.Getenv(envVar)
		if val == "" {
			continue
		}
		if loc := normalizeBCP47(val); loc != "" {
			return loc
		}
	}
	return "en"
}

func getWindowsLocale() (string, error) {
	buf := make([]uint16, windows.LOCALE_NAME_MAX_LENGTH)
	n, err := windows.GetUserDefaultLocaleName(&buf[0], int32(len(buf)))
	if err != nil || n == 0 {
		return "", fmt.Errorf("GetUserDefaultLocaleName: %w", err)
	}
	return windows.UTF16ToString(buf[:n]), nil
}

// normalizeBCP47 is shared between platform files (kept here for Windows build).
func normalizeBCP47(s string) string {
	if s == "" {
		return "en"
	}
	lower := strings.ToLower(s)
	for _, sep := range []string{"-", "_", "."} {
		if idx := strings.Index(lower, sep); idx >= 0 {
			lower = lower[:idx]
		}
	}
	if lower == "ru" {
		return "ru"
	}
	return "en"
}
```

Note: `normalizeBCP47` appears in both files because each file compiles only on its platform (build tags) — no linker conflict.

- [ ] **Step 5: Ensure golang.org/x/sys is a direct dependency**

Run:
```bash
go get golang.org/x/sys/windows
go mod tidy
```

- [ ] **Step 6: Run tests**

Run:
```bash
go test ./internal/i18n/ -v
```
Expected: all subtests pass.

- [ ] **Step 7: Verify cross-compile**

Run:
```bash
go build ./...
GOOS=windows GOARCH=amd64 go build ./...
```
Expected: both compile without errors.

- [ ] **Step 8: Commit**

```bash
git add internal/i18n/ go.mod go.sum
git commit -m "fix: detect Windows locale via GetUserDefaultLocaleName"
```

---

### Task 2: Progress Store

**Files:**
- Create: `frontend/src/stores/progress.ts`

- [ ] **Step 1: Create progress store**

Create `frontend/src/stores/progress.ts`:
```typescript
import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface ProgressStream {
  active: boolean
  message: string
  current: number
  total: number
}

function emptyStream(): ProgressStream {
  return { active: false, message: '', current: 0, total: 0 }
}

export const useProgressStore = defineStore('progress', () => {
  const scan = ref<ProgressStream>(emptyStream())
  const load = ref<ProgressStream>(emptyStream())
  const preview = ref<{ busy: boolean }>({ busy: false })

  function startScan(message: string, total: number = 0): void {
    scan.value = { active: true, message, current: 0, total }
  }

  function updateScan(current: number, total: number, message: string): void {
    scan.value = { active: true, message, current, total }
  }

  function finishScan(): void {
    scan.value = emptyStream()
  }

  function startLoad(message: string, total: number = 0): void {
    load.value = { active: true, message, current: 0, total }
  }

  function updateLoad(current: number, total: number, message: string): void {
    load.value = { active: true, message, current, total }
  }

  function finishLoad(): void {
    load.value = emptyStream()
  }

  function startPreview(): void {
    preview.value = { busy: true }
  }

  function finishPreview(): void {
    preview.value = { busy: false }
  }

  return {
    scan,
    load,
    preview,
    startScan,
    updateScan,
    finishScan,
    startLoad,
    updateLoad,
    finishLoad,
    startPreview,
    finishPreview,
  }
})
```

- [ ] **Step 2: Verify TypeScript compiles**

Run:
```bash
cd frontend && npx vue-tsc --noEmit
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/stores/progress.ts
git commit -m "feat: add progress store for scan/load/preview streams"
```

---

### Task 3: Status Bar and Debug Log Split

**Files:**
- Delete: `frontend/src/components/DebugPanel.vue`
- Create: `frontend/src/components/StatusBar.vue`
- Create: `frontend/src/components/DebugLog.vue`
- Modify: `frontend/src/views/MainView.vue`

- [ ] **Step 1: Create DebugLog component (extracted log viewer)**

Create `frontend/src/components/DebugLog.vue`:
```vue
<script setup lang="ts">
import { ref, nextTick, watch } from 'vue'
import { NButton, NScrollbar } from 'naive-ui'
import { useDebugStore } from '@/stores/debug'

const debug = useDebugStore()
const scrollRef = ref<InstanceType<typeof NScrollbar> | null>(null)

watch(() => debug.logs.length, () => {
  nextTick(() => {
    scrollRef.value?.scrollTo({ top: 999999 })
  })
})

function levelColor(level: string): string {
  switch (level) {
    case 'error': return '#ff6b6b'
    case 'warn': return '#ffc107'
    default: return '#90caf9'
  }
}
</script>

<template>
  <div v-if="debug.visible" class="debug-log">
    <div class="debug-log-header">
      <span class="log-title">Debug Log</span>
      <div class="log-actions">
        <NButton size="tiny" @click="debug.clear()">Clear</NButton>
      </div>
    </div>
    <NScrollbar ref="scrollRef" style="max-height: 180px">
      <div class="log-entries">
        <div
          v-for="(entry, i) in debug.logs"
          :key="i"
          class="log-entry"
        >
          <span class="log-time">{{ entry.time }}</span>
          <span class="log-level" :style="{ color: levelColor(entry.level) }">
            [{{ entry.level.toUpperCase() }}]
          </span>
          <span class="log-msg">{{ entry.message }}</span>
        </div>
        <div v-if="debug.logs.length === 0" class="log-empty">No logs yet</div>
      </div>
    </NScrollbar>
  </div>
</template>

<style scoped>
.debug-log {
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 11px;
  border-top: 1px solid #333;
}

.debug-log-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 10px;
  background: #252526;
  border-bottom: 1px solid #333;
}

.log-title {
  font-weight: bold;
  color: #007acc;
}

.log-entries {
  padding: 4px 10px;
}

.log-entry {
  display: flex;
  gap: 6px;
  line-height: 1.6;
}

.log-time { color: #666; }
.log-level { min-width: 50px; }
.log-msg { white-space: pre-wrap; word-break: break-all; }
.log-empty { color: #666; padding: 8px; }
</style>
```

- [ ] **Step 2: Create StatusBar component**

Create `frontend/src/components/StatusBar.vue`:
```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useProjectStore } from '@/stores/project'
import { useUndoStore } from '@/stores/undo'
import { usePreviewStore } from '@/stores/preview'
import { useDebugStore } from '@/stores/debug'

const projectStore = useProjectStore()
const undoStore = useUndoStore()
const previewStore = usePreviewStore()
const debug = useDebugStore()

const ffmpegStatus = computed(() => {
  if (previewStore.ffmpegReady) {
    return { text: 'ffmpeg ready', color: '#4caf50' }
  }
  if (previewStore.ffmpegDownloading) {
    const pct = Math.round(previewStore.ffmpegProgress * 100)
    return { text: `downloading ${pct}%`, color: '#ffc107' }
  }
  return { text: 'not ready', color: '#ff6b6b' }
})

const episodesCount = computed(() => {
  const total = projectStore.videoEntries.length
  let checked = 0
  for (const [, v] of projectStore.fileChecks) {
    if (v) checked++
  }
  return `${checked}/${total}`
})

const translationsCount = computed(() => {
  // Placeholder until Task 9 lands: show sourceTypes length
  return projectStore.sourceTypes?.length ?? 0
})

const stylesGroupsCount = computed(() => projectStore.groupedStyles.length)
</script>

<template>
  <div class="status-bar" @click="debug.toggle()">
    <span :style="{ color: ffmpegStatus.color }">● {{ ffmpegStatus.text }}</span>
    <span class="sep">|</span>
    <span>episodes: {{ episodesCount }}</span>
    <span class="sep">|</span>
    <span>translations: {{ translationsCount }}</span>
    <span class="sep">|</span>
    <span>styles: {{ stylesGroupsCount }} groups</span>
    <span class="sep">|</span>
    <span>undo: {{ undoStore.undoStack.length }}</span>
    <span v-if="projectStore.dirty" class="sep">|</span>
    <span v-if="projectStore.dirty" style="color: #ffc107">● unsaved</span>
    <span class="spacer"></span>
    <span class="toggle-hint">⌃ debug ({{ debug.visible ? 'hide' : 'show' }})</span>
  </div>
</template>

<style scoped>
.status-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 10px;
  background: #252526;
  color: #d4d4d4;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 11px;
  border-top: 2px solid #007acc;
  cursor: pointer;
  flex-shrink: 0;
  user-select: none;
}

.status-bar:hover {
  background: #2a2a2b;
}

.sep {
  color: #555;
}

.spacer {
  flex: 1;
}

.toggle-hint {
  color: #888;
}
</style>
```

- [ ] **Step 3: Update MainView to mount StatusBar + DebugLog**

In `frontend/src/views/MainView.vue`, replace the `DebugPanel` import and usage.

Replace:
```typescript
import DebugPanel from '@/components/DebugPanel.vue'
```
With:
```typescript
import DebugLog from '@/components/DebugLog.vue'
import StatusBar from '@/components/StatusBar.vue'
```

Replace the template fragment:
```vue
    <DebugPanel />
  </div>
</template>
```
With:
```vue
    <DebugLog />
    <StatusBar />
  </div>
</template>
```

- [ ] **Step 4: Delete old DebugPanel**

Run:
```bash
rm frontend/src/components/DebugPanel.vue
```

- [ ] **Step 5: Verify build**

Run:
```bash
cd frontend && npx vue-tsc --noEmit
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/StatusBar.vue frontend/src/components/DebugLog.vue frontend/src/components/DebugPanel.vue frontend/src/views/MainView.vue
git commit -m "feat: split DebugPanel into permanent StatusBar + collapsible DebugLog"
```

- [ ] **Step 7: Rebuild Windows exe**

Run:
```bash
/home/hakastein/go/bin/wails build -platform windows/amd64
```
Expected: builds to `build/bin/subtitles-editor.exe`.

---

### Task 4: FFmpeg Double-Seek + Scale

**Files:**
- Modify: `internal/ffmpeg/extract.go`
- Modify: `internal/ffmpeg/extract_test.go`

- [ ] **Step 1: Write test for new frame args with double-seek + scale**

Add to `internal/ffmpeg/extract_test.go`:
```go
func TestBuildFrameArgs_DoubleSeekAndScale(t *testing.T) {
	args := buildFrameArgs("/videos/ep01.mkv", "/tmp/sub.ass", 30*time.Second, 960)

	// Expect two -ss flags: fast before -i, fine after -i
	ssCount := 0
	var ssFast, ssFine string
	iIdx := -1
	for i, a := range args {
		if a == "-ss" && i+1 < len(args) {
			ssCount++
			if iIdx == -1 {
				ssFast = args[i+1]
			} else {
				ssFine = args[i+1]
			}
		}
		if a == "-i" {
			iIdx = i
		}
	}
	if ssCount != 2 {
		t.Fatalf("expected 2 -ss flags, got %d: %v", ssCount, args)
	}
	if iIdx < 0 {
		t.Fatal("no -i flag found")
	}

	// 30s - 10s = 20s fast seek, then 10s fine seek
	if ssFast != "20.000" {
		t.Errorf("fast seek = %q, want 20.000", ssFast)
	}
	if ssFine != "10.000" {
		t.Errorf("fine seek = %q, want 10.000", ssFine)
	}

	// Verify -vf has scale + subtitles
	var vfArg string
	for i, a := range args {
		if a == "-vf" && i+1 < len(args) {
			vfArg = args[i+1]
		}
	}
	if !strings.Contains(vfArg, "scale=960:-1") {
		t.Errorf("-vf missing scale: %q", vfArg)
	}
	if !strings.Contains(vfArg, "subtitles=") {
		t.Errorf("-vf missing subtitles: %q", vfArg)
	}
}

func TestBuildFrameArgs_SeekBelowThreshold(t *testing.T) {
	// For a target before 10s, fast seek is 0
	args := buildFrameArgs("/videos/ep01.mkv", "/tmp/sub.ass", 5*time.Second, 960)
	var ssFast, ssFine string
	first := true
	for i, a := range args {
		if a == "-ss" && i+1 < len(args) {
			if first {
				ssFast = args[i+1]
				first = false
			} else {
				ssFine = args[i+1]
			}
		}
	}
	if ssFast != "0.000" {
		t.Errorf("fast seek for 5s = %q, want 0.000", ssFast)
	}
	if ssFine != "5.000" {
		t.Errorf("fine seek for 5s = %q, want 5.000", ssFine)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/ffmpeg/ -run TestBuildFrameArgs_DoubleSeek -v
```
Expected: FAIL — signature mismatch (buildFrameArgs doesn't take widthPx yet).

- [ ] **Step 3: Update buildFrameArgs signature**

In `internal/ffmpeg/extract.go`, find the existing `buildFrameArgs` and replace with:
```go
// buildFrameArgs constructs ffmpeg arguments to render a single frame at `at`
// with subtitles burned in, scaled to widthPx pixels wide.
// Uses double-seek (fast -ss before -i, fine -ss after -i) to avoid decoding
// the entire file from the start.
func buildFrameArgs(videoPath, subPath string, at time.Duration, widthPx int) []string {
	totalSec := at.Seconds()
	fastSec := totalSec - 10.0
	if fastSec < 0 {
		fastSec = 0
	}
	fineSec := totalSec - fastSec

	vf := fmt.Sprintf("scale=%d:-1,subtitles='%s'", widthPx, escapeFilterPath(subPath))
	return []string{
		"-ss", fmt.Sprintf("%.3f", fastSec),
		"-i", videoPath,
		"-ss", fmt.Sprintf("%.3f", fineSec),
		"-vf", vf,
		"-frames:v", "1",
		"-f", "image2",
		"pipe:1",
	}
}
```

- [ ] **Step 4: Update existing TestBuildFrameArgs test to match new signature**

In `internal/ffmpeg/extract_test.go`, find the existing `TestBuildFrameArgs` and replace:
```go
func TestBuildFrameArgs(t *testing.T) {
	args := buildFrameArgs("/path/to/video.mkv", "/path/to/sub.ass", 90*time.Second, 960)

	required := []string{"-i", "/path/to/video.mkv", "-vf", "-frames:v", "1", "-f", "image2", "pipe:1"}
	for _, req := range required {
		found := false
		for _, arg := range args {
			if arg == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected arg %q in frame args: %v", req, args)
		}
	}
}
```

And update `TestBuildFrameArgsWindowsPath` to pass widthPx:
```go
func TestBuildFrameArgsWindowsPath(t *testing.T) {
	args := buildFrameArgs(`C:\Videos\ep01.mkv`, `C:\Temp\sub.ass`, 5*time.Second, 1280)
	var vfArg string
	for i, a := range args {
		if a == "-vf" && i+1 < len(args) {
			vfArg = args[i+1]
		}
	}
	if vfArg == "" {
		t.Fatal("no -vf argument found")
	}
	if !strings.Contains(vfArg, "C\\:") {
		t.Errorf("-vf arg should escape colon: %q", vfArg)
	}
	if !strings.Contains(vfArg, "scale=1280:-1") {
		t.Errorf("-vf arg should include scale: %q", vfArg)
	}
}
```

- [ ] **Step 5: Update ExtractFrame signature to accept widthPx**

In `internal/ffmpeg/extract.go`, find `ExtractFrame` and change:
```go
// OLD:
func (e *Extractor) ExtractFrame(ctx context.Context, videoPath, subPath string, at time.Duration) (string, error) {
	args := buildFrameArgs(videoPath, subPath, at)
```
To:
```go
func (e *Extractor) ExtractFrame(ctx context.Context, videoPath, subPath string, at time.Duration, widthPx int) (string, error) {
	if widthPx < 1 {
		widthPx = 960 // sensible default
	}
	args := buildFrameArgs(videoPath, subPath, at, widthPx)
```

- [ ] **Step 6: Update LastFrameCommand helper to take widthPx**

In `internal/ffmpeg/extract.go`, find `LastFrameCommand` and replace with:
```go
// LastFrameCommand returns the ffmpeg command that would be built for frame
// extraction. Useful for logging/debugging.
func LastFrameCommand(binPath, videoPath, subPath string, at time.Duration, widthPx int) string {
	args := buildFrameArgs(videoPath, subPath, at, widthPx)
	return binPath + " " + strings.Join(args, " ")
}
```

- [ ] **Step 7: Update preview.Generator to pass widthPx through**

In `internal/preview/preview.go`, change `GenerateFrame` signature and pass widthPx:
```go
func (g *Generator) GenerateFrame(ctx context.Context, videoPath string, subFile *parser.SubtitleFile, at time.Duration, widthPx int) (*FrameResult, error) {
	// Cancel previous generation if one is in progress.
	g.mu.Lock()
	if g.cancelFn != nil {
		g.cancelFn()
	}
	genCtx, cancel := context.WithCancel(ctx)
	g.cancelFn = cancel
	g.genID++
	myID := g.genID
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		if g.genID == myID {
			g.cancelFn = nil
		}
		g.mu.Unlock()
		cancel()
	}()

	tmpPath, err := parser.WriteTempFile(subFile)
	if err != nil {
		return nil, fmt.Errorf("preview: writing temp file: %w", err)
	}
	defer os.Remove(tmpPath)

	base64PNG, err := g.extractor.ExtractFrame(genCtx, videoPath, tmpPath, at, widthPx)
	if err != nil {
		return nil, fmt.Errorf("preview: extracting frame: %w", err)
	}

	return &FrameResult{
		Base64PNG: base64PNG,
		Timecode:  formatTimecode(at),
	}, nil
}
```

- [ ] **Step 8: Update App.GeneratePreviewFrame signature**

In `app.go`, find `GeneratePreviewFrame` and update:
```go
// GeneratePreviewFrame renders a single preview frame for the given file
// with the provided styles applied at atMs. widthPx is the target rendering
// width (from the frontend container size). Pass 0 to use default.
func (a *App) GeneratePreviewFrame(fileID string, videoPath string, styles []parser.SubtitleStyle, atMs int64, widthPx int) (*preview.FrameResult, error) {
	if a.previewGen == nil {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	orig, ok := a.parsedFiles[fileID]
	if !ok {
		return nil, fmt.Errorf("file %q not found", fileID)
	}

	modified := *orig
	modified.Styles = styles

	runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("preview: file.Path=%q Source=%q styles=%d events=%d widthPx=%d", modified.Path, modified.Source, len(modified.Styles), len(modified.Events), widthPx))

	at := time.Duration(atMs) * time.Millisecond
	result, err := a.previewGen.GenerateFrame(a.ctx, videoPath, &modified, at, widthPx)
	if err != nil {
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("GeneratePreviewFrame error: %v", err))
		return nil, err
	}
	return result, nil
}
```

- [ ] **Step 9: Run Go tests**

Run:
```bash
go test ./... -v
```
Expected: all pass.

- [ ] **Step 10: Commit**

```bash
git add internal/ffmpeg/ internal/preview/preview.go app.go
git commit -m "feat: ffmpeg double-seek + dynamic scale for faster preview"
```

---

### Task 5: Dynamic widthPx Frontend Pipeline

**Files:**
- Modify: `frontend/src/services/editor.ts`
- Modify: `frontend/src/wails.d.ts`
- Modify: `frontend/src/components/PreviewArea.vue`

- [ ] **Step 1: Update editor service signature**

In `frontend/src/services/editor.ts`, replace:
```typescript
export async function generatePreviewFrame(fileId: string, videoPath: string, styles: SubtitleStyle[], atMs: number): Promise<FrameResult> { return window.go.main.App.GeneratePreviewFrame(fileId, videoPath, styles, atMs) }
```
With:
```typescript
export async function generatePreviewFrame(
  fileId: string,
  videoPath: string,
  styles: SubtitleStyle[],
  atMs: number,
  widthPx: number,
): Promise<FrameResult> {
  return window.go.main.App.GeneratePreviewFrame(fileId, videoPath, styles, atMs, widthPx)
}
```

- [ ] **Step 2: Update wails.d.ts**

In `frontend/src/wails.d.ts`, update `GeneratePreviewFrame`:
```typescript
GeneratePreviewFrame(fileId: string, videoPath: string, styles: SubtitleStyle[], atMs: number, widthPx: number): Promise<FrameResult>
```

- [ ] **Step 3: Add ResizeObserver to PreviewArea**

In `frontend/src/components/PreviewArea.vue`, add to the imports at top of `<script setup>`:
```typescript
import { onMounted, onUnmounted } from 'vue'
```

Add state near other refs:
```typescript
const frameContainerRef = ref<HTMLElement | null>(null)
const previewWidthPx = ref(960)
let resizeObserver: ResizeObserver | null = null
let resizeDebounceTimer: ReturnType<typeof setTimeout> | null = null
```

Add lifecycle hooks:
```typescript
onMounted(() => {
  if (!frameContainerRef.value) return
  resizeObserver = new ResizeObserver((entries) => {
    const entry = entries[0]
    if (!entry) return
    const cssWidth = entry.contentRect.width
    const pxWidth = Math.round(cssWidth * (window.devicePixelRatio || 1))
    // Round to even number for better codec compatibility
    const rounded = Math.max(320, pxWidth - (pxWidth % 2))

    if (resizeDebounceTimer) clearTimeout(resizeDebounceTimer)
    resizeDebounceTimer = setTimeout(() => {
      if (rounded !== previewWidthPx.value) {
        previewWidthPx.value = rounded
        debug.info(`preview width resized → ${rounded}px`)
        // Trigger a re-render
        schedulePreview()
      }
    }, 200)
  })
  resizeObserver.observe(frameContainerRef.value)
})

onUnmounted(() => {
  resizeObserver?.disconnect()
  if (resizeDebounceTimer) clearTimeout(resizeDebounceTimer)
})
```

- [ ] **Step 4: Update generatePreview to pass widthPx**

In `PreviewArea.vue`, find the `generatePreview` function body and change the service call:
```typescript
    const result = await editorService.generatePreviewFrame(
      file.id,
      file.videoPath,
      file.modifiedStyles,
      atMs,
      previewWidthPx.value,
    )
```

- [ ] **Step 5: Attach the ref to the frame container in template**

In `PreviewArea.vue` template, find:
```vue
<div class="preview-frame-container">
```
Change to:
```vue
<div class="preview-frame-container" ref="frameContainerRef">
```

- [ ] **Step 6: Verify build**

Run:
```bash
cd frontend && npx vue-tsc --noEmit
```
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/services/editor.ts frontend/src/wails.d.ts frontend/src/components/PreviewArea.vue
git commit -m "feat: pass dynamic preview width to ffmpeg scale filter"
```

- [ ] **Step 8: Rebuild Windows exe**

Run:
```bash
/home/hakastein/go/bin/wails build -platform windows/amd64
```
Expected: success.

---

### Task 6: Base-Frame Cache

**Files:**
- Create: `internal/preview/cache.go`
- Create: `internal/preview/cache_test.go`
- Modify: `internal/ffmpeg/extract.go` (add `ExtractBaseFrame`, `OverlayFrame`)
- Modify: `internal/ffmpeg/extract_test.go`
- Modify: `internal/preview/preview.go` (two-pass using cache)
- Modify: `app.go` (pass cache dir to Generator)

- [ ] **Step 1: Write cache tests**

Create `internal/preview/cache_test.go`:
```go
package preview

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir, 1024*1024) // 1MB limit

	payload := []byte("fake PNG data")
	key := cache.Key("/videos/ep01.mkv", 21470*time.Millisecond, 960)

	if cache.Exists(key) {
		t.Fatal("key should not exist yet")
	}

	if err := cache.Write(key, payload); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !cache.Exists(key) {
		t.Fatal("key should exist after write")
	}

	got, err := cache.Read(key)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("Read payload = %q, want %q", got, payload)
	}
}

func TestCacheKeyStability(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir, 1024*1024)

	k1 := cache.Key("/videos/ep01.mkv", 1000*time.Millisecond, 960)
	k2 := cache.Key("/videos/ep01.mkv", 1000*time.Millisecond, 960)
	if k1 != k2 {
		t.Errorf("same inputs produced different keys: %q vs %q", k1, k2)
	}

	k3 := cache.Key("/videos/ep01.mkv", 1000*time.Millisecond, 1280)
	if k1 == k3 {
		t.Errorf("different widthPx should produce different keys")
	}

	k4 := cache.Key("/videos/ep02.mkv", 1000*time.Millisecond, 960)
	if k1 == k4 {
		t.Errorf("different videoPath should produce different keys")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir, 30) // very small limit so evictions fire

	// Write 3 entries, each 15 bytes
	k1 := cache.Key("/a", 1*time.Second, 960)
	k2 := cache.Key("/b", 1*time.Second, 960)
	k3 := cache.Key("/c", 1*time.Second, 960)

	data := make([]byte, 15)
	for i := range data {
		data[i] = 'x'
	}

	if err := cache.Write(k1, data); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // ensure mtime differs
	if err := cache.Write(k2, data); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := cache.Write(k3, data); err != nil {
		t.Fatal(err)
	}

	// After k3 write, total = 45 bytes > 30 limit → k1 (oldest) evicted
	if cache.Exists(k1) {
		t.Error("k1 should have been evicted (oldest)")
	}
	if !cache.Exists(k2) {
		t.Error("k2 should still exist")
	}
	if !cache.Exists(k3) {
		t.Error("k3 should still exist")
	}
}

func TestCacheCreatesDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "doesnotexist")
	cache := NewCache(dir, 1024)

	key := cache.Key("/v", time.Second, 960)
	if err := cache.Write(key, []byte("x")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("cache dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("cache path is not a directory")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/preview/ -v
```
Expected: compile error — `NewCache` undefined.

- [ ] **Step 3: Implement cache**

Create `internal/preview/cache.go`:
```go
package preview

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Cache stores base video frames on disk, keyed by (videoPath, time, widthPx).
// When total cache size exceeds maxBytes, the oldest (by mtime) entries are
// evicted until the cache is under the limit again.
type Cache struct {
	dir      string
	maxBytes int64
	mu       sync.Mutex
}

// NewCache creates a cache rooted at dir. maxBytes is the total size budget
// (0 disables eviction — use for testing).
func NewCache(dir string, maxBytes int64) *Cache {
	return &Cache{dir: dir, maxBytes: maxBytes}
}

// Key derives a stable filename (no extension) from the cache inputs.
func (c *Cache) Key(videoPath string, at time.Duration, widthPx int) string {
	h := sha1.New()
	fmt.Fprintf(h, "%s|%d|%d", videoPath, at.Milliseconds(), widthPx)
	return hex.EncodeToString(h.Sum(nil))
}

// Path returns the absolute path to where a given key's data lives.
func (c *Cache) Path(key string) string {
	return filepath.Join(c.dir, key+".png")
}

// Exists reports whether the cache has an entry for this key.
func (c *Cache) Exists(key string) bool {
	info, err := os.Stat(c.Path(key))
	return err == nil && !info.IsDir()
}

// Read returns the cached bytes for this key.
func (c *Cache) Read(key string) ([]byte, error) {
	return os.ReadFile(c.Path(key))
}

// Write stores data under key. Creates the cache directory if needed.
// Triggers LRU eviction after the write.
func (c *Cache) Write(key string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("cache: create dir %q: %w", c.dir, err)
	}
	if err := os.WriteFile(c.Path(key), data, 0644); err != nil {
		return fmt.Errorf("cache: write %q: %w", key, err)
	}
	c.evictIfNeeded()
	return nil
}

// evictIfNeeded removes oldest entries until cache size is under maxBytes.
// Must be called with mu held.
func (c *Cache) evictIfNeeded() {
	if c.maxBytes <= 0 {
		return
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}

	type entryInfo struct {
		path  string
		size  int64
		mtime time.Time
	}
	var infos []entryInfo
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		infos = append(infos, entryInfo{
			path:  filepath.Join(c.dir, e.Name()),
			size:  info.Size(),
			mtime: info.ModTime(),
		})
		total += info.Size()
	}
	if total <= c.maxBytes {
		return
	}

	// Sort oldest first
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].mtime.Before(infos[j].mtime)
	})

	for _, e := range infos {
		if total <= c.maxBytes {
			break
		}
		if err := os.Remove(e.path); err == nil {
			total -= e.size
		}
	}
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/preview/ -v
```
Expected: all pass.

- [ ] **Step 5: Add ExtractBaseFrame test**

Add to `internal/ffmpeg/extract_test.go`:
```go
func TestBuildBaseFrameArgs(t *testing.T) {
	args := buildBaseFrameArgs("/videos/ep01.mkv", 30*time.Second, 960, "/tmp/base.png")

	// No -vf subtitles=, just scale
	var vfArg string
	for i, a := range args {
		if a == "-vf" && i+1 < len(args) {
			vfArg = args[i+1]
		}
	}
	if vfArg != "scale=960:-1" {
		t.Errorf("-vf = %q, want scale=960:-1", vfArg)
	}

	// Last arg should be the output path
	last := args[len(args)-1]
	if last != "/tmp/base.png" {
		t.Errorf("last arg = %q, want output path", last)
	}

	// Should use double-seek
	ssCount := 0
	for _, a := range args {
		if a == "-ss" {
			ssCount++
		}
	}
	if ssCount != 2 {
		t.Errorf("expected 2 -ss flags, got %d", ssCount)
	}
}

func TestBuildOverlayArgs(t *testing.T) {
	args := buildOverlayArgs("/tmp/base.png", "/tmp/sub.ass", 21*time.Second)

	// Should have -itsoffset
	var offset string
	for i, a := range args {
		if a == "-itsoffset" && i+1 < len(args) {
			offset = args[i+1]
		}
	}
	if offset != "21.000" {
		t.Errorf("-itsoffset = %q, want 21.000", offset)
	}

	// Should have -loop 1
	hasLoop := false
	for i, a := range args {
		if a == "-loop" && i+1 < len(args) && args[i+1] == "1" {
			hasLoop = true
		}
	}
	if !hasLoop {
		t.Error("expected -loop 1")
	}

	// -vf should have subtitles=
	var vfArg string
	for i, a := range args {
		if a == "-vf" && i+1 < len(args) {
			vfArg = args[i+1]
		}
	}
	if !strings.Contains(vfArg, "subtitles=") {
		t.Errorf("-vf missing subtitles: %q", vfArg)
	}
}
```

- [ ] **Step 6: Implement base-frame and overlay ffmpeg builders**

Add to `internal/ffmpeg/extract.go` after `buildFrameArgs`:
```go
// buildBaseFrameArgs constructs arguments for extracting a base frame (no subtitles)
// scaled to widthPx wide, written to outputPath. Uses double-seek.
func buildBaseFrameArgs(videoPath string, at time.Duration, widthPx int, outputPath string) []string {
	totalSec := at.Seconds()
	fastSec := totalSec - 10.0
	if fastSec < 0 {
		fastSec = 0
	}
	fineSec := totalSec - fastSec

	vf := fmt.Sprintf("scale=%d:-1", widthPx)
	return []string{
		"-ss", fmt.Sprintf("%.3f", fastSec),
		"-i", videoPath,
		"-ss", fmt.Sprintf("%.3f", fineSec),
		"-vf", vf,
		"-frames:v", "1",
		"-y",
		outputPath,
	}
}

// buildOverlayArgs constructs arguments for applying subtitles to a pre-rendered
// base frame (PNG). Uses -itsoffset so the subtitles filter sees the frame at
// time `at` rather than 0.
func buildOverlayArgs(basePath, subPath string, at time.Duration) []string {
	vf := fmt.Sprintf("subtitles='%s'", escapeFilterPath(subPath))
	return []string{
		"-itsoffset", fmt.Sprintf("%.3f", at.Seconds()),
		"-loop", "1",
		"-i", basePath,
		"-vf", vf,
		"-t", "1",
		"-frames:v", "1",
		"-f", "image2",
		"pipe:1",
	}
}

// ExtractBaseFrame renders a subtitle-less frame to disk.
func (e *Extractor) ExtractBaseFrame(ctx context.Context, videoPath string, at time.Duration, widthPx int, outputPath string) error {
	args := buildBaseFrameArgs(videoPath, at, widthPx, outputPath)
	cmd := exec.CommandContext(ctx, e.binPath, args...)
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extract base frame: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

// OverlayFrame applies subtitles to an existing base frame PNG and returns
// the result as a base64-encoded PNG.
func (e *Extractor) OverlayFrame(ctx context.Context, basePath, subPath string, at time.Duration) (string, error) {
	args := buildOverlayArgs(basePath, subPath, at)
	cmd := exec.CommandContext(ctx, e.binPath, args...)
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffmpeg overlay frame: %w\nstderr: %s", err, stderr.String())
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
```

- [ ] **Step 7: Run ffmpeg tests**

Run:
```bash
go test ./internal/ffmpeg/ -v
```
Expected: all pass.

- [ ] **Step 8: Wire cache into Generator**

Replace `internal/preview/preview.go` entirely:
```go
package preview

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"subtitles-editor/internal/ffmpeg"
	"subtitles-editor/internal/parser"
)

type FrameResult struct {
	Base64PNG string `json:"base64Png"`
	Timecode  string `json:"timecode"`
}

// Generator manages preview frame generation with cancellation and a
// base-frame disk cache for fast re-renders at the same timecode.
type Generator struct {
	extractor *ffmpeg.Extractor
	cache     *Cache
	mu        sync.Mutex
	cancelFn  context.CancelFunc
	genID     int64
}

// NewGenerator creates a Generator backed by the given Extractor and cache.
// Pass a nil cache to disable caching entirely.
func NewGenerator(extractor *ffmpeg.Extractor, cache *Cache) *Generator {
	return &Generator{extractor: extractor, cache: cache}
}

// GenerateFrame produces a frame with subtitles burned in. When the cache is
// enabled, the base frame (no subtitles) is cached, and only the overlay pass
// runs on subsequent style edits at the same timecode.
func (g *Generator) GenerateFrame(ctx context.Context, videoPath string, subFile *parser.SubtitleFile, at time.Duration, widthPx int) (*FrameResult, error) {
	g.mu.Lock()
	if g.cancelFn != nil {
		g.cancelFn()
	}
	genCtx, cancel := context.WithCancel(ctx)
	g.cancelFn = cancel
	g.genID++
	myID := g.genID
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		if g.genID == myID {
			g.cancelFn = nil
		}
		g.mu.Unlock()
		cancel()
	}()

	tmpSubPath, err := parser.WriteTempFile(subFile)
	if err != nil {
		return nil, fmt.Errorf("preview: write temp subtitle: %w", err)
	}
	defer os.Remove(tmpSubPath)

	// If cache is disabled, fall back to single-pass rendering.
	if g.cache == nil {
		base64PNG, err := g.extractor.ExtractFrame(genCtx, videoPath, tmpSubPath, at, widthPx)
		if err != nil {
			return nil, fmt.Errorf("preview: extract frame: %w", err)
		}
		return &FrameResult{Base64PNG: base64PNG, Timecode: formatTimecode(at)}, nil
	}

	// Two-pass with cache.
	key := g.cache.Key(videoPath, at, widthPx)
	basePath := g.cache.Path(key)

	if !g.cache.Exists(key) {
		if err := g.extractor.ExtractBaseFrame(genCtx, videoPath, at, widthPx, basePath); err != nil {
			return nil, fmt.Errorf("preview: extract base frame: %w", err)
		}
		// Trigger eviction via empty write (file already on disk after extract).
		// Read it back, then rewrite through Write so LRU accounting runs.
		if data, err := os.ReadFile(basePath); err == nil {
			_ = g.cache.Write(key, data)
		}
	}

	base64PNG, err := g.extractor.OverlayFrame(genCtx, basePath, tmpSubPath, at)
	if err != nil {
		return nil, fmt.Errorf("preview: overlay frame: %w", err)
	}

	return &FrameResult{Base64PNG: base64PNG, Timecode: formatTimecode(at)}, nil
}

func formatTimecode(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
```

- [ ] **Step 9: Update App to wire the cache**

In `app.go`, find `initFFmpeg()` and update both branches (found and downloaded):

Find:
```go
if path := a.ffmpegMgr.Find(); path != "" {
    a.extractor = ffmpeg.NewExtractor(path)
    a.previewGen = preview.NewGenerator(a.extractor)
    ...
```

Replace the `previewGen` line with:
```go
    a.previewGen = preview.NewGenerator(a.extractor, preview.NewCache(filepath.Join(a.dataDir, "preview-cache"), 100*1024*1024))
```

Do the same for the post-download branch.

- [ ] **Step 10: Run full test suite**

Run:
```bash
go test ./... -v
```
Expected: all pass.

- [ ] **Step 11: Commit**

```bash
git add internal/preview/ internal/ffmpeg/ app.go
git commit -m "feat: base-frame cache with LRU eviction for near-instant style re-renders"
```

- [ ] **Step 12: Rebuild Windows exe**

Run:
```bash
/home/hakastein/go/bin/wails build -platform windows/amd64
```

---

### Task 7: Scan Progress Events

**Files:**
- Modify: `app.go` (scanEmbeddedTracks emits progress)
- Modify: `frontend/src/services/scan.ts` (no change needed — event listener is on frontend)
- Modify: `frontend/src/components/Toolbar.vue` (show progress)
- Modify: `frontend/src/views/MainView.vue` (subscribe to scan events)

- [ ] **Step 1: Emit scan progress from Go**

In `app.go`, find `ScanFolder` and update:
```go
func (a *App) ScanFolder(dir string) (*scan.FolderScanResult, error) {
	runtime.EventsEmit(a.ctx, "progress:scan", map[string]interface{}{
		"stage":   "reading",
		"current": 0,
		"total":   0,
		"message": "Reading directory",
	})

	result, err := scan.ScanFolder(dir)
	if err != nil {
		runtime.EventsEmit(a.ctx, "progress:scan", map[string]interface{}{"stage": "done"})
		return nil, err
	}

	if a.extractor != nil {
		if err := a.scanEmbeddedTracks(dir, result); err != nil {
			runtime.EventsEmit(a.ctx, "progress:scan", map[string]interface{}{"stage": "done"})
			return result, nil
		}
	}

	runtime.EventsEmit(a.ctx, "progress:scan", map[string]interface{}{"stage": "done"})
	return result, nil
}
```

Now update `scanEmbeddedTracks` to emit per-video progress. Find the loop over video files and wrap:
```go
func (a *App) scanEmbeddedTracks(dir string, result *scan.FolderScanResult) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true,
	}

	// First pass: collect video files so we know the total
	var videoFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if videoExts[ext] {
			videoFiles = append(videoFiles, entry.Name())
		}
	}

	for i, name := range videoFiles {
		runtime.EventsEmit(a.ctx, "progress:scan", map[string]interface{}{
			"stage":   "probing",
			"current": i + 1,
			"total":   len(videoFiles),
			"message": fmt.Sprintf("Probing %s", name),
		})

		videoPath := filepath.Join(dir, name)
		tracks, err := a.extractor.ListTracks(context.Background(), videoPath)
		if err != nil || len(tracks) == 0 {
			continue
		}

		result.Files = append(result.Files, scan.ScannedFile{
			Path:      videoPath,
			VideoPath: videoPath,
			Type:      "embedded",
			Tracks:    tracks,
		})
	}
	return nil
}
```

- [ ] **Step 2: Subscribe to scan events in MainView**

In `frontend/src/views/MainView.vue`, add import:
```typescript
import { useProgressStore } from '@/stores/progress'
```

Add to setup:
```typescript
const progressStore = useProgressStore()
```

In `onMounted`, add event subscription:
```typescript
  window.runtime.EventsOn('progress:scan', (data: unknown) => {
    const d = data as { stage: string; current?: number; total?: number; message?: string }
    if (d.stage === 'done') {
      progressStore.finishScan()
    } else {
      progressStore.updateScan(d.current ?? 0, d.total ?? 0, d.message ?? '')
      progressStore.scan.active = true
    }
  })
```

- [ ] **Step 3: Show progress in Toolbar**

In `frontend/src/components/Toolbar.vue`, add to `<script setup>` imports:
```typescript
import { useProgressStore } from '@/stores/progress'
import { NProgress } from 'naive-ui'
```

Add to setup:
```typescript
const progressStore = useProgressStore()
```

In the template, after the existing button group and before the locale select, add:
```vue
    <div v-if="progressStore.scan.active" class="scan-progress">
      <NProgress
        type="line"
        :percentage="progressStore.scan.total > 0
          ? Math.round((progressStore.scan.current / progressStore.scan.total) * 100)
          : 0"
        :indicator-placement="'inside'"
        :height="14"
        style="width: 200px"
      />
      <span class="scan-message">{{ progressStore.scan.message }}</span>
    </div>
```

Add styles:
```css
.scan-progress {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: 16px;
}
.scan-message {
  font-size: 11px;
  color: var(--n-text-color-3, #888);
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

- [ ] **Step 4: Verify build**

Run:
```bash
go build .
cd frontend && npx vue-tsc --noEmit
```
Expected: both succeed.

- [ ] **Step 5: Commit**

```bash
git add app.go frontend/src/views/MainView.vue frontend/src/components/Toolbar.vue
git commit -m "feat: scan progress events + inline toolbar progress bar"
```

- [ ] **Step 6: Rebuild Windows exe**

Run:
```bash
/home/hakastein/go/bin/wails build -platform windows/amd64
```

---

### Task 8: Preview Progress Overlay

**Files:**
- Modify: `frontend/src/components/PreviewArea.vue`

- [ ] **Step 1: Hook progress store into PreviewArea**

In `PreviewArea.vue`, add to imports:
```typescript
import { useProgressStore } from '@/stores/progress'
```

Add in setup:
```typescript
const progressStore = useProgressStore()
```

Wrap the `generatePreview` function to toggle the preview busy flag:
```typescript
async function generatePreview() {
  const file = activeFile.value
  const style = currentStyle.value

  if (!file) { debug.info('preview: skip — no active file'); return }
  if (!style) { debug.info('preview: skip — no style selected'); return }
  if (!file.videoPath) { debug.warn('preview: skip — no videoPath on file ' + file.id); return }
  if (!previewStore.ffmpegReady) { debug.warn('preview: skip — ffmpeg not ready'); return }

  const atMs = currentEvent.value
    ? durationToMs(currentEvent.value.startTime)
    : previewStore.currentTimeMs

  debug.info(`preview: requesting frame file=${file.id} video=${file.videoPath} at=${atMs}ms styles=${file.modifiedStyles.length} widthPx=${previewWidthPx.value}`)
  previewStore.setLoading(true)
  progressStore.startPreview()
  try {
    const result = await editorService.generatePreviewFrame(
      file.id,
      file.videoPath,
      file.modifiedStyles,
      atMs,
      previewWidthPx.value,
    )
    debug.info(`preview: frame received, base64 length=${result.base64Png.length} tc=${result.timecode}`)
    previewStore.setFrame(result.base64Png, result.timecode)
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    debug.error(`preview: frame generation failed — ${msg}`)
    previewStore.setLoading(false)
  } finally {
    progressStore.finishPreview()
  }
}
```

- [ ] **Step 2: Add progress overlay bar in template**

In `PreviewArea.vue` template, inside `.preview-frame-container` but above any image or overlay:
```vue
    <div class="preview-frame-container" ref="frameContainerRef">
      <div v-if="progressStore.preview.busy" class="preview-progress-bar">
        <div class="preview-progress-stripe"></div>
      </div>
      <!-- existing contents: loading spinner, image, etc -->
```

Add CSS in `<style scoped>`:
```css
.preview-progress-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: #0a2540;
  overflow: hidden;
  z-index: 5;
}

.preview-progress-stripe {
  width: 35%;
  height: 100%;
  background: linear-gradient(90deg, transparent, #2080f0, transparent);
  animation: preview-stripe 1.2s linear infinite;
}

@keyframes preview-stripe {
  from { transform: translateX(-100%); }
  to { transform: translateX(285%); }
}
```

- [ ] **Step 3: Verify build**

Run:
```bash
cd frontend && npx vue-tsc --noEmit
```

- [ ] **Step 4: Commit and rebuild**

```bash
git add frontend/src/components/PreviewArea.vue
git commit -m "feat: animated progress bar over preview while ffmpeg renders"
/home/hakastein/go/bin/wails build -platform windows/amd64
```

---

### Task 9: Translation Data Model (Store Refactor)

**Files:**
- Modify: `frontend/src/services/types.ts` (add Translation type)
- Modify: `frontend/src/stores/project.ts` (translation-first state)

- [ ] **Step 1: Add Translation type**

In `frontend/src/services/types.ts`, add at the end (before the helpers):
```typescript
export interface TranslationInstance {
  videoPath: string
  subtitlePath?: string
  trackIndex?: number
  trackTitle?: string
}

export interface Translation {
  key: string
  label: string
  kind: 'external' | 'embedded'
  perEpisode: Record<string, TranslationInstance> // videoPath → instance
  coverageCount: number
  totalEpisodes: number
}
```

- [ ] **Step 2: Update project store state and computed**

In `frontend/src/stores/project.ts`:

Replace the block:
```typescript
  // New video-centric state for the split FileTree UI
  const fileChecks = ref<Map<string, boolean>>(new Map()) // videoPath → checked
  const selectedSourceKey = ref<string | null>(null)
  const sourceLoadingState = ref<'idle' | 'loading'>('idle')
```
With:
```typescript
  // Translation-first selection state
  const episodeChecks = ref<Map<string, boolean>>(new Map()) // videoPath → checked
  const selectedTranslationKeys = ref<string[]>([])
  const sourceLoadingState = ref<'idle' | 'loading'>('idle')
```

Add to imports at top of file:
```typescript
import type { Translation, TranslationInstance } from '@/services/types'
```

Replace the `sourceTypes` computed (and related types `SourceType`, `SourceTypeInstance`) with a new `translations` computed:
```typescript
  // Available translations across all videos in the folder.
  const translations = computed<Translation[]>(() => {
    const totalVideos = videoEntries.value.length
    const byKey = new Map<string, Translation>()

    for (const sf of scannedFiles.value) {
      if (sf.type === 'external' && sf.videoPath) {
        const suffix = subtitleSuffix(sf.videoPath, sf.path)
        const key = `ext:${suffix}`
        if (!byKey.has(key)) {
          byKey.set(key, {
            key,
            label: suffix || sf.path,
            kind: 'external',
            perEpisode: {},
            coverageCount: 0,
            totalEpisodes: totalVideos,
          })
        }
        const t = byKey.get(key)!
        t.perEpisode[sf.videoPath] = { videoPath: sf.videoPath, subtitlePath: sf.path }
      } else if (sf.type === 'embedded') {
        for (const track of sf.tracks) {
          const title = track.title || `Track ${track.index}`
          const key = `emb:${title}:${track.language}`
          if (!byKey.has(key)) {
            byKey.set(key, {
              key,
              label: track.language ? `${title} (${track.language})` : title,
              kind: 'embedded',
              perEpisode: {},
              coverageCount: 0,
              totalEpisodes: totalVideos,
            })
          }
          const t = byKey.get(key)!
          t.perEpisode[sf.videoPath] = {
            videoPath: sf.videoPath,
            trackIndex: track.index,
            trackTitle: track.title,
          }
        }
      }
    }

    for (const t of byKey.values()) {
      t.coverageCount = Object.keys(t.perEpisode).length
    }

    return Array.from(byKey.values()).sort((a, b) => b.coverageCount - a.coverageCount || a.label.localeCompare(b.label))
  })
```

Delete the old `sourceTypes` computed and `SourceType` / `SourceTypeInstance` types/interfaces.

Replace the `groupedStyles` computed to iterate the new state:
```typescript
  // Styles grouped by name for the currently selected translations × checked episodes.
  const groupedStyles = computed<GroupedStyle[]>(() => {
    if (selectedTranslationKeys.value.length === 0) return []

    const groups = new Map<string, GroupedStyle>()

    for (const transKey of selectedTranslationKeys.value) {
      const trans = translations.value.find(t => t.key === transKey)
      if (!trans) continue

      for (const entry of videoEntries.value) {
        const checked = episodeChecks.value.get(entry.videoPath) ?? false
        if (!checked) continue
        const inst = trans.perEpisode[entry.videoPath]
        if (!inst) continue

        const loaded = findLoadedForInstance(inst, trans.kind)
        if (!loaded) continue

        for (const style of loaded.modifiedStyles) {
          if (!groups.has(style.name)) {
            groups.set(style.name, {
              styleName: style.name,
              representative: style,
              instances: [],
              episodesLabel: '',
            })
          }
          groups.get(style.name)!.instances.push({
            videoPath: entry.videoPath,
            episode: entry.episode,
            fileId: loaded.id,
            styleName: style.name,
          })
        }
      }
    }

    for (const group of groups.values()) {
      const eps = group.instances.map(i => i.episode).filter((e): e is number => e !== null)
      group.episodesLabel = eps.length > 0 ? collapseRanges(eps) : ''
    }

    return Array.from(groups.values()).sort((a, b) => a.styleName.localeCompare(b.styleName))
  })

  function findLoadedForInstance(inst: TranslationInstance, kind: 'external' | 'embedded') {
    if (kind === 'external' && inst.subtitlePath) {
      return loadedFiles.value.get(basename(inst.subtitlePath)) ?? null
    }
    if (kind === 'embedded' && inst.trackIndex !== undefined) {
      return loadedFiles.value.get(`${basename(inst.videoPath)}:track:${inst.trackIndex}`) ?? null
    }
    return null
  }
```

Replace `loadSourceStyles` with a new `loadTranslationStyles`:
```typescript
  /** Load subtitles for all selected translations × checked episodes. */
  async function loadTranslationStyles(): Promise<void> {
    if (selectedTranslationKeys.value.length === 0) return

    const progress = useProgressStore()

    // Count total loads needed upfront
    const toLoad: Array<{ trans: Translation; inst: TranslationInstance }> = []
    for (const key of selectedTranslationKeys.value) {
      const trans = translations.value.find(t => t.key === key)
      if (!trans) continue
      for (const entry of videoEntries.value) {
        const checked = episodeChecks.value.get(entry.videoPath) ?? false
        if (!checked) continue
        const inst = trans.perEpisode[entry.videoPath]
        if (!inst) continue
        if (findLoadedForInstance(inst, trans.kind)) continue
        toLoad.push({ trans, inst })
      }
    }

    if (toLoad.length === 0) return

    sourceLoadingState.value = 'loading'
    progress.startLoad(`Loading styles`, toLoad.length)
    try {
      for (let i = 0; i < toLoad.length; i++) {
        const { trans, inst } = toLoad[i]
        progress.updateLoad(i, toLoad.length, `Loading ${basename(inst.videoPath)}`)

        if (trans.kind === 'external' && inst.subtitlePath) {
          const scanned = scannedFiles.value.find(f => f.path === inst.subtitlePath)
          if (scanned) await loadFile(scanned)
        } else if (trans.kind === 'embedded' && inst.trackIndex !== undefined) {
          await extractTrack(inst.videoPath, inst.trackIndex, inst.trackTitle || '')
        }
      }
      progress.updateLoad(toLoad.length, toLoad.length, 'Done')
    } finally {
      sourceLoadingState.value = 'idle'
      progress.finishLoad()
    }
  }

  /** Translation selection handlers. */
  function selectTranslation(key: string, additive: boolean = false): void {
    if (additive) {
      const idx = selectedTranslationKeys.value.indexOf(key)
      if (idx >= 0) {
        selectedTranslationKeys.value = selectedTranslationKeys.value.filter(k => k !== key)
      } else {
        selectedTranslationKeys.value = [...selectedTranslationKeys.value, key]
      }
    } else {
      selectedTranslationKeys.value = [key]
    }
    // Recompute episode checks as union of covered videos across selected translations
    const coveredPaths = new Set<string>()
    for (const tk of selectedTranslationKeys.value) {
      const t = translations.value.find(x => x.key === tk)
      if (!t) continue
      for (const vp of Object.keys(t.perEpisode)) {
        coveredPaths.add(vp)
      }
    }
    const next = new Map<string, boolean>()
    for (const e of videoEntries.value) {
      next.set(e.videoPath, coveredPaths.has(e.videoPath))
    }
    episodeChecks.value = next
  }

  function toggleEpisode(videoPath: string, value: boolean): void {
    const next = new Map(episodeChecks.value)
    next.set(videoPath, value)
    episodeChecks.value = next
  }
```

Add import at top of file:
```typescript
import { useProgressStore } from './progress'
```

Update the watch to use the new state:
```typescript
  watch([selectedTranslationKeys, episodeChecks], () => {
    if (selectedTranslationKeys.value.length > 0) {
      loadTranslationStyles().catch((err: unknown) => {
        debug.error(`loadTranslationStyles failed: ${err}`)
      })
    }
  }, { deep: true })
```

Remove the old watch on `selectedSourceKey`/`fileChecks`.

Update `openFolder` to reset new state:
```typescript
    dirty.value = false
    episodeChecks.value = new Map()
    selectedTranslationKeys.value = []
    undoStore.clear()
```

Remove the "Initialize all video checkboxes as checked" block (episodes start empty now — they auto-fill when a translation is clicked).

Update the store's `return` object. Remove `fileChecks, selectedSourceKey, sourceTypes`. Add `episodeChecks, selectedTranslationKeys, translations, selectTranslation, toggleEpisode`.

- [ ] **Step 2: Update StatusBar to use new state**

In `frontend/src/components/StatusBar.vue`, replace the placeholder `translationsCount` computed:
```typescript
const translationsCount = computed(() => projectStore.selectedTranslationKeys.length)
```

Replace `episodesCount`:
```typescript
const episodesCount = computed(() => {
  const total = projectStore.videoEntries.length
  let checked = 0
  for (const [, v] of projectStore.episodeChecks) {
    if (v) checked++
  }
  return `${checked}/${total}`
})
```

- [ ] **Step 3: Verify TS compiles**

Run:
```bash
cd frontend && npx vue-tsc --noEmit
```
Expected: FilePanel.vue will show errors (expected — we refactor it next task). Everything else should compile.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/services/types.ts frontend/src/stores/project.ts frontend/src/components/StatusBar.vue
git commit -m "feat: translation-first state model in project store"
```

---

### Task 10: FilePanel Translation + Episodes Layout

**Files:**
- Modify: `frontend/src/components/FilePanel.vue` (rewrite)

- [ ] **Step 1: Rewrite FilePanel with new layout**

Replace `frontend/src/components/FilePanel.vue` entirely:
```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCheckbox, NEmpty, NScrollbar, NProgress } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { useDebugStore } from '@/stores/debug'
import { useProgressStore } from '@/stores/progress'

const { t } = useI18n()
const projectStore = useProjectStore()
const debug = useDebugStore()
const progressStore = useProgressStore()

// Translations: visible list (not dropdown). Ctrl/Cmd+click for multi-select.
function handleTranslationClick(key: string, event: MouseEvent) {
  const additive = event.ctrlKey || event.metaKey
  projectStore.selectTranslation(key, additive)
  debug.info(`FilePanel: translation ${key} ${additive ? '+add' : 'replace'}`)
}

function isTranslationSelected(key: string): boolean {
  return projectStore.selectedTranslationKeys.includes(key)
}

// Episodes: checkbox list.
function episodeIsChecked(videoPath: string): boolean {
  return projectStore.episodeChecks.get(videoPath) ?? false
}

function toggleEpisode(videoPath: string, value: boolean) {
  projectStore.toggleEpisode(videoPath, value)
}

const checkedCount = computed(() => {
  let n = 0
  for (const [, v] of projectStore.episodeChecks) {
    if (v) n++
  }
  return n
})

// Styles: grouped list with header progress bar.
function isStyleSelected(styleName: string): boolean {
  const group = projectStore.groupedStyles.find(g => g.styleName === styleName)
  if (!group) return false
  return group.instances.every(i =>
    projectStore.selectedStyleKeys.includes(`${i.fileId}::${i.styleName}`),
  )
}

function handleStyleClick(styleName: string, event: MouseEvent) {
  const additive = event.ctrlKey || event.metaKey
  projectStore.selectGroupedStyle(styleName, additive)
}

function styleInfo(style: { fontName: string; fontSize: number; bold: boolean; italic: boolean }) {
  const parts = [style.fontName, String(style.fontSize)]
  if (style.bold) parts.push('B')
  if (style.italic) parts.push('I')
  return parts.join(' · ')
}

const loadPercentage = computed(() => {
  const p = progressStore.load
  return p.total > 0 ? Math.round((p.current / p.total) * 100) : 0
})
</script>

<template>
  <div class="file-panel">
    <!-- Left sub-column: translations (top) + episodes (bottom) -->
    <div class="left-col">
      <!-- Translations -->
      <div class="translations-section">
        <div class="section-header">Translations</div>
        <NEmpty
          v-if="projectStore.translations.length === 0"
          description="No translations"
          size="small"
          style="padding: 12px"
        />
        <NScrollbar v-else style="flex: 1">
          <div
            v-for="trans in projectStore.translations"
            :key="trans.key"
            class="trans-row"
            :class="{ active: isTranslationSelected(trans.key) }"
            @click="handleTranslationClick(trans.key, $event)"
          >
            <span class="trans-label" :title="trans.label">{{ trans.label }}</span>
            <span class="trans-coverage">{{ trans.coverageCount }}/{{ trans.totalEpisodes }}</span>
          </div>
        </NScrollbar>
      </div>

      <!-- Episodes -->
      <div class="episodes-section">
        <div class="section-header">
          <span>Episodes</span>
          <span class="header-muted">{{ checkedCount }}/{{ projectStore.videoEntries.length }}</span>
        </div>
        <NEmpty
          v-if="projectStore.videoEntries.length === 0"
          :description="t('fileTree.noFiles')"
          size="small"
          style="padding: 12px"
        />
        <NScrollbar v-else style="flex: 1">
          <label
            v-for="entry in projectStore.videoEntries"
            :key="entry.videoPath"
            class="ep-row"
            :class="{ disabled: !episodeIsChecked(entry.videoPath) }"
          >
            <NCheckbox
              :checked="episodeIsChecked(entry.videoPath)"
              @update:checked="(v: boolean) => toggleEpisode(entry.videoPath, v)"
            />
            <span v-if="entry.episode !== null" class="ep-badge">
              {{ String(entry.episode).padStart(2, '0') }}
            </span>
            <span class="ep-name" :title="entry.videoPath">{{ entry.videoName }}</span>
          </label>
        </NScrollbar>
      </div>
    </div>

    <!-- Right sub-column: styles -->
    <div class="styles-col">
      <div class="section-header styles-header">
        <span>Styles</span>
        <NProgress
          v-if="progressStore.load.active"
          type="line"
          :percentage="loadPercentage"
          :show-indicator="false"
          :height="4"
          style="flex: 1"
        />
        <span v-else class="header-muted">
          {{ projectStore.groupedStyles.length }}
        </span>
      </div>

      <div v-if="progressStore.load.active" class="load-message">
        {{ progressStore.load.message }}
      </div>

      <NEmpty
        v-else-if="projectStore.selectedTranslationKeys.length === 0"
        description="Pick a translation to see styles"
        size="small"
        style="padding: 20px 12px"
      />

      <NEmpty
        v-else-if="projectStore.groupedStyles.length === 0"
        description="No styles loaded"
        size="small"
        style="padding: 20px 12px"
      />

      <NScrollbar v-else style="flex: 1">
        <div
          v-for="group in projectStore.groupedStyles"
          :key="group.styleName"
          class="style-row"
          :class="{ active: isStyleSelected(group.styleName) }"
          @click="handleStyleClick(group.styleName, $event)"
        >
          <div class="style-main">
            <span class="style-name">{{ group.styleName }}</span>
            <span v-if="group.episodesLabel" class="episodes-label">
              ep {{ group.episodesLabel }}
            </span>
          </div>
          <div class="style-info">
            {{ styleInfo(group.representative) }}
            <span class="instance-count">({{ group.instances.length }})</span>
          </div>
        </div>
      </NScrollbar>
    </div>
  </div>
</template>

<style scoped>
.file-panel {
  display: flex;
  height: 100%;
  overflow: hidden;
}

.left-col {
  width: 260px;
  border-right: 1px solid var(--n-border-color, #e0e0e6);
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex-shrink: 0;
}

.translations-section {
  flex: 0 0 40%;
  border-bottom: 1px solid var(--n-border-color, #e0e0e6);
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.episodes-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.styles-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}

.section-header {
  padding: 6px 10px;
  font-weight: 600;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e6);
  background: var(--n-color, #fff);
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.styles-header {
  gap: 10px;
}

.header-muted {
  font-weight: 400;
  color: var(--n-text-color-3, #888);
}

.load-message {
  padding: 6px 10px;
  font-size: 11px;
  color: var(--n-text-color-3, #888);
}

/* Translations */
.trans-row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  padding: 5px 10px;
  cursor: pointer;
  font-size: 12px;
  border-left: 3px solid transparent;
}
.trans-row:hover {
  background: var(--n-color-hover, rgba(0, 0, 0, 0.04));
}
.trans-row.active {
  background: var(--n-color-target-hover, rgba(32, 128, 240, 0.12));
  border-left-color: var(--n-color-target, #2080f0);
}
.trans-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}
.trans-coverage {
  color: var(--n-text-color-3, #888);
  font-size: 11px;
  white-space: nowrap;
}

/* Episodes */
.ep-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 10px;
  cursor: pointer;
  font-size: 12px;
  line-height: 1.3;
}
.ep-row:hover {
  background: var(--n-color-hover, rgba(0, 0, 0, 0.04));
}
.ep-row.disabled {
  opacity: 0.5;
}
.ep-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ep-badge {
  display: inline-block;
  background: var(--n-color-target, #2080f0);
  color: white;
  font-size: 10px;
  font-weight: 600;
  padding: 1px 4px;
  border-radius: 3px;
}

/* Styles */
.style-row {
  display: flex;
  flex-direction: column;
  padding: 6px 10px;
  cursor: pointer;
  border-left: 2px solid transparent;
}
.style-row:hover {
  background: var(--n-color-hover, rgba(0, 0, 0, 0.04));
}
.style-row.active {
  background: var(--n-color-target-hover, rgba(32, 128, 240, 0.12));
  border-left-color: var(--n-color-target, #2080f0);
}
.style-main {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
}
.style-name {
  font-weight: 600;
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.episodes-label {
  font-size: 11px;
  color: var(--n-text-color-3, #888);
  white-space: nowrap;
}
.style-info {
  font-size: 11px;
  color: var(--n-text-color-3, #888);
  margin-top: 2px;
}
.instance-count {
  margin-left: 6px;
  opacity: 0.7;
}
</style>
```

- [ ] **Step 2: Verify TS compiles**

Run:
```bash
cd frontend && npx vue-tsc --noEmit
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/FilePanel.vue
git commit -m "feat: FilePanel shows translations list + episodes list + styles"
```

- [ ] **Step 4: Rebuild Windows exe**

Run:
```bash
/home/hakastein/go/bin/wails build -platform windows/amd64
```

---

### Task 11: Full Verification

**Files:** No new files — verification only.

- [ ] **Step 1: Run all Go tests**

Run:
```bash
go test ./... -v
```
Expected: every package passes. Specifically verify:
- `internal/i18n/` — normalizeBCP47 tests pass
- `internal/preview/` — cache round-trip and LRU eviction pass
- `internal/ffmpeg/` — frame args, base frame args, overlay args tests pass

- [ ] **Step 2: Verify Go builds for both Linux and Windows**

Run:
```bash
go build ./...
GOOS=windows GOARCH=amd64 go build ./...
```
Expected: both succeed.

- [ ] **Step 3: Verify frontend builds**

Run:
```bash
cd frontend && npx vue-tsc --noEmit && npm run build
```
Expected: succeeds.

- [ ] **Step 4: Full Wails build for Windows**

Run:
```bash
/home/hakastein/go/bin/wails build -platform windows/amd64
```
Expected: produces `build/bin/subtitles-editor.exe`.

- [ ] **Step 5: Smoke-check each feature in the running app**

Manual verification on Windows:
- Language: open app → check that UI language matches system locale.
- Status bar: always visible at bottom; click toggles debug log; F12 also toggles.
- Open a folder (one with MKVs) → observe scan progress in toolbar during track probing.
- Pick a translation → episodes auto-fill. Ctrl+click another translation → union expands.
- While styles load, see the thin blue bar in Styles header; message shows current file.
- Click a style → preview renders. Observe thin top bar during render.
- Edit the style → re-render is near-instant (cache hit on base frame).
- Resize window → preview re-renders after 200ms debounce at new width.
- Save → embedded writes go to `<video>.<title>.ass`.
