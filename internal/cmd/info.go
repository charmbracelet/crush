package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show configuration information",
	Long:  `Display information about the current configuration including the active config file, log path, and configured providers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		cfg, err := config.Load(cwd, false)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %v", err)
		}

		// Initialize styles
		t := styles.CurrentTheme()
		const maxWidth = 80

		// Determine which config file is being used
		configFile := findActiveConfigFile(cfg.WorkingDir())

		// Build styled output
		var sections []string

		// Title
		title := core.Title("Configuration Information", maxWidth)
		sections = append(sections, title)

		// Configuration details section
		configSection := renderConfigSection(t, configFile, cfg, maxWidth)
		sections = append(sections, "", configSection)

		// Providers section
		providerSection := renderProvidersSection(t, cfg, maxWidth)
		sections = append(sections, "", providerSection)

		// LSP section
		if len(cfg.LSP) > 0 {
			lspSection := renderLSPSection(t, cfg, maxWidth)
			sections = append(sections, "", lspSection)
		}

		// MCP section
		if len(cfg.MCP) > 0 {
			mcpSection := renderMCPSection(t, cfg, maxWidth)
			sections = append(sections, "", mcpSection)
		}

		// Output the styled content
		output := lipgloss.JoinVertical(lipgloss.Left, sections...)
		fmt.Println(output)

		return nil
	},
}

// findActiveConfigFile determines which configuration file is actually being used
func findActiveConfigFile(workingDir string) string {
	// Check configuration files in order of precedence
	configPaths := config.GetConfigPaths(workingDir)

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "No configuration file found (using defaults)"
}

// renderConfigSection renders the configuration details section with styling
func renderConfigSection(t *styles.Theme, configFile string, cfg *config.Config, maxWidth int) string {
	// Build config details
	var details []string

	details = append(details,
		fmt.Sprintf("%s %s",
			t.S().Subtle.Render("Configuration File:"),
			t.S().Text.Render(configFile)))

	// Log path
	logPath := config.LogPath(cfg.Options.DataDirectory)

	details = append(details,
		fmt.Sprintf("%s %s",
			t.S().Subtle.Render("Log Path:"),
			t.S().Text.Render(logPath)))

	details = append(details,
		fmt.Sprintf("%s %s",
			t.S().Subtle.Render("Working Directory:"),
			t.S().Text.Render(cfg.WorkingDir())))

	details = append(details,
		fmt.Sprintf("%s %s",
			t.S().Subtle.Render("Data Directory:"),
			t.S().Text.Render(cfg.Options.DataDirectory)))

	return lipgloss.JoinVertical(lipgloss.Left, details...)
}

// renderProvidersSection renders the providers section with styling
func renderProvidersSection(t *styles.Theme, cfg *config.Config, maxWidth int) string {
	sectionTitle := core.Section("Providers", maxWidth)

	if cfg.Providers.Len() == 0 {
		noProviders := t.S().Muted.Render("  No providers configured")
		return lipgloss.JoinVertical(lipgloss.Left, sectionTitle, "", noProviders)
	}

	var providers []string
	for provider := range cfg.Providers.Seq() {
		var statusColor lipgloss.Style
		var statusText string

		if provider.Disable {
			statusColor = t.S().Muted
			statusText = "disabled"
		} else {
			statusColor = t.S().Success
			statusText = "enabled"
		}

		providerLine := fmt.Sprintf("  %s %s %s",
			t.S().Text.Render("•"),
			t.S().Title.Render(fmt.Sprintf("%s (%s):", provider.Name, provider.ID)),
			statusColor.Render(statusText))

		providers = append(providers, providerLine)

		if provider.BaseURL != "" {
			urlLine := fmt.Sprintf("    %s %s",
				t.S().Subtle.Render("URL:"),
				t.S().Text.Render(provider.BaseURL))
			providers = append(providers, urlLine)
		}

		modelLine := fmt.Sprintf("    %s %s",
			t.S().Subtle.Render("Models:"),
			t.S().Text.Render(fmt.Sprintf("%d", len(provider.Models))))
		providers = append(providers, modelLine)
	}

	return lipgloss.JoinVertical(lipgloss.Left, append([]string{sectionTitle, ""}, providers...)...)
}

// renderLSPSection renders the LSP servers section with styling
func renderLSPSection(t *styles.Theme, cfg *config.Config, maxWidth int) string {
	sectionTitle := core.Section("Language Servers", maxWidth)

	var lsps []string
	for _, lsp := range cfg.LSP.Sorted() {
		var statusColor lipgloss.Style
		var statusText string

		if lsp.LSP.Disabled {
			statusColor = t.S().Muted
			statusText = "disabled"
		} else {
			statusColor = t.S().Success
			statusText = "enabled"
		}

		lspLine := fmt.Sprintf("  %s %s %s",
			t.S().Text.Render("•"),
			t.S().Title.Render(fmt.Sprintf("%s (%s):", lsp.Name, lsp.LSP.Command)),
			statusColor.Render(statusText))

		lsps = append(lsps, lspLine)
	}

	return lipgloss.JoinVertical(lipgloss.Left, append([]string{sectionTitle, ""}, lsps...)...)
}

// renderMCPSection renders the MCP servers section with styling
func renderMCPSection(t *styles.Theme, cfg *config.Config, maxWidth int) string {
	sectionTitle := core.Section("MCP Servers", maxWidth)

	var mcps []string
	for _, mcp := range cfg.MCP.Sorted() {
		var statusColor lipgloss.Style
		var statusText string

		if mcp.MCP.Disabled {
			statusColor = t.S().Muted
			statusText = "disabled"
		} else {
			statusColor = t.S().Success
			statusText = "enabled"
		}

		mcpLine := fmt.Sprintf("  %s %s %s",
			t.S().Text.Render("•"),
			t.S().Title.Render(fmt.Sprintf("%s (%s):", mcp.Name, mcp.MCP.Type)),
			statusColor.Render(statusText))

		mcps = append(mcps, mcpLine)
	}

	return lipgloss.JoinVertical(lipgloss.Left, append([]string{sectionTitle, ""}, mcps...)...)
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
