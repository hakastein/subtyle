package parser

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	astisub "github.com/asticode/go-astisub"
)

// ParseFile parses an ASS/SSA subtitle file from disk.
func ParseFile(path string) (*SubtitleFile, error) {
	subs, err := astisub.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("parser: opening %q failed: %w", path, err)
	}

	sf, err := mapSubtitles(subs)
	if err != nil {
		return nil, err
	}
	sf.Path = path
	sf.ID = filepath.Base(path)
	sf.Source = "external"
	ensureNonNilSlices(sf)
	return sf, nil
}

// ensureNonNilSlices guarantees Styles and Events are empty slices, not nil,
// so that JSON serialization produces [] instead of null (frontend expects arrays).
func ensureNonNilSlices(sf *SubtitleFile) {
	if sf.Styles == nil {
		sf.Styles = []SubtitleStyle{}
	}
	if sf.Events == nil {
		sf.Events = []SubtitleEvent{}
	}
}

// ParseBytes parses an ASS/SSA subtitle file from a byte slice.
// id is used as the SubtitleFile.ID.
func ParseBytes(data []byte, id string) (*SubtitleFile, error) {
	subs, err := astisub.ReadFromSSA(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parser: parsing bytes failed: %w", err)
	}

	sf, err := mapSubtitles(subs)
	if err != nil {
		return nil, err
	}
	sf.ID = id
	sf.Source = "embedded"
	ensureNonNilSlices(sf)
	return sf, nil
}

// mapSubtitles converts go-astisub Subtitles to our domain SubtitleFile.
func mapSubtitles(subs *astisub.Subtitles) (*SubtitleFile, error) {
	sf := &SubtitleFile{}

	// Map styles — go-astisub stores them in a map; sort for determinism isn't
	// required here, but we keep insertion order by iterating through items to
	// collect style names in encounter order.
	styleOrder := collectStyleOrder(subs)
	for _, name := range styleOrder {
		st, ok := subs.Styles[name]
		if !ok {
			continue
		}
		sf.Styles = append(sf.Styles, mapStyle(name, st))
	}

	// Map events (items)
	for _, item := range subs.Items {
		event := mapEvent(item)
		sf.Events = append(sf.Events, event)
	}

	return sf, nil
}

// collectStyleOrder returns style names in the order they first appear in items,
// followed by any remaining styles not referenced by items.
func collectStyleOrder(subs *astisub.Subtitles) []string {
	seen := make(map[string]bool)
	var order []string

	for _, item := range subs.Items {
		if item.Style != nil && !seen[item.Style.ID] {
			seen[item.Style.ID] = true
			order = append(order, item.Style.ID)
		}
	}

	// Add remaining styles (not referenced in events)
	for name := range subs.Styles {
		if !seen[name] {
			seen[name] = true
			order = append(order, name)
		}
	}

	return order
}

// mapStyle converts a go-astisub Style to our SubtitleStyle.
func mapStyle(name string, st *astisub.Style) SubtitleStyle {
	s := SubtitleStyle{Name: name}

	if st.InlineStyle == nil {
		return s
	}
	a := st.InlineStyle

	s.FontName = a.SSAFontName
	if a.SSAFontSize != nil {
		s.FontSize = *a.SSAFontSize
	}
	if a.SSABold != nil {
		s.Bold = *a.SSABold
	}
	if a.SSAItalic != nil {
		s.Italic = *a.SSAItalic
	}
	if a.SSAUnderline != nil {
		s.Underline = *a.SSAUnderline
	}
	if a.SSAStrikeout != nil {
		s.Strikeout = *a.SSAStrikeout
	}
	if a.SSAPrimaryColour != nil {
		s.PrimaryColour = mapColor(a.SSAPrimaryColour)
	}
	if a.SSASecondaryColour != nil {
		s.SecondaryColour = mapColor(a.SSASecondaryColour)
	}
	if a.SSAOutlineColour != nil {
		s.OutlineColour = mapColor(a.SSAOutlineColour)
	}
	if a.SSABackColour != nil {
		s.BackColour = mapColor(a.SSABackColour)
	}
	if a.SSAOutline != nil {
		s.Outline = *a.SSAOutline
	}
	if a.SSAShadow != nil {
		s.Shadow = *a.SSAShadow
	}
	if a.SSAScaleX != nil {
		s.ScaleX = *a.SSAScaleX
	}
	if a.SSAScaleY != nil {
		s.ScaleY = *a.SSAScaleY
	}
	if a.SSASpacing != nil {
		s.Spacing = *a.SSASpacing
	}
	if a.SSAAngle != nil {
		s.Angle = *a.SSAAngle
	}
	if a.SSAAlignment != nil {
		s.Alignment = *a.SSAAlignment
	}
	if a.SSAMarginLeft != nil {
		s.MarginL = *a.SSAMarginLeft
	}
	if a.SSAMarginRight != nil {
		s.MarginR = *a.SSAMarginRight
	}
	if a.SSAMarginVertical != nil {
		s.MarginV = *a.SSAMarginVertical
	}
	return s
}

// mapColor converts a go-astisub Color to our Color.
// go-astisub Color: Alpha field uses ASS convention (0=opaque, 255=transparent).
// Our Color: A field uses standard convention (255=opaque, 0=transparent).
func mapColor(c *astisub.Color) Color {
	if c == nil {
		return Color{}
	}
	return Color{
		R: c.Red,
		G: c.Green,
		B: c.Blue,
		A: 255 - c.Alpha,
	}
}

// mapEvent converts a go-astisub Item to our SubtitleEvent.
func mapEvent(item *astisub.Item) SubtitleEvent {
	e := SubtitleEvent{
		StartTime: item.StartAt,
		EndTime:   item.EndAt,
	}

	if item.Style != nil {
		e.StyleName = item.Style.ID
	}

	// Collect text from all lines and line items
	var lines []string
	for _, line := range item.Lines {
		var parts []string
		for _, li := range line.Items {
			parts = append(parts, li.Text)
		}
		lines = append(lines, strings.Join(parts, ""))
	}
	e.Text = strings.Join(lines, "\n")

	return e
}

// readFileBytes is a helper used by tests.
func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}
