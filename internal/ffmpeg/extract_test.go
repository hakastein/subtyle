package ffmpeg

import (
	"strings"
	"testing"
	"time"
)

func TestBuildFrameArgs(t *testing.T) {
	args := buildFrameArgs("/path/to/video.mkv", "/path/to/sub.ass", 90*time.Second, 960)

	required := []string{"-i", "/path/to/video.mkv", "-vf", "-frames:v", "1", "-f", "image2", "pipe:1"}
	for _, req := range required {
		found := false
		for _, arg := range args {
			if arg == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected arg %q in frame args: %v", req, args)
		}
	}
}

func TestEscapeFilterPath(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"unix path", "/tmp/sub.ass", "/tmp/sub.ass"},
		{"windows path", `C:\Users\test\sub.ass`, "C\\:/Users/test/sub.ass"},
		{"windows with spaces", `C:\Users\my user\sub file.ass`, "C\\:/Users/my user/sub file.ass"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeFilterPath(tt.input)
			if got != tt.expect {
				t.Errorf("escapeFilterPath(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestBuildFrameArgsWindowsPath(t *testing.T) {
	args := buildFrameArgs(`C:\Videos\ep01.mkv`, `C:\Temp\sub.ass`, 5*time.Second, 1280)
	var vfArg string
	for i, a := range args {
		if a == "-vf" && i+1 < len(args) {
			vfArg = args[i+1]
		}
	}
	if vfArg == "" {
		t.Fatal("no -vf argument found")
	}
	if !strings.Contains(vfArg, "C\\:") {
		t.Errorf("-vf arg should escape colon: %q", vfArg)
	}
	if !strings.Contains(vfArg, "scale=1280:-1") {
		t.Errorf("-vf arg should include scale: %q", vfArg)
	}
}

func TestBuildListTracksArgs(t *testing.T) {
	args := buildListTracksArgs("/path/to/video.mkv")

	required := []string{"-i", "/path/to/video.mkv"}
	for _, req := range required {
		found := false
		for _, arg := range args {
			if arg == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected arg %q in list tracks args: %v", req, args)
		}
	}
}

func TestBuildExtractTrackArgs(t *testing.T) {
	args := buildExtractTrackArgs("/path/to/video.mkv", 2, "/output/track.ass")

	required := []string{"-i", "/path/to/video.mkv", "-map", "0:s:2", "-c:s", "copy", "/output/track.ass"}
	for _, req := range required {
		found := false
		for _, arg := range args {
			if arg == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected arg %q in extract track args: %v", req, args)
		}
	}
}

const sampleFFmpegStderr = `ffmpeg version n7.1 Copyright (c) 2000-2024 the FFmpeg developers
Input #0, matroska,webm, from 'video.mkv':
  Metadata:
    title           : Vinland Saga
  Duration: 00:23:40.10, start: 0.000000, bitrate: 8000 kb/s
    Stream #0:0: Video: hevc, yuv420p, 1920x1080, 23.98 fps
    Stream #0:1: Audio: flac, 48000 Hz, stereo
    Stream #0:2(rus): Subtitle: ass
      Metadata:
        title           : Russian [Anku]
    Stream #0:3(eng): Subtitle: srt
    Stream #0:4(jpn): Subtitle: ass
      Metadata:
        title           : Japanese
`

func TestParseTrackList(t *testing.T) {
	tracks := parseTrackList(sampleFFmpegStderr)

	// Should find 2 ASS tracks, skip SRT
	if len(tracks) != 2 {
		t.Fatalf("expected 2 ASS tracks, got %d: %+v", len(tracks), tracks)
	}

	// First track: index 0, language rus, title "Russian [Anku]"
	if tracks[0].Index != 0 {
		t.Errorf("expected track[0].Index = 0, got %d", tracks[0].Index)
	}
	if tracks[0].Language != "rus" {
		t.Errorf("expected track[0].Language = 'rus', got %s", tracks[0].Language)
	}
	if tracks[0].Title != "Russian [Anku]" {
		t.Errorf("expected track[0].Title = 'Russian [Anku]', got %s", tracks[0].Title)
	}

	// Second track: index 2 (subtitle-relative, SRT at index 1 is skipped from result but counted)
	if tracks[1].Index != 2 {
		t.Errorf("expected track[1].Index = 2, got %d", tracks[1].Index)
	}
	if tracks[1].Language != "jpn" {
		t.Errorf("expected track[1].Language = 'jpn', got %s", tracks[1].Language)
	}
	if tracks[1].Title != "Japanese" {
		t.Errorf("expected track[1].Title = 'Japanese', got %s", tracks[1].Title)
	}
}

func TestParseDuration(t *testing.T) {
	d, err := parseDuration("Duration: 00:23:40.10, start: 0.000000, bitrate: 8000 kb/s")
	if err != nil {
		t.Fatalf("parseDuration error: %v", err)
	}

	expected := 23*time.Minute + 40*time.Second + 100*time.Millisecond
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestParseDuration_FromStderr(t *testing.T) {
	d, err := parseDuration(sampleFFmpegStderr)
	if err != nil {
		t.Fatalf("parseDuration from stderr error: %v", err)
	}

	expected := 23*time.Minute + 40*time.Second + 100*time.Millisecond
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestBuildFrameArgs_DoubleSeekAndScale(t *testing.T) {
	args := buildFrameArgs("/videos/ep01.mkv", "/tmp/sub.ass", 30*time.Second, 960)

	// Expect two -ss flags: fast before -i, fine after -i
	ssCount := 0
	var ssFast, ssFine string
	iIdx := -1
	for i, a := range args {
		if a == "-ss" && i+1 < len(args) {
			ssCount++
			if iIdx == -1 {
				ssFast = args[i+1]
			} else {
				ssFine = args[i+1]
			}
		}
		if a == "-i" {
			iIdx = i
		}
	}
	if ssCount != 2 {
		t.Fatalf("expected 2 -ss flags, got %d: %v", ssCount, args)
	}
	if iIdx < 0 {
		t.Fatal("no -i flag found")
	}

	// 30s - 10s = 20s fast seek, then 10s fine seek
	if ssFast != "20.000" {
		t.Errorf("fast seek = %q, want 20.000", ssFast)
	}
	if ssFine != "10.000" {
		t.Errorf("fine seek = %q, want 10.000", ssFine)
	}

	// Verify -vf has scale + subtitles
	var vfArg string
	for i, a := range args {
		if a == "-vf" && i+1 < len(args) {
			vfArg = args[i+1]
		}
	}
	if !strings.Contains(vfArg, "scale=960:-1") {
		t.Errorf("-vf missing scale: %q", vfArg)
	}
	if !strings.Contains(vfArg, "subtitles=") {
		t.Errorf("-vf missing subtitles: %q", vfArg)
	}
}

func TestBuildFrameArgs_SeekBelowThreshold(t *testing.T) {
	// For a target before 10s, fast seek is 0
	args := buildFrameArgs("/videos/ep01.mkv", "/tmp/sub.ass", 5*time.Second, 960)
	var ssFast, ssFine string
	first := true
	for i, a := range args {
		if a == "-ss" && i+1 < len(args) {
			if first {
				ssFast = args[i+1]
				first = false
			} else {
				ssFine = args[i+1]
			}
		}
	}
	if ssFast != "0.000" {
		t.Errorf("fast seek for 5s = %q, want 0.000", ssFast)
	}
	if ssFine != "5.000" {
		t.Errorf("fine seek for 5s = %q, want 5.000", ssFine)
	}
}

func TestBuildBaseFrameArgs(t *testing.T) {
	args := buildBaseFrameArgs("/videos/ep01.mkv", 30*time.Second, 960, "/tmp/base.png")

	// No -vf subtitles=, just scale
	var vfArg string
	for i, a := range args {
		if a == "-vf" && i+1 < len(args) {
			vfArg = args[i+1]
		}
	}
	if vfArg != "scale=960:-1" {
		t.Errorf("-vf = %q, want scale=960:-1", vfArg)
	}

	// Last arg should be the output path
	last := args[len(args)-1]
	if last != "/tmp/base.png" {
		t.Errorf("last arg = %q, want output path", last)
	}

	// Should use double-seek
	ssCount := 0
	for _, a := range args {
		if a == "-ss" {
			ssCount++
		}
	}
	if ssCount != 2 {
		t.Errorf("expected 2 -ss flags, got %d", ssCount)
	}
}

func TestBuildOverlayArgs(t *testing.T) {
	args := buildOverlayArgs("/tmp/base.png", "/tmp/sub.ass", 21*time.Second)

	var offset string
	for i, a := range args {
		if a == "-itsoffset" && i+1 < len(args) {
			offset = args[i+1]
		}
	}
	if offset != "21.000" {
		t.Errorf("-itsoffset = %q, want 21.000", offset)
	}

	hasLoop := false
	for i, a := range args {
		if a == "-loop" && i+1 < len(args) && args[i+1] == "1" {
			hasLoop = true
		}
	}
	if !hasLoop {
		t.Error("expected -loop 1")
	}

	var vfArg string
	for i, a := range args {
		if a == "-vf" && i+1 < len(args) {
			vfArg = args[i+1]
		}
	}
	if !strings.Contains(vfArg, "subtitles=") {
		t.Errorf("-vf missing subtitles: %q", vfArg)
	}
}
