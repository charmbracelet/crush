package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

// whitelistDockerTools contains Docker MCP tools that don't require permission.
var whitelistDockerTools = []string{
	"mcp_docker_mcp-find",
	"mcp_docker_mcp-add",
	"mcp_docker_mcp-remove",
	"mcp_docker_mcp-config-set",
	"mcp_docker_code-mode",
}

// GetMCPTools gets all the currently available MCP tools.
func GetMCPTools(permissions permission.Service, cfg *config.ConfigStore, wd string) []*Tool {
	var result []*Tool
	for mcpName, tools := range mcp.Tools() {
		for _, tool := range tools {
			result = append(result, &Tool{
				mcpName:     mcpName,
				tool:        tool,
				permissions: permissions,
				workingDir:  wd,
				cfg:         cfg,
			})
		}
	}
	return result
}

// Tool is a tool from a MCP.
type Tool struct {
	mcpName         string
	tool            *mcp.Tool
	cfg             *config.ConfigStore
	permissions     permission.Service
	workingDir      string
	providerOptions fantasy.ProviderOptions
}

func (m *Tool) SetProviderOptions(opts fantasy.ProviderOptions) {
	m.providerOptions = opts
}

func (m *Tool) ProviderOptions() fantasy.ProviderOptions {
	return m.providerOptions
}

func (m *Tool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", m.mcpName, m.tool.Name)
}

func (m *Tool) MCP() string {
	return m.mcpName
}

func (m *Tool) MCPToolName() string {
	return m.tool.Name
}

func (m *Tool) Info() fantasy.ToolInfo {
	parameters := make(map[string]any)
	required := make([]string, 0)

	if input, ok := m.tool.InputSchema.(map[string]any); ok {
		if props, ok := input["properties"].(map[string]any); ok {
			parameters = props
		}
		if req, ok := input["required"].([]any); ok {
			// Convert []any -> []string when elements are strings
			for _, v := range req {
				if s, ok := v.(string); ok {
					required = append(required, s)
				}
			}
		} else if reqStr, ok := input["required"].([]string); ok {
			// Handle case where it's already []string
			required = reqStr
		}
	}

	return fantasy.ToolInfo{
		Name:        m.Name(),
		Description: m.tool.Description,
		Parameters:  parameters,
		Required:    required,
	}
}

func (m *Tool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for creating a new file")
	}
	if state, ok := mcp.GetState(m.mcpName); ok && state.Channel && GetChannelFromContext(ctx) != m.mcpName {
		return fantasy.NewTextErrorResponse("This channel tool is only available for messages from its originating channel."), nil
	}

	// Skip permission for whitelisted Docker MCP tools.
	if !slices.Contains(whitelistDockerTools, params.Name) {
		permissionDescription := fmt.Sprintf("execute %s with the following parameters:", m.Info().Name)
		p, err := m.permissions.Request(
			ctx,
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				ToolCallID:  params.ID,
				Path:        m.workingDir,
				ToolName:    m.Info().Name,
				Action:      "execute",
				Description: permissionDescription,
				Params:      params.Input,
			},
		)
		if err != nil {
			return fantasy.ToolResponse{}, err
		}
		if !p {
			return NewPermissionDeniedResponse(), nil
		}
	}

	result, err := mcp.RunTool(ctx, m.cfg, m.mcpName, m.tool.Name, params.Input)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	// Divert A2UI surface payloads to UI-only metadata only when a chat UI
	// will actually render them: on channel-originated turns the reply
	// travels back over the channel and the model needs the payload to
	// relay it, disable_a2ui deployments opted out of surfaces entirely,
	// and a zero content width means no interactive UI tagged this turn.
	// Mirrors the diversion rule in read_mcp_resource.go.
	divert := GetChannelFromContext(ctx) == "" &&
		!m.cfg.Config().Options.DisableA2UI &&
		GetContentWidthFromContext(ctx) > 0
	content, metadata := splitMCPToolResult(result, m.mcpName, divert)

	switch result.Type {
	case "image", "media":
		if !GetSupportsImagesFromContext(ctx) {
			modelName := GetModelNameFromContext(ctx)
			return fantasy.NewTextErrorResponse(fmt.Sprintf("This model (%s) does not support image data.", modelName)), nil
		}

		var response fantasy.ToolResponse
		if result.Type == "image" {
			response = fantasy.NewImageResponse(result.Data, result.MediaType)
		} else {
			response = fantasy.NewMediaResponse(result.Data, result.MediaType)
		}
		response.Content = content
		if len(metadata.A2UISurfaces) > 0 {
			response = fantasy.WithResponseMetadata(response, metadata)
		}
		return response, nil
	default:
		resp := fantasy.NewTextResponse(content)
		if len(metadata.A2UISurfaces) > 0 {
			resp = fantasy.WithResponseMetadata(resp, metadata)
		}
		return resp, nil
	}
}

// splitMCPToolResult partitions an MCP tool result into model-facing text
// and, when divert is set, UI-only A2UI surface payloads carried in response
// metadata. The surfaces travel exactly like read_mcp_resource's: wrapped in
// <a2ui-json> tags on ReadMCPResourceResponseMetadata, with a single-line
// placeholder left for the model so it cannot echo the JSON back and
// double-render the surface. When divert is false the raw payload is folded
// back into the model-facing content, so the model can still relay or
// summarize the data.
//
// The audience annotation decides each surface's path (see a2uiAudience),
// but only among renderers that actually exist: a ["user"]-only payload
// renders and is withheld from the model; an ["assistant"]-only payload
// reaches the model but is never rendered; an empty audience does both.
//
// The ["user"] case is conditional on something rendering it. The
// annotation means "the user can already see this, don't repeat it" — so
// when divert is false nothing renders the surface, the user sees nothing,
// and the model is their only path to it. Withholding the payload there
// leaves a channel-originated turn with an empty tool result and the agent
// replying with nothing or a hallucinated summary.
func splitMCPToolResult(result mcp.ToolResult, mcpName string, divert bool) (string, ReadMCPResourceResponseMetadata) {
	var metadata ReadMCPResourceResponseMetadata
	content := result.Content
	appendToContent := func(s string) {
		if content != "" {
			content += "\n"
		}
		content += s
	}
	for _, surface := range result.Surfaces {
		if divert && surface.RenderForUser {
			// Render for the user via metadata; the model gets a
			// placeholder so it cannot echo the JSON back and double-render.
			metadata.A2UISurfaces = append(metadata.A2UISurfaces, "<a2ui-json>"+surface.Payload+"</a2ui-json>")
			metadata.MCPSurfaceProvenance = append(metadata.MCPSurfaceProvenance, mcpName)
			uri := strings.NewReplacer("\n", " ", "\r", " ").Replace(surface.URI)
			appendToContent(A2UISurfacePlaceholderPrefix + uri + " from MCP server " + mcpName + " — the user can already see it; do not repeat or echo its JSON payload]")
			continue
		}
		if !divert {
			// Nothing will render this surface — the turn came in over a
			// channel, it is a headless run, or the deployment disabled
			// surfaces. The model is the user's only path to the payload,
			// so fold it back whatever the audience says.
			appendToContent(surface.Payload)
			continue
		}
		// A chat UI is rendering this turn's surfaces, but not this one:
		// it is ["assistant"]-only. It reaches the model alone.
		if !surface.AssistantVisible {
			continue
		}
		appendToContent(surface.Payload)
	}
	return content, metadata
}
