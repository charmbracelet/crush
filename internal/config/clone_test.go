package config

import (
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

// TestCloneForWrite_Isolation verifies that mutating a clone never reaches
// back into the original Config. This is the contract the store's
// copy-on-write mutators depend on for race-free publishing.
func TestCloneForWrite_Isolation(t *testing.T) {
	t.Parallel()

	orig := &Config{
		Models: map[SelectedModelType]SelectedModel{
			SelectedModelTypeLarge: {Provider: "openai", Model: "gpt-4"},
		},
		RecentModels: map[SelectedModelType][]SelectedModel{
			SelectedModelTypeLarge: {{Provider: "openai", Model: "gpt-4"}},
		},
		MCP:       MCPs{"a": {}},
		Providers: csync.NewMap[string, ProviderConfig](),
		Options: &Options{
			TUI: &TUIOptions{CompactMode: false},
		},
	}

	clone := orig.cloneForWrite()

	// Mutate every field the typed mutators touch.
	clone.Models[SelectedModelTypeLarge] = SelectedModel{Provider: "anthropic", Model: "claude"}
	clone.RecentModels[SelectedModelTypeLarge] = []SelectedModel{{Provider: "anthropic", Model: "claude"}}
	clone.MCP["b"] = MCPConfig{}
	clone.Options.TUI.CompactMode = true
	enabled := true
	clone.Options.TUI.Transparent = &enabled

	// The original must be untouched.
	require.Equal(t, "openai", orig.Models[SelectedModelTypeLarge].Provider, "Models leaked")
	require.Equal(t, "openai", orig.RecentModels[SelectedModelTypeLarge][0].Provider, "RecentModels leaked")
	require.NotContains(t, orig.MCP, "b", "MCP leaked")
	require.False(t, orig.Options.TUI.CompactMode, "Options.TUI.CompactMode leaked")
	require.Nil(t, orig.Options.TUI.Transparent, "Options.TUI.Transparent leaked")
}
