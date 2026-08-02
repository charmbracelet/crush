package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type stubWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *stubWorkspace) Config() *config.Config {
	return w.cfg
}

func TestCommands_DefaultCommands_IncludesWorkflowProgress(t *testing.T) {
	s := styles.CharmtonePantera()
	cfg := &config.Config{}
	com := &common.Common{
		Styles:    &s,
		Workspace: &stubWorkspace{cfg: cfg},
	}

	cmdsDialog, err := NewCommands(com, "session_123", true, false, false, nil, nil)
	require.NoError(t, err)

	items := cmdsDialog.defaultCommands()
	found := false
	for _, item := range items {
		if item.id == "workflow_progress" {
			found = true
			require.Equal(t, "Workflow Progress", item.title)
			require.Equal(t, "ctrl+w", item.shortcut)
			require.Equal(t, ActionOpenDialog{DialogID: WorkflowPopupID}, item.action)
			break
		}
	}
	require.True(t, found, "expected workflow_progress command in default system commands")
}
