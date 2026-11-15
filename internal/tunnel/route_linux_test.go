//go:build linux

package tunnel

import (
	"strings"
	"testing"
)

func TestParseIPRouteGetVia(t *testing.T) {
	output := "8.8.8.8 via 10.0.2.2 dev eth0 src 10.0.2.15 uid 0 \n    cache\n"
	hop := parseIPRouteGet(output)
	if hop.gateway != "10.0.2.2" {
		t.Errorf("gateway = %q, want 10.0.2.2", hop.gateway)
	}
	if hop.iface != "eth0" {
		t.Errorf("interface = %q, want eth0", hop.iface)
	}
	if hop.onlink {
		t.Error("onlink = true, want false")
	}
	if hop.src != "10.0.2.15" {
		t.Errorf("src = %q, want 10.0.2.15", hop.src)
	}
}

func TestParseIPRouteGetOnLinkFlag(t *testing.T) {
	output := "1.2.3.4 via 10.9.0.1 dev eth0 src 10.0.0.8 uid 0 onlink\n"
	hop := parseIPRouteGet(output)
	if hop.gateway != "10.9.0.1" || hop.iface != "eth0" || !hop.onlink {
		t.Fatalf("hop = %+v", hop)
	}
	args := hostRouteArgs("add", serverHop{ip: "1.2.3.4", routeHop: hop})
	want := "route add 1.2.3.4 via 10.9.0.1 dev eth0 src 10.0.0.8 onlink"
	if got := strings.Join(args, " "); got != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

func TestParseIPRouteGetOnLink(t *testing.T) {
	output := "8.8.8.8 dev wg0 src 10.7.0.2 uid 0\n"
	hop := parseIPRouteGet(output)
	if hop.gateway != "" {
		t.Errorf("gateway = %q, want empty", hop.gateway)
	}
	if hop.iface != "wg0" {
		t.Errorf("interface = %q, want wg0", hop.iface)
	}
}

func TestParseWGEndpoints(t *testing.T) {
	output := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEF=\t203.0.113.5:51820\n" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb=\t(none)\n" +
		"cccccccccccccccccccccccccccccccccccccccccccc=\t[2001:db8::1]:51820\n"
	got, err := parseWGEndpoints(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "203.0.113.5" || got[1] != "2001:db8::1" {
		t.Fatalf("endpoints = %#v", got)
	}
	if _, err := parseWGEndpoints("k=\tvpn.example:51820\n"); err == nil {
		t.Fatal("hostname endpoint was accepted")
	}
}

func TestParseWGFwmark(t *testing.T) {
	if parseWGFwmark("off\n") != "" || parseWGFwmark("0") != "" || parseWGFwmark("51820\n") != "51820" {
		t.Fatal("fwmark parse mismatch")
	}
}

func TestLinkIsWireGuard(t *testing.T) {
	if linkIsWireGuard("2: wireguard0: <BROADCAST>\n    link/ether aa:bb") {
		t.Fatal("interface name reported as wireguard")
	}
	if !linkIsWireGuard("3: wg0: <POINTOPOINT,NOARP>\n    link/none\n    wireguard\n") {
		t.Fatal("wireguard link not detected")
	}
}

func TestHostRouteMatches(t *testing.T) {
	h := serverHop{ip: "1.2.3.4", routeHop: routeHop{gateway: "10.0.0.1", iface: "eth0", src: "10.0.0.8"}}
	if !hostRouteMatches("1.2.3.4 via 10.0.0.1 dev eth0 src 10.0.0.8\n", h) {
		t.Fatal("same hop did not match")
	}
	if hostRouteMatches("1.2.3.4 via 10.0.0.1 dev eth1 src 10.0.0.8\n", h) {
		t.Fatal("different device matched")
	}
}
