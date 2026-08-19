package cli

import (
	"fmt"

	"github.com/aimuzov/proxray/internal/profile"
	"github.com/aimuzov/proxray/internal/rawconf"
	"github.com/aimuzov/proxray/internal/xray"
)

// runtimeOptions describes the config to generate for a connection.
type runtimeOptions struct {
	SocksPort int
	HTTPPort  int
	Bypass    string // "ru" or "off"; ignored for JSON subscription nodes

	// Tunnel marks TUN mode: traffic the node would send direct is routed
	// through the proxy instead, since a direct socket loops back into utun.
	Tunnel bool
	// PinIPs maps a server hostname to the IP to dial, so xray reaches exactly
	// the addresses TUN mode routed around the tunnel.
	PinIPs map[string]string
}

// buildRuntimeConfig produces the xray-core config for a node: a JSON
// subscription entry is used as the panel shipped it, a share link is turned
// into a config by our own builder.
func buildRuntimeConfig(n profile.Node, opts runtimeOptions) ([]byte, error) {
	if n.Config != nil {
		return n.Config.Render(rawconf.RenderOptions{
			LogLevel:     xrayLogLevel(verbose),
			SocksPort:    opts.SocksPort,
			HTTPPort:     opts.HTTPPort,
			TunnelDirect: opts.Tunnel,
			PinIPs:       opts.PinIPs,
		})
	}

	srv := *n.Server
	if ip, ok := opts.PinIPs[srv.Address]; ok {
		// Keep the TLS SNI on the domain while dialing the pinned address.
		if srv.SNI == "" {
			srv.SNI = srv.Address
		}
		srv.Address = ip
	}
	cfg, err := xray.BuildConfig(&srv, xray.Options{
		SocksPort: opts.SocksPort,
		HTTPPort:  opts.HTTPPort,
		Bypass:    opts.Bypass,
		LogLevel:  xrayLogLevel(verbose),
	})
	if err != nil {
		return nil, err
	}
	return cfg.JSON()
}

// nodeAddresses lists the proxy server hostnames a node dials.
func nodeAddresses(n profile.Node) []string {
	if n.Config != nil {
		return n.Config.Addresses()
	}
	return []string{n.Server.Address}
}

// resolveNodeIPs resolves every server the node dials. It returns the pins for
// the config (one address per hostname) and the full set of IPs, whose routes
// TUN mode pins to the physical gateway so the tunnel does not loop.
func resolveNodeIPs(n profile.Node) (map[string]string, []string, error) {
	pins := map[string]string{}
	var all []string
	for _, host := range nodeAddresses(n) {
		ips, err := resolveIPv4(host)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve server address %q: %w", host, err)
		}
		pins[host] = ips[0]
		all = append(all, ips...)
	}
	if len(all) == 0 {
		return nil, nil, fmt.Errorf("node %q has no server address", n.Tag)
	}
	return pins, all, nil
}
