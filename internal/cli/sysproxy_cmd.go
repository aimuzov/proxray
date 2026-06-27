package cli

import (
	"github.com/spf13/cobra"

	"github.com/aimuzov/proxray/internal/sysproxy"
	"github.com/aimuzov/proxray/internal/ui"
)

func newSysProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "system-proxy",
		Aliases: []string{"sysproxy"},
		Short:   "Manage the macOS system proxy",
	}
	cmd.AddCommand(sysProxyOffCmd())
	return cmd
}

func sysProxyOffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: "Turn off the system proxy on all network services (emergency reset)",
		Long: "Disables the SOCKS, HTTP and HTTPS system proxies on every network\n" +
			"service. Use this if a previous session was killed (e.g. with kill -9)\n" +
			"and left the system proxy enabled. Requires sudo.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sysproxy.DisableAll(); err != nil {
				return err
			}
			ui.Success("System proxy disabled on all network services.")
			return nil
		},
	}
}
