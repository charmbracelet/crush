package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/spf13/cobra"
)

var updateProvidersSource string

var updateProvidersCmd = &cobra.Command{
	Use:   "update-providers [path-or-url]",
	Short: "Update providers",
	Long:  `Update provider information from a specified local path or remote URL.`,
	Example: `
# Update Catwalk providers remotely (default), plus authenticated live providers
crush update-providers

# Update Catwalk providers from a custom URL
crush update-providers https://example.com/providers.json

# Update Catwalk providers from a local file
crush update-providers /path/to/local-providers.json

# Update Catwalk providers from embedded version
crush update-providers embedded

# Update Hyper provider information
crush update-providers --source=hyper

# Update Hyper from a custom URL
crush update-providers --source=hyper https://hyper.example.com

# Update Venice provider models
crush update-providers --source=venice

# Update Copilot provider models
crush update-providers --source=copilot
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// NOTE(@andreynering): We want to skip logging output do stdout here.
		slog.SetDefault(slog.New(slog.DiscardHandler))

		var pathOrURL string
		if len(args) > 0 {
			pathOrURL = args[0]
		}

		var err error
		switch updateProvidersSource {
		case "catwalk":
			err = config.UpdateProviders(pathOrURL)
			if err == nil && pathOrURL == "" {
				updateAuthenticatedLiveProviders(cmd)
			}
		case "hyper":
			err = config.UpdateHyper(pathOrURL)
		case "venice", "copilot":
			cfg, loadErr := loadUpdateProvidersConfig(cmd)
			if loadErr != nil {
				return loadErr
			}
			if updateProvidersSource == "venice" {
				err = config.UpdateVenice(pathOrURL, cfg)
			} else {
				err = config.UpdateCopilot(pathOrURL, cfg)
			}
		default:
			return fmt.Errorf("invalid source %q, must be 'catwalk', 'hyper', 'venice', or 'copilot'", updateProvidersSource)
		}

		if err != nil {
			return err
		}

		// NOTE(@andreynering): This style is more-or-less copied from Fang's
		// error message, adapted for success.
		headerStyle := lipgloss.NewStyle().
			Foreground(charmtone.Butter).
			Background(charmtone.Guac).
			Bold(true).
			Padding(0, 1).
			Margin(1).
			MarginLeft(2).
			SetString("SUCCESS")
		textStyle := lipgloss.NewStyle().
			MarginLeft(2).
			SetString(fmt.Sprintf("%s provider updated successfully.", updateProvidersSource))

		fmt.Printf("%s\n%s\n\n", headerStyle.Render(), textStyle.Render())
		return nil
	},
}

func updateAuthenticatedLiveProviders(cmd *cobra.Command) {
	cfg, err := loadUpdateProvidersConfig(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Note: skipping Venice and Copilot updates: %v\n", err)
		return
	}
	if err := config.UpdateVenice("", cfg); err != nil && !config.IsMissingLiveProviderCredentials(err) {
		fmt.Fprintf(os.Stderr, "Note: skipping Venice update: %v\n", err)
	}
	if err := config.UpdateCopilot("", cfg); err != nil && !config.IsMissingLiveProviderCredentials(err) {
		fmt.Fprintf(os.Stderr, "Note: skipping Copilot update: %v\n", err)
	}
}

func loadUpdateProvidersConfig(cmd *cobra.Command) (*config.Config, error) {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return nil, err
	}
	dataDir, _ := cmd.Flags().GetString("data-dir")
	debug, _ := cmd.Flags().GetBool("debug")

	// The config is loaded only to resolve provider credentials;
	// updateLiveProvider fetches explicitly afterwards. Disable provider
	// auto-update during Load so it doesn't fetch the same endpoints
	// first (avoiding a double fetch per provider).
	previous, hadPrevious := os.LookupEnv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE")
	_ = os.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	defer func() {
		if hadPrevious {
			_ = os.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", previous)
		} else {
			_ = os.Unsetenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE")
		}
	}()

	store, err := config.Load(cwd, dataDir, debug)
	if err != nil {
		return nil, err
	}
	return store.Config(), nil
}

func init() {
	updateProvidersCmd.Flags().StringVar(&updateProvidersSource, "source", "catwalk", "Provider source to update (catwalk, hyper, venice, or copilot)")
}
