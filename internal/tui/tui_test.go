package tui

import (
	"io"
	"log/slog"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	exitVal := m.Run()
	os.Exit(exitVal)
}

// TestMCPEventWithUnconfiguredApp is an integration test verifying that we can
// start up the TUI properly when there are MCP servers but no AI agents
// configured.
func TestMCPEventWithUnconfiguredApp(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Avoid touching any real config or database
	conn, err := driver.Open(":memory:", func(c *sqlite3.Conn) error {
		return c.Exec("PRAGMA foreign_keys = ON;")
	})
	require.NoError(t, err)
	defer conn.Close()

	cfg, err := config.Init(t.TempDir(), t.TempDir(), false)
	require.NoError(t, err)

	cfg.MCP = map[string]config.MCPConfig{
		"test-mcp": {
			Type:     config.MCPStdio,
			Command:  "echo",
			Disabled: true,
		},
	}

	require.False(t, cfg.IsConfigured())

	mcpEvents := mcp.SubscribeEvents(ctx)

	appInstance, err := app.New(ctx, conn, cfg)
	require.NoError(t, err)
	defer appInstance.Shutdown()

	require.Nil(t, appInstance.AgentCoordinator)

	ui := New(appInstance)
	program := tea.NewProgram(ui, tea.WithContext(ctx), tea.WithoutRenderer())

	go appInstance.Subscribe(program)

	// Wait for MCP event, then quit
	go func() {
		for ev := range mcpEvents {
			if ev.Payload.Type != mcp.EventStateChanged {
				continue
			}

			program.Send(tea.Quit())
			return
		}
	}()

	_, err = program.Run()
	require.NoError(t, err)
}
