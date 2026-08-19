package rawconf

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// RenderOptions are the local adjustments applied to a subscription config
// before it is handed to xray-core. Everything else — routing, balancers, dns,
// observatory — is kept exactly as the panel shipped it.
type RenderOptions struct {
	LogLevel  string // xray loglevel; default "warning"
	Listen    string // inbound listen address; default 127.0.0.1
	SocksPort int    // SOCKS inbound port; 0 drops the SOCKS inbound
	HTTPPort  int    // HTTP inbound port; 0 drops the HTTP inbound

	// TunnelDirect rewrites routing so traffic the panel sends out the direct
	// outbound goes through the proxy instead. TUN mode needs it: a direct
	// socket would loop back into the utun device and hang.
	TunnelDirect bool

	// PinIPs maps a server hostname to the IP address to dial instead. TUN mode
	// pins outbounds to the addresses whose routes it excluded from the tunnel.
	PinIPs map[string]string
}

// privateRanges are the local networks a rule may send direct even in TUN mode:
// they are reached over the physical interface's own, more specific routes.
var privateRanges = map[string]bool{
	"10.0.0.0/8":         true,
	"172.16.0.0/12":      true,
	"192.168.0.0/16":     true,
	"169.254.0.0/16":     true,
	"224.0.0.0/4":        true,
	"255.255.255.255/32": true,
	"fe80::/10":          true,
	"fc00::/7":           true,
	"ff00::/8":           true,
	"127.0.0.0/8":        true,
	"::1/128":            true,
	"geoip:private":      true,
}

// Render produces the config to feed xray-core.
func (c *Config) Render(opts RenderOptions) ([]byte, error) {
	cfg, err := c.clone()
	if err != nil {
		return nil, err
	}

	loglevel := opts.LogLevel
	if loglevel == "" {
		loglevel = "warning"
	}
	cfg["log"] = map[string]any{"loglevel": loglevel}

	if err := rewriteInbounds(cfg, opts); err != nil {
		return nil, err
	}
	if opts.TunnelDirect {
		tunnelDirectRules(cfg)
	}
	pinAddresses(cfg, opts.PinIPs)

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("rawconf: render: %w", err)
	}
	return out, nil
}

func (c *Config) clone() (map[string]any, error) {
	raw, err := json.Marshal(c.raw)
	if err != nil {
		return nil, fmt.Errorf("rawconf: clone: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("rawconf: clone: %w", err)
	}
	return out, nil
}

// rewriteInbounds points the config's SOCKS and HTTP inbounds at our ports,
// keeping their tags: routing rules select traffic by inboundTag, so renaming
// them would silently detach the panel's rules. Inbounds of other kinds (the
// dokodemo-door entries that feed loopback chains) are left untouched.
func rewriteInbounds(cfg map[string]any, opts RenderOptions) error {
	if opts.SocksPort <= 0 && opts.HTTPPort <= 0 {
		return errors.New("rawconf: no inbound configured (set SocksPort or HTTPPort)")
	}
	listen := opts.Listen
	if listen == "" {
		listen = "127.0.0.1"
	}

	socksDone, httpDone := false, false
	kept := make([]any, 0, len(sliceOf(cfg["inbounds"])))
	for _, item := range sliceOf(cfg["inbounds"]) {
		in := mapOf(item)
		port := 0
		switch {
		case in == nil:
			continue
		case str(in["protocol"]) == "socks" && !socksDone:
			socksDone, port = true, opts.SocksPort
		case str(in["protocol"]) == "http" && !httpDone:
			httpDone, port = true, opts.HTTPPort
		default:
			kept = append(kept, in)
			continue
		}
		if port <= 0 {
			continue // the caller disabled this listener
		}
		in["listen"], in["port"] = listen, port
		kept = append(kept, in)
	}

	// A config without a local listener of its own still has to be reachable.
	if !socksDone && opts.SocksPort > 0 {
		kept = append(kept, localInbound("socks", listen, opts.SocksPort,
			map[string]any{"auth": "noauth", "udp": true}))
	}
	if !httpDone && opts.HTTPPort > 0 {
		kept = append(kept, localInbound("http", listen, opts.HTTPPort, map[string]any{}))
	}

	cfg["inbounds"] = kept
	return nil
}

func localInbound(protocol, listen string, port int, settings map[string]any) map[string]any {
	return map[string]any{
		"tag":      protocol,
		"listen":   listen,
		"port":     port,
		"protocol": protocol,
		"settings": settings,
		"sniffing": map[string]any{
			"enabled":      true,
			"destOverride": []any{"http", "tls", "quic"},
		},
	}
}

// tunnelDirectRules drops the routing rules that would send traffic out the
// direct outbound, so it falls through to the rules that proxy it. Rules that
// only match private networks stay: those are LAN destinations, which keep
// their own routes outside the tunnel.
func tunnelDirectRules(cfg map[string]any) {
	routing := mapOf(cfg["routing"])
	if routing == nil {
		return
	}
	direct := directTags(cfg)
	balancers := directBalancers(routing, direct)

	kept := make([]any, 0, len(sliceOf(routing["rules"])))
	for _, item := range sliceOf(routing["rules"]) {
		rule := mapOf(item)
		if rule == nil {
			continue
		}
		goesDirect := direct[str(rule["outboundTag"])] || balancers[str(rule["balancerTag"])]
		if goesDirect && !matchesPrivateOnly(rule) {
			continue
		}
		kept = append(kept, rule)
	}
	routing["rules"] = kept
}

// directTags collects the tags of outbounds that exit through the local
// network rather than a proxy server.
func directTags(cfg map[string]any) map[string]bool {
	tags := map[string]bool{}
	for _, item := range sliceOf(cfg["outbounds"]) {
		ob := mapOf(item)
		switch str(ob["protocol"]) {
		case "freedom", "direct":
			if tag := str(ob["tag"]); tag != "" {
				tags[tag] = true
			}
		}
	}
	return tags
}

// directBalancers finds balancers that can only ever pick a direct outbound.
// A balancer selector holds tag prefixes, so "direct" also matches "direct-ru".
func directBalancers(routing map[string]any, direct map[string]bool) map[string]bool {
	out := map[string]bool{}
	for _, item := range sliceOf(routing["balancers"]) {
		b := mapOf(item)
		tag := str(b["tag"])
		if tag == "" {
			continue
		}
		selectors := stringsOf(b["selector"])
		if len(selectors) == 0 {
			continue
		}
		allDirect := true
		for _, sel := range selectors {
			if !matchesOnlyDirect(sel, direct) {
				allDirect = false
				break
			}
		}
		if allDirect {
			out[tag] = true
		}
	}
	return out
}

// matchesOnlyDirect reports whether a selector prefix matches at least one
// outbound and every outbound it matches is a direct one.
func matchesOnlyDirect(selector string, direct map[string]bool) bool {
	matched := false
	for tag := range direct {
		if strings.HasPrefix(tag, selector) {
			matched = true
		}
	}
	return matched
}

// matchesPrivateOnly reports whether a rule targets nothing but private
// networks, in which case sending it direct is still correct inside a tunnel.
func matchesPrivateOnly(rule map[string]any) bool {
	if len(stringsOf(rule["domain"])) > 0 {
		return false
	}
	ips := stringsOf(rule["ip"])
	if len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !privateRanges[ip] {
			return false
		}
	}
	return true
}

// pinAddresses replaces server hostnames with the addresses TUN mode routed
// around the tunnel, keeping the TLS SNI on the original hostname so the
// handshake still presents the name the server expects.
func pinAddresses(cfg map[string]any, pins map[string]string) {
	if len(pins) == 0 {
		return
	}
	for _, item := range sliceOf(cfg["outbounds"]) {
		ob := mapOf(item)
		if ob == nil || serviceProtocols[str(ob["protocol"])] {
			continue
		}
		for _, srv := range servers(ob) {
			host := str(srv["address"])
			ip, ok := pins[host]
			if !ok || ip == host {
				continue
			}
			srv["address"] = ip
			pinServerName(mapOf(ob["streamSettings"]), host)
		}
	}
}

func pinServerName(stream map[string]any, host string) {
	if stream == nil {
		return
	}
	var key string
	switch str(stream["security"]) {
	case "tls":
		key = "tlsSettings"
	case "reality":
		key = "realitySettings"
	default:
		return
	}
	settings := mapOf(stream[key])
	if settings == nil {
		// Without settings xray would derive the SNI from the address, which we
		// just replaced with an IP.
		settings = map[string]any{}
		stream[key] = settings
	}
	if str(settings["serverName"]) == "" {
		settings["serverName"] = host
	}
}
