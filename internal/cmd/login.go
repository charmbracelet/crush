package cmd

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/signal"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/clipboard"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	openaioauth "github.com/charmbracelet/crush/internal/oauth/openai"
	xaioauth "github.com/charmbracelet/crush/internal/oauth/xai"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Aliases: []string{"auth"},
	Use:     "login [platform]",
	Short:   "Login Crush to a platform",
	Long: `Login Crush to a specified platform.
The platform should be provided as an argument.
Available platforms are: hyper, copilot, openai, xai.`,
	Example: `
# Authenticate with Charm Hyper
crush login

# Authenticate with GitHub Copilot
crush login copilot

# Authenticate with ChatGPT (OpenAI subscription)
crush login openai

# Authenticate with xAI SuperGrok
crush login xai

# Force re-authentication even if already logged in
crush login -f copilot
  `,
	ValidArgs: []cobra.Completion{
		"hyper",
		"copilot",
		"github",
		"github-copilot",
		"openai",
		"chatgpt",
		"xai",
		"grok",
	},
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, cleanup, err := setupWorkspaceWithProgressBar(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		provider := "hyper"
		if len(args) > 0 {
			provider = args[0]
		}
		force, _ := cmd.Flags().GetBool("force")
		switch provider {
		case "hyper":
			return loginHyper(ws, force)
		case "copilot", "github", "github-copilot":
			return loginCopilot(ws, force)
		case "openai", "chatgpt":
			return loginOpenAI(ws, force)
		case "xai", "grok":
			return loginXAI(ws, force)
		default:
			return fmt.Errorf("unknown platform: %s", args[0])
		}
	},
}

func init() {
	loginCmd.Flags().BoolP("force", "f", false, "Force re-authentication even if already logged in")
}

func loginHyper(ws workspace.Workspace, force bool) error {
	ctx := getLoginContext()

	if !force {
		cfg := ws.Config()
		if cfg != nil {
			if pc, ok := cfg.Providers.Get("hyper"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to Hyper.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	resp, err := hyper.InitiateDeviceAuth(ctx)
	if err != nil {
		return err
	}

	clipboard.WriteText(resp.UserCode)
	fmt.Println("The following code should be on clipboard already:")

	fmt.Println()
	lipgloss.Println(lipgloss.NewStyle().Bold(true).Render(resp.UserCode))
	fmt.Println()
	fmt.Println("Press enter to open this URL, and then paste it there:")
	fmt.Println()
	lipgloss.Println(lipgloss.NewStyle().Hyperlink(resp.VerificationURL, "id=hyper").Render(resp.VerificationURL))
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
		ws.SetConfigField(config.ScopeGlobal, "providers.hyper.api_key", token.AccessToken),
		ws.SetConfigField(config.ScopeGlobal, "providers.hyper.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with Hyper!")
	return nil
}

func loginCopilot(ws workspace.Workspace, force bool) error {
	loginCtx := getLoginContext()

	if !force {
		cfg := ws.Config()
		if cfg != nil {
			if pc, ok := cfg.Providers.Get("copilot"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to GitHub Copilot.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	diskToken, hasDiskToken := copilot.RefreshTokenFromDisk()
	var token *oauth.Token

	switch {
	case hasDiskToken:
		fmt.Println("Found existing GitHub Copilot token on disk. Using it to authenticate...")

		t, err := copilot.RefreshToken(loginCtx, diskToken)
		if err != nil {
			return fmt.Errorf("unable to refresh token from disk: %w", err)
		}
		token = t
	default:
		fmt.Println("Requesting device code from GitHub...")
		dc, err := copilot.RequestDeviceCode(loginCtx)
		if err != nil {
			return err
		}

		clipboard.WriteText(dc.UserCode)
		fmt.Println()
		fmt.Println("The following code should be on clipboard already:")
		fmt.Println()
		lipgloss.Println(lipgloss.NewStyle().Bold(true).Render(dc.UserCode))
		fmt.Println()
		fmt.Println("Press enter to open this URL and authenticate with GitHub Copilot:")
		fmt.Println()
		lipgloss.Println(lipgloss.NewStyle().Hyperlink(dc.VerificationURI, "id=copilot").Render(dc.VerificationURI))
		fmt.Println()
		waitEnter()
		if err := browser.OpenURL(dc.VerificationURI); err != nil {
			fmt.Println("Could not open the URL. You'll need to manually open the URL in your browser.")
		}

		fmt.Println("Waiting for authorization...")

		t, err := copilot.PollForToken(loginCtx, dc)
		if err == copilot.ErrNotAvailable {
			fmt.Println()
			fmt.Println("GitHub Copilot is unavailable for this account. To signup, go to the following page:")
			fmt.Println()
			lipgloss.Println(lipgloss.NewStyle().Hyperlink(copilot.SignupURL, "id=copilot-signup").Render(copilot.SignupURL))
			fmt.Println()
			fmt.Println("You may be able to request free access if eligible. For more information, see:")
			fmt.Println()
			lipgloss.Println(lipgloss.NewStyle().Hyperlink(copilot.FreeURL, "id=copilot-free").Render(copilot.FreeURL))
		}
		if err != nil {
			return err
		}
		token = t
	}

	if err := cmp.Or(
		ws.SetConfigField(config.ScopeGlobal, "providers.copilot.api_key", token.AccessToken),
		ws.SetConfigField(config.ScopeGlobal, "providers.copilot.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with GitHub Copilot!")
	return nil
}

func loginOpenAI(ws workspace.Workspace, force bool) error {
	loginCtx := getLoginContext()

	if !force {
		cfg := ws.Config()
		if cfg != nil {
			if pc, ok := cfg.Providers.Get("openai"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to OpenAI (ChatGPT).")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	fmt.Println("Requesting device code from OpenAI...")
	fmt.Println("(If this fails, enable device code auth in ChatGPT Codex security settings.)")
	dc, err := openaioauth.RequestDeviceCode(loginCtx)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Open the following URL and enter the code to authenticate with ChatGPT:")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(openaioauth.VerificationURL, "id=openai").Render(openaioauth.VerificationURL))
	fmt.Println()
	fmt.Println("Code:", lipgloss.NewStyle().Bold(true).Render(dc.UserCode))
	fmt.Println()
	fmt.Println("Waiting for authorization...")

	token, err := openaioauth.PollForToken(loginCtx, dc)
	if err != nil {
		return err
	}

	if err := cmp.Or(
		ws.SetConfigField(config.ScopeGlobal, "providers.openai.api_key", token.AccessToken),
		ws.SetConfigField(config.ScopeGlobal, "providers.openai.oauth", token),
	); err != nil {
		return err
	}

	if accountID := openaioauth.ChatGPTAccountID(token.AccessToken); accountID != "" {
		if err := ws.SetConfigField(config.ScopeGlobal, "providers.openai.extra_headers.ChatGPT-Account-ID", accountID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save ChatGPT account ID: %v\n", err)
		}
	}

	fmt.Println()
	fmt.Println("You're now authenticated with OpenAI (ChatGPT)!")
	return nil
}

func loginXAI(ws workspace.Workspace, force bool) error {
	loginCtx := getLoginContext()

	if !force {
		cfg := ws.Config()
		if cfg != nil {
			if pc, ok := cfg.Providers.Get("xai"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to xAI.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	fmt.Println("Requesting device code from xAI...")
	dc, err := xaioauth.RequestDeviceCode(loginCtx)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Open the following URL and enter the code to authenticate with xAI SuperGrok:")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(dc.VerificationURI, "id=xai").Render(dc.VerificationURI))
	fmt.Println()
	fmt.Println("Code:", lipgloss.NewStyle().Bold(true).Render(dc.UserCode))
	fmt.Println()
	fmt.Println("Waiting for authorization...")

	token, err := xaioauth.PollForToken(loginCtx, dc)
	if err != nil {
		return err
	}

	if err := cmp.Or(
		ws.SetConfigField(config.ScopeGlobal, "providers.xai.api_key", token.AccessToken),
		ws.SetConfigField(config.ScopeGlobal, "providers.xai.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with xAI!")
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
