package backend

import (
	"context"

	tea "charm.land/bubbletea/v2"

	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
)

// SubscribeEvents returns the event channel for a workspace's app.
func (b *Backend) SubscribeEvents(workspaceID string) (<-chan tea.Msg, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Events(), nil
}

// GetLSPStates returns the state of all LSP clients.
func (b *Backend) GetLSPStates(workspaceID string) (map[string]app.LSPClientInfo, error) {
	_, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return app.GetLSPStates(), nil
}

// GetLSPDiagnostics returns diagnostics for a specific LSP client in
// the workspace.
func (b *Backend) GetLSPDiagnostics(workspaceID, lspName string) (any, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	for name, client := range ws.LSPManager.Clients().Seq2() {
		if name == lspName {
			return client.GetDiagnostics(), nil
		}
	}

	return nil, ErrLSPClientNotFound
}

// GetWorkspaceConfig returns the workspace-level configuration.
func (b *Backend) GetWorkspaceConfig(workspaceID string) (*config.Config, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Cfg.Config(), nil
}

// GetWorkspaceProviders returns the configured providers for a
// workspace.
func (b *Backend) GetWorkspaceProviders(workspaceID string) (any, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	providers, _ := config.Providers(ws.Cfg.Config())
	return providers, nil
}

// LSPStart starts an LSP server for the given path.
func (b *Backend) LSPStart(ctx context.Context, workspaceID, path string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	ws.LSPManager.Start(ctx, path)
	return nil
}

// LSPStopAll stops all LSP servers for a workspace.
func (b *Backend) LSPStopAll(ctx context.Context, workspaceID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	ws.LSPManager.StopAll(ctx)
	return nil
}

// MCPGetStates returns the current state of all MCP clients.
func (b *Backend) MCPGetStates(_ string) map[string]mcptools.ClientInfo {
	return mcptools.GetStates()
}

// MCPRefreshPrompts refreshes prompts for a named MCP client.
func (b *Backend) MCPRefreshPrompts(ctx context.Context, _ string, name string) {
	mcptools.RefreshPrompts(ctx, name)
}

// MCPRefreshResources refreshes resources for a named MCP client.
func (b *Backend) MCPRefreshResources(ctx context.Context, _ string, name string) {
	mcptools.RefreshResources(ctx, name)
}
