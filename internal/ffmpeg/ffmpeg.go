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
)

const ffmpegDownloadURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"

// ProgressFunc is called during download with bytes received and total size.
type ProgressFunc func(received, total int64)

// Manager handles finding and downloading ffmpeg.
type Manager struct {
	dataDir string
	binPath string
}

// NewManager creates a new Manager with the given data directory.
func NewManager(dataDir string) *Manager {
	return &Manager{dataDir: dataDir}
}

// BinPath returns the resolved path to the ffmpeg binary (empty if not yet found).
func (m *Manager) BinPath() string {
	return m.binPath
}

// Find checks PATH first, then the cached location. Returns path or empty string.
func (m *Manager) Find() string {
	// Check PATH
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		m.binPath = path
		return path
	}

	// Check cached location
	cached := m.cachedBinaryPath()
	if m.binaryExists() {
		m.binPath = cached
		return cached
	}

	return ""
}

// Download downloads the static ffmpeg build for Windows amd64.
// It extracts bin/ffmpeg.exe from the zip and stores it in dataDir.
func (m *Manager) Download(ctx context.Context, progressFn ProgressFunc) error {
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Download to temp file
	tmpFile, err := os.CreateTemp(m.dataDir, "ffmpeg-download-*.zip.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL(), nil)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	total := resp.ContentLength
	var received int64

	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tmpFile.Write(buf[:n]); writeErr != nil {
				tmpFile.Close()
				return fmt.Errorf("write temp file: %w", writeErr)
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
			return fmt.Errorf("read response: %w", readErr)
		}
	}
	tmpFile.Close()

	// Extract ffmpeg.exe from the zip
	destTmp := m.cachedBinaryPath() + ".tmp"
	if err := extractFFmpegFromZip(tmpPath, destTmp); err != nil {
		os.Remove(destTmp)
		return fmt.Errorf("extract ffmpeg: %w", err)
	}

	// Atomic rename
	if err := os.Rename(destTmp, m.cachedBinaryPath()); err != nil {
		os.Remove(destTmp)
		return fmt.Errorf("rename ffmpeg binary: %w", err)
	}

	m.binPath = m.cachedBinaryPath()
	return nil
}

// cachedBinaryPath returns the expected path for the cached ffmpeg binary.
func (m *Manager) cachedBinaryPath() string {
	return filepath.Join(m.dataDir, "ffmpeg.exe")
}

// binaryExists reports whether the cached binary exists on disk.
func (m *Manager) binaryExists() bool {
	_, err := os.Stat(m.cachedBinaryPath())
	return err == nil
}

// downloadURL returns the URL used to download ffmpeg.
func downloadURL() string {
	return ffmpegDownloadURL
}

// extractFFmpegFromZip extracts bin/ffmpeg.exe from the zip archive at zipPath
// and writes it to destPath.
func extractFFmpegFromZip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		// Match path like "ffmpeg-master-latest-win64-gpl/bin/ffmpeg.exe"
		if filepath.Base(f.Name) != "ffmpeg.exe" {
			continue
		}
		dir := filepath.Dir(f.Name)
		if filepath.Base(dir) != "bin" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry: %w", err)
		}

		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			return fmt.Errorf("create dest file: %w", err)
		}

		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return fmt.Errorf("copy ffmpeg binary: %w", copyErr)
		}
		return nil
	}

	return fmt.Errorf("ffmpeg.exe not found in zip archive")
}
