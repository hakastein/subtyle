package project

import (
	"os"
	"testing"
	"time"

	"subtitles-editor/internal/parser"
)

func makeTestState() *ProjectState {
	return &ProjectState{
		FolderPath: "/test/folder",
		SavedAt:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Dirty:      true,
		Files: []FileState{
			{
				ID:     "file1",
				Path:   "/test/folder/subs.ass",
				Source: "external",
				OriginalStyles: []parser.SubtitleStyle{
					{
						Name:          "Default",
						FontName:      "Arial",
						FontSize:      20,
						PrimaryColour: parser.Color{R: 255, G: 255, B: 255, A: 255},
					},
				},
				ModifiedStyles: []parser.SubtitleStyle{
					{
						Name:          "Default",
						FontName:      "Helvetica",
						FontSize:      24,
						Bold:          true,
						PrimaryColour: parser.Color{R: 255, G: 0, B: 0, A: 255},
					},
				},
				Events: []parser.SubtitleEvent{
					{
						StyleName: "Default",
						StartTime: 1 * time.Second,
						EndTime:   3 * time.Second,
						Text:      "Hello",
					},
				},
			},
		},
		UndoStack: []UndoEntry{
			{
				ID:          1,
				Description: "Change font size",
				Changes: []UndoChange{
					{
						FileID:    "file1",
						StyleName: "Default",
						Field:     "fontSize",
						OldValue:  float64(20),
						NewValue:  float64(24),
					},
				},
			},
		},
		RedoStack:      []UndoEntry{},
		ActiveFileID:   "file1",
		SelectedStyles: []string{"Default"},
	}
}

func TestSaveLoad(t *testing.T) {
	dir, err := os.MkdirTemp("", "project_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	m := NewManager(dir)
	state := makeTestState()

	if err := m.Save(state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if !m.HasAutosave() {
		t.Error("HasAutosave should return true after Save")
	}

	loaded, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.FolderPath != state.FolderPath {
		t.Errorf("FolderPath: expected %q, got %q", state.FolderPath, loaded.FolderPath)
	}
	if loaded.Dirty != state.Dirty {
		t.Errorf("Dirty: expected %v, got %v", state.Dirty, loaded.Dirty)
	}
	if loaded.ActiveFileID != state.ActiveFileID {
		t.Errorf("ActiveFileID: expected %q, got %q", state.ActiveFileID, loaded.ActiveFileID)
	}

	if len(loaded.SelectedStyles) != len(state.SelectedStyles) {
		t.Fatalf("SelectedStyles len: expected %d, got %d", len(state.SelectedStyles), len(loaded.SelectedStyles))
	}
	if loaded.SelectedStyles[0] != state.SelectedStyles[0] {
		t.Errorf("SelectedStyles[0]: expected %q, got %q", state.SelectedStyles[0], loaded.SelectedStyles[0])
	}

	if len(loaded.Files) != 1 {
		t.Fatalf("Files len: expected 1, got %d", len(loaded.Files))
	}
	f := loaded.Files[0]
	if f.ID != "file1" {
		t.Errorf("FileID: expected file1, got %q", f.ID)
	}

	if len(f.OriginalStyles) != 1 {
		t.Fatalf("OriginalStyles len: expected 1, got %d", len(f.OriginalStyles))
	}
	if f.OriginalStyles[0].FontName != "Arial" {
		t.Errorf("OriginalStyles FontName: expected Arial, got %q", f.OriginalStyles[0].FontName)
	}

	if len(f.ModifiedStyles) != 1 {
		t.Fatalf("ModifiedStyles len: expected 1, got %d", len(f.ModifiedStyles))
	}
	if f.ModifiedStyles[0].FontName != "Helvetica" {
		t.Errorf("ModifiedStyles FontName: expected Helvetica, got %q", f.ModifiedStyles[0].FontName)
	}
	if f.ModifiedStyles[0].FontSize != 24 {
		t.Errorf("ModifiedStyles FontSize: expected 24, got %v", f.ModifiedStyles[0].FontSize)
	}
	if !f.ModifiedStyles[0].Bold {
		t.Error("ModifiedStyles Bold: expected true")
	}
	if f.ModifiedStyles[0].PrimaryColour != (parser.Color{R: 255, G: 0, B: 0, A: 255}) {
		t.Errorf("ModifiedStyles PrimaryColour: unexpected value %v", f.ModifiedStyles[0].PrimaryColour)
	}
}

func TestDelete(t *testing.T) {
	dir, err := os.MkdirTemp("", "project_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	m := NewManager(dir)
	state := makeTestState()

	if err := m.Save(state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := m.Delete(); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if m.HasAutosave() {
		t.Error("HasAutosave should return false after Delete")
	}
}

func TestLoadWithoutAutosave(t *testing.T) {
	dir, err := os.MkdirTemp("", "project_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	m := NewManager(dir)

	_, err = m.Load()
	if err == nil {
		t.Error("expected error when loading without autosave, got nil")
	}
}
