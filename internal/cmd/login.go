package cmd

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	hyperp "github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/claude"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Aliases: []string{"auth"},
	Use:     "login [platform]",
	Short:   "Login Crush to a platform",
	Long: `Login Crush to a specified platform.
The platform should be provided as an argument.
Available platforms are: hyper, claude.`,
	Example: `
# Authenticate with Charm Hyper
crush login

# Authenticate with Claude Code Max
crush login claude
  `,
	ValidArgs: []cobra.Completion{
		"hyper",
		"claude",
		"anthropic",
	},
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := setupAppWithProgressBar(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		provider := "hyper"
		if len(args) > 0 {
			provider = args[0]
		}
		switch provider {
		case "hyper":
			return loginHyper()
		case "anthropic", "claude":
			return loginClaude()
		default:
			return fmt.Errorf("unknown platform: %s", args[0])
		}
	},
}

func loginHyper() error {
	cfg := config.Get()
	if !hyperp.Enabled() {
		return fmt.Errorf("hyper not enabled")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	resp, err := hyper.InitiateDeviceAuth(ctx)
	if err != nil {
		return err
	}

	if clipboard.WriteAll(resp.UserCode) == nil {
		fmt.Println("The following code should be on clipboard already:")
	} else {
		fmt.Println("Copy the following code:")
	}

	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Bold(true).Render(resp.UserCode))
	fmt.Println()
	fmt.Println("Press enter to open this URL, and then paste it there:")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(resp.VerificationURL, "id=hyper").Render(resp.VerificationURL))
	fmt.Println()
	waitEnter()
	if err := browser.OpenURL(resp.VerificationURL); err != nil {
		fmt.Println("Could not open the URL. You'll need to manually open the URL in your browser.")
	}

	fmt.Println("Exchanging authorization code...")
	refreshToken, err := hyper.PollForToken(ctx, resp.DeviceCode, resp.ExpiresIn)
	if err != nil {
		return err
	}

	fmt.Println("Exchanging refresh token for access token...")
	token, err := hyper.ExchangeToken(ctx, refreshToken)
	if err != nil {
		return err
	}

	fmt.Println("Verifying access token...")
	introspect, err := hyper.IntrospectToken(ctx, token.AccessToken)
	if err != nil {
		return fmt.Errorf("token introspection failed: %w", err)
	}
	if !introspect.Active {
		return fmt.Errorf("access token is not active")
	}

	if err := cmp.Or(
		cfg.SetConfigField("providers.hyper.api_key", token.AccessToken),
		cfg.SetConfigField("providers.hyper.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with Hyper!")
	return nil
}

func loginClaude() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

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

func waitEnter() {
	_, _ = fmt.Scanln()
}
