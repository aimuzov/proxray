package xray

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/aimuzov/happ-cli/internal/link"
)

// get navigates a decoded JSON tree using a dotted path with numeric indices,
// e.g. "outbounds.0.settings.vnext.0.address".
func get(t *testing.T, root any, path string) any {
	t.Helper()
	cur := root
	for _, seg := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[seg]
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				t.Fatalf("path %q: bad index %q (len=%d)", path, seg, len(node))
			}
			cur = node[idx]
		default:
			t.Fatalf("path %q: cannot descend into %T at %q", path, cur, seg)
		}
	}
	return cur
}

func buildMap(t *testing.T, s *link.Server, opts Options) map[string]any {
	t.Helper()
	cfg, err := BuildConfig(s, opts)
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	return m
}

func TestBuildConfigInbounds(t *testing.T) {
	s := &link.Server{Protocol: "vless", Address: "h", Port: 443, UUID: "u", Network: "tcp", Security: "none"}
	m := buildMap(t, s, Options{SocksPort: 10808, HTTPPort: 10809, Listen: "127.0.0.1"})

	if p := get(t, m, "inbounds.0.protocol"); p != "socks" {
		t.Errorf("inbound0 protocol = %v, want socks", p)
	}
	if p := get(t, m, "inbounds.0.port"); p != float64(10808) {
		t.Errorf("socks port = %v, want 10808", p)
	}
	if p := get(t, m, "inbounds.1.protocol"); p != "http" {
		t.Errorf("inbound1 protocol = %v, want http", p)
	}
	if p := get(t, m, "inbounds.1.port"); p != float64(10809) {
		t.Errorf("http port = %v, want 10809", p)
	}
}

func TestBuildConfigVLESSReality(t *testing.T) {
	s := &link.Server{
		Protocol: "vless", Address: "ex.com", Port: 443, UUID: "uuid-x",
		Flow: "xtls-rprx-vision", Network: "tcp", Security: "reality",
		SNI: "www.google.com", PublicKey: "PBK", ShortID: "sid1", SpiderX: "/", Fingerprint: "chrome",
	}
	m := buildMap(t, s, Options{SocksPort: 10808})

	if v := get(t, m, "outbounds.0.protocol"); v != "vless" {
		t.Errorf("protocol = %v", v)
	}
	if v := get(t, m, "outbounds.0.tag"); v != "proxy" {
		t.Errorf("tag = %v, want proxy", v)
	}
	if v := get(t, m, "outbounds.0.settings.vnext.0.address"); v != "ex.com" {
		t.Errorf("address = %v", v)
	}
	if v := get(t, m, "outbounds.0.settings.vnext.0.port"); v != float64(443) {
		t.Errorf("port = %v", v)
	}
	if v := get(t, m, "outbounds.0.settings.vnext.0.users.0.id"); v != "uuid-x" {
		t.Errorf("id = %v", v)
	}
	if v := get(t, m, "outbounds.0.settings.vnext.0.users.0.flow"); v != "xtls-rprx-vision" {
		t.Errorf("flow = %v", v)
	}
	if v := get(t, m, "outbounds.0.streamSettings.security"); v != "reality" {
		t.Errorf("security = %v", v)
	}
	if v := get(t, m, "outbounds.0.streamSettings.realitySettings.publicKey"); v != "PBK" {
		t.Errorf("publicKey = %v", v)
	}
	if v := get(t, m, "outbounds.0.streamSettings.realitySettings.serverName"); v != "www.google.com" {
		t.Errorf("serverName = %v", v)
	}
	if v := get(t, m, "outbounds.0.streamSettings.realitySettings.shortId"); v != "sid1" {
		t.Errorf("shortId = %v", v)
	}
	// direct + block outbounds present
	if v := get(t, m, "outbounds.1.protocol"); v != "freedom" {
		t.Errorf("outbound1 = %v, want freedom", v)
	}
}

func TestBuildConfigTrojanWebSocketTLS(t *testing.T) {
	s := &link.Server{
		Protocol: "trojan", Address: "tj.com", Port: 443, Password: "secret",
		Network: "ws", Path: "/wp", Host: "cdn.com", Security: "tls", SNI: "cdn.com", Fingerprint: "chrome",
	}
	m := buildMap(t, s, Options{SocksPort: 10808})

	if v := get(t, m, "outbounds.0.protocol"); v != "trojan" {
		t.Errorf("protocol = %v", v)
	}
	if v := get(t, m, "outbounds.0.settings.servers.0.password"); v != "secret" {
		t.Errorf("password = %v", v)
	}
	if v := get(t, m, "outbounds.0.streamSettings.network"); v != "ws" {
		t.Errorf("network = %v", v)
	}
	if v := get(t, m, "outbounds.0.streamSettings.wsSettings.path"); v != "/wp" {
		t.Errorf("ws path = %v", v)
	}
	if v := get(t, m, "outbounds.0.streamSettings.wsSettings.headers.Host"); v != "cdn.com" {
		t.Errorf("ws host header = %v", v)
	}
	if v := get(t, m, "outbounds.0.streamSettings.security"); v != "tls" {
		t.Errorf("security = %v", v)
	}
	if v := get(t, m, "outbounds.0.streamSettings.tlsSettings.serverName"); v != "cdn.com" {
		t.Errorf("tls serverName = %v", v)
	}
}

func TestBuildConfigShadowsocks(t *testing.T) {
	s := &link.Server{Protocol: "shadowsocks", Address: "ss.com", Port: 8388, Method: "aes-256-gcm", Password: "p", Network: "tcp"}
	m := buildMap(t, s, Options{SocksPort: 10808})

	if v := get(t, m, "outbounds.0.protocol"); v != "shadowsocks" {
		t.Errorf("protocol = %v", v)
	}
	if v := get(t, m, "outbounds.0.settings.servers.0.method"); v != "aes-256-gcm" {
		t.Errorf("method = %v", v)
	}
	if v := get(t, m, "outbounds.0.settings.servers.0.password"); v != "p" {
		t.Errorf("password = %v", v)
	}
}

func TestBuildConfigGRPCReality(t *testing.T) {
	s := &link.Server{
		Protocol: "vless", Address: "g.com", Port: 443, UUID: "u",
		Network: "grpc", ServiceName: "gsvc", Security: "reality", PublicKey: "K", SNI: "sni.com",
	}
	m := buildMap(t, s, Options{SocksPort: 10808})
	if v := get(t, m, "outbounds.0.streamSettings.network"); v != "grpc" {
		t.Errorf("network = %v", v)
	}
	if v := get(t, m, "outbounds.0.streamSettings.grpcSettings.serviceName"); v != "gsvc" {
		t.Errorf("grpc serviceName = %v", v)
	}
}

func TestBuildConfigRejectsHysteria2(t *testing.T) {
	s := &link.Server{Protocol: "hysteria2", Address: "h", Port: 443, Password: "p"}
	if _, err := BuildConfig(s, Options{SocksPort: 10808}); err == nil {
		t.Fatal("expected error for hysteria2 (unsupported by xray-core)")
	}
}

func TestBuildConfigBypassRU(t *testing.T) {
	s := &link.Server{Protocol: "vless", Address: "h", Port: 443, UUID: "u", Network: "tcp", Security: "none"}
	m := buildMap(t, s, Options{SocksPort: 10808, Bypass: "ru"})

	if v := get(t, m, "routing.domainStrategy"); v != "IPIfNonMatch" {
		t.Errorf("domainStrategy = %v, want IPIfNonMatch", v)
	}
	if v := get(t, m, "routing.rules.0.domain.0"); v != "geosite:category-ru" {
		t.Errorf("rule0 domain0 = %v, want geosite:category-ru", v)
	}
	if v := get(t, m, "routing.rules.0.outboundTag"); v != "direct" {
		t.Errorf("rule0 outboundTag = %v, want direct", v)
	}
	if v := get(t, m, "routing.rules.1.ip.0"); v != "geoip:private" {
		t.Errorf("rule1 ip0 = %v, want geoip:private", v)
	}
	if v := get(t, m, "routing.rules.1.ip.1"); v != "geoip:ru" {
		t.Errorf("rule1 ip1 = %v, want geoip:ru", v)
	}
}

func TestBuildConfigBypassOff(t *testing.T) {
	s := &link.Server{Protocol: "vless", Address: "h", Port: 443, UUID: "u", Network: "tcp", Security: "none"}
	m := buildMap(t, s, Options{SocksPort: 10808, Bypass: "off"})

	if _, ok := m["routing"]; ok {
		t.Errorf("routing present for bypass=off, want absent")
	}
}
