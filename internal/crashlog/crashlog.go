// Package crashlog handles panic recovery and writes crash dumps to disk.
// Goal: never let an unhandled panic kill the app silently.
package crashlog

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

var (
	logPath string
	mu      sync.Mutex
)

// Init sets up the crash log path. Subsequent Recover calls write there.
func Init(dataDir string) {
	mu.Lock()
	defer mu.Unlock()
	logPath = filepath.Join(dataDir, "crash.log")
}

// Path returns the configured crash log path (may be empty if Init not called).
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return logPath
}

// Recover is meant to be deferred at the top of goroutines and Wails-bound
// methods. It recovers panics, writes a dump to crash.log, and returns the
// panic value as an error. Pass a label identifying the call site.
//
// Usage:
//
//	defer func() {
//		if err := crashlog.Recover("ParseFile"); err != nil {
//			// handle or re-raise
//		}
//	}()
func Recover(label string) error {
	r := recover()
	if r == nil {
		return nil
	}
	return RecoverFrom(label, r)
}

// RecoverFrom writes the crash log for an already-recovered panic value.
// Use when the caller needs to do additional work before/after recovery.
func RecoverFrom(label string, r interface{}) error {
	trace := debug.Stack()
	entry := fmt.Sprintf(
		"\n=== PANIC at %s ===\nTime: %s\nLabel: %s\nValue: %v\nStack:\n%s\n",
		time.Now().Format(time.RFC3339),
		time.Now().Format(time.RFC3339),
		label,
		r,
		trace,
	)

	// Always print to stderr — helps when launched from terminal
	fmt.Fprint(os.Stderr, entry)

	// Best-effort write to crash log
	mu.Lock()
	path := logPath
	mu.Unlock()
	if path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
			_, _ = f.WriteString(entry)
			_ = f.Close()
		}
	}

	return fmt.Errorf("panic in %s: %v", label, r)
}

// Guard wraps a no-argument function with panic recovery. Useful for goroutines
// where you don't need the error back.
func Guard(label string, fn func()) {
	defer func() {
		_ = Recover(label)
	}()
	fn()
}
