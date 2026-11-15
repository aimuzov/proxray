//go:build linux

package tunnel

import "testing"

func TestParseIPRouteGetVia(t *testing.T) {
	output := "8.8.8.8 via 10.0.2.2 dev eth0 src 10.0.2.15 uid 0 \n    cache\n"
	gw, iface := parseIPRouteGet(output)
	if gw != "10.0.2.2" {
		t.Errorf("gateway = %q, want 10.0.2.2", gw)
	}
	if iface != "eth0" {
		t.Errorf("interface = %q, want eth0", iface)
	}
}

func TestParseIPRouteGetOnLink(t *testing.T) {
	output := "8.8.8.8 dev wg0 src 10.7.0.2 uid 0\n"
	gw, iface := parseIPRouteGet(output)
	if gw != "" {
		t.Errorf("gateway = %q, want empty", gw)
	}
	if iface != "wg0" {
		t.Errorf("interface = %q, want wg0", iface)
	}
}
