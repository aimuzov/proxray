//go:build linux

package tunnel

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// routeHop is how the kernel currently reaches an address.
type routeHop struct {
	gateway string // empty for an on-link route
	iface   string
	onlink  bool // gateway is not covered by a link-scope route
}

// parseIPRouteGet extracts the gateway, interface, and onlink flag from
// `ip route get <dest>` output.
func parseIPRouteGet(output string) routeHop {
	var hop routeHop
	line, _, _ := strings.Cut(output, "\n")
	fields := strings.Fields(line)
	for i := 0; i < len(fields); i++ {
		switch fields[i] {
		case "via":
			if i+1 < len(fields) {
				hop.gateway = fields[i+1]
			}
		case "dev":
			if i+1 < len(fields) {
				hop.iface = fields[i+1]
			}
		case "onlink":
			hop.onlink = true
		}
	}
	return hop
}

// nextHop reports how the system currently routes to dest, so the proxy
// server's traffic can be pinned to that path while everything else is
// redirected into the tunnel.
func nextHop(dest string) (routeHop, error) {
	out, err := exec.Command("ip", "route", "get", dest).Output()
	if err != nil {
		return routeHop{}, fmt.Errorf("ip route get %s: %w", dest, err)
	}
	hop := parseIPRouteGet(string(out))
	if hop.iface == "" {
		return routeHop{}, fmt.Errorf("no route to %s", dest)
	}
	return hop, nil
}

// InterfaceTo reports the network interface the system currently uses to reach
// dest, queried before the tunnel rewrites the routing table.
func InterfaceTo(dest string) (string, error) {
	hop, err := nextHop(dest)
	if err != nil {
		return "", err
	}
	return hop.iface, nil
}

// parseWGEndpoints lists peer endpoint IPs from `wg show <iface> endpoints`.
// Lines look like "<public-key>\t<ip>:<port>" or "<key>\t(none)".
func parseWGEndpoints(output string) []string {
	var ips []string
	seen := map[string]struct{}{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[1] == "(none)" {
			continue
		}
		host, _, err := net.SplitHostPort(fields[1])
		if err != nil {
			continue
		}
		host = strings.Trim(host, "[]")
		if net.ParseIP(host) == nil {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		ips = append(ips, host)
	}
	return ips
}

// linkIsWireGuard reports whether `ip -d link show` output describes a
// WireGuard device.
func linkIsWireGuard(output string) bool {
	return strings.Contains(output, "wireguard")
}

// wireguardEndpoints returns the peer endpoint IPs of iface when it is a
// WireGuard device. A non-WireGuard interface yields a nil slice.
func wireguardEndpoints(iface string) ([]string, error) {
	link, err := exec.Command("ip", "-d", "link", "show", "dev", iface).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ip -d link show %s: %w: %s", iface, err, strings.TrimSpace(string(link)))
	}
	if !linkIsWireGuard(string(link)) {
		return nil, nil
	}
	out, err := exec.Command("wg", "show", iface, "endpoints").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("interface %s is WireGuard but its endpoints could not be read (is wireguard-tools installed?): %w: %s", iface, err, strings.TrimSpace(string(out)))
	}
	return parseWGEndpoints(string(out)), nil
}

// run executes a command, wrapping failures with the command line for context.
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
