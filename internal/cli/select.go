package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aimuzov/proxray/internal/link"
)

// selectServer picks a server from the list by selector:
//   - "" selects the first server;
//   - a number selects by 1-based index;
//   - anything else is a case-insensitive substring match on the tag.
//
// It returns the server and its 0-based index.
func selectServer(servers []*link.Server, selector string) (*link.Server, int, error) {
	if len(servers) == 0 {
		return nil, 0, fmt.Errorf("no servers available (add a subscription first)")
	}

	selector = strings.TrimSpace(selector)
	if selector == "" {
		return servers[0], 0, nil
	}

	if n, err := strconv.Atoi(selector); err == nil {
		if n < 1 || n > len(servers) {
			return nil, 0, fmt.Errorf("server index %d out of range (1..%d)", n, len(servers))
		}
		return servers[n-1], n - 1, nil
	}

	needle := strings.ToLower(selector)
	for i, s := range servers {
		if strings.Contains(strings.ToLower(s.Tag), needle) {
			return s, i, nil
		}
	}
	return nil, 0, fmt.Errorf("no server matches %q", selector)
}
