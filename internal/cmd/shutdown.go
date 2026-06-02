package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/taigrr/crush/internal/server"
)

var shutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Shut down the Crush daemon",
	RunE: func(cmd *cobra.Command, _ []string) error {
		hostURL, err := server.ParseHostURL(clientHost)
		if err != nil {
			return fmt.Errorf("invalid host URL: %v", err)
		}

		if err := ensureServer(cmd, hostURL); err != nil {
			return err
		}

		c, err := newControlClient(hostURL)
		if err != nil {
			return err
		}

		if err := c.ShutdownServer(cmd.Context()); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(shutdownCmd)
}
