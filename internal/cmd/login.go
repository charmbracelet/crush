package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/clipboard"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	_ "github.com/charmbracelet/crush/internal/oauth/hyper"
	_ "github.com/charmbracelet/crush/internal/oauth/openai"
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
Available platforms are: hyper, copilot, openai.`,
	Example: `
# Authenticate with Charm Hyper
crush login

# Authenticate with GitHub Copilot
crush login copilot

# Authenticate with OpenAI (ChatGPT / Codex)
crush login openai

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
		"codex",
	},
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, cleanup, err := setupWorkspaceWithProgressBar(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		platform := "hyper"
		if len(args) > 0 {
			platform = args[0]
		}
		force, _ := cmd.Flags().GetBool("force")

		targetID := normalizePlatform(platform)
		p := oauth.Get(targetID)
		if p == nil {
			return fmt.Errorf("unknown platform: %s", platform)
		}

		return loginOAuth(ws, p, force)
	},
}

func init() {
	loginCmd.Flags().BoolP("force", "f", false, "Force re-authentication even if already logged in")
}

func normalizePlatform(platform string) string {
	switch platform {
	case "github", "github-copilot":
		return string(catwalk.InferenceProviderCopilot)
	case "chatgpt", "codex":
		return string(catwalk.InferenceProviderOpenAI)
	default:
		return platform
	}
}

func loginOAuth(ws workspace.Workspace, p oauth.Provider, force bool) error {
	ctx := getLoginContext()

	if !force {
		cfg := ws.Config()
		if cfg != nil {
			if pc, ok := cfg.Providers.Get(p.ID()); ok && pc.OAuthToken != nil {
				fmt.Printf("You are already logged in to %s.\n", p.Name())
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	// For GitHub Copilot, check disk token first if available.
	if p.ID() == string(catwalk.InferenceProviderCopilot) {
		if diskToken, hasDiskToken := copilot.RefreshTokenFromDisk(); hasDiskToken {
			fmt.Println("Found existing GitHub Copilot token on disk. Using it to authenticate...")
			tok, err := copilot.RefreshToken(ctx, diskToken)
			if err == nil {
				if err := ws.SetProviderAPIKey(config.ScopeGlobal, p.ID(), tok); err != nil {
					return err
				}
				fmt.Println()
				fmt.Printf("You're now authenticated with %s!\n", p.Name())
				return nil
			}
		}
	}

	var token *oauth.Token

	switch prov := p.(type) {
	case oauth.BrowserFlowProvider:
		session, err := prov.StartBrowserFlow(ctx)
		if err != nil {
			return err
		}
		defer session.Close()

		fmt.Printf("Press enter to open this URL and authenticate with %s:\n\n", p.Name())
		lipgloss.Println(lipgloss.NewStyle().Hyperlink(session.URL(), "id="+p.ID()).Render(session.URL()))
		fmt.Println()
		waitEnter()
		if err := browser.OpenURL(session.URL()); err != nil {
			fmt.Println("Could not open the URL. You'll need to manually open the URL in your browser.")
		}

		fmt.Println("Waiting for authorization in browser...")
		t, err := session.Wait(ctx)
		if err != nil {
			return err
		}
		token = t

	case oauth.DeviceFlowProvider:
		dc, err := prov.RequestDeviceCode(ctx)
		if err != nil {
			return err
		}

		if dc.UserCode != "" {
			clipboard.WriteText(dc.UserCode)
			fmt.Println("The following code should be on clipboard already:")
			fmt.Println()
			lipgloss.Println(lipgloss.NewStyle().Bold(true).Render(dc.UserCode))
			fmt.Println()
		}

		fmt.Printf("Press enter to open this URL and authenticate with %s:\n\n", p.Name())
		lipgloss.Println(lipgloss.NewStyle().Hyperlink(dc.VerificationURI, "id="+p.ID()).Render(dc.VerificationURI))
		fmt.Println()
		waitEnter()
		if err := browser.OpenURL(dc.VerificationURI); err != nil {
			fmt.Println("Could not open the URL. You'll need to manually open the URL in your browser.")
		}

		fmt.Println("Waiting for authorization...")
		t, err := prov.PollForToken(ctx, dc)
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

	default:
		return fmt.Errorf("platform %s does not support interactive login", p.Name())
	}

	if err := ws.SetProviderAPIKey(config.ScopeGlobal, p.ID(), token); err != nil {
		return err
	}

	// After authenticating with Hyper, re-fetch the provider catalog.
	if p.ID() == "hyper" {
		if storeWs, ok := ws.(interface{ RefetchHyperProvider(context.Context) error }); ok {
			_ = storeWs.RefetchHyperProvider(ctx)
		}
	}

	fmt.Println()
	fmt.Printf("You're now authenticated with %s!\n", p.Name())
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
