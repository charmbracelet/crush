package cmd

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/claude"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Aliases: []string{"auth"},
	Use:     "login [platform]",
	Short:   "Login Crush to a platform",
	Long: `Login Crush to a specified platform.
The platform should be provided as an argument.
Available platforms are: claude.`,
	Example: `
# Authenticate with Claude Code Max
crush login claude
  `,
	ValidArgs: []cobra.Completion{
		"claude",
		"anthropic",
	},
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return fmt.Errorf("wrong number of arguments")
		}
		if len(args) == 0 || args[0] == "" {
			return cmd.Help()
		}

		app, err := setupAppWithProgressBar(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		switch args[0] {
		case "anthropic", "claude":
			return loginClaude()
		default:
			return fmt.Errorf("unknown platform: %s", args[0])
		}
	},
}

func loginClaude() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	go func() {
		<-ctx.Done()
		cancel()
		os.Exit(1)
	}()

	verifier, challenge, err := claude.GetChallenge()
	if err != nil {
		return err
	}
	url, err := claude.AuthorizeURL(verifier, challenge)
	if err != nil {
		return err
	}
	fmt.Println("Open the following URL and follow the instructions to authenticate with Claude Code Max:")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(url, "id=claude").Render(url))
	fmt.Println()
	fmt.Println("Press enter to continue...")
	if _, err := fmt.Scanln(); err != nil {
		return err
	}

	fmt.Println("Now paste and code from Anthropic and press enter...")
	fmt.Println()
	fmt.Print("> ")
	var code string
	for code == "" {
		_, _ = fmt.Scanln(&code)
		code = strings.TrimSpace(code)
	}

	fmt.Println()
	fmt.Println("Exchanging authorization code...")
	token, err := claude.ExchangeToken(ctx, code, verifier)
	if err != nil {
		return err
	}

	cfg := config.Get()
	if err := cmp.Or(
		cfg.SetConfigField("providers.anthropic.api_key", token.AccessToken),
		cfg.SetConfigField("providers.anthropic.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with Claude Code Max!")
	return nil
}
