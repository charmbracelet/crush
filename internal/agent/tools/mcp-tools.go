package tools

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

// LargeMCPContentThreshold is 3x the max of fetch, 5x the max of bash because it should be assumed that a large
// MCP style request was made on purpose.
const LargeMCPContentThreshold = 150000 // 150KB

// mcpMaxOutputLength is the max output length for MCP tools, can be overridden by setting CRUSH_MCP_MAX_OUTPUT.
var mcpMaxOutputLength int

func init() {
	mcpMaxOutputLength = LargeMCPContentThreshold
	if v := os.Getenv("CRUSH_MCP_MAX_OUTPUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			mcpMaxOutputLength = n
		}
	}
}

// GetMCPTools gets all the currently available MCP tools.
func GetMCPTools(permissions permission.Service, cfg *config.Config, wd string) []*Tool {
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
	cfg             *config.Config
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
	permissionDescription := fmt.Sprintf("execute %s with the following parameters:", m.Info().Name)
	p, err := m.permissions.Request(ctx,
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
		return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
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
		content := result.Content
		if len(content) > mcpMaxOutputLength {
			content = truncateMCPOutput(content)
		}
		return fantasy.NewTextResponse(content), nil
	}
}

func truncateMCPOutput(content string) string {
	if len(content) <= mcpMaxOutputLength {
		return content
	}

	// truncate in bytes, add truncation language in characters
	truncated := content[:mcpMaxOutputLength]

	truncatedChars := len([]rune(content)) - len([]rune(truncated))
	totalChars := len([]rune(content))
	shownChars := len([]rune(truncated))

	return fmt.Sprintf("%s\n\n... [truncated %s, showing first %s of %s total]",
		truncated,
		formatChars(truncatedChars),
		formatChars(shownChars),
		formatChars(totalChars))
}

func formatChars(chars int) string {
	const (
		K = 1000
		M = 1000 * K
	)
	switch {
	case chars >= M:
		return fmt.Sprintf("%.1fM chars", float64(chars)/float64(M))
	case chars >= K:
		return fmt.Sprintf("%.1fK chars", float64(chars)/float64(K))
	default:
		return fmt.Sprintf("%d chars", chars)
	}
}
