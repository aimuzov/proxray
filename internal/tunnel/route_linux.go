//go:build linux

package tunnel

import (
	"fmt"
	"os/exec"
	"strings"
)

// parseIPRouteGet extracts the gateway and interface from `ip route get <dest>`
// output. Either field may be empty: an on-link or interface-scoped route has
// no "via".
func parseIPRouteGet(output string) (gateway, iface string) {
	line, _, _ := strings.Cut(output, "\n")
	fields := strings.Fields(line)
	for i := 0; i < len(fields)-1; i++ {
		switch fields[i] {
		case "via":
			gateway = fields[i+1]
		case "dev":
			iface = fields[i+1]
		}
	}
	return gateway, iface
}

// nextHop reports how the system currently routes to dest, so the proxy
// server's traffic can be pinned to that path while everything else is
// redirected into the tunnel.
func nextHop(dest string) (gateway, iface string, err error) {
	out, err := exec.Command("ip", "route", "get", dest).Output()
	if err != nil {
		return "", "", fmt.Errorf("ip route get %s: %w", dest, err)
	}
	gateway, iface = parseIPRouteGet(string(out))
	if iface == "" {
		return "", "", fmt.Errorf("no route to %s", dest)
	}
	return gateway, iface, nil
}

// InterfaceTo reports the network interface the system currently uses to reach
// dest, queried before the tunnel rewrites the routing table.
func InterfaceTo(dest string) (string, error) {
	_, iface, err := nextHop(dest)
	if err != nil {
		return "", err
	}
	return iface, nil
}

// run executes a command, wrapping failures with the command line for context.
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
