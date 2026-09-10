package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// stubWorkspace is a minimal Workspace whose only meaningful
// method is Config(). All other methods panic if called, so the
// test fails loudly if defaultCommands starts depending on them.
type stubWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (s *stubWorkspace) Config() *config.Config { return s.cfg }

// findSummaryCommand returns the "Switch Summary Model" item from
// the default commands list, or nil if absent.
func findSummaryCommand(commands []*CommandItem) *CommandItem {
	for _, cmd := range commands {
		if cmd.id == "switch_summary_model" {
			return cmd
		}
	}
	return nil
}

// TestSummaryCommandDescriptionWhenAbsent verifies that the command
// palette shows a warning description when no summary model is
// configured.
func TestSummaryCommandDescriptionWhenAbsent(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	cfg := &config.Config{Models: map[config.SelectedModelType]config.SelectedModel{}}
	com := &common.Common{Styles: &s, Workspace: &stubWorkspace{cfg: cfg}}

	cmds, err := NewCommands(com, "", false, false, false, nil, nil)
	require.NoError(t, err)

	summaryCmd := findSummaryCommand(cmds.defaultCommands())
	require.NotNil(t, summaryCmd, "Switch Summary Model command must exist")
	require.Contains(t, summaryCmd.Description(), "No summary model set")
}

// TestSummaryCommandDescriptionWhenConfigured verifies that the
// command palette shows a neutral description when a summary model
// is configured.
func TestSummaryCommandDescriptionWhenConfigured(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	cfg := &config.Config{
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeSummary: {Provider: "openai", Model: "gpt-4o"},
		},
	}
	com := &common.Common{Styles: &s, Workspace: &stubWorkspace{cfg: cfg}}

	cmds, err := NewCommands(com, "", false, false, false, nil, nil)
	require.NoError(t, err)

	summaryCmd := findSummaryCommand(cmds.defaultCommands())
	require.NotNil(t, summaryCmd, "Switch Summary Model command must exist")
	require.Equal(t, "Choose a model for notebook summary generation", summaryCmd.Description())
	require.NotContains(t, summaryCmd.Description(), "No summary model set")
}
