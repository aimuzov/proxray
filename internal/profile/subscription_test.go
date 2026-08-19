package profile

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseBodyBase64List(t *testing.T) {
	links := strings.Join([]string{
		"vless://uuid-1@a.example.com:443?type=tcp&security=reality&pbk=k#A",
		"trojan://" + "pw" + "@b.example.com:443#B",
		"", // blank line should be skipped
		"ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:p")) + "@c.example.com:8388#C",
	}, "\n")
	body := []byte(base64.StdEncoding.EncodeToString([]byte(links)))

	parsed, err := ParseBody(body)
	servers := parsed.Servers
	if err != nil {
		t.Fatalf("ParseBody error: %v", err)
	}
	if len(servers) != 3 {
		t.Fatalf("got %d servers, want 3: %+v", len(servers), servers)
	}
	if servers[0].Protocol != "vless" || servers[1].Protocol != "trojan" || servers[2].Protocol != "shadowsocks" {
		t.Errorf("protocols = %q %q %q", servers[0].Protocol, servers[1].Protocol, servers[2].Protocol)
	}
}

func TestParseBodyPlainList(t *testing.T) {
	// Not base64 — already contains scheme markers.
	body := []byte("vless://uuid-1@a.example.com:443?type=tcp#A\nvless://uuid-2@d.example.com:443?type=tcp#D\n")
	parsed, err := ParseBody(body)
	servers := parsed.Servers
	if err != nil {
		t.Fatalf("ParseBody error: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
}

func TestParseBodySkipsUnparseable(t *testing.T) {
	links := "vless://uuid-1@a.example.com:443?type=tcp#A\nssr://garbage\nnot-a-link\n"
	body := []byte(base64.StdEncoding.EncodeToString([]byte(links)))
	parsed, err := ParseBody(body)
	servers := parsed.Servers
	if err != nil {
		t.Fatalf("ParseBody error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1 (skip unsupported)", len(servers))
	}
}

func TestParseBodyJSONSubscription(t *testing.T) {
	body := []byte(`[{"remarks":"Poland","meta":{"serverDescription":"main"},
		"outbounds":[{"tag":"proxy","protocol":"vless",
			"settings":{"vnext":[{"address":"pl.example.com","port":443}]}},
			{"tag":"direct","protocol":"freedom"}]}]`)

	parsed, err := ParseBody(body)
	if err != nil {
		t.Fatalf("ParseBody error: %v", err)
	}
	if len(parsed.Servers) != 0 || len(parsed.Configs) != 1 {
		t.Fatalf("parsed = %d servers, %d configs", len(parsed.Servers), len(parsed.Configs))
	}

	node := NodeFromConfig(parsed.Configs[0])
	if node.Tag != "Poland" || node.Note != "main" {
		t.Errorf("node = %+v", node)
	}
	if node.Address != "pl.example.com:443" {
		t.Errorf("Address = %q", node.Address)
	}
	if len(node.Protocols) != 1 || node.Protocols[0] != "vless" {
		t.Errorf("Protocols = %v (service outbounds must not count)", node.Protocols)
	}
}

func TestParseBodyBase64JSONSubscription(t *testing.T) {
	inner := `[{"remarks":"Poland","outbounds":[{"tag":"proxy","protocol":"vless",
		"settings":{"vnext":[{"address":"pl.example.com","port":443}]}}]}]`
	body := []byte(base64.StdEncoding.EncodeToString([]byte(inner)))

	parsed, err := ParseBody(body)
	if err != nil {
		t.Fatalf("ParseBody error: %v", err)
	}
	if len(parsed.Configs) != 1 {
		t.Fatalf("got %d configs, want 1", len(parsed.Configs))
	}
}

func TestParseBodyRejectsForeignJSON(t *testing.T) {
	// A Clash document is JSON but not an xray config: better to fail loudly
	// than to store a subscription with zero servers.
	body := []byte(`{"proxies":[{"name":"a","type":"vless","server":"a.example.com"}]}`)
	if _, err := ParseBody(body); err == nil {
		t.Fatal("expected an error for a non-xray JSON body")
	}
}

func TestParseUserInfo(t *testing.T) {
	ui, ok := ParseUserInfo("upload=455727941; download=6174315083; total=1073741824000; expire=1671815872")
	if !ok {
		t.Fatal("ParseUserInfo returned ok=false")
	}
	if ui.Upload != 455727941 {
		t.Errorf("Upload = %d", ui.Upload)
	}
	if ui.Download != 6174315083 {
		t.Errorf("Download = %d", ui.Download)
	}
	if ui.Total != 1073741824000 {
		t.Errorf("Total = %d", ui.Total)
	}
	if ui.Expire.IsZero() || ui.Expire.Unix() != 1671815872 {
		t.Errorf("Expire = %v (unix %d)", ui.Expire, ui.Expire.Unix())
	}
	if ui.Remaining() != ui.Total-ui.Upload-ui.Download {
		t.Errorf("Remaining = %d", ui.Remaining())
	}
}

func TestParseUserInfoEmpty(t *testing.T) {
	if _, ok := ParseUserInfo(""); ok {
		t.Error("expected ok=false for empty header")
	}
}
