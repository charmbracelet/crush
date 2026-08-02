package tools

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net/url"
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

// A2UISurfacePlaceholderPrefix starts every model-facing placeholder that
// stands in for a diverted A2UI surface. The chat renderer uses it to strip
// placeholder lines from the tool content it shows the user.
const A2UISurfacePlaceholderPrefix = "[A2UI surface rendered in the chat UI from "

// splitMCPResourceContents partitions resource contents into model-facing
// text parts and, when divert is set, UI-only A2UI surface payloads carried
// in response metadata. divert is false when no chat UI will render the
// metadata (channel-originated turns, disable_a2ui deployments): the raw
// payload then stays in the model-facing content so the model can still
// relay or summarize the data.
func splitMCPResourceContents(contents []*mcp.ResourceContents, divert bool) ([]string, ReadMCPResourceResponseMetadata) {
	var textParts []string
	var metadata ReadMCPResourceResponseMetadata
	for _, content := range contents {
		if content == nil {
			continue
		}
		text := content.Text
		if text == "" && len(content.Blob) > 0 {
			// Blob is a legal delivery for any MIME type, including
			// application/a2ui+json — normalize it to text here so a
			// blob-delivered surface cannot slip past the diversion below.
			text = string(content.Blob)
		}
		if text == "" {
			slog.Debug("MCP resource content missing text/blob", "uri", content.URI)
			continue
		}
		// A2UI surfaces go to the UI via metadata, not the model-facing
		// content: the chat renderer draws the surface itself, and the
		// model only ever echoed the raw JSON back, double-rendering the
		// surface.
		if divert && content.MIMEType == a2uiMIMEType {
			metadata.A2UISurfaces = append(metadata.A2UISurfaces, "<a2ui-json>"+text+"</a2ui-json>")
			// The placeholder is a single-line protocol the chat renderer
			// strips line-wise — a server-controlled URI must not be able
			// to break out of it with embedded newlines. The injected
			// width hint is stripped too: servers echo the request URI
			// verbatim, and a model that re-reads the echoed ?w=N URI
			// after a terminal resize would freeze the surface at the old
			// width (a2uiWidthHint respects an existing w=).
			uri := strings.NewReplacer("\n", " ", "\r", " ").Replace(content.URI)
			uri = stripA2UIWidthParam(uri)
			textParts = append(textParts, A2UISurfacePlaceholderPrefix+uri+" — the user can already see it; do not repeat or echo its JSON payload]")
			continue
		}
		textParts = append(textParts, text)
	}
	return textParts, metadata
}

//go:embed read_mcp_resource.md
var readMCPResourceDescription string

// a2uiWidthHint appends w=N to an A2UI surface URI so the server sizes its
// pre-rendered geometry to the host's content width. It is a no-op for
// non-A2UI URIs and for URIs that already carry a width hint (an explicit
// w= is respected wherever it came from). The /a2ui path suffix is the
// convention the in-house A2UI resource templates share (cairn,
// switchboard); the template registry's MIME types would be the
// protocol-level signal, but templates can legitimately fail to list, so
// the convention stays the trigger.
//
// The URI is parsed only to decide — the hint is appended to the original
// string, never re-serialized, because MCP servers match URIs textually
// against their templates.
func a2uiWidthHint(uri string, width int) string {
	u, err := url.Parse(uri)
	if err != nil || !strings.HasSuffix(u.Path, "/a2ui") {
		return uri
	}
	if u.Query().Has("w") {
		return uri
	}
	sep := "?"
	if u.RawQuery != "" {
		sep = "&"
	}
	return fmt.Sprintf("%s%sw=%d", uri, sep, width)
}

// stripA2UIWidthParam removes the w= query parameter from a URI for display
// in the model-facing placeholder. This URI is informational (never
// fetched), so re-encoding any remaining query parameters is harmless.
func stripA2UIWidthParam(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || !u.Query().Has("w") {
		return uri
	}
	q := u.Query()
	q.Del("w")
	base, _, _ := strings.Cut(uri, "?")
	if enc := q.Encode(); enc != "" {
		return base + "?" + enc
	}
	return base
}

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

			// A2UI surface templates (…/a2ui) pre-render their bar
			// geometry server-side, so pass the UI's content width as
			// the ?w= hint — the surface then fills the chat card
			// instead of a fixed default budget. Only when the URI
			// carries no width of its own and the turn has a UI width
			// (0 = headless/remote turn).
			if w := GetContentWidthFromContext(ctx); w > 0 {
				params.URI = a2uiWidthHint(params.URI, w)
			}

			contents, err := mcp.ReadResource(ctx, cfg, params.MCPName, params.URI)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if len(contents) == 0 {
				return fantasy.NewTextResponse(""), nil
			}

			// Divert A2UI payloads to UI-only metadata only when a chat UI
			// will actually render them: on channel-originated turns the
			// reply travels back over the channel and the model needs the
			// payload to relay it, disable_a2ui deployments opted out of
			// surfaces entirely, and a zero content width means no
			// interactive UI tagged this turn (headless crush run, remote
			// clients that predate the wire field).
			divert := GetChannelFromContext(ctx) == "" &&
				!cfg.Config().Options.DisableA2UI &&
				GetContentWidthFromContext(ctx) > 0
			textParts, metadata := splitMCPResourceContents(contents, divert)

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
