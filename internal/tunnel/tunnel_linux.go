//go:build linux

package tunnel

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xjasonlyu/tun2socks/v2/engine"

	"github.com/aimuzov/proxray/internal/log"
)

// Tunnel is a running TUN tunnel and the routing changes it installed.
type Tunnel struct {
	teardown []func()
}

// serverHop records how to reach an IP around the tunnel.
type serverHop struct {
	ip string
	routeHop
}

// Start creates the tun device, points all traffic at the SOCKS proxy via
// tun2socks, and installs the routing changes. It requires root.
func Start(opts Options) (*Tunnel, error) {
	opts.withDefaults()

	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("TUN mode requires root; re-run with sudo")
	}
	if len(opts.ServerIPs) == 0 {
		return nil, fmt.Errorf("TUN mode requires the resolved server IP(s) to pin around the tunnel")
	}

	// Discover the current next hop to each server IP before touching routes,
	// so the proxy's own connection keeps using whatever path works now (a
	// physical gateway, or an already-active VPN interface).
	hops := make([]serverHop, 0, len(opts.ServerIPs))
	for _, ip := range opts.ServerIPs {
		hop, err := nextHop(ip)
		if err != nil {
			return nil, fmt.Errorf("find route to server %s: %w", ip, err)
		}
		hops = append(hops, serverHop{ip: ip, routeHop: hop})
	}
	wgHops, err := wireguardBypassHops(hops)
	if err != nil {
		return nil, err
	}
	hops = append(hops, wgHops...)

	key := &engine.Key{
		Proxy:      "socks5://" + opts.SocksAddr,
		Device:     "tun://" + opts.TunName,
		LogLevel:   opts.LogLevel,
		MTU:        opts.MTU,
		UDPTimeout: 30 * time.Second,
	}
	engine.Insert(key)
	engine.Start()

	t := &Tunnel{}
	t.teardown = append(t.teardown, func() { engine.Stop() })

	rollback := func(cause error) (*Tunnel, error) {
		t.Close()
		return nil, cause
	}

	// Address the interface and bring it up. 198.18.0.0/15 is the benchmarking
	// range tun2socks uses as a non-routable local endpoint.
	if err := run("ip", "addr", "replace", opts.TunIP+"/15", "dev", opts.TunName); err != nil {
		return rollback(fmt.Errorf("configure %s: %w", opts.TunName, err))
	}
	if err := run("ip", "link", "set", "dev", opts.TunName, "up"); err != nil {
		return rollback(fmt.Errorf("bring up %s: %w", opts.TunName, err))
	}

	// Pin each server IP (and any WireGuard endpoint it rides on) to its
	// current next hop. `ip route add` fails if that prefix already exists, so
	// a route another VPN installed is left untouched and setup aborts.
	for _, h := range hops {
		if err := addHostRoute(h); err != nil {
			return rollback(fmt.Errorf("pin route %s: %w", h.ip, err))
		}
		ip := h.ip
		t.teardown = append(t.teardown, func() { _ = delHostRoute(ip) })
	}

	// Override the default route with two /1 routes scoped to the tun device,
	// leaving the real default route intact for clean teardown.
	for _, cidr := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("ip", "route", "add", cidr, "dev", opts.TunName); err != nil {
			return rollback(fmt.Errorf("install default override %s: %w", cidr, err))
		}
		cidr := cidr
		t.teardown = append(t.teardown, func() { _ = run("ip", "route", "del", cidr) })
	}

	// Block global IPv6 by routing it to the loopback. The proxy path is IPv4,
	// so without this, IPv6-capable sites connect over IPv6 outside the tunnel.
	// Link-local keeps working via its more-specific route. Best-effort: a
	// failure is logged and startup continues, which can leave an IPv6 leak.
	for _, cidr := range []string{"::/1", "8000::/1"} {
		if err := run("ip", "-6", "route", "add", cidr, "dev", "lo"); err != nil {
			log.Warn("could not block IPv6 (possible IPv6 leak)", "cidr", cidr, "err", err)
			continue
		}
		cidr := cidr
		t.teardown = append(t.teardown, func() { _ = run("ip", "-6", "route", "del", cidr) })
	}

	return t, nil
}

// wireguardBypassHops pins the UDP endpoints of a WireGuard device that the
// proxy server is reached through. Without those routes, the /1 overrides
// capture the encapsulated packets and loop them back into the tunnel.
func wireguardBypassHops(hops []serverHop) ([]serverHop, error) {
	seenIface := map[string]struct{}{}
	seenIP := map[string]struct{}{}
	for _, h := range hops {
		seenIP[h.ip] = struct{}{}
	}
	var out []serverHop
	for _, h := range hops {
		if _, ok := seenIface[h.iface]; ok {
			continue
		}
		seenIface[h.iface] = struct{}{}
		endpoints, err := wireguardEndpoints(h.iface)
		if err != nil {
			return nil, err
		}
		for _, ip := range endpoints {
			if _, ok := seenIP[ip]; ok {
				continue
			}
			seenIP[ip] = struct{}{}
			hop, err := nextHop(ip)
			if err != nil {
				return nil, fmt.Errorf("wireguard %s endpoint %s: %w", h.iface, ip, err)
			}
			if hop.iface == h.iface {
				return nil, fmt.Errorf("wireguard %s endpoint %s is only reachable through %s; disconnect that VPN before TUN mode", h.iface, ip, h.iface)
			}
			out = append(out, serverHop{ip: ip, routeHop: hop})
		}
	}
	return out, nil
}

func addHostRoute(h serverHop) error {
	return run("ip", hostRouteArgs("add", h)...)
}

func delHostRoute(ip string) error {
	return run("ip", hostRouteArgs("del", serverHop{ip: ip})...)
}

func hostRouteArgs(action string, h serverHop) []string {
	args := []string{}
	if strings.Contains(h.ip, ":") {
		args = append(args, "-6")
	}
	args = append(args, "route", action, h.ip)
	if action == "add" {
		if h.gateway != "" {
			args = append(args, "via", h.gateway)
		}
		if h.iface != "" {
			args = append(args, "dev", h.iface)
		}
		if h.onlink {
			args = append(args, "onlink")
		}
	}
	return args
}

// Close stops the tunnel and reverses the routing changes in reverse order.
func (t *Tunnel) Close() error {
	if t == nil {
		return nil
	}
	for i := len(t.teardown) - 1; i >= 0; i-- {
		t.teardown[i]()
	}
	t.teardown = nil
	return nil
}
