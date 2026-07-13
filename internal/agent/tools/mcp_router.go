package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"

	"charm.land/fantasy"
	mcpruntime "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

const (
	MCPToolSearchToolName = "mcp_tool_search"
	MCPToolCallToolName   = "mcp_tool_call"
	defaultMCPToolMatches = 5
	maxMCPToolMatches     = 10
)

//go:embed mcp_tool_search.md
var mcpToolSearchDescription string

//go:embed mcp_tool_call.md
var mcpToolCallDescription string

type MCPToolSearchParams struct {
	Query  string `json:"query,omitempty" description:"Capability or task to search for, such as forecast, repository file, or browser screenshot."`
	Server string `json:"server,omitempty" description:"Optional exact MCP server name used to restrict the search."`
	Limit  int    `json:"limit,omitempty" description:"Maximum matches to return. Defaults to 5 and cannot exceed 10."`
}

type MCPToolCallParams struct {
	Server    string         `json:"server" description:"Exact MCP server name returned by mcp_tool_search."`
	Tool      string         `json:"tool" description:"Exact server-native tool name returned by mcp_tool_search."`
	Arguments map[string]any `json:"arguments" description:"Arguments matching the input schema returned by mcp_tool_search. Use an empty object when the tool has no arguments."`
}

type mcpToolMatch struct {
	tool  *Tool
	score int
}

// NewMCPToolSearchTool creates a compact catalog over every currently usable
// MCP tool. The catalog is rebuilt per call so servers added or refreshed
// during a session become discoverable without rebuilding the agent.
func NewMCPToolSearchTool(permissions permission.Service, cfg *config.ConfigStore, workingDir string, allowed map[string][]string) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		MCPToolSearchToolName,
		mcpToolSearchDescription,
		func(_ context.Context, params MCPToolSearchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			available := allowedMCPTools(GetMCPTools(permissions, cfg, workingDir), allowed)
			return fantasy.NewTextResponse(formatMCPToolSearch(available, params)), nil
		},
	)
}

// NewMCPToolCallTool creates a single dispatcher for MCP tools discovered
// through mcp_tool_search. The selected native tool still performs its normal
// permission check and uses its original MCP transport.
func NewMCPToolCallTool(permissions permission.Service, cfg *config.ConfigStore, workingDir string, allowed map[string][]string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MCPToolCallToolName,
		mcpToolCallDescription,
		func(ctx context.Context, params MCPToolCallParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			server := strings.TrimSpace(params.Server)
			toolName := strings.TrimSpace(params.Tool)
			if server == "" || toolName == "" {
				return fantasy.NewTextErrorResponse("server and tool are required; use mcp_tool_search to obtain their exact names"), nil
			}

			available := allowedMCPTools(GetMCPTools(permissions, cfg, workingDir), allowed)
			for _, tool := range available {
				if tool.MCP() != server || tool.MCPToolName() != toolName {
					continue
				}
				input, err := json.Marshal(params.Arguments)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to encode MCP arguments: %v", err)), nil
				}
				return tool.Run(ctx, fantasy.ToolCall{
					ID:    call.ID,
					Name:  tool.Name(),
					Input: string(input),
				})
			}

			return fantasy.NewTextErrorResponse(fmt.Sprintf("MCP tool %q on server %q is not currently usable; run mcp_tool_search again", toolName, server)), nil
		},
	)
}

func allowedMCPTools(available []*Tool, allowed map[string][]string) []*Tool {
	if allowed == nil {
		return available
	}
	if len(allowed) == 0 {
		return nil
	}
	filtered := make([]*Tool, 0, len(available))
	for _, tool := range available {
		allowedTools, ok := allowed[tool.MCP()]
		if !ok {
			continue
		}
		if len(allowedTools) == 0 || slices.Contains(allowedTools, tool.MCPToolName()) {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func formatMCPToolSearch(available []*Tool, params MCPToolSearchParams) string {
	query := strings.TrimSpace(params.Query)
	server := strings.TrimSpace(params.Server)
	limit := params.Limit
	if limit <= 0 {
		limit = defaultMCPToolMatches
	}
	limit = min(limit, maxMCPToolMatches)

	if query == "" && server == "" {
		return formatMCPServerInventory(available)
	}

	matches := make([]mcpToolMatch, 0, len(available))
	for _, tool := range available {
		if server != "" && !strings.EqualFold(server, tool.MCP()) {
			continue
		}
		score := scoreMCPTool(tool, query)
		if query != "" && score == 0 {
			continue
		}
		matches = append(matches, mcpToolMatch{tool: tool, score: score})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].tool.Name() < matches[j].tool.Name()
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	if len(matches) == 0 {
		return "No usable MCP tools matched. Search with a broader capability, omit server, or inspect the connected-server inventory with an empty query."
	}

	var output strings.Builder
	output.WriteString("Matching MCP tools (use exact server and tool values with mcp_tool_call):\n")
	output.WriteString(formatMatchedMCPInstructions(matches))
	for _, match := range matches {
		info := match.tool.Info()
		schema, _ := json.Marshal(map[string]any{
			"type":       "object",
			"properties": info.Parameters,
			"required":   info.Required,
		})
		fmt.Fprintf(&output, "\n- server: %s\n  tool: %s\n  description: %s\n  arguments_schema: %s\n",
			match.tool.MCP(), match.tool.MCPToolName(), compactMCPDescription(info.Description), schema)
	}
	return strings.TrimSpace(output.String())
}

func formatMatchedMCPInstructions(matches []mcpToolMatch) string {
	servers := make(map[string]bool)
	for _, match := range matches {
		servers[match.tool.MCP()] = true
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	slices.Sort(names)

	var output strings.Builder
	for _, name := range names {
		state, ok := mcpruntime.GetState(name)
		if !ok || state.State != mcpruntime.StateConnected || state.Client == nil {
			continue
		}
		instructions := compactMCPInstructions(state.Client.InitializeResult().Instructions)
		if instructions == "" {
			continue
		}
		fmt.Fprintf(&output, "\nServer instructions for %s: %s\n", name, instructions)
	}
	return output.String()
}

func formatMCPServerInventory(available []*Tool) string {
	counts := make(map[string]int)
	for _, tool := range available {
		counts[tool.MCP()]++
	}
	if len(counts) == 0 {
		return "No usable MCP tools are currently connected."
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	slices.Sort(names)
	var output strings.Builder
	output.WriteString("Connected MCP tool inventory:\n")
	for _, name := range names {
		fmt.Fprintf(&output, "- %s: %d tools\n", name, counts[name])
	}
	output.WriteString("Search by capability before calling a tool.")
	return strings.TrimSpace(output.String())
}

func scoreMCPTool(tool *Tool, query string) int {
	if query == "" {
		return 1
	}
	query = strings.ToLower(query)
	server := strings.ToLower(tool.MCP())
	name := strings.ToLower(tool.MCPToolName())
	description := strings.ToLower(tool.Info().Description)
	score := 0
	if query == server || query == name {
		score += 100
	}
	if strings.Contains(name, query) {
		score += 30
	}
	if strings.Contains(description, query) {
		score += 12
	}
	for _, token := range strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(token) < 2 {
			continue
		}
		if strings.Contains(name, token) {
			score += 10
		}
		if strings.Contains(server, token) {
			score += 8
		}
		if strings.Contains(description, token) {
			score += 3
		}
	}
	return score
}

func compactMCPDescription(description string) string {
	description = strings.Join(strings.Fields(description), " ")
	const maxDescription = 240
	if len(description) <= maxDescription {
		return description
	}
	return description[:maxDescription-1] + "…"
}

func compactMCPInstructions(instructions string) string {
	instructions = strings.Join(strings.Fields(instructions), " ")
	const maxInstructions = 1_000
	if len(instructions) <= maxInstructions {
		return instructions
	}
	return instructions[:maxInstructions-1] + "…"
}
