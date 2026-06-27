package cli

import (
	"context"
	"fmt"
	"net"

	"github.com/spf13/cobra"

	"github.com/aimuzov/happ-cli/internal/link"
	"github.com/aimuzov/happ-cli/internal/sysproxy"
	"github.com/aimuzov/happ-cli/internal/tunnel"
	"github.com/aimuzov/happ-cli/internal/xray"
)

func newConnectCmd() *cobra.Command {
	var mode, subName string
	var bypassFlag string
	var socksPort, httpPort int
	var systemProxy bool
	cmd := &cobra.Command{
		Use:     "connect [selector]",
		Aliases: []string{"up"},
		Short:   "Connect to a server (proxy or full TUN tunnel)",
		Long: "Connect runs in the foreground until interrupted (Ctrl+C).\n\n" +
			"selector picks the server: empty = first, a number = 1-based index,\n" +
			"or a case-insensitive substring of the server tag.\n\n" +
			"Modes:\n" +
			"  proxy  local SOCKS5 + HTTP proxy on 127.0.0.1 (no root)\n" +
			"  tun    system-wide VPN via a utun device (requires sudo, macOS)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeServerSelector,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			sub, err := resolveSub(st, subName)
			if err != nil {
				return err
			}
			selector := ""
			if len(args) > 0 {
				selector = args[0]
			}
			srv, idx, err := selectServer(sub.Servers(), selector)
			if err != nil {
				return err
			}
			if !xray.Supported(srv.Protocol) {
				return fmt.Errorf("server #%d uses %q, which xray-core cannot dial; pick another server", idx+1, srv.Protocol)
			}

			bypass, err := normalizeBypass(sub.Bypass)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("bypass") {
				if bypass, err = normalizeBypass(bypassFlag); err != nil {
					return err
				}
			}

			fmt.Printf("Server #%d: %s [%s] %s:%d\n", idx+1, srv.Tag, srv.Protocol, srv.Address, srv.Port)

			// prepareGeo may download the geo databases (first run / daily
			// refresh); print the server line first so a slow download isn't
			// silent.
			if err := prepareGeo(bypass); err != nil {
				return err
			}

			switch mode {
			case "proxy":
				return runProxy(cmd.Context(), srv, socksPort, httpPort, systemProxy, bypass)
			case "tun":
				return runTun(cmd.Context(), srv, socksPort, bypass)
			default:
				return fmt.Errorf("unknown mode %q (use 'proxy' or 'tun')", mode)
			}
		},
	}
	cmd.Flags().StringVarP(&mode, "mode", "m", "proxy", "connection mode: proxy or tun")
	cmd.Flags().IntVar(&socksPort, "socks", defaultSocksPort, "local SOCKS5 port")
	cmd.Flags().IntVar(&httpPort, "http", defaultHTTPPort, "local HTTP proxy port (proxy mode)")
	cmd.Flags().StringVar(&subName, "sub", "", "subscription name (default: active)")
	cmd.Flags().BoolVar(&systemProxy, "system-proxy", false, "set the macOS system SOCKS proxy (requires sudo, proxy mode)")
	cmd.Flags().StringVar(&bypassFlag, "bypass", "", "route a region's traffic direct, outside the tunnel: 'ru' or 'off' (default: subscription setting, or 'ru')")
	_ = cmd.RegisterFlagCompletionFunc("bypass", completeBypassValue)
	_ = cmd.RegisterFlagCompletionFunc("sub", completeSubFlag)
	_ = cmd.RegisterFlagCompletionFunc("mode", func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return []cobra.Completion{
			cobra.CompletionWithDesc("proxy", "local SOCKS5 + HTTP proxy (no root)"),
			cobra.CompletionWithDesc("tun", "system-wide VPN via utun (sudo)"),
		}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func runProxy(ctx context.Context, srv *link.Server, socksPort, httpPort int, systemProxy bool, bypass string) error {
	cfg, err := xray.BuildConfig(srv, xray.Options{SocksPort: socksPort, HTTPPort: httpPort, Bypass: bypass})
	if err != nil {
		return err
	}
	raw, err := cfg.JSON()
	if err != nil {
		return err
	}
	inst, err := xray.Start(raw)
	if err != nil {
		return err
	}
	defer inst.Close()

	fmt.Printf("Proxy is up:\n  SOCKS5  127.0.0.1:%d\n  HTTP    127.0.0.1:%d\n", socksPort, httpPort)

	if systemProxy {
		restore, err := sysproxy.Enable("127.0.0.1", socksPort, httpPort)
		if err != nil {
			return fmt.Errorf("enable system proxy: %w", err)
		}
		defer func() {
			if err := restore(); err != nil {
				fmt.Println("warning: failed to restore system proxy:", err)
			}
		}()
		fmt.Println("System SOCKS/HTTP proxy set on all network services (will be restored on exit).")
	}

	fmt.Println("Press Ctrl+C to disconnect.")
	<-ctx.Done()
	fmt.Println("\nDisconnecting...")
	return nil
}

func runTun(ctx context.Context, srv *link.Server, socksPort int, bypass string) error {
	ips, err := resolveIPv4(srv.Address)
	if err != nil {
		return fmt.Errorf("resolve server address %q: %w", srv.Address, err)
	}

	// Pin the outbound to a concrete IP and keep the TLS SNI on the domain, so
	// xray dials the exact IP we route around the tunnel.
	pinned := *srv
	if pinned.SNI == "" {
		pinned.SNI = srv.Address
	}
	pinned.Address = ips[0]

	// RU bypass is not effective in tun mode yet: even with the direct outbound
	// bound to the server-route interface (IP_BOUND_IF), its sockets still loop
	// back through utun. Until that is solved, force bypass off in tun mode so RU
	// sites keep working through the tunnel instead of hanging. Bypass remains
	// effective in proxy / --system-proxy modes.
	if bypass != "off" {
		fmt.Println("warning: --bypass is not supported in tun mode yet; routing all traffic through the tunnel")
		bypass = "off"
	}

	cfg, err := xray.BuildConfig(&pinned, xray.Options{SocksPort: socksPort, Bypass: bypass})
	if err != nil {
		return err
	}
	raw, err := cfg.JSON()
	if err != nil {
		return err
	}
	inst, err := xray.Start(raw)
	if err != nil {
		return err
	}
	defer inst.Close()

	tun, err := tunnel.Start(tunnel.Options{
		SocksAddr: fmt.Sprintf("127.0.0.1:%d", socksPort),
		ServerIPs: ips,
	})
	if err != nil {
		return err
	}
	defer tun.Close()

	fmt.Printf("TUN tunnel is up; all traffic is routed through %s.\n", srv.Tag)
	fmt.Println("Press Ctrl+C to disconnect and restore routing.")
	<-ctx.Done()
	fmt.Println("\nDisconnecting and restoring routes...")
	return nil
}

// resolveIPv4 returns the IPv4 addresses for host, or host itself if it is one.
func resolveIPv4(host string) ([]string, error) {
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() == nil {
			return nil, fmt.Errorf("IPv6 server addresses are not supported in TUN mode yet")
		}
		return []string{host}, nil
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, a := range addrs {
		if v4 := a.To4(); v4 != nil {
			out = append(out, v4.String())
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no IPv4 address found for %q", host)
	}
	return out, nil
}
