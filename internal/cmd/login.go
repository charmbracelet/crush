package cmd

import (
	"bufio"
	"context"
	"errors"
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
	openaioauth "github.com/charmbracelet/crush/internal/oauth/openai"
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

# Authenticate with OpenAI Codex (ChatGPT OAuth)
crush login openai-codex
  `,
	ValidArgs: []cobra.Completion{
		"hyper",
		"copilot",
		"openai",
		"openai-codex",
		"codex",
		"chatgpt",
		"github",
		"github-copilot",
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
		case "openai", "openai-codex", "codex", "chatgpt":
			return loginOpenAICodex()
		default:
			return fmt.Errorf("unknown platform: %s", args[0])
		}
	},
}

func loginHyper() error {
	if !hyperp.Enabled() {
		return fmt.Errorf("hyper not enabled")
	}
	return loginOAuth(
		hyperp.Name,
		"Hyper",
		func(ctx context.Context) (*oauth.Token, error) {
			resp, err := hyper.InitiateDeviceAuth(ctx)
			if err != nil {
				return nil, err
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
				return nil, err
			}

			fmt.Println("Exchanging refresh token for access token...")
			token, err := hyper.ExchangeToken(ctx, refreshToken)
			if err != nil {
				return nil, err
			}

			fmt.Println("Verifying access token...")
			introspect, err := hyper.IntrospectToken(ctx, token.AccessToken)
			if err != nil {
				return nil, fmt.Errorf("token introspection failed: %w", err)
			}
			if !introspect.Active {
				return nil, fmt.Errorf("access token is not active")
			}
			return token, nil
		},
	)
}

func loginOAuth(providerID, providerName string, authFunc func(context.Context) (*oauth.Token, error)) error {
	cfg := config.Get()
	if cfg.HasConfigField(fmt.Sprintf("providers.%s.oauth", providerID)) {
		fmt.Printf("You are already logged in to %s.\n", providerName)
		return nil
	}

	ctx := getLoginContext()
	token, err := authFunc(ctx)
	if err != nil {
		return err
	}

	if err := cfg.SetProviderAPIKey(providerID, token); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("You're now authenticated with %s!\n", providerName)
	return nil
}

func loginCopilot() error {
	cfg := config.Get()
	if cfg.HasConfigField("providers.copilot.oauth") {
		fmt.Println("You are already logged in to GitHub Copilot.")
		return nil
	}

	diskToken, hasDiskToken := copilot.RefreshTokenFromDisk()
	return loginOAuth(
		"copilot",
		"GitHub Copilot",
		func(ctx context.Context) (*oauth.Token, error) {
			if hasDiskToken {
				fmt.Println("Found existing GitHub Copilot token on disk. Using it to authenticate...")
				token, err := copilot.RefreshToken(ctx, diskToken)
				if err != nil {
					return nil, fmt.Errorf("unable to refresh token from disk: %w", err)
				}
				return token, nil
			}
			return loginCopilotWithDeviceFlow(ctx)
		},
	)
}

func loginCopilotWithDeviceFlow(ctx context.Context) (*oauth.Token, error) {
	fmt.Println("Requesting device code from GitHub...")

	dc, err := copilot.RequestDeviceCode(ctx)
	if err != nil {
		return nil, err
	}

	fmt.Println()
	fmt.Println("Open the following URL and follow the instructions to authenticate with GitHub Copilot:")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(dc.VerificationURI, "id=copilot").Render(dc.VerificationURI))
	fmt.Println()
	fmt.Println("Code:", lipgloss.NewStyle().Bold(true).Render(dc.UserCode))
	fmt.Println()
	fmt.Println("Waiting for authorization...")

	token, err := copilot.PollForToken(ctx, dc)
	if err == copilot.ErrNotAvailable {
		fmt.Println()
		fmt.Println("GitHub Copilot is unavailable for this account. To signup, go to the following page:")
		fmt.Println()
		fmt.Println(lipgloss.NewStyle().Hyperlink(copilot.SignupURL, "id=copilot-signup").Render(copilot.SignupURL))
		fmt.Println()
		fmt.Println("You may be able to request free access if eligible. For more information, see:")
		fmt.Println()
		fmt.Println(lipgloss.NewStyle().Hyperlink(copilot.FreeURL, "id=copilot-free").Render(copilot.FreeURL))
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	return token, nil
}

func loginOpenAICodex() error {
	return loginOAuth(
		config.OpenAICodexProviderID,
		"OpenAI Codex",
		func(ctx context.Context) (*oauth.Token, error) {
			flow, err := openaioauth.CreateAuthorizationFlow()
			if err != nil {
				return nil, err
			}

			server, err := openaioauth.StartLocalServer(flow.State)
			if err != nil {
				fmt.Println("Could not start local callback server; falling back to manual login.")
				return loginOpenAICodexManual(ctx, flow)
			}
			defer server.Close()

			fmt.Println("Opening browser for OpenAI Codex authentication...")
			if err := browser.OpenURL(flow.URL); err != nil {
				fmt.Println("Could not open the browser. Use this URL to continue:")
				fmt.Println(flow.URL)
			}

			code, err := waitForOpenAICodexCode(ctx, server)
			if err != nil {
				fmt.Println("Timed out waiting for the OAuth callback; falling back to manual login.")
				return loginOpenAICodexManual(ctx, flow)
			}
			return openaioauth.ExchangeAuthorizationCode(ctx, code, flow.Verifier)
		},
	)
}

func waitForOpenAICodexCode(ctx context.Context, server *openaioauth.LocalServer) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	return server.WaitForCode(ctx)
}

func loginOpenAICodexManual(ctx context.Context, flow openaioauth.AuthFlow) (*oauth.Token, error) {
	fmt.Println("Open the following URL in your browser and complete the login:")
	fmt.Println()
	fmt.Println(flow.URL)
	fmt.Println()
	fmt.Println("Paste the full redirect URL (or just the code) and press enter:")
	codeInput, err := readLine()
	if err != nil {
		return nil, err
	}
	code, state := openaioauth.ParseAuthorizationInput(codeInput)
	if code == "" {
		return nil, fmt.Errorf("authorization code not provided")
	}
	if state != "" && state != flow.State {
		return nil, fmt.Errorf("authorization state does not match, please retry")
	}
	return openaioauth.ExchangeAuthorizationCode(ctx, code, flow.Verifier)
}

func readLine() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(value), nil
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
