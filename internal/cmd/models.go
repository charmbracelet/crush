package cmd

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List all available models from known providers",
	Long:  `List all available models from known providers. Shows provider name and model IDs. Unconfigured providers are marked with (not configured).`,
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

		cfg, err := config.Init(cmd.Context(), cwd, dataDir, debug)
		if err != nil {
			return err
		}

		term := strings.ToLower(strings.Join(args, " "))

		type providerEntry struct {
			name       string
			models     []string
			configured bool
		}

		entries := make(map[string]*providerEntry)

		// Add configured providers first.
		for providerID, provider := range cfg.Config().Providers.Seq2() {
			if provider.Disable {
				continue
			}
			entry := &providerEntry{
				name:       provider.Name,
				configured: true,
			}
			for _, model := range provider.Models {
				if term != "" {
					matched := false
					for _, s := range []string{provider.ID, provider.Name, model.ID, model.Name} {
						if strings.Contains(strings.ToLower(s), term) {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}
				entry.models = append(entry.models, model.ID)
			}
			if len(entry.models) > 0 {
				slices.Sort(entry.models)
				entries[providerID] = entry
			}
		}

		// Add known but unconfigured providers from catwalk.
		for _, kp := range cfg.KnownProviders() {
			providerID := string(kp.ID)
			if _, exists := entries[providerID]; exists {
				continue
			}
			entry := &providerEntry{
				name:       kp.Name,
				configured: false,
			}
			for _, model := range kp.Models {
				if term != "" {
					matched := false
					for _, s := range []string{providerID, kp.Name, model.ID, model.Name} {
						if strings.Contains(strings.ToLower(s), term) {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}
				entry.models = append(entry.models, model.ID)
			}
			if len(entry.models) > 0 {
				slices.Sort(entry.models)
				entries[providerID] = entry
			}
		}

		var providerIDs []string
		for id := range entries {
			providerIDs = append(providerIDs, id)
		}
		sort.Strings(providerIDs)

		if len(providerIDs) == 0 && len(args) == 0 {
			return fmt.Errorf("no providers found")
		}
		if len(providerIDs) == 0 {
			return fmt.Errorf("no providers found matching %q", term)
		}

		if !isatty.IsTerminal(os.Stdout.Fd()) {
			for _, providerID := range providerIDs {
				entry := entries[providerID]
				for _, modelID := range entry.models {
					fmt.Println(providerID + "/" + modelID)
				}
			}
			return nil
		}

		t := tree.New()
		for _, providerID := range providerIDs {
			entry := entries[providerID]
			label := providerID
			if !entry.configured {
				label += " (not configured)"
			}
			providerNode := tree.Root(label)
			for _, modelID := range entry.models {
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
