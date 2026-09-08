package model

import (
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

// Remapping an action must reach the help line: the status bar derives
// from live bindings, not literals.
func TestHelpHonesty_ShortHelpShowsRemappedKeys(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.keyMap = DefaultKeyMap()

	warnings := ApplyKeybinds(&u.keyMap, map[string][]string{
		"global.tab": {"ctrl+t"},
	})
	require.Empty(t, warnings)

	for _, b := range u.ShortHelp() {
		if b.Help().Desc == "focus chat" {
			require.Equal(t, "ctrl+t", b.Help().Key)
			return
		}
	}
	t.Fatal("tab binding missing from short help")
}

// The commands hint composes both trigger keys from live bindings.
func TestHelpHonesty_CommandsHintFromLiveBindings(t *testing.T) {
	t.Parallel()

	u := newTestUIForHelp()
	u.focus = uiFocusEditor

	found := false
	for _, b := range u.FullHelp() {
		for _, binding := range b {
			if binding.Help().Desc == "commands" {
				require.Equal(t, "/ or ctrl+p", binding.Help().Key)
				found = true
			}
		}
	}
	require.True(t, found, "commands binding missing from full help")
}

// newTestUIForHelp builds a UI with the minimum FullHelp touches:
// textarea, attachments, a workspace-backed config, and a default
// keymap.
func newTestUIForHelp() *UI {
	u := newTestUI()
	u.keyMap = DefaultKeyMap()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.com = common.DefaultCommon(&testWorkspace{cfg: &config.Config{
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "test-provider", Model: "test-model"},
		},
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Agents:    map[string]config.Agent{config.AgentCoder: {Model: config.SelectedModelTypeLarge}},
	}})
	return u
}

// Unbound bindings render an empty help key instead of panicking.
func TestHelpHonesty_UnboundBindingKeepsEmptyKey(t *testing.T) {
	t.Parallel()

	b := key.NewBinding()
	require.Equal(t, "", firstKey(b))
}
