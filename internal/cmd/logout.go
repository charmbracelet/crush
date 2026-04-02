package cmd

import (
	"fmt"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/openai_codex"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout [platform]",
	Short: "Logout Crush from a platform",
	Long: `Logout Crush from a specified platform.
Available platforms are: openai-codex, copilot, hyper.`,
	ValidArgs: []cobra.Completion{
		openai_codex.ProviderID,
		"codex",
		"copilot",
		"github",
		"github-copilot",
		"hyper",
	},
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		providerID, err := resolveLogoutProvider(args[0])
		if err != nil {
			return err
		}

		return logoutProvider(app.Store(), providerID)
	},
}

func resolveLogoutProvider(input string) (string, error) {
	switch input {
	case openai_codex.ProviderID, "codex":
		return openai_codex.ProviderID, nil
	case "copilot", "github", "github-copilot":
		return string(catwalk.InferenceProviderCopilot), nil
	case "hyper":
		return hyper.Name, nil
	default:
		return "", fmt.Errorf("unknown platform: %s", input)
	}
}

func logoutProvider(cfg *config.ConfigStore, providerID string) error {
	fields := []string{
		fmt.Sprintf("providers.%s.api_key", providerID),
		fmt.Sprintf("providers.%s.oauth", providerID),
		fmt.Sprintf("providers.%s.extra_headers", providerID),
	}
	for _, field := range fields {
		// Ignore delete errors to keep logout idempotent.
		_ = cfg.RemoveConfigField(config.ScopeGlobal, field)
	}

	if providerCfg, ok := cfg.Config().Providers.Get(providerID); ok {
		providerCfg.APIKey = ""
		providerCfg.OAuthToken = nil
		providerCfg.ExtraHeaders = map[string]string{}
		cfg.Config().Providers.Set(providerID, providerCfg)
	}

	fmt.Printf("Logged out from %s.\n", providerID)
	return nil
}
