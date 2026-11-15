//go:build linux

package tunnel

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

type routeHop struct {
	gateway string
	iface   string
	onlink  bool
}

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

func nextHop(dest string) (routeHop, error) {
	return nextHopMark(dest, "")
}

func nextHopMark(dest, mark string) (routeHop, error) {
	args := []string{"route", "get", dest}
	if mark != "" {
		args = append(args, "mark", mark)
	}
	out, err := exec.Command("ip", args...).Output()
	if err != nil {
		return routeHop{}, fmt.Errorf("ip route get %s: %w", dest, err)
	}
	hop := parseIPRouteGet(string(out))
	if hop.iface == "" {
		return routeHop{}, fmt.Errorf("no route to %s", dest)
	}
	return hop, nil
}

func InterfaceTo(dest string) (string, error) {
	hop, err := nextHop(dest)
	if err != nil {
		return "", err
	}
	return hop.iface, nil
}

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

func linkIsWireGuard(output string) bool {
	return strings.Contains(output, "wireguard")
}

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

func parseWGFwmark(output string) string {
	s := strings.TrimSpace(output)
	if s == "" || s == "off" || s == "0" {
		return ""
	}
	return s
}

func wireguardFwmark(iface string) (string, error) {
	out, err := exec.Command("wg", "show", iface, "fwmark").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("wg show %s fwmark: %w: %s", iface, err, strings.TrimSpace(string(out)))
	}
	return parseWGFwmark(string(out)), nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
