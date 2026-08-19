package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/aimuzov/proxray/internal/profile"
	"github.com/aimuzov/proxray/internal/store"
	"github.com/aimuzov/proxray/internal/ui"
)

func newSubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sub",
		Short:   "Manage HAPP subscriptions",
		Aliases: []string{"subscription"},
	}
	cmd.AddCommand(subAddCmd(), subListCmd(), subUpdateCmd(), subRemoveCmd(), subUseCmd())
	return cmd
}

func subAddCmd() *cobra.Command {
	var name, userAgent string
	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Add and fetch a subscription",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			entry, err := fetchEntry(cmd.Context(), args[0], name, userAgent)
			if err != nil {
				return err
			}
			if err := st.Upsert(entry); err != nil {
				return err
			}
			ui.Success("Added subscription %q (%d servers).", entry.Name, len(entry.Nodes()))
			if st.Active() == entry.Name {
				ui.Info("It is now the active subscription.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "name for the subscription (default: derived from title/host)")
	cmd.Flags().StringVar(&userAgent, "ua", profile.DefaultUserAgent, "User-Agent sent when fetching")
	return cmd
}

func subUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "update [name]",
		Short:             "Re-fetch a subscription (default: active)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeSubNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			sub, err := resolveSub(st, name)
			if err != nil {
				return err
			}
			entry, err := fetchEntry(cmd.Context(), sub.URL, sub.Name, sub.UserAgent)
			if err != nil {
				return err
			}
			entry.Bypass = sub.Bypass // preserve the per-subscription bypass setting across updates
			if err := st.Upsert(entry); err != nil {
				return err
			}
			ui.Success("Updated %q (%d servers).", entry.Name, len(entry.Nodes()))
			return nil
		},
	}
	return cmd
}

func subListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List subscriptions",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			subs := st.Subscriptions()
			if len(subs) == 0 {
				ui.Info("No subscriptions. Add one with 'happ sub add <url>'.")
				return nil
			}
			rows := make([][]string, 0, len(subs))
			for _, s := range subs {
				active := ""
				if s.Name == st.Active() {
					active = "*"
				}
				expires := "-"
				if s.UserInfo != nil {
					expires = expiryString(s.UserInfo.Expire)
				}
				rows = append(rows, []string{
					active, s.Name, s.Title,
					fmt.Sprintf("%d", len(s.Nodes())),
					formatTraffic(s.UserInfo), expires,
				})
			}
			fmt.Print(ui.Table(
				[]string{"ACTIVE", "NAME", "TITLE", "SERVERS", "TRAFFIC", "EXPIRES"},
				rows,
			))
			return nil
		},
	}
	return cmd
}

func subRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "rm <name>",
		Aliases:           []string{"remove", "delete"},
		Short:             "Remove a subscription",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSubNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			if err := st.Remove(args[0]); err != nil {
				return err
			}
			ui.Success("Removed %q.", args[0])
			return nil
		},
	}
	return cmd
}

func subUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "use <name>",
		Short:             "Set the active subscription",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSubNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			if err := st.SetActive(args[0]); err != nil {
				return err
			}
			ui.Success("Active subscription is now %q.", args[0])
			return nil
		},
	}
	return cmd
}

// fetchEntry downloads a subscription and builds a store entry from it.
func fetchEntry(ctx context.Context, rawURL, name, userAgent string) (store.SubEntry, error) {
	sub, err := profile.Fetch(ctx, rawURL, userAgent)
	if err != nil {
		return store.SubEntry{}, err
	}
	if name == "" {
		name = defaultName(sub.Title, rawURL)
	}
	entry := store.SubEntry{
		Name:           name,
		URL:            rawURL,
		UserAgent:      userAgent,
		Title:          sub.Title,
		SupportURL:     sub.SupportURL,
		UpdateInterval: sub.UpdateInterval,
		UserInfo:       sub.UserInfo,
		UpdatedAt:      time.Now().Format(time.RFC3339),
	}
	for _, s := range sub.Body.Servers {
		entry.Links = append(entry.Links, s.Raw)
	}
	for _, c := range sub.Body.Configs {
		raw, err := c.JSON()
		if err != nil {
			return store.SubEntry{}, err
		}
		entry.Configs = append(entry.Configs, raw)
	}
	return entry, nil
}
