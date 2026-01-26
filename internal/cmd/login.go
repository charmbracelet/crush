package cmd

import (
	"bufio"
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
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/anthropic"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
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
Available platforms are: hyper, copilot, anthropic.`,
	Example: `
# Authenticate with Charm Hyper
crush login

# Authenticate with GitHub Copilot
crush login copilot

# Authenticate with Anthropic (Claude)
crush login anthropic
  `,
	ValidArgs: []cobra.Completion{
		"hyper",
		"copilot",
		"github",
		"github-copilot",
		"anthropic",
		"claude",
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
		case "copilot", "github", "github-copilot":
			return loginCopilot()
		case "anthropic", "claude":
			return loginAnthropic()
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
	ctx := getLoginContext()

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

func loginCopilot() error {
	ctx := getLoginContext()

	cfg := config.Get()
	if cfg.HasConfigField("providers.copilot.oauth") {
		fmt.Println("You are already logged in to GitHub Copilot.")
		return nil
	}

	diskToken, hasDiskToken := copilot.RefreshTokenFromDisk()
	var token *oauth.Token

	switch {
	case hasDiskToken:
		fmt.Println("Found existing GitHub Copilot token on disk. Using it to authenticate...")

		t, err := copilot.RefreshToken(ctx, diskToken)
		if err != nil {
			return fmt.Errorf("unable to refresh token from disk: %w", err)
		}
		token = t
	default:
		fmt.Println("Requesting device code from GitHub...")
		dc, err := copilot.RequestDeviceCode(ctx)
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("Open the following URL and follow the instructions to authenticate with GitHub Copilot:")
		fmt.Println()
		fmt.Println(lipgloss.NewStyle().Hyperlink(dc.VerificationURI, "id=copilot").Render(dc.VerificationURI))
		fmt.Println()
		fmt.Println("Code:", lipgloss.NewStyle().Bold(true).Render(dc.UserCode))
		fmt.Println()
		fmt.Println("Waiting for authorization...")

		t, err := copilot.PollForToken(ctx, dc)
		if err == copilot.ErrNotAvailable {
			fmt.Println()
			fmt.Println("GitHub Copilot is unavailable for this account. To signup, go to the following page:")
			fmt.Println()
			fmt.Println(lipgloss.NewStyle().Hyperlink(copilot.SignupURL, "id=copilot-signup").Render(copilot.SignupURL))
			fmt.Println()
			fmt.Println("You may be able to request free access if eligible. For more information, see:")
			fmt.Println()
			fmt.Println(lipgloss.NewStyle().Hyperlink(copilot.FreeURL, "id=copilot-free").Render(copilot.FreeURL))
		}
		if err != nil {
			return err
		}
		token = t
	}

	if err := cmp.Or(
		cfg.SetConfigField("providers.copilot.api_key", token.AccessToken),
		cfg.SetConfigField("providers.copilot.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with GitHub Copilot!")
	return nil
}

func getLoginContext() context.Context {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	go func() {
		<-ctx.Done()
		cancel()
		os.Exit(1)
	}()
	return ctx
}

func waitEnter() {
	_, _ = fmt.Scanln()
}

func loginAnthropic() error {
	ctx := getLoginContext()
	cfg := config.Get()

	if cfg.HasConfigField("providers.anthropic.oauth") {
		fmt.Println("You are already logged in to Anthropic.")
		return nil
	}

	fmt.Println("🔐 Starting Anthropic OAuth flow...")
	fmt.Println()

	// Initiate the OAuth flow
	authParams, err := anthropic.InitiateAuth()
	if err != nil {
		return fmt.Errorf("failed to initiate auth: %w", err)
	}

	fmt.Println("Opening browser to:")
	fmt.Println(authParams.AuthURL)
	fmt.Println()

	if err := browser.OpenURL(authParams.AuthURL); err != nil {
		fmt.Println("Could not open the URL. Please manually open it in your browser.")
	}

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("After authorizing, the page will show a code and state.")
	fmt.Println("Copy them and paste in this format: code#state")
	fmt.Println("Example: abc123xyz...#def456uvw...")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Paste code#state here: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	codeState := strings.TrimSpace(input)

	if !strings.Contains(codeState, "#") {
		return fmt.Errorf("invalid format. Expected: code#state")
	}

	parts := strings.Split(codeState, "#")
	code := parts[0]
	returnedState := parts[1]

	// Validate state (CSRF protection)
	if returnedState != authParams.State {
		return fmt.Errorf("state mismatch - possible CSRF attack")
	}

	fmt.Println()
	fmt.Println("✅ Authorization code received!")
	fmt.Println()
	fmt.Println("🔄 Exchanging for tokens...")

	// Exchange code for tokens
	token, err := anthropic.ExchangeCode(ctx, code, authParams.CodeVerifier, returnedState)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	// Save the tokens
	// IMPORTANT: Prefix with "Bearer " so coordinator uses Authorization header
	bearerToken := "Bearer " + token.AccessToken
	if err := cmp.Or(
		cfg.SetConfigField("providers.anthropic.api_key", bearerToken),
		cfg.SetConfigField("providers.anthropic.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println("✅ Tokens obtained successfully!")
	fmt.Println()
	fmt.Println("You're now authenticated with Anthropic!")
	return nil
}
