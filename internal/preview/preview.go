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

// FrameResult holds a rendered preview frame as base64 PNG and its timecode.
type FrameResult struct {
	Base64PNG string `json:"base64Png"`
	Timecode  string `json:"timecode"`
}

// Generator manages preview frame generation, cancelling any in-progress
// generation when a new request arrives.
type Generator struct {
	extractor *ffmpeg.Extractor
	mu        sync.Mutex
	cancelFn  context.CancelFunc
	genID     int64
}

// NewGenerator creates a Generator backed by the given Extractor.
func NewGenerator(extractor *ffmpeg.Extractor) *Generator {
	return &Generator{extractor: extractor}
}

// GenerateFrame cancels any pending generation, then renders a single frame
// from videoPath at the given offset using the styles from subFile.
// Returns the frame as a base64 PNG and the formatted timecode.
func (g *Generator) GenerateFrame(ctx context.Context, videoPath string, subFile *parser.SubtitleFile, at time.Duration, widthPx int) (*FrameResult, error) {
	// Cancel previous generation if one is in progress.
	g.mu.Lock()
	if g.cancelFn != nil {
		g.cancelFn()
	}
	genCtx, cancel := context.WithCancel(ctx)
	g.cancelFn = cancel
	g.genID++
	myID := g.genID
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		// Only clear if this is still the active generation.
		if g.genID == myID {
			g.cancelFn = nil
		}
		g.mu.Unlock()
		cancel()
	}()

	// Write a temporary ASS file with the current styles.
	tmpPath, err := parser.WriteTempFile(subFile)
	if err != nil {
		return nil, fmt.Errorf("preview: writing temp file: %w", err)
	}
	defer os.Remove(tmpPath)

	// Extract the frame via ffmpeg.
	base64PNG, err := g.extractor.ExtractFrame(genCtx, videoPath, tmpPath, at, widthPx)
	if err != nil {
		return nil, fmt.Errorf("preview: extracting frame: %w", err)
	}

	return &FrameResult{
		Base64PNG: base64PNG,
		Timecode:  formatTimecode(at),
	}, nil
}

// formatTimecode formats a duration as HH:MM:SS.mmm.
func formatTimecode(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
