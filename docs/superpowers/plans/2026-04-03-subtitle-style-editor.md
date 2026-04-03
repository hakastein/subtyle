# Subtitle Style Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a desktop app for visually editing ASS/SSA subtitle styles across a folder of video/subtitle files, with real-time preview, undo/redo, and session autosave.

**Architecture:** Go + Wails v2 backend handles file I/O, ffmpeg operations, and project serialization (gob). Vue 3 + Pinia frontend owns the working copy of styles and undo stack for instant UI. Sync via debounced Wails calls. Three-column layout: file tree | preview | style editor.

**Tech Stack:** Go 1.22+, Wails v2, go-astisub, ffmpeg (auto-download), Vue 3 (Composition API), TypeScript, Pinia, Naive UI, vue-i18n, vitest.

**Spec:** `docs/superpowers/specs/2026-04-03-subtitle-style-editor-design.md`

---

## File Map

### Go Backend

| File | Responsibility |
|---|---|
| `cmd/app/main.go` | Wails app entry, bind all Go structs |
| `internal/parser/types.go` | Domain DTOs: SubtitleStyle, Color, SubtitleEvent, SubtitleFile (scan types live in scan package) |
| `internal/parser/color.go` | ASS `&HAABBGGRR` ↔ RGBA conversion |
| `internal/parser/color_test.go` | Tests for color conversion |
| `internal/parser/parser.go` | Parse ASS/SSA via go-astisub → DTOs |
| `internal/parser/parser_test.go` | Tests with sample ASS files |
| `internal/parser/writer.go` | Write DTOs → ASS/SSA files |
| `internal/parser/writer_test.go` | Round-trip tests: parse → modify → write → parse |
| `internal/parser/testdata/sample.ass` | Test fixture: minimal ASS file |
| `internal/scan/scan.go` | Scan folder for subtitle/video files, match by name |
| `internal/scan/scan_test.go` | Tests with temp directory fixtures |
| `internal/ffmpeg/ffmpeg.go` | Find ffmpeg in PATH, download if missing |
| `internal/ffmpeg/ffmpeg_test.go` | Tests for path resolution logic |
| `internal/ffmpeg/extract.go` | Frame extraction, track listing, track extraction |
| `internal/ffmpeg/extract_test.go` | Tests for command building (not actual ffmpeg) |
| `internal/editor/editor.go` | Apply style changes, generate temp ASS |
| `internal/editor/editor_test.go` | Tests for style mutation logic |
| `internal/project/project.go` | Gob serialization, autosave, restore |
| `internal/project/project_test.go` | Round-trip serialization tests |
| `internal/preview/preview.go` | Orchestrate preview: debounce + ffmpeg |
| `internal/i18n/i18n.go` | Detect system locale on Windows |
| `go.mod` | Module definition |
| `wails.json` | Wails project config |

### Vue Frontend

| File | Responsibility |
|---|---|
| `frontend/src/main.ts` | App entry, mount Vue + Pinia + i18n + Naive UI |
| `frontend/src/App.vue` | Root component, restore dialog gate |
| `frontend/src/i18n/en.json` | English translations |
| `frontend/src/i18n/ru.json` | Russian translations |
| `frontend/src/i18n/index.ts` | vue-i18n setup |
| `frontend/src/services/scan.ts` | Wails bindings: OpenFolder, ScanFolder |
| `frontend/src/services/parser.ts` | Wails bindings: ParseFile, SaveFile |
| `frontend/src/services/editor.ts` | Wails bindings: GeneratePreviewFrame |
| `frontend/src/services/ffmpeg.ts` | Wails bindings: GetFfmpegStatus |
| `frontend/src/services/project.ts` | Wails bindings: CheckAutosave, RestoreProject, SaveProject, Autosave |
| `frontend/src/services/types.ts` | Shared TypeScript types matching Go DTOs |
| `frontend/src/stores/project.ts` | Main store: files, styles, selection, dirty state |
| `frontend/src/stores/undo.ts` | Undo/redo stack |
| `frontend/src/stores/preview.ts` | Preview state: current frame, loading |
| `frontend/src/views/MainView.vue` | Three-column layout |
| `frontend/src/views/RestoreDialog.vue` | Session restore prompt |
| `frontend/src/components/Toolbar.vue` | Top bar: Open, Save, Undo, Redo, language |
| `frontend/src/components/FileTree.vue` | Left panel: file/style tree with multi-select |
| `frontend/src/components/StyleEditor.vue` | Right panel: style editing fields |
| `frontend/src/components/PreviewArea.vue` | Center: video frame + CSS subtitle overlay |
| `frontend/src/components/Timeline.vue` | Timeline bar with event markers |
| `frontend/src/components/CssSubtitleOverlay.vue` | CSS-approximated subtitle rendering |
| `frontend/package.json` | Frontend dependencies |
| `frontend/tsconfig.json` | TypeScript config |
| `frontend/vite.config.ts` | Vite config |

---

### Task 1: Wails v2 Project Scaffolding

**Files:**
- Create: `go.mod`, `wails.json`, `cmd/app/main.go`
- Create: `frontend/package.json`, `frontend/tsconfig.json`, `frontend/vite.config.ts`
- Create: `frontend/index.html`, `frontend/src/main.ts`, `frontend/src/App.vue`
- Create: `.gitignore`

- [ ] **Step 1: Install Wails CLI (if needed) and init project**

Run:
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

- [ ] **Step 2: Create go.mod**

```bash
go mod init subtitles-editor
```

- [ ] **Step 3: Create .gitignore**

Create `.gitignore`:
```gitignore
# Build
build/
dist/
node_modules/
frontend/dist/

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
Thumbs.db

# Wails
frontend/wailsjs/

# Superpowers
.superpowers/
```

- [ ] **Step 4: Create wails.json**

Create `wails.json`:
```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "subtitles-editor",
  "outputfilename": "subtitles-editor",
  "frontend:install": "npm install",
  "frontend:build": "npm run build",
  "frontend:dev:watcher": "npm run dev",
  "frontend:dev:serverUrl": "auto",
  "author": {
    "name": ""
  }
}
```

- [ ] **Step 5: Create cmd/app/main.go**

Create `cmd/app/main.go`:
```go
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := &App{}

	err := wails.Run(&options.App{
		Title:  "Subtitle Style Editor",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 6: Create App struct**

Create `cmd/app/app.go`:
```go
package main

import (
	"context"
)

// App holds the application state and exposes methods to the frontend.
type App struct {
	ctx context.Context
}

// startup is called when the app starts. The context is stored
// so we can call the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}
```

- [ ] **Step 7: Create frontend/package.json**

Create `frontend/package.json`:
```json
{
  "name": "subtitles-editor-frontend",
  "private": true,
  "version": "0.0.1",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "test": "vitest run",
    "test:watch": "vitest"
  },
  "dependencies": {
    "vue": "^3.4.0",
    "pinia": "^2.1.0",
    "naive-ui": "^2.38.0",
    "vue-i18n": "^9.10.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.0.0",
    "typescript": "^5.4.0",
    "vite": "^5.2.0",
    "vue-tsc": "^2.0.0",
    "vitest": "^1.4.0",
    "@vue/test-utils": "^2.4.0",
    "jsdom": "^24.0.0"
  }
}
```

- [ ] **Step 8: Create frontend/tsconfig.json**

Create `frontend/tsconfig.json`:
```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "esModuleInterop": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "skipLibCheck": true,
    "noEmit": true,
    "paths": {
      "@/*": ["./src/*"]
    },
    "baseUrl": "."
  },
  "include": ["src/**/*.ts", "src/**/*.vue"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

Create `frontend/tsconfig.node.json`:
```json
{
  "compilerOptions": {
    "composite": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 9: Create frontend/vite.config.ts**

Create `frontend/vite.config.ts`:
```typescript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  test: {
    environment: 'jsdom',
  },
})
```

- [ ] **Step 10: Create frontend/index.html**

Create `frontend/index.html`:
```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Subtitle Style Editor</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 11: Create minimal frontend/src/main.ts and App.vue**

Create `frontend/src/env.d.ts`:
```typescript
/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}
```

Create `frontend/src/App.vue`:
```vue
<script setup lang="ts">
</script>

<template>
  <n-config-provider>
    <n-message-provider>
      <div id="app-root">
        <p>Subtitle Style Editor — scaffolding works</p>
      </div>
    </n-message-provider>
  </n-config-provider>
</template>
```

Create `frontend/src/main.ts`:
```typescript
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'

const app = createApp(App)
app.use(createPinia())
app.mount('#app')
```

- [ ] **Step 12: Install Go dependencies and verify build**

Run:
```bash
cd /home/hakastein/work/subtitles
go get github.com/wailsapp/wails/v2
go mod tidy
```

Run:
```bash
cd frontend && npm install && cd ..
```

Run:
```bash
wails build
```

Expected: builds without errors, produces `build/bin/subtitles-editor` (or `.exe` on Windows).

- [ ] **Step 13: Commit**

```bash
git add .
git commit -m "feat: scaffold Wails v2 project with Vue 3 + Pinia + Naive UI"
```

---

### Task 2: Domain Types and Color Conversion

**Files:**
- Create: `internal/parser/types.go`
- Create: `internal/parser/color.go`
- Create: `internal/parser/color_test.go`

- [ ] **Step 1: Write color conversion tests**

Create `internal/parser/color_test.go`:
```go
package parser

import (
	"testing"
)

func TestParseASSColor(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Color
	}{
		{
			name:  "white opaque",
			input: "&H00FFFFFF",
			want:  Color{R: 255, G: 255, B: 255, A: 255},
		},
		{
			name:  "red opaque",
			input: "&H000000FF",
			want:  Color{R: 255, G: 0, B: 0, A: 255},
		},
		{
			name:  "blue opaque",
			input: "&H00FF0000",
			want:  Color{R: 0, G: 0, B: 255, A: 255},
		},
		{
			name:  "green half transparent",
			input: "&H8000FF00",
			want:  Color{R: 0, G: 255, B: 0, A: 127},
		},
		{
			name:  "fully transparent black",
			input: "&HFF000000",
			want:  Color{R: 0, G: 0, B: 0, A: 0},
		},
		{
			name:  "lowercase",
			input: "&h00ffffff",
			want:  Color{R: 255, G: 255, B: 255, A: 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseASSColor(tt.input)
			if err != nil {
				t.Fatalf("ParseASSColor(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseASSColor(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseASSColorInvalid(t *testing.T) {
	invalids := []string{"", "FFFFFF", "&HGGGGGGGG", "&H00FF"}
	for _, s := range invalids {
		_, err := ParseASSColor(s)
		if err == nil {
			t.Errorf("ParseASSColor(%q) should return error", s)
		}
	}
}

func TestFormatASSColor(t *testing.T) {
	tests := []struct {
		name  string
		input Color
		want  string
	}{
		{
			name:  "white opaque",
			input: Color{R: 255, G: 255, B: 255, A: 255},
			want:  "&H00FFFFFF",
		},
		{
			name:  "red opaque",
			input: Color{R: 255, G: 0, B: 0, A: 255},
			want:  "&H000000FF",
		},
		{
			name:  "green half transparent",
			input: Color{R: 0, G: 255, B: 0, A: 127},
			want:  "&H8000FF00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatASSColor(tt.input)
			if got != tt.want {
				t.Errorf("FormatASSColor(%+v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestColorRoundTrip(t *testing.T) {
	colors := []Color{
		{R: 0, G: 0, B: 0, A: 255},
		{R: 255, G: 255, B: 255, A: 0},
		{R: 128, G: 64, B: 32, A: 200},
	}
	for _, c := range colors {
		s := FormatASSColor(c)
		got, err := ParseASSColor(s)
		if err != nil {
			t.Fatalf("round trip failed for %+v: %v", c, err)
		}
		if got != c {
			t.Errorf("round trip %+v → %q → %+v", c, s, got)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/parser/ -v`
Expected: compilation error — types and functions not defined yet.

- [ ] **Step 3: Create domain types**

Create `internal/parser/types.go`:
```go
// Package parser handles parsing and writing ASS/SSA subtitle files.
// It defines clean domain types and maps to/from go-astisub types.
package parser

import "time"

// Color represents an RGBA color value.
type Color struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
	A uint8 `json:"a"` // 255 = fully opaque, 0 = fully transparent
}

// SubtitleStyle represents a named style from an ASS/SSA subtitle file.
type SubtitleStyle struct {
	Name            string  `json:"name"`
	FontName        string  `json:"fontName"`
	FontSize        float64 `json:"fontSize"`
	Bold            bool    `json:"bold"`
	Italic          bool    `json:"italic"`
	Underline       bool    `json:"underline"`
	Strikeout       bool    `json:"strikeout"`
	PrimaryColour   Color   `json:"primaryColour"`
	SecondaryColour Color   `json:"secondaryColour"`
	OutlineColour   Color   `json:"outlineColour"`
	BackColour      Color   `json:"backColour"`
	Outline         float64 `json:"outline"`
	Shadow          float64 `json:"shadow"`
	ScaleX          float64 `json:"scaleX"`
	ScaleY          float64 `json:"scaleY"`
	Spacing         float64 `json:"spacing"`
	Angle           float64 `json:"angle"`
	Alignment       int     `json:"alignment"` // ASS numpad 1-9
	MarginL         int     `json:"marginL"`
	MarginR         int     `json:"marginR"`
	MarginV         int     `json:"marginV"`
}

// SubtitleEvent represents a single subtitle dialogue line.
type SubtitleEvent struct {
	StyleName string        `json:"styleName"`
	StartTime time.Duration `json:"startTime"`
	EndTime   time.Duration `json:"endTime"`
	Text      string        `json:"text"`
}

// SubtitleFile holds parsed data from a single subtitle file.
type SubtitleFile struct {
	ID      string          `json:"id"`
	Path    string          `json:"path"`
	Source  string          `json:"source"` // "external" or "embedded"
	TrackID int             `json:"trackId"`
	Styles  []SubtitleStyle `json:"styles"`
	Events  []SubtitleEvent `json:"events"`
}

// Note: ScannedFile, TrackInfo, FolderScanResult are defined in the scan package.
```

- [ ] **Step 4: Implement color conversion**

Create `internal/parser/color.go`:
```go
package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseASSColor parses an ASS color string in &HAABBGGRR format to an RGBA Color.
// ASS alpha: 00 = opaque, FF = transparent (inverted from standard RGBA).
func ParseASSColor(s string) (Color, error) {
	s = strings.TrimSpace(s)
	upper := strings.ToUpper(s)

	if !strings.HasPrefix(upper, "&H") {
		return Color{}, fmt.Errorf("parse ASS color %q: missing &H prefix", s)
	}

	hex := upper[2:]
	if len(hex) != 8 {
		return Color{}, fmt.Errorf("parse ASS color %q: expected 8 hex digits, got %d", s, len(hex))
	}

	val, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("parse ASS color %q: %w", s, err)
	}

	aa := uint8((val >> 24) & 0xFF)
	bb := uint8((val >> 16) & 0xFF)
	gg := uint8((val >> 8) & 0xFF)
	rr := uint8(val & 0xFF)

	return Color{
		R: rr,
		G: gg,
		B: bb,
		A: 255 - aa, // invert: ASS 00=opaque → our 255=opaque
	}, nil
}

// FormatASSColor converts an RGBA Color to ASS &HAABBGGRR format.
func FormatASSColor(c Color) string {
	aa := 255 - c.A // invert back
	return fmt.Sprintf("&H%02X%02X%02X%02X", aa, c.B, c.G, c.R)
}
```

- [ ] **Step 5: Run tests**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/parser/ -v`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/parser/
git commit -m "feat: add domain types and ASS color conversion with tests"
```

---

### Task 3: ASS Parser

**Files:**
- Create: `internal/parser/parser.go`
- Create: `internal/parser/parser_test.go`
- Create: `internal/parser/testdata/sample.ass`

- [ ] **Step 1: Create test fixture**

Create `internal/parser/testdata/sample.ass`:
```
[Script Info]
ScriptType: v4.00+
PlayResX: 1920
PlayResY: 1080

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,48,&H00FFFFFF,&H000000FF,&H00000000,&H80000000,-1,0,0,0,100,100,0,0,1,2,1,2,10,10,10,1
Style: Signs,Impact,36,&H00FFFFFF,&H000000FF,&H003C0000,&H00000000,0,0,0,0,100,100,0,0,1,3,0,8,10,10,10,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:05.00,Default,,0,0,0,,Hello world
Dialogue: 0,0:00:10.00,0:00:15.00,Default,,0,0,0,,Second line
Dialogue: 0,0:01:00.00,0:01:05.00,Signs,,0,0,0,,Sign text here
```

- [ ] **Step 2: Write parser tests**

Create `internal/parser/parser_test.go`:
```go
package parser

import (
	"path/filepath"
	"testing"
	"time"
)

func TestParseFile(t *testing.T) {
	path := filepath.Join("testdata", "sample.ass")
	result, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile(%q) error: %v", path, err)
	}

	if len(result.Styles) != 2 {
		t.Fatalf("expected 2 styles, got %d", len(result.Styles))
	}

	// Check Default style
	def := result.Styles[0]
	if def.Name != "Default" {
		t.Errorf("style[0].Name = %q, want %q", def.Name, "Default")
	}
	if def.FontName != "Arial" {
		t.Errorf("style[0].FontName = %q, want %q", def.FontName, "Arial")
	}
	if def.FontSize != 48 {
		t.Errorf("style[0].FontSize = %v, want 48", def.FontSize)
	}
	if !def.Bold {
		t.Error("style[0].Bold should be true (ASS -1 = bold)")
	}
	if def.Italic {
		t.Error("style[0].Italic should be false")
	}
	if def.Alignment != 2 {
		t.Errorf("style[0].Alignment = %d, want 2", def.Alignment)
	}
	if def.Outline != 2 {
		t.Errorf("style[0].Outline = %v, want 2", def.Outline)
	}
	if def.PrimaryColour != (Color{R: 255, G: 255, B: 255, A: 255}) {
		t.Errorf("style[0].PrimaryColour = %+v, want white opaque", def.PrimaryColour)
	}

	// Check Signs style
	signs := result.Styles[1]
	if signs.Name != "Signs" {
		t.Errorf("style[1].Name = %q, want %q", signs.Name, "Signs")
	}
	if signs.FontName != "Impact" {
		t.Errorf("style[1].FontName = %q, want %q", signs.FontName, "Impact")
	}
	if signs.Alignment != 8 {
		t.Errorf("style[1].Alignment = %d, want 8", signs.Alignment)
	}

	// Check events
	if len(result.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(result.Events))
	}

	ev0 := result.Events[0]
	if ev0.StyleName != "Default" {
		t.Errorf("event[0].StyleName = %q, want %q", ev0.StyleName, "Default")
	}
	if ev0.StartTime != 1*time.Second {
		t.Errorf("event[0].StartTime = %v, want 1s", ev0.StartTime)
	}
	if ev0.EndTime != 5*time.Second {
		t.Errorf("event[0].EndTime = %v, want 5s", ev0.EndTime)
	}
	if ev0.Text != "Hello world" {
		t.Errorf("event[0].Text = %q, want %q", ev0.Text, "Hello world")
	}

	ev2 := result.Events[2]
	if ev2.StyleName != "Signs" {
		t.Errorf("event[2].StyleName = %q, want %q", ev2.StyleName, "Signs")
	}
}

func TestParseFileNotFound(t *testing.T) {
	_, err := ParseFile("testdata/nonexistent.ass")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/parser/ -run TestParseFile -v`
Expected: compilation error — `ParseFile` not defined.

- [ ] **Step 4: Implement parser**

Create `internal/parser/parser.go`:
```go
package parser

import (
	"fmt"
	"os"
	"time"

	astisub "github.com/asticode/go-astisub"
)

// ParseFile reads an ASS/SSA file and returns parsed subtitle data as domain types.
func ParseFile(path string) (*SubtitleFile, error) {
	subs, err := astisub.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("parse subtitle file %q: %w", path, err)
	}

	styles, err := mapStyles(subs)
	if err != nil {
		return nil, fmt.Errorf("map styles from %q: %w", path, err)
	}

	events := mapEvents(subs)

	return &SubtitleFile{
		ID:     path,
		Path:   path,
		Source: "external",
		Styles: styles,
		Events: events,
	}, nil
}

// ParseBytes reads ASS/SSA content from bytes and returns parsed subtitle data.
func ParseBytes(data []byte, id string) (*SubtitleFile, error) {
	tmpFile, err := os.CreateTemp("", "sub-*.ass")
	if err != nil {
		return nil, fmt.Errorf("create temp file for parsing: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp file for parsing: %w", err)
	}
	tmpFile.Close()

	result, err := ParseFile(tmpFile.Name())
	if err != nil {
		return nil, err
	}
	result.ID = id
	result.Source = "embedded"
	return result, nil
}

func mapStyles(subs *astisub.Subtitles) ([]SubtitleStyle, error) {
	if subs.Metadata == nil || len(subs.Metadata.ASSStyles) == 0 {
		return nil, nil
	}

	styles := make([]SubtitleStyle, 0, len(subs.Metadata.ASSStyles))
	for _, as := range subs.Metadata.ASSStyles {
		s := as.Style
		style, err := mapOneStyle(s)
		if err != nil {
			return nil, fmt.Errorf("map style %q: %w", s.Name, err)
		}
		styles = append(styles, style)
	}
	return styles, nil
}

func mapOneStyle(s astisub.Style) (SubtitleStyle, error) {
	primary, err := ParseASSColor(colorToString(s.InlineStyle.ASSPrimaryColour))
	if err != nil {
		primary = Color{R: 255, G: 255, B: 255, A: 255}
	}
	secondary, err := ParseASSColor(colorToString(s.InlineStyle.ASSSecondaryColour))
	if err != nil {
		secondary = Color{}
	}
	outline, err := ParseASSColor(colorToString(s.InlineStyle.ASSOutlineColour))
	if err != nil {
		outline = Color{}
	}
	back, err := ParseASSColor(colorToString(s.InlineStyle.ASSBackColour))
	if err != nil {
		back = Color{}
	}

	return SubtitleStyle{
		Name:            s.Name,
		FontName:        s.InlineStyle.ASSFontName,
		FontSize:        s.InlineStyle.ASSFontSize,
		Bold:            s.InlineStyle.ASSBold,
		Italic:          s.InlineStyle.ASSItalic,
		Underline:       s.InlineStyle.ASSUnderline,
		Strikeout:       s.InlineStyle.ASSStrikeout,
		PrimaryColour:   primary,
		SecondaryColour: secondary,
		OutlineColour:   outline,
		BackColour:      back,
		Outline:         s.InlineStyle.ASSOutline,
		Shadow:          s.InlineStyle.ASSShadow,
		ScaleX:          s.InlineStyle.ASSScaleX,
		ScaleY:          s.InlineStyle.ASSScaleY,
		Spacing:         s.InlineStyle.ASSSpacing,
		Angle:           s.InlineStyle.ASSAngle,
		Alignment:       s.InlineStyle.ASSAlignment,
		MarginL:         s.InlineStyle.ASSMarginLeft,
		MarginR:         s.InlineStyle.ASSMarginRight,
		MarginV:         s.InlineStyle.ASSMarginVertical,
	}, nil
}

func colorToString(c *astisub.Color) string {
	if c == nil {
		return "&H00000000"
	}
	return c.String()
}

func mapEvents(subs *astisub.Subtitles) []SubtitleEvent {
	events := make([]SubtitleEvent, 0, len(subs.Items))
	for _, item := range subs.Items {
		styleName := ""
		if item.InlineStyle != nil && item.InlineStyle.ASSEffect != "" {
			styleName = item.InlineStyle.ASSEffect
		}
		if item.Style != nil {
			styleName = item.Style.Name
		}

		text := ""
		for _, line := range item.Lines {
			for _, li := range line.Items {
				text += li.Text
			}
			text += "\\N"
		}
		if len(text) > 2 {
			text = text[:len(text)-2] // trim trailing \N
		}

		var start, end time.Duration
		if item.StartAt != nil {
			start = *item.StartAt
		}
		if item.EndAt != nil {
			end = *item.EndAt
		}

		events = append(events, SubtitleEvent{
			StyleName: styleName,
			StartTime: start,
			EndTime:   end,
			Text:      text,
		})
	}
	return events
}
```

- [ ] **Step 5: Get go-astisub dependency**

Run:
```bash
cd /home/hakastein/work/subtitles && go get github.com/asticode/go-astisub && go mod tidy
```

- [ ] **Step 6: Run tests and fix issues**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/parser/ -v`
Expected: all tests pass. If go-astisub's API differs (field names, color format), adjust `mapOneStyle` accordingly. The test fixture is a valid ASS file, so parsing should work.

**Important:** go-astisub's `Style` and `InlineStyle` field names may vary by version. If tests fail due to field access, check the go-astisub source:
```bash
go doc github.com/asticode/go-astisub.Style
go doc github.com/asticode/go-astisub.StyleAttributes
```
And adjust field mappings in `mapOneStyle` accordingly.

- [ ] **Step 7: Commit**

```bash
git add internal/parser/ go.mod go.sum
git commit -m "feat: add ASS/SSA parser with go-astisub mapping and tests"
```

---

### Task 4: ASS Writer

**Files:**
- Create: `internal/parser/writer.go`
- Create: `internal/parser/writer_test.go`

- [ ] **Step 1: Write round-trip tests**

Create `internal/parser/writer_test.go`:
```go
package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile(t *testing.T) {
	// Parse original
	path := filepath.Join("testdata", "sample.ass")
	original, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	// Write to temp
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "output.ass")
	err = WriteFile(outPath, original)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Verify file exists and is non-empty
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output file stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Parse the written file
	reparsed, err := ParseFile(outPath)
	if err != nil {
		t.Fatalf("ParseFile(output): %v", err)
	}

	// Compare styles
	if len(reparsed.Styles) != len(original.Styles) {
		t.Fatalf("style count: got %d, want %d", len(reparsed.Styles), len(original.Styles))
	}

	for i, orig := range original.Styles {
		got := reparsed.Styles[i]
		if got.Name != orig.Name {
			t.Errorf("style[%d].Name = %q, want %q", i, got.Name, orig.Name)
		}
		if got.FontName != orig.FontName {
			t.Errorf("style[%d].FontName = %q, want %q", i, got.FontName, orig.FontName)
		}
		if got.FontSize != orig.FontSize {
			t.Errorf("style[%d].FontSize = %v, want %v", i, got.FontSize, orig.FontSize)
		}
		if got.Bold != orig.Bold {
			t.Errorf("style[%d].Bold = %v, want %v", i, got.Bold, orig.Bold)
		}
		if got.Alignment != orig.Alignment {
			t.Errorf("style[%d].Alignment = %d, want %d", i, got.Alignment, orig.Alignment)
		}
		if got.PrimaryColour != orig.PrimaryColour {
			t.Errorf("style[%d].PrimaryColour = %+v, want %+v", i, got.PrimaryColour, orig.PrimaryColour)
		}
	}

	// Compare events
	if len(reparsed.Events) != len(original.Events) {
		t.Fatalf("event count: got %d, want %d", len(reparsed.Events), len(original.Events))
	}
	for i, orig := range original.Events {
		got := reparsed.Events[i]
		if got.StyleName != orig.StyleName {
			t.Errorf("event[%d].StyleName = %q, want %q", i, got.StyleName, orig.StyleName)
		}
	}
}

func TestWriteFileWithModifiedStyles(t *testing.T) {
	path := filepath.Join("testdata", "sample.ass")
	original, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	// Modify a style
	original.Styles[0].FontSize = 72
	original.Styles[0].FontName = "Verdana"
	original.Styles[0].Bold = false
	original.Styles[0].PrimaryColour = Color{R: 255, G: 0, B: 0, A: 255}

	// Write
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "modified.ass")
	err = WriteFile(outPath, original)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Re-parse and verify
	reparsed, err := ParseFile(outPath)
	if err != nil {
		t.Fatalf("ParseFile(modified): %v", err)
	}

	got := reparsed.Styles[0]
	if got.FontSize != 72 {
		t.Errorf("FontSize = %v, want 72", got.FontSize)
	}
	if got.FontName != "Verdana" {
		t.Errorf("FontName = %q, want %q", got.FontName, "Verdana")
	}
	if got.Bold {
		t.Error("Bold should be false after modification")
	}
	if got.PrimaryColour != (Color{R: 255, G: 0, B: 0, A: 255}) {
		t.Errorf("PrimaryColour = %+v, want red", got.PrimaryColour)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/parser/ -run TestWrite -v`
Expected: compilation error — `WriteFile` not defined.

- [ ] **Step 3: Implement writer**

Create `internal/parser/writer.go`:
```go
package parser

import (
	"fmt"
	"os"
	"time"

	astisub "github.com/asticode/go-astisub"
)

// WriteFile writes a SubtitleFile back to disk in ASS format.
// It reconstructs a go-astisub Subtitles object from the domain types
// and uses go-astisub's writer for correct ASS formatting.
func WriteFile(path string, sf *SubtitleFile) error {
	// Parse original to preserve metadata and event details
	var subs *astisub.Subtitles
	if sf.Path != "" {
		var err error
		subs, err = astisub.OpenFile(sf.Path)
		if err != nil {
			// If original is unreadable, build from scratch
			subs = astisub.NewSubtitles()
		}
	} else {
		subs = astisub.NewSubtitles()
	}

	// Update styles
	updateASSStyles(subs, sf.Styles)

	// Write to file
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", path, err)
	}
	defer f.Close()

	if err := subs.WriteToASS(f); err != nil {
		return fmt.Errorf("write ASS to %q: %w", path, err)
	}
	return nil
}

// WriteTempFile creates a temporary ASS file with modified styles.
// The caller is responsible for removing the file when done.
func WriteTempFile(sf *SubtitleFile) (string, error) {
	tmpFile, err := os.CreateTemp("", "sub-preview-*.ass")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	if err := WriteFile(tmpPath, sf); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

func updateASSStyles(subs *astisub.Subtitles, styles []SubtitleStyle) {
	if subs.Metadata == nil {
		subs.Metadata = &astisub.Metadata{}
	}

	assStyles := make([]astisub.ASSStyle, 0, len(styles))
	for _, s := range styles {
		as := astisub.ASSStyle{
			Style: astisub.Style{
				Name: s.Name,
				InlineStyle: &astisub.StyleAttributes{
					ASSFontName:       s.FontName,
					ASSFontSize:       s.FontSize,
					ASSBold:           s.Bold,
					ASSItalic:         s.Italic,
					ASSUnderline:      s.Underline,
					ASSStrikeout:      s.Strikeout,
					ASSPrimaryColour:  parseColorPtr(FormatASSColor(s.PrimaryColour)),
					ASSSecondaryColour: parseColorPtr(FormatASSColor(s.SecondaryColour)),
					ASSOutlineColour:  parseColorPtr(FormatASSColor(s.OutlineColour)),
					ASSBackColour:     parseColorPtr(FormatASSColor(s.BackColour)),
					ASSOutline:        s.Outline,
					ASSShadow:         s.Shadow,
					ASSScaleX:         s.ScaleX,
					ASSScaleY:         s.ScaleY,
					ASSSpacing:        s.Spacing,
					ASSAngle:          s.Angle,
					ASSAlignment:      s.Alignment,
					ASSMarginLeft:     s.MarginL,
					ASSMarginRight:    s.MarginR,
					ASSMarginVertical: s.MarginV,
					ASSEncoding:       1,
					ASSBorderStyle:    1,
				},
			},
		}
		assStyles = append(assStyles, as)
	}
	subs.Metadata.ASSStyles = assStyles
}

func parseColorPtr(s string) *astisub.Color {
	c, err := astisub.ASSColorFromString(s)
	if err != nil {
		return nil
	}
	return c
}

// DurationToASSTime converts a time.Duration to ASS time format H:MM:SS.CC.
func DurationToASSTime(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	cs := int(d.Milliseconds()/10) % 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/parser/ -v`
Expected: all tests pass.

**Note:** go-astisub's `ASSColorFromString` may use a different format or may not exist. If it doesn't, create a helper:
```go
func parseColorPtr(s string) *astisub.Color {
	// Build astisub.Color manually from our parsed color
	c, err := ParseASSColor(s)
	if err != nil {
		return nil
	}
	ac := &astisub.Color{
		Alpha: 255 - c.A,
		Blue:  c.B,
		Green: c.G,
		Red:   c.R,
	}
	return ac
}
```

Adjust based on go-astisub's actual `Color` struct fields.

- [ ] **Step 5: Commit**

```bash
git add internal/parser/writer.go internal/parser/writer_test.go
git commit -m "feat: add ASS writer with round-trip tests"
```

---

### Task 5: Folder Scanner

**Files:**
- Create: `internal/scan/scan.go`
- Create: `internal/scan/scan_test.go`

- [ ] **Step 1: Write scanner tests**

Create `internal/scan/scan_test.go`:
```go
package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFolderExternal(t *testing.T) {
	dir := t.TempDir()

	// Create subtitle files
	writeFile(t, dir, "episode01.ass", "[Script Info]\n")
	writeFile(t, dir, "episode01.mkv", "fake video")
	writeFile(t, dir, "episode02.ass", "[Script Info]\n")
	writeFile(t, dir, "episode02.mp4", "fake video")
	writeFile(t, dir, "episode03.ass", "[Script Info]\n")
	// No video for ep03

	result, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}

	if len(result.Files) < 3 {
		t.Fatalf("expected at least 3 files, got %d", len(result.Files))
	}

	// Find ep01
	var ep01 *ScannedFile
	for i, f := range result.Files {
		if filepath.Base(f.Path) == "episode01.ass" {
			ep01 = &result.Files[i]
			break
		}
	}
	if ep01 == nil {
		t.Fatal("episode01.ass not found")
	}
	if ep01.Type != "external" {
		t.Errorf("ep01.Type = %q, want external", ep01.Type)
	}
	if filepath.Base(ep01.VideoPath) != "episode01.mkv" {
		t.Errorf("ep01.VideoPath = %q, want episode01.mkv", ep01.VideoPath)
	}

	// Find ep03 - no video match
	var ep03 *ScannedFile
	for i, f := range result.Files {
		if filepath.Base(f.Path) == "episode03.ass" {
			ep03 = &result.Files[i]
			break
		}
	}
	if ep03 == nil {
		t.Fatal("episode03.ass not found")
	}
	if ep03.VideoPath != "" {
		t.Errorf("ep03 should have no video match, got %q", ep03.VideoPath)
	}
}

func TestScanFolderMatchesMultiDotNames(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "[Kawaiika-Raws] Vinland Saga 01 [BDRip].mkv", "fake")
	writeFile(t, dir, "[Kawaiika-Raws] Vinland Saga 01 [BDRip].rus.[Anku].ass", "fake")

	result, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}

	found := false
	for _, f := range result.Files {
		if filepath.Base(f.Path) == "[Kawaiika-Raws] Vinland Saga 01 [BDRip].rus.[Anku].ass" {
			found = true
			if filepath.Base(f.VideoPath) != "[Kawaiika-Raws] Vinland Saga 01 [BDRip].mkv" {
				t.Errorf("VideoPath = %q, want mkv", f.VideoPath)
			}
		}
	}
	if !found {
		t.Error("ass file not found in scan results")
	}
}

func TestScanFolderEmpty(t *testing.T) {
	dir := t.TempDir()
	result, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}
	if len(result.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(result.Files))
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/scan/ -v`
Expected: compilation error.

- [ ] **Step 3: Implement scanner**

Create `internal/scan/scan.go`:
```go
// Package scan handles discovering subtitle and video files in a folder.
package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScannedFile represents a file found during folder scanning.
type ScannedFile struct {
	Path      string      `json:"path"`
	VideoPath string      `json:"videoPath"`
	Type      string      `json:"type"` // "external" or "embedded"
	Tracks    []TrackInfo `json:"tracks"`
}

// TrackInfo describes a subtitle track inside a container.
type TrackInfo struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Title    string `json:"title"`
}

// FolderScanResult holds the list of files found when scanning a folder.
type FolderScanResult struct {
	Files []ScannedFile `json:"files"`
}

var subtitleExts = map[string]bool{
	".ass": true,
	".ssa": true,
}

var videoExts = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".webm": true,
}

// ScanFolder discovers subtitle files (ASS/SSA) and video files in the given directory.
// For external .ass/.ssa files, it attempts to match a video file by base name prefix.
// It does NOT recurse into subdirectories.
func ScanFolder(dir string) (*FolderScanResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", dir, err)
	}

	var subFiles []string
	videoFiles := make(map[string]string) // base name (no ext) → full path

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))

		if subtitleExts[ext] {
			subFiles = append(subFiles, filepath.Join(dir, name))
		}
		if videoExts[ext] {
			base := strings.TrimSuffix(name, filepath.Ext(name))
			videoFiles[base] = filepath.Join(dir, name)
		}
	}

	result := &FolderScanResult{
		Files: make([]ScannedFile, 0, len(subFiles)),
	}

	for _, subPath := range subFiles {
		sf := ScannedFile{
			Path: subPath,
			Type: "external",
		}
		sf.VideoPath = matchVideo(subPath, videoFiles)
		result.Files = append(result.Files, sf)
	}

	return result, nil
}

// matchVideo tries to find a video file whose base name is a prefix of the subtitle file name.
// For "episode01.rus.[Anku].ass", it tries progressively shorter prefixes:
// "episode01.rus.[Anku]" → "episode01.rus" → "episode01" until a video match is found.
func matchVideo(subPath string, videoFiles map[string]string) string {
	name := filepath.Base(subPath)

	// Strip the subtitle extension
	base := strings.TrimSuffix(name, filepath.Ext(name))

	// Try exact match first, then progressively shorter prefixes
	for {
		if vp, ok := videoFiles[base]; ok {
			return vp
		}
		// Find last dot to trim
		idx := strings.LastIndex(base, ".")
		if idx < 0 {
			break
		}
		base = base[:idx]
	}
	return ""
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/scan/ -v`
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/scan/
git commit -m "feat: add folder scanner with video-subtitle matching"
```

---

### Task 6: FFmpeg Discovery and Download

**Files:**
- Create: `internal/ffmpeg/ffmpeg.go`
- Create: `internal/ffmpeg/ffmpeg_test.go`

- [ ] **Step 1: Write tests for ffmpeg path resolution**

Create `internal/ffmpeg/ffmpeg_test.go`:
```go
package ffmpeg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindCachedBinary(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "ffmpeg.exe")
	os.WriteFile(fakeBin, []byte("fake"), 0755)

	m := &Manager{dataDir: dir}
	path := m.cachedBinaryPath()

	if path != fakeBin {
		t.Errorf("cachedBinaryPath() = %q, want %q", path, fakeBin)
	}
}

func TestCachedBinaryNotFound(t *testing.T) {
	dir := t.TempDir()
	m := &Manager{dataDir: dir}

	// cachedBinaryPath returns the expected path regardless
	path := m.cachedBinaryPath()
	if path == "" {
		t.Error("cachedBinaryPath() should return expected path even if not present")
	}

	// but binaryExists should return false
	if m.binaryExists(path) {
		t.Error("binaryExists should be false for missing file")
	}
}

func TestDownloadURL(t *testing.T) {
	url := downloadURL()
	if url == "" {
		t.Error("downloadURL() should not be empty")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/ffmpeg/ -v`
Expected: compilation error.

- [ ] **Step 3: Implement ffmpeg manager**

Create `internal/ffmpeg/ffmpeg.go`:
```go
// Package ffmpeg handles finding, downloading, and managing the ffmpeg binary.
package ffmpeg

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProgressFunc is called during download with bytes received and total bytes.
type ProgressFunc func(received, total int64)

// Manager handles ffmpeg binary discovery and download.
type Manager struct {
	dataDir string
	binPath string
}

// NewManager creates a new ffmpeg Manager. dataDir is the application data directory
// where ffmpeg will be stored if downloaded.
func NewManager(dataDir string) *Manager {
	return &Manager{dataDir: dataDir}
}

// BinPath returns the resolved path to the ffmpeg binary.
// Returns empty string if ffmpeg has not been found or downloaded yet.
func (m *Manager) BinPath() string {
	return m.binPath
}

// Find looks for ffmpeg in PATH first, then in the cached location.
// Returns the path to the ffmpeg binary if found, or empty string.
func (m *Manager) Find() string {
	// Check PATH
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		m.binPath = p
		return p
	}

	// Check cached
	cached := m.cachedBinaryPath()
	if m.binaryExists(cached) {
		m.binPath = cached
		return cached
	}

	return ""
}

// Download downloads a static ffmpeg build for Windows amd64.
// It calls progressFn periodically with download progress.
func (m *Manager) Download(ctx context.Context, progressFn ProgressFunc) error {
	url := downloadURL()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download ffmpeg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download ffmpeg: HTTP %d", resp.StatusCode)
	}

	// Download to temp file
	tmpFile, err := os.CreateTemp(m.dataDir, "ffmpeg-download-*.zip")
	if err != nil {
		if mkErr := os.MkdirAll(m.dataDir, 0755); mkErr != nil {
			return fmt.Errorf("create data dir %q: %w", m.dataDir, mkErr)
		}
		tmpFile, err = os.CreateTemp(m.dataDir, "ffmpeg-download-*.zip")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	total := resp.ContentLength
	var received int64
	buf := make([]byte, 32*1024)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tmpFile.Write(buf[:n]); writeErr != nil {
				tmpFile.Close()
				return fmt.Errorf("write download data: %w", writeErr)
			}
			received += int64(n)
			if progressFn != nil {
				progressFn(received, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			tmpFile.Close()
			return fmt.Errorf("read download data: %w", readErr)
		}
		if ctx.Err() != nil {
			tmpFile.Close()
			return ctx.Err()
		}
	}
	tmpFile.Close()

	// Extract ffmpeg.exe from zip
	if err := extractFFmpegFromZip(tmpPath, m.cachedBinaryPath()); err != nil {
		return fmt.Errorf("extract ffmpeg from zip: %w", err)
	}

	m.binPath = m.cachedBinaryPath()
	return nil
}

func (m *Manager) cachedBinaryPath() string {
	return filepath.Join(m.dataDir, "ffmpeg.exe")
}

func (m *Manager) binaryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func downloadURL() string {
	// BtbN GitHub releases — static Windows build
	return "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
}

func extractFFmpegFromZip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip %q: %w", zipPath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "bin/ffmpeg.exe") {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open %q in zip: %w", f.Name, err)
			}
			defer rc.Close()

			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("create dir for %q: %w", destPath, err)
			}

			out, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("create %q: %w", destPath, err)
			}
			defer out.Close()

			if _, err := io.Copy(out, rc); err != nil {
				return fmt.Errorf("extract ffmpeg.exe: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("ffmpeg.exe not found in zip archive")
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/ffmpeg/ -v`
Expected: all tests pass (they don't do actual downloads).

- [ ] **Step 5: Commit**

```bash
git add internal/ffmpeg/ffmpeg.go internal/ffmpeg/ffmpeg_test.go
git commit -m "feat: add ffmpeg discovery and download manager"
```

---

### Task 7: FFmpeg Frame and Track Extraction

**Files:**
- Create: `internal/ffmpeg/extract.go`
- Create: `internal/ffmpeg/extract_test.go`

- [ ] **Step 1: Write tests for command building**

Create `internal/ffmpeg/extract_test.go`:
```go
package ffmpeg

import (
	"testing"
	"time"
)

func TestBuildFrameCommand(t *testing.T) {
	args := buildFrameArgs("/path/to/video.mkv", "/tmp/sub.ass", 5*time.Second)

	expected := []string{
		"-ss", "5.000",
		"-i", "/path/to/video.mkv",
		"-vf", "subtitles=/tmp/sub.ass",
		"-frames:v", "1",
		"-f", "image2",
		"pipe:1",
	}

	if len(args) != len(expected) {
		t.Fatalf("args length = %d, want %d\nargs: %v", len(args), len(expected), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestBuildListTracksCommand(t *testing.T) {
	args := buildListTracksArgs("/path/to/video.mkv")

	if len(args) != 2 {
		t.Fatalf("args length = %d, want 2", len(args))
	}
	if args[0] != "-i" || args[1] != "/path/to/video.mkv" {
		t.Errorf("args = %v, want [-i /path/to/video.mkv]", args)
	}
}

func TestBuildExtractTrackCommand(t *testing.T) {
	args := buildExtractTrackArgs("/path/to/video.mkv", 2, "/tmp/output.ass")

	expected := []string{
		"-i", "/path/to/video.mkv",
		"-map", "0:s:2",
		"-c:s", "copy",
		"/tmp/output.ass",
	}

	if len(args) != len(expected) {
		t.Fatalf("args length = %d, want %d", len(args), len(expected))
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestParseTrackList(t *testing.T) {
	stderr := `Input #0, matroska,webm, from 'video.mkv':
  Duration: 00:23:40.10, start: 0.000000, bitrate: 8000 kb/s
    Stream #0:0: Video: hevc, yuv420p, 1920x1080, 23.98 fps
    Stream #0:1: Audio: flac, 48000 Hz, stereo
    Stream #0:2(rus): Subtitle: ass
      Metadata:
        title           : Russian [Anku]
    Stream #0:3(eng): Subtitle: srt
    Stream #0:4(jpn): Subtitle: ass
      Metadata:
        title           : Japanese`

	tracks := parseTrackList(stderr)

	// Should find 2 ASS tracks, skip SRT
	if len(tracks) != 2 {
		t.Fatalf("expected 2 ASS tracks, got %d: %+v", len(tracks), tracks)
	}

	if tracks[0].Index != 0 {
		t.Errorf("track[0].Index = %d, want 0", tracks[0].Index)
	}
	if tracks[0].Language != "rus" {
		t.Errorf("track[0].Language = %q, want rus", tracks[0].Language)
	}
	if tracks[0].Title != "Russian [Anku]" {
		t.Errorf("track[0].Title = %q, want 'Russian [Anku]'", tracks[0].Title)
	}

	if tracks[1].Index != 1 {
		t.Errorf("track[1].Index = %d, want 1", tracks[1].Index)
	}
	if tracks[1].Language != "jpn" {
		t.Errorf("track[1].Language = %q, want jpn", tracks[1].Language)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/ffmpeg/ -run TestBuild -v`
Expected: compilation error.

- [ ] **Step 3: Implement extraction functions**

Create `internal/ffmpeg/extract.go`:
```go
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Extractor handles ffmpeg operations for frame and track extraction.
type Extractor struct {
	binPath string
}

// NewExtractor creates an Extractor with the given ffmpeg binary path.
func NewExtractor(binPath string) *Extractor {
	return &Extractor{binPath: binPath}
}

// ExtractFrame extracts a single video frame at the given time with subtitles overlaid.
// Returns the frame as a base64-encoded PNG string.
func (e *Extractor) ExtractFrame(ctx context.Context, videoPath, subPath string, at time.Duration) (string, error) {
	args := buildFrameArgs(videoPath, subPath, at)
	cmd := exec.CommandContext(ctx, e.binPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("extract frame at %v: %w\nstderr: %s", at, err, stderr.String())
	}

	encoded := base64.StdEncoding.EncodeToString(stdout.Bytes())
	return encoded, nil
}

// ListTracks lists ASS/SSA subtitle tracks in a video container.
func (e *Extractor) ListTracks(ctx context.Context, videoPath string) ([]TrackInfo, error) {
	args := buildListTracksArgs(videoPath)
	cmd := exec.CommandContext(ctx, e.binPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// ffmpeg -i always returns exit code 1 when no output specified, that's expected
	cmd.Run()

	return parseTrackList(stderr.String()), nil
}

// ExtractTrack extracts a subtitle track from a video container to an ASS file.
func (e *Extractor) ExtractTrack(ctx context.Context, videoPath string, trackIndex int, outputPath string) error {
	args := buildExtractTrackArgs(videoPath, trackIndex, outputPath)
	cmd := exec.CommandContext(ctx, e.binPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extract track %d from %q: %w\nstderr: %s", trackIndex, videoPath, err, stderr.String())
	}
	return nil
}

func buildFrameArgs(videoPath, subPath string, at time.Duration) []string {
	seconds := at.Seconds()
	return []string{
		"-ss", fmt.Sprintf("%.3f", seconds),
		"-i", videoPath,
		"-vf", fmt.Sprintf("subtitles=%s", subPath),
		"-frames:v", "1",
		"-f", "image2",
		"pipe:1",
	}
}

func buildListTracksArgs(videoPath string) []string {
	return []string{"-i", videoPath}
}

func buildExtractTrackArgs(videoPath string, trackIndex int, outputPath string) []string {
	return []string{
		"-i", videoPath,
		"-map", fmt.Sprintf("0:s:%d", trackIndex),
		"-c:s", "copy",
		outputPath,
	}
}

var (
	// Matches: Stream #0:2(rus): Subtitle: ass
	streamRe = regexp.MustCompile(`Stream #0:(\d+)(?:\((\w+)\))?: Subtitle: (ass|ssa)`)
	// Matches:   title   : Some Title
	titleRe = regexp.MustCompile(`^\s+title\s+:\s*(.+)$`)
)

func parseTrackList(stderr string) []TrackInfo {
	lines := strings.Split(stderr, "\n")
	var tracks []TrackInfo
	subIndex := 0

	for i, line := range lines {
		m := streamRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		lang := m[2] // may be empty

		// Look for title in next few lines
		title := ""
		for j := i + 1; j < len(lines) && j <= i+3; j++ {
			tm := titleRe.FindStringSubmatch(lines[j])
			if tm != nil {
				title = strings.TrimSpace(tm[1])
				break
			}
			// Stop if we hit another Stream line
			if strings.Contains(lines[j], "Stream #") {
				break
			}
		}

		tracks = append(tracks, TrackInfo{
			Index:    subIndex,
			Language: lang,
			Title:    title,
		})
		subIndex++
	}
	return tracks
}

// VideoDuration extracts the total duration of a video file.
func (e *Extractor) VideoDuration(ctx context.Context, videoPath string) (time.Duration, error) {
	args := buildListTracksArgs(videoPath)
	cmd := exec.CommandContext(ctx, e.binPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Run()

	return parseDuration(stderr.String())
}

var durationRe = regexp.MustCompile(`Duration:\s+(\d+):(\d+):(\d+)\.(\d+)`)

func parseDuration(stderr string) (time.Duration, error) {
	m := durationRe.FindStringSubmatch(stderr)
	if m == nil {
		return 0, fmt.Errorf("duration not found in ffmpeg output")
	}

	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	s, _ := strconv.Atoi(m[3])
	cs, _ := strconv.Atoi(m[4])

	d := time.Duration(h)*time.Hour +
		time.Duration(min)*time.Minute +
		time.Duration(s)*time.Second +
		time.Duration(cs)*10*time.Millisecond

	return d, nil
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/ffmpeg/ -v`
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/ffmpeg/extract.go internal/ffmpeg/extract_test.go
git commit -m "feat: add ffmpeg frame extraction, track listing, and parsing"
```

---

### Task 8: Style Editor Backend

**Files:**
- Create: `internal/editor/editor.go`
- Create: `internal/editor/editor_test.go`

- [ ] **Step 1: Write editor tests**

Create `internal/editor/editor_test.go`:
```go
package editor

import (
	"subtitles-editor/internal/parser"
	"testing"
)

func TestApplyChange(t *testing.T) {
	style := parser.SubtitleStyle{
		Name:     "Default",
		FontName: "Arial",
		FontSize: 48,
		Bold:     true,
	}

	changed, err := ApplyChange(style, "fontSize", 72.0)
	if err != nil {
		t.Fatalf("ApplyChange: %v", err)
	}
	if changed.FontSize != 72 {
		t.Errorf("FontSize = %v, want 72", changed.FontSize)
	}
	// Other fields unchanged
	if changed.FontName != "Arial" {
		t.Errorf("FontName changed to %q", changed.FontName)
	}
	if !changed.Bold {
		t.Error("Bold should still be true")
	}
}

func TestApplyChangeBool(t *testing.T) {
	style := parser.SubtitleStyle{Name: "Default", Bold: true}

	changed, err := ApplyChange(style, "bold", false)
	if err != nil {
		t.Fatalf("ApplyChange: %v", err)
	}
	if changed.Bold {
		t.Error("Bold should be false")
	}
}

func TestApplyChangeColor(t *testing.T) {
	style := parser.SubtitleStyle{Name: "Default"}

	color := parser.Color{R: 255, G: 0, B: 0, A: 255}
	changed, err := ApplyChange(style, "primaryColour", color)
	if err != nil {
		t.Fatalf("ApplyChange: %v", err)
	}
	if changed.PrimaryColour != color {
		t.Errorf("PrimaryColour = %+v, want %+v", changed.PrimaryColour, color)
	}
}

func TestApplyChangeString(t *testing.T) {
	style := parser.SubtitleStyle{Name: "Default", FontName: "Arial"}

	changed, err := ApplyChange(style, "fontName", "Verdana")
	if err != nil {
		t.Fatalf("ApplyChange: %v", err)
	}
	if changed.FontName != "Verdana" {
		t.Errorf("FontName = %q, want Verdana", changed.FontName)
	}
}

func TestApplyChangeInt(t *testing.T) {
	style := parser.SubtitleStyle{Name: "Default", Alignment: 2}

	changed, err := ApplyChange(style, "alignment", 8)
	if err != nil {
		t.Fatalf("ApplyChange: %v", err)
	}
	if changed.Alignment != 8 {
		t.Errorf("Alignment = %d, want 8", changed.Alignment)
	}
}

func TestApplyChangeUnknownField(t *testing.T) {
	style := parser.SubtitleStyle{Name: "Default"}

	_, err := ApplyChange(style, "nonexistent", 42)
	if err == nil {
		t.Error("expected error for unknown field")
	}
}

func TestApplyBatch(t *testing.T) {
	styles := []parser.SubtitleStyle{
		{Name: "Default", FontSize: 48},
		{Name: "Signs", FontSize: 36},
	}

	changes := []StyleChange{
		{StyleName: "Default", Field: "fontSize", Value: 72.0},
		{StyleName: "Signs", Field: "fontSize", Value: 72.0},
	}

	result, err := ApplyBatch(styles, changes)
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	for _, s := range result {
		if s.FontSize != 72 {
			t.Errorf("style %q FontSize = %v, want 72", s.Name, s.FontSize)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/editor/ -v`
Expected: compilation error.

- [ ] **Step 3: Implement editor**

Create `internal/editor/editor.go`:
```go
// Package editor handles applying style changes to subtitle data.
package editor

import (
	"encoding/json"
	"fmt"

	"subtitles-editor/internal/parser"
)

// StyleChange represents a single field change to a named style.
type StyleChange struct {
	StyleName string      `json:"styleName"`
	Field     string      `json:"field"`
	Value     interface{} `json:"value"`
}

// ApplyChange applies a single field change to a SubtitleStyle and returns the modified copy.
func ApplyChange(style parser.SubtitleStyle, field string, value interface{}) (parser.SubtitleStyle, error) {
	switch field {
	case "fontName":
		v, err := toString(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.FontName = v
	case "fontSize":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.FontSize = v
	case "bold":
		v, err := toBool(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.Bold = v
	case "italic":
		v, err := toBool(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.Italic = v
	case "underline":
		v, err := toBool(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.Underline = v
	case "strikeout":
		v, err := toBool(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.Strikeout = v
	case "primaryColour":
		v, err := toColor(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.PrimaryColour = v
	case "secondaryColour":
		v, err := toColor(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.SecondaryColour = v
	case "outlineColour":
		v, err := toColor(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.OutlineColour = v
	case "backColour":
		v, err := toColor(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.BackColour = v
	case "outline":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.Outline = v
	case "shadow":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.Shadow = v
	case "scaleX":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.ScaleX = v
	case "scaleY":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.ScaleY = v
	case "spacing":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.Spacing = v
	case "angle":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.Angle = v
	case "alignment":
		v, err := toInt(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.Alignment = v
	case "marginL":
		v, err := toInt(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.MarginL = v
	case "marginR":
		v, err := toInt(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.MarginR = v
	case "marginV":
		v, err := toInt(value)
		if err != nil {
			return style, fmt.Errorf("field %q: %w", field, err)
		}
		style.MarginV = v
	default:
		return style, fmt.Errorf("unknown style field %q", field)
	}
	return style, nil
}

// ApplyBatch applies multiple changes to a slice of styles.
// Returns a new slice with changes applied.
func ApplyBatch(styles []parser.SubtitleStyle, changes []StyleChange) ([]parser.SubtitleStyle, error) {
	result := make([]parser.SubtitleStyle, len(styles))
	copy(result, styles)

	for _, ch := range changes {
		for i, s := range result {
			if s.Name == ch.StyleName {
				modified, err := ApplyChange(s, ch.Field, ch.Value)
				if err != nil {
					return nil, fmt.Errorf("apply change to %q.%s: %w", ch.StyleName, ch.Field, err)
				}
				result[i] = modified
			}
		}
	}
	return result, nil
}

func toString(v interface{}) (string, error) {
	if s, ok := v.(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("expected string, got %T", v)
}

func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case json.Number:
		return val.Float64()
	}
	return 0, fmt.Errorf("expected number, got %T", v)
}

func toInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case float64:
		return int(val), nil
	case json.Number:
		i, err := val.Int64()
		return int(i), err
	}
	return 0, fmt.Errorf("expected integer, got %T", v)
}

func toBool(v interface{}) (bool, error) {
	if b, ok := v.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("expected bool, got %T", v)
}

func toColor(v interface{}) (parser.Color, error) {
	if c, ok := v.(parser.Color); ok {
		return c, nil
	}
	// Handle map from JSON
	if m, ok := v.(map[string]interface{}); ok {
		r, _ := toInt(m["r"])
		g, _ := toInt(m["g"])
		b, _ := toInt(m["b"])
		a, _ := toInt(m["a"])
		return parser.Color{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, nil
	}
	return parser.Color{}, fmt.Errorf("expected Color, got %T", v)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/editor/ -v`
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/editor/
git commit -m "feat: add style editor with field-level mutations and batch apply"
```

---

### Task 9: Project Serialization (Autosave)

**Files:**
- Create: `internal/project/project.go`
- Create: `internal/project/project_test.go`

- [ ] **Step 1: Write serialization tests**

Create `internal/project/project_test.go`:
```go
package project

import (
	"path/filepath"
	"subtitles-editor/internal/parser"
	"testing"
	"time"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	state := &ProjectState{
		FolderPath: "/some/folder",
		SavedAt:    time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		Dirty:      true,
		Files: []FileState{
			{
				ID:     "file1",
				Path:   "/some/folder/ep01.ass",
				Source: "external",
				OriginalStyles: []parser.SubtitleStyle{
					{Name: "Default", FontName: "Arial", FontSize: 48},
				},
				ModifiedStyles: []parser.SubtitleStyle{
					{Name: "Default", FontName: "Verdana", FontSize: 72},
				},
			},
		},
		ActiveFileID:   "file1",
		SelectedStyles: []string{"Default"},
	}

	// Save
	if err := m.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists
	if !m.HasAutosave() {
		t.Fatal("HasAutosave should return true after save")
	}

	// Load
	loaded, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.FolderPath != state.FolderPath {
		t.Errorf("FolderPath = %q, want %q", loaded.FolderPath, state.FolderPath)
	}
	if !loaded.Dirty {
		t.Error("Dirty should be true")
	}
	if len(loaded.Files) != 1 {
		t.Fatalf("Files count = %d, want 1", len(loaded.Files))
	}
	if loaded.Files[0].ModifiedStyles[0].FontName != "Verdana" {
		t.Error("Modified style not preserved")
	}
	if loaded.Files[0].ModifiedStyles[0].FontSize != 72 {
		t.Error("Modified font size not preserved")
	}
}

func TestDeleteAutosave(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	state := &ProjectState{FolderPath: "/test", Dirty: true}
	m.Save(state)

	if !m.HasAutosave() {
		t.Fatal("should have autosave")
	}

	if err := m.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if m.HasAutosave() {
		t.Error("should not have autosave after delete")
	}
}

func TestLoadNoAutosave(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	if m.HasAutosave() {
		t.Error("should not have autosave initially")
	}

	_, err := m.Load()
	if err == nil {
		t.Error("Load should return error when no autosave")
	}
}

func TestAutosavePath(t *testing.T) {
	m := NewManager("/some/dir")
	path := m.autosavePath()
	if filepath.Base(path) != "autosave.gob" {
		t.Errorf("autosave filename = %q, want autosave.gob", filepath.Base(path))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/project/ -v`
Expected: compilation error.

- [ ] **Step 3: Implement project manager**

Create `internal/project/project.go`:
```go
// Package project handles autosave and session restoration via gob serialization.
package project

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"subtitles-editor/internal/parser"
)

// UndoChange represents a single field change within an undo entry.
type UndoChange struct {
	FileID    string      `json:"fileId"`
	StyleName string      `json:"styleName"`
	Field     string      `json:"field"`
	OldValue  interface{} `json:"oldValue"`
	NewValue  interface{} `json:"newValue"`
}

// UndoEntry represents a single undoable action.
type UndoEntry struct {
	ID          int          `json:"id"`
	Description string       `json:"description"`
	Changes     []UndoChange `json:"changes"`
}

// FileState holds the state of a single subtitle file.
type FileState struct {
	ID             string                  `json:"id"`
	Path           string                  `json:"path"`
	Source         string                  `json:"source"`
	TrackID        int                     `json:"trackId"`
	VideoPath      string                  `json:"videoPath"`
	OriginalStyles []parser.SubtitleStyle  `json:"originalStyles"`
	ModifiedStyles []parser.SubtitleStyle  `json:"modifiedStyles"`
	Events         []parser.SubtitleEvent  `json:"events"`
}

// ProjectState holds the full serializable state of a work session.
type ProjectState struct {
	FolderPath     string      `json:"folderPath"`
	SavedAt        time.Time   `json:"savedAt"`
	Dirty          bool        `json:"dirty"`
	Files          []FileState `json:"files"`
	UndoStack      []UndoEntry `json:"undoStack"`
	RedoStack      []UndoEntry `json:"redoStack"`
	ActiveFileID   string      `json:"activeFileId"`
	SelectedStyles []string    `json:"selectedStyles"`
}

// Manager handles project serialization to/from gob files.
type Manager struct {
	dataDir string
}

// NewManager creates a project Manager. dataDir is where autosave.gob is stored.
func NewManager(dataDir string) *Manager {
	return &Manager{dataDir: dataDir}
}

// Save serializes the project state to autosave.gob.
func (m *Manager) Save(state *ProjectState) error {
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	path := m.autosavePath()
	tmpPath := path + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create autosave temp file: %w", err)
	}

	enc := gob.NewEncoder(f)
	if err := enc.Encode(state); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode project state: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close autosave file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename autosave file: %w", err)
	}

	return nil
}

// Load reads the autosave.gob file and returns the project state.
func (m *Manager) Load() (*ProjectState, error) {
	path := m.autosavePath()

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open autosave file: %w", err)
	}
	defer f.Close()

	var state ProjectState
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&state); err != nil {
		return nil, fmt.Errorf("decode project state: %w", err)
	}

	return &state, nil
}

// HasAutosave returns true if an autosave file exists.
func (m *Manager) HasAutosave() bool {
	info, err := os.Stat(m.autosavePath())
	return err == nil && !info.IsDir()
}

// Delete removes the autosave file.
func (m *Manager) Delete() error {
	path := m.autosavePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete autosave: %w", err)
	}
	return nil
}

func (m *Manager) autosavePath() string {
	return filepath.Join(m.dataDir, "autosave.gob")
}

func init() {
	// Register types that may be stored as interface{} in UndoChange
	gob.Register(parser.Color{})
	gob.Register(float64(0))
	gob.Register(int(0))
	gob.Register("")
	gob.Register(true)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/hakastein/work/subtitles && go test ./internal/project/ -v`
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/project/
git commit -m "feat: add project autosave/restore with gob serialization"
```

---

### Task 10: i18n Backend + Preview Orchestration

**Files:**
- Create: `internal/i18n/i18n.go`
- Create: `internal/preview/preview.go`

- [ ] **Step 1: Implement locale detection**

Create `internal/i18n/i18n.go`:
```go
// Package i18n detects the system locale for the frontend.
package i18n

import (
	"os"
	"strings"
)

// DetectLocale returns the best matching locale ("en" or "ru").
// On Windows it checks environment variables.
// Falls back to "en" if detection fails.
func DetectLocale() string {
	// Check common env vars
	for _, key := range []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		val := os.Getenv(key)
		if val == "" {
			continue
		}
		lower := strings.ToLower(val)
		if strings.HasPrefix(lower, "ru") {
			return "ru"
		}
		if strings.HasPrefix(lower, "en") {
			return "en"
		}
	}

	return "en"
}
```

- [ ] **Step 2: Implement preview orchestrator**

Create `internal/preview/preview.go`:
```go
// Package preview orchestrates frame generation with subtitle overlay.
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

// FrameResult holds the result of a preview frame generation.
type FrameResult struct {
	Base64PNG string `json:"base64Png"`
	Timecode  string `json:"timecode"`
}

// Generator handles preview frame generation with cancellation support.
type Generator struct {
	extractor *ffmpeg.Extractor

	mu        sync.Mutex
	cancelFn  context.CancelFunc
}

// NewGenerator creates a preview Generator.
func NewGenerator(extractor *ffmpeg.Extractor) *Generator {
	return &Generator{extractor: extractor}
}

// GenerateFrame extracts a video frame with subtitles overlaid at the given time.
// It cancels any previously running frame generation.
// subFile should contain the current (possibly modified) subtitle data.
func (g *Generator) GenerateFrame(ctx context.Context, videoPath string, subFile *parser.SubtitleFile, at time.Duration) (*FrameResult, error) {
	// Cancel previous generation
	g.mu.Lock()
	if g.cancelFn != nil {
		g.cancelFn()
	}
	genCtx, cancel := context.WithCancel(ctx)
	g.cancelFn = cancel
	g.mu.Unlock()

	defer cancel()

	// Write temp ASS file with current styles
	tmpPath, err := parser.WriteTempFile(subFile)
	if err != nil {
		return nil, fmt.Errorf("write temp subtitle file: %w", err)
	}
	defer os.Remove(tmpPath)

	base64PNG, err := g.extractor.ExtractFrame(genCtx, videoPath, tmpPath, at)
	if err != nil {
		return nil, fmt.Errorf("extract preview frame: %w", err)
	}

	h := int(at.Hours())
	m := int(at.Minutes()) % 60
	s := int(at.Seconds()) % 60

	return &FrameResult{
		Base64PNG: base64PNG,
		Timecode:  fmt.Sprintf("%d:%02d:%02d", h, m, s),
	}, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/i18n/ internal/preview/
git commit -m "feat: add locale detection and preview frame generator"
```

---

### Task 11: Wails App Binding — Backend API

**Files:**
- Modify: `cmd/app/app.go`
- Modify: `cmd/app/main.go`

- [ ] **Step 1: Implement full App struct with all backend bindings**

Replace `cmd/app/app.go`:
```go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"subtitles-editor/internal/editor"
	"subtitles-editor/internal/ffmpeg"
	i18nPkg "subtitles-editor/internal/i18n"
	"subtitles-editor/internal/parser"
	"subtitles-editor/internal/preview"
	"subtitles-editor/internal/project"
	"subtitles-editor/internal/scan"
)

// App holds the application state and exposes methods to the frontend via Wails bindings.
type App struct {
	ctx       context.Context
	ffmpegMgr *ffmpeg.Manager
	extractor *ffmpeg.Extractor
	previewGen *preview.Generator
	projectMgr *project.Manager
	dataDir   string

	// Parsed files cache
	parsedFiles map[string]*parser.SubtitleFile
}

func newApp() *App {
	dataDir, err := os.UserConfigDir()
	if err != nil {
		dataDir = "."
	}
	dataDir = filepath.Join(dataDir, "subtitles-editor")

	return &App{
		dataDir:     dataDir,
		ffmpegMgr:   ffmpeg.NewManager(dataDir),
		projectMgr:  project.NewManager(dataDir),
		parsedFiles: make(map[string]*parser.SubtitleFile),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Try to find ffmpeg
	go a.initFFmpeg()
}

func (a *App) initFFmpeg() {
	path := a.ffmpegMgr.Find()
	if path != "" {
		a.extractor = ffmpeg.NewExtractor(path)
		a.previewGen = preview.NewGenerator(a.extractor)
		runtime.EventsEmit(a.ctx, "ffmpeg:ready", path)
		return
	}

	runtime.EventsEmit(a.ctx, "ffmpeg:downloading", nil)
	err := a.ffmpegMgr.Download(a.ctx, func(received, total int64) {
		runtime.EventsEmit(a.ctx, "ffmpeg:progress", map[string]int64{
			"received": received,
			"total":    total,
		})
	})
	if err != nil {
		runtime.EventsEmit(a.ctx, "ffmpeg:error", err.Error())
		return
	}

	a.extractor = ffmpeg.NewExtractor(a.ffmpegMgr.BinPath())
	a.previewGen = preview.NewGenerator(a.extractor)
	runtime.EventsEmit(a.ctx, "ffmpeg:ready", a.ffmpegMgr.BinPath())
}

// GetLocale returns the detected system locale ("en" or "ru").
func (a *App) GetLocale() string {
	return i18nPkg.DetectLocale()
}

// OpenFolder opens a native folder dialog and returns the selected path.
func (a *App) OpenFolder() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select subtitle folder",
	})
	if err != nil {
		return "", fmt.Errorf("open folder dialog: %w", err)
	}
	return dir, nil
}

// ScanFolder scans a directory for subtitle and video files.
func (a *App) ScanFolder(dir string) (*scan.FolderScanResult, error) {
	result, err := scan.ScanFolder(dir)
	if err != nil {
		return nil, fmt.Errorf("scan folder %q: %w", dir, err)
	}

	// For video files, list embedded subtitle tracks
	if a.extractor != nil {
		for i, f := range result.Files {
			if f.Type == "external" && f.VideoPath != "" {
				// Nothing to do for external files
				continue
			}
		_ = i // placeholder for embedded track listing below
		}

		// Also scan for video-only files (might have embedded subs)
		a.scanEmbeddedTracks(dir, result)
	}

	return result, nil
}

func (a *App) scanEmbeddedTracks(dir string, result *scan.FolderScanResult) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	videoExts := map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if !videoExts[ext] {
			continue
		}

		videoPath := filepath.Join(dir, e.Name())
		tracks, err := a.extractor.ListTracks(a.ctx, videoPath)
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
}

// ParseFile parses a subtitle file and returns its styles and events.
func (a *App) ParseFile(path string) (*parser.SubtitleFile, error) {
	sf, err := parser.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("parse file %q: %w", path, err)
	}
	a.parsedFiles[sf.ID] = sf
	return sf, nil
}

// ExtractTrack extracts a subtitle track from a video container.
func (a *App) ExtractTrack(videoPath string, trackIndex int) (*parser.SubtitleFile, error) {
	if a.extractor == nil {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	tmpDir := os.TempDir()
	outPath := filepath.Join(tmpDir, fmt.Sprintf("track_%d_%d.ass", trackIndex, time.Now().UnixNano()))

	if err := a.extractor.ExtractTrack(a.ctx, videoPath, trackIndex, outPath); err != nil {
		return nil, fmt.Errorf("extract track: %w", err)
	}

	sf, err := parser.ParseFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("parse extracted track: %w", err)
	}

	sf.ID = fmt.Sprintf("%s:track:%d", videoPath, trackIndex)
	sf.Source = "embedded"
	sf.TrackID = trackIndex
	a.parsedFiles[sf.ID] = sf
	return sf, nil
}

// GeneratePreviewFrame generates a preview frame with subtitles at the given time.
func (a *App) GeneratePreviewFrame(fileID string, videoPath string, styles []parser.SubtitleStyle, atMs int64) (*preview.FrameResult, error) {
	if a.previewGen == nil {
		return nil, fmt.Errorf("ffmpeg not available for preview")
	}

	sf, ok := a.parsedFiles[fileID]
	if !ok {
		return nil, fmt.Errorf("file %q not loaded", fileID)
	}

	// Create a copy with modified styles
	modified := &parser.SubtitleFile{
		ID:      sf.ID,
		Path:    sf.Path,
		Source:  sf.Source,
		TrackID: sf.TrackID,
		Styles:  styles,
		Events:  sf.Events,
	}

	at := time.Duration(atMs) * time.Millisecond
	return a.previewGen.GenerateFrame(a.ctx, videoPath, modified, at)
}

// SaveFile writes modified styles back to a subtitle file.
func (a *App) SaveFile(fileID string, styles []parser.SubtitleStyle) error {
	sf, ok := a.parsedFiles[fileID]
	if !ok {
		return fmt.Errorf("file %q not loaded", fileID)
	}

	modified := &parser.SubtitleFile{
		ID:      sf.ID,
		Path:    sf.Path,
		Source:  sf.Source,
		TrackID: sf.TrackID,
		Styles:  styles,
		Events:  sf.Events,
	}

	outPath := sf.Path
	if sf.Source == "embedded" {
		// Save next to video with [modified] suffix
		dir := filepath.Dir(sf.Path)
		base := filepath.Base(sf.Path)
		ext := filepath.Ext(base)
		name := base[:len(base)-len(ext)]
		outPath = filepath.Join(dir, name+".[modified]"+ext)
	}

	if err := parser.WriteFile(outPath, modified); err != nil {
		return fmt.Errorf("save file %q: %w", outPath, err)
	}
	return nil
}

// SaveAll writes modified styles for all specified files.
func (a *App) SaveAll(fileStyles map[string][]parser.SubtitleStyle) error {
	for fileID, styles := range fileStyles {
		if err := a.SaveFile(fileID, styles); err != nil {
			return fmt.Errorf("save all — file %q: %w", fileID, err)
		}
	}
	return nil
}

// CheckAutosave checks if an autosave file exists and returns its metadata.
func (a *App) CheckAutosave() (*project.ProjectState, error) {
	if !a.projectMgr.HasAutosave() {
		return nil, nil
	}
	state, err := a.projectMgr.Load()
	if err != nil {
		return nil, fmt.Errorf("load autosave: %w", err)
	}
	if !state.Dirty {
		return nil, nil
	}
	return state, nil
}

// RestoreProject loads the autosaved project state.
func (a *App) RestoreProject() (*project.ProjectState, error) {
	return a.projectMgr.Load()
}

// Autosave saves the current project state.
func (a *App) Autosave(state *project.ProjectState) error {
	state.SavedAt = time.Now()
	return a.projectMgr.Save(state)
}

// DeleteAutosave removes the autosave file.
func (a *App) DeleteAutosave() error {
	return a.projectMgr.Delete()
}

// GetVideoDuration returns the duration of a video file in milliseconds.
func (a *App) GetVideoDuration(videoPath string) (int64, error) {
	if a.extractor == nil {
		return 0, fmt.Errorf("ffmpeg not available")
	}
	d, err := a.extractor.VideoDuration(a.ctx, videoPath)
	if err != nil {
		return 0, err
	}
	return d.Milliseconds(), nil
}
```

- [ ] **Step 2: Update main.go to use newApp()**

Replace `cmd/app/main.go`:
```go
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := newApp()

	err := wails.Run(&options.App{
		Title:  "Subtitle Style Editor",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 3: Verify Go compilation**

Run: `cd /home/hakastein/work/subtitles && go build ./cmd/app/`
Expected: compiles without errors. Fix any import issues.

- [ ] **Step 4: Commit**

```bash
git add cmd/app/
git commit -m "feat: wire all backend packages into Wails app bindings"
```

---

### Task 12: Frontend TypeScript Types and Services

**Files:**
- Create: `frontend/src/services/types.ts`
- Create: `frontend/src/services/scan.ts`
- Create: `frontend/src/services/parser.ts`
- Create: `frontend/src/services/editor.ts`
- Create: `frontend/src/services/ffmpeg.ts`
- Create: `frontend/src/services/project.ts`

- [ ] **Step 1: Create shared types**

Create `frontend/src/services/types.ts`:
```typescript
export interface Color {
  r: number
  g: number
  b: number
  a: number // 0-255, 255 = opaque
}

export interface SubtitleStyle {
  name: string
  fontName: string
  fontSize: number
  bold: boolean
  italic: boolean
  underline: boolean
  strikeout: boolean
  primaryColour: Color
  secondaryColour: Color
  outlineColour: Color
  backColour: Color
  outline: number
  shadow: number
  scaleX: number
  scaleY: number
  spacing: number
  angle: number
  alignment: number // 1-9
  marginL: number
  marginR: number
  marginV: number
}

export interface SubtitleEvent {
  styleName: string
  startTime: number // nanoseconds (Go time.Duration)
  endTime: number
  text: string
}

export interface SubtitleFile {
  id: string
  path: string
  source: 'external' | 'embedded'
  trackId: number
  styles: SubtitleStyle[]
  events: SubtitleEvent[]
}

export interface TrackInfo {
  index: number
  language: string
  title: string
}

export interface ScannedFile {
  path: string
  videoPath: string
  type: 'external' | 'embedded'
  tracks: TrackInfo[]
}

export interface FolderScanResult {
  files: ScannedFile[]
}

export interface FrameResult {
  base64Png: string
  timecode: string
}

export interface UndoChange {
  fileId: string
  styleName: string
  field: string
  oldValue: unknown
  newValue: unknown
}

export interface UndoEntry {
  id: number
  description: string
  changes: UndoChange[]
}

export interface FileState {
  id: string
  path: string
  source: string
  trackId: number
  videoPath: string
  originalStyles: SubtitleStyle[]
  modifiedStyles: SubtitleStyle[]
  events: SubtitleEvent[]
}

export interface ProjectState {
  folderPath: string
  savedAt: string
  dirty: boolean
  files: FileState[]
  undoStack: UndoEntry[]
  redoStack: UndoEntry[]
  activeFileId: string
  selectedStyles: string[]
}

/** Convert Go time.Duration (nanoseconds) to milliseconds */
export function durationToMs(ns: number): number {
  return Math.round(ns / 1_000_000)
}

/** Convert milliseconds to display string HH:MM:SS */
export function msToTimecode(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000)
  const h = Math.floor(totalSeconds / 3600)
  const m = Math.floor((totalSeconds % 3600) / 60)
  const s = totalSeconds % 60
  return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}
```

- [ ] **Step 2: Create service wrappers**

Create `frontend/src/services/scan.ts`:
```typescript
import type { FolderScanResult } from './types'

export async function openFolder(): Promise<string> {
  return window.go.main.App.OpenFolder()
}

export async function scanFolder(dir: string): Promise<FolderScanResult> {
  return window.go.main.App.ScanFolder(dir)
}
```

Create `frontend/src/services/parser.ts`:
```typescript
import type { SubtitleFile, SubtitleStyle } from './types'

export async function parseFile(path: string): Promise<SubtitleFile> {
  return window.go.main.App.ParseFile(path)
}

export async function extractTrack(videoPath: string, trackIndex: number): Promise<SubtitleFile> {
  return window.go.main.App.ExtractTrack(videoPath, trackIndex)
}

export async function saveFile(fileId: string, styles: SubtitleStyle[]): Promise<void> {
  return window.go.main.App.SaveFile(fileId, styles)
}

export async function saveAll(fileStyles: Record<string, SubtitleStyle[]>): Promise<void> {
  return window.go.main.App.SaveAll(fileStyles)
}
```

Create `frontend/src/services/editor.ts`:
```typescript
import type { FrameResult, SubtitleStyle } from './types'

export async function generatePreviewFrame(
  fileId: string,
  videoPath: string,
  styles: SubtitleStyle[],
  atMs: number,
): Promise<FrameResult> {
  return window.go.main.App.GeneratePreviewFrame(fileId, videoPath, styles, atMs)
}

export async function getVideoDuration(videoPath: string): Promise<number> {
  return window.go.main.App.GetVideoDuration(videoPath)
}
```

Create `frontend/src/services/ffmpeg.ts`:
```typescript
import { EventsOn } from '../../wailsjs/runtime/runtime'

export interface FFmpegProgress {
  received: number
  total: number
}

export function onFFmpegReady(callback: (path: string) => void): void {
  EventsOn('ffmpeg:ready', callback)
}

export function onFFmpegDownloading(callback: () => void): void {
  EventsOn('ffmpeg:downloading', callback)
}

export function onFFmpegProgress(callback: (progress: FFmpegProgress) => void): void {
  EventsOn('ffmpeg:progress', callback)
}

export function onFFmpegError(callback: (error: string) => void): void {
  EventsOn('ffmpeg:error', callback)
}
```

Create `frontend/src/services/project.ts`:
```typescript
import type { ProjectState } from './types'

export async function getLocale(): Promise<string> {
  return window.go.main.App.GetLocale()
}

export async function checkAutosave(): Promise<ProjectState | null> {
  return window.go.main.App.CheckAutosave()
}

export async function restoreProject(): Promise<ProjectState> {
  return window.go.main.App.RestoreProject()
}

export async function autosave(state: ProjectState): Promise<void> {
  return window.go.main.App.Autosave(state)
}

export async function deleteAutosave(): Promise<void> {
  return window.go.main.App.DeleteAutosave()
}
```

- [ ] **Step 3: Create Wails type declarations**

Create `frontend/src/wails.d.ts`:
```typescript
import type {
  FolderScanResult,
  SubtitleFile,
  SubtitleStyle,
  FrameResult,
  ProjectState,
} from './services/types'

declare global {
  interface Window {
    go: {
      main: {
        App: {
          GetLocale(): Promise<string>
          OpenFolder(): Promise<string>
          ScanFolder(dir: string): Promise<FolderScanResult>
          ParseFile(path: string): Promise<SubtitleFile>
          ExtractTrack(videoPath: string, trackIndex: number): Promise<SubtitleFile>
          GeneratePreviewFrame(
            fileId: string,
            videoPath: string,
            styles: SubtitleStyle[],
            atMs: number,
          ): Promise<FrameResult>
          SaveFile(fileId: string, styles: SubtitleStyle[]): Promise<void>
          SaveAll(fileStyles: Record<string, SubtitleStyle[]>): Promise<void>
          CheckAutosave(): Promise<ProjectState | null>
          RestoreProject(): Promise<ProjectState>
          Autosave(state: ProjectState): Promise<void>
          DeleteAutosave(): Promise<void>
          GetVideoDuration(videoPath: string): Promise<number>
        }
      }
    }
  }
}
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/services/ frontend/src/wails.d.ts
git commit -m "feat: add TypeScript types and Wails service wrappers"
```

---

### Task 13: Frontend i18n Setup

**Files:**
- Create: `frontend/src/i18n/en.json`
- Create: `frontend/src/i18n/ru.json`
- Create: `frontend/src/i18n/index.ts`

- [ ] **Step 1: Create English translations**

Create `frontend/src/i18n/en.json`:
```json
{
  "app": {
    "title": "Subtitle Style Editor"
  },
  "toolbar": {
    "open": "Open Folder",
    "save": "Save",
    "undo": "Undo",
    "redo": "Redo"
  },
  "fileTree": {
    "title": "Files & Styles",
    "noFiles": "No files loaded. Open a folder to begin.",
    "embedded": "embedded"
  },
  "editor": {
    "title": "Style Editor",
    "noSelection": "Select a style to edit",
    "multipleSelected": "{count} styles selected",
    "fontName": "Font Name",
    "fontSize": "Font Size",
    "bold": "Bold",
    "italic": "Italic",
    "underline": "Underline",
    "strikeout": "Strikeout",
    "primaryColour": "Primary Color",
    "secondaryColour": "Secondary Color",
    "outlineColour": "Outline Color",
    "backColour": "Back Color",
    "outline": "Outline",
    "shadow": "Shadow",
    "scaleX": "Scale X",
    "scaleY": "Scale Y",
    "spacing": "Spacing",
    "angle": "Angle",
    "alignment": "Alignment",
    "marginL": "Margin L",
    "marginR": "Margin R",
    "marginV": "Margin V"
  },
  "preview": {
    "title": "Preview",
    "noVideo": "No video file associated",
    "loading": "Generating preview...",
    "ffmpegMissing": "ffmpeg not available for preview"
  },
  "timeline": {
    "noEvents": "No events for selected style"
  },
  "ffmpeg": {
    "downloading": "ffmpeg not found, downloading...",
    "progress": "Downloading ffmpeg: {percent}%",
    "ready": "ffmpeg ready",
    "error": "ffmpeg error: {message}"
  },
  "project": {
    "restoreTitle": "Unsaved Work Found",
    "restoreMessage": "There is unsaved work from {date}. Would you like to continue?",
    "restoreYes": "Continue",
    "restoreNo": "Start Fresh",
    "saved": "Saved successfully",
    "saveError": "Failed to save: {message}"
  },
  "tracks": {
    "selectTitle": "Select Subtitle Tracks",
    "selectMessage": "Choose which subtitle tracks to extract:",
    "extract": "Extract",
    "cancel": "Cancel"
  }
}
```

- [ ] **Step 2: Create Russian translations**

Create `frontend/src/i18n/ru.json`:
```json
{
  "app": {
    "title": "Редактор стилей субтитров"
  },
  "toolbar": {
    "open": "Открыть папку",
    "save": "Сохранить",
    "undo": "Отменить",
    "redo": "Повторить"
  },
  "fileTree": {
    "title": "Файлы и стили",
    "noFiles": "Нет загруженных файлов. Откройте папку для начала работы.",
    "embedded": "встроенные"
  },
  "editor": {
    "title": "Редактор стилей",
    "noSelection": "Выберите стиль для редактирования",
    "multipleSelected": "Выбрано стилей: {count}",
    "fontName": "Шрифт",
    "fontSize": "Размер",
    "bold": "Жирный",
    "italic": "Курсив",
    "underline": "Подчёркивание",
    "strikeout": "Зачёркивание",
    "primaryColour": "Основной цвет",
    "secondaryColour": "Доп. цвет",
    "outlineColour": "Цвет обводки",
    "backColour": "Цвет фона",
    "outline": "Обводка",
    "shadow": "Тень",
    "scaleX": "Масштаб X",
    "scaleY": "Масштаб Y",
    "spacing": "Интервал",
    "angle": "Угол",
    "alignment": "Выравнивание",
    "marginL": "Отступ Л",
    "marginR": "Отступ П",
    "marginV": "Отступ В"
  },
  "preview": {
    "title": "Превью",
    "noVideo": "Видеофайл не привязан",
    "loading": "Генерация превью...",
    "ffmpegMissing": "ffmpeg недоступен для превью"
  },
  "timeline": {
    "noEvents": "Нет событий для выбранного стиля"
  },
  "ffmpeg": {
    "downloading": "ffmpeg не найден, скачиваю...",
    "progress": "Скачивание ffmpeg: {percent}%",
    "ready": "ffmpeg готов",
    "error": "Ошибка ffmpeg: {message}"
  },
  "project": {
    "restoreTitle": "Найдена незавершённая работа",
    "restoreMessage": "Есть незавершённая работа от {date}. Продолжить?",
    "restoreYes": "Продолжить",
    "restoreNo": "Начать заново",
    "saved": "Сохранено",
    "saveError": "Ошибка сохранения: {message}"
  },
  "tracks": {
    "selectTitle": "Выбор дорожек субтитров",
    "selectMessage": "Выберите дорожки для извлечения:",
    "extract": "Извлечь",
    "cancel": "Отмена"
  }
}
```

- [ ] **Step 3: Create i18n setup**

Create `frontend/src/i18n/index.ts`:
```typescript
import { createI18n } from 'vue-i18n'
import en from './en.json'
import ru from './ru.json'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackLocale: 'en',
  messages: { en, ru },
})

export default i18n

export function setLocale(locale: string): void {
  const supported = ['en', 'ru']
  const resolved = supported.includes(locale) ? locale : 'en'
  i18n.global.locale.value = resolved
  localStorage.setItem('locale', resolved)
}

export function getSavedLocale(): string | null {
  return localStorage.getItem('locale')
}
```

- [ ] **Step 4: Update main.ts to use i18n**

Replace `frontend/src/main.ts`:
```typescript
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import i18n from './i18n'
import App from './App.vue'

const app = createApp(App)
app.use(createPinia())
app.use(i18n)
app.mount('#app')
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/i18n/ frontend/src/main.ts
git commit -m "feat: add i18n setup with EN and RU translations"
```

---

### Task 14: Pinia Stores — Project, Undo, Preview

**Files:**
- Create: `frontend/src/stores/project.ts`
- Create: `frontend/src/stores/undo.ts`
- Create: `frontend/src/stores/preview.ts`

- [ ] **Step 1: Create undo store**

Create `frontend/src/stores/undo.ts`:
```typescript
import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { UndoEntry, UndoChange } from '@/services/types'

export const useUndoStore = defineStore('undo', () => {
  const undoStack = ref<UndoEntry[]>([])
  const redoStack = ref<UndoEntry[]>([])
  let nextId = 1

  function push(description: string, changes: UndoChange[]): UndoEntry {
    const entry: UndoEntry = {
      id: nextId++,
      description,
      changes,
    }
    undoStack.value.push(entry)
    redoStack.value = [] // clear redo on new action
    return entry
  }

  function undo(): UndoEntry | null {
    const entry = undoStack.value.pop()
    if (!entry) return null
    redoStack.value.push(entry)
    return entry
  }

  function redo(): UndoEntry | null {
    const entry = redoStack.value.pop()
    if (!entry) return null
    undoStack.value.push(entry)
    return entry
  }

  function canUndo(): boolean {
    return undoStack.value.length > 0
  }

  function canRedo(): boolean {
    return redoStack.value.length > 0
  }

  function clear(): void {
    undoStack.value = []
    redoStack.value = []
    nextId = 1
  }

  function restore(undo: UndoEntry[], redo: UndoEntry[]): void {
    undoStack.value = undo
    redoStack.value = redo
    const maxId = Math.max(
      0,
      ...undo.map(e => e.id),
      ...redo.map(e => e.id),
    )
    nextId = maxId + 1
  }

  return {
    undoStack,
    redoStack,
    push,
    undo,
    redo,
    canUndo,
    canRedo,
    clear,
    restore,
  }
})
```

- [ ] **Step 2: Create preview store**

Create `frontend/src/stores/preview.ts`:
```typescript
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const usePreviewStore = defineStore('preview', () => {
  const frameBase64 = ref<string | null>(null)
  const timecode = ref('')
  const loading = ref(false)
  const ffmpegReady = ref(false)
  const ffmpegDownloading = ref(false)
  const ffmpegProgress = ref(0)
  const currentTimeMs = ref(0)
  const videoDurationMs = ref(0)

  function setFrame(base64: string, tc: string): void {
    frameBase64.value = base64
    timecode.value = tc
    loading.value = false
  }

  function setLoading(isLoading: boolean): void {
    loading.value = isLoading
  }

  function clearFrame(): void {
    frameBase64.value = null
    timecode.value = ''
  }

  return {
    frameBase64,
    timecode,
    loading,
    ffmpegReady,
    ffmpegDownloading,
    ffmpegProgress,
    currentTimeMs,
    videoDurationMs,
    setFrame,
    setLoading,
    clearFrame,
  }
})
```

- [ ] **Step 3: Create main project store**

Create `frontend/src/stores/project.ts`:
```typescript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type {
  ScannedFile,
  SubtitleFile,
  SubtitleStyle,
  SubtitleEvent,
  UndoChange,
  ProjectState,
  FileState,
} from '@/services/types'
import { useUndoStore } from './undo'
import { usePreviewStore } from './preview'
import * as scanService from '@/services/scan'
import * as parserService from '@/services/parser'
import * as editorService from '@/services/editor'
import * as projectService from '@/services/project'
import { durationToMs } from '@/services/types'

interface LoadedFile {
  id: string
  path: string
  videoPath: string
  source: 'external' | 'embedded'
  trackId: number
  originalStyles: SubtitleStyle[]
  modifiedStyles: SubtitleStyle[]
  events: SubtitleEvent[]
}

export const useProjectStore = defineStore('project', () => {
  const folderPath = ref('')
  const scannedFiles = ref<ScannedFile[]>([])
  const loadedFiles = ref<Map<string, LoadedFile>>(new Map())
  const selectedStyleKeys = ref<string[]>([]) // "fileId::styleName"
  const dirty = ref(false)

  const undoStore = useUndoStore()
  const previewStore = usePreviewStore()

  let autosaveTimer: ReturnType<typeof setTimeout> | null = null

  // Computed: currently selected file for preview
  const activeFile = computed<LoadedFile | null>(() => {
    if (selectedStyleKeys.value.length === 0) return null
    const firstKey = selectedStyleKeys.value[0]
    const fileId = firstKey.split('::')[0]
    return loadedFiles.value.get(fileId) ?? null
  })

  // Computed: selected styles
  const selectedStyles = computed<Array<{ fileId: string; style: SubtitleStyle }>>(() => {
    const result: Array<{ fileId: string; style: SubtitleStyle }> = []
    for (const key of selectedStyleKeys.value) {
      const [fileId, styleName] = key.split('::')
      const file = loadedFiles.value.get(fileId)
      if (!file) continue
      const style = file.modifiedStyles.find(s => s.name === styleName)
      if (style) result.push({ fileId, style })
    }
    return result
  })

  async function openFolder(): Promise<void> {
    const dir = await scanService.openFolder()
    if (!dir) return

    folderPath.value = dir
    const result = await scanService.scanFolder(dir)
    scannedFiles.value = result.files
    loadedFiles.value.clear()
    selectedStyleKeys.value = []
    undoStore.clear()
    dirty.value = false
  }

  async function loadFile(scannedFile: ScannedFile): Promise<void> {
    let sf: SubtitleFile

    if (scannedFile.type === 'embedded') {
      // Will be handled by extractTrack
      return
    }

    sf = await parserService.parseFile(scannedFile.path)

    const loaded: LoadedFile = {
      id: sf.id,
      path: sf.path,
      videoPath: scannedFile.videoPath,
      source: sf.source as 'external' | 'embedded',
      trackId: sf.trackId,
      originalStyles: structuredClone(sf.styles),
      modifiedStyles: structuredClone(sf.styles),
      events: sf.events,
    }
    loadedFiles.value.set(sf.id, loaded)

    if (scannedFile.videoPath && previewStore.ffmpegReady) {
      const durationMs = await editorService.getVideoDuration(scannedFile.videoPath)
      previewStore.videoDurationMs = durationMs
    }
  }

  async function extractTrack(videoPath: string, trackIndex: number): Promise<void> {
    const sf = await parserService.extractTrack(videoPath, trackIndex)

    const loaded: LoadedFile = {
      id: sf.id,
      path: sf.path,
      videoPath: videoPath,
      source: 'embedded',
      trackId: sf.trackId,
      originalStyles: structuredClone(sf.styles),
      modifiedStyles: structuredClone(sf.styles),
      events: sf.events,
    }
    loadedFiles.value.set(sf.id, loaded)
  }

  function selectStyle(fileId: string, styleName: string, multi: boolean): void {
    const key = `${fileId}::${styleName}`
    if (multi) {
      const idx = selectedStyleKeys.value.indexOf(key)
      if (idx >= 0) {
        selectedStyleKeys.value.splice(idx, 1)
      } else {
        selectedStyleKeys.value.push(key)
      }
    } else {
      selectedStyleKeys.value = [key]
    }
  }

  function updateStyle(fileId: string, styleName: string, field: string, value: unknown): void {
    const file = loadedFiles.value.get(fileId)
    if (!file) return

    const styleIdx = file.modifiedStyles.findIndex(s => s.name === styleName)
    if (styleIdx < 0) return

    const oldValue = (file.modifiedStyles[styleIdx] as Record<string, unknown>)[field]

    // Build changes for all selected styles with this field
    const changes: UndoChange[] = []

    for (const key of selectedStyleKeys.value) {
      const [fId, sName] = key.split('::')
      const f = loadedFiles.value.get(fId)
      if (!f) continue

      const sIdx = f.modifiedStyles.findIndex(s => s.name === sName)
      if (sIdx < 0) continue

      const oldVal = (f.modifiedStyles[sIdx] as Record<string, unknown>)[field]
      changes.push({
        fileId: fId,
        styleName: sName,
        field,
        oldValue: structuredClone(oldVal),
        newValue: structuredClone(value),
      })

      // Apply the change
      ;(f.modifiedStyles[sIdx] as Record<string, unknown>)[field] = structuredClone(value)
    }

    if (changes.length > 0) {
      undoStore.push(`Change ${field}`, changes)
      dirty.value = true
      scheduleAutosave()
    }
  }

  function applyUndo(): void {
    const entry = undoStore.undo()
    if (!entry) return

    for (const change of entry.changes) {
      const file = loadedFiles.value.get(change.fileId)
      if (!file) continue
      const style = file.modifiedStyles.find(s => s.name === change.styleName)
      if (!style) continue
      ;(style as Record<string, unknown>)[change.field] = structuredClone(change.oldValue)
    }
    dirty.value = true
    scheduleAutosave()
  }

  function applyRedo(): void {
    const entry = undoStore.redo()
    if (!entry) return

    for (const change of entry.changes) {
      const file = loadedFiles.value.get(change.fileId)
      if (!file) continue
      const style = file.modifiedStyles.find(s => s.name === change.styleName)
      if (!style) continue
      ;(style as Record<string, unknown>)[change.field] = structuredClone(change.newValue)
    }
    dirty.value = true
    scheduleAutosave()
  }

  async function save(): Promise<void> {
    const fileStyles: Record<string, SubtitleStyle[]> = {}
    for (const [id, file] of loadedFiles.value) {
      fileStyles[id] = file.modifiedStyles
    }
    await parserService.saveAll(fileStyles)
    dirty.value = false

    // Update originals to match
    for (const file of loadedFiles.value.values()) {
      file.originalStyles = structuredClone(file.modifiedStyles)
    }

    scheduleAutosave()
  }

  function scheduleAutosave(): void {
    if (autosaveTimer) clearTimeout(autosaveTimer)
    autosaveTimer = setTimeout(() => {
      doAutosave()
    }, 2000)
  }

  async function doAutosave(): Promise<void> {
    const files: FileState[] = []
    for (const file of loadedFiles.value.values()) {
      files.push({
        id: file.id,
        path: file.path,
        source: file.source,
        trackId: file.trackId,
        videoPath: file.videoPath,
        originalStyles: file.originalStyles,
        modifiedStyles: file.modifiedStyles,
        events: file.events,
      })
    }

    const state: ProjectState = {
      folderPath: folderPath.value,
      savedAt: new Date().toISOString(),
      dirty: dirty.value,
      files,
      undoStack: undoStore.undoStack,
      redoStack: undoStore.redoStack,
      activeFileId: activeFile.value?.id ?? '',
      selectedStyles: selectedStyleKeys.value,
    }

    await projectService.autosave(state)
  }

  async function restoreFromAutosave(state: ProjectState): Promise<void> {
    folderPath.value = state.folderPath
    dirty.value = state.dirty
    selectedStyleKeys.value = state.selectedStyles

    loadedFiles.value.clear()
    for (const fs of state.files) {
      loadedFiles.value.set(fs.id, {
        id: fs.id,
        path: fs.path,
        videoPath: fs.videoPath,
        source: fs.source as 'external' | 'embedded',
        trackId: fs.trackId,
        originalStyles: fs.originalStyles,
        modifiedStyles: fs.modifiedStyles,
        events: fs.events,
      })
    }

    undoStore.restore(state.undoStack, state.redoStack)

    // Re-scan folder to get scannedFiles
    if (state.folderPath) {
      const result = await scanService.scanFolder(state.folderPath)
      scannedFiles.value = result.files
    }
  }

  function getEventsForStyle(fileId: string, styleName: string): SubtitleEvent[] {
    const file = loadedFiles.value.get(fileId)
    if (!file) return []
    return file.events.filter(e => e.styleName === styleName)
  }

  return {
    folderPath,
    scannedFiles,
    loadedFiles,
    selectedStyleKeys,
    dirty,
    activeFile,
    selectedStyles,
    openFolder,
    loadFile,
    extractTrack,
    selectStyle,
    updateStyle,
    applyUndo,
    applyRedo,
    save,
    restoreFromAutosave,
    getEventsForStyle,
  }
})
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/stores/
git commit -m "feat: add Pinia stores for project state, undo/redo, and preview"
```

---

### Task 15: Vue Components — Toolbar and FileTree

**Files:**
- Create: `frontend/src/components/Toolbar.vue`
- Create: `frontend/src/components/FileTree.vue`

- [ ] **Step 1: Create Toolbar component**

Create `frontend/src/components/Toolbar.vue`:
```vue
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { NButton, NButtonGroup, NSpace, NSelect } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { useUndoStore } from '@/stores/undo'
import { setLocale } from '@/i18n'
import { computed } from 'vue'

const { t, locale } = useI18n()
const projectStore = useProjectStore()
const undoStore = useUndoStore()

const localeOptions = [
  { label: 'EN', value: 'en' },
  { label: 'RU', value: 'ru' },
]

const currentLocale = computed({
  get: () => locale.value,
  set: (val: string) => setLocale(val),
})

async function handleOpen() {
  try {
    await projectStore.openFolder()
  } catch (e) {
    console.error('Failed to open folder:', e)
  }
}

async function handleSave() {
  try {
    await projectStore.save()
  } catch (e) {
    console.error('Failed to save:', e)
  }
}

function handleUndo() {
  projectStore.applyUndo()
}

function handleRedo() {
  projectStore.applyRedo()
}
</script>

<template>
  <div class="toolbar">
    <NSpace align="center" :size="8">
      <NButton @click="handleOpen" size="small">
        {{ t('toolbar.open') }}
      </NButton>
      <NButton
        @click="handleSave"
        size="small"
        :disabled="!projectStore.dirty"
      >
        {{ t('toolbar.save') }}
      </NButton>
      <NButtonGroup size="small">
        <NButton
          @click="handleUndo"
          :disabled="!undoStore.canUndo()"
        >
          {{ t('toolbar.undo') }}
        </NButton>
        <NButton
          @click="handleRedo"
          :disabled="!undoStore.canRedo()"
        >
          {{ t('toolbar.redo') }}
        </NButton>
      </NButtonGroup>
    </NSpace>
    <NSelect
      v-model:value="currentLocale"
      :options="localeOptions"
      size="small"
      style="width: 70px"
    />
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  border-bottom: 1px solid var(--n-border-color);
  background: var(--n-color);
}
</style>
```

- [ ] **Step 2: Create FileTree component**

Create `frontend/src/components/FileTree.vue`:
```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { NTree, NText, NTag, NSpace } from 'naive-ui'
import type { TreeOption } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import type { ScannedFile, SubtitleStyle } from '@/services/types'
import { basename } from '@/services/types'

const { t } = useI18n()
const projectStore = useProjectStore()

const expandedKeys = ref<string[]>([])

const treeData = computed<TreeOption[]>(() => {
  const nodes: TreeOption[] = []

  // External subtitle files
  for (const sf of projectStore.scannedFiles) {
    if (sf.type === 'external') {
      const fileNode = buildExternalFileNode(sf)
      nodes.push(fileNode)
    } else {
      const fileNode = buildEmbeddedFileNode(sf)
      nodes.push(fileNode)
    }
  }

  return nodes
})

function buildExternalFileNode(sf: ScannedFile): TreeOption {
  const loaded = projectStore.loadedFiles.get(sf.path)
  const children: TreeOption[] = loaded
    ? loaded.modifiedStyles.map(s => buildStyleNode(sf.path, s))
    : []

  return {
    key: sf.path,
    label: getFileName(sf.path),
    children,
    isLeaf: false,
  }
}

function buildEmbeddedFileNode(sf: ScannedFile): TreeOption {
  const children: TreeOption[] = []

  for (const track of sf.tracks) {
    const trackId = `${sf.path}:track:${track.index}`
    const loaded = projectStore.loadedFiles.get(trackId)
    const trackLabel = track.title || track.language || `Track ${track.index}`

    if (loaded) {
      const styleNodes = loaded.modifiedStyles.map(s => buildStyleNode(trackId, s))
      children.push({
        key: trackId,
        label: trackLabel,
        children: styleNodes,
        isLeaf: false,
      })
    } else {
      children.push({
        key: trackId,
        label: trackLabel,
        isLeaf: true,
      })
    }
  }

  return {
    key: `embedded:${sf.path}`,
    label: `🎬 ${getFileName(sf.path)}`,
    children,
    isLeaf: false,
  }
}

function buildStyleNode(fileId: string, style: SubtitleStyle): TreeOption {
  return {
    key: `${fileId}::${style.name}`,
    label: style.name,
    isLeaf: true,
    suffix: () => buildStyleSuffix(style),
  }
}

function buildStyleSuffix(style: SubtitleStyle): string {
  const parts = [style.fontName, `${style.fontSize}`]
  if (style.bold) parts.push('B')
  if (style.italic) parts.push('I')
  return parts.join(' ')
}

function getFileName(path: string): string {
  const parts = path.replace(/\\/g, '/').split('/')
  return parts[parts.length - 1]
}

async function handleExpand(keys: string[], option: TreeOption[]) {
  expandedKeys.value = keys as string[]

  // Load file when expanding
  for (const key of keys) {
    if (typeof key !== 'string') continue

    // External file
    const scanned = projectStore.scannedFiles.find(f => f.path === key)
    if (scanned && scanned.type === 'external' && !projectStore.loadedFiles.has(key)) {
      await projectStore.loadFile(scanned)
    }
  }
}

function handleSelect(keys: Array<string | number>, option: TreeOption[]) {
  // Only handle style nodes (contain ::)
  const styleKeys = (keys as string[]).filter(k => k.includes('::'))
  if (styleKeys.length > 0) {
    const [fileId, styleName] = styleKeys[0].split('::')
    const isMulti = window.event instanceof MouseEvent && window.event.ctrlKey
    projectStore.selectStyle(fileId, styleName, isMulti)
  }
}

function handleTrackClick(videoPath: string, trackIndex: number) {
  const trackId = `${videoPath}:track:${trackIndex}`
  if (!projectStore.loadedFiles.has(trackId)) {
    projectStore.extractTrack(videoPath, trackIndex)
  }
}
</script>

<template>
  <div class="file-tree">
    <div class="panel-header">{{ t('fileTree.title') }}</div>
    <div v-if="projectStore.scannedFiles.length === 0" class="empty-state">
      <NText depth="3">{{ t('fileTree.noFiles') }}</NText>
    </div>
    <NTree
      v-else
      :data="treeData"
      :expanded-keys="expandedKeys"
      :selected-keys="projectStore.selectedStyleKeys"
      selectable
      multiple
      @update:expanded-keys="handleExpand"
      @update:selected-keys="handleSelect"
      block-line
    />
  </div>
</template>

<style scoped>
.file-tree {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: auto;
}

.panel-header {
  padding: 8px 12px;
  font-weight: 600;
  font-size: 13px;
  border-bottom: 1px solid var(--n-border-color);
  background: var(--n-color);
}

.empty-state {
  padding: 16px;
  text-align: center;
}
</style>
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/Toolbar.vue frontend/src/components/FileTree.vue
git commit -m "feat: add Toolbar and FileTree Vue components"
```

---

### Task 16: Vue Components — StyleEditor

**Files:**
- Create: `frontend/src/components/StyleEditor.vue`

- [ ] **Step 1: Create StyleEditor component**

Create `frontend/src/components/StyleEditor.vue`:
```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NInput,
  NInputNumber,
  NButton,
  NButtonGroup,
  NColorPicker,
  NSpace,
  NText,
  NGrid,
  NGi,
  NDivider,
} from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import type { Color } from '@/services/types'

const { t } = useI18n()
const projectStore = useProjectStore()

const selected = computed(() => projectStore.selectedStyles)

const hasSelection = computed(() => selected.value.length > 0)
const isMultiple = computed(() => selected.value.length > 1)

// For single selection, return the style directly.
// For multiple, return the first one (fields show first value).
const currentStyle = computed(() => {
  if (selected.value.length === 0) return null
  return selected.value[0].style
})

function updateField(field: string, value: unknown) {
  if (!currentStyle.value) return
  const firstSelected = selected.value[0]
  projectStore.updateStyle(firstSelected.fileId, firstSelected.style.name, field, value)
}

function colorToRgba(c: Color): string {
  return `rgba(${c.r}, ${c.g}, ${c.b}, ${c.a / 255})`
}

function rgbaToColor(rgba: string): Color {
  const match = rgba.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)/)
  if (!match) return { r: 255, g: 255, b: 255, a: 255 }
  return {
    r: parseInt(match[1]),
    g: parseInt(match[2]),
    b: parseInt(match[3]),
    a: match[4] !== undefined ? Math.round(parseFloat(match[4]) * 255) : 255,
  }
}

function handleColorChange(field: string, rgba: string) {
  updateField(field, rgbaToColor(rgba))
}

const alignments = [7, 8, 9, 4, 5, 6, 1, 2, 3] // numpad layout top-to-bottom
</script>

<template>
  <div class="style-editor">
    <div class="panel-header">{{ t('editor.title') }}</div>

    <div v-if="!hasSelection" class="empty-state">
      <NText depth="3">{{ t('editor.noSelection') }}</NText>
    </div>

    <div v-else class="editor-content">
      <div v-if="isMultiple" class="multi-info">
        <NText>{{ t('editor.multipleSelected', { count: selected.length }) }}</NText>
      </div>

      <!-- Font Name -->
      <div class="field-group">
        <label>{{ t('editor.fontName') }}</label>
        <NInput
          :value="currentStyle?.fontName"
          @update:value="(v: string) => updateField('fontName', v)"
          size="small"
        />
      </div>

      <!-- Font Size -->
      <div class="field-group">
        <label>{{ t('editor.fontSize') }}</label>
        <NInputNumber
          :value="currentStyle?.fontSize"
          @update:value="(v: number | null) => v !== null && updateField('fontSize', v)"
          size="small"
          :min="1"
        />
      </div>

      <!-- Bold, Italic, Underline, Strikeout -->
      <div class="field-group">
        <NButtonGroup size="small">
          <NButton
            :type="currentStyle?.bold ? 'primary' : 'default'"
            @click="updateField('bold', !currentStyle?.bold)"
            style="font-weight: bold"
          >B</NButton>
          <NButton
            :type="currentStyle?.italic ? 'primary' : 'default'"
            @click="updateField('italic', !currentStyle?.italic)"
            style="font-style: italic"
          >I</NButton>
          <NButton
            :type="currentStyle?.underline ? 'primary' : 'default'"
            @click="updateField('underline', !currentStyle?.underline)"
            style="text-decoration: underline"
          >U</NButton>
          <NButton
            :type="currentStyle?.strikeout ? 'primary' : 'default'"
            @click="updateField('strikeout', !currentStyle?.strikeout)"
            style="text-decoration: line-through"
          >S</NButton>
        </NButtonGroup>
      </div>

      <NDivider style="margin: 8px 0" />

      <!-- Colors -->
      <div class="field-group">
        <label>{{ t('editor.primaryColour') }}</label>
        <NColorPicker
          :value="currentStyle ? colorToRgba(currentStyle.primaryColour) : undefined"
          @update:value="(v: string) => handleColorChange('primaryColour', v)"
          size="small"
          :modes="['rgba']"
          :show-alpha="true"
        />
      </div>

      <div class="field-group">
        <label>{{ t('editor.secondaryColour') }}</label>
        <NColorPicker
          :value="currentStyle ? colorToRgba(currentStyle.secondaryColour) : undefined"
          @update:value="(v: string) => handleColorChange('secondaryColour', v)"
          size="small"
          :modes="['rgba']"
          :show-alpha="true"
        />
      </div>

      <div class="field-group">
        <label>{{ t('editor.outlineColour') }}</label>
        <NColorPicker
          :value="currentStyle ? colorToRgba(currentStyle.outlineColour) : undefined"
          @update:value="(v: string) => handleColorChange('outlineColour', v)"
          size="small"
          :modes="['rgba']"
          :show-alpha="true"
        />
      </div>

      <div class="field-group">
        <label>{{ t('editor.backColour') }}</label>
        <NColorPicker
          :value="currentStyle ? colorToRgba(currentStyle.backColour) : undefined"
          @update:value="(v: string) => handleColorChange('backColour', v)"
          size="small"
          :modes="['rgba']"
          :show-alpha="true"
        />
      </div>

      <NDivider style="margin: 8px 0" />

      <!-- Outline & Shadow -->
      <NGrid :cols="2" :x-gap="8">
        <NGi>
          <div class="field-group">
            <label>{{ t('editor.outline') }}</label>
            <NInputNumber
              :value="currentStyle?.outline"
              @update:value="(v: number | null) => v !== null && updateField('outline', v)"
              size="small"
              :min="0"
            />
          </div>
        </NGi>
        <NGi>
          <div class="field-group">
            <label>{{ t('editor.shadow') }}</label>
            <NInputNumber
              :value="currentStyle?.shadow"
              @update:value="(v: number | null) => v !== null && updateField('shadow', v)"
              size="small"
              :min="0"
            />
          </div>
        </NGi>
      </NGrid>

      <!-- Scale X/Y -->
      <NGrid :cols="2" :x-gap="8">
        <NGi>
          <div class="field-group">
            <label>{{ t('editor.scaleX') }}</label>
            <NInputNumber
              :value="currentStyle?.scaleX"
              @update:value="(v: number | null) => v !== null && updateField('scaleX', v)"
              size="small"
              :min="0"
            />
          </div>
        </NGi>
        <NGi>
          <div class="field-group">
            <label>{{ t('editor.scaleY') }}</label>
            <NInputNumber
              :value="currentStyle?.scaleY"
              @update:value="(v: number | null) => v !== null && updateField('scaleY', v)"
              size="small"
              :min="0"
            />
          </div>
        </NGi>
      </NGrid>

      <!-- Spacing & Angle -->
      <NGrid :cols="2" :x-gap="8">
        <NGi>
          <div class="field-group">
            <label>{{ t('editor.spacing') }}</label>
            <NInputNumber
              :value="currentStyle?.spacing"
              @update:value="(v: number | null) => v !== null && updateField('spacing', v)"
              size="small"
            />
          </div>
        </NGi>
        <NGi>
          <div class="field-group">
            <label>{{ t('editor.angle') }}</label>
            <NInputNumber
              :value="currentStyle?.angle"
              @update:value="(v: number | null) => v !== null && updateField('angle', v)"
              size="small"
            />
          </div>
        </NGi>
      </NGrid>

      <NDivider style="margin: 8px 0" />

      <!-- Alignment 3x3 grid -->
      <div class="field-group">
        <label>{{ t('editor.alignment') }}</label>
        <div class="alignment-grid">
          <button
            v-for="a in alignments"
            :key="a"
            :class="['align-btn', { active: currentStyle?.alignment === a }]"
            @click="updateField('alignment', a)"
          >
            {{ a }}
          </button>
        </div>
      </div>

      <!-- Margins -->
      <NGrid :cols="3" :x-gap="8">
        <NGi>
          <div class="field-group">
            <label>{{ t('editor.marginL') }}</label>
            <NInputNumber
              :value="currentStyle?.marginL"
              @update:value="(v: number | null) => v !== null && updateField('marginL', v)"
              size="small"
              :min="0"
            />
          </div>
        </NGi>
        <NGi>
          <div class="field-group">
            <label>{{ t('editor.marginR') }}</label>
            <NInputNumber
              :value="currentStyle?.marginR"
              @update:value="(v: number | null) => v !== null && updateField('marginR', v)"
              size="small"
              :min="0"
            />
          </div>
        </NGi>
        <NGi>
          <div class="field-group">
            <label>{{ t('editor.marginV') }}</label>
            <NInputNumber
              :value="currentStyle?.marginV"
              @update:value="(v: number | null) => v !== null && updateField('marginV', v)"
              size="small"
              :min="0"
            />
          </div>
        </NGi>
      </NGrid>
    </div>
  </div>
</template>

<style scoped>
.style-editor {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: auto;
}

.panel-header {
  padding: 8px 12px;
  font-weight: 600;
  font-size: 13px;
  border-bottom: 1px solid var(--n-border-color);
  background: var(--n-color);
}

.empty-state {
  padding: 16px;
  text-align: center;
}

.editor-content {
  padding: 8px 12px;
  overflow-y: auto;
}

.multi-info {
  padding: 4px 0 8px;
}

.field-group {
  margin-bottom: 8px;
}

.field-group label {
  display: block;
  font-size: 11px;
  color: var(--n-text-color-3);
  margin-bottom: 2px;
}

.alignment-grid {
  display: grid;
  grid-template-columns: repeat(3, 28px);
  gap: 2px;
}

.align-btn {
  width: 28px;
  height: 28px;
  border: 1px solid var(--n-border-color);
  border-radius: 3px;
  background: var(--n-color);
  cursor: pointer;
  font-size: 11px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.align-btn.active {
  background: var(--n-color-target);
  color: white;
  border-color: var(--n-color-target);
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/StyleEditor.vue
git commit -m "feat: add StyleEditor component with all ASS style fields"
```

---

### Task 17: Vue Components — Preview, Timeline, CSS Overlay

**Files:**
- Create: `frontend/src/components/CssSubtitleOverlay.vue`
- Create: `frontend/src/components/Timeline.vue`
- Create: `frontend/src/components/PreviewArea.vue`

- [ ] **Step 1: Create CSS subtitle overlay**

Create `frontend/src/components/CssSubtitleOverlay.vue`:
```vue
<script setup lang="ts">
import { computed } from 'vue'
import type { SubtitleStyle, Color } from '@/services/types'

const props = defineProps<{
  style: SubtitleStyle | null
  text: string
}>()

function colorToCss(c: Color): string {
  return `rgba(${c.r}, ${c.g}, ${c.b}, ${c.a / 255})`
}

const cssStyle = computed(() => {
  const s = props.style
  if (!s) return {}

  const result: Record<string, string> = {
    fontFamily: s.fontName || 'Arial',
    fontSize: `${s.fontSize * 0.6}px`, // approximate scaling for preview
    color: colorToCss(s.primaryColour),
    fontWeight: s.bold ? 'bold' : 'normal',
    fontStyle: s.italic ? 'italic' : 'normal',
    letterSpacing: `${s.spacing}px`,
    transform: `scaleX(${(s.scaleX || 100) / 100}) scaleY(${(s.scaleY || 100) / 100}) rotate(${s.angle || 0}deg)`,
  }

  // Text decorations
  const decorations: string[] = []
  if (s.underline) decorations.push('underline')
  if (s.strikeout) decorations.push('line-through')
  if (decorations.length) result.textDecoration = decorations.join(' ')

  // Outline
  if (s.outline > 0) {
    result.webkitTextStroke = `${s.outline}px ${colorToCss(s.outlineColour)}`
    result.paintOrder = 'stroke fill'
  }

  // Shadow
  if (s.shadow > 0) {
    result.textShadow = `${s.shadow}px ${s.shadow}px 0px ${colorToCss(s.backColour)}`
  }

  return result
})

// ASS alignment to CSS positioning
const positionStyle = computed(() => {
  const s = props.style
  if (!s) return {}

  const a = s.alignment || 2
  const result: Record<string, string> = {
    position: 'absolute',
    padding: `${s.marginV || 10}px ${s.marginR || 10}px ${s.marginV || 10}px ${s.marginL || 10}px`,
  }

  // Vertical: 1-3 bottom, 4-6 middle, 7-9 top
  if (a <= 3) {
    result.bottom = '0'
  } else if (a <= 6) {
    result.top = '50%'
    result.transform = 'translateY(-50%)'
  } else {
    result.top = '0'
  }

  // Horizontal: 1,4,7 left; 2,5,8 center; 3,6,9 right
  const col = ((a - 1) % 3)
  if (col === 0) {
    result.left = '0'
    result.textAlign = 'left'
  } else if (col === 1) {
    result.left = '0'
    result.right = '0'
    result.textAlign = 'center'
  } else {
    result.right = '0'
    result.textAlign = 'right'
  }

  return result
})
</script>

<template>
  <div class="css-overlay" :style="positionStyle">
    <span :style="cssStyle" class="subtitle-text">{{ text }}</span>
  </div>
</template>

<style scoped>
.css-overlay {
  pointer-events: none;
  z-index: 10;
}

.subtitle-text {
  white-space: pre-wrap;
  line-height: 1.3;
}
</style>
```

- [ ] **Step 2: Create Timeline component**

Create `frontend/src/components/Timeline.vue`:
```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton, NText } from 'naive-ui'
import type { SubtitleEvent } from '@/services/types'
import { usePreviewStore } from '@/stores/preview'
import { durationToMs, msToTimecode } from '@/services/types'

const props = defineProps<{
  events: SubtitleEvent[]
  currentEventIndex: number
}>()

const emit = defineEmits<{
  seek: [timeMs: number, eventIndex: number]
  prev: []
  next: []
}>()

const { t } = useI18n()
const previewStore = usePreviewStore()

const totalMs = computed(() => previewStore.videoDurationMs || 1)

const markers = computed(() => {
  return props.events.map((ev, i) => ({
    index: i,
    startMs: durationToMs(ev.startTime),
    leftPercent: (durationToMs(ev.startTime) / totalMs.value) * 100,
    active: i === props.currentEventIndex,
  }))
})

function handleMarkerClick(index: number) {
  const ev = props.events[index]
  const ms = durationToMs(ev.startTime)
  emit('seek', ms, index)
}
</script>

<template>
  <div class="timeline">
    <NText depth="3" class="timecode">{{ msToTimecode(0) }}</NText>
    <div class="timeline-bar" v-if="events.length > 0">
      <div
        v-for="marker in markers"
        :key="marker.index"
        class="marker"
        :class="{ active: marker.active }"
        :style="{ left: `${marker.leftPercent}%` }"
        @click="handleMarkerClick(marker.index)"
      />
    </div>
    <div v-else class="timeline-bar empty">
      <NText depth="3" style="font-size: 11px">{{ t('timeline.noEvents') }}</NText>
    </div>
    <NText depth="3" class="timecode">{{ msToTimecode(totalMs) }}</NText>
    <NButton size="tiny" quaternary @click="$emit('prev')" :disabled="events.length === 0">
      ◀
    </NButton>
    <NButton size="tiny" quaternary @click="$emit('next')" :disabled="events.length === 0">
      ▶
    </NButton>
  </div>
</template>

<style scoped>
.timeline {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  background: #2a2a2a;
  border-top: 1px solid #444;
}

.timecode {
  font-size: 10px;
  font-family: monospace;
  color: #888;
  white-space: nowrap;
}

.timeline-bar {
  flex: 1;
  height: 16px;
  background: #333;
  border-radius: 8px;
  position: relative;
  cursor: pointer;
}

.timeline-bar.empty {
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: default;
}

.marker {
  position: absolute;
  width: 3px;
  height: 100%;
  background: #4a9eff;
  border-radius: 2px;
  cursor: pointer;
  transition: background 0.15s;
}

.marker:hover {
  background: #6bb5ff;
}

.marker.active {
  background: #ff9e4a;
}
</style>
```

- [ ] **Step 3: Create PreviewArea component**

Create `frontend/src/components/PreviewArea.vue`:
```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NSpin, NText } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { usePreviewStore } from '@/stores/preview'
import CssSubtitleOverlay from './CssSubtitleOverlay.vue'
import Timeline from './Timeline.vue'
import * as editorService from '@/services/editor'
import { durationToMs } from '@/services/types'
import type { SubtitleEvent } from '@/services/types'

const { t } = useI18n()
const projectStore = useProjectStore()
const previewStore = usePreviewStore()

const currentEventIndex = ref(0)

// Events for the first selected style
const styleEvents = computed<SubtitleEvent[]>(() => {
  if (projectStore.selectedStyles.length === 0) return []
  const first = projectStore.selectedStyles[0]
  return projectStore.getEventsForStyle(first.fileId, first.style.name)
})

// Current event text for CSS overlay
const overlayText = computed(() => {
  if (styleEvents.value.length === 0) return ''
  const idx = Math.min(currentEventIndex.value, styleEvents.value.length - 1)
  return styleEvents.value[idx]?.text ?? ''
})

// Current style for CSS overlay
const overlayStyle = computed(() => {
  if (projectStore.selectedStyles.length === 0) return null
  return projectStore.selectedStyles[0].style
})

// Video path for preview
const videoPath = computed(() => projectStore.activeFile?.videoPath ?? '')

// Debounced ffmpeg preview
let debounceTimer: ReturnType<typeof setTimeout> | null = null

watch(
  [() => projectStore.selectedStyles, () => currentEventIndex.value],
  () => {
    if (!previewStore.ffmpegReady || !videoPath.value) return
    if (projectStore.selectedStyles.length === 0) return

    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => requestFfmpegFrame(), 500)
  },
  { deep: true },
)

async function requestFfmpegFrame() {
  const file = projectStore.activeFile
  if (!file || !videoPath.value) return

  const idx = Math.min(currentEventIndex.value, styleEvents.value.length - 1)
  if (idx < 0) return

  const ev = styleEvents.value[idx]
  const atMs = durationToMs(ev.startTime) + 500 // slightly after start

  previewStore.setLoading(true)
  try {
    const result = await editorService.generatePreviewFrame(
      file.id,
      videoPath.value,
      file.modifiedStyles,
      atMs,
    )
    previewStore.setFrame(result.base64Png, result.timecode)
  } catch (e) {
    console.error('Preview frame error:', e)
    previewStore.setLoading(false)
  }
}

function handleSeek(timeMs: number, eventIndex: number) {
  currentEventIndex.value = eventIndex
  previewStore.currentTimeMs = timeMs
}

function handlePrev() {
  if (currentEventIndex.value > 0) {
    currentEventIndex.value--
    const ev = styleEvents.value[currentEventIndex.value]
    previewStore.currentTimeMs = durationToMs(ev.startTime)
  }
}

function handleNext() {
  if (currentEventIndex.value < styleEvents.value.length - 1) {
    currentEventIndex.value++
    const ev = styleEvents.value[currentEventIndex.value]
    previewStore.currentTimeMs = durationToMs(ev.startTime)
  }
}
</script>

<template>
  <div class="preview-area">
    <div class="frame-container">
      <div v-if="!videoPath" class="no-video">
        <NText depth="3">{{ t('preview.noVideo') }}</NText>
      </div>
      <template v-else>
        <!-- FFmpeg rendered frame -->
        <img
          v-if="previewStore.frameBase64"
          :src="'data:image/png;base64,' + previewStore.frameBase64"
          class="preview-frame"
          alt="Preview"
        />
        <!-- CSS overlay (shown when no ffmpeg frame or as instant preview) -->
        <div
          v-if="!previewStore.frameBase64"
          class="css-preview-bg"
        >
          <CssSubtitleOverlay
            :style="overlayStyle"
            :text="overlayText"
          />
        </div>
        <!-- Loading spinner -->
        <div v-if="previewStore.loading" class="loading-overlay">
          <NSpin size="small" />
        </div>
      </template>
    </div>
    <Timeline
      :events="styleEvents"
      :current-event-index="currentEventIndex"
      @seek="handleSeek"
      @prev="handlePrev"
      @next="handleNext"
    />
  </div>
</template>

<style scoped>
.preview-area {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #1a1a1a;
}

.frame-container {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  min-height: 0;
}

.no-video {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}

.preview-frame {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.css-preview-bg {
  width: 90%;
  height: 80%;
  background: #333;
  border-radius: 4px;
  position: relative;
}

.loading-overlay {
  position: absolute;
  top: 8px;
  right: 8px;
}
</style>
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/CssSubtitleOverlay.vue frontend/src/components/Timeline.vue frontend/src/components/PreviewArea.vue
git commit -m "feat: add PreviewArea, Timeline, and CssSubtitleOverlay components"
```

---

### Task 18: Main View, Restore Dialog, and App Wiring

**Files:**
- Create: `frontend/src/views/MainView.vue`
- Create: `frontend/src/views/RestoreDialog.vue`
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Create RestoreDialog**

Create `frontend/src/views/RestoreDialog.vue`:
```vue
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { NModal, NCard, NButton, NSpace, NText } from 'naive-ui'
import type { ProjectState } from '@/services/types'

const props = defineProps<{
  show: boolean
  state: ProjectState | null
}>()

const emit = defineEmits<{
  restore: []
  discard: []
}>()

const { t } = useI18n()

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}
</script>

<template>
  <NModal :show="show" :mask-closable="false">
    <NCard
      :title="t('project.restoreTitle')"
      style="width: 420px"
      :bordered="false"
    >
      <NText>
        {{ t('project.restoreMessage', { date: state ? formatDate(state.savedAt) : '' }) }}
      </NText>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="$emit('discard')">
            {{ t('project.restoreNo') }}
          </NButton>
          <NButton type="primary" @click="$emit('restore')">
            {{ t('project.restoreYes') }}
          </NButton>
        </NSpace>
      </template>
    </NCard>
  </NModal>
</template>
```

- [ ] **Step 2: Create MainView**

Create `frontend/src/views/MainView.vue`:
```vue
<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import Toolbar from '@/components/Toolbar.vue'
import FileTree from '@/components/FileTree.vue'
import StyleEditor from '@/components/StyleEditor.vue'
import PreviewArea from '@/components/PreviewArea.vue'
import { useProjectStore } from '@/stores/project'
import { useUndoStore } from '@/stores/undo'
import { usePreviewStore } from '@/stores/preview'
import {
  onFFmpegReady,
  onFFmpegDownloading,
  onFFmpegProgress,
  onFFmpegError,
} from '@/services/ffmpeg'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'

const projectStore = useProjectStore()
const undoStore = useUndoStore()
const previewStore = usePreviewStore()
const message = useMessage()
const { t } = useI18n()

function handleKeydown(e: KeyboardEvent) {
  if (e.ctrlKey && e.key === 'z' && !e.shiftKey) {
    e.preventDefault()
    projectStore.applyUndo()
  } else if (
    (e.ctrlKey && e.key === 'y') ||
    (e.ctrlKey && e.shiftKey && e.key === 'Z')
  ) {
    e.preventDefault()
    projectStore.applyRedo()
  } else if (e.ctrlKey && e.key === 's') {
    e.preventDefault()
    projectStore.save().then(() => {
      message.success(t('project.saved'))
    }).catch((err: Error) => {
      message.error(t('project.saveError', { message: err.message }))
    })
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)

  onFFmpegReady(() => {
    previewStore.ffmpegReady = true
    previewStore.ffmpegDownloading = false
  })

  onFFmpegDownloading(() => {
    previewStore.ffmpegDownloading = true
    message.info(t('ffmpeg.downloading'))
  })

  onFFmpegProgress((progress) => {
    if (progress.total > 0) {
      previewStore.ffmpegProgress = Math.round((progress.received / progress.total) * 100)
    }
  })

  onFFmpegError((error) => {
    message.error(t('ffmpeg.error', { message: error }))
  })
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="main-layout">
    <Toolbar />
    <div class="content">
      <div class="panel-left">
        <FileTree />
      </div>
      <div class="panel-center">
        <PreviewArea />
      </div>
      <div class="panel-right">
        <StyleEditor />
      </div>
    </div>
  </div>
</template>

<style scoped>
.main-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

.content {
  display: flex;
  flex: 1;
  min-height: 0;
}

.panel-left {
  width: 260px;
  border-right: 1px solid var(--n-border-color);
  overflow: hidden;
}

.panel-center {
  flex: 1;
  min-width: 0;
}

.panel-right {
  width: 280px;
  border-left: 1px solid var(--n-border-color);
  overflow: hidden;
}
</style>
```

- [ ] **Step 3: Update App.vue**

Replace `frontend/src/App.vue`:
```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NConfigProvider, NMessageProvider, NDialogProvider } from 'naive-ui'
import MainView from '@/views/MainView.vue'
import RestoreDialog from '@/views/RestoreDialog.vue'
import { useProjectStore } from '@/stores/project'
import * as projectService from '@/services/project'
import { setLocale, getSavedLocale } from '@/i18n'
import type { ProjectState } from '@/services/types'

const projectStore = useProjectStore()

const showRestore = ref(false)
const restoreState = ref<ProjectState | null>(null)
const initialized = ref(false)

onMounted(async () => {
  // Detect locale
  const saved = getSavedLocale()
  if (saved) {
    setLocale(saved)
  } else {
    try {
      const detected = await projectService.getLocale()
      setLocale(detected)
    } catch {
      setLocale('en')
    }
  }

  // Check for autosave
  try {
    const state = await projectService.checkAutosave()
    if (state) {
      restoreState.value = state
      showRestore.value = true
      return
    }
  } catch (e) {
    console.error('Failed to check autosave:', e)
  }

  initialized.value = true
})

async function handleRestore() {
  showRestore.value = false
  if (restoreState.value) {
    await projectStore.restoreFromAutosave(restoreState.value)
  }
  initialized.value = true
}

async function handleDiscard() {
  showRestore.value = false
  await projectService.deleteAutosave()
  initialized.value = true
}
</script>

<template>
  <NConfigProvider>
    <NMessageProvider>
      <NDialogProvider>
        <RestoreDialog
          :show="showRestore"
          :state="restoreState"
          @restore="handleRestore"
          @discard="handleDiscard"
        />
        <MainView v-if="initialized" />
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>

<style>
html, body {
  margin: 0;
  padding: 0;
  height: 100%;
  overflow: hidden;
}

#app {
  height: 100%;
}
</style>
```

- [ ] **Step 4: Verify frontend compiles**

Run:
```bash
cd /home/hakastein/work/subtitles/frontend && npm run build
```
Expected: compiles (or shows fixable type errors). Fix any issues.

Note: Wails generates `frontend/wailsjs/` bindings. During standalone frontend build, the Wails runtime imports will fail. This is expected — the app compiles correctly via `wails build`. For standalone `npm run build` to work, you may need to create stub files:

Create `frontend/wailsjs/runtime/runtime.d.ts`:
```typescript
export function EventsOn(eventName: string, callback: (...args: never[]) => void): void
export function EventsEmit(eventName: string, ...args: unknown[]): void
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/ frontend/src/App.vue
git commit -m "feat: add MainView, RestoreDialog, and App wiring with keyboard shortcuts"
```

---

### Task 19: Full Build and Integration Test

**Files:** No new files — verification only.

- [ ] **Step 1: Run all Go tests**

Run:
```bash
cd /home/hakastein/work/subtitles && go test ./... -v
```
Expected: all tests pass.

- [ ] **Step 2: Run Go build**

Run:
```bash
cd /home/hakastein/work/subtitles && go build ./cmd/app/
```
Expected: compiles without errors.

- [ ] **Step 3: Install frontend dependencies and build**

Run:
```bash
cd /home/hakastein/work/subtitles/frontend && npm install && npm run build
```
Expected: builds successfully (may need Wails runtime stubs).

- [ ] **Step 4: Run Wails build**

Run:
```bash
cd /home/hakastein/work/subtitles && wails build
```
Expected: produces `build/bin/subtitles-editor.exe`.

- [ ] **Step 5: Commit final state**

```bash
git add .
git commit -m "feat: complete subtitle style editor — all packages and components wired"
```
