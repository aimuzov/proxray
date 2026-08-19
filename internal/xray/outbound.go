package xray

import (
	"fmt"

	"github.com/aimuzov/proxray/internal/link"
)

// Supported reports whether xray-core can dial the given protocol as an
// outbound. TUIC and other cores' protocols are not supported here.
func Supported(protocol string) bool {
	switch protocol {
	case "vless", "vmess", "trojan", "shadowsocks",
		"hysteria", "hysteria2", "wireguard", "socks", "http":
		return true
	default:
		return false
	}
}

// SupportedServer reports whether xray-core can dial this exact server, which
// for some protocols depends on more than the protocol name.
func SupportedServer(s *link.Server) bool {
	if !Supported(s.Protocol) {
		return false
	}
	// xray's Hysteria2 implementation has no obfuscation layer.
	return !isHysteria(s.Protocol) || s.Obfs == ""
}

func isHysteria(protocol string) bool {
	return protocol == "hysteria" || protocol == "hysteria2"
}

// buildOutbound converts a normalized server into the proxy outbound, tagged
// "proxy". It returns an error for protocols xray-core cannot dial.
func buildOutbound(s *link.Server) (*Outbound, error) {
	out := &Outbound{Tag: "proxy", Protocol: s.Protocol}

	switch s.Protocol {
	case "vless":
		enc := "none"
		out.Settings = map[string]any{
			"vnext": []any{map[string]any{
				"address": s.Address,
				"port":    s.Port,
				"users": []any{map[string]any{
					"id":         s.UUID,
					"encryption": enc,
					"flow":       s.Flow,
				}},
			}},
		}
	case "vmess":
		security := s.Method
		if security == "" {
			security = "auto"
		}
		out.Settings = map[string]any{
			"vnext": []any{map[string]any{
				"address": s.Address,
				"port":    s.Port,
				"users": []any{map[string]any{
					"id":       s.UUID,
					"alterId":  s.AlterID,
					"security": security,
				}},
			}},
		}
	case "trojan":
		out.Settings = map[string]any{
			"servers": []any{map[string]any{
				"address":  s.Address,
				"port":     s.Port,
				"password": s.Password,
				"flow":     s.Flow,
			}},
		}
	case "shadowsocks":
		out.Settings = map[string]any{
			"servers": []any{map[string]any{
				"address":  s.Address,
				"port":     s.Port,
				"method":   s.Method,
				"password": s.Password,
			}},
		}
	case "hysteria", "hysteria2":
		return buildHysteriaOutbound(s)
	default:
		return nil, fmt.Errorf("xray: protocol %q is not supported by xray-core (use a sing-box based core)", s.Protocol)
	}

	out.StreamSettings = buildStream(s)
	return out, nil
}

// buildHysteriaOutbound builds a Hysteria2 outbound. xray spells it as protocol
// "hysteria" with version 2, and carries the credentials in the transport
// rather than in the proxy settings.
func buildHysteriaOutbound(s *link.Server) (*Outbound, error) {
	if s.Obfs != "" {
		return nil, fmt.Errorf("xray: hysteria2 obfuscation (%q) is not supported by xray-core", s.Obfs)
	}

	alpn := s.ALPN
	if len(alpn) == 0 {
		alpn = []string{"h3"}
	}
	return &Outbound{
		Tag:      "proxy",
		Protocol: "hysteria",
		Settings: map[string]any{
			"version": 2,
			"address": s.Address,
			"port":    s.Port,
		},
		StreamSettings: &StreamSettings{
			Network:          "hysteria",
			Security:         "tls",
			HysteriaSettings: &HysteriaSettings{Version: 2, Auth: s.Password},
			TLSSettings: &TLSSettings{
				ServerName:    tlsServerName(s),
				AllowInsecure: s.AllowInsecure,
				ALPN:          alpn,
				Fingerprint:   s.Fingerprint,
			},
		},
	}, nil
}

// buildStream produces streamSettings, or nil for a plain tcp/none transport
// (shadowsocks and bare VMess/VLESS) where xray defaults suffice.
func buildStream(s *link.Server) *StreamSettings {
	network := s.Network
	if network == "" {
		network = "tcp"
	}
	security := s.Security
	if security == "" {
		security = "none"
	}

	trivial := network == "tcp" && security == "none" && s.HeaderType == ""
	if trivial {
		return nil
	}

	ss := &StreamSettings{Network: network, Security: security}

	switch security {
	case "tls":
		ss.TLSSettings = &TLSSettings{
			ServerName:    tlsServerName(s),
			AllowInsecure: s.AllowInsecure,
			ALPN:          s.ALPN,
			Fingerprint:   s.Fingerprint,
		}
	case "reality":
		fp := s.Fingerprint
		if fp == "" {
			fp = "chrome"
		}
		ss.RealitySettings = &RealitySettings{
			ServerName:  tlsServerName(s),
			Fingerprint: fp,
			PublicKey:   s.PublicKey,
			ShortID:     s.ShortID,
			SpiderX:     s.SpiderX,
		}
	}

	switch network {
	case "ws":
		ws := &WSSettings{Path: s.Path}
		if host := wsHost(s); host != "" {
			ws.Headers = map[string]string{"Host": host}
		}
		ss.WSSettings = ws
	case "grpc":
		name := s.ServiceName
		if name == "" {
			name = s.Path
		}
		ss.GRPCSettings = &GRPCSettings{ServiceName: name}
	case "http", "h2":
		ss.Network = "http"
		h := &HTTPSettings{Path: s.Path}
		if host := wsHost(s); host != "" {
			h.Host = []string{host}
		}
		ss.HTTPSettings = h
	case "tcp":
		if s.HeaderType == "http" {
			ss.TCPSettings = &TCPSettings{Header: &TCPHeader{Type: "http"}}
		}
	}

	return ss
}

// tlsServerName chooses the SNI: explicit sni, then ws Host, then address.
func tlsServerName(s *link.Server) string {
	if s.SNI != "" {
		return s.SNI
	}
	if s.Host != "" {
		return s.Host
	}
	return s.Address
}

// wsHost is the Host header for ws/http transports: explicit host, else sni.
func wsHost(s *link.Server) string {
	if s.Host != "" {
		return s.Host
	}
	return s.SNI
}
