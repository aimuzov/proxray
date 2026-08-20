package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aimuzov/proxray/internal/device"
	"github.com/aimuzov/proxray/internal/store"
	"github.com/aimuzov/proxray/internal/ui"
)

func newHWIDCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hwid",
		Short: "Show the device id sent to panels that limit devices",
		Long: "Panels with a device limit (Remnawave's HWID Device Limit) count devices by the\n" +
			"x-hwid header proxray sends when fetching a subscription. The id is derived from\n" +
			"this machine and stored, so it survives re-adding a subscription.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			id, err := ensureHWID(st)
			if err != nil {
				return err
			}
			info := device.Detect()
			fmt.Print(ui.Table(
				[]string{"HEADER", "VALUE"},
				[][]string{
					{"x-hwid", id},
					{"x-device-os", orDash(info.OS)},
					{"x-ver-os", orDash(info.Version)},
					{"x-device-model", orDash(info.Model)},
				},
			))
			return nil
		},
	}
	cmd.AddCommand(hwidSetCmd(), hwidResetCmd())
	return cmd
}

func hwidSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <id>",
		Short: "Use a specific device id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !device.Valid(args[0]) {
				return fmt.Errorf("invalid device id %q: panels accept 10-64 characters of a-z, A-Z, 0-9, '=' and '-'", args[0])
			}
			st, err := openStore()
			if err != nil {
				return err
			}
			if err := st.SetHWID(args[0]); err != nil {
				return err
			}
			ui.Success("Device id set to %s.", args[0])
			ui.Info("Run 'proxray sub update' to register it with the panel.")
			return nil
		},
	}
}

func hwidResetCmd() *cobra.Command {
	var random bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Forget the stored device id and derive it from this machine again",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			// Clearing the stored id is enough to re-derive it from the
			// machine; --random is for looking like a different device on a
			// machine whose own id never changes.
			if err := st.SetHWID(""); err != nil {
				return err
			}
			if random {
				id, err := device.Random()
				if err != nil {
					return fmt.Errorf("generate device id: %w", err)
				}
				if err := st.SetHWID(id); err != nil {
					return err
				}
				ui.Success("Device id is now %s.", id)
				return nil
			}
			id, err := ensureHWID(st)
			if err != nil {
				return err
			}
			ui.Success("Device id is now %s.", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&random, "random", false, "generate a random id instead of deriving it from the machine")
	return cmd
}

// ensureHWID returns the stored device id, deriving and storing one on first
// use. Storing it keeps the id from drifting if the machine lookup later fails
// and leaves room for 'hwid set' to override it.
func ensureHWID(st *store.Store) (string, error) {
	if id := st.HWID(); id != "" {
		return id, nil
	}
	id := device.Detect().HWID
	if id == "" {
		var err error
		if id, err = device.Random(); err != nil {
			return "", fmt.Errorf("generate device id: %w", err)
		}
	}
	if err := st.SetHWID(id); err != nil {
		return "", err
	}
	return id, nil
}

func orDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
