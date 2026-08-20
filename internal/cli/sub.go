package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/aimuzov/proxray/internal/device"
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
	var noHWID bool
	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Add and fetch a subscription",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			entry, err := fetchEntry(cmd.Context(), st, subRequest{
				URL:       args[0],
				Name:      name,
				UserAgent: userAgent,
				NoHWID:    noHWID,
			})
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
	cmd.Flags().BoolVar(&noHWID, "no-hwid", false, "do not identify this machine to the panel (see 'proxray hwid')")
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
			entry, err := fetchEntry(cmd.Context(), st, subRequest{
				URL:       sub.URL,
				Name:      sub.Name,
				UserAgent: sub.UserAgent,
				NoHWID:    sub.NoHWID,
			})
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

// subRequest is what a fetch needs to know up front; the rest of the entry
// comes out of the response.
type subRequest struct {
	URL       string
	Name      string
	UserAgent string
	NoHWID    bool
}

// fetchEntry downloads a subscription and builds a store entry from it.
func fetchEntry(ctx context.Context, st *store.Store, req subRequest) (store.SubEntry, error) {
	opts := profile.FetchOptions{UserAgent: req.UserAgent}
	if !req.NoHWID {
		hwid, err := ensureHWID(st)
		if err != nil {
			return store.SubEntry{}, err
		}
		info := device.Detect()
		opts.HWID = hwid
		opts.DeviceOS, opts.OSVersion, opts.DeviceModel = info.OS, info.Version, info.Model
	}

	sub, err := profile.Fetch(ctx, req.URL, opts)
	if err != nil {
		return store.SubEntry{}, err
	}
	// Report the id whenever it was sent, not only when the panel says it
	// enforces a limit: most panels only read the header and answer nothing, and
	// without this line there is no way to tell from the CLI that it went at all.
	if opts.HWID != "" {
		if sub.HWIDActive {
			ui.Info("Sent device id %s; the panel enforces a device limit.", opts.HWID)
		} else {
			ui.Info("Sent device id %s (run with -v to see what the panel answered).", opts.HWID)
		}
	}
	name := req.Name
	if name == "" {
		name = defaultName(sub.Title, req.URL)
	}
	entry := store.SubEntry{
		Name:           name,
		URL:            req.URL,
		UserAgent:      req.UserAgent,
		NoHWID:         req.NoHWID,
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
