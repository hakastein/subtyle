//go:build !windows

package i18n

import (
	"os"
	"strings"
)

// DetectLocale checks environment variables and returns "ru" or "en".
func DetectLocale() string {
	for _, envVar := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		val := os.Getenv(envVar)
		if val == "" {
			continue
		}
		if loc := normalizeBCP47(val); loc != "" {
			return loc
		}
	}
	return "en"
}

// normalizeBCP47 takes a locale string (e.g. "ru-RU", "ru_RU.UTF-8", "RU")
// and returns "ru" or "en".
func normalizeBCP47(s string) string {
	if s == "" {
		return "en"
	}
	lower := strings.ToLower(s)
	for _, sep := range []string{"-", "_", "."} {
		if idx := strings.Index(lower, sep); idx >= 0 {
			lower = lower[:idx]
		}
	}
	if lower == "ru" {
		return "ru"
	}
	return "en"
}
