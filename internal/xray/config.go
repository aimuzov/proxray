// Package xray builds xray-core configuration from a normalized link.Server.
// It deliberately has no dependency on xray-core itself; it only produces the
// human-readable JSON config that the run subpackage feeds to the embedded core.
package xray

import (
	"encoding/json"
	"fmt"

	"github.com/aimuzov/happ-cli/internal/link"
)

// Options controls the inbounds and logging of the generated config.
type Options struct {
	LogLevel  string // xray loglevel; default "warning"
	Listen    string // inbound listen address; default 127.0.0.1
	SocksPort int    // SOCKS inbound port; 0 disables
	HTTPPort  int    // HTTP inbound port; 0 disables
	Bypass    string // "ru" routes RU traffic direct; "off"/"" adds no routing
}

// Config is the root xray configuration object.
type Config struct {
	Log       *LogConfig     `json:"log,omitempty"`
	Inbounds  []Inbound      `json:"inbounds"`
	Outbounds []Outbound     `json:"outbounds"`
	Routing   *RoutingConfig `json:"routing,omitempty"`
}

// JSON marshals the config to the bytes accepted by xray-core's JSON loader.
func (c *Config) JSON() ([]byte, error) { return json.MarshalIndent(c, "", "  ") }

type LogConfig struct {
	Loglevel string `json:"loglevel,omitempty"`
}

type Inbound struct {
	Tag      string    `json:"tag,omitempty"`
	Listen   string    `json:"listen,omitempty"`
	Port     int       `json:"port"`
	Protocol string    `json:"protocol"`
	Settings any       `json:"settings,omitempty"`
	Sniffing *Sniffing `json:"sniffing,omitempty"`
}

type Sniffing struct {
	Enabled      bool     `json:"enabled"`
	DestOverride []string `json:"destOverride"`
}

type Outbound struct {
	Tag            string          `json:"tag,omitempty"`
	Protocol       string          `json:"protocol"`
	Settings       any             `json:"settings,omitempty"`
	StreamSettings *StreamSettings `json:"streamSettings,omitempty"`
}

type RoutingConfig struct {
	DomainStrategy string        `json:"domainStrategy,omitempty"`
	Rules          []RoutingRule `json:"rules,omitempty"`
}

type RoutingRule struct {
	Type        string   `json:"type"`
	Domain      []string `json:"domain,omitempty"`
	IP          []string `json:"ip,omitempty"`
	OutboundTag string   `json:"outboundTag"`
}

// ruBypassRouting routes Russian domains/IPs (and private IPs) straight out the
// "direct" outbound; everything else falls through to the first ("proxy") one.
func ruBypassRouting() *RoutingConfig {
	return &RoutingConfig{
		DomainStrategy: "IPIfNonMatch",
		Rules: []RoutingRule{
			{Type: "field", Domain: []string{"geosite:category-ru"}, OutboundTag: "direct"},
			{Type: "field", IP: []string{"geoip:private", "geoip:ru"}, OutboundTag: "direct"},
		},
	}
}

type StreamSettings struct {
	Network         string           `json:"network,omitempty"`
	Security        string           `json:"security,omitempty"`
	TLSSettings     *TLSSettings     `json:"tlsSettings,omitempty"`
	RealitySettings *RealitySettings `json:"realitySettings,omitempty"`
	WSSettings      *WSSettings      `json:"wsSettings,omitempty"`
	GRPCSettings    *GRPCSettings    `json:"grpcSettings,omitempty"`
	HTTPSettings    *HTTPSettings    `json:"httpSettings,omitempty"`
	TCPSettings     *TCPSettings     `json:"tcpSettings,omitempty"`
}

type TLSSettings struct {
	ServerName    string   `json:"serverName,omitempty"`
	AllowInsecure bool     `json:"allowInsecure,omitempty"`
	ALPN          []string `json:"alpn,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
}

type RealitySettings struct {
	ServerName  string `json:"serverName,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
	ShortID     string `json:"shortId,omitempty"`
	SpiderX     string `json:"spiderX,omitempty"`
}

type WSSettings struct {
	Path    string            `json:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type GRPCSettings struct {
	ServiceName string `json:"serviceName,omitempty"`
}

type HTTPSettings struct {
	Path string   `json:"path,omitempty"`
	Host []string `json:"host,omitempty"`
}

type TCPSettings struct {
	Header *TCPHeader `json:"header,omitempty"`
}

type TCPHeader struct {
	Type string `json:"type,omitempty"`
}

// BuildConfig turns a server and inbound options into an xray Config.
func BuildConfig(s *link.Server, opts Options) (*Config, error) {
	out, err := buildOutbound(s)
	if err != nil {
		return nil, err
	}

	inbounds, err := buildInbounds(opts)
	if err != nil {
		return nil, err
	}

	loglevel := opts.LogLevel
	if loglevel == "" {
		loglevel = "warning"
	}

	cfg := &Config{
		Log:      &LogConfig{Loglevel: loglevel},
		Inbounds: inbounds,
		Outbounds: []Outbound{
			*out,
			{Tag: "direct", Protocol: "freedom"},
			{Tag: "block", Protocol: "blackhole"},
		},
	}
	if opts.Bypass == "ru" {
		cfg.Routing = ruBypassRouting()
	}
	return cfg, nil
}

func buildInbounds(opts Options) ([]Inbound, error) {
	listen := opts.Listen
	if listen == "" {
		listen = "127.0.0.1"
	}
	sniff := &Sniffing{Enabled: true, DestOverride: []string{"http", "tls", "quic"}}

	var inbounds []Inbound
	if opts.SocksPort > 0 {
		inbounds = append(inbounds, Inbound{
			Tag:      "socks-in",
			Listen:   listen,
			Port:     opts.SocksPort,
			Protocol: "socks",
			Settings: map[string]any{"auth": "noauth", "udp": true},
			Sniffing: sniff,
		})
	}
	if opts.HTTPPort > 0 {
		inbounds = append(inbounds, Inbound{
			Tag:      "http-in",
			Listen:   listen,
			Port:     opts.HTTPPort,
			Protocol: "http",
			Settings: map[string]any{},
			Sniffing: sniff,
		})
	}
	if len(inbounds) == 0 {
		return nil, fmt.Errorf("xray: no inbound configured (set SocksPort or HTTPPort)")
	}
	return inbounds, nil
}
