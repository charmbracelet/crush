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

// ToolMedia represents a single media payload returned by an MCP tool.
type ToolMedia struct {
	Type      string
	Data      []byte
	MediaType string
}

// ToolResult represents the result of running an MCP tool.
type ToolResult struct {
	Type            string
	Content         string
	Data            []byte
	MediaType       string
	AdditionalMedia []ToolMedia
}

var allTools = csync.NewMap[string, []*Tool]()

// Tools returns all available MCP tools.
func Tools() iter.Seq2[string, []*Tool] {
	return allTools.Seq2()
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
		if prev, ok := states.Get(name); ok {
			updateState(name, stateForError(err), err, nil, prev.Counts)
		}
		return ToolResult{}, err
	}

	if len(result.Content) == 0 {
		return ToolResult{Type: "text", Content: ""}, nil
	}

	return resultFromMCPContent(result.Content), nil
}

func resultFromMCPContent(content []mcp.Content) ToolResult {
	var textParts []string
	mediaParts := make([]ToolMedia, 0, len(content))

	for _, v := range content {
		switch item := v.(type) {
		case *mcp.TextContent:
			textParts = append(textParts, item.Text)
		case *mcp.ImageContent:
			mediaParts = append(mediaParts, ToolMedia{
				Type:      "image",
				Data:      ensureBase64(item.Data),
				MediaType: item.MIMEType,
			})
		case *mcp.AudioContent:
			mediaParts = append(mediaParts, ToolMedia{
				Type:      "media",
				Data:      ensureBase64(item.Data),
				MediaType: item.MIMEType,
			})
		default:
			textParts = append(textParts, fmt.Sprintf("%v", v))
		}
	}

	textContent := strings.Join(textParts, "\n")
	if len(mediaParts) == 0 {
		return ToolResult{Type: "text", Content: textContent}
	}

	result := ToolResult{
		Type:      mediaParts[0].Type,
		Content:   textContent,
		Data:      mediaParts[0].Data,
		MediaType: mediaParts[0].MediaType,
	}
	if len(mediaParts) > 1 {
		result.AdditionalMedia = append(result.AdditionalMedia, mediaParts[1:]...)
	}
	return result
}

// RefreshTools gets the updated list of tools from the MCP and updates the
// global state.
func RefreshTools(ctx context.Context, cfg *config.ConfigStore, name string) {
	session, ok := sessions.Get(name)
	if !ok {
		slog.Warn("Refresh tools: no session", "name", name)
		return
	}

	tools, err := getTools(ctx, session)
	if err != nil {
		updateState(name, stateForError(err), err, nil, Counts{})
		return
	}

	toolCount := updateTools(cfg, name, tools)

	prev, _ := states.Get(name)
	prev.Counts.Tools = toolCount
	updateState(name, StateConnected, nil, session, prev.Counts)
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
	tools = filterDisabledTools(cfg, name, tools)
	if len(tools) == 0 {
		allTools.Del(name)
		return 0
	}
	allTools.Set(name, tools)
	return len(tools)
}

// filterDisabledTools removes tools that are disabled via config.
func filterDisabledTools(cfg *config.ConfigStore, mcpName string, tools []*Tool) []*Tool {
	mcpCfg, ok := cfg.Config().MCP[mcpName]
	if !ok || len(mcpCfg.DisabledTools) == 0 {
		return tools
	}

	filtered := make([]*Tool, 0, len(tools))
	for _, tool := range tools {
		if !slices.Contains(mcpCfg.DisabledTools, tool.Name) {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// ensureBase64 normalizes valid base64 input and guarantees padded
// base64.StdEncoding output; otherwise it encodes raw binary data.
func ensureBase64(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	normalized := normalizeBase64Input(data)
	if decoded, ok := decodeBase64(normalized); ok {
		encoded := make([]byte, base64.StdEncoding.EncodedLen(len(decoded)))
		base64.StdEncoding.Encode(encoded, decoded)
		return encoded
	}

	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(encoded, data)
	return encoded
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

	// For RawStdEncoding (no padding), apply stricter heuristics to reduce
	// false positives. We require:
	// 1. Minimum length of 8 characters (more likely to be real base64)
	// 2. Length must be a multiple of 4 when padding is added (base64 alignment)
	// This reduces the chance of misinterpreting random ASCII text as base64.
	if len(s) >= 8 && len(s)%4 == 0 {
		decoded, err = base64.RawStdEncoding.DecodeString(s)
		if err == nil {
			return decoded, true
		}
	}
	return nil, false
}

// isValidBase64 checks if the data appears to be valid base64-encoded content.
func isValidBase64(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	for _, b := range data {
		if b > 127 {
			return false
		}
	}

	s := string(data)
	if _, err := base64.StdEncoding.DecodeString(s); err == nil {
		return true
	}
	_, err := base64.RawStdEncoding.DecodeString(s)
	return err == nil
}
