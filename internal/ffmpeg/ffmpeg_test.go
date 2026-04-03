package ffmpeg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCachedBinaryPath(t *testing.T) {
	m := NewManager("/some/data/dir")
	expected := filepath.Join("/some/data/dir", "ffmpeg.exe")
	got := m.cachedBinaryPath()
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestBinaryExists_Missing(t *testing.T) {
	m := NewManager(t.TempDir())
	if m.binaryExists() {
		t.Error("expected binaryExists to return false for missing file")
	}
}

func TestBinaryExists_Present(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Create the binary file
	if err := os.WriteFile(m.cachedBinaryPath(), []byte("fake"), 0755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	if !m.binaryExists() {
		t.Error("expected binaryExists to return true for existing file")
	}
}

func TestDownloadURL_NonEmpty(t *testing.T) {
	url := downloadURL()
	if url == "" {
		t.Error("expected non-empty download URL")
	}
}
