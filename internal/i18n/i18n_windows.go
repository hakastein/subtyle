//go:build windows

package i18n

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const localeNameMaxLength = 85 // LOCALE_NAME_MAX_LENGTH

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procGetUserDefaultLocaleName = kernel32.NewProc("GetUserDefaultLocaleName")
)

// DetectLocale uses Windows GetUserDefaultLocaleName, with env-var fallback.
func DetectLocale() string {
	if loc, err := getWindowsLocale(); err == nil {
		if normalized := normalizeBCP47(loc); normalized != "" {
			return normalized
		}
	}
	// Fallback to env vars (WSL, etc.)
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

func getWindowsLocale() (string, error) {
	buf := make([]uint16, localeNameMaxLength)
	r, _, err := procGetUserDefaultLocaleName.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(localeNameMaxLength),
	)
	if r == 0 {
		return "", fmt.Errorf("GetUserDefaultLocaleName failed: %w", err)
	}
	return syscall.UTF16ToString(buf), nil
}

// normalizeBCP47 is shared between platform files (each file compiles only on
// its platform via build tags — no linker conflict).
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
