// Package log is the diagnostic logger for happ. It writes leveled, colored
// messages to stderr. User-facing results belong in internal/ui, not here.
package log

import (
	"io"
	"os"

	charmlog "github.com/charmbracelet/log"
)

// logOptions formats every line as "<time> <LEVEL> <message>" with a compact,
// human-readable clock (no microseconds, no date).
var logOptions = charmlog.Options{
	ReportTimestamp: true,
	TimeFormat:      "15:04:05",
}

var logger = charmlog.NewWithOptions(os.Stderr, logOptions)

// SetVerbose raises the level to DEBUG when v is true, else INFO.
func SetVerbose(v bool) {
	if v {
		logger.SetLevel(charmlog.DebugLevel)
		return
	}
	logger.SetLevel(charmlog.InfoLevel)
}

// SetOutput redirects the logger (used by tests). It resets to a fresh logger
// on w so the color profile is recomputed for the new writer.
func SetOutput(w io.Writer) {
	logger = charmlog.NewWithOptions(w, logOptions)
}

func Debug(msg string, kv ...any) { logger.Debug(msg, kv...) }
func Info(msg string, kv ...any)  { logger.Info(msg, kv...) }
func Warn(msg string, kv ...any)  { logger.Warn(msg, kv...) }
func Error(msg string, kv ...any) { logger.Error(msg, kv...) }
