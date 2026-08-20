package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aimuzov/proxray/internal/device"
	"github.com/aimuzov/proxray/internal/store"
	"github.com/aimuzov/proxray/internal/ui"
)

func TestEnsureHWIDGeneratesOnceAndPersists(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	id, err := ensureHWID(st)
	if err != nil {
		t.Fatalf("ensureHWID: %v", err)
	}
	if !device.Valid(id) {
		t.Fatalf("ensureHWID produced %q, which panels reject", id)
	}
	if again, err := ensureHWID(st); err != nil || again != id {
		t.Errorf("second ensureHWID = %q, %v; want the first id back", again, err)
	}

	// A device that changed id between updates would look like a new device to
	// the panel and eat another slot.
	reopened, err := store.Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := reopened.HWID(); got != id {
		t.Errorf("stored HWID = %q, want %q", got, id)
	}
}

func TestEnsureHWIDKeepsAnOverride(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.SetHWID("deadbeef1234"); err != nil {
		t.Fatalf("SetHWID: %v", err)
	}
	if got, err := ensureHWID(st); err != nil || got != "deadbeef1234" {
		t.Errorf("ensureHWID = %q, %v; want the stored override", got, err)
	}
}

// The panel usually answers nothing about the device id, so the CLI itself has
// to say it was sent — otherwise there is no way to tell from the terminal.
func TestFetchEntryReportsSentHWID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte("trojan://pw@b.example.com:443#B"))))
	}))
	defer srv.Close()

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	var buf bytes.Buffer
	ui.SetOutput(&buf)
	t.Cleanup(func() { ui.SetOutput(os.Stdout) })

	entry, err := fetchEntry(context.Background(), st, subRequest{URL: srv.URL, Name: "t"})
	if err != nil {
		t.Fatalf("fetchEntry: %v", err)
	}
	if entry.NoHWID {
		t.Error("NoHWID should stay false when the id was sent")
	}
	if got := buf.String(); !strings.Contains(got, st.HWID()) {
		t.Errorf("output does not report the sent device id %q; got:\n%s", st.HWID(), got)
	}
}

func TestFetchEntryStaysQuietWithNoHWID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-hwid") != "" {
			t.Errorf("x-hwid sent despite --no-hwid: %q", r.Header.Get("x-hwid"))
		}
		_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte("trojan://pw@b.example.com:443#B"))))
	}))
	defer srv.Close()

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	var buf bytes.Buffer
	ui.SetOutput(&buf)
	t.Cleanup(func() { ui.SetOutput(os.Stdout) })

	if _, err := fetchEntry(context.Background(), st, subRequest{URL: srv.URL, Name: "t", NoHWID: true}); err != nil {
		t.Fatalf("fetchEntry: %v", err)
	}
	if strings.Contains(buf.String(), "device id") {
		t.Errorf("--no-hwid should not report a device id; got:\n%s", buf.String())
	}
	// Nothing was sent, so nothing should have been generated and stored either.
	if got := st.HWID(); got != "" {
		t.Errorf("stored HWID = %q, want empty", got)
	}
}

func TestHWIDSetRejectsInvalidID(t *testing.T) {
	homeDir = t.TempDir()
	t.Cleanup(func() { homeDir = "" })

	cmd := newHWIDCmd()
	cmd.SetArgs([]string{"set", "no"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("hwid set accepted an id the panel would reject")
	}
}
