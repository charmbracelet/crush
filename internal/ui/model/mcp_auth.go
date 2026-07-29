package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

// isAuthTimeout reports whether an error indicates the OAuth flow was
// cancelled or timed out. The SDK wraps context errors inside its own
// messages (e.g. "authorization cancelled: context canceled"), so we
// check both the error chain and the message text.
func isAuthTimeout(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "authorization cancelled")
}

// authenticateMCP runs the OAuth flow for a named MCP server using the
// provided context. The dialog owns the context and cancels it if the
// user closes the dialog.
func (m *UI) authenticateMCP(ctx context.Context, name string) tea.Cmd {
	return func() tea.Msg {
		if err := m.com.Workspace.MCPAuthenticate(ctx, name); err != nil {
			if isAuthTimeout(err) {
				return dialog.ActionMCPAuthErrored{Name: name, Error: fmt.Errorf("authentication timed out")}
			}
			return dialog.ActionMCPAuthErrored{Name: name, Error: err}
		}
		return dialog.ActionMCPAuthComplete{Name: name}
	}
}

// openMCPAuthDialog opens the MCP authentication dialog if any servers
// are pending auth. If the dialog is already open, it brings it to the
// front instead.
func (m *UI) openMCPAuthDialog() tea.Cmd {
	pending := m.com.Workspace.MCPPendingAuth()
	if len(pending) == 0 {
		return nil
	}
	if m.dialog.ContainsDialog(dialog.MCPAuthID) {
		m.dialog.BringToFront(dialog.MCPAuthID)
		return nil
	}
	dlg, cmd := dialog.NewMCPAuth(m.com, pending, m.com.Workspace.MCPAuthURL)
	m.dialog.OpenDialog(dlg)
	return cmd
}

// checkPendingMCPAuth waits for MCP initialization to finish and then
// checks whether any OAuth MCPs need authentication. This runs as a
// Bubble Tea command so it doesn't block the UI.
func (m *UI) checkPendingMCPAuth() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := mcp.WaitForInit(ctx); err != nil {
			return nil
		}
		return mcpStateChangedMsg{
			states: m.com.Workspace.MCPGetStates(),
		}
	}
}
