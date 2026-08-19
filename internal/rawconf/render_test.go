package rawconf

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
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

func render(t *testing.T, c *Config, opts RenderOptions) map[string]any {
	t.Helper()
	raw, err := c.Render(opts)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal rendered config: %v", err)
	}
	return m
}

// inbounds indexes the rendered inbounds by tag.
func inbounds(t *testing.T, m map[string]any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, item := range sliceOf(m["inbounds"]) {
		in := mapOf(item)
		out[str(in["tag"])] = in
	}
	return out
}

func TestRenderRewritesPortsKeepingTags(t *testing.T) {
	configs := fixture(t)
	m := render(t, configs[0], RenderOptions{SocksPort: 1080, HTTPPort: 1081, LogLevel: "info"})

	if lvl := get(t, m, "log.loglevel"); lvl != "info" {
		t.Errorf("loglevel = %v", lvl)
	}

	ins := inbounds(t, m)
	// The tags are what routing rules select on, so they must survive.
	if socks := ins["socks"]; socks == nil || intOf(socks["port"]) != 1080 || socks["listen"] != "127.0.0.1" {
		t.Errorf("socks inbound = %+v", socks)
	}
	if http := ins["http"]; http == nil || intOf(http["port"]) != 1081 {
		t.Errorf("http inbound = %+v", http)
	}
	// The dokodemo-door entries feeding the loopback chains stay as shipped.
	if entry := ins["entry-wl1"]; entry == nil || intOf(entry["port"]) != 10820 {
		t.Errorf("entry-wl1 inbound = %+v", entry)
	}
	if len(ins) != 5 {
		t.Errorf("got %d inbounds, want 5", len(ins))
	}
}

func TestRenderDropsHTTPInboundWhenDisabled(t *testing.T) {
	configs := fixture(t)
	m := render(t, configs[1], RenderOptions{SocksPort: 1080})

	ins := inbounds(t, m)
	if _, ok := ins["http"]; ok {
		t.Error("http inbound should be dropped when HTTPPort is 0")
	}
	if ins["socks"] == nil {
		t.Fatal("socks inbound missing")
	}
}

func TestRenderAddsMissingInbounds(t *testing.T) {
	body := []byte(`{"outbounds":[{"tag":"proxy","protocol":"vless",
		"settings":{"vnext":[{"address":"a.example.com","port":443}]}}]}`)
	configs, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	m := render(t, configs[0], RenderOptions{SocksPort: 1080, HTTPPort: 1081, Listen: "0.0.0.0"})

	ins := inbounds(t, m)
	if socks := ins["socks"]; socks == nil || intOf(socks["port"]) != 1080 || socks["listen"] != "0.0.0.0" {
		t.Errorf("socks inbound = %+v", socks)
	}
	if http := ins["http"]; http == nil || intOf(http["port"]) != 1081 {
		t.Errorf("http inbound = %+v", http)
	}
}

func TestRenderRequiresAnInbound(t *testing.T) {
	configs := fixture(t)
	if _, err := configs[0].Render(RenderOptions{}); err == nil {
		t.Fatal("expected an error when neither port is set")
	}
}

func TestRenderLeavesRoutingAlone(t *testing.T) {
	configs := fixture(t)
	before := len(sliceOf(mapOf(configs[1].raw["routing"])["rules"]))
	m := render(t, configs[1], RenderOptions{SocksPort: 1080, HTTPPort: 1081})

	if after := len(sliceOf(get(t, m, "routing.rules"))); after != before {
		t.Errorf("routing rules = %d, want %d (proxy mode keeps the panel's routing)", after, before)
	}
}

func TestRenderTunnelDirectDropsDirectRules(t *testing.T) {
	configs := fixture(t)
	m := render(t, configs[1], RenderOptions{SocksPort: 1080, TunnelDirect: true})

	rules := sliceOf(get(t, m, "routing.rules"))
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3: %v", len(rules), rules)
	}
	// The private-network rule survives: those destinations keep their own
	// routes outside the tunnel.
	if tag := str(mapOf(rules[0])["outboundTag"]); tag != "direct" {
		t.Errorf("rule 0 outboundTag = %q, want the private-network rule", tag)
	}
	for _, item := range rules[1:] {
		if tag := str(mapOf(item)["outboundTag"]); tag == "direct" {
			t.Errorf("rule %v still routes direct", item)
		}
	}
}

func TestRenderTunnelDirectDropsDirectBalancerRules(t *testing.T) {
	configs := fixture(t)
	m := render(t, configs[0], RenderOptions{SocksPort: 1080, TunnelDirect: true})

	for _, item := range sliceOf(get(t, m, "routing.rules")) {
		rule := mapOf(item)
		// lb_ru selects only the direct outbound, so rules pointing at it must go.
		if str(rule["balancerTag"]) == "lb_ru" {
			t.Errorf("rule %v still routes through the direct-only balancer", rule)
		}
	}
	// Balancers over real proxy outbounds are untouched.
	var geo bool
	for _, item := range sliceOf(get(t, m, "routing.balancers")) {
		if str(mapOf(item)["tag"]) == "lb_geo" {
			geo = true
		}
	}
	if !geo {
		t.Error("lb_geo balancer went missing")
	}
}

func TestRenderPinsAddresses(t *testing.T) {
	configs := fixture(t)
	host := configs[1].Addresses()[0]
	m := render(t, configs[1], RenderOptions{
		SocksPort: 1080,
		PinIPs:    map[string]string{host: "203.0.113.7"},
	})

	if addr := get(t, m, "outbounds.0.settings.vnext.0.address"); addr != "203.0.113.7" {
		t.Errorf("address = %v, want the pinned IP", addr)
	}
	// Reality ships its own masquerade serverName; pinning must not touch it.
	if sni := get(t, m, "outbounds.0.streamSettings.realitySettings.serverName"); sni != "intel.com" {
		t.Errorf("reality serverName = %v, want the shipped one", sni)
	}
}

func TestRenderPinKeepsTLSServerName(t *testing.T) {
	body := []byte(`{"outbounds":[{"tag":"proxy","protocol":"vless",
		"settings":{"vnext":[{"address":"a.example.com","port":443}]},
		"streamSettings":{"network":"tcp","security":"tls"}}]}`)
	configs, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	m := render(t, configs[0], RenderOptions{
		SocksPort: 1080,
		PinIPs:    map[string]string{"a.example.com": "203.0.113.7"},
	})

	if addr := get(t, m, "outbounds.0.settings.vnext.0.address"); addr != "203.0.113.7" {
		t.Errorf("address = %v", addr)
	}
	if sni := get(t, m, "outbounds.0.streamSettings.tlsSettings.serverName"); sni != "a.example.com" {
		t.Errorf("tls serverName = %v, want the original hostname", sni)
	}
}

func TestRenderDoesNotMutateSource(t *testing.T) {
	configs := fixture(t)
	before, err := configs[1].JSON()
	if err != nil {
		t.Fatalf("JSON error: %v", err)
	}
	render(t, configs[1], RenderOptions{SocksPort: 1080, TunnelDirect: true,
		PinIPs: map[string]string{configs[1].Addresses()[0]: "203.0.113.7"}})

	after, err := configs[1].JSON()
	if err != nil {
		t.Fatalf("JSON error: %v", err)
	}
	if string(before) != string(after) {
		t.Error("Render mutated the stored config")
	}
}
