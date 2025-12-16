// Package tuieditor provides a dialog component for embedding terminal-based
// editors (vim, nvim, nano, etc.) in the TUI.
package tuieditor

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/termdialog"
	"github.com/charmbracelet/crush/internal/tui/util"
)

// DialogID is the unique identifier for the embedded editor dialog.
const DialogID dialogs.DialogID = "tui_editor"

// EditorResultMsg is sent when the embedded editor closes with the file content.
type EditorResultMsg struct {
	Content string
	Err     error
}

// knownTUIEditors is a list of terminal-based editors that can be embedded.
var knownTUIEditors = []string{
	"vim",
	"nvim",
	"vi",
	"nano",
	"helix",
	"hx",
	"micro",
	"emacs",
	"joe",
	"ne",
	"jed",
	"kak",
	"pico",
	"mcedit",
	"mg",
	"zile",
}

// IsTUIEditor returns true if the given editor command is a known TUI editor.
func IsTUIEditor(editor string) bool {
	base := filepath.Base(editor)
	if idx := strings.Index(base, " "); idx != -1 {
		base = base[:idx]
	}
	return slices.Contains(knownTUIEditors, base)
}

// Config holds configuration for the embedded editor dialog.
type Config struct {
	// FilePath is the path to the file to edit.
	FilePath string
	// Editor is the editor command to use.
	Editor string
	// WorkingDir is the working directory for the editor.
	WorkingDir string
}

// NewDialog creates a new embedded editor dialog. The context controls the
// lifetime of the editor process - when cancelled, the process will be killed.
// When the editor exits, an EditorResultMsg is emitted with the file content.
func NewDialog(ctx context.Context, cfg Config) *termdialog.Dialog {
	editorCmd := cfg.Editor
	if editorCmd == "" {
		editorCmd = "nvim"
	}

	parts := strings.Fields(editorCmd)
	cmdName := parts[0]
	args := append(parts[1:], cfg.FilePath)

	cmd := terminal.PrepareCmd(
		ctx,
		cmdName,
		args,
		cfg.WorkingDir,
		nil,
	)

	filePath := cfg.FilePath

	return termdialog.New(termdialog.Config{
		ID:         DialogID,
		Title:      "Editor",
		LoadingMsg: "Starting editor...",
		Term:       terminal.New(terminal.Config{Context: ctx, Cmd: cmd}),
		OnClose: func() tea.Cmd {
			content, err := os.ReadFile(filePath)
			_ = os.Remove(filePath)

			if err != nil {
				return util.CmdHandler(EditorResultMsg{Err: err})
			}
			return util.CmdHandler(EditorResultMsg{
				Content: strings.TrimSpace(string(content)),
			})
		},
	})
}
