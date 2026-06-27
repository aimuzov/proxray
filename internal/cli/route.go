package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRouteCmd() *cobra.Command {
	var subName string
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Show or change RU-traffic bypass routing",
		Long: "route controls whether Russian traffic is sent direct (outside the\n" +
			"tunnel). Bypass is 'ru' by default so sites like ozon.ru keep working.",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			sub, err := resolveSub(st, subName)
			if err != nil {
				return err
			}
			bypass, err := normalizeBypass(sub.Bypass)
			if err != nil {
				return err
			}
			fmt.Printf("Subscription %q: bypass = %s\n", sub.Name, bypass)
			return nil
		},
	}

	set := &cobra.Command{
		Use:               "set <ru|off>",
		Short:             "Persist the bypass setting for a subscription",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeBypassValue,
		RunE: func(cmd *cobra.Command, args []string) error {
			bypass, err := normalizeBypass(args[0])
			if err != nil {
				return err
			}
			st, err := openStore()
			if err != nil {
				return err
			}
			sub, err := resolveSub(st, subName)
			if err != nil {
				return err
			}
			sub.Bypass = bypass
			if err := st.Upsert(sub); err != nil {
				return err
			}
			fmt.Printf("Subscription %q: bypass set to %s\n", sub.Name, bypass)
			return nil
		},
	}

	update := &cobra.Command{
		Use:   "update",
		Short: "Force-refresh the geoip/geosite databases",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := geoDir()
			if err != nil {
				return err
			}
			fmt.Println("Updating geo databases...")
			if err := geoUpdate(dir); err != nil {
				return err
			}
			fmt.Printf("Geo databases updated in %s\n", dir)
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&subName, "sub", "", "subscription name (default: active)")
	_ = cmd.RegisterFlagCompletionFunc("sub", completeSubFlag)
	cmd.AddCommand(set, update)
	return cmd
}
