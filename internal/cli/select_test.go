package cli

import (
	"testing"

	"github.com/aimuzov/proxray/internal/link"
	"github.com/aimuzov/proxray/internal/profile"
	"github.com/aimuzov/proxray/internal/rawconf"
)

func nodes() []profile.Node {
	return []profile.Node{
		profile.NodeFromServer(&link.Server{Tag: "Netherlands #1", Protocol: "vless"}),
		profile.NodeFromServer(&link.Server{Tag: "Germany Frankfurt", Protocol: "trojan"}),
		profile.NodeFromServer(&link.Server{Tag: "USA West", Protocol: "vmess"}),
	}
}

// configNode builds a node backed by a JSON subscription entry with the given
// outbounds, expressed as protocol/address pairs.
func configNode(t *testing.T, body string) profile.Node {
	t.Helper()
	configs, err := rawconf.Parse([]byte(body))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return profile.NodeFromConfig(configs[0])
}

func TestSelectNodeDefaultsToFirst(t *testing.T) {
	n, idx, err := selectNode(nodes(), "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 0 || n.Tag != "Netherlands #1" {
		t.Errorf("got idx=%d tag=%q", idx, n.Tag)
	}
}

func TestSelectNodeByIndex(t *testing.T) {
	n, idx, err := selectNode(nodes(), "2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1 || n.Tag != "Germany Frankfurt" {
		t.Errorf("got idx=%d tag=%q", idx, n.Tag)
	}
}

func TestSelectNodeByTagSubstringCaseInsensitive(t *testing.T) {
	n, _, err := selectNode(nodes(), "frankfurt")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n.Tag != "Germany Frankfurt" {
		t.Errorf("got tag=%q", n.Tag)
	}
}

func TestSelectNodeIndexOutOfRange(t *testing.T) {
	if _, _, err := selectNode(nodes(), "9"); err == nil {
		t.Error("expected error for out-of-range index")
	}
}

func TestSelectNodeNoMatch(t *testing.T) {
	if _, _, err := selectNode(nodes(), "antarctica"); err == nil {
		t.Error("expected error for no tag match")
	}
}

func TestSelectNodeEmptyList(t *testing.T) {
	if _, _, err := selectNode(nil, ""); err == nil {
		t.Error("expected error for empty server list")
	}
}

func TestSelectNodeMatchesConfigRemarks(t *testing.T) {
	list := append(nodes(), configNode(t, `{"remarks":"🇵🇱 Poland","outbounds":[
		{"tag":"proxy","protocol":"vless","settings":{"vnext":[{"address":"pl.example.com","port":443}]}}]}`))

	n, idx, err := selectNode(list, "poland")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 3 || n.Config == nil {
		t.Errorf("got idx=%d node=%+v", idx, n)
	}
}

func TestSupportedRejectsUnknownProtocolInPool(t *testing.T) {
	node := configNode(t, `{"remarks":"pool","outbounds":[
		{"tag":"a","protocol":"vless","settings":{"vnext":[{"address":"a.example.com","port":443}]}},
		{"tag":"b","protocol":"tuic","settings":{"servers":[{"address":"b.example.com","port":443}]}}]}`)

	if supported(node) {
		t.Error("a pool containing a protocol xray cannot dial must be unsupported")
	}
	if got := protocolList(node); got != "vless+tuic" {
		t.Errorf("protocolList = %q", got)
	}
}

func TestSupportedAcceptsHysteriaPool(t *testing.T) {
	node := configNode(t, `{"remarks":"turbo","outbounds":[
		{"tag":"proxy","protocol":"hysteria","settings":{"address":"h.example.com","port":8443,"version":2}}]}`)

	if !supported(node) {
		t.Error("xray-core dials hysteria2, so the node must be supported")
	}
	if got := endpointSummary(node); got != "h.example.com:8443" {
		t.Errorf("endpointSummary = %q", got)
	}
}

func TestEndpointSummaryCountsPool(t *testing.T) {
	node := configNode(t, `{"remarks":"pool","outbounds":[
		{"tag":"a","protocol":"vless","settings":{"vnext":[{"address":"a.example.com","port":443}]}},
		{"tag":"b","protocol":"vless","settings":{"vnext":[{"address":"b.example.com","port":443}]}},
		{"tag":"direct","protocol":"freedom"}]}`)

	if got := endpointSummary(node); got != "2 servers" {
		t.Errorf("endpointSummary = %q, want the pooled server count", got)
	}
}
