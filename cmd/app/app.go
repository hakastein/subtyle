package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	ctx         context.Context
	ffmpegMgr   *ffmpeg.Manager
	extractor   *ffmpeg.Extractor
	previewGen  *preview.Generator
	projectMgr  *project.Manager
	dataDir     string
	parsedFiles map[string]*parser.SubtitleFile
}

// newApp creates a new App with all managers initialized.
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

// startup is called when the app starts. The context is stored and ffmpeg
// initialization is launched in a goroutine so the UI is not blocked.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.initFFmpeg()
}

// initFFmpeg attempts to find or download the ffmpeg binary and emits
// progress events to the frontend.
func (a *App) initFFmpeg() {
	if path := a.ffmpegMgr.Find(); path != "" {
		a.extractor = ffmpeg.NewExtractor(path)
		a.previewGen = preview.NewGenerator(a.extractor)
		runtime.EventsEmit(a.ctx, "ffmpeg:ready")
		return
	}

	runtime.EventsEmit(a.ctx, "ffmpeg:downloading")

	err := a.ffmpegMgr.Download(context.Background(), func(received, total int64) {
		runtime.EventsEmit(a.ctx, "ffmpeg:progress", received, total)
	})
	if err != nil {
		runtime.EventsEmit(a.ctx, "ffmpeg:error", err.Error())
		return
	}

	path := a.ffmpegMgr.BinPath()
	a.extractor = ffmpeg.NewExtractor(path)
	a.previewGen = preview.NewGenerator(a.extractor)
	runtime.EventsEmit(a.ctx, "ffmpeg:ready")
}

// GetLocale returns the detected system locale ("en" or "ru").
func (a *App) GetLocale() string {
	return i18nPkg.DetectLocale()
}

// OpenFolder opens a native directory picker dialog and returns the selected path.
func (a *App) OpenFolder() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// ScanFolder scans the given directory for subtitle and video files.
// For video files that have embedded ASS tracks, those tracks are added
// as additional ScannedFile entries.
func (a *App) ScanFolder(dir string) (*scan.FolderScanResult, error) {
	result, err := scan.ScanFolder(dir)
	if err != nil {
		return nil, err
	}

	if a.extractor != nil {
		if err := a.scanEmbeddedTracks(dir, result); err != nil {
			// Non-fatal: return what we have from the basic scan.
			return result, nil
		}
	}

	return result, nil
}

// scanEmbeddedTracks iterates all video files in the directory and appends
// embedded ASS track entries to the scan result.
func (a *App) scanEmbeddedTracks(dir string, result *scan.FolderScanResult) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true,
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !videoExts[ext] {
			continue
		}

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

// ParseFile parses an ASS/SSA subtitle file from the given path and caches it.
func (a *App) ParseFile(path string) (*parser.SubtitleFile, error) {
	sf, err := parser.ParseFile(path)
	if err != nil {
		return nil, err
	}
	a.parsedFiles[sf.ID] = sf
	return sf, nil
}

// ExtractTrack extracts an embedded subtitle track from a video file,
// parses it, caches it, and returns the SubtitleFile.
func (a *App) ExtractTrack(videoPath string, trackIndex int) (*parser.SubtitleFile, error) {
	if a.extractor == nil {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	tmpFile, err := os.CreateTemp("", "subtitles_track_*.ass")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := a.extractor.ExtractTrack(context.Background(), videoPath, trackIndex, tmpPath); err != nil {
		return nil, err
	}

	sf, err := parser.ParseFile(tmpPath)
	if err != nil {
		return nil, err
	}

	// Use a stable ID based on video path and track index.
	sf.ID = fmt.Sprintf("%s:track:%d", filepath.Base(videoPath), trackIndex)
	sf.Source = "embedded"
	sf.TrackID = trackIndex
	a.parsedFiles[sf.ID] = sf
	return sf, nil
}

// GeneratePreviewFrame renders a single preview frame for the given file
// with the provided styles applied at the given time offset (in milliseconds).
func (a *App) GeneratePreviewFrame(fileID string, videoPath string, styles []parser.SubtitleStyle, atMs int64) (*preview.FrameResult, error) {
	if a.previewGen == nil {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	orig, ok := a.parsedFiles[fileID]
	if !ok {
		return nil, fmt.Errorf("file %q not found", fileID)
	}

	// Create a copy with the updated styles.
	modified := *orig
	modified.Styles = styles

	at := time.Duration(atMs) * time.Millisecond
	return a.previewGen.GenerateFrame(a.ctx, videoPath, &modified, at)
}

// SaveFile saves the styles for the given file ID. For embedded-source files,
// the output is written next to the video as "filename.[modified].ass".
func (a *App) SaveFile(fileID string, styles []parser.SubtitleStyle) error {
	sf, ok := a.parsedFiles[fileID]
	if !ok {
		return fmt.Errorf("file %q not found", fileID)
	}

	// Apply the provided styles.
	modified := *sf
	modified.Styles = styles

	var outPath string
	if sf.Source == "embedded" {
		// Save next to the video file.
		base := strings.TrimSuffix(sf.Path, filepath.Ext(sf.Path))
		outPath = base + ".[modified].ass"
	} else {
		outPath = sf.Path
	}

	return parser.WriteFile(outPath, &modified)
}

// SaveAll iterates over the provided map of fileID→styles and calls SaveFile for each.
func (a *App) SaveAll(fileStyles map[string][]parser.SubtitleStyle) error {
	for fileID, styles := range fileStyles {
		if err := a.SaveFile(fileID, styles); err != nil {
			return fmt.Errorf("saving %q: %w", fileID, err)
		}
	}
	return nil
}

// CheckAutosave returns the project state if an autosave exists and is dirty,
// otherwise returns nil.
func (a *App) CheckAutosave() (*project.ProjectState, error) {
	if !a.projectMgr.HasAutosave() {
		return nil, nil
	}
	state, err := a.projectMgr.Load()
	if err != nil {
		return nil, err
	}
	if !state.Dirty {
		return nil, nil
	}
	return state, nil
}

// RestoreProject loads and returns the saved project state from disk.
func (a *App) RestoreProject() (*project.ProjectState, error) {
	return a.projectMgr.Load()
}

// Autosave sets the SavedAt timestamp and persists the project state to disk.
func (a *App) Autosave(state *project.ProjectState) error {
	state.SavedAt = time.Now()
	return a.projectMgr.Save(state)
}

// DeleteAutosave removes the autosave file from disk.
func (a *App) DeleteAutosave() error {
	return a.projectMgr.Delete()
}

// GetVideoDuration returns the total duration of the given video file in milliseconds.
func (a *App) GetVideoDuration(videoPath string) (int64, error) {
	if a.extractor == nil {
		return 0, fmt.Errorf("ffmpeg not available")
	}
	dur, err := a.extractor.VideoDuration(context.Background(), videoPath)
	if err != nil {
		return 0, err
	}
	return dur.Milliseconds(), nil
}

// Ensure editor package is used (it is imported for potential future use in bindings).
var _ = editor.ApplyChange
