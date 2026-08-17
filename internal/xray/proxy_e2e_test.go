package xray

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/net/proxy"

	"github.com/aimuzov/proxray/internal/link"
)

// TestProxyEndToEnd stands up a real xray Shadowsocks server, a client built
// from a link.Server via BuildConfig, and verifies an HTTP request routed
// through the client's SOCKS inbound reaches a target server through the proxy.
func TestProxyEndToEnd(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "pong")
	}))
	defer target.Close()

	const method, password = "aes-256-gcm", "e2e-secret"
	ssPort := freePort(t)

	serverCfg := fmt.Sprintf(`{
      "log": {"loglevel": "warning"},
      "inbounds": [{
        "tag": "ss-in", "listen": "127.0.0.1", "port": %d, "protocol": "shadowsocks",
        "settings": {"method": %q, "password": %q, "network": "tcp"}
      }],
      "outbounds": [{"protocol": "freedom"}]
    }`, ssPort, method, password)

	server := startInstance(t, []byte(serverCfg))
	defer server.Close()

	clientSrv := &link.Server{
		Protocol: "shadowsocks", Address: "127.0.0.1", Port: ssPort,
		Method: method, Password: password, Network: "tcp",
	}
	socksPort := freePort(t)
	cfg, err := BuildConfig(clientSrv, Options{SocksPort: socksPort})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}
	raw, err := cfg.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	client := startInstance(t, raw)
	defer client.Close()

	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), nil, proxy.Direct)
	if err != nil {
		t.Fatalf("socks dialer: %v", err)
	}
	httpClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{Dial: dialer.Dial},
	}

	// Retry briefly while both instances finish binding.
	var body string
	for i := 0; i < 20; i++ {
		resp, err := httpClient.Get(target.URL)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body = string(b)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if body != "pong" {
		t.Fatalf("response through proxy = %q, want %q", body, "pong")
	}
}
