package project

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"subtitles-editor/internal/parser"
)

func init() {
	gob.Register(parser.Color{})
	gob.Register(float64(0))
	gob.Register(int(0))
	gob.Register("")
	gob.Register(true)
}

// UndoChange represents a single field change that can be undone.
type UndoChange struct {
	FileID    string
	StyleName string
	Field     string
	OldValue  interface{}
	NewValue  interface{}
}

// UndoEntry groups related changes into a single undo operation.
type UndoEntry struct {
	ID          int
	Description string
	Changes     []UndoChange
}

// FileState holds the state of a single subtitle file.
type FileState struct {
	ID             string
	Path           string
	Source         string
	TrackID        int
	VideoPath      string
	OriginalStyles []parser.SubtitleStyle
	ModifiedStyles []parser.SubtitleStyle
	Events         []parser.SubtitleEvent
}

// ProjectState holds the full state of the project for autosave/restore.
type ProjectState struct {
	FolderPath     string
	SavedAt        time.Time
	Dirty          bool
	Files          []FileState
	UndoStack      []UndoEntry
	RedoStack      []UndoEntry
	ActiveFileID   string
	SelectedStyles []string
}

const autosaveFile = "autosave.gob"

// Manager handles autosave persistence for project state.
type Manager struct {
	dataDir string
}

// NewManager creates a new Manager that stores autosave data in dataDir.
func NewManager(dataDir string) *Manager {
	return &Manager{dataDir: dataDir}
}

func (m *Manager) autosavePath() string {
	return filepath.Join(m.dataDir, autosaveFile)
}

// Save writes the project state to disk atomically using a temp file + rename.
func (m *Manager) Save(state *ProjectState) error {
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return fmt.Errorf("project: creating data dir: %w", err)
	}

	tmpPath := m.autosavePath() + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("project: creating temp file: %w", err)
	}

	enc := gob.NewEncoder(f)
	if err := enc.Encode(state); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("project: encoding state: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("project: closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, m.autosavePath()); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("project: renaming temp file: %w", err)
	}

	return nil
}

// Load reads the project state from the autosave file.
func (m *Manager) Load() (*ProjectState, error) {
	f, err := os.Open(m.autosavePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project: no autosave found")
		}
		return nil, fmt.Errorf("project: opening autosave: %w", err)
	}
	defer f.Close()

	var state ProjectState
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&state); err != nil {
		return nil, fmt.Errorf("project: decoding state: %w", err)
	}

	return &state, nil
}

// HasAutosave returns true if an autosave file exists.
func (m *Manager) HasAutosave() bool {
	_, err := os.Stat(m.autosavePath())
	return err == nil
}

// Delete removes the autosave file.
func (m *Manager) Delete() error {
	err := os.Remove(m.autosavePath())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("project: deleting autosave: %w", err)
	}
	return nil
}
