package editor

import (
	"fmt"

	"subtitles-editor/internal/parser"
)

// StyleChange represents a single field change for a named style.
type StyleChange struct {
	StyleName string      `json:"styleName"`
	Field     string      `json:"field"`
	Value     interface{} `json:"value"`
}

// ApplyChange applies a field-level change to a SubtitleStyle and returns the updated style.
func ApplyChange(style parser.SubtitleStyle, field string, value interface{}) (parser.SubtitleStyle, error) {
	switch field {
	case "fontName":
		v, err := toString(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.FontName = v
	case "fontSize":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.FontSize = v
	case "bold":
		v, err := toBool(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.Bold = v
	case "italic":
		v, err := toBool(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.Italic = v
	case "underline":
		v, err := toBool(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.Underline = v
	case "strikeout":
		v, err := toBool(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.Strikeout = v
	case "primaryColour":
		v, err := toColor(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.PrimaryColour = v
	case "secondaryColour":
		v, err := toColor(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.SecondaryColour = v
	case "outlineColour":
		v, err := toColor(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.OutlineColour = v
	case "backColour":
		v, err := toColor(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.BackColour = v
	case "outline":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.Outline = v
	case "shadow":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.Shadow = v
	case "scaleX":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.ScaleX = v
	case "scaleY":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.ScaleY = v
	case "spacing":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.Spacing = v
	case "angle":
		v, err := toFloat64(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.Angle = v
	case "alignment":
		v, err := toInt(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.Alignment = v
	case "marginL":
		v, err := toInt(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.MarginL = v
	case "marginR":
		v, err := toInt(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.MarginR = v
	case "marginV":
		v, err := toInt(value)
		if err != nil {
			return style, fmt.Errorf("editor: field %q: %w", field, err)
		}
		style.MarginV = v
	default:
		return style, fmt.Errorf("editor: unknown field %q", field)
	}
	return style, nil
}

// ApplyBatch applies multiple changes to a slice of styles, matching by StyleName.
func ApplyBatch(styles []parser.SubtitleStyle, changes []StyleChange) ([]parser.SubtitleStyle, error) {
	result := make([]parser.SubtitleStyle, len(styles))
	copy(result, styles)

	for _, change := range changes {
		for i, style := range result {
			if style.Name == change.StyleName {
				updated, err := ApplyChange(style, change.Field, change.Value)
				if err != nil {
					return nil, err
				}
				result[i] = updated
			}
		}
	}
	return result, nil
}

// toString converts a value to string.
func toString(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	}
	return "", fmt.Errorf("expected string, got %T", v)
}

// toFloat64 converts a value to float64 (handles JSON number types).
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	}
	return 0, fmt.Errorf("expected float64, got %T", v)
}

// toInt converts a value to int (handles JSON number types).
func toInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case float64:
		return int(val), nil
	case float32:
		return int(val), nil
	}
	return 0, fmt.Errorf("expected int, got %T", v)
}

// toBool converts a value to bool.
func toBool(v interface{}) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	}
	return false, fmt.Errorf("expected bool, got %T", v)
}

// toColor converts a value to parser.Color.
// Accepts parser.Color directly or map[string]interface{} (from JSON).
func toColor(v interface{}) (parser.Color, error) {
	switch val := v.(type) {
	case parser.Color:
		return val, nil
	case map[string]interface{}:
		r, err := toUint8FromMap(val, "r")
		if err != nil {
			return parser.Color{}, err
		}
		g, err := toUint8FromMap(val, "g")
		if err != nil {
			return parser.Color{}, err
		}
		b, err := toUint8FromMap(val, "b")
		if err != nil {
			return parser.Color{}, err
		}
		a, err := toUint8FromMap(val, "a")
		if err != nil {
			return parser.Color{}, err
		}
		return parser.Color{R: r, G: g, B: b, A: a}, nil
	}
	return parser.Color{}, fmt.Errorf("expected Color or map[string]interface{}, got %T", v)
}

// toUint8FromMap reads a key from a map and converts it to uint8.
func toUint8FromMap(m map[string]interface{}, key string) (uint8, error) {
	raw, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("missing key %q in color map", key)
	}
	switch val := raw.(type) {
	case float64:
		return uint8(val), nil
	case int:
		return uint8(val), nil
	}
	return 0, fmt.Errorf("key %q: expected number, got %T", key, raw)
}
