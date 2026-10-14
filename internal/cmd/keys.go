package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	ui "github.com/charmbracelet/crush/internal/ui/model"
	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "List every action and its key bindings",
	Long: `List every bindable action with its keys, after applying keybindings.json
from the config directory (next to themes/). Use an action name as the key in
that file: {"chat.new_session": ["ctrl+n"], "suspend": []}. An empty list
disables the action.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		return runKeys(cmd.OutOrStdout(), cmd.ErrOrStderr(), asJSON)
	},
}

func runKeys(out, errOut io.Writer, asJSON bool) error {
	km, warns := ui.LoadKeyMap(ui.KeybindingsPath())
	for _, w := range warns {
		fmt.Fprintln(errOut, "warning:", w)
	}
	entries := ui.KeyEntries(&km)
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACTION\tKEYS\tHELP")
	for _, e := range entries {
		keys := strings.Join(e.Keys, ", ")
		if !e.Enabled {
			keys = "(disabled)"
		} else if e.DisplayOnly {
			keys += " (display only)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Action, keys, e.Help)
	}
	return tw.Flush()
}

func init() {
	keysCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(keysCmd)
}
