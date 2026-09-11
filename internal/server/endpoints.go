package server

import (
	"github.com/charmbracelet/crush/internal/apigen"
	"github.com/charmbracelet/crush/internal/proto"
)

// endpoints is the single source of truth for the v1 API: every entry
// drives both mux registration and the documentation served under
// /v1/docs/. Add or change an endpoint here, never by hand elsewhere.
func (c *controllerV1) endpoints() []apigen.Endpoint {
	return []apigen.Endpoint{
		apigen.Get("/v1/health").
			Summary("Health check").
			Tags("system").
			Handle(c.handleGetHealth),

		apigen.Get("/v1/version").
			Summary("Get server version").
			Tags("system").
			Responds(proto.VersionInfo{}).
			Handle(c.handleGetVersion),

		apigen.Post("/v1/control").
			Summary("Send server control command").
			Description("Accepts a control command (e.g. shutdown, shutdown_if_idle). "+
				"Shutdown is conditional: only the backend can rule on idleness without "+
				"racing a session that arrives between a client's own check and its request.").
			Tags("system").
			Accepts(proto.ServerControl{}).
			Fails(400, 409).
			Handle(c.handlePostControl),

		apigen.Get("/v1/config").
			Summary("Get server config").
			Tags("system").
			Handle(c.handleGetConfig),

		apigen.Delete("/v1/clients/{client_id}").
			Summary("Retire a client").
			Description("Releases every claim the client holds.").
			Tags("system").
			PathParam("client_id", "Client ID (UUID)").
			Fails(400).
			Handle(c.handleDeleteClient),

		apigen.Get("/v1/workspaces").
			Summary("List workspaces").
			Tags("workspaces").
			Responds([]proto.Workspace{}).
			Handle(c.handleGetWorkspaces),

		apigen.Post("/v1/workspaces").
			Summary("Create workspace").
			Tags("workspaces").
			Accepts(proto.Workspace{}).
			Responds(proto.Workspace{}).
			Fails(400, 500).
			Handle(c.handlePostWorkspaces),

		apigen.Get("/v1/workspaces/{id}").
			Summary("Get workspace").
			Tags("workspaces").
			PathParam("id", "Workspace ID").
			Responds(proto.Workspace{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspace),

		apigen.Delete("/v1/workspaces/{id}").
			Summary("Delete workspace").
			Tags("workspaces").
			PathParam("id", "Workspace ID").
			Fails(404).
			Handle(c.handleDeleteWorkspaces),

		apigen.Post("/v1/workspaces/{id}/current-session").
			Summary("Set current session for a client").
			Description("An empty session_id clears the entry (e.g. the client "+
				"is on the landing screen).").
			Tags("workspaces").
			PathParam("id", "Workspace ID").
			RequiredQueryParam("client_id", "Client ID (UUID)").
			Accepts(proto.CurrentSession{}).
			Fails(400, 404).
			Handle(c.handlePostWorkspaceCurrentSession),

		apigen.Get("/v1/workspaces/{id}/config").
			Summary("Get workspace config").
			Description("Returns the resolved workspace configuration (config.Config).").
			Tags("workspaces").
			PathParam("id", "Workspace ID").
			Fails(404, 500).
			Handle(c.handleGetWorkspaceConfig),

		apigen.Get("/v1/workspaces/{id}/providers").
			Summary("Get workspace providers").
			Description("Lists the providers available to the workspace.").
			Tags("workspaces").
			PathParam("id", "Workspace ID").
			Fails(404, 500).
			Handle(c.handleGetWorkspaceProviders),

		apigen.Get("/v1/workspaces/{id}/events").
			Summary("Stream workspace events (SSE)").
			Description("Streams workspace events as Server-Sent Events. The client must "+
				"identify itself with the client_id query parameter; the stream attaches "+
				"the client to the workspace until it disconnects.").
			Tags("workspaces").
			PathParam("id", "Workspace ID").
			RequiredQueryParam("client_id", "Client ID (UUID)").
			SSE().
			Fails(400, 404, 500).
			Handle(c.handleGetWorkspaceEvents),

		apigen.Get("/v1/workspaces/{id}/sessions").
			Summary("List sessions").
			Tags("sessions").
			PathParam("id", "Workspace ID").
			Responds([]proto.Session{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceSessions),

		apigen.Post("/v1/workspaces/{id}/sessions").
			Summary("Create session").
			Tags("sessions").
			PathParam("id", "Workspace ID").
			Accepts(proto.Session{}).
			Responds(proto.Session{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceSessions),

		apigen.Get("/v1/workspaces/{id}/sessions/{sid}").
			Summary("Get session").
			Tags("sessions").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Responds(proto.Session{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceSession),

		apigen.Put("/v1/workspaces/{id}/sessions/{sid}").
			Summary("Update session").
			Tags("sessions").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Accepts(proto.Session{}).
			Responds(proto.Session{}).
			Fails(400, 404, 500).
			Handle(c.handlePutWorkspaceSession),

		apigen.Delete("/v1/workspaces/{id}/sessions/{sid}").
			Summary("Delete session").
			Tags("sessions").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Fails(404, 500).
			Handle(c.handleDeleteWorkspaceSession),

		apigen.Get("/v1/workspaces/{id}/sessions/{sid}/history").
			Summary("Get session history").
			Tags("sessions").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Fails(404, 500).
			Handle(c.handleGetWorkspaceSessionHistory),

		apigen.Get("/v1/workspaces/{id}/sessions/{sid}/messages").
			Summary("Get session messages").
			Tags("sessions").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Responds([]proto.Message{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceSessionMessages),

		apigen.Get("/v1/workspaces/{id}/sessions/{sid}/messages/user").
			Summary("Get user messages for session").
			Tags("sessions").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Responds([]proto.Message{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceSessionUserMessages),

		apigen.Get("/v1/workspaces/{id}/messages/user").
			Summary("Get all user messages for workspace").
			Tags("workspaces").
			PathParam("id", "Workspace ID").
			Responds([]proto.Message{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceAllUserMessages),

		apigen.Get("/v1/workspaces/{id}/sessions/{sid}/filetracker/files").
			Summary("List tracked files for session").
			Tags("filetracker").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Responds([]string{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceSessionFileTrackerFiles),

		apigen.Post("/v1/workspaces/{id}/filetracker/read").
			Summary("Record file read").
			Tags("filetracker").
			PathParam("id", "Workspace ID").
			Accepts(proto.FileTrackerReadRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceFileTrackerRead),

		apigen.Get("/v1/workspaces/{id}/filetracker/lastread").
			Summary("Get last read time for file").
			Tags("filetracker").
			PathParam("id", "Workspace ID").
			QueryParam("session_id", "Session ID").
			RequiredQueryParam("path", "File path").
			Fails(404, 500).
			Handle(c.handleGetWorkspaceFileTrackerLastRead),

		apigen.Get("/v1/workspaces/{id}/lsps").
			Summary("List LSP clients").
			Tags("lsp").
			PathParam("id", "Workspace ID").
			Responds(map[string]proto.LSPClientInfo{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceLSPs),

		apigen.Get("/v1/workspaces/{id}/lsps/{lsp}/diagnostics").
			Summary("Get LSP diagnostics").
			Tags("lsp").
			PathParam("id", "Workspace ID").
			PathParam("lsp", "LSP client name").
			Fails(404, 500).
			Handle(c.handleGetWorkspaceLSPDiagnostics),

		apigen.Post("/v1/workspaces/{id}/lsps/start").
			Summary("Start LSP server").
			Tags("lsp").
			PathParam("id", "Workspace ID").
			Accepts(proto.LSPStartRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceLSPStart),

		apigen.Post("/v1/workspaces/{id}/lsps/stop").
			Summary("Stop all LSP servers").
			Tags("lsp").
			PathParam("id", "Workspace ID").
			Fails(404, 500).
			Handle(c.handlePostWorkspaceLSPStopAll),

		apigen.Get("/v1/workspaces/{id}/permissions/skip").
			Summary("Get skip permissions status").
			Tags("permissions").
			PathParam("id", "Workspace ID").
			Responds(proto.PermissionSkipRequest{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspacePermissionsSkip),

		apigen.Post("/v1/workspaces/{id}/permissions/skip").
			Summary("Set skip permissions").
			Tags("permissions").
			PathParam("id", "Workspace ID").
			Accepts(proto.PermissionSkipRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspacePermissionsSkip),

		apigen.Post("/v1/workspaces/{id}/permissions/grant").
			Summary("Grant permission").
			Tags("permissions").
			PathParam("id", "Workspace ID").
			Accepts(proto.PermissionGrant{}).
			Responds(proto.PermissionGrantResponse{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspacePermissionsGrant),

		apigen.Post("/v1/workspaces/{id}/questions/answer").
			Summary("Answer question batch").
			Tags("questions").
			PathParam("id", "Workspace ID").
			Accepts(proto.QuestionAnswer{}).
			Responds(proto.QuestionAnswerResponse{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceQuestionsAnswer),

		apigen.Post("/v1/workspaces/{id}/questions/cancel").
			Summary("Cancel question batch").
			Tags("questions").
			PathParam("id", "Workspace ID").
			Responds(proto.QuestionAnswerResponse{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceQuestionsCancel),

		apigen.Get("/v1/workspaces/{id}/agent").
			Summary("Get agent info").
			Tags("agent").
			PathParam("id", "Workspace ID").
			Responds(proto.AgentInfo{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceAgent),

		apigen.Post("/v1/workspaces/{id}/agent").
			Summary("Send message to agent").
			Description("Validates and accepts the prompt, then dispatches the run "+
				"detached from the requesting HTTP connection: the run survives client "+
				"disconnects and is only ended by the explicit cancel endpoint.").
			Tags("agent").
			PathParam("id", "Workspace ID").
			Accepts(proto.AgentMessage{}).
			Status(202).
			Fails(400, 404, 409, 500).
			Handle(c.handlePostWorkspaceAgent),

		apigen.Post("/v1/workspaces/{id}/agent/init").
			Summary("Initialize agent").
			Description("The request body is optional.").
			Tags("agent").
			PathParam("id", "Workspace ID").
			Accepts(proto.AgentInitRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceAgentInit),

		apigen.Post("/v1/workspaces/{id}/agent/update").
			Summary("Update agent").
			Tags("agent").
			PathParam("id", "Workspace ID").
			Fails(404, 500).
			Handle(c.handlePostWorkspaceAgentUpdate),

		apigen.Get("/v1/workspaces/{id}/agent/sessions/{sid}").
			Summary("Get agent session").
			Tags("agent").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Responds(proto.AgentSession{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceAgentSession),

		apigen.Post("/v1/workspaces/{id}/agent/sessions/{sid}/cancel").
			Summary("Cancel agent session").
			Tags("agent").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Fails(404, 500).
			Handle(c.handlePostWorkspaceAgentSessionCancel),

		apigen.Get("/v1/workspaces/{id}/agent/sessions/{sid}/prompts/queued").
			Summary("Get queued prompt status").
			Description("Returns the number of queued prompts for the session.").
			Tags("agent").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Responds(0).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceAgentSessionPromptQueued),

		apigen.Get("/v1/workspaces/{id}/agent/sessions/{sid}/prompts/list").
			Summary("List queued prompts").
			Tags("agent").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Responds([]string{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceAgentSessionPromptList),

		apigen.Post("/v1/workspaces/{id}/agent/sessions/{sid}/prompts/clear").
			Summary("Clear prompt queue").
			Tags("agent").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Fails(404, 500).
			Handle(c.handlePostWorkspaceAgentSessionPromptClear),

		apigen.Post("/v1/workspaces/{id}/agent/sessions/{sid}/summarize").
			Summary("Summarize session").
			Tags("agent").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Fails(404, 500).
			Handle(c.handlePostWorkspaceAgentSessionSummarize),

		apigen.Post("/v1/workspaces/{id}/agent/sessions/{sid}/shell").
			Summary("Run shell command").
			Description("Runs a shell command in the workspace on behalf of the session.").
			Tags("agent").
			PathParam("id", "Workspace ID").
			PathParam("sid", "Session ID").
			Accepts(proto.ShellCommandRequest{}).
			Responds(proto.ShellCommandResponse{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceAgentSessionShell),

		apigen.Get("/v1/workspaces/{id}/agent/default-small-model").
			Summary("Get default small model").
			Description("Returns the default small model (config.SelectedModel) for the provider.").
			Tags("agent").
			PathParam("id", "Workspace ID").
			QueryParam("provider_id", "Provider ID").
			Fails(404, 500).
			Handle(c.handleGetWorkspaceAgentDefaultSmallModel),

		apigen.Post("/v1/workspaces/{id}/config/set").
			Summary("Set a config field").
			Tags("config").
			PathParam("id", "Workspace ID").
			Accepts(proto.ConfigSetRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceConfigSet),

		apigen.Post("/v1/workspaces/{id}/config/remove").
			Summary("Remove a config field").
			Tags("config").
			PathParam("id", "Workspace ID").
			Accepts(proto.ConfigRemoveRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceConfigRemove),

		apigen.Post("/v1/workspaces/{id}/config/model").
			Summary("Set the preferred model").
			Tags("config").
			PathParam("id", "Workspace ID").
			Accepts(proto.ConfigModelRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceConfigModel),

		apigen.Post("/v1/workspaces/{id}/config/compact").
			Summary("Set compact mode").
			Tags("config").
			PathParam("id", "Workspace ID").
			Accepts(proto.ConfigCompactRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceConfigCompact),

		apigen.Post("/v1/workspaces/{id}/config/provider-key").
			Summary("Set provider API key").
			Tags("config").
			PathParam("id", "Workspace ID").
			Accepts(proto.ConfigProviderKeyRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceConfigProviderKey),

		apigen.Post("/v1/workspaces/{id}/config/import-copilot").
			Summary("Import Copilot credentials").
			Tags("config").
			PathParam("id", "Workspace ID").
			Responds(proto.ImportCopilotResponse{}).
			Fails(404, 500).
			Handle(c.handlePostWorkspaceConfigImportCopilot),

		apigen.Post("/v1/workspaces/{id}/config/refresh-oauth").
			Summary("Refresh OAuth token").
			Tags("config").
			PathParam("id", "Workspace ID").
			Accepts(proto.ConfigRefreshOAuthRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceConfigRefreshOAuth),

		apigen.Get("/v1/workspaces/{id}/project/needs-init").
			Summary("Check if project needs initialization").
			Tags("project").
			PathParam("id", "Workspace ID").
			Responds(proto.ProjectNeedsInitResponse{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceProjectNeedsInit),

		apigen.Post("/v1/workspaces/{id}/project/init").
			Summary("Mark project as initialized").
			Tags("project").
			PathParam("id", "Workspace ID").
			Fails(404, 500).
			Handle(c.handlePostWorkspaceProjectInit),

		apigen.Get("/v1/workspaces/{id}/project/init-prompt").
			Summary("Get project initialization prompt").
			Tags("project").
			PathParam("id", "Workspace ID").
			Responds(proto.ProjectInitPromptResponse{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceProjectInitPrompt),

		apigen.Get("/v1/workspaces/{id}/skills").
			Summary("List visible skills").
			Tags("skills").
			PathParam("id", "Workspace ID").
			Responds([]proto.SkillInfo{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceSkills),

		apigen.Post("/v1/workspaces/{id}/skills/read").
			Summary("Read skill content").
			Tags("skills").
			PathParam("id", "Workspace ID").
			Accepts(proto.ReadSkillRequest{}).
			Responds(proto.ReadSkillResponse{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceSkillRead),

		apigen.Post("/v1/workspaces/{id}/mcp/refresh-tools").
			Summary("Refresh MCP tools").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Accepts(proto.MCPNameRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceMCPRefreshTools),

		apigen.Post("/v1/workspaces/{id}/mcp/read-resource").
			Summary("Read MCP resource").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Accepts(proto.MCPReadResourceRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceMCPReadResource),

		apigen.Get("/v1/workspaces/{id}/mcp/prompts").
			Summary("Get MCP prompts").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Responds([]proto.MCPPrompt{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceMCPPrompts),

		apigen.Post("/v1/workspaces/{id}/mcp/get-prompt").
			Summary("Get MCP prompt").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Accepts(proto.MCPGetPromptRequest{}).
			Responds(proto.MCPGetPromptResponse{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceMCPGetPrompt),

		apigen.Get("/v1/workspaces/{id}/mcp/states").
			Summary("Get MCP client states").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Responds(map[string]proto.MCPClientInfo{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceMCPStates),

		apigen.Get("/v1/workspaces/{id}/mcp/pending-auth").
			Summary("Get MCP servers pending OAuth").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Responds([]proto.MCPPendingAuthServer{}).
			Fails(404, 500).
			Handle(c.handleGetWorkspaceMCPPendingAuth),

		apigen.Get("/v1/workspaces/{id}/mcp/auth-url").
			Summary("Get MCP OAuth authorization URL").
			Description("Returns the current OAuth authorization URL for a named MCP "+
				"server, if a flow is in progress.").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			RequiredQueryParam("name", "MCP server name").
			Responds(proto.MCPAuthResponse{}).
			Fails(400).
			Handle(c.handleGetWorkspaceMCPAuthURL),

		apigen.Post("/v1/workspaces/{id}/mcp/auth").
			Summary("Authenticate an MCP server").
			Description("Runs the OAuth flow for a named MCP server. The local browser "+
				"is suppressed on the server; the client polls pending-auth and auth-url "+
				"to surface the authorization URL. The call blocks until the flow "+
				"completes or the request context is cancelled.").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Accepts(proto.MCPNameRequest{}).
			Responds(proto.MCPAuthResponse{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceMCPAuth),

		apigen.Post("/v1/workspaces/{id}/mcp/refresh-prompts").
			Summary("Refresh MCP prompts").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Accepts(proto.MCPNameRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceMCPRefreshPrompts),

		apigen.Post("/v1/workspaces/{id}/mcp/refresh-resources").
			Summary("Refresh MCP resources").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Accepts(proto.MCPNameRequest{}).
			Fails(400, 404, 500).
			Handle(c.handlePostWorkspaceMCPRefreshResources),

		apigen.Post("/v1/workspaces/{id}/mcp/docker/enable").
			Summary("Enable Docker MCP").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Fails(404, 500).
			Handle(c.handlePostWorkspaceMCPEnableDocker),

		apigen.Post("/v1/workspaces/{id}/mcp/docker/disable").
			Summary("Disable Docker MCP").
			Tags("mcp").
			PathParam("id", "Workspace ID").
			Fails(404, 500).
			Handle(c.handlePostWorkspaceMCPDisableDocker),
	}
}
