package profile

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aimuzov/proxray/internal/log"
)

func TestFetch(t *testing.T) {
	links := strings.Join([]string{
		"vless://uuid-1@a.example.com:443?type=tcp&security=reality&pbk=k#A",
		"trojan://" + "pw" + "@b.example.com:443#B",
	}, "\n")

	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Header().Set("x-hwid-active", "true")
		w.Header().Set("subscription-userinfo", "upload=1; download=2; total=100; expire=0")
		w.Header().Set("profile-title", "base64:"+base64.StdEncoding.EncodeToString([]byte("My VPN")))
		w.Header().Set("profile-update-interval", "24")
		w.Header().Set("support-url", "https://support.example.com")
		_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(links))))
	}))
	defer srv.Close()

	sub, err := Fetch(context.Background(), srv.URL, FetchOptions{
		UserAgent:   "Happ/1.0",
		HWID:        "abcdef0123456789",
		DeviceOS:    "macOS",
		OSVersion:   "15.3",
		DeviceModel: "MacBookPro18,3",
	})
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	for header, want := range map[string]string{
		"User-Agent":     "Happ/1.0",
		"X-Hwid":         "abcdef0123456789",
		"X-Device-Os":    "macOS",
		"X-Ver-Os":       "15.3",
		"X-Device-Model": "MacBookPro18,3",
	} {
		if g := got.Get(header); g != want {
			t.Errorf("%s sent = %q, want %q", header, g, want)
		}
	}
	if !sub.HWIDActive {
		t.Error("HWIDActive = false, want true when the panel reports x-hwid-active")
	}
	if sub.Title != "My VPN" {
		t.Errorf("Title = %q, want 'My VPN'", sub.Title)
	}
	if sub.UpdateInterval != 24 {
		t.Errorf("UpdateInterval = %d, want 24", sub.UpdateInterval)
	}
	if sub.SupportURL != "https://support.example.com" {
		t.Errorf("SupportURL = %q", sub.SupportURL)
	}
	if sub.UserInfo == nil || sub.UserInfo.Total != 100 {
		t.Errorf("UserInfo = %+v", sub.UserInfo)
	}
	if len(sub.Nodes()) != 2 {
		t.Fatalf("got %d servers, want 2", len(sub.Nodes()))
	}
}

// Panels that only read the device id answer with no x-hwid-* headers at all.
// That silence is the common case, so -v has to state it rather than print
// nothing and leave the user unable to tell the id was sent.
func TestFetchLogsHWIDExchange(t *testing.T) {
	tests := []struct {
		name    string
		set     map[string]string
		verdict string
	}{
		{"silent panel", nil, "none (panel ignores the device id)"},
		{"panel enforces", map[string]string{"x-hwid-active": "true"}, "x-hwid-active=true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tt.set {
					w.Header().Set(k, v)
				}
				_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte("trojan://pw@b.example.com:443#B"))))
			}))
			defer srv.Close()

			var buf bytes.Buffer
			log.SetOutput(&buf)
			log.SetVerbose(true)
			t.Cleanup(func() { log.SetVerbose(false) })

			if _, err := Fetch(context.Background(), srv.URL, FetchOptions{HWID: "abcdef0123456789"}); err != nil {
				t.Fatalf("Fetch error: %v", err)
			}

			got := buf.String()
			for _, want := range []string{"abcdef0123456789", tt.verdict} {
				if !strings.Contains(got, want) {
					t.Errorf("debug log missing %q; got:\n%s", want, got)
				}
			}
			// The path carries the subscription token and must never be logged.
			if strings.Contains(got, srv.URL) {
				t.Errorf("debug log leaked the subscription URL:\n%s", got)
			}
		})
	}
}

// A panel enforcing a device limit answers 404, so the reason has to be read
// off the headers or the user is left guessing.
func TestFetchReportsDeviceLimit(t *testing.T) {
	tests := []struct {
		name   string
		header string
		opts   FetchOptions
		want   string
	}{
		{"limit reached", "x-hwid-max-devices-reached", FetchOptions{HWID: "abcdef0123456789"}, "device limit reached"},
		{"id required", "x-hwid-not-supported", FetchOptions{}, "requires a device id"},
		{"id not sent", "", FetchOptions{}, "--no-hwid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.header != "" {
					w.Header().Set(tt.header, "true")
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer srv.Close()

			_, err := Fetch(context.Background(), srv.URL, tt.opts)
			if err == nil {
				t.Fatal("Fetch succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}
