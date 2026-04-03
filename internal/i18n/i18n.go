package i18n

import (
	"os"
	"strings"
)

// DetectLocale detects the system locale by checking environment variables.
// Returns "ru" for Russian locale, "en" for everything else.
func DetectLocale() string {
	for _, envVar := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		val := os.Getenv(envVar)
		if val == "" {
			continue
		}
		// Normalize: take only the language part before _ or .
		lang := strings.ToLower(val)
		if idx := strings.IndexAny(lang, "_."); idx != -1 {
			lang = lang[:idx]
		}
		if lang == "ru" {
			return "ru"
		}
		if lang != "" {
			return "en"
		}
	}
	return "en"
}
