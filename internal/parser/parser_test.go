package parser

import (
	"testing"
	"time"
)

func TestParseFile(t *testing.T) {
	sf, err := ParseFile("testdata/sample.ass")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Check styles count
	if len(sf.Styles) != 2 {
		t.Fatalf("expected 2 styles, got %d", len(sf.Styles))
	}

	// Find Default and Signs styles
	var defaultStyle, signsStyle *SubtitleStyle
	for i := range sf.Styles {
		switch sf.Styles[i].Name {
		case "Default":
			defaultStyle = &sf.Styles[i]
		case "Signs":
			signsStyle = &sf.Styles[i]
		}
	}

	if defaultStyle == nil {
		t.Fatal("Default style not found")
	}
	if signsStyle == nil {
		t.Fatal("Signs style not found")
	}

	// Verify Default style
	t.Run("Default style", func(t *testing.T) {
		if defaultStyle.FontName != "Arial" {
			t.Errorf("FontName = %q, want %q", defaultStyle.FontName, "Arial")
		}
		if defaultStyle.FontSize != 48 {
			t.Errorf("FontSize = %v, want 48", defaultStyle.FontSize)
		}
		if !defaultStyle.Bold {
			t.Error("Bold = false, want true (ASS -1 means bold)")
		}
		if defaultStyle.Alignment != 2 {
			t.Errorf("Alignment = %d, want 2", defaultStyle.Alignment)
		}
		if defaultStyle.Outline != 2 {
			t.Errorf("Outline = %v, want 2", defaultStyle.Outline)
		}
		// White primary colour
		wantPrimary := Color{R: 255, G: 255, B: 255, A: 255}
		if defaultStyle.PrimaryColour != wantPrimary {
			t.Errorf("PrimaryColour = %+v, want %+v", defaultStyle.PrimaryColour, wantPrimary)
		}
	})

	// Verify Signs style
	t.Run("Signs style", func(t *testing.T) {
		if signsStyle.FontName != "Impact" {
			t.Errorf("FontName = %q, want %q", signsStyle.FontName, "Impact")
		}
		if signsStyle.FontSize != 36 {
			t.Errorf("FontSize = %v, want 36", signsStyle.FontSize)
		}
		if signsStyle.Alignment != 8 {
			t.Errorf("Alignment = %d, want 8", signsStyle.Alignment)
		}
	})

	// Check events count
	if len(sf.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(sf.Events))
	}

	// Verify events
	t.Run("events", func(t *testing.T) {
		e0 := sf.Events[0]
		if e0.StyleName != "Default" {
			t.Errorf("Events[0].StyleName = %q, want %q", e0.StyleName, "Default")
		}
		if e0.StartTime != 1*time.Second {
			t.Errorf("Events[0].StartTime = %v, want %v", e0.StartTime, 1*time.Second)
		}
		if e0.EndTime != 5*time.Second {
			t.Errorf("Events[0].EndTime = %v, want %v", e0.EndTime, 5*time.Second)
		}
		if e0.Text != "Hello world" {
			t.Errorf("Events[0].Text = %q, want %q", e0.Text, "Hello world")
		}

		e1 := sf.Events[1]
		if e1.StyleName != "Default" {
			t.Errorf("Events[1].StyleName = %q, want %q", e1.StyleName, "Default")
		}
		if e1.StartTime != 10*time.Second {
			t.Errorf("Events[1].StartTime = %v, want %v", e1.StartTime, 10*time.Second)
		}
		if e1.EndTime != 15*time.Second {
			t.Errorf("Events[1].EndTime = %v, want %v", e1.EndTime, 15*time.Second)
		}
		if e1.Text != "Second line" {
			t.Errorf("Events[1].Text = %q, want %q", e1.Text, "Second line")
		}

		e2 := sf.Events[2]
		if e2.StyleName != "Signs" {
			t.Errorf("Events[2].StyleName = %q, want %q", e2.StyleName, "Signs")
		}
		if e2.StartTime != 60*time.Second {
			t.Errorf("Events[2].StartTime = %v, want %v", e2.StartTime, 60*time.Second)
		}
		if e2.EndTime != 65*time.Second {
			t.Errorf("Events[2].EndTime = %v, want %v", e2.EndTime, 65*time.Second)
		}
		if e2.Text != "Sign text here" {
			t.Errorf("Events[2].Text = %q, want %q", e2.Text, "Sign text here")
		}
	})
}

func TestParseBytes(t *testing.T) {
	data, err := readFileBytes("testdata/sample.ass")
	if err != nil {
		t.Fatalf("reading test file failed: %v", err)
	}

	sf, err := ParseBytes(data, "test-id")
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	if sf.ID != "test-id" {
		t.Errorf("ID = %q, want %q", sf.ID, "test-id")
	}
	if len(sf.Styles) != 2 {
		t.Errorf("expected 2 styles, got %d", len(sf.Styles))
	}
	if len(sf.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(sf.Events))
	}
}
