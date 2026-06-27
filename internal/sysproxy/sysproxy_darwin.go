//go:build darwin

// Package sysproxy sets and restores the macOS system proxy via networksetup,
// so apps that honor the system proxy (browsers) route through proxray without a
// TUN device. Changing the system proxy requires root.
package sysproxy

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Restore reverts the system proxy to its previous state.
type Restore func() error

type proxyState struct {
	Enabled bool
	Server  string
	Port    int
}

// kind is one networksetup proxy type and the port proxray exposes for it. The
// token drives all three commands: -get<token>, -set<token>, -set<token>state.
type kind struct {
	token string
	port  int
}

// proxyKinds maps proxray's local inbounds to the macOS proxy types: SOCKS to the
// SOCKS port, and both HTTP (web) and HTTPS (secure web) to the HTTP port.
func proxyKinds(socksPort, httpPort int) []kind {
	return []kind{
		{token: "socksfirewallproxy", port: socksPort},
		{token: "webproxy", port: httpPort},
		{token: "securewebproxy", port: httpPort},
	}
}

// allTokens lists every proxy type proxray manages, for a blanket disable.
var allTokens = []string{"socksfirewallproxy", "webproxy", "securewebproxy"}

// parseServices extracts enabled network service names from the output of
// `networksetup -listallnetworkservices` (the header line and disabled
// services prefixed with "*" are skipped).
func parseServices(output string) []string {
	var services []string
	for i, line := range strings.Split(output, "\n") {
		if i == 0 && strings.HasPrefix(line, "An asterisk") {
			continue // header
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") {
			continue // blank or disabled
		}
		services = append(services, line)
	}
	return services
}

// parseProxyState parses the output of `networksetup -get<kind>proxy <service>`.
func parseProxyState(output string) proxyState {
	var st proxyState
	for _, line := range strings.Split(output, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "Enabled":
			st.Enabled = strings.EqualFold(val, "Yes")
		case "Server":
			st.Server = val
		case "Port":
			st.Port, _ = strconv.Atoi(val)
		}
	}
	return st
}

// Enable points the system SOCKS proxy at host:socksPort and the HTTP/HTTPS
// proxies at host:httpPort on every enabled network service, returning a
// Restore that reverts each one to its prior state.
func Enable(host string, socksPort, httpPort int) (Restore, error) {
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("setting the system proxy requires root; re-run with sudo")
	}

	services := parseServices(runOut("networksetup", "-listallnetworkservices"))
	if len(services) == 0 {
		return nil, fmt.Errorf("no enabled network services found")
	}
	kinds := proxyKinds(socksPort, httpPort)

	var restores []func() error
	revert := func() error {
		var firstErr error
		for i := len(restores) - 1; i >= 0; i-- {
			if err := restores[i](); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	for _, svc := range services {
		for _, k := range kinds {
			prev := parseProxyState(runOut("networksetup", "-get"+k.token, svc))
			if err := setProxy(svc, k.token, host, k.port); err != nil {
				_ = revert()
				return nil, fmt.Errorf("set %s on %q: %w", k.token, svc, err)
			}
			svc, token, prev := svc, k.token, prev
			restores = append(restores, func() error { return restoreProxy(svc, token, prev) })
		}
	}

	return revert, nil
}

// DisableAll turns off every proxy type proxray manages on every enabled network
// service. Used as an emergency reset when a prior session did not restore.
func DisableAll() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("changing the system proxy requires root; re-run with sudo")
	}
	services := parseServices(runOut("networksetup", "-listallnetworkservices"))
	if len(services) == 0 {
		return fmt.Errorf("no enabled network services found")
	}
	var firstErr error
	for _, svc := range services {
		for _, token := range allTokens {
			if err := run("networksetup", "-set"+token+"state", svc, "off"); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func setProxy(service, token, host string, port int) error {
	if err := run("networksetup", "-set"+token, service, host, strconv.Itoa(port)); err != nil {
		return err
	}
	return run("networksetup", "-set"+token+"state", service, "on")
}

func restoreProxy(service, token string, prev proxyState) error {
	if prev.Enabled && prev.Server != "" {
		if err := run("networksetup", "-set"+token, service, prev.Server, strconv.Itoa(prev.Port)); err != nil {
			return err
		}
		return run("networksetup", "-set"+token+"state", service, "on")
	}
	return run("networksetup", "-set"+token+"state", service, "off")
}

func run(name string, args ...string) error {
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runOut(name string, args ...string) string {
	out, _ := exec.Command(name, args...).Output()
	return string(out)
}
