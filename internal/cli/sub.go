package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/aimuzov/happ-cli/internal/profile"
	"github.com/aimuzov/happ-cli/internal/store"
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
			fmt.Printf("Added subscription %q (%d servers).\n", entry.Name, len(entry.Links))
			if st.Active() == entry.Name {
				fmt.Printf("It is now the active subscription.\n")
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
			fmt.Printf("Updated %q (%d servers).\n", entry.Name, len(entry.Links))
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
				fmt.Println("No subscriptions. Add one with 'happ sub add <url>'.")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "ACTIVE\tNAME\tTITLE\tSERVERS\tTRAFFIC\tEXPIRES")
			for _, s := range subs {
				active := ""
				if s.Name == st.Active() {
					active = "*"
				}
				expires := "-"
				if s.UserInfo != nil {
					expires = expiryString(s.UserInfo.Expire)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n", active, s.Name, s.Title, len(s.Links), formatTraffic(s.UserInfo), expires)
			}
			return tw.Flush()
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
			fmt.Printf("Removed %q.\n", args[0])
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
			fmt.Printf("Active subscription is now %q.\n", args[0])
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
	links := make([]string, 0, len(sub.Servers))
	for _, s := range sub.Servers {
		links = append(links, s.Raw)
	}
	return store.SubEntry{
		Name:           name,
		URL:            rawURL,
		UserAgent:      userAgent,
		Title:          sub.Title,
		SupportURL:     sub.SupportURL,
		UpdateInterval: sub.UpdateInterval,
		UserInfo:       sub.UserInfo,
		UpdatedAt:      time.Now().Format(time.RFC3339),
		Links:          links,
	}, nil
}
