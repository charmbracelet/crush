package workspace

import (
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/require"
)

func TestAppWorkspace_ActiveSubagents_NilManagerReturnsNil(t *testing.T) {
	t.Parallel()

	w := &AppWorkspace{app: &app.App{}}
	require.Nil(t, w.ActiveSubagents())
}

func TestAppWorkspace_ActiveSubagents_MapsManagerOutput(t *testing.T) {
	t.Parallel()

	mgr := subagents.NewManager(nil, []*subagents.Subagent{
		{Name: "code-reviewer", Description: "Reviews code."},
		{Name: "tester", Description: "Writes tests."},
	}, nil)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{app: &app.App{Subagents: mgr}}

	got := w.ActiveSubagents()
	require.Len(t, got, 2)
	require.Equal(t, "code-reviewer", got[0].Name)
	require.Equal(t, "Reviews code.", got[0].Description)
	require.Equal(t, "tester", got[1].Name)
	require.Equal(t, "Writes tests.", got[1].Description)
}

func TestAppWorkspace_ActiveSubagents_EmptyManagerReturnsEmpty(t *testing.T) {
	t.Parallel()

	mgr := subagents.NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{app: &app.App{Subagents: mgr}}
	require.Empty(t, w.ActiveSubagents())
}

func TestClientWorkspace_ActiveSubagents_AlwaysNil(t *testing.T) {
	t.Parallel()

	w := &ClientWorkspace{}
	require.Nil(t, w.ActiveSubagents())
}
