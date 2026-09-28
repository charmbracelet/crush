package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/login"
	"github.com/charmbracelet/crush/internal/oauth/antigravity"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/workspace"
)

var loginCmd = &cobra.Command{
	Aliases: []string{"auth"},
	Use:     "login [platform]",
	Short:   "Login Crush to a platform",
	Long: `Login Crush to a specified platform.
The platform should be provided as an argument.
Available platforms are: hyper, copilot, openai, gemini.`,
	Example: `
# Authenticate with Charm Hyper
crush login

# Authenticate with GitHub Copilot
crush login copilot

# Authenticate with a ChatGPT (OpenAI) account
crush login openai

# Authenticate with a Google AI subscription
crush login gemini

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
		"gemini",
		"antigravity",
		"gemini-sub",
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
		case "gemini", "antigravity", "gemini-sub":
			return loginGemini(ws, force)
		default:
			return fmt.Errorf("unknown platform: %s", args[0])
		}
	},
}

func init() {
	loginCmd.Flags().BoolP("force", "f", false, "Force re-authentication even if already logged in")
}

func loginHyper(ws workspace.Workspace, force bool) error {
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

	ctx := getLoginContext()
	token, err := login.Run(ctx, login.PlatformHyper)
	if err != nil {
		return err
	}

	if err := ws.SetProviderAPIKey(config.ScopeGlobal, "hyper", token); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with Hyper!")
	return nil
}

func loginCopilot(ws workspace.Workspace, force bool) error {
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

	ctx := getLoginContext()

	if diskToken, hasDiskToken := copilot.RefreshTokenFromDisk(); hasDiskToken {
		fmt.Println("Found existing GitHub Copilot token on disk. Using it to authenticate...")
		token, err := copilot.RefreshToken(ctx, diskToken)
		if err != nil {
			return fmt.Errorf("unable to refresh token from disk: %w", err)
		}
		if err := ws.SetProviderAPIKey(config.ScopeGlobal, "copilot", token); err != nil {
			return err
		}
		fmt.Println()
		fmt.Println("You're now authenticated with GitHub Copilot!")
		return nil
	}

	token, err := login.Run(ctx, login.PlatformCopilot)
	if err != nil {
		if errors.Is(err, copilot.ErrNotAvailable) {
			fmt.Println()
			fmt.Println("GitHub Copilot is unavailable for this account. To signup, go to the following page:")
			fmt.Println()
			lipgloss.Println(lipgloss.NewStyle().Hyperlink(copilot.SignupURL, "id=copilot-signup").Render(copilot.SignupURL))
			fmt.Println()
			fmt.Println("You may be able to request free access if eligible. For more information, see:")
			fmt.Println()
			lipgloss.Println(lipgloss.NewStyle().Hyperlink(copilot.FreeURL, "id=copilot-free").Render(copilot.FreeURL))
		}
		return err
	}

	if err := ws.SetProviderAPIKey(config.ScopeGlobal, "copilot", token); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with GitHub Copilot!")
	return nil
}

func loginOpenAI(ws workspace.Workspace, force bool) error {
	if !force {
		cfg := ws.Config()
		if cfg != nil {
			if pc, ok := cfg.Providers.Get("openai"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to OpenAI with a ChatGPT account.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	ctx := getLoginContext()
	token, err := login.Run(ctx, login.PlatformOpenAI)
	if err != nil {
		return err
	}

	if err := ws.SetProviderAPIKey(config.ScopeGlobal, "openai", token); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with your ChatGPT account!")
	return nil
}

// loginGemini attaches a Google AI subscription.
//
// Someone already signed in with the Antigravity CLI does not need a second
// browser trip: that session is imported from the system keyring, and the
// browser flow only runs when there is nothing to import.
func loginGemini(ws workspace.Workspace, force bool) error {
	if !force {
		cfg := ws.Config()
		if cfg != nil {
			if pc, ok := cfg.Providers.Get(antigravity.ProviderID); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to a Google AI subscription.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	ctx := getLoginContext()

	// Every later refresh needs the client secret, so ask for it now rather
	// than letting an imported Antigravity session become unusable the first
	// time its access token lapses.
	secret, err := googleSubscriptionClientSecret(ws)
	if err != nil {
		return err
	}

	token, err := antigravity.ImportFromKeyring(ctx)
	if err == nil {
		fmt.Println("Found an existing Antigravity login. Importing it...")
	} else {
		if !errors.Is(err, antigravity.ErrNoKeyring) {
			fmt.Printf("Could not import an Antigravity login (%v); signing in instead.\n", err)
		}
		token, err = login.Run(ctx, login.PlatformGemini, login.WithClientSecret(secret))
		if err != nil {
			return err
		}
	}

	if err := ws.SetProviderAPIKey(config.ScopeGlobal, antigravity.ProviderID, token); err != nil {
		return err
	}

	fmt.Println()
	if token.AccountID != "" {
		fmt.Printf("You're now authenticated with your Google AI subscription (%s).\n", token.AccountID)
		return nil
	}
	fmt.Println("You're now authenticated with your Google AI subscription!")
	return nil
}

// googleSubscriptionClientSecret returns the OAuth client secret the Google
// subscription needs, prompting for it when nothing supplies it and then
// remembering the answer in the global configuration.
//
// The value is shared by every user of Google's CLI rather than personal, but
// it cannot be committed, so entering it once is what stands in for shipping
// it. Non-interactive runs get the error instead of a prompt, which keeps
// scripts from hanging.
func googleSubscriptionClientSecret(ws workspace.Workspace) (string, error) {
	if secret := ws.Config().GoogleSubscriptionClientSecret(); secret != "" {
		return secret, nil
	}
	if !term.IsTerminal(os.Stdin.Fd()) {
		return "", antigravity.ErrClientSecretRequired
	}

	fmt.Println("The Google AI subscription signs in as Google's Antigravity CLI, whose OAuth")
	fmt.Println("client secret is the same for everyone rather than personal. It is not one")
	fmt.Println("crush can ship, so you supply it once: paste it below, set")
	fmt.Println("providers.gemini-sub.oauth_client_secret in crush.json, or export")
	fmt.Println("CRUSH_ANTIGRAVITY_CLIENT_SECRET.")
	fmt.Println()
	fmt.Println("It is saved in " + config.GlobalConfigData() + " and asked for only once.")
	fmt.Print("Client secret (input hidden): ")
	read, err := term.ReadPassword(os.Stdin.Fd())
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read client secret: %w", err)
	}
	secret := strings.TrimSpace(string(read))
	if secret == "" {
		return "", antigravity.ErrClientSecretRequired
	}

	key := "providers." + antigravity.ProviderID + ".oauth_client_secret"
	if err := ws.SetConfigField(config.ScopeGlobal, key, secret); err != nil {
		return "", fmt.Errorf("save the client secret: %w", err)
	}
	fmt.Println("Saved " + key + ".")
	return secret, nil
}

func getLoginContext() context.Context {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return ctx
}
