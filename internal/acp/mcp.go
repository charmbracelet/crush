package acp

import (
	"context"
	"fmt"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	acp "github.com/coder/acp-go-sdk"
)

// ConnectMCPServers exposes the session-scoped MCP setup to the dispatcher,
// which serves session/load itself. The request list is authoritative for the
// session, so servers it already had are replaced rather than merged: the spec
// defines a supplied list as the complete resulting set.
func (a *crushAgent) ConnectMCPServers(ctx context.Context, sessionID string, servers []acp.McpServer) error {
	mcp.TeardownSession(sessionID)
	return a.connectSessionMCPServers(ctx, sessionID, servers)
}

// connectSessionMCPServers starts the MCP servers an ACP session asked for in
// its setup request.
//
// The spec makes stdio a baseline every Agent must support
// (session-setup.md:458,466), so ignoring the field silently — which is what
// Crush did until now — left the client believing its servers were connected.
//
// Each server is registered under a name unique to the session
// (mcp.ACPServerName). Every registry in the MCP package is keyed by name
// alone, so two sessions asking for a server called "github" would otherwise
// overwrite each other's connection; unique names sidestep that without
// changing the key type.
//
// The servers run on a context detached from the request, because the request
// context is cancelled the moment the session/new response is written and the
// connection must outlive it. TeardownSession closes them on session/close.
func (a *crushAgent) connectSessionMCPServers(ctx context.Context, sessionID string, servers []acp.McpServer) error {
	if len(servers) == 0 {
		return nil
	}
	store := a.store()
	if store == nil {
		return fmt.Errorf("connect mcp servers: no configuration store")
	}

	lifeCtx := context.WithoutCancel(ctx)
	for _, s := range servers {
		spec, name, ok := acpMCPServer(s, sessionID)
		if !ok {
			// Only stdio is advertised (mcpCapabilities is empty), so a client
			// sending another transport is outside what Crush claimed to
			// support. Ignoring is the tolerant reading; failing the session
			// over a transport we never offered would be worse.
			a.log.Warn("Ignoring unsupported ACP MCP server", "sessionId", sessionID)
			continue
		}
		if err := mcp.InitializeSingleWithConfig(lifeCtx, name, spec, store); err != nil {
			// A server the client asked for and we could not start is worth
			// surfacing rather than silently dropping: the session would
			// otherwise look healthy while missing tools. Anything already
			// connected for this session is torn down so the failure does not
			// leave a half-connected session behind.
			mcp.TeardownSession(sessionID)
			return fmt.Errorf("connect mcp server %q: %w", s.Stdio.Name, err)
		}
		a.log.Info("Connected ACP MCP server", "sessionId", sessionID, "server", s.Stdio.Name, "registryName", name)
	}
	return nil
}

// acpMCPServer converts one ACP server description into a Crush MCP config and
// the unique registry name to file it under. ok is false when the server uses a
// transport Crush does not serve over ACP.
func acpMCPServer(s acp.McpServer, sessionID string) (spec config.MCPConfig, name string, ok bool) {
	if s.Stdio == nil {
		return config.MCPConfig{}, "", false
	}
	spec = config.MCPConfig{
		Type:    config.MCPStdio,
		Command: s.Stdio.Command,
		Args:    s.Stdio.Args,
	}
	if len(s.Stdio.Env) > 0 {
		spec.Env = make(map[string]string, len(s.Stdio.Env))
		for _, e := range s.Stdio.Env {
			spec.Env[e.Name] = e.Value
		}
	}
	return spec, mcp.ACPServerName(sessionID, s.Stdio.Name), true
}
