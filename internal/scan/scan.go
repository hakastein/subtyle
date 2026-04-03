package scan

import (
	"os"
	"path/filepath"
	"strings"
)

// ScannedFile represents a subtitle file found during folder scan.
type ScannedFile struct {
	Path      string      `json:"path"`
	VideoPath string      `json:"videoPath"`
	Type      string      `json:"type"` // "external" or "embedded"
	Tracks    []TrackInfo `json:"tracks"`
}

// TrackInfo represents a subtitle track within a video file.
type TrackInfo struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Title    string `json:"title"`
}

// FolderScanResult holds results of a folder scan.
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

// ScanFolder scans a directory (non-recursive) for subtitle and video files,
// matching each subtitle file to a video by progressively shorter name prefixes.
func ScanFolder(dir string) (*FolderScanResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var subtitlePaths []string
	// map from base name (without extension) to full path
	videoByName := make(map[string]string)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		fullPath := filepath.Join(dir, name)

		if subtitleExts[ext] {
			subtitlePaths = append(subtitlePaths, fullPath)
		} else if videoExts[ext] {
			baseName := strings.TrimSuffix(name, filepath.Ext(name))
			videoByName[baseName] = fullPath
		}
	}

	result := &FolderScanResult{
		Files: make([]ScannedFile, 0, len(subtitlePaths)),
	}

	for _, subPath := range subtitlePaths {
		subName := filepath.Base(subPath)
		subExt := filepath.Ext(subName)
		stem := strings.TrimSuffix(subName, subExt)

		videoPath := findVideoMatch(stem, videoByName)

		result.Files = append(result.Files, ScannedFile{
			Path:      subPath,
			VideoPath: videoPath,
			Type:      "external",
			Tracks:    []TrackInfo{},
		})
	}

	return result, nil
}

// findVideoMatch tries to match a subtitle stem to a video by progressively
// stripping dot-separated suffixes from the right.
func findVideoMatch(stem string, videoByName map[string]string) string {
	candidate := stem
	for {
		if path, ok := videoByName[candidate]; ok {
			return path
		}
		// Find the last dot and strip from there
		idx := strings.LastIndex(candidate, ".")
		if idx < 0 {
			break
		}
		candidate = candidate[:idx]
	}
	return ""
}
