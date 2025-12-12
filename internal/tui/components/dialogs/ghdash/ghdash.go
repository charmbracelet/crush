// Package ghdash provides a dialog component for embedding gh-dash in the TUI.
package ghdash

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

// DialogID is the unique identifier for the gh-dash dialog.
const DialogID dialogs.DialogID = "ghdash"

// NewDialog creates a new gh-dash dialog. The context controls the lifetime
// of the gh-dash process - when cancelled, the process will be killed.
func NewDialog(ctx context.Context, workingDir string) *termdialog.Dialog {
	configFile := createThemedConfig()

	cmd := terminal.PrepareCmd(
		ctx,
		"gh",
		[]string{"dash", "--config", configFile},
		workingDir,
		nil,
	)

	return termdialog.New(termdialog.Config{
		ID:         DialogID,
		Title:      "GitHub Dashboard",
		LoadingMsg: "Starting gh-dash...",
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

// createThemedConfig creates a temporary gh-dash config file with Crush theme.
func createThemedConfig() string {
	t := styles.CurrentTheme()

	config := fmt.Sprintf(`theme:
  colors:
    text:
      primary: "%s"
      secondary: "%s"
      inverted: "%s"
      faint: "%s"
      warning: "%s"
      success: "%s"
      error: "%s"
    background:
      selected: "%s"
    border:
      primary: "%s"
      secondary: "%s"
      faint: "%s"
`,
		colorToHex(t.FgBase),
		colorToHex(t.FgMuted),
		colorToHex(t.FgSelected),
		colorToHex(t.FgSubtle),
		colorToHex(t.Warning),
		colorToHex(t.Success),
		colorToHex(t.Error),
		colorToHex(t.Primary),
		colorToHex(t.BorderFocus),
		colorToHex(t.FgMuted),
		colorToHex(t.BgSubtle),
	)

	f, err := os.CreateTemp("", "crush-ghdash-*.yml")
	if err != nil {
		return ""
	}
	defer f.Close()

	_, _ = f.WriteString(config)
	return f.Name()
}
