package i18n

import "testing"

func TestNormalizeBCP47(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ru-RU", "ru"},
		{"en-US", "en"},
		{"RU", "ru"},
		{"ru_RU.UTF-8", "ru"},
		{"en_US.UTF-8", "en"},
		{"", "en"},
		{"de-DE", "en"}, // unsupported → default
		{"ru", "ru"},
	}

	for _, tt := range tests {
		got := normalizeBCP47(tt.input)
		if got != tt.want {
			t.Errorf("normalizeBCP47(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
