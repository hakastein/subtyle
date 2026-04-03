package parser

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	astisub "github.com/asticode/go-astisub"
)

// WriteFile reads the original ASS file from sf.Path (to preserve metadata and
// events), updates styles with those from sf, and writes the result to path.
// If sf.Path is empty or the original file cannot be read, a new Subtitles
// object is built from sf directly.
func WriteFile(path string, sf *SubtitleFile) error {
	data, err := buildASSBytes(sf)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("parser: writing to %q failed: %w", path, err)
	}
	return nil
}

// WriteTempFile creates a temporary ASS file with the contents of sf.
// The caller is responsible for removing the file when done.
func WriteTempFile(sf *SubtitleFile) (string, error) {
	f, err := os.CreateTemp("", "subtitles_*.ass")
	if err != nil {
		return "", fmt.Errorf("parser: creating temp file failed: %w", err)
	}
	tmpPath := f.Name()
	f.Close()

	if err := WriteFile(tmpPath, sf); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

// buildASSBytes produces the ASS file content.
// It reads the original file (if sf.Path is set) to preserve metadata and
// events, then injects the styles from sf.
func buildASSBytes(sf *SubtitleFile) ([]byte, error) {
	// If we have an original file, read its raw content and replace the styles
	// section only. This preserves all original formatting and metadata.
	if sf.Path != "" {
		orig, err := os.ReadFile(sf.Path)
		if err == nil {
			return replaceStylesSection(orig, sf.Styles)
		}
	}

	// No original file: build from scratch using go-astisub for events/metadata,
	// then replace the styles section with our own output.
	var buf bytes.Buffer
	subs := buildSubtitles(sf)
	if err := subs.WriteToSSA(&buf); err != nil {
		return nil, fmt.Errorf("parser: writing SSA failed: %w", err)
	}
	return replaceStylesSection(buf.Bytes(), sf.Styles)
}

// replaceStylesSection replaces the [V4 Styles] / [V4+ Styles] block in the
// given ASS file content with freshly rendered styles from our domain types.
func replaceStylesSection(content []byte, styles []SubtitleStyle) ([]byte, error) {
	lines := strings.Split(string(content), "\n")

	var out []string
	inStyles := false
	stylesWritten := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect styles section header
		lowerTrimmed := strings.ToLower(trimmed)
		if lowerTrimmed == "[v4+ styles]" || lowerTrimmed == "[v4 styles]" || lowerTrimmed == "[v4 styles+]" {
			inStyles = true
			out = append(out, line)
			// Write our styles immediately after the section header
			out = append(out, renderStylesBlock(styles)...)
			stylesWritten = true
			continue
		}

		// Skip old style lines (Format: and Style:) while in the styles section
		if inStyles {
			if strings.HasPrefix(trimmed, "Format:") || strings.HasPrefix(trimmed, "Style:") {
				continue
			}
			// A new section starts or a non-style line
			if strings.HasPrefix(trimmed, "[") {
				inStyles = false
			}
		}

		out = append(out, line)
	}

	// If no styles section was found, append one before the Events section
	if !stylesWritten {
		var final []string
		for _, line := range out {
			trimmed := strings.TrimSpace(line)
			if strings.ToLower(trimmed) == "[events]" {
				final = append(final, "[V4+ Styles]")
				final = append(final, renderStylesBlock(styles)...)
				final = append(final, "")
			}
			final = append(final, line)
		}
		out = final
	}

	return []byte(strings.Join(out, "\n")), nil
}

// renderStylesBlock returns the Format and Style lines for an ASS styles block.
func renderStylesBlock(styles []SubtitleStyle) []string {
	var lines []string
	lines = append(lines, "Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding")
	for _, s := range styles {
		lines = append(lines, renderStyleLine(s))
	}
	return lines
}

// renderStyleLine renders a single ASS Style: line for the given SubtitleStyle.
// Uses -1 for true and 0 for false (ASS convention).
func renderStyleLine(s SubtitleStyle) string {
	bold := boolToASS(s.Bold)
	italic := boolToASS(s.Italic)
	underline := boolToASS(s.Underline)
	strikeout := boolToASS(s.Strikeout)

	return fmt.Sprintf(
		"Style: %s,%s,%.0f,%s,%s,%s,%s,%s,%s,%s,%s,%.0f,%.0f,%.4g,%.4g,1,%.4g,%.4g,%d,%d,%d,%d,1",
		s.Name,
		s.FontName,
		s.FontSize,
		FormatASSColor(s.PrimaryColour),
		FormatASSColor(s.SecondaryColour),
		FormatASSColor(s.OutlineColour),
		FormatASSColor(s.BackColour),
		bold,
		italic,
		underline,
		strikeout,
		s.ScaleX,
		s.ScaleY,
		s.Spacing,
		s.Angle,
		s.Outline,
		s.Shadow,
		s.Alignment,
		s.MarginL,
		s.MarginR,
		s.MarginV,
	)
}

// boolToASS converts a bool to ASS -1/0 format.
func boolToASS(b bool) string {
	if b {
		return "-1"
	}
	return "0"
}

// buildSubtitles creates a minimal astisub.Subtitles from sf when no original
// file is available.
func buildSubtitles(sf *SubtitleFile) *astisub.Subtitles {
	subs := astisub.NewSubtitles()
	subs.Metadata = &astisub.Metadata{
		SSAScriptType: "v4.00+",
	}

	for _, ev := range sf.Events {
		item := &astisub.Item{
			StartAt: ev.StartTime,
			EndAt:   ev.EndTime,
			Lines: []astisub.Line{
				{Items: []astisub.LineItem{{Text: ev.Text}}},
			},
		}
		subs.Items = append(subs.Items, item)
	}

	return subs
}
