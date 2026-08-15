package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool = mcp.Tool

// A2UIJSONMIMEType is the canonical MIME type identifying an A2UI surface
// payload, per the A2UI-over-MCP contract. A2UILegacyMIMEType is the legacy
// spelling reference clients also accept.
const (
	A2UIJSONMIMEType   = "application/a2ui+json"
	A2UILegacyMIMEType = "application/json+a2ui"
)

// IsA2UIMIMEType reports whether mime identifies an A2UI surface payload,
// accepting both the canonical and legacy spellings.
func IsA2UIMIMEType(mime string) bool {
	return mime == A2UIJSONMIMEType || mime == A2UILegacyMIMEType
}

// A2UISurface is a UI-only A2UI surface payload extracted from a tool
// result's EmbeddedResource content. The chat UI renders it as a live
// surface; the model never sees the raw JSON.
type A2UISurface struct {
	// Payload is the raw A2UI JSON.
	Payload string
	// URI is the embedded resource's URI, for display/diagnostics.
	URI string
	// RenderForUser reports whether the payload should be rendered for the
	// user. False only when the audience is ["assistant"] alone: the spec's
	// verbalization contract renders for every audience except that one.
	RenderForUser bool
	// AssistantVisible reports whether the server annotated the payload
	// for the LLM as well as the user (empty audience or one containing
	// "assistant"). An audience of ["user"] alone hides the raw JSON from
	// the model.
	AssistantVisible bool
}

// ToolResult represents the result of running an MCP tool.
type ToolResult struct {
	Type      string
	Content   string
	Data      []byte
	MediaType string
	// Surfaces carries any A2UI surface payloads found in the result's
	// EmbeddedResource content. They are kept out of Content: the chat UI
	// draws them, and the model only ever echoed the JSON back.
	Surfaces []A2UISurface
}

var allTools = csync.NewMap[string, []*Tool]()

// Tools returns all available MCP tools.
func Tools() iter.Seq2[string, []*Tool] {
	return allTools.Seq2()
}

// a2uiToolNames returns the a2ui_* tools an MCP server exposes, for
// ClientInfo.A2UITools. The names travel with the state so a client/server
// TUI — which never populates this registry — can still tell whether a
// server speaks the surface round-trip.
func a2uiToolNames(name string) []string {
	tools, ok := allTools.Get(name)
	if !ok {
		return nil
	}
	var out []string
	for _, t := range tools {
		if strings.HasPrefix(t.Name, "a2ui_") {
			out = append(out, t.Name)
		}
	}
	return out
}

// RunTool runs an MCP tool with the given input parameters.
func RunTool(ctx context.Context, cfg *config.ConfigStore, name, toolName string, input string) (ToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return ToolResult{}, fmt.Errorf("error parsing parameters: %s", err)
	}

	c, err := getOrRenewClient(ctx, cfg, name)
	if err != nil {
		return ToolResult{}, err
	}
	result, err := c.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return ToolResult{}, err
	}

	if len(result.Content) == 0 {
		return ToolResult{Type: "text", Content: ""}, nil
	}

	return extractToolResult(result), nil
}

// extractToolResult partitions a CallToolResult's content into model-facing
// text, binary media, and UI-only A2UI surfaces — the extraction half of
// RunTool, split out so tests can drive it through a live in-memory server.
func extractToolResult(result *mcp.CallToolResult) ToolResult {
	var textParts []string
	var surfaces []A2UISurface
	var imageData []byte
	var imageMimeType string
	var audioData []byte
	var audioMimeType string

	for _, v := range result.Content {
		switch content := v.(type) {
		case *mcp.TextContent:
			textParts = append(textParts, content.Text)
		case *mcp.ImageContent:
			if imageData == nil {
				imageData = content.Data
				imageMimeType = content.MIMEType
			}
		case *mcp.AudioContent:
			if audioData == nil {
				audioData = content.Data
				audioMimeType = content.MIMEType
			}
		case *mcp.EmbeddedResource:
			// A2UI surfaces arrive as embedded resources — pull them
			// aside for the UI instead of stringifying them into the
			// model-facing text.
			if content.Resource != nil && IsA2UIMIMEType(content.Resource.MIMEType) {
				payload := content.Resource.Text
				if payload == "" && len(content.Resource.Blob) > 0 {
					payload = string(content.Resource.Blob)
				}
				if payload != "" {
					render, assistantVisible := a2uiAudience(content.Annotations)
					surfaces = append(surfaces, A2UISurface{
						Payload:          payload,
						URI:              content.Resource.URI,
						RenderForUser:    render,
						AssistantVisible: assistantVisible,
					})
				}
				continue
			}
			textParts = append(textParts, fmt.Sprintf("%v", v))
		default:
			textParts = append(textParts, fmt.Sprintf("%v", v))
		}
	}

	textContent := strings.Join(textParts, "\n")

	// We need to make sure the data is base64
	// when using something like docker + playwright the data was not returned correctly.
	if imageData != nil {
		return ToolResult{
			Type:      "image",
			Content:   textContent,
			Data:      ensureRawBytes(imageData),
			MediaType: imageMimeType,
			Surfaces:  surfaces,
		}
	}

	if audioData != nil {
		return ToolResult{
			Type:      "media",
			Content:   textContent,
			Data:      ensureRawBytes(audioData),
			MediaType: audioMimeType,
			Surfaces:  surfaces,
		}
	}

	return ToolResult{
		Type:     "text",
		Content:  textContent,
		Surfaces: surfaces,
	}
}

// a2uiAudience resolves an A2UI embedded resource's audience annotations
// into the spec's verbalization contract:
//
//	audience (empty)      → render for the user, visible to the model
//	["user"]              → render for the user, hidden from the model
//	["assistant"]         → visible to the model, NOT rendered
//	["user","assistant"]  → both
//
// It returns (renderForUser, assistantVisible).
func a2uiAudience(annotations *mcp.Annotations) (bool, bool) {
	if annotations == nil || len(annotations.Audience) == 0 {
		return true, true
	}
	forUser := slices.Contains(annotations.Audience, mcp.Role("user"))
	forAssistant := slices.Contains(annotations.Audience, mcp.Role("assistant"))
	return forUser, forAssistant
}

// RefreshTools gets the updated list of tools from the MCP and updates the
// global state.
func RefreshTools(ctx context.Context, cfg *config.ConfigStore, name string) {
	// Runs under the per-name lifecycle lock so a concurrent renewal can't
	// swap the session between our Get and the state update below.
	mu := renewLock(name)
	mu.Lock()
	defer mu.Unlock()

	session, ok := sessions.Get(name)
	if !ok {
		slog.Warn("Refresh tools: no session", "name", name)
		return
	}

	tools, err := getTools(ctx, session)
	if err != nil {
		updateState(name, StateError, err, session, Counts{})
		return
	}

	toolCount := updateTools(cfg, name, tools)

	prev, _ := states.Get(name)
	prev.Counts.Tools = toolCount
	updateState(name, StateConnected, nil, session, prev.Counts)
}

// registerSessionTools lists the tools a live session exposes and writes them
// into the shared registry, returning the number registered after any
// configured allow/deny filtering. It is the single seam through which a
// (re)connected session's tools enter the registry, so both the initial
// connect and a lazy renew repopulate the tool list the agent sends to the LLM
// instead of leaving it empty.
func registerSessionTools(ctx context.Context, cfg *config.ConfigStore, name string, sess *ClientSession) (int, error) {
	tools, err := getTools(ctx, sess)
	if err != nil {
		return 0, err
	}
	return updateTools(cfg, name, tools), nil
}

func getTools(ctx context.Context, session *ClientSession) ([]*Tool, error) {
	// Always call ListTools to get the actual available tools.
	// The InitializeResult Capabilities.Tools field may be an empty object {},
	// which is valid per MCP spec, but we still need to call ListTools to discover tools.
	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func updateTools(cfg *config.ConfigStore, name string, tools []*Tool) int {
	mcpCfg, ok := cfg.Config().MCP[name]
	if ok {
		tools = filterTools(mcpCfg, tools)
	}
	if len(tools) == 0 {
		allTools.Del(name)
		return 0
	}
	allTools.Set(name, tools)
	return len(tools)
}

// filterTools filters tools based on enabled_tools (allow list) and
// disabled_tools (deny list) from the MCP config.
func filterTools(mcpCfg config.MCPConfig, tools []*Tool) []*Tool {
	if len(mcpCfg.EnabledTools) > 0 {
		filtered := make([]*Tool, 0, len(mcpCfg.EnabledTools))
		for _, tool := range tools {
			if slices.Contains(mcpCfg.EnabledTools, tool.Name) {
				filtered = append(filtered, tool)
			}
		}
		tools = filtered
	}

	if len(mcpCfg.DisabledTools) > 0 {
		filtered := make([]*Tool, 0, len(tools))
		for _, tool := range tools {
			if !slices.Contains(mcpCfg.DisabledTools, tool.Name) {
				filtered = append(filtered, tool)
			}
		}
		tools = filtered
	}

	return tools
}

// ensureRawBytes normalizes MCP media data into raw binary bytes.
//
// The MCP Go SDK's json.Unmarshal normally base64-decodes
// ImageContent.Data into raw bytes automatically. However, some MCP
// transports (notably Docker over stdio) can deliver data in
// unexpected formats. This function handles both cases:
//
//   - If data looks like a valid base64 string (ASCII-only, decodable)
//     it is decoded and the raw bytes are returned.
//   - If data is already raw binary (contains bytes > 127) it is
//     returned as-is.
func ensureRawBytes(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	normalized := normalizeBase64Input(data)
	if decoded, ok := decodeBase64(normalized); ok {
		return decoded
	}

	// Already raw binary — return unchanged.
	return data
}

func normalizeBase64Input(data []byte) []byte {
	normalized := strings.Join(strings.Fields(string(data)), "")
	return []byte(normalized)
}

func decodeBase64(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return data, true
	}

	for _, b := range data {
		if b > 127 {
			return nil, false
		}
	}

	s := string(data)
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		return decoded, true
	}
	decoded, err = base64.RawStdEncoding.DecodeString(s)
	if err == nil {
		return decoded, true
	}
	return nil, false
}
