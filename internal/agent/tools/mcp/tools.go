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

// ToolResult represents the result of running an MCP tool.
type ToolResult struct {
	Type      string
	Content   string
	Data      []byte
	MediaType string
}

var allTools = csync.NewMap[string, []*Tool]()

// Tools returns all available MCP tools.
func Tools() iter.Seq2[string, []*Tool] {
	return allTools.Seq2()
}

// RunTool runs an MCP tool with the given input parameters.
func RunTool(ctx context.Context, name, toolName string, input string) (ToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return ToolResult{}, fmt.Errorf("error parsing parameters: %s", err)
	}

	c, err := getOrRenewClient(ctx, name)
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

	var textParts []string
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
		default:
			textParts = append(textParts, fmt.Sprintf("%v", v))
		}
	}

	textContent := strings.Join(textParts, "\n")

	// We need to to make sure the data is base64
	// when using something like docker + playwright the data was not returned correctly
	if imageData != nil {
		return ToolResult{
			Type:      "image",
			Content:   textContent,
			Data:      ensureBase64(imageData),
			MediaType: imageMimeType,
		}, nil
	}

	if audioData != nil {
		return ToolResult{
			Type:      "media",
			Content:   textContent,
			Data:      ensureBase64(audioData),
			MediaType: audioMimeType,
		}, nil
	}

	return ToolResult{
		Type:    "text",
		Content: textContent,
	}, nil
}

// RefreshTools gets the updated list of tools from the MCP and updates the
// global state.
func RefreshTools(ctx context.Context, name string) {
	session, ok := sessions.Get(name)
	if !ok {
		slog.Warn("Refresh tools: no session", "name", name)
		return
	}

	tools, err := getTools(ctx, session)
	if err != nil {
		updateState(name, StateError, err, nil, Counts{})
		return
	}

	toolCount := updateTools(name, tools)

	prev, _ := states.Get(name)
	prev.Counts.Tools = toolCount
	updateState(name, StateConnected, nil, session, prev.Counts)
}

func getTools(ctx context.Context, session *mcp.ClientSession) ([]*Tool, error) {
	// Always call ListTools to get the actual available tools.
	// The InitializeResult Capabilities.Tools field may be an empty object {},
	// which is valid per MCP spec, but we still need to call ListTools to discover tools.
	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func updateTools(name string, tools []*Tool) int {
	tools = filterDisabledTools(name, tools)
	if len(tools) == 0 {
		allTools.Del(name)
		return 0
	}
	allTools.Set(name, tools)
	return len(tools)
}

// filterDisabledTools removes tools that are disabled via config.
func filterDisabledTools(mcpName string, tools []*Tool) []*Tool {
	cfg := config.Get()
	mcpCfg, ok := cfg.MCP[mcpName]
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

// ensureBase64 checks if data is valid base64 and returns it as-is if so,
// otherwise encodes the raw binary data to base64.
func ensureBase64(data []byte) []byte {
	// Check if the data is already valid base64 by attempting to decode it.
	// Valid base64 should only contain ASCII characters (A-Z, a-z, 0-9, +, /, =).
	if isValidBase64(data) {
		return data
	}
	// Data is raw binary, encode it to base64.
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(encoded, data)
	return encoded
}

// isValidBase64 checks if the data appears to be valid base64-encoded content.
func isValidBase64(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	// Base64 strings should only contain ASCII characters.
	for _, b := range data {
		if b > 127 {
			return false
		}
	}
	// Try to decode to verify it's valid base64.
	_, err := base64.StdEncoding.DecodeString(string(data))
	return err == nil
}
