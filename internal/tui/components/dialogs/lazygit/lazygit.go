// Package lazygit provides a dialog component for embedding lazygit in the TUI.
package lazygit

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/termdialog"
	"github.com/charmbracelet/crush/internal/tui/styles"
)

// DialogID is the unique identifier for the lazygit dialog.
const DialogID dialogs.DialogID = "lazygit"

// NewDialog creates a new lazygit dialog. The context controls the lifetime
// of the lazygit process - when cancelled, the process will be killed.
func NewDialog(ctx context.Context, workingDir string) *termdialog.Dialog {
	themeConfig := createThemedConfig()
	configEnv := buildConfigEnv(themeConfig)

	cmd := terminal.PrepareCmd(
		ctx,
		"lazygit",
		nil,
		workingDir,
		[]string{configEnv},
	)

	return termdialog.New(termdialog.Config{
		ID:         DialogID,
		Title:      "Lazygit",
		LoadingMsg: "Starting lazygit...",
		Term:       terminal.New(terminal.Config{Context: ctx, Cmd: cmd}),
		OnClose: func() {
			if themeConfig != "" {
				_ = os.Remove(themeConfig)
			}
		},
	})
}

// buildConfigEnv builds the LG_CONFIG_FILE env var, merging user's default
// config (if it exists) with our theme override. User config comes first so
// our theme settings take precedence.
func buildConfigEnv(themeConfig string) string {
	userConfig := defaultConfigPath()
	if userConfig != "" {
		if _, err := os.Stat(userConfig); err == nil {
			return "LG_CONFIG_FILE=" + userConfig + "," + themeConfig
		}
	}
	return "LG_CONFIG_FILE=" + themeConfig
}

// defaultConfigPath returns the default lazygit config path for the current OS.
func defaultConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "lazygit", "config.yml")
}

// colorToHex converts a color.Color to a hex string.
func colorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// createThemedConfig creates a temporary lazygit config file with Crush theme.
// Theme mappings align with Crush's UX patterns:
// - Borders: BorderFocus (purple) for active, Border (gray) for inactive
// - Selection: Primary (purple) background matches app's TextSelected style
// - Status: Success (green), Error (red), Info (blue), Warning (orange)
func createThemedConfig() string {
	t := styles.CurrentTheme()

	config := fmt.Sprintf(`git:
  autoFetch: true
gui:
  theme:
    activeBorderColor:
      - "%s"
      - bold
    inactiveBorderColor:
      - "%s"
    searchingActiveBorderColor:
      - "%s"
      - bold
    optionsTextColor:
      - "%s"
    selectedLineBgColor:
      - "%s"
    inactiveViewSelectedLineBgColor:
      - "%s"
    cherryPickedCommitFgColor:
      - "%s"
    cherryPickedCommitBgColor:
      - "%s"
    markedBaseCommitFgColor:
      - "%s"
    markedBaseCommitBgColor:
      - "%s"
    unstagedChangesColor:
      - "%s"
    defaultFgColor:
      - default
`,
		colorToHex(t.BorderFocus),
		colorToHex(t.FgMuted),
		colorToHex(t.Info),
		colorToHex(t.FgMuted),
		colorToHex(t.Primary),
		colorToHex(t.BgSubtle),
		colorToHex(t.Success),
		colorToHex(t.BgSubtle),
		colorToHex(t.Info),
		colorToHex(t.BgSubtle),
		colorToHex(t.Error),
	)

	f, err := os.CreateTemp("", "crush-lazygit-*.yml")
	if err != nil {
		return ""
	}
	defer f.Close()

	_, _ = f.WriteString(config)
	return f.Name()
}
