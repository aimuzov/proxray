//go:build linux

package tunnel

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/xjasonlyu/tun2socks/v2/engine"
)

type Tunnel struct {
	teardown []func()
}

type serverHop struct {
	ip string
	routeHop
}

func Start(opts Options) (*Tunnel, error) {
	opts.withDefaults()

	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("TUN mode requires root; re-run with sudo")
	}
	if len(opts.ServerIPs) == 0 {
		return nil, fmt.Errorf("TUN mode requires the resolved server IP(s) to pin around the tunnel")
	}

	hops := make([]serverHop, 0, len(opts.ServerIPs))
	seenServer := map[string]struct{}{}
	for _, ip := range opts.ServerIPs {
		if _, ok := seenServer[ip]; ok {
			continue
		}
		seenServer[ip] = struct{}{}

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

	if err := run("ip", "addr", "replace", opts.TunIP+"/32", "dev", opts.TunName); err != nil {
		return rollback(fmt.Errorf("configure %s: %w", opts.TunName, err))
	}
	if err := run("ip", "link", "set", "dev", opts.TunName, "up"); err != nil {
		return rollback(fmt.Errorf("bring up %s: %w", opts.TunName, err))
	}

	for _, h := range hops {
		undo, err := installHostRoute(h)
		if err != nil {
			return rollback(fmt.Errorf("pin route %s: %w", h.ip, err))
		}
		t.teardown = append(t.teardown, undo)
	}

	for _, cidr := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		undo, err := installDevRoute(cidr, opts.TunName)
		if err != nil {
			return rollback(fmt.Errorf("install default override %s: %w", cidr, err))
		}
		t.teardown = append(t.teardown, undo)
	}

	for _, cidr := range []string{"::/1", "8000::/1"} {
		undo, err := installBlackhole(cidr)
		if err != nil {
			return rollback(fmt.Errorf("block IPv6 %s: %w", cidr, err))
		}
		t.teardown = append(t.teardown, undo)
	}

	return t, nil
}

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
		if len(endpoints) == 0 {
			continue
		}
		mark, err := wireguardFwmark(h.iface)
		if err != nil {
			return nil, err
		}
		for _, ip := range endpoints {
			if _, ok := seenIP[ip]; ok {
				continue
			}
			seenIP[ip] = struct{}{}
			hop, err := nextHopMark(ip, mark)
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

func installHostRoute(h serverHop) (func(), error) {
	existing, err := showRoute(h.ip, strings.Contains(h.ip, ":"))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(existing) != "" {
		if !hostRouteMatches(existing, h) {
			return nil, fmt.Errorf("route %s already exists and does not match this tunnel: %s", h.ip, strings.TrimSpace(existing))
		}
		if err := delHostRoute(h); err != nil {
			return nil, err
		}
	}
	if err := run("ip", hostRouteArgs("add", h)...); err != nil {
		return nil, err
	}
	saved := h
	return func() { _ = delHostRoute(saved) }, nil
}

func installDevRoute(cidr, dev string) (func(), error) {
	existing, err := showRoute(cidr, false)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(existing) != "" {
		if !strings.Contains(existing, "dev "+dev) {
			return nil, fmt.Errorf("route %s already exists on another device: %s", cidr, strings.TrimSpace(existing))
		}
		if err := run("ip", "route", "del", cidr, "dev", dev); err != nil {
			return nil, err
		}
	}
	if err := run("ip", "route", "add", cidr, "dev", dev); err != nil {
		return nil, err
	}
	return func() { _ = run("ip", "route", "del", cidr, "dev", dev) }, nil
}

func installBlackhole(cidr string) (func(), error) {
	existing, err := showRoute(cidr, true)
	if err != nil {
		return nil, err
	}
	if text := strings.TrimSpace(existing); text != "" {
		if !strings.Contains(text, "blackhole") {
			return nil, fmt.Errorf("IPv6 route %s already exists and is not a blackhole: %s", cidr, text)
		}
		if err := run("ip", "-6", "route", "del", "blackhole", cidr); err != nil {
			return nil, err
		}
	}
	if err := run("ip", "-6", "route", "add", "blackhole", cidr); err != nil {
		return nil, err
	}
	return func() { _ = run("ip", "-6", "route", "del", "blackhole", cidr) }, nil
}

func showRoute(prefix string, v6 bool) (string, error) {
	args := []string{"route", "show", prefix}
	if v6 {
		args = []string{"-6", "route", "show", prefix}
	}
	out, err := exec.Command("ip", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ip %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func hostRouteMatches(existing string, h serverHop) bool {
	line, _, _ := strings.Cut(existing, "\n")
	line = strings.TrimSpace(line)
	if h.gateway != "" && !strings.Contains(line, "via "+h.gateway) {
		return false
	}
	if h.gateway == "" && strings.Contains(line, " via ") {
		return false
	}
	if h.iface != "" && !strings.Contains(line, "dev "+h.iface) {
		return false
	}
	if h.src != "" && !strings.Contains(line, "src "+h.src) {
		return false
	}
	if h.onlink != strings.Contains(line, "onlink") {
		return false
	}
	return line != ""
}

func delHostRoute(h serverHop) error {
	return run("ip", hostRouteArgs("del", h)...)
}

func hostRouteArgs(action string, h serverHop) []string {
	args := []string{}
	if strings.Contains(h.ip, ":") {
		args = append(args, "-6")
	}
	args = append(args, "route", action, h.ip)
	if h.gateway != "" {
		args = append(args, "via", h.gateway)
	}
	if h.iface != "" {
		args = append(args, "dev", h.iface)
	}
	if action == "add" {
		if h.src != "" {
			args = append(args, "src", h.src)
		}
		if h.onlink {
			args = append(args, "onlink")
		}
	}
	return args
}

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
