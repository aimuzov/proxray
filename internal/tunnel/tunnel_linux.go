//go:build linux

package tunnel

import (
	"fmt"
	"os"
	"time"

	"github.com/xjasonlyu/tun2socks/v2/engine"

	"github.com/aimuzov/proxray/internal/log"
)

// Tunnel is a running TUN tunnel and the routing changes it installed.
type Tunnel struct {
	teardown []func()
}

// serverHop records how to reach a proxy server IP around the tunnel.
type serverHop struct {
	ip      string
	gateway string // empty for an on-link route
	iface   string
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
		gw, iface, err := nextHop(ip)
		if err != nil {
			return nil, fmt.Errorf("find route to server %s: %w", ip, err)
		}
		hops = append(hops, serverHop{ip: ip, gateway: gw, iface: iface})
	}

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

	// Pin each server IP to its current next hop (bypass the tunnel).
	for _, h := range hops {
		args := []string{"route", "replace", h.ip}
		if h.gateway != "" {
			args = append(args, "via", h.gateway)
		}
		args = append(args, "dev", h.iface)
		if err := run("ip", args...); err != nil {
			return rollback(fmt.Errorf("pin server route %s: %w", h.ip, err))
		}
		ip := h.ip
		t.teardown = append(t.teardown, func() { _ = run("ip", "route", "del", ip) })
	}

	// Override the default route with two /1 routes scoped to the tun device,
	// leaving the real default route intact for clean teardown.
	for _, cidr := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("ip", "route", "replace", cidr, "dev", opts.TunName); err != nil {
			return rollback(fmt.Errorf("install default override %s: %w", cidr, err))
		}
		cidr := cidr
		t.teardown = append(t.teardown, func() { _ = run("ip", "route", "del", cidr) })
	}

	// Block global IPv6 by routing it to the loopback. The proxy path is IPv4,
	// so without this, IPv6-capable sites connect over IPv6 outside the tunnel.
	// Link-local keeps working via its more-specific route. Best-effort.
	for _, cidr := range []string{"::/1", "8000::/1"} {
		if err := run("ip", "-6", "route", "replace", cidr, "dev", "lo"); err != nil {
			log.Warn("could not block IPv6 (possible IPv6 leak)", "cidr", cidr, "err", err)
			continue
		}
		cidr := cidr
		t.teardown = append(t.teardown, func() { _ = run("ip", "-6", "route", "del", cidr) })
	}

	return t, nil
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
