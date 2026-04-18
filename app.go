package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"subtitles-editor/internal/crashlog"
	"subtitles-editor/internal/ffmpeg"
	i18nPkg "subtitles-editor/internal/i18n"
	"subtitles-editor/internal/mkv"
	"subtitles-editor/internal/parser"
	"subtitles-editor/internal/preview"
	"subtitles-editor/internal/project"
	"subtitles-editor/internal/scan"
)

// guard wraps an App method's body with panic recovery. The recovered panic
// is logged to crash.log and returned as a generic error so the frontend
// still receives something (instead of the whole process dying).
func (a *App) guard(name string, retErr *error) {
	if r := recover(); r != nil {
		err := crashlog.RecoverFrom(name, r)
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("[PANIC] %s: %v", name, r))
			runtime.EventsEmit(a.ctx, "app:error", err.Error())
		}
		if retErr != nil {
			*retErr = err
		}
	}
}

// App holds the application state and exposes methods to the frontend via Wails bindings.
type App struct {
	ctx         context.Context
	ffmpegMgr   *ffmpeg.Manager
	extractor   *ffmpeg.Extractor
	previewGen  *preview.Generator
	projectMgr  *project.Manager
	dataDir     string
	parsedFiles map[string]*parser.SubtitleFile

	ffmpegStateMu sync.Mutex
	ffmpegState   FfmpegState

	lastFolderMu sync.Mutex
	lastFolder   string
}

// setFfmpegState updates the bootstrap status and emits a snapshot event.
func (a *App) setFfmpegState(status string, progress float64, errMsg string) {
	a.ffmpegStateMu.Lock()
	a.ffmpegState = FfmpegState{Status: status, Progress: progress, Error: errMsg}
	a.ffmpegStateMu.Unlock()
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
	crashlog.Init(a.dataDir)
	go crashlog.Guard("initFFmpeg", a.initFFmpeg)
}

// GetCrashLogPath returns the path to the crash log file (if any).
func (a *App) GetCrashLogPath() string {
	return crashlog.Path()
}

// initFFmpeg attempts to find or download the ffmpeg binary and emits
// progress events to the frontend.
func (a *App) initFFmpeg() {
	a.setFfmpegState("not_found", 0, "")

	if path := a.ffmpegMgr.Find(); path != "" {
		a.extractor = ffmpeg.NewExtractor(path)
		a.previewGen = preview.NewGenerator(a.extractor, preview.NewCache(filepath.Join(a.dataDir, "preview-cache"), 100*1024*1024))
		diag := a.extractor.Diagnose(a.ctx)
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("ffmpeg: %s | subtitles filter: %v | libass: %v", diag.Version, diag.HasSubtitlesFilter, diag.HasLibass))
		if !diag.HasSubtitlesFilter {
			runtime.EventsEmit(a.ctx, "debug:log", "WARNING: ffmpeg does not have subtitles filter — subtitle overlay will not work!")
		}
		a.setFfmpegState("ready", 1, "")
		runtime.EventsEmit(a.ctx, "ffmpeg:ready")
		return
	}

	a.setFfmpegState("downloading", 0, "")
	runtime.EventsEmit(a.ctx, "ffmpeg:downloading")

	err := a.ffmpegMgr.Download(context.Background(), func(received, total int64) {
		pct := 0.0
		if total > 0 {
			pct = float64(received) / float64(total)
		}
		a.setFfmpegState("downloading", pct, "")
		runtime.EventsEmit(a.ctx, "ffmpeg:progress", received, total)
	})
	if err != nil {
		a.setFfmpegState("error", 0, err.Error())
		runtime.EventsEmit(a.ctx, "ffmpeg:error", err.Error())
		return
	}

	path := a.ffmpegMgr.BinPath()
	a.extractor = ffmpeg.NewExtractor(path)
	a.previewGen = preview.NewGenerator(a.extractor, preview.NewCache(filepath.Join(a.dataDir, "preview-cache"), 100*1024*1024))
	diag := a.extractor.Diagnose(a.ctx)
	runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("ffmpeg downloaded: %s | subtitles filter: %v | libass: %v", diag.Version, diag.HasSubtitlesFilter, diag.HasLibass))
	if !diag.HasSubtitlesFilter {
		runtime.EventsEmit(a.ctx, "debug:log", "WARNING: downloaded ffmpeg does not have subtitles filter!")
	}
	a.setFfmpegState("ready", 1, "")
	runtime.EventsEmit(a.ctx, "ffmpeg:ready")

	// If the user opened a folder before ffmpeg was ready, re-scan embedded tracks now.
	a.lastFolderMu.Lock()
	folder := a.lastFolder
	a.lastFolderMu.Unlock()
	if folder != "" {
		a.rescanEmbeddedAfterReady(folder)
	}
}

// rescanEmbeddedAfterReady runs scanEmbeddedTracks for a folder and emits
// the newly-discovered entries as a "scan:embedded-added" event so the
// frontend can append them to its scan state. Always emits a final
// progress:scan "done" so the toolbar bar disappears.
func (a *App) rescanEmbeddedAfterReady(dir string) {
	runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("rescanning embedded tracks in %s", dir))
	defer runtime.EventsEmit(a.ctx, "progress:scan", map[string]interface{}{"stage": "done"})

	fresh := &scan.FolderScanResult{}
	if err := a.scanEmbeddedTracks(dir, fresh); err != nil {
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("rescan failed: %v", err))
		return
	}
	runtime.EventsEmit(a.ctx, "scan:embedded-added", fresh.Files)
	runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("rescan found %d embedded entries", len(fresh.Files)))
}

// IsFfmpegReady returns true if ffmpeg has been found or downloaded.
func (a *App) IsFfmpegReady() bool {
	return a.extractor != nil
}

// FfmpegState describes the ffmpeg bootstrap status for the frontend.
type FfmpegState struct {
	Status   string  `json:"status"` // "ready" | "downloading" | "not_found" | "error"
	Progress float64 `json:"progress"` // 0..1 during download
	Error    string  `json:"error"`
}

// GetFfmpegState returns a snapshot of the ffmpeg bootstrap status so the
// frontend can recover the correct state even if it missed the emitted events
// (e.g. because the UI mounted after they fired).
func (a *App) GetFfmpegState() FfmpegState {
	a.ffmpegStateMu.Lock()
	defer a.ffmpegStateMu.Unlock()
	return a.ffmpegState
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
func (a *App) ScanFolder(dir string) (result *scan.FolderScanResult, err error) {
	defer a.guard("ScanFolder", &err)

	// Remember last folder so we can re-scan for embedded tracks once ffmpeg
	// becomes available (in case user opened the folder before download finished).
	a.lastFolderMu.Lock()
	a.lastFolder = dir
	a.lastFolderMu.Unlock()

	runtime.EventsEmit(a.ctx, "progress:scan", map[string]interface{}{
		"stage":   "reading",
		"current": 0,
		"total":   0,
		"message": "Reading directory",
	})

	result, err = scan.ScanFolder(dir)
	if err != nil {
		runtime.EventsEmit(a.ctx, "progress:scan", map[string]interface{}{"stage": "done"})
		return nil, err
	}

	if a.extractor != nil {
		if embedErr := a.scanEmbeddedTracks(dir, result); embedErr != nil {
			runtime.EventsEmit(a.ctx, "progress:scan", map[string]interface{}{"stage": "done"})
			return result, nil
		}
	}

	runtime.EventsEmit(a.ctx, "progress:scan", map[string]interface{}{"stage": "done"})
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

// ParseFile parses an ASS/SSA subtitle file from the given path and caches it.
func (a *App) ParseFile(path string) (sf *parser.SubtitleFile, err error) {
	defer a.guard("ParseFile", &err)
	sf, err = parser.ParseFile(path)
	if err != nil {
		return nil, err
	}
	a.parsedFiles[sf.ID] = sf
	return sf, nil
}

// trackCachePaths returns (stylesPath, fullPath) in the extract dir.
func (a *App) trackCachePaths(videoPath string, trackIndex int) (string, string) {
	extractDir := filepath.Join(a.dataDir, "extracted")
	base := fmt.Sprintf("%s_track%d", filepath.Base(videoPath), trackIndex)
	stylesPath := filepath.Join(extractDir, base+".styles.ass")
	fullPath := filepath.Join(extractDir, base+".ass")
	return stylesPath, fullPath
}

// ExtractTrack does a fast styles-only extraction for an embedded track.
// Only parses the MKV Tracks section (~100KB) so styles show up instantly.
// Events are loaded on-demand via EnsureFullTrack when preview or save needs them.
// Falls back to ffmpeg for non-MKV sources or when native parsing fails.
func (a *App) ExtractTrack(videoPath string, trackIndex int, trackTitle string) (sf *parser.SubtitleFile, err error) {
	defer a.guard("ExtractTrack", &err)
	if a.extractor == nil {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	extractDir := filepath.Join(a.dataDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return nil, fmt.Errorf("creating extract dir: %w", err)
	}

	stableID := fmt.Sprintf("%s:track:%d", filepath.Base(videoPath), trackIndex)
	stylesPath, fullPath := a.trackCachePaths(videoPath, trackIndex)

	// Prefer full cache if already available — then we have events too for free.
	parsePath := ""
	hasEvents := false
	if existsAndFresh(fullPath, videoPath) {
		parsePath = fullPath
		hasEvents = true
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("ExtractTrack: full cache hit %s", filepath.Base(fullPath)))
	} else if existsAndFresh(stylesPath, videoPath) {
		parsePath = stylesPath
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("ExtractTrack: styles cache hit %s", filepath.Base(stylesPath)))
	} else {
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("ExtractTrack: styles-only extract %s track %d", filepath.Base(videoPath), trackIndex))
		if err := mkv.ExtractStylesOnly(videoPath, trackIndex, stylesPath); err == nil {
			parsePath = stylesPath
			runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("[mkv native styles-only] %s track %d", filepath.Base(videoPath), trackIndex))
		} else {
			// Fall back to full extraction (ffmpeg path works on any container).
			runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("ExtractTrack: native styles-only failed (%v), full ffmpeg extract", err))
			if err := a.extractor.ExtractTrack(context.Background(), videoPath, trackIndex, fullPath); err != nil {
				return nil, err
			}
			parsePath = fullPath
			hasEvents = true
		}
	}

	sf, err = parser.ParseFile(parsePath)
	if err != nil {
		return nil, err
	}

	sf.ID = stableID
	sf.Source = "embedded"
	sf.TrackID = trackIndex
	sf.TrackTitle = trackTitle
	_ = hasEvents // kept for clarity; callers check len(sf.Events)
	a.parsedFiles[sf.ID] = sf
	return sf, nil
}

// EnsureFullTrack upgrades a previously styles-only extracted track to include
// all events. Call before preview or save for embedded tracks. No-op for
// external files and already-full tracks.
func (a *App) EnsureFullTrack(fileID string, videoPath string) (err error) {
	defer a.guard("EnsureFullTrack", &err)
	sf, ok := a.parsedFiles[fileID]
	if !ok {
		return fmt.Errorf("file %q not loaded", fileID)
	}
	if sf.Source != "embedded" || len(sf.Events) > 0 {
		return nil
	}
	if videoPath == "" {
		return fmt.Errorf("video path required for EnsureFullTrack(%q)", fileID)
	}

	_, fullPath := a.trackCachePaths(videoPath, sf.TrackID)

	if !existsAndFresh(fullPath, videoPath) {
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("EnsureFullTrack: full extract %s", filepath.Base(videoPath)))
		if err := mkv.ExtractASSTrack(videoPath, sf.TrackID, fullPath); err != nil {
			runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("EnsureFullTrack: native failed (%v), ffmpeg fallback", err))
			if err := a.extractor.ExtractTrack(context.Background(), videoPath, sf.TrackID, fullPath); err != nil {
				return err
			}
		}
	} else {
		runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("EnsureFullTrack: cache hit %s", filepath.Base(fullPath)))
	}

	full, err := parser.ParseFile(fullPath)
	if err != nil {
		return fmt.Errorf("parse full: %w", err)
	}
	// Preserve user-modified styles; replace only events + path.
	sf.Events = full.Events
	sf.Path = fullPath
	return nil
}

// existsAndFresh reports whether outPath exists and its mtime is >= the source's mtime.
// A stale extract (source changed after extraction) is treated as missing.
func existsAndFresh(outPath, srcPath string) bool {
	out, err := os.Stat(outPath)
	if err != nil || out.IsDir() || out.Size() == 0 {
		return false
	}
	src, err := os.Stat(srcPath)
	if err != nil {
		return false
	}
	return !out.ModTime().Before(src.ModTime())
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
func (a *App) GeneratePreviewFrame(fileID string, videoPath string, styles []parser.SubtitleStyle, atMs int64) (result *preview.FrameResult, err error) {
	defer a.guard("GeneratePreviewFrame", &err)
	if a.previewGen == nil {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	orig, ok := a.parsedFiles[fileID]
	if !ok {
		return nil, fmt.Errorf("file %q not found", fileID)
	}

	// Preview needs events (for the subtitles filter to have anything to render).
	// Lazily upgrade a styles-only track on first preview request.
	if orig.Source == "embedded" && len(orig.Events) == 0 {
		if err := a.EnsureFullTrack(fileID, videoPath); err != nil {
			return nil, fmt.Errorf("ensure full track: %w", err)
		}
		orig = a.parsedFiles[fileID]
	}

	modified := *orig
	modified.Styles = styles

	runtime.EventsEmit(a.ctx, "debug:log", fmt.Sprintf("preview: file.Path=%q Source=%q styles=%d events=%d", modified.Path, modified.Source, len(modified.Styles), len(modified.Events)))

	at := time.Duration(atMs) * time.Millisecond
	result, err = a.previewGen.GenerateFrame(a.ctx, videoPath, &modified, at)
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

	// Save needs events — upgrade the track if it was only styles-loaded.
	if sf.Source == "embedded" && len(sf.Events) == 0 {
		if err := a.EnsureFullTrack(req.FileID, req.VideoPath); err != nil {
			return "", fmt.Errorf("ensure full track before save: %w", err)
		}
		sf = a.parsedFiles[req.FileID]
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
