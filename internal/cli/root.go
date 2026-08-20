// Package cli implements the proxray command-line interface.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aimuzov/proxray/internal/log"
	"github.com/aimuzov/proxray/internal/store"
)

var homeDir string

// verbose mirrors the persistent -v/--verbose flag so any command (e.g. connect)
// can read it after PersistentPreRunE runs.
var verbose bool

// Execute runs the root command with the given context (cancelled on SIGINT).
// version is reported by `proxray --version` / `proxray version`.
func Execute(ctx context.Context, version string) error {
	return newRootCmd(version).ExecuteContext(ctx)
}

func newRootCmd(version string) *cobra.Command {
	var interactive bool
	root := &cobra.Command{
		Use:   "proxray",
		Short: "HAPP-compatible terminal VPN client",
		Long: "proxray is a terminal VPN client compatible with HAPP subscription profiles.\n" +
			"It fetches a subscription, parses its share links (VLESS/VMess/Trojan/Shadowsocks),\n" +
			"and connects through an embedded xray-core, either as a local proxy or a full TUN tunnel.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if interactive {
				return runInteractive(cmd.Context())
			}
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			log.SetVerbose(verbose)
			return nil
		},
	}
	root.Flags().BoolVarP(&interactive, "interactive", "i", false, "interactively pick server and method, elevating via sudo as needed")
	root.PersistentFlags().StringVar(&homeDir, "home", "", "config directory (default: per-user config dir + /proxray)")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose (debug) logging")
	root.AddCommand(
		newSubCmd(),
		newHWIDCmd(),
		newListCmd(),
		newConnectCmd(),
		newConfigCmd(),
		newSysProxyCmd(),
		newRouteCmd(),
	)
	return root
}

// storeDir resolves the config directory: the --home override, or the default.
func storeDir() (string, error) {
	if homeDir != "" {
		return homeDir, nil
	}
	return store.DefaultDir()
}

func openStore() (*store.Store, error) {
	dir, err := storeDir()
	if err != nil {
		return nil, err
	}
	return store.Open(dir)
}

// resolveSub returns the named subscription, or the active one when name is "".
func resolveSub(st *store.Store, name string) (store.SubEntry, error) {
	if name == "" {
		name = st.Active()
	}
	if name == "" {
		return store.SubEntry{}, fmt.Errorf("no subscription specified and none is active; add one with 'proxray sub add <url>'")
	}
	sub, ok := st.Find(name)
	if !ok {
		return store.SubEntry{}, fmt.Errorf("subscription %q not found", name)
	}
	return sub, nil
}
