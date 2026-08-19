package profile

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetch(t *testing.T) {
	links := strings.Join([]string{
		"vless://uuid-1@a.example.com:443?type=tcp&security=reality&pbk=k#A",
		"trojan://" + "pw" + "@b.example.com:443#B",
	}, "\n")

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("subscription-userinfo", "upload=1; download=2; total=100; expire=0")
		w.Header().Set("profile-title", "base64:"+base64.StdEncoding.EncodeToString([]byte("My VPN")))
		w.Header().Set("profile-update-interval", "24")
		w.Header().Set("support-url", "https://support.example.com")
		_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(links))))
	}))
	defer srv.Close()

	sub, err := Fetch(context.Background(), srv.URL, "Happ/1.0")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if gotUA != "Happ/1.0" {
		t.Errorf("User-Agent sent = %q, want Happ/1.0", gotUA)
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
