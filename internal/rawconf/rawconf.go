// Package rawconf handles JSON subscriptions: bodies that carry ready-made
// xray-core configurations instead of share links. Each entry is a complete
// config — inbounds, outbounds, routing, balancers — curated by the panel, so
// it is kept verbatim and only adjusted where the local client must have a say
// (listen ports, log level, TUN mode). The package deliberately has no
// dependency on xray-core; it manipulates the decoded JSON tree.
package rawconf

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrNotJSON reports that a body is not a JSON subscription, so the caller can
// fall back to parsing it as a list of share links.
var ErrNotJSON = errors.New("rawconf: not a JSON document")

// serviceProtocols are outbounds that do not dial a proxy server: they either
// exit locally, drop traffic, or re-inject it into routing.
var serviceProtocols = map[string]bool{
	"freedom":   true,
	"direct":    true,
	"blackhole": true,
	"block":     true,
	"loopback":  true,
	"dns":       true,
}

// Config is one entry of a JSON subscription: a full xray-core config plus the
// display metadata (remarks, meta) that clients show in their server list.
type Config struct {
	raw map[string]any
}

// Endpoint is one dialable proxy server described by an outbound.
type Endpoint struct {
	Tag      string
	Protocol string
	Address  string
	Port     int
}

// Parse decodes a JSON subscription body: either an array of configs (the
// common form) or a single config object. It returns ErrNotJSON for a body that
// does not even look like JSON.
func Parse(body []byte) ([]*Config, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || (trimmed[0] != '[' && trimmed[0] != '{') {
		return nil, ErrNotJSON
	}

	var raws []json.RawMessage
	if trimmed[0] == '[' {
		if err := json.Unmarshal([]byte(trimmed), &raws); err != nil {
			return nil, fmt.Errorf("rawconf: parse subscription: %w", err)
		}
	} else {
		raws = []json.RawMessage{json.RawMessage(trimmed)}
	}

	configs := make([]*Config, 0, len(raws))
	for i, raw := range raws {
		cfg, err := parseOne(raw)
		if err != nil {
			return nil, fmt.Errorf("rawconf: entry %d: %w", i+1, err)
		}
		configs = append(configs, cfg)
	}
	if len(configs) == 0 {
		return nil, errors.New("rawconf: subscription contains no configs")
	}
	return configs, nil
}

func parseOne(raw json.RawMessage) (*Config, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if len(sliceOf(m["outbounds"])) == 0 {
		return nil, errors.New("no outbounds (not an xray-core config)")
	}
	return &Config{raw: m}, nil
}

// JSON returns the config as stored, without any local adjustments.
func (c *Config) JSON() ([]byte, error) { return json.Marshal(c.raw) }

// Remarks is the display name of the entry.
func (c *Config) Remarks() string { return str(c.raw["remarks"]) }

// Description is the panel's short note about the entry, if any.
func (c *Config) Description() string {
	return str(mapOf(c.raw["meta"])["serverDescription"])
}

// Endpoints lists the proxy servers the config can dial, in outbound order.
func (c *Config) Endpoints() []Endpoint {
	var out []Endpoint
	for _, o := range sliceOf(c.raw["outbounds"]) {
		ob := mapOf(o)
		protocol := str(ob["protocol"])
		if protocol == "" || serviceProtocols[protocol] {
			continue
		}
		tag := str(ob["tag"])
		for _, srv := range servers(ob) {
			out = append(out, Endpoint{
				Tag:      tag,
				Protocol: protocol,
				Address:  str(srv["address"]),
				Port:     intOf(srv["port"]),
			})
		}
	}
	return out
}

// Protocols lists the distinct protocols of the dialable outbounds, in the
// order they first appear.
func (c *Config) Protocols() []string {
	var out []string
	seen := map[string]bool{}
	for _, e := range c.Endpoints() {
		if !seen[e.Protocol] {
			seen[e.Protocol] = true
			out = append(out, e.Protocol)
		}
	}
	return out
}

// Addresses lists the distinct server hostnames the config dials. TUN mode
// resolves them so their routes can be pinned to the physical gateway.
func (c *Config) Addresses() []string {
	var out []string
	seen := map[string]bool{}
	for _, e := range c.Endpoints() {
		if e.Address != "" && !seen[e.Address] {
			seen[e.Address] = true
			out = append(out, e.Address)
		}
	}
	return out
}

// UsesGeoAssets reports whether any routing rule references the geoip/geosite
// databases, which xray-core loads from disk at startup.
func (c *Config) UsesGeoAssets() bool {
	raw, err := json.Marshal(c.raw)
	if err != nil {
		return false
	}
	return strings.Contains(string(raw), "geoip:") || strings.Contains(string(raw), "geosite:")
}

// servers returns the address-carrying maps of an outbound. xray spells them
// three ways: vnext (vless/vmess), servers (trojan/shadowsocks/socks/http), and
// a bare address/port pair (hysteria).
func servers(outbound map[string]any) []map[string]any {
	settings := mapOf(outbound["settings"])
	var out []map[string]any
	for _, key := range []string{"vnext", "servers"} {
		for _, v := range sliceOf(settings[key]) {
			if m := mapOf(v); m != nil {
				out = append(out, m)
			}
		}
	}
	if len(out) == 0 && settings["address"] != nil {
		out = append(out, settings)
	}
	return out
}

func mapOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func sliceOf(v any) []any {
	s, _ := v.([]any)
	return s
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// intOf reads a JSON number, which decodes into float64 through any.
func intOf(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

// strings returns the value of a field that xray accepts as a list of strings.
func stringsOf(v any) []string {
	var out []string
	for _, item := range sliceOf(v) {
		if s := str(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}
