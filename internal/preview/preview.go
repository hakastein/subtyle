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

type FrameResult struct {
	Base64PNG string `json:"base64Png"`
	Timecode  string `json:"timecode"`
}

// Generator manages preview frame generation with cancellation and a
// base-frame disk cache for fast re-renders at the same timecode.
type Generator struct {
	extractor *ffmpeg.Extractor
	cache     *Cache
	mu        sync.Mutex
	cancelFn  context.CancelFunc
	genID     int64
}

// NewGenerator creates a Generator backed by the given Extractor and cache.
// Pass a nil cache to disable caching entirely.
func NewGenerator(extractor *ffmpeg.Extractor, cache *Cache) *Generator {
	return &Generator{extractor: extractor, cache: cache}
}

// GenerateFrame produces a frame with subtitles burned in. When the cache is
// enabled, the base frame (no subtitles) is cached, and only the overlay pass
// runs on subsequent style edits at the same timecode.
func (g *Generator) GenerateFrame(ctx context.Context, videoPath string, subFile *parser.SubtitleFile, at time.Duration) (*FrameResult, error) {
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
		if g.genID == myID {
			g.cancelFn = nil
		}
		g.mu.Unlock()
		cancel()
	}()

	tmpSubPath, err := parser.WriteTempFile(subFile)
	if err != nil {
		return nil, fmt.Errorf("preview: write temp subtitle: %w", err)
	}
	defer os.Remove(tmpSubPath)

	// If cache is disabled, fall back to single-pass rendering.
	if g.cache == nil {
		base64PNG, err := g.extractor.ExtractFrame(genCtx, videoPath, tmpSubPath, at)
		if err != nil {
			return nil, fmt.Errorf("preview: extract frame: %w", err)
		}
		return &FrameResult{Base64PNG: base64PNG, Timecode: formatTimecode(at)}, nil
	}

	// Two-pass with cache.
	key := g.cache.Key(videoPath, at)
	basePath := g.cache.Path(key)

	if !g.cache.Exists(key) {
		// Ensure cache dir exists before ffmpeg writes to it
		if err := g.cache.EnsureDir(); err != nil {
			return nil, fmt.Errorf("preview: ensure cache dir: %w", err)
		}
		if err := g.extractor.ExtractBaseFrame(genCtx, videoPath, at, basePath); err != nil {
			return nil, fmt.Errorf("preview: extract base frame: %w", err)
		}
		// Trigger LRU accounting without re-writing the same bytes
		g.cache.Touch(key)
	}

	base64PNG, err := g.extractor.OverlayFrame(genCtx, basePath, tmpSubPath, at)
	if err != nil {
		return nil, fmt.Errorf("preview: overlay frame: %w", err)
	}

	return &FrameResult{Base64PNG: base64PNG, Timecode: formatTimecode(at)}, nil
}

func formatTimecode(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
