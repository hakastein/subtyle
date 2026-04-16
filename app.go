package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

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
		diag := a.extractor.Diagnose(a.ctx)
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("ffmpeg: %s | subtitles filter: %v | libass: %v", diag.Version, diag.HasSubtitlesFilter, diag.HasLibass))
		if !diag.HasSubtitlesFilter {
			runtime.EventsEmit(a.ctx, "debug:log", "WARNING: ffmpeg does not have subtitles filter — subtitle overlay will not work!")
		}
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
	diag := a.extractor.Diagnose(a.ctx)
	runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("ffmpeg downloaded: %s | subtitles filter: %v | libass: %v", diag.Version, diag.HasSubtitlesFilter, diag.HasLibass))
	if !diag.HasSubtitlesFilter {
		runtime.EventsEmit(a.ctx, "debug:log", "WARNING: downloaded ffmpeg does not have subtitles filter!")
	}
	runtime.EventsEmit(a.ctx, "ffmpeg:ready")
}

// IsFfmpegReady returns true if ffmpeg has been found or downloaded.
func (a *App) IsFfmpegReady() bool {
	return a.extractor != nil
}

// GetFfmpegDiag returns diagnostic info about the ffmpeg binary.
func (a *App) GetFfmpegDiag() *ffmpeg.DiagInfo {
	if a.extractor == nil {
		return &ffmpeg.DiagInfo{Path: "not found"}
	}
	return a.extractor.Diagnose(a.ctx)
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
// parses it, caches it, and returns the SubtitleFile. trackTitle is the
// human-readable name from container metadata, used for saving.
func (a *App) ExtractTrack(videoPath string, trackIndex int, trackTitle string) (*parser.SubtitleFile, error) {
	if a.extractor == nil {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	// Store extracted track in app data dir so it persists for preview rendering
	extractDir := filepath.Join(a.dataDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return nil, fmt.Errorf("creating extract dir: %w", err)
	}

	stableID := fmt.Sprintf("%s:track:%d", filepath.Base(videoPath), trackIndex)
	outPath := filepath.Join(extractDir, fmt.Sprintf("%s_track%d.ass", filepath.Base(videoPath), trackIndex))

	if err := a.extractor.ExtractTrack(context.Background(), videoPath, trackIndex, outPath); err != nil {
		return nil, err
	}

	sf, err := parser.ParseFile(outPath)
	if err != nil {
		return nil, err
	}

	sf.ID = stableID
	sf.Source = "embedded"
	sf.TrackID = trackIndex
	sf.TrackTitle = trackTitle
	a.parsedFiles[sf.ID] = sf
	return sf, nil
}

// sanitizeFilename replaces characters that are invalid in Windows filenames
// with an underscore. Also trims trailing dots/spaces.
func sanitizeFilename(s string) string {
	invalid := []string{`<`, `>`, `:`, `"`, `/`, `\`, `|`, `?`, `*`}
	for _, c := range invalid {
		s = strings.ReplaceAll(s, c, "_")
	}
	s = strings.TrimRight(s, ". ")
	return s
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

	// Log the command that will be built for debugging
	if a.extractor != nil {
		// Write temp to see what path would be used
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("preview: file.Path=%q file.Source=%q styles=%d events=%d", modified.Path, modified.Source, len(modified.Styles), len(modified.Events)))
	}

	result, err := a.previewGen.GenerateFrame(a.ctx, videoPath, &modified, at)
	if err != nil {
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("GeneratePreviewFrame error: %v", err))
		return nil, err
	}
	return result, nil
}

// SaveRequest describes a single file save operation with its video context
// (needed for embedded subtitles to compute the output path next to the video).
type SaveRequest struct {
	FileID    string                 `json:"fileId"`
	VideoPath string                 `json:"videoPath"`
	Styles    []parser.SubtitleStyle `json:"styles"`
}

// SaveFile saves the styles for the given file. For embedded-source files,
// the output is written next to videoPath as "<videoname>.[modified].ass".
func (a *App) SaveFile(req SaveRequest) (string, error) {
	sf, ok := a.parsedFiles[req.FileID]
	if !ok {
		return "", fmt.Errorf("file %q not found", req.FileID)
	}

	// Apply the provided styles.
	modified := *sf
	modified.Styles = req.Styles

	var outPath string
	if sf.Source == "embedded" {
		if req.VideoPath == "" {
			return "", fmt.Errorf("videoPath required for embedded file %q", req.FileID)
		}
		// Save next to the video file.
		// Use track title if available: <videobase>.<title>.ass
		// Fall back to track index if title is empty: <videobase>.track<N>.ass
		videoDir := filepath.Dir(req.VideoPath)
		videoBase := strings.TrimSuffix(filepath.Base(req.VideoPath), filepath.Ext(req.VideoPath))
		var suffix string
		if sf.TrackTitle != "" {
			suffix = sanitizeFilename(sf.TrackTitle)
		} else {
			suffix = fmt.Sprintf("track%d", sf.TrackID)
		}
		outPath = filepath.Join(videoDir, fmt.Sprintf("%s.%s.ass", videoBase, suffix))
	} else {
		outPath = sf.Path
	}

	runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("SaveFile: %s (source=%s) → %s", req.FileID, sf.Source, outPath))

	if err := parser.WriteFile(outPath, &modified); err != nil {
		return "", err
	}
	return outPath, nil
}

// SaveAll iterates over the provided save requests and calls SaveFile for each.
// Returns a list of paths that were written, one per successful save.
func (a *App) SaveAll(requests []SaveRequest) ([]string, error) {
	var paths []string
	for _, req := range requests {
		out, err := a.SaveFile(req)
		if err != nil {
			return paths, fmt.Errorf("saving %q: %w", req.FileID, err)
		}
		paths = append(paths, out)
	}
	return paths, nil
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
