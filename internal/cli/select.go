package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aimuzov/proxray/internal/profile"
	"github.com/aimuzov/proxray/internal/xray"
)

// selectNode picks a node from the list by selector:
//   - "" selects the first node;
//   - a number selects by 1-based index;
//   - anything else is a case-insensitive substring match on the tag.
//
// It returns the node and its 0-based index.
func selectNode(nodes []profile.Node, selector string) (profile.Node, int, error) {
	if len(nodes) == 0 {
		return profile.Node{}, 0, fmt.Errorf("no servers available (add a subscription first)")
	}

	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nodes[0], 0, nil
	}

	if n, err := strconv.Atoi(selector); err == nil {
		if n < 1 || n > len(nodes) {
			return profile.Node{}, 0, fmt.Errorf("server index %d out of range (1..%d)", n, len(nodes))
		}
		return nodes[n-1], n - 1, nil
	}

	needle := strings.ToLower(selector)
	for i, n := range nodes {
		if strings.Contains(strings.ToLower(n.Tag), needle) {
			return n, i, nil
		}
	}
	return profile.Node{}, 0, fmt.Errorf("no server matches %q", selector)
}

// supported reports whether xray-core can dial every protocol the node uses. A
// JSON subscription entry may pool several outbounds, and one unsupported
// protocol among them is enough to make the whole config unloadable.
func supported(n profile.Node) bool {
	if n.Server != nil {
		return xray.SupportedServer(n.Server)
	}
	if len(n.Protocols) == 0 {
		return false
	}
	for _, p := range n.Protocols {
		if !xray.Supported(p) {
			return false
		}
	}
	return true
}

// protocolList renders the node's protocols for display, e.g. "vless" or
// "vless+trojan".
func protocolList(n profile.Node) string {
	if len(n.Protocols) == 0 {
		return "unknown"
	}
	return strings.Join(n.Protocols, "+")
}

// endpointSummary describes where a node connects: a single address, or the
// number of servers pooled behind a JSON subscription entry.
func endpointSummary(n profile.Node) string {
	if n.Address != "" {
		return n.Address
	}
	return fmt.Sprintf("%d servers", n.EndpointCount())
}
