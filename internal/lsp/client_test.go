package lsp

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	ctx := context.Background()

	// Create a simple config for testing
	cfg := config.LSPConfig{
		Command:   "$THE_CMD", // Use echo as a dummy command that won't fail
		Args:      []string{"hello"},
		FileTypes: []string{"go"},
		Env:       map[string]string{},
	}

	// Test creating a powernap client - this will likely fail with echo
	// but we can still test the basic structure
	client, err := New(ctx, "test", cfg, config.NewEnvironmentVariableResolver(env.NewFromMap(map[string]string{
		"THE_CMD": "echo",
	})), ".", false)
	if err != nil {
		// Expected to fail with echo command, skip the rest
		t.Skipf("Powernap client creation failed as expected with dummy command: %v", err)
		return
	}

	// If we get here, test basic interface methods
	if client.GetName() != "test" {
		t.Errorf("Expected name 'test', got '%s'", client.GetName())
	}

	if !client.HandlesFile("test.go") {
		t.Error("Expected client to handle .go files")
	}

	if client.HandlesFile("test.py") {
		t.Error("Expected client to not handle .py files")
	}

	// Test server state
	client.SetServerState(StateReady)
	if client.GetServerState() != StateReady {
		t.Error("Expected server state to be StateReady")
	}

	// Clean up - expect this to fail with echo command
	if err := client.Close(t.Context()); err != nil {
		// Expected to fail with echo command
		t.Logf("Close failed as expected with dummy command: %v", err)
	}
}

func TestNilClient(t *testing.T) {
	t.Parallel()

	var c *Client

	require.False(t, c.HandlesFile("/some/file.go"))
	require.Equal(t, DiagnosticCounts{}, c.GetDiagnosticCounts())
	require.Nil(t, c.GetDiagnostics())
	require.Nil(t, c.OpenFileOnDemand(context.Background(), "/some/file.go"))
	require.Nil(t, c.NotifyChange(context.Background(), "/some/file.go"))
	c.WaitForDiagnostics(context.Background(), time.Second)
}

func TestCapabilityEnabled(t *testing.T) {
	t.Parallel()

	require.False(t, capabilityEnabled(nil))
	require.False(t, capabilityEnabled(false))
	require.True(t, capabilityEnabled(true))
	require.True(t, capabilityEnabled(protocol.Or_ServerCapabilities_codeActionProvider{Value: true}))
	require.False(t, capabilityEnabled(protocol.Or_ServerCapabilities_codeActionProvider{Value: false}))
	require.True(t, capabilityEnabled(protocol.Or_ServerCapabilities_documentFormattingProvider{Value: protocol.DocumentFormattingOptions{}}))
	require.True(t, capabilityEnabled(protocol.Or_ServerCapabilities_renameProvider{Value: protocol.RenameOptions{}}))
}

func TestCodeActionResults(t *testing.T) {
	t.Parallel()

	command := protocol.Command{Title: "Run fix", Command: "fix.command"}
	actions := codeActionResults([]protocol.Or_Result_textDocument_codeAction_Item0_Elem{
		{Value: protocol.CodeAction{Title: "Use fmt.Errorf"}},
		{Value: command},
	})

	require.Len(t, actions, 2)
	require.Equal(t, "Use fmt.Errorf", actions[0].Title)
	require.Equal(t, "Run fix", actions[1].Title)
	require.NotNil(t, actions[1].Command)
	require.Equal(t, "fix.command", actions[1].Command.Command)
}
