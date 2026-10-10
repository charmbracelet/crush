package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

// whitelistDockerTools contains Docker MCP tools that don't require permission.
var whitelistDockerTools = []string{
	MCPToolName(config.DockerMCPName, "mcp-find"),
	MCPToolName(config.DockerMCPName, "mcp-add"),
	MCPToolName(config.DockerMCPName, "mcp-remove"),
	MCPToolName(config.DockerMCPName, "mcp-config-set"),
	MCPToolName(config.DockerMCPName, "code-mode"),
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

// maxMCPToolNameLen caps a generated tool name. Anthropic accepts up to 128
// characters, but MCP's own SEP-986 settles on 64 and other providers hold to
// that too, so 64 is the length that travels everywhere.
const maxMCPToolNameLen = 64

// mcpNameDisallowed matches every character an Anthropic tool name may not
// contain. The API rejects the whole request over one bad name, so a server
// called "docker.io" would otherwise take down every tool in the session:
//
//	tools.0.custom.name: String should match pattern '^[a-zA-Z0-9_-]{1,128}$'
var mcpNameDisallowed = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// mcpUnderscoreRun matches a run of underscores.
var mcpUnderscoreRun = regexp.MustCompile(`_{2,}`)

// MCPToolName is the wire name for a tool provided by an MCP server.
//
// The doubled underscores are not cosmetic. Anthropic bills a request whose
// tool names match "mcp_" followed by a single underscore-free segment against
// extra usage rather than a Claude subscription, and answers with "You're out
// of extra usage" once that balance is empty. Names using the doubled form
// Claude Code itself uses are billed to the subscription normally. Measured by
// sending the same tool under both spellings: mcp_x is rejected, mcp__x is
// accepted.
//
// It is also the only form that can be read back. With one underscore,
// "mcp_ssh_ssh_connect" could be the server "ssh" or the server "ssh_ssh";
// with two, everything up to the first "__" after the prefix is the server and
// the rest is the tool.
//
// This follows Claude Code rather than a standard: MCP specifies a character
// set and a length for tool names but says nothing about how a host namespaces
// tools from several servers.
func MCPToolName(server, tool string) string {
	name := MCPToolPrefix(server) + sanitizeMCPSegment(tool)
	if len(name) <= maxMCPToolNameLen {
		return name
	}
	// Truncating alone could map two tools onto one name, leaving the model
	// unable to tell them apart. A digest of the full name keeps them
	// distinct. The server half is already bounded, so what gets trimmed here
	// is the tool half and the delimiter survives.
	return shortenMCPSegment(name, maxMCPToolNameLen)
}

// maxMCPServerSegmentLen caps the server half of a generated name. Real server
// names are a word or two, so this never binds in practice; it exists so a
// pathological one cannot consume the whole budget and leave no room for the
// delimiter and the tool.
const maxMCPServerSegmentLen = 32

// MCPToolPrefix is the prefix [MCPToolName] gives every tool from one server.
//
// Underscore runs are collapsed in the server segment, and only there: "__" is
// the delimiter, so a server whose name contains one would make the boundary
// unreadable. A tool name keeps its own underscores, since everything after
// the delimiter belongs to it either way.
func MCPToolPrefix(server string) string {
	clean := mcpUnderscoreRun.ReplaceAllString(sanitizeMCPSegment(server), "_")
	return fmt.Sprintf("mcp__%s__", shortenMCPSegment(clean, maxMCPServerSegmentLen))
}

// shortenMCPSegment trims s to at most n characters, ending it with a digest of
// the original so two long names that share a prefix stay distinct.
func shortenMCPSegment(s string, n int) string {
	if len(s) <= n {
		return s
	}
	sum := sha256.Sum256([]byte(s))
	suffix := "_" + hex.EncodeToString(sum[:])[:6]
	return s[:n-len(suffix)] + suffix
}

// sanitizeMCPSegment replaces the characters Anthropic rejects in a tool name
// with underscores, as Claude Code does.
func sanitizeMCPSegment(s string) string {
	return mcpNameDisallowed.ReplaceAllString(s, "_")
}

func (m *Tool) Name() string {
	return MCPToolName(m.mcpName, m.tool.Name)
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
	// A channel-scoped tool is callable from a local turn (channel ==
	// "", so the TUI user can ask the agent to send via Signal) and from
	// turns originating on its own channel, but not from other channels.
	ch := GetChannelFromContext(ctx)
	if state, ok := mcp.GetState(m.mcpName); ok && state.Channel && ch != "" && ch != m.mcpName {
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
		response.Content = result.Content
		return response, nil
	default:
		return fantasy.NewTextResponse(result.Content), nil
	}
}

// ParseMCPToolName splits an MCP tool name into its server and tool halves.
// It accepts the legacy mcp_server_tool spelling too, so tool calls recorded
// before the rename still resolve.
func ParseMCPToolName(name string) (server, tool string, ok bool) {
	if rest, found := strings.CutPrefix(name, "mcp__"); found {
		server, tool, ok = strings.Cut(rest, "__")
		return server, tool, ok && server != "" && tool != ""
	}
	if rest, found := strings.CutPrefix(name, "mcp_"); found {
		server, tool, ok = strings.Cut(rest, "_")
		return server, tool, ok && server != "" && tool != ""
	}
	return "", "", false
}
