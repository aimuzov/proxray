package rawconf

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fixture loads the anonymized JSON subscription used across these tests: a
// balancer entry, a plain VLESS/Reality entry, and a Hysteria2 entry.
func fixture(t *testing.T) []*Config {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "subscription.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	configs, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(configs) != 3 {
		t.Fatalf("got %d configs, want 3", len(configs))
	}
	return configs
}

func TestParseArray(t *testing.T) {
	configs := fixture(t)

	if got := configs[1].Remarks(); got != "🇵🇱 Польша 🦫" {
		t.Errorf("Remarks = %q", got)
	}
	if got := configs[1].Description(); got != "Основной сервер" {
		t.Errorf("Description = %q", got)
	}
}

func TestParseSingleObject(t *testing.T) {
	body := []byte(`{"remarks":"solo","outbounds":[{"tag":"proxy","protocol":"vless",
		"settings":{"vnext":[{"address":"a.example.com","port":443}]}}]}`)
	configs, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(configs) != 1 || configs[0].Remarks() != "solo" {
		t.Fatalf("got %d configs (%+v)", len(configs), configs)
	}
}

func TestParseNotJSON(t *testing.T) {
	for _, body := range []string{
		"vless://uuid@a.example.com:443#A\n",
		"dmxlc3M6Ly91dWlkQGEuZXhhbXBsZS5jb206NDQzI0E=",
		"",
	} {
		if _, err := Parse([]byte(body)); !errors.Is(err, ErrNotJSON) {
			t.Errorf("Parse(%q) error = %v, want ErrNotJSON", body, err)
		}
	}
}

func TestParseRejectsForeignJSON(t *testing.T) {
	// The same panel serves a Clash document to Clash clients; it is JSON, but
	// not an xray config, and must not be mistaken for one.
	body := []byte(`{"proxies":[{"name":"a","type":"vless","server":"a.example.com"}]}`)
	_, err := Parse(body)
	if err == nil || errors.Is(err, ErrNotJSON) {
		t.Fatalf("Parse error = %v, want a descriptive failure", err)
	}
}

func TestParseMalformedJSON(t *testing.T) {
	if _, err := Parse([]byte(`[{"outbounds":`)); err == nil || errors.Is(err, ErrNotJSON) {
		t.Fatalf("Parse error = %v, want a parse failure", err)
	}
}

func TestEndpoints(t *testing.T) {
	configs := fixture(t)

	// Balancer entry: every dialable outbound is listed, service ones are not.
	got := configs[0].Endpoints()
	if len(got) != 3 {
		t.Fatalf("balancer entry: got %d endpoints, want 3: %+v", len(got), got)
	}
	if got[0].Tag != "node-0" || got[0].Protocol != "vless" || got[0].Port != 443 {
		t.Errorf("endpoint 0 = %+v", got[0])
	}

	// Hysteria2 keeps its address next to the settings, not under vnext/servers.
	hy := configs[2].Endpoints()
	if len(hy) != 1 {
		t.Fatalf("hysteria entry: got %d endpoints, want 1: %+v", len(hy), hy)
	}
	if hy[0].Protocol != "hysteria" || hy[0].Address == "" || hy[0].Port != 8443 {
		t.Errorf("hysteria endpoint = %+v", hy[0])
	}
}

func TestEndpointsFromServersList(t *testing.T) {
	body := []byte(`{"outbounds":[{"tag":"proxy","protocol":"trojan",
		"settings":{"servers":[{"address":"t.example.com","port":8443,"password":"p"}]}}]}`)
	configs, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	got := configs[0].Endpoints()
	if len(got) != 1 || got[0].Address != "t.example.com" || got[0].Port != 8443 {
		t.Fatalf("Endpoints = %+v", got)
	}
}

func TestProtocolsAndAddresses(t *testing.T) {
	configs := fixture(t)

	if got := configs[0].Protocols(); len(got) != 1 || got[0] != "vless" {
		t.Errorf("Protocols = %v, want [vless] (deduplicated)", got)
	}
	if got := configs[0].Addresses(); len(got) != 3 {
		t.Errorf("Addresses = %v, want 3 distinct hosts", got)
	}
	if got := configs[2].Protocols(); len(got) != 1 || got[0] != "hysteria" {
		t.Errorf("Protocols = %v, want [hysteria]", got)
	}
}

func TestUsesGeoAssets(t *testing.T) {
	configs := fixture(t)
	if configs[1].UsesGeoAssets() {
		t.Error("fixture routes by regexp and CIDR, not geo databases")
	}

	body := []byte(`{"outbounds":[{"protocol":"freedom","tag":"direct"}],
		"routing":{"rules":[{"type":"field","domain":["geosite:category-ru"],"outboundTag":"direct"}]}}`)
	configs, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !configs[0].UsesGeoAssets() {
		t.Error("geosite: rule should require the geo databases")
	}
}
