package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"slices"
	"strings"
	"time"

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
	IsError   bool
}

var allTools = csync.NewMap[string, []*Tool]()
var allToolFilters = csync.NewMap[string, ToolFilterInfo]()

// ToolFilterInfo describes how configured MCP tool filters affected the
// server's advertised tools.
type ToolFilterInfo struct {
	Advertised        []string
	Usable            []string
	UnmatchedDisabled []string
	UnmatchedEnabled  []string
}

const defaultToolTimeout = 60 * time.Second

// Tools returns all available MCP tools.
func Tools() iter.Seq2[string, []*Tool] {
	return allTools.Seq2()
}

// GetToolFilterInfo returns the latest runtime tool-filter result for an MCP.
func GetToolFilterInfo(name string) (ToolFilterInfo, bool) {
	info, ok := allToolFilters.Get(name)
	if !ok {
		return ToolFilterInfo{}, false
	}
	info.Advertised = slices.Clone(info.Advertised)
	info.Usable = slices.Clone(info.Usable)
	info.UnmatchedDisabled = slices.Clone(info.UnmatchedDisabled)
	info.UnmatchedEnabled = slices.Clone(info.UnmatchedEnabled)
	return info, true
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

	toolTimeout := mcpToolTimeout(cfg.Config().MCP[name])
	callCtx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()

	result, err := c.CallTool(callCtx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		err = maybeToolTimeoutErr(callCtx, err, toolTimeout, name, toolName)
		if isToolTimeout(callCtx, err) {
			markToolTimeout(name, c, err)
		}
		return ToolResult{}, err
	}

	return convertCallToolResult(result), nil
}

func convertCallToolResult(result *mcp.CallToolResult) ToolResult {
	if len(result.Content) == 0 {
		return ToolResult{Type: "text", IsError: result.IsError}
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

	// We need to make sure the data is base64
	// when using something like docker + playwright the data was not returned correctly.
	if imageData != nil {
		return ToolResult{
			Type:      "image",
			Content:   textContent,
			Data:      ensureRawBytes(imageData),
			MediaType: imageMimeType,
			IsError:   result.IsError,
		}
	}

	if audioData != nil {
		return ToolResult{
			Type:      "media",
			Content:   textContent,
			Data:      ensureRawBytes(audioData),
			MediaType: audioMimeType,
			IsError:   result.IsError,
		}
	}

	return ToolResult{
		Type:    "text",
		Content: textContent,
		IsError: result.IsError,
	}
}

func mcpToolTimeout(m config.MCPConfig) time.Duration {
	if m.ToolTimeout <= 0 {
		return defaultToolTimeout
	}
	return time.Duration(m.ToolTimeout) * time.Second
}

func maybeToolTimeoutErr(ctx context.Context, err error, timeout time.Duration, name, toolName string) error {
	if isToolTimeout(ctx, err) {
		return fmt.Errorf("mcp tool call %s/%s timed out after %s", name, toolName, timeout)
	}
	return err
}

func isToolTimeout(ctx context.Context, err error) bool {
	return errors.Is(ctx.Err(), context.DeadlineExceeded) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func markToolTimeout(name string, session *ClientSession, err error) {
	state, _ := states.Get(name)
	if closeErr := session.Close(); closeErr != nil &&
		!errors.Is(closeErr, context.Canceled) &&
		!errors.Is(closeErr, io.EOF) &&
		closeErr.Error() != "signal: killed" {
		slog.Warn("Failed to close timed out MCP session", "name", name, "error", closeErr)
	}
	updateState(name, StateError, err, nil, state.Counts)
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
		updateState(name, StateError, err, nil, Counts{})
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
	mcpCfg, ok := cfg.Config().MCP[name]
	info := analyzeToolFilters(mcpCfg, tools)
	allToolFilters.Set(name, info)
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

func analyzeToolFilters(mcpCfg config.MCPConfig, tools []*Tool) ToolFilterInfo {
	info := ToolFilterInfo{
		Advertised: make([]string, 0, len(tools)),
	}
	for _, tool := range tools {
		info.Advertised = append(info.Advertised, tool.Name)
	}
	slices.Sort(info.Advertised)

	for _, name := range mcpCfg.DisabledTools {
		if !slices.Contains(info.Advertised, name) {
			info.UnmatchedDisabled = append(info.UnmatchedDisabled, name)
		}
	}
	for _, name := range mcpCfg.EnabledTools {
		if !slices.Contains(info.Advertised, name) {
			info.UnmatchedEnabled = append(info.UnmatchedEnabled, name)
		}
	}
	slices.Sort(info.UnmatchedDisabled)
	slices.Sort(info.UnmatchedEnabled)

	filtered := filterTools(mcpCfg, tools)
	info.Usable = make([]string, 0, len(filtered))
	for _, tool := range filtered {
		info.Usable = append(info.Usable, tool.Name)
	}
	slices.Sort(info.Usable)
	return info
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
