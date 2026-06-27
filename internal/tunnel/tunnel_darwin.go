//go:build darwin

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
	gateway string // empty for an interface-scoped route
	iface   string
}

// Start creates the utun device, points all traffic at the SOCKS proxy via
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

	// Bring the tun interface up with a point-to-point address.
	if err := run("ifconfig", opts.TunName, opts.TunIP, opts.TunIP, "up"); err != nil {
		return rollback(fmt.Errorf("configure %s: %w", opts.TunName, err))
	}

	// Pin each server IP to its current next hop (bypass the tunnel). Replace
	// any pre-existing route so a stale entry doesn't abort setup.
	for _, h := range hops {
		_ = run("route", "delete", "-host", h.ip)
		var addErr error
		if h.gateway != "" {
			addErr = run("route", "add", "-host", h.ip, h.gateway)
		} else {
			addErr = run("route", "add", "-host", h.ip, "-interface", h.iface)
		}
		if addErr != nil {
			return rollback(fmt.Errorf("pin server route %s: %w", h.ip, addErr))
		}
		ip := h.ip
		t.teardown = append(t.teardown, func() { _ = run("route", "delete", "-host", ip) })
	}

	// Override the default route with two /1 routes scoped to the tun device,
	// leaving the real default route intact for clean teardown.
	for _, cidr := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("route", "add", "-net", cidr, "-interface", opts.TunName); err != nil {
			return rollback(fmt.Errorf("install default override %s: %w", cidr, err))
		}
		cidr := cidr
		t.teardown = append(t.teardown, func() { _ = run("route", "delete", "-net", cidr) })
	}

	// Block global IPv6 by routing it to the loopback. The proxy path is IPv4,
	// so without this, IPv6-capable sites (Google, YouTube) connect over IPv6
	// outside the tunnel and appear from the real location. Apps fall back to
	// IPv4, which goes through the tunnel. Link-local (fe80::/...) keeps working
	// via its more-specific route. Best-effort: warn but don't abort on failure.
	for _, cidr := range []string{"::/1", "8000::/1"} {
		if err := run("route", "add", "-inet6", "-net", cidr, "-interface", "lo0"); err != nil {
			log.Warn("could not block IPv6 (possible IPv6 leak)", "cidr", cidr, "err", err)
			continue
		}
		cidr := cidr
		t.teardown = append(t.teardown, func() { _ = run("route", "delete", "-inet6", "-net", cidr) })
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
