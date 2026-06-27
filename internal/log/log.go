// Package log is the diagnostic logger for happ. It writes leveled, colored
// messages to stderr. User-facing results belong in internal/ui, not here.
package log

import (
	"io"
	"os"

	charmlog "github.com/charmbracelet/log"
)

var logger = charmlog.NewWithOptions(os.Stderr, charmlog.Options{
	ReportTimestamp: false,
})

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
	logger = charmlog.NewWithOptions(w, charmlog.Options{ReportTimestamp: false})
}

func Debug(msg string, kv ...any) { logger.Debug(msg, kv...) }
func Info(msg string, kv ...any)  { logger.Info(msg, kv...) }
func Warn(msg string, kv ...any)  { logger.Warn(msg, kv...) }
func Error(msg string, kv ...any) { logger.Error(msg, kv...) }
