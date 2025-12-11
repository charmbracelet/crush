// Package lazygit provides a dialog component for embedding lazygit in the TUI.
package lazygit

import (
	"context"
	"fmt"
	"image/color"
	"os"

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
	configFile := createThemedConfig()

	cmd := terminal.PrepareCmd(
		ctx,
		"lazygit",
		nil,
		workingDir,
		[]string{"LG_CONFIG_FILE=" + configFile},
	)

	return termdialog.New(termdialog.Config{
		ID:         DialogID,
		Title:      "Lazygit",
		LoadingMsg: "Starting lazygit...",
		Term:       terminal.New(terminal.Config{Context: ctx, Cmd: cmd}),
		OnClose: func() {
			if configFile != "" {
				_ = os.Remove(configFile)
			}
		},
	})
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

	config := fmt.Sprintf(`gui:
  border: rounded
  showFileTree: true
  showRandomTip: false
  showCommandLog: false
  showBottomLine: true
  showPanelJumps: false
  nerdFontsVersion: ""
  showFileIcons: false
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
		colorToHex(t.BorderFocus), // Active border: purple (Charple)
		colorToHex(t.Border),      // Inactive border: gray (Charcoal)
		colorToHex(t.Info),        // Search border: blue (Malibu) - calmer than warning
		colorToHex(t.FgMuted),     // Options text: muted gray (Squid) - matches help text
		colorToHex(t.Primary),     // Selected line bg: purple (Charple) - matches TextSelected
		colorToHex(t.BgSubtle),    // Inactive selected: subtle gray (Charcoal)
		colorToHex(t.Success),     // Cherry-picked fg: green (Guac) - positive action
		colorToHex(t.BgSubtle),    // Cherry-picked bg: subtle (Charcoal)
		colorToHex(t.Info),        // Marked base fg: blue (Malibu) - distinct from cherry
		colorToHex(t.BgSubtle),    // Marked base bg: subtle (Charcoal)
		colorToHex(t.Error),       // Unstaged changes: red (Sriracha)
	)

	f, err := os.CreateTemp("", "crush-lazygit-*.yml")
	if err != nil {
		return ""
	}
	defer f.Close()

	_, _ = f.WriteString(config)
	return f.Name()
}
