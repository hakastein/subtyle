package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func createTempFiles(t *testing.T, files []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		path := filepath.Join(dir, f)
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatalf("failed to create temp file %s: %v", f, err)
		}
	}
	return dir
}

func TestScanFolder_SimpleNames(t *testing.T) {
	dir := createTempFiles(t, []string{
		"episode01.ass",
		"episode01.mkv",
		"episode02.ass",
		"episode02.mkv",
	})

	result, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder error: %v", err)
	}

	if len(result.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result.Files))
	}

	for _, f := range result.Files {
		if f.VideoPath == "" {
			t.Errorf("expected video match for %s, got empty", f.Path)
		}
		if f.Type != "external" {
			t.Errorf("expected type 'external', got %s", f.Type)
		}
	}
}

func TestScanFolder_ComplexNames(t *testing.T) {
	dir := createTempFiles(t, []string{
		"[Kawaiika-Raws] Vinland Saga 01 [BDRip].rus.[Anku].ass",
		"[Kawaiika-Raws] Vinland Saga 01 [BDRip].mkv",
	})

	result, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder error: %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}

	f := result.Files[0]
	if f.VideoPath == "" {
		t.Errorf("expected video match for complex name, got empty")
	}

	expectedBase := "[Kawaiika-Raws] Vinland Saga 01 [BDRip].mkv"
	if filepath.Base(f.VideoPath) != expectedBase {
		t.Errorf("expected video %s, got %s", expectedBase, filepath.Base(f.VideoPath))
	}
}

func TestScanFolder_NoVideoMatch(t *testing.T) {
	dir := createTempFiles(t, []string{
		"episode01.ass",
		"other_video.mkv",
	})

	result, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder error: %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}

	f := result.Files[0]
	if f.VideoPath != "" {
		t.Errorf("expected no video match, got %s", f.VideoPath)
	}
}

func TestScanFolder_EmptyFolder(t *testing.T) {
	dir := t.TempDir()

	result, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder error: %v", err)
	}

	if len(result.Files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(result.Files))
	}
}

func TestScanFolder_SSAExtension(t *testing.T) {
	dir := createTempFiles(t, []string{
		"episode01.ssa",
		"episode01.avi",
	})

	result, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder error: %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}

	f := result.Files[0]
	if f.VideoPath == "" {
		t.Errorf("expected video match for .ssa file, got empty")
	}
}
