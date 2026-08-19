package profile

import (
	"fmt"

	"github.com/aimuzov/proxray/internal/link"
	"github.com/aimuzov/proxray/internal/rawconf"
)

// Node is one selectable entry of a subscription, whichever format it came in:
// a server parsed from a share link, or a ready-made xray config from a JSON
// subscription. Exactly one of Server and Config is set.
type Node struct {
	Tag       string   // display name: the link fragment or the config remarks
	Note      string   // the panel's description of the entry, if any
	Protocols []string // protocols of the proxy servers behind this entry
	Address   string   // "host:port"; empty when the entry holds several servers

	Server *link.Server
	Config *rawconf.Config
}

// NodeFromServer describes a server parsed from a share link.
func NodeFromServer(s *link.Server) Node {
	return Node{
		Tag:       s.Tag,
		Protocols: []string{s.Protocol},
		Address:   fmt.Sprintf("%s:%d", s.Address, s.Port),
		Server:    s,
	}
}

// NodeFromConfig describes an entry of a JSON subscription. Such an entry can
// carry several outbounds (a balancer pool), in which case it has no single
// address to show.
func NodeFromConfig(c *rawconf.Config) Node {
	n := Node{
		Tag:       c.Remarks(),
		Note:      c.Description(),
		Protocols: c.Protocols(),
		Config:    c,
	}
	if endpoints := c.Endpoints(); len(endpoints) == 1 {
		n.Address = fmt.Sprintf("%s:%d", endpoints[0].Address, endpoints[0].Port)
	}
	return n
}

// EndpointCount is how many proxy servers the entry can dial.
func (n Node) EndpointCount() int {
	switch {
	case n.Config != nil:
		return len(n.Config.Endpoints())
	case n.Server != nil:
		return 1
	default:
		return 0
	}
}
