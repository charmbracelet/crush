package cmd

import (
	"fmt"
	"slices"
	"sort"

	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List all available models from configured providers",
	Long:  `List all available models from configured providers. Shows provider name and model IDs.`,
	Example: `  # List all available models
  crush models`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		dataDir, _ := cmd.Flags().GetString("data-dir")
		debug, _ := cmd.Flags().GetBool("debug")

		cfg, err := config.Init(cwd, dataDir, debug)
		if err != nil {
			return err
		}

		if !cfg.IsConfigured() {
			return fmt.Errorf("no providers configured - please run 'crush' to set up a provider interactively")
		}

		var providerIDs []string
		providerModels := make(map[string][]string)

		for providerID, provider := range cfg.Providers.Seq2() {
			if provider.Disable {
				continue
			}
			providerIDs = append(providerIDs, providerID)
			for _, model := range provider.Models {
				providerModels[providerID] = append(providerModels[providerID], model.ID)
			}
			slices.Sort(providerModels[providerID])
		}
		sort.Strings(providerIDs)

		t := tree.New()
		for _, providerID := range providerIDs {
			providerNode := tree.Root(providerID)
			for _, modelID := range providerModels[providerID] {
				providerNode.Child(modelID)
			}
			t.Child(providerNode)
		}

		cmd.Println(t)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}
