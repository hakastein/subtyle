package parser

import (
	"os"
	"testing"
)

func TestWriteRoundTrip(t *testing.T) {
	// Parse the original file
	original, err := ParseFile("testdata/sample.ass")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Write to a temp file
	tmpPath, err := WriteTempFile(original)
	if err != nil {
		t.Fatalf("WriteTempFile failed: %v", err)
	}
	defer os.Remove(tmpPath)

	// Parse the written file
	parsed, err := ParseFile(tmpPath)
	if err != nil {
		t.Fatalf("ParseFile of temp file failed: %v", err)
	}

	// Compare style counts
	if len(parsed.Styles) != len(original.Styles) {
		t.Errorf("style count: got %d, want %d", len(parsed.Styles), len(original.Styles))
	}

	// Compare each style by name
	origByName := make(map[string]SubtitleStyle)
	for _, s := range original.Styles {
		origByName[s.Name] = s
	}
	for _, s := range parsed.Styles {
		orig, ok := origByName[s.Name]
		if !ok {
			t.Errorf("unexpected style %q in written file", s.Name)
			continue
		}
		if s.FontName != orig.FontName {
			t.Errorf("style %q: FontName = %q, want %q", s.Name, s.FontName, orig.FontName)
		}
		if s.FontSize != orig.FontSize {
			t.Errorf("style %q: FontSize = %v, want %v", s.Name, s.FontSize, orig.FontSize)
		}
		if s.Bold != orig.Bold {
			t.Errorf("style %q: Bold = %v, want %v", s.Name, s.Bold, orig.Bold)
		}
		if s.Alignment != orig.Alignment {
			t.Errorf("style %q: Alignment = %d, want %d", s.Name, s.Alignment, orig.Alignment)
		}
		if s.PrimaryColour != orig.PrimaryColour {
			t.Errorf("style %q: PrimaryColour = %+v, want %+v", s.Name, s.PrimaryColour, orig.PrimaryColour)
		}
	}
}

func TestWriteModifiedRoundTrip(t *testing.T) {
	// Parse original
	sf, err := ParseFile("testdata/sample.ass")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Modify styles
	for i := range sf.Styles {
		if sf.Styles[i].Name == "Default" {
			sf.Styles[i].FontName = "Times New Roman"
			sf.Styles[i].FontSize = 64
			sf.Styles[i].Bold = false
			sf.Styles[i].PrimaryColour = Color{R: 255, G: 255, B: 0, A: 255} // yellow
		}
	}

	// Write to temp
	tmpPath, err := WriteTempFile(sf)
	if err != nil {
		t.Fatalf("WriteTempFile failed: %v", err)
	}
	defer os.Remove(tmpPath)

	// Parse back
	parsed, err := ParseFile(tmpPath)
	if err != nil {
		t.Fatalf("ParseFile of temp file failed: %v", err)
	}

	// Verify changes
	var found bool
	for _, s := range parsed.Styles {
		if s.Name != "Default" {
			continue
		}
		found = true
		if s.FontName != "Times New Roman" {
			t.Errorf("FontName = %q, want %q", s.FontName, "Times New Roman")
		}
		if s.FontSize != 64 {
			t.Errorf("FontSize = %v, want 64", s.FontSize)
		}
		if s.Bold {
			t.Error("Bold = true, want false")
		}
		wantColor := Color{R: 255, G: 255, B: 0, A: 255}
		if s.PrimaryColour != wantColor {
			t.Errorf("PrimaryColour = %+v, want %+v", s.PrimaryColour, wantColor)
		}
	}
	if !found {
		t.Error("Default style not found in parsed output")
	}
}

func TestWriteFile(t *testing.T) {
	sf, err := ParseFile("testdata/sample.ass")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Write to explicit temp path
	f, err := os.CreateTemp("", "write_test_*.ass")
	if err != nil {
		t.Fatalf("creating temp file failed: %v", err)
	}
	f.Close()
	defer os.Remove(f.Name())

	if err := WriteFile(f.Name(), sf); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify file exists and is parseable
	parsed, err := ParseFile(f.Name())
	if err != nil {
		t.Fatalf("ParseFile after WriteFile failed: %v", err)
	}
	if len(parsed.Styles) != len(sf.Styles) {
		t.Errorf("style count: got %d, want %d", len(parsed.Styles), len(sf.Styles))
	}
}
