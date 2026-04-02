package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth/openai_codex"
	"github.com/charmbracelet/crush/internal/openaicodex"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show OpenAI Codex subscription usage limits",
	Long:  `Show current OpenAI Codex subscription usage limits (for example 5h and weekly windows).`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		cfg := app.Config()
		provider, ok := cfg.Providers.Get(openaicodex.ProviderID)
		if !ok || provider.Disable {
			return fmt.Errorf("provider %q is not configured", openaicodex.ProviderID)
		}

		accessToken, err := app.Store().Resolve(provider.APIKey)
		if err != nil {
			return fmt.Errorf("resolve provider API key: %w", err)
		}
		accountID := strings.TrimSpace(provider.ExtraHeaders["chatgpt-account-id"])
		if accountID == "" {
			accountID, err = openai_codex.ExtractAccountID(accessToken)
			if err != nil {
				return fmt.Errorf("resolve chatgpt account id: %w", err)
			}
		}

		report, err := openaicodex.FetchUsage(cmd.Context(), openaicodex.UsageRequest{
			BaseURL:     provider.BaseURL,
			AccessToken: accessToken,
			AccountID:   accountID,
		})
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), openaicodex.FormatDetailedReport(report, time.Now()))
		return nil
	},
}
