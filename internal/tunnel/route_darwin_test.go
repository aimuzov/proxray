//go:build darwin

package tunnel

import "testing"

func TestParseRouteGetWithGateway(t *testing.T) {
	output := `   route to: 1.2.3.4
destination: default
       mask: default
    gateway: 192.168.1.1
  interface: en0
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING>
`
	gw, iface := parseRouteGet(output)
	if gw != "192.168.1.1" {
		t.Errorf("gateway = %q, want 192.168.1.1", gw)
	}
	if iface != "en0" {
		t.Errorf("interface = %q, want en0", iface)
	}
}

func TestParseRouteGetInterfaceOnly(t *testing.T) {
	output := `   route to: 1.2.3.4
destination: default
       mask: default
  interface: utun12
      flags: <UP,DONE,CLONING,STATIC,GLOBAL>
`
	gw, iface := parseRouteGet(output)
	if gw != "" {
		t.Errorf("gateway = %q, want empty", gw)
	}
	if iface != "utun12" {
		t.Errorf("interface = %q, want utun12", iface)
	}
}
