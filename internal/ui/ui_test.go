package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aimuzov/happ-cli/internal/link"
)

func TestStatusHelpersWritePlainText(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)

	Success("proxy up on %d", 1080)
	Info("press ctrl+c")
	Warn("restore failed")

	out := buf.String()
	for _, want := range []string{"proxy up on 1080", "press ctrl+c", "restore failed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestServerLine(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	Server(0, link.Server{Tag: "Tokyo", Protocol: "vless", Address: "1.2.3.4", Port: 443})
	out := buf.String()
	for _, want := range []string{"#1", "Tokyo", "vless", "1.2.3.4", "443"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in server line:\n%s", want, out)
		}
	}
}

func TestEndpoints(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	Endpoints(1080, 8080)
	out := buf.String()
	if !strings.Contains(out, "1080") || !strings.Contains(out, "8080") {
		t.Fatalf("endpoints must list both ports, got:\n%s", out)
	}
}

func TestTableRendersHeadersAndRows(t *testing.T) {
	SetOutput(new(bytes.Buffer)) // force Ascii profile
	got := Table(
		[]string{"#", "TAG"},
		[][]string{{"1", "Tokyo"}, {"2", "Berlin"}},
	)
	for _, want := range []string{"#", "TAG", "Tokyo", "Berlin"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing %q in:\n%s", want, got)
		}
	}
}
