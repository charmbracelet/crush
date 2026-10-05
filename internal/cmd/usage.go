package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/usage"
	"github.com/spf13/cobra"
)

var usageCmd = &cobra.Command{
	Use:   "usage [provider]",
	Short: "Show the quota left on a provider's plan",
	Long: `Show the remaining quota reported by providers that declare a usage endpoint.

A provider declares where its allowance is read with the usage block of a
plugin or crushrc, which is how a subscription plan surfaces what is left
without any Crush code:

  provider add example --usage-url "https://api.example.com/quota" \
    --usage-groups groups --usage-meters buckets

Providers with no declared endpoint are skipped.`,
	Example: `
# Show the quota for every provider that reports one
crush usage

# Show it for a single provider
crush usage example-plan`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		dataDir, _ := cmd.Flags().GetString("data-dir")
		debug, _ := cmd.Flags().GetBool("debug")

		store, err := config.Init(cwd, dataDir, debug)
		if err != nil {
			return err
		}

		wanted := ""
		if len(args) == 1 {
			wanted = args[0]
		}

		var (
			resolver = config.NewShellVariableResolver(env.New())
			reported int
			firstErr error
		)
		for _, id := range sortedUsageProviders(store.Config(), wanted) {
			pc, ok := store.Config().Providers.Get(id)
			if !ok {
				return fmt.Errorf("provider %s is not configured", id)
			}
			reported++

			fmt.Printf("%s\n", pc.Name)
			meters, err := fetchProviderUsage(cmd.Context(), store, id, pc, resolver)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %v\n\n", err)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			for _, meter := range meters {
				fmt.Printf("  %s\n", meter.Summary())
			}
			fmt.Println()
		}

		if reported == 0 {
			fmt.Println("No provider declares a usage endpoint.")
			fmt.Println("Add --usage-url to a provider in a plugin or crushrc to see the quota left.")
			return nil
		}
		return firstErr
	},
}

// sortedUsageProviders lists provider IDs that declare a usage endpoint,
// in name order for stable output, or just the one asked for.
func sortedUsageProviders(cfg *config.Config, wanted string) []string {
	var ids []string
	for id, pc := range cfg.Providers.Seq2() {
		if wanted != "" {
			if id == wanted {
				return []string{id}
			}
			continue
		}
		if pc.Usage != nil && !pc.Disable {
			ids = append(ids, id)
		}
	}
	return ids
}

// fetchProviderUsage reads the meters for one provider. A signed-in provider
// is refreshed first when its access token has lapsed, so a long-running
// session keeps reporting a live figure rather than failing on a stale token.
func fetchProviderUsage(
	ctx context.Context,
	store *config.ConfigStore,
	id string,
	pc config.ProviderConfig,
	resolver config.VariableResolver,
) ([]usage.Meter, error) {
	if pc.Usage == nil {
		return nil, fmt.Errorf("provider %s declares no usage endpoint", id)
	}

	bearer := ""
	if pc.OAuthToken != nil {
		if pc.OAuthToken.IsExpired() {
			refreshCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			if err := store.RefreshOAuthToken(refreshCtx, config.ScopeGlobal, id); err != nil {
				return nil, fmt.Errorf("refresh login before reading quota: %w", err)
			}
			if fresh, ok := store.Config().Providers.Get(id); ok {
				pc = fresh
			}
		}
		if pc.OAuthToken != nil {
			bearer = pc.OAuthToken.AccessToken
		}
	}
	if bearer == "" {
		resolved, err := resolver.ResolveValue(pc.APIKey)
		if err != nil {
			return nil, fmt.Errorf("resolving provider %s credential: %w", id, err)
		}
		bearer = resolved
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return usage.Fetch(fetchCtx, pc.Usage, bearer, pc.ExtraHeaders)
}

func init() {
	rootCmd.AddCommand(usageCmd)
}
