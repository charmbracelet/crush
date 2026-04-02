package cmd

import (
	"bufio"
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	hyperp "github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	"github.com/charmbracelet/crush/internal/oauth/openai_codex"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Aliases: []string{"auth"},
	Use:     "login [platform]",
	Short:   "Login Crush to a platform",
	Long: `Login Crush to a specified platform.
The platform should be provided as an argument.
Available platforms are: hyper, copilot, openai-codex.`,
	Example: `
# Authenticate with Charm Hyper
crush login

# Authenticate with GitHub Copilot
crush login copilot

# Authenticate with ChatGPT Plus/Pro Codex subscription
crush login openai-codex
  `,
	ValidArgs: []cobra.Completion{
		"hyper",
		"copilot",
		"github",
		"github-copilot",
		openai_codex.ProviderID,
		"codex",
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
			return loginHyper(app.Store())
		case "copilot", "github", "github-copilot":
			return loginCopilot(app.Store())
		case openai_codex.ProviderID, "codex":
			return loginOpenAICodex(app.Store())
		default:
			return fmt.Errorf("unknown platform: %s", args[0])
		}
	},
}

func loginHyper(cfg *config.ConfigStore) error {
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
		cfg.SetConfigField(config.ScopeGlobal, "providers.hyper.api_key", token.AccessToken),
		cfg.SetConfigField(config.ScopeGlobal, "providers.hyper.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with Hyper!")
	return nil
}

func loginCopilot(cfg *config.ConfigStore) error {
	ctx := getLoginContext()

	if cfg.HasConfigField(config.ScopeGlobal, "providers.copilot.oauth") {
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
		cfg.SetConfigField(config.ScopeGlobal, "providers.copilot.api_key", token.AccessToken),
		cfg.SetConfigField(config.ScopeGlobal, "providers.copilot.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with GitHub Copilot!")
	return nil
}

func loginOpenAICodex(cfg *config.ConfigStore) error {
	ctx := getLoginContext()

	flow, err := openai_codex.StartAuthFlow("crush")
	if err != nil {
		return fmt.Errorf("failed to create auth flow: %w", err)
	}

	server, serverErr := openai_codex.StartLocalCallbackServer(flow.State)
	if serverErr != nil {
		fmt.Printf("Could not start local callback server on %s (%v). Manual paste fallback will be used.\n", "127.0.0.1:1455", serverErr)
	}
	if server != nil {
		defer server.Close(context.Background())
	}

	fmt.Println("Open the following URL to authenticate with ChatGPT Plus/Pro (Codex Subscription):")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(flow.AuthURL, "id=openai-codex").Render(flow.AuthURL))
	fmt.Println()
	fmt.Println("Press enter to open it in your browser.")
	waitEnter()
	if err := browser.OpenURL(flow.AuthURL); err != nil {
		fmt.Println("Could not open the URL automatically. Please open it manually.")
	}

	var code string
	if server != nil {
		waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		fmt.Println("Waiting for browser callback...")
		cbCode, err := server.WaitForCode(waitCtx)
		switch {
		case err == nil && cbCode != "":
			code = cbCode
		case err != nil && err != context.DeadlineExceeded && err != context.Canceled:
			fmt.Printf("Browser callback failed: %v\n", err)
		default:
			fmt.Println("Browser callback not received in time. Falling back to manual paste.")
		}
	}

	if strings.TrimSpace(code) == "" {
		fmt.Println()
		fmt.Println("Paste the authorization code or full redirect URL:")
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("read manual authorization input: %w", err)
		}
		parsedCode, parsedState := openai_codex.ParseAuthorizationInput(input)
		if parsedState != "" && parsedState != flow.State {
			return fmt.Errorf("state mismatch in manual authorization input")
		}
		code = strings.TrimSpace(parsedCode)
	}

	if code == "" {
		return fmt.Errorf("authorization code not provided")
	}

	token, err := openai_codex.ExchangeAuthorizationCode(ctx, code, flow.Verifier)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	if err := cfg.SetProviderAPIKey(config.ScopeGlobal, openai_codex.ProviderID, token); err != nil {
		return fmt.Errorf("failed to persist token: %w", err)
	}

	fmt.Println()
	fmt.Println("You're now authenticated with ChatGPT Codex subscription!")
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
