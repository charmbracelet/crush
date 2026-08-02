package tools

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/permission"
)

type ReadMCPResourceParams struct {
	MCPName string `json:"mcp_name" description:"The MCP server name"`
	URI     string `json:"uri" description:"The resource URI to read"`
}

type ReadMCPResourcePermissionsParams struct {
	MCPName string `json:"mcp_name"`
	URI     string `json:"uri"`
}

const ReadMCPResourceToolName = "read_mcp_resource"

// a2uiMIMEType is the MIME type MCP resource templates use for A2UI surfaces.
// A2UI payloads are delivered to the UI through response metadata (never to
// the model): the model gets a compact placeholder instead of the raw JSON,
// so it cannot echo the payload back and double-render the surface.
const a2uiMIMEType = "application/a2ui+json"

// ReadMCPResourceResponseMetadata carries the UI-only A2UI surface payload
// for a resource whose MIME type is application/a2ui+json.
type ReadMCPResourceResponseMetadata struct {
	A2UISurfaces []string `json:"a2ui_surfaces,omitempty"`
}

//go:embed read_mcp_resource.md
var readMCPResourceDescription string

func NewReadMCPResourceTool(cfg *config.ConfigStore, permissions permission.Service) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		ReadMCPResourceToolName,
		readMCPResourceDescription,
		func(ctx context.Context, params ReadMCPResourceParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			params.MCPName = strings.TrimSpace(params.MCPName)
			params.URI = strings.TrimSpace(params.URI)
			if params.MCPName == "" {
				return fantasy.NewTextErrorResponse("mcp_name parameter is required"), nil
			}
			if params.URI == "" {
				return fantasy.NewTextErrorResponse("uri parameter is required"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for reading MCP resources")
			}

			relPath := filepathext.SmartJoin(cfg.WorkingDir(), cmp.Or(params.URI, "mcp-resource"))
			p, err := permissions.Request(
				ctx,
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        relPath,
					ToolCallID:  call.ID,
					ToolName:    ReadMCPResourceToolName,
					Action:      "read",
					Description: fmt.Sprintf("Read MCP resource from %s", params.MCPName),
					Params:      ReadMCPResourcePermissionsParams(params),
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return NewPermissionDeniedResponse(), nil
			}

			contents, err := mcp.ReadResource(ctx, cfg, params.MCPName, params.URI)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if len(contents) == 0 {
				return fantasy.NewTextResponse(""), nil
			}

			var textParts []string
			var metadata ReadMCPResourceResponseMetadata
			for _, content := range contents {
				if content == nil {
					continue
				}
				if content.Text != "" {
					// A2UI surfaces go to the UI via metadata, not the
					// model-facing content: the chat renderer renders
					// the surface itself, and the raw JSON is
					// unreadable to the model anyway — it only ever
					// echoed it back, double-rendering the surface.
					if content.MIMEType == a2uiMIMEType {
						metadata.A2UISurfaces = append(metadata.A2UISurfaces, "<a2ui-json>"+content.Text+"</a2ui-json>")
						textParts = append(textParts, "[A2UI surface rendered in the chat UI from "+content.URI+" — the user can already see it; do not repeat or echo its JSON payload]")
						continue
					}
					textParts = append(textParts, content.Text)
					continue
				}
				if len(content.Blob) > 0 {
					textParts = append(textParts, string(content.Blob))
					continue
				}
				slog.Debug("MCP resource content missing text/blob", "uri", content.URI)
			}

			if len(textParts) == 0 {
				return fantasy.NewTextResponse(""), nil
			}

			resp := fantasy.NewTextResponse(strings.Join(textParts, "\n"))
			if len(metadata.A2UISurfaces) > 0 {
				resp = fantasy.WithResponseMetadata(resp, metadata)
			}
			return resp, nil
		},
	)
}
