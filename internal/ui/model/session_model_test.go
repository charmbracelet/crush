package model

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// saveModelWorkspace records the order saveCoderModel touches the workspace.
type saveModelWorkspace struct {
	workspace.Workspace
	calls  []string
	pinErr error
}

func (w *saveModelWorkspace) SetSessionModel(_ context.Context, sessionID string, _ config.SelectedModel) error {
	w.calls = append(w.calls, "pin:"+sessionID)
	return w.pinErr
}

func (w *saveModelWorkspace) UpdatePreferredModel(config.Scope, config.SelectedModelType, config.SelectedModel) error {
	w.calls = append(w.calls, "default")
	return nil
}

func (w *saveModelWorkspace) UpdateAgentModel(context.Context) error {
	w.calls = append(w.calls, "rebuild")
	return nil
}

// A settings change lands on the current session before it becomes the
// default, so a selection that cannot serve changes nothing.
func TestSaveCoderModelPinsSessionFirst(t *testing.T) {
	t.Parallel()
	sel := config.SelectedModel{Provider: "p", Model: "m", Think: true}

	ws := &saveModelWorkspace{}
	require.NoError(t, saveCoderModel(ws, "s1", sel))
	require.Equal(t, []string{"pin:s1", "default", "rebuild"}, ws.calls)

	ws = &saveModelWorkspace{}
	require.NoError(t, saveCoderModel(ws, "", sel))
	require.Equal(t, []string{"default", "rebuild"}, ws.calls, "with no session only the default changes")

	ws = &saveModelWorkspace{pinErr: errors.New("cannot build")}
	require.Error(t, saveCoderModel(ws, "s1", sel))
	require.Equal(t, []string{"pin:s1"}, ws.calls, "a refused pin leaves the default alone")
}
