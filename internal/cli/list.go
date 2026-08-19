package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aimuzov/proxray/internal/ui"
)

func newListCmd() *cobra.Command {
	var subName string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "servers"},
		Short:   "List servers in a subscription",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			sub, err := resolveSub(st, subName)
			if err != nil {
				return err
			}
			nodes := sub.Nodes()
			if len(nodes) == 0 {
				ui.Info("Subscription %q has no servers.", sub.Name)
				return nil
			}
			// Only JSON subscriptions describe their entries, so the note
			// column appears only when there is something to put in it.
			notes := false
			for _, n := range nodes {
				notes = notes || n.Note != ""
			}

			rows := make([][]string, 0, len(nodes))
			for i, n := range nodes {
				protocol := protocolList(n)
				if !supported(n) {
					protocol += " (unsupported)"
				}
				row := []string{
					fmt.Sprintf("%d", i+1),
					protocol,
					endpointSummary(n),
					n.Tag,
				}
				if notes {
					row = append(row, n.Note)
				}
				rows = append(rows, row)
			}

			header := []string{"#", "PROTOCOL", "ADDRESS", "TAG"}
			if notes {
				header = append(header, "NOTE")
			}
			fmt.Print(ui.Table(header, rows))
			return nil
		},
	}
	cmd.Flags().StringVar(&subName, "sub", "", "subscription name (default: active)")
	_ = cmd.RegisterFlagCompletionFunc("sub", completeSubFlag)
	return cmd
}

func newConfigCmd() *cobra.Command {
	var subName string
	var socksPort, httpPort int
	cmd := &cobra.Command{
		Use:               "config [selector]",
		Short:             "Print the generated xray-core config for a server",
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
			node, _, err := selectNode(sub.Nodes(), selector)
			if err != nil {
				return err
			}
			raw, err := buildRuntimeConfig(node, runtimeOptions{SocksPort: socksPort, HTTPPort: httpPort})
			if err != nil {
				return err
			}
			fmt.Println(string(raw))
			return nil
		},
	}
	cmd.Flags().StringVar(&subName, "sub", "", "subscription name (default: active)")
	cmd.Flags().IntVar(&socksPort, "socks", 10808, "SOCKS5 inbound port")
	cmd.Flags().IntVar(&httpPort, "http", 10809, "HTTP inbound port")
	_ = cmd.RegisterFlagCompletionFunc("sub", completeSubFlag)
	return cmd
}
