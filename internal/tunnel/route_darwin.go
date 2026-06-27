//go:build darwin

package tunnel

import (
	"fmt"
	"os/exec"
	"strings"
)

// parseRouteGet extracts the gateway and interface from `route -n get <dest>`
// output. Either field may be empty (an interface-only default route, such as
// an already-active VPN on a utun device, has no gateway line).
func parseRouteGet(output string) (gateway, iface string) {
	for _, line := range strings.Split(output, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "gateway":
			gateway = strings.TrimSpace(val)
		case "interface":
			iface = strings.TrimSpace(val)
		}
	}
	return gateway, iface
}

// nextHop reports how the system currently routes to dest, so the proxy
// server's traffic can be pinned to that path while everything else is
// redirected into the tunnel.
func nextHop(dest string) (gateway, iface string, err error) {
	out, err := exec.Command("route", "-n", "get", dest).Output()
	if err != nil {
		return "", "", fmt.Errorf("route get %s: %w", dest, err)
	}
	gateway, iface = parseRouteGet(string(out))
	if gateway == "" && iface == "" {
		return "", "", fmt.Errorf("no route to %s", dest)
	}
	return gateway, iface, nil
}

// InterfaceTo reports the network interface the system currently uses to reach
// dest, queried before the tunnel rewrites the routing table. Reserved for a
// future tun-mode RU bypass: binding the direct outbound to this interface via
// IP_BOUND_IF was not enough on its own to keep bypassed sockets from looping
// back through utun, so tun mode currently forces bypass off. Not called yet.
func InterfaceTo(dest string) (string, error) {
	_, iface, err := nextHop(dest)
	if err != nil {
		return "", err
	}
	if iface == "" {
		return "", fmt.Errorf("no interface in route to %s", dest)
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
