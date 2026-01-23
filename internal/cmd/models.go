package cmd

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/mattn/go-isatty"
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

		term := strings.ToLower(strings.Join(args, " "))
		filter := func(p config.ProviderConfig, m catwalk.Model) bool {
			for _, s := range []string{p.ID, p.Name, m.ID, m.Name} {
				if term == "" || strings.Contains(strings.ToLower(s), term) {
					return true
				}
			}
			return false
		}

		var providerIDs []string
		providerModels := make(map[string][]string)

		for providerID, provider := range cfg.Providers.Seq2() {
			if provider.Disable {
				continue
			}
			var found bool
			for _, model := range provider.Models {
				if !filter(provider, model) {
					continue
				}
				providerModels[providerID] = append(providerModels[providerID], model.ID)
				found = true
			}
			if !found {
				continue
			}
			slices.Sort(providerModels[providerID])
			providerIDs = append(providerIDs, providerID)
		}
		sort.Strings(providerIDs)

		if len(providerIDs) == 0 && len(args) == 0 {
			return fmt.Errorf("no enabled providers found")
		}
		if len(providerIDs) == 0 {
			return fmt.Errorf("no enabled providers found matching %q", term)
		}

		if !isatty.IsTerminal(os.Stdout.Fd()) {
			for _, providerID := range providerIDs {
				for _, modelID := range providerModels[providerID] {
					fmt.Println(providerID + "/" + modelID)
				}
			}
			return nil
		}

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
