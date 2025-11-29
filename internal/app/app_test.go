package app

import (
	"io"
	"log/slog"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	exitVal := m.Run()
	os.Exit(exitVal)
}

// TestUpdateAgentModel_NilCoordinator verifies that UpdateAgentModel handles
// a nil AgentCoordinator gracefully. This occurs when MCP servers are
// configured but no AI agents are set up.
func TestUpdateAgentModel_NilCoordinator(t *testing.T) {
	app := &App{
		AgentCoordinator: nil,
		events:           make(chan tea.Msg, 100),
	}

	err := app.UpdateAgentModel(t.Context())
	require.NoError(t, err)
}
