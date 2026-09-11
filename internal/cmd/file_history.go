package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/filehistory"
	"github.com/spf13/cobra"
)

var fileHistoryCmd = &cobra.Command{
	Use:   "file-history <session-id> list|restore|redo|delete [checkpoint-id]",
	Short: "Restore workspace files independently of conversation history",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if (args[1] == "restore") != (len(args) == 3) {
			return fmt.Errorf("only restore takes a checkpoint ID")
		}
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		binary, err := cmd.Flags().GetString("binary")
		if err != nil {
			return err
		}
		store := filehistory.New(cwd, filepath.Join(filepath.Dir(config.GlobalConfigData()), "file-history"), binary)
		target := ""
		if len(args) == 3 {
			target = args[2]
		}
		events, runErr := store.Command(cmd.Context(), args[0], args[1], target)
		encoder := json.NewEncoder(cmd.OutOrStdout())
		for _, event := range events {
			if err := encoder.Encode(event); err != nil {
				return err
			}
		}
		return runErr
	},
}

func init() {
	fileHistoryCmd.Flags().String("binary", "", "Override the automatically managed filesnap executable")
}
