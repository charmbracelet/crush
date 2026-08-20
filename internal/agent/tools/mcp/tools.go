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
	"unicode/utf8"

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
	textContent = capToolResult(textContent, maxToolResultBytes(cfg, name))

	// We need to make sure the data is base64
	// when using something like docker + playwright the data was not returned correctly.
	if imageData != nil {
		return ToolResult{
			Type:      "image",
			Content:   textContent,
			Data:      ensureRawBytes(imageData),
			MediaType: imageMimeType,
		}, nil
	}

	if audioData != nil {
		return ToolResult{
			Type:      "media",
			Content:   textContent,
			Data:      ensureRawBytes(audioData),
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

// defaultMaxToolResultBytes is the default cap applied to MCP tool result
// text before it is added to the model context, matching the size suggested
// in the config schema default. Oversized results from third-party MCP
// servers are the most common way a single tool call blows past a model's
// context window (see #2835 for the built-in tool precedent).
const defaultMaxToolResultBytes = 131072

// maxToolResultBytes returns the configured cap for the MCP server, falling
// back to the default when the server doesn't set one.
func maxToolResultBytes(cfg *config.ConfigStore, name string) int {
	m := cfg.Config().MCP[name]
	if m.MaxToolResultBytes > 0 {
		return m.MaxToolResultBytes
	}
	return defaultMaxToolResultBytes
}

// capToolResult caps text content to maxBytes, keeping the head (~75%) and
// tail (~25%) of the cap and inserting a marker that states how much was
// dropped. Keeping the tail preserves the final error/exit summary, and the
// marker teaches the model to narrow its next call instead of retrying the
// same oversized one. Content within the cap is returned unchanged. The
// split points are adjusted so the result never splits a multi-byte UTF-8
// rune.
func capToolResult(content string, maxBytes int) string {
	if maxBytes <= 0 || len(content) <= maxBytes {
		return content
	}

	headBytes := maxBytes * 3 / 4
	tailBytes := maxBytes - headBytes

	head := content[:trimRune(content, headBytes)]
	tail := content[trimRuneBack(content, len(content)-tailBytes):]

	dropped := len(content) - len(head) - len(tail)
	return fmt.Sprintf("%s\n\n[... %s truncated (returned first %s + last %s). Narrow the command with head/tail/grep, or write to a file and sample it. ...]\n\n%s",
		head, formatBytes(dropped), formatBytes(len(head)), formatBytes(len(tail)), tail)
}

// trimRune returns the largest index <= n that is a UTF-8 rune boundary in s,
// so a byte slice ending there never splits a rune.
func trimRune(s string, n int) int {
	for n > 0 && n < len(s) && !utf8.RuneStart(s[n]) {
		n--
	}
	return n
}

// trimRuneBack returns the smallest index >= n that is a UTF-8 rune boundary
// in s, so a byte slice starting there never splits a rune.
func trimRuneBack(s string, n int) int {
	for n < len(s) && !utf8.RuneStart(s[n]) {
		n++
	}
	return n
}

// formatBytes renders a byte count in a human-readable form, e.g. 41.2 MB.
func formatBytes(bytes int) string {
	const (
		kb = 1024
		mb = kb * 1024
	)
	switch {
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
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
