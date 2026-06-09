package server

import (
	"encoding/json"
	"net/http"

	"github.com/charmbracelet/crush/internal/proto"
)

// handlePostWorkspaceConfigSet sets a configuration field.
func (c *controllerV1) handlePostWorkspaceConfigSet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.ConfigSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.SetConfigField(id, req.Scope, req.Key, req.Value); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceConfigRemove removes a configuration field.
func (c *controllerV1) handlePostWorkspaceConfigRemove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.ConfigRemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.RemoveConfigField(id, req.Scope, req.Key); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceConfigModel updates the preferred model.
func (c *controllerV1) handlePostWorkspaceConfigModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.ConfigModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.UpdatePreferredModel(id, req.Scope, req.ModelType, req.Model); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceConfigCompact sets compact mode.
func (c *controllerV1) handlePostWorkspaceConfigCompact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.ConfigCompactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.SetCompactMode(id, req.Scope, req.Enabled); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceConfigProviderKey sets a provider API key.
func (c *controllerV1) handlePostWorkspaceConfigProviderKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.ConfigProviderKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	apiKey, err := req.DecodeAPIKey()
	if err != nil {
		c.server.logError(r, "Failed to decode api key", "error", err, "kind", req.Kind)
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := c.backend.SetProviderAPIKey(id, req.Scope, req.ProviderID, apiKey); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceConfigImportCopilot imports Copilot credentials.
func (c *controllerV1) handlePostWorkspaceConfigImportCopilot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token, ok, err := c.backend.ImportCopilot(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.ImportCopilotResponse{Token: token, Success: ok})
}

// handlePostWorkspaceConfigRefreshOAuth refreshes an OAuth token for a provider.
func (c *controllerV1) handlePostWorkspaceConfigRefreshOAuth(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.ConfigRefreshOAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.RefreshOAuthToken(r.Context(), id, req.Scope, req.ProviderID); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleGetWorkspaceProjectNeedsInit reports whether a project needs initialization.
func (c *controllerV1) handleGetWorkspaceProjectNeedsInit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	needs, err := c.backend.ProjectNeedsInitialization(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.ProjectNeedsInitResponse{NeedsInit: needs})
}

// handlePostWorkspaceProjectInit marks the project as initialized.
func (c *controllerV1) handlePostWorkspaceProjectInit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := c.backend.MarkProjectInitialized(id); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleGetWorkspaceProjectInitPrompt returns the project initialization prompt.
func (c *controllerV1) handleGetWorkspaceProjectInitPrompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	prompt, err := c.backend.InitializePrompt(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.ProjectInitPromptResponse{Prompt: prompt})
}

// handleGetWorkspaceSkills returns the effective visible skills for a workspace.
func (c *controllerV1) handleGetWorkspaceSkills(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	skills, err := c.backend.ListSkills(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, skills)
}

// handlePostWorkspaceSkillRead reads a skill's content by ID.
func (c *controllerV1) handlePostWorkspaceSkillRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.ReadSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	content, result, err := c.backend.ReadSkill(r.Context(), id, req.SkillID)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.ReadSkillResponse{Content: content, Result: result})
}

// handlePostWorkspaceMCPEnableDocker enables the Docker MCP server.
func (c *controllerV1) handlePostWorkspaceMCPEnableDocker(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := c.backend.EnableDockerMCP(r.Context(), id); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceMCPDisableDocker disables the Docker MCP server.
func (c *controllerV1) handlePostWorkspaceMCPDisableDocker(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := c.backend.DisableDockerMCP(id); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceMCPRefreshTools refreshes tools for a named MCP server.
func (c *controllerV1) handlePostWorkspaceMCPRefreshTools(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.MCPNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.RefreshMCPTools(r.Context(), id, req.Name); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceMCPReadResource reads a resource from an MCP server.
func (c *controllerV1) handlePostWorkspaceMCPReadResource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.MCPReadResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	contents, err := c.backend.ReadMCPResource(r.Context(), id, req.Name, req.URI)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, contents)
}

// handlePostWorkspaceMCPGetPrompt retrieves a prompt from an MCP server.
func (c *controllerV1) handlePostWorkspaceMCPGetPrompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.MCPGetPromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	prompt, err := c.backend.GetMCPPrompt(id, req.ClientID, req.PromptID, req.Args)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.MCPGetPromptResponse{Prompt: prompt})
}

// handleGetWorkspaceMCPStates returns the state of all MCP clients.
func (c *controllerV1) handleGetWorkspaceMCPStates(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	states := c.backend.MCPGetStates(id)
	result := make(map[string]proto.MCPClientInfo, len(states))
	for k, v := range states {
		result[k] = proto.MCPClientInfo{
			Name:          v.Name,
			State:         proto.MCPState(v.State),
			Error:         v.Error,
			ToolCount:     v.Counts.Tools,
			PromptCount:   v.Counts.Prompts,
			ResourceCount: v.Counts.Resources,
			ConnectedAt:   v.ConnectedAt,
		}
	}
	jsonEncode(w, result)
}

// handlePostWorkspaceMCPRefreshPrompts refreshes prompts for a named MCP server.
func (c *controllerV1) handlePostWorkspaceMCPRefreshPrompts(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.MCPNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	c.backend.MCPRefreshPrompts(r.Context(), id, req.Name)
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceMCPRefreshResources refreshes resources for a named MCP server.
func (c *controllerV1) handlePostWorkspaceMCPRefreshResources(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.MCPNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	c.backend.MCPRefreshResources(r.Context(), id, req.Name)
	w.WriteHeader(http.StatusOK)
}
