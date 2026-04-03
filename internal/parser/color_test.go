package parser

import (
	"testing"
)

func TestParseASSColor(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Color
		wantErr bool
	}{
		{
			name:  "white opaque",
			input: "&H00FFFFFF",
			want:  Color{R: 255, G: 255, B: 255, A: 255},
		},
		{
			name:  "red opaque",
			input: "&H000000FF",
			want:  Color{R: 255, G: 0, B: 0, A: 255},
		},
		{
			name:  "blue opaque",
			input: "&H00FF0000",
			want:  Color{R: 0, G: 0, B: 255, A: 255},
		},
		{
			name:  "green half transparent",
			input: "&H8000FF00",
			want:  Color{R: 0, G: 255, B: 0, A: 127},
		},
		{
			name:  "fully transparent black",
			input: "&HFF000000",
			want:  Color{R: 0, G: 0, B: 0, A: 0},
		},
		{
			name:  "lowercase input",
			input: "&h00ffffff",
			want:  Color{R: 255, G: 255, B: 255, A: 255},
		},
		{
			name:    "missing prefix",
			input:   "00FFFFFF",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "&H00FF",
			wantErr: true,
		},
		{
			name:    "invalid hex chars",
			input:   "&H00GGFFFF",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseASSColor(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseASSColor(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseASSColor(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatASSColor(t *testing.T) {
	tests := []struct {
		name  string
		input Color
		want  string
	}{
		{
			name:  "white opaque",
			input: Color{R: 255, G: 255, B: 255, A: 255},
			want:  "&H00FFFFFF",
		},
		{
			name:  "red opaque",
			input: Color{R: 255, G: 0, B: 0, A: 255},
			want:  "&H000000FF",
		},
		{
			name:  "fully transparent black",
			input: Color{R: 0, G: 0, B: 0, A: 0},
			want:  "&HFF000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatASSColor(tt.input)
			if got != tt.want {
				t.Errorf("FormatASSColor(%+v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	colors := []Color{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 0, B: 0, A: 255},
		{R: 0, G: 0, B: 255, A: 255},
		{R: 0, G: 255, B: 0, A: 127},
		{R: 0, G: 0, B: 0, A: 0},
		{R: 128, G: 64, B: 32, A: 200},
	}

	for _, c := range colors {
		s := FormatASSColor(c)
		got, err := ParseASSColor(s)
		if err != nil {
			t.Errorf("ParseASSColor(FormatASSColor(%+v)) error: %v", c, err)
			continue
		}
		if got != c {
			t.Errorf("round-trip failed for %+v: got %+v (via %q)", c, got, s)
		}
	}
}
