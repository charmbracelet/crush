package cmd

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List all available models from configured providers",
	Long:  `List all available models from configured providers. Shows provider name and model IDs.`,
	Example: `# List all available models
crush models

# Search models
crush models gpt5`,
	Args: cobra.ArbitraryArgs,
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

		filter := func(p config.ProviderConfig, m catwalk.Model) bool { return true }
		if len(args) > 0 {
			filter = func(p config.ProviderConfig, m catwalk.Model) bool {
				input := strings.ToLower(strings.Join(args, " "))
				contains := func(s string) bool {
					return strings.Contains(strings.ToLower(s), input)
				}
				return contains(p.ID) ||
					contains(p.Name) ||
					contains(m.ID) ||
					contains(m.Name)
			}
		}

		var providerIDs []string
		providerModels := make(map[string][]string)

		for providerID, provider := range cfg.Providers.Seq2() {
			if provider.Disable {
				continue
			}
			providerIDs = append(providerIDs, providerID)
			for _, model := range provider.Models {
				if !filter(provider, model) {
					continue
				}
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
			if providerNode.Children().Length() > 0 {
				t.Child(providerNode)
			}
		}

		cmd.Println(t)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}
