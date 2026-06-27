package xraylog

import (
	"bytes"
	"strings"
	"testing"

	xlog "github.com/xtls/xray-core/common/log"

	"github.com/aimuzov/proxray/internal/log"
)

func TestAccessHiddenWithoutVerbose(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetVerbose(false)

	Handler{verbose: false}.Handle(&xlog.AccessMessage{To: "//apple.com:443", Detour: "http-in >> proxy"})

	if buf.Len() != 0 {
		t.Fatalf("access must be silent without verbose, got:\n%s", buf.String())
	}
}

func TestAccessShownWithVerbose(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetVerbose(true)

	Handler{verbose: true}.Handle(&xlog.AccessMessage{To: "//apple.com:443", Detour: "http-in >> proxy"})

	out := buf.String()
	if !strings.Contains(out, "apple.com:443") {
		t.Fatalf("access must show the destination host, got:\n%s", out)
	}
	if strings.Contains(out, "//") {
		t.Fatalf("leading // must be stripped, got:\n%s", out)
	}
	if !strings.Contains(out, "proxy") {
		t.Fatalf("access must show the short detour, got:\n%s", out)
	}
}

func TestGeneralWarningAlwaysShown(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetVerbose(false)

	Handler{verbose: false}.Handle(&xlog.GeneralMessage{Severity: xlog.Severity_Warning, Content: "Xray started"})

	out := buf.String()
	if !strings.Contains(out, "Xray started") || !strings.Contains(out, "WARN") {
		t.Fatalf("warning must always be shown with WARN level, got:\n%s", out)
	}
}

func TestGeneralInfoGatedByVerbose(t *testing.T) {
	var buf bytes.Buffer

	log.SetOutput(&buf)
	log.SetVerbose(false)
	Handler{verbose: false}.Handle(&xlog.GeneralMessage{Severity: xlog.Severity_Info, Content: "noisy"})
	if buf.Len() != 0 {
		t.Fatalf("info must be hidden without verbose, got:\n%s", buf.String())
	}

	buf.Reset()
	log.SetVerbose(true)
	Handler{verbose: true}.Handle(&xlog.GeneralMessage{Severity: xlog.Severity_Info, Content: "noisy"})
	if !strings.Contains(buf.String(), "noisy") {
		t.Fatalf("info must be shown with verbose, got:\n%s", buf.String())
	}
}

func TestCleanHost(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"//apple.com:443", "apple.com:443"},
		{"apple.com:443", "apple.com:443"},
	} {
		if got := cleanHost(tc.in); got != tc.want {
			t.Errorf("cleanHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestShortDetour(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"http-in >> proxy", "proxy"},
		{"socks-in >> proxy", "proxy"},
		{"direct", "direct"},
		{"", ""},
	} {
		if got := shortDetour(tc.in); got != tc.want {
			t.Errorf("shortDetour(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
