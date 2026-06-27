package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestVerboseGatesDebug(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)

	SetVerbose(false)
	Debug("hidden-debug")
	Info("shown-info")
	if strings.Contains(buf.String(), "hidden-debug") {
		t.Fatalf("debug must be suppressed at default level, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "shown-info") {
		t.Fatalf("info must be visible, got:\n%s", buf.String())
	}

	buf.Reset()
	SetVerbose(true)
	Debug("now-visible")
	if !strings.Contains(buf.String(), "now-visible") {
		t.Fatalf("debug must be visible after SetVerbose(true), got:\n%s", buf.String())
	}
}

func TestLevelHelpersWriteMessage(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	SetVerbose(false)
	Warn("disk-low", "free", "10MB")
	out := buf.String()
	if !strings.Contains(out, "disk-low") || !strings.Contains(out, "free") {
		t.Fatalf("warn must include message and key, got:\n%s", out)
	}
}
