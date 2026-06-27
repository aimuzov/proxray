package cli

import (
	"github.com/spf13/cobra"

	"github.com/aimuzov/happ-cli/internal/link"
	"github.com/aimuzov/happ-cli/internal/store"
)

// serverTags turns servers into shell completions, using the protocol as the
// description shown next to each tag.
func serverTags(servers []*link.Server) []cobra.Completion {
	out := make([]cobra.Completion, 0, len(servers))
	for _, s := range servers {
		out = append(out, cobra.CompletionWithDesc(s.Tag, s.Protocol))
	}
	return out
}

// subNames turns stored subscriptions into shell completions, annotating each
// with its title and marking the active one.
func subNames(st *store.Store) []cobra.Completion {
	subs := st.Subscriptions()
	out := make([]cobra.Completion, 0, len(subs))
	for _, s := range subs {
		desc := s.Title
		if s.Name == st.Active() {
			if desc != "" {
				desc += " "
			}
			desc += "(active)"
		}
		if desc == "" {
			out = append(out, cobra.Completion(s.Name))
			continue
		}
		out = append(out, cobra.CompletionWithDesc(s.Name, desc))
	}
	return out
}

// completeSubNames completes a single subscription-name argument. Cobra and the
// shell filter the returned list by the typed prefix.
func completeSubNames(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	st, err := openStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return subNames(st), cobra.ShellCompDirectiveNoFileComp
}

// completeSubFlag completes the --sub flag value with subscription names.
func completeSubFlag(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	st, err := openStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return subNames(st), cobra.ShellCompDirectiveNoFileComp
}

// completeBypassValue completes the route-bypass argument with the two valid
// values and their descriptions.
func completeBypassValue(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return []cobra.Completion{
		cobra.CompletionWithDesc("ru", "route Russian traffic direct (default)"),
		cobra.CompletionWithDesc("off", "route all traffic through the tunnel"),
	}, cobra.ShellCompDirectiveNoFileComp
}

// completeServerSelector completes the connect selector with server tags of the
// chosen subscription (--sub, or the active one).
func completeServerSelector(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	st, err := openStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	name, _ := cmd.Flags().GetString("sub")
	sub, err := resolveSub(st, name)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return serverTags(sub.Servers()), cobra.ShellCompDirectiveNoFileComp
}
