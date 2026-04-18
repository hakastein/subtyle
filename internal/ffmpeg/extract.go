package ffmpeg

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"subtitles-editor/internal/scan"
)

// Extractor uses an ffmpeg binary to extract frames and subtitle tracks.
type Extractor struct {
	binPath string
}

// NewExtractor creates an Extractor using the given ffmpeg binary path.
func NewExtractor(binPath string) *Extractor {
	return &Extractor{binPath: binPath}
}

// ExtractFrame renders a video frame at the given time with subtitles burned in,
// returning the frame as a base64-encoded PNG string.
func (e *Extractor) ExtractFrame(ctx context.Context, videoPath, subPath string, at time.Duration) (string, error) {
	args := buildFrameArgs(videoPath, subPath, at)
	cmd := exec.CommandContext(ctx, e.binPath, args...)
	hideWindow(cmd)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()

	// Always log for debugging
	fmt.Fprintf(os.Stderr, "[ffmpeg] cmd: %s %s\n", e.binPath, strings.Join(args, " "))
	if stderrStr := stderr.String(); stderrStr != "" {
		// Only log last 500 chars of stderr to avoid flooding
		if len(stderrStr) > 500 {
			stderrStr = "..." + stderrStr[len(stderrStr)-500:]
		}
		fmt.Fprintf(os.Stderr, "[ffmpeg stderr] %s\n", stderrStr)
	}

	if err != nil {
		return "", fmt.Errorf("ffmpeg extract frame: %w\nstderr: %s\nargs: %v", err, stderr.String(), args)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// LastFrameCommand returns the ffmpeg command that would be built for frame extraction (for debugging).
func LastFrameCommand(binPath, videoPath, subPath string, at time.Duration) string {
	args := buildFrameArgs(videoPath, subPath, at)
	return binPath + " " + strings.Join(args, " ")
}

// ListTracks returns all ASS/SSA subtitle tracks embedded in the video.
func (e *Extractor) ListTracks(ctx context.Context, videoPath string) ([]scan.TrackInfo, error) {
	args := buildListTracksArgs(videoPath)
	cmd := exec.CommandContext(ctx, e.binPath, args...)
	hideWindow(cmd)
	// ffmpeg prints stream info to stderr
	stderr, err := cmd.CombinedOutput()
	// ffmpeg returns non-zero when given no output; that's expected
	if err != nil && len(stderr) == 0 {
		return nil, fmt.Errorf("ffmpeg list tracks: %w", err)
	}
	return parseTrackList(string(stderr)), nil
}

// ExtractTrack extracts a subtitle track by its subtitle-relative index from the video
// and writes it to outputPath.
func (e *Extractor) ExtractTrack(ctx context.Context, videoPath string, trackIndex int, outputPath string) error {
	args := buildExtractTrackArgs(videoPath, trackIndex, outputPath)
	cmd := exec.CommandContext(ctx, e.binPath, args...)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg extract track: %w\n%s", err, out)
	}
	return nil
}

// VideoDuration returns the total duration of the video.
func (e *Extractor) VideoDuration(ctx context.Context, videoPath string) (time.Duration, error) {
	args := buildListTracksArgs(videoPath)
	cmd := exec.CommandContext(ctx, e.binPath, args...)
	hideWindow(cmd)
	stderr, _ := cmd.CombinedOutput()
	return parseDuration(string(stderr))
}

// DiagInfo holds diagnostic information about the ffmpeg binary.
type DiagInfo struct {
	Path              string `json:"path"`
	Version           string `json:"version"`
	HasSubtitlesFilter bool   `json:"hasSubtitlesFilter"`
	HasLibass         bool   `json:"hasLibass"`
	Filters           string `json:"filters"` // raw output for debug
}

// Diagnose checks ffmpeg capabilities and returns diagnostic info.
func (e *Extractor) Diagnose(ctx context.Context) *DiagInfo {
	info := &DiagInfo{Path: e.binPath}

	// Get version
	cmd := exec.CommandContext(ctx, e.binPath, "-version")
	hideWindow(cmd)
	if out, err := cmd.Output(); err == nil {
		lines := strings.SplitN(string(out), "\n", 2)
		if len(lines) > 0 {
			info.Version = strings.TrimSpace(lines[0])
		}
	}

	// Check filters
	cmd = exec.CommandContext(ctx, e.binPath, "-filters")
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err == nil {
		output := string(out)
		info.Filters = output
		info.HasSubtitlesFilter = strings.Contains(output, "subtitles")
		info.HasLibass = strings.Contains(output, "libass")
	}

	return info
}

// buildFrameArgs constructs ffmpeg arguments to render a single frame at `at`
// with subtitles burned in at the source resolution.
// Uses double-seek (fast -ss before -i, fine -ss after -i) to avoid decoding
// the entire file from the start.
func buildFrameArgs(videoPath, subPath string, at time.Duration) []string {
	totalSec := at.Seconds()
	fastSec := totalSec - 10.0
	if fastSec < 0 {
		fastSec = 0
	}
	fineSec := totalSec - fastSec

	vf := fmt.Sprintf("subtitles='%s'", escapeFilterPath(subPath))
	return []string{
		"-ss", fmt.Sprintf("%.3f", fastSec),
		"-i", videoPath,
		"-ss", fmt.Sprintf("%.3f", fineSec),
		"-vf", vf,
		"-frames:v", "1",
		"-f", "image2",
		"pipe:1",
	}
}

// buildBaseFrameArgs constructs arguments for extracting a base frame (no subtitles)
// at source resolution, written to outputPath. Uses double-seek.
func buildBaseFrameArgs(videoPath string, at time.Duration, outputPath string) []string {
	totalSec := at.Seconds()
	fastSec := totalSec - 10.0
	if fastSec < 0 {
		fastSec = 0
	}
	fineSec := totalSec - fastSec

	return []string{
		"-ss", fmt.Sprintf("%.3f", fastSec),
		"-i", videoPath,
		"-ss", fmt.Sprintf("%.3f", fineSec),
		"-frames:v", "1",
		"-update", "1",
		"-y",
		outputPath,
	}
}

// buildOverlayArgs constructs arguments for applying subtitles to a pre-rendered
// base frame (PNG). Uses -itsoffset so the subtitles filter sees the frame at
// time `at` rather than 0.
func buildOverlayArgs(basePath, subPath string, at time.Duration) []string {
	vf := fmt.Sprintf("subtitles='%s'", escapeFilterPath(subPath))
	return []string{
		"-itsoffset", fmt.Sprintf("%.3f", at.Seconds()),
		"-loop", "1",
		"-i", basePath,
		"-vf", vf,
		"-t", "1",
		"-frames:v", "1",
		"-f", "image2",
		"pipe:1",
	}
}

// ExtractBaseFrame renders a subtitle-less frame to disk.
func (e *Extractor) ExtractBaseFrame(ctx context.Context, videoPath string, at time.Duration, outputPath string) error {
	args := buildBaseFrameArgs(videoPath, at, outputPath)
	cmd := exec.CommandContext(ctx, e.binPath, args...)
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extract base frame: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

// OverlayFrame applies subtitles to an existing base frame PNG and returns
// the result as a base64-encoded PNG.
func (e *Extractor) OverlayFrame(ctx context.Context, basePath, subPath string, at time.Duration) (string, error) {
	args := buildOverlayArgs(basePath, subPath, at)
	cmd := exec.CommandContext(ctx, e.binPath, args...)
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffmpeg overlay frame: %w\nstderr: %s", err, stderr.String())
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// escapeFilterPath escapes a file path for use in ffmpeg filter expressions.
// On Windows, backslashes and colons need special escaping.
func escapeFilterPath(path string) string {
	// Replace backslashes with forward slashes (ffmpeg accepts both on Windows)
	path = strings.ReplaceAll(path, "\\", "/")
	// Escape colons (C: drive letters) and single quotes for ffmpeg filter syntax
	path = strings.ReplaceAll(path, ":", "\\:")
	path = strings.ReplaceAll(path, "'", "'\\''")
	return path
}

// buildListTracksArgs constructs the ffmpeg argument list for listing streams.
func buildListTracksArgs(videoPath string) []string {
	return []string{"-i", videoPath}
}

// buildExtractTrackArgs constructs the ffmpeg argument list for extracting a subtitle track.
func buildExtractTrackArgs(videoPath string, trackIndex int, outputPath string) []string {
	return []string{
		"-i", videoPath,
		"-map", fmt.Sprintf("0:s:%d", trackIndex),
		"-c:s", "copy",
		outputPath,
	}
}

// streamInfo holds temporary parsing state for a single stream.
type streamInfo struct {
	streamLine string
	language   string
	codec      string
	title      string
	subtitleN  int // subtitle-relative index if this is a subtitle stream
}

var (
	reStream = regexp.MustCompile(`Stream #\d+:\d+(?:\((\w+)\))?: Subtitle: (\w+)`)
	reTitle  = regexp.MustCompile(`title\s*:\s*(.+)$`)
	reDur    = regexp.MustCompile(`Duration:\s*(\d{2}):(\d{2}):(\d{2})\.(\d+)`)
)

// parseTrackList parses ffmpeg stderr output and returns ASS/SSA subtitle tracks.
// Track Index is the subtitle-stream-relative index counting ALL subtitle streams
// (needed for -map 0:s:N), but only ASS/SSA tracks are included in the result.
func parseTrackList(stderr string) []scan.TrackInfo {
	// First pass: find ALL subtitle streams to get correct subtitle-relative indices
	type subStream struct {
		lang  string
		codec string
		title string
		subIdx int // subtitle-relative index (counting all sub streams)
	}

	var allSubs []subStream
	lines := strings.Split(stderr, "\n")

	// Match any subtitle stream (not just ASS)
	reAnySub := regexp.MustCompile(`Stream #\d+:\d+(?:\((\w+)\))?: Subtitle: (\w+)`)

	for i, line := range lines {
		m := reAnySub.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lang := m[1]
		codec := strings.ToLower(m[2])

		title := ""
		for j := i + 1; j < len(lines) && j <= i+3; j++ {
			tm := reTitle.FindStringSubmatch(strings.TrimSpace(lines[j]))
			if tm != nil {
				title = strings.TrimSpace(tm[1])
				break
			}
			if strings.Contains(lines[j], "Stream #") {
				break
			}
		}

		allSubs = append(allSubs, subStream{
			lang:   lang,
			codec:  codec,
			title:  title,
			subIdx: len(allSubs), // 0-based across ALL subtitle streams
		})
	}

	// Second pass: filter to ASS/SSA only, keeping correct subtitle-relative index
	var tracks []scan.TrackInfo
	for _, s := range allSubs {
		if s.codec == "ass" || s.codec == "ssa" {
			tracks = append(tracks, scan.TrackInfo{
				Index:    s.subIdx,
				Language: s.lang,
				Title:    s.title,
			})
		}
	}
	return tracks
}

// parseDuration extracts the video duration from ffmpeg stderr output.
func parseDuration(stderr string) (time.Duration, error) {
	m := reDur.FindStringSubmatch(stderr)
	if m == nil {
		return 0, fmt.Errorf("duration not found in ffmpeg output")
	}

	hours, _ := strconv.Atoi(m[1])
	minutes, _ := strconv.Atoi(m[2])
	seconds, _ := strconv.Atoi(m[3])

	// Fractional seconds: the number of digits matters
	fracStr := m[4]
	// Pad or truncate to milliseconds (3 digits)
	for len(fracStr) < 3 {
		fracStr += "0"
	}
	if len(fracStr) > 3 {
		fracStr = fracStr[:3]
	}
	millis, _ := strconv.Atoi(fracStr)

	d := time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(millis)*time.Millisecond

	return d, nil
}

// formatDuration formats a time.Duration as HH:MM:SS.mmm for ffmpeg -ss argument.
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
