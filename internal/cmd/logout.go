package cmd

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/signal"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
)

var (
	logoutHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	logoutItemStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	logoutPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("215"))
)

var logoutCmd = &cobra.Command{
	Aliases: []string{"signout"},
	Use:     "logout [platform]",
	Short:   "Logout Crush from a platform",
	Long: `Logout Crush from a specified platform, removing stored credentials.
The platform should be provided as an argument.
If no argument is given, a list of logged-in platforms will be shown.
Available platforms are: hyper, copilot.`,
	Example: `
# Sign out from Charm Hyper
crush logout hyper

# Sign out from GitHub Copilot
crush logout copilot
  `,
	ValidArgs: []cobra.Completion{
		"hyper",
		"copilot",
		"github",
		"github-copilot",
	},
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, ws, cleanup, err := connectToServer(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		progressEnabled := ws.Config.Options.Progress == nil || *ws.Config.Options.Progress
		if progressEnabled && supportsProgressBar() {
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
			defer func() { _, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar) }()
		}

		var provider string
		if len(args) == 0 {
			provider, err = pickLoggedInProvider(c, ws.ID)
			if err != nil {
				return err
			}
			if provider == "" {
				return nil
			}
		} else {
			provider = args[0]
		}

		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Print(logoutPromptStyle.Render(fmt.Sprintf("Are you sure you want to logout %s? (y/N) ", provider)))
			var response string
			_, err := fmt.Scanln(&response)
			if err != nil || (response != "y" && response != "Y" && response != "yes" && response != "Yes" && response != "YES") {
				fmt.Println(logoutHeaderStyle.Render("Logout cancelled."))
				return nil
			}
		}

		switch provider {
		case "hyper":
			return logoutHyper(c, ws.ID)
		case "copilot", "github", "github-copilot":
			return logoutCopilot(c, ws.ID)
		default:
			return fmt.Errorf("unknown platform: %s", provider)
		}
	},
}

func logoutHyper(c *client.Client, wsID string) error {
	ctx := getLogoutContext()

	if err := cmp.Or(
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.hyper.api_key"),
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.hyper.oauth"),
	); err != nil {
		return err
	}

	fmt.Println(logoutHeaderStyle.Render("Successfully logged out of Hyper."))
	return nil
}

func logoutCopilot(c *client.Client, wsID string) error {
	ctx := getLogoutContext()

	if err := cmp.Or(
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.copilot.api_key"),
		c.RemoveConfigField(ctx, wsID, config.ScopeGlobal, "providers.copilot.oauth"),
	); err != nil {
		return err
	}

	fmt.Println(logoutHeaderStyle.Render("Successfully logged out of GitHub Copilot."))
	return nil
}

func pickLoggedInProvider(c *client.Client, wsID string) (string, error) {
	ctx := getLogoutContext()

	cfg, err := c.GetConfig(ctx, wsID)
	if err != nil {
		return "", fmt.Errorf("failed to get config: %w", err)
	}

	type loggedInProvider struct {
		id   string
		name string
	}

	// Only OAuth-based providers support login/logout. Keep this list in sync
	// with the switch in RunE and the login command.
	oauthProviders := map[string]string{
		"hyper":   "Hyper",
		"copilot": "GitHub Copilot",
	}

	var loggedIn []loggedInProvider
	for id, name := range oauthProviders {
		if p, ok := cfg.Providers.Get(id); ok && p.OAuthToken != nil {
			loggedIn = append(loggedIn, loggedInProvider{id: id, name: name})
		}
	}

	if len(loggedIn) == 0 {
		fmt.Println(logoutPromptStyle.Render("You are not logged in to any platform."))
		return "", nil
	}

	if len(loggedIn) == 1 {
		return loggedIn[0].id, nil
	}

	fmt.Println(logoutHeaderStyle.Render("Logged-in platforms:"))
	for i, p := range loggedIn {
		fmt.Println(logoutItemStyle.Render(fmt.Sprintf("  %d. %s", i+1, p.name)))
	}
	fmt.Print(logoutPromptStyle.Render(fmt.Sprintf("Select a platform to logout (1-%d): ", len(loggedIn))))

	var choice int
	_, err = fmt.Scanln(&choice)
	if err != nil || choice < 1 || choice > len(loggedIn) {
		fmt.Println(logoutHeaderStyle.Render("Logout cancelled."))
		return "", nil
	}

	return loggedIn[choice-1].id, nil
}

func init() {
	logoutCmd.Flags().BoolP("force", "f", false, "Skip logout confirmation prompt")
}

func getLogoutContext() context.Context {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	go func() {
		<-ctx.Done()
		cancel()
		os.Exit(1)
	}()
	return ctx
}
