package dialog_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

func TestAuthMethod_ToggleAndSelect(t *testing.T) {
	t.Parallel()

	com := &common.Common{}
	provider := catwalk.Provider{
		ID:   "openai",
		Name: "OpenAI",
	}
	model := config.SelectedModel{
		Provider: "openai",
		Model:    "gpt-4o",
	}

	dlg := dialog.NewAuthMethod(com, provider, model, config.SelectedModelTypeLarge)
	require.Equal(t, dialog.AuthMethodID, dlg.ID())

	// Default selection is OAuth (index 0)
	action := dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selectAction, ok := action.(dialog.ActionSelectAuthMethod)
	require.True(t, ok)
	require.True(t, selectAction.UseOAuth)
	require.Equal(t, "openai", string(selectAction.Provider.ID))

	// Down arrow toggles to API key (index 1)
	dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	action = dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selectAction, ok = action.(dialog.ActionSelectAuthMethod)
	require.True(t, ok)
	require.False(t, selectAction.UseOAuth)

	// Up arrow toggles back to OAuth (index 0)
	dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	action = dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selectAction, ok = action.(dialog.ActionSelectAuthMethod)
	require.True(t, ok)
	require.True(t, selectAction.UseOAuth)

	// Esc cancels dialog
	action = dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	_, ok = action.(dialog.ActionClose)
	require.True(t, ok)
}
