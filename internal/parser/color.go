package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseASSColor parses an ASS color string in &HAABBGGRR format.
// ASS alpha: 00 = fully opaque, FF = fully transparent (inverted).
// Our Color.A: 255 = fully opaque, 0 = fully transparent.
func ParseASSColor(s string) (Color, error) {
	s = strings.TrimSpace(s)

	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, "&H") {
		return Color{}, fmt.Errorf("parser: invalid ASS color format %q: must start with &H", s)
	}

	hex := s[2:]
	if len(hex) != 8 {
		return Color{}, fmt.Errorf("parser: invalid ASS color format %q: expected 8 hex digits after &H", s)
	}

	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("parser: invalid ASS color format %q: %w", s, err)
	}

	assAlpha := uint8((v >> 24) & 0xff)
	b := uint8((v >> 16) & 0xff)
	g := uint8((v >> 8) & 0xff)
	r := uint8(v & 0xff)

	// Convert ASS alpha (0=opaque, 255=transparent) to standard (255=opaque, 0=transparent)
	a := 255 - assAlpha

	return Color{R: r, G: g, B: b, A: a}, nil
}

// FormatASSColor formats a Color to ASS &HAABBGGRR format.
// Our Color.A: 255 = fully opaque, 0 = fully transparent.
// ASS alpha: 00 = fully opaque, FF = fully transparent (inverted).
func FormatASSColor(c Color) string {
	assAlpha := 255 - c.A
	return fmt.Sprintf("&H%02X%02X%02X%02X", assAlpha, c.B, c.G, c.R)
}
