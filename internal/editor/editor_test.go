package editor

import (
	"testing"

	"subtitles-editor/internal/parser"
)

func defaultStyle(name string) parser.SubtitleStyle {
	return parser.SubtitleStyle{
		Name:     name,
		FontName: "Arial",
		FontSize: 20.0,
		Bold:     false,
		Alignment: 2,
		PrimaryColour: parser.Color{R: 255, G: 255, B: 255, A: 255},
	}
}

func TestApplyChange_FontSize(t *testing.T) {
	s := defaultStyle("Default")
	result, err := ApplyChange(s, "fontSize", float64(36))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FontSize != 36.0 {
		t.Errorf("expected fontSize 36, got %v", result.FontSize)
	}
}

func TestApplyChange_Bold(t *testing.T) {
	s := defaultStyle("Default")
	result, err := ApplyChange(s, "bold", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Bold {
		t.Error("expected bold to be true")
	}
}

func TestApplyChange_PrimaryColour(t *testing.T) {
	s := defaultStyle("Default")
	color := parser.Color{R: 255, G: 0, B: 0, A: 255}
	result, err := ApplyChange(s, "primaryColour", color)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PrimaryColour != color {
		t.Errorf("expected primaryColour %v, got %v", color, result.PrimaryColour)
	}
}

func TestApplyChange_PrimaryColour_Map(t *testing.T) {
	s := defaultStyle("Default")
	colorMap := map[string]interface{}{
		"r": float64(0),
		"g": float64(255),
		"b": float64(0),
		"a": float64(255),
	}
	result, err := ApplyChange(s, "primaryColour", colorMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := parser.Color{R: 0, G: 255, B: 0, A: 255}
	if result.PrimaryColour != expected {
		t.Errorf("expected primaryColour %v, got %v", expected, result.PrimaryColour)
	}
}

func TestApplyChange_FontName(t *testing.T) {
	s := defaultStyle("Default")
	result, err := ApplyChange(s, "fontName", "Helvetica")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FontName != "Helvetica" {
		t.Errorf("expected fontName Helvetica, got %v", result.FontName)
	}
}

func TestApplyChange_Alignment(t *testing.T) {
	s := defaultStyle("Default")
	result, err := ApplyChange(s, "alignment", float64(5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Alignment != 5 {
		t.Errorf("expected alignment 5, got %v", result.Alignment)
	}
}

func TestApplyChange_UnknownField(t *testing.T) {
	s := defaultStyle("Default")
	_, err := ApplyChange(s, "nonExistentField", "value")
	if err == nil {
		t.Error("expected error for unknown field, got nil")
	}
}

func TestApplyBatch(t *testing.T) {
	styles := []parser.SubtitleStyle{
		defaultStyle("Default"),
		defaultStyle("Title"),
	}

	changes := []StyleChange{
		{StyleName: "Default", Field: "fontSize", Value: float64(24)},
		{StyleName: "Title", Field: "bold", Value: true},
	}

	result, err := ApplyBatch(styles, changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result[0].FontSize != 24 {
		t.Errorf("expected Default fontSize 24, got %v", result[0].FontSize)
	}
	if !result[1].Bold {
		t.Error("expected Title bold to be true")
	}
}
