package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aimuzov/proxray/internal/link"
	"github.com/aimuzov/proxray/internal/profile"
	"github.com/aimuzov/proxray/internal/rawconf"
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

func TestNodeLine(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	Node(0, profile.NodeFromServer(&link.Server{
		Tag: "Tokyo", Protocol: "vless", Address: "1.2.3.4", Port: 443,
	}))
	out := buf.String()
	for _, want := range []string{"#1", "Tokyo", "vless", "1.2.3.4", "443"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in server line:\n%s", want, out)
		}
	}
}

func TestNodeLineCountsPooledServers(t *testing.T) {
	configs, err := rawconf.Parse([]byte(`{"remarks":"Auto","outbounds":[
		{"tag":"a","protocol":"vless","settings":{"vnext":[{"address":"a.example.com","port":443}]}},
		{"tag":"b","protocol":"vless","settings":{"vnext":[{"address":"b.example.com","port":443}]}}]}`))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	var buf bytes.Buffer
	SetOutput(&buf)
	Node(2, profile.NodeFromConfig(configs[0]))
	out := buf.String()
	for _, want := range []string{"#3", "Auto", "vless", "2 servers"} {
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
