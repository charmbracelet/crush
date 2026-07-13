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
	defaultMCPToolMatches = 5
)

//go:embed mcp_tool_search.md
var mcpToolSearchDescription string

type MCPToolSearchParams struct {
	Query      string `json:"query,omitempty" description:"Capability to search for, or select:<exact_native_tool_name> to load one or more comma-separated native tool schemas."`
	Server     string `json:"server,omitempty" description:"Optional exact MCP server name used to restrict a keyword search."`
	MaxResults int    `json:"max_results,omitempty" description:"Number of ranked keyword matches to return. Defaults to 5; no fixed catalog ceiling is applied."`
	Offset     int    `json:"offset,omitempty" description:"Zero-based match offset for reading the next result page."`
	Limit      int    `json:"limit,omitempty" description:"Deprecated alias for max_results."`
}

type mcpToolMatch struct {
	tool  *Tool
	score int
}

// DeferredToolProvider resolves names selected through a compact discovery
// tool. The agent exposes resolved tools natively on the following step.
type DeferredToolProvider interface {
	fantasy.AgentTool
	ResolveDeferredTools(names []string) []fantasy.AgentTool
}

type mcpToolSearchTool struct {
	fantasy.AgentTool
	permissions permission.Service
	cfg         *config.ConfigStore
	workingDir  string
	allowed     map[string][]string
}

// NewMCPToolSearchTool creates a compact catalog over every currently usable
// MCP tool. The catalog is rebuilt per call so servers added or refreshed
// during a session become discoverable without rebuilding the agent.
func NewMCPToolSearchTool(permissions permission.Service, cfg *config.ConfigStore, workingDir string, allowed map[string][]string) fantasy.AgentTool {
	search := &mcpToolSearchTool{
		permissions: permissions,
		cfg:         cfg,
		workingDir:  workingDir,
		allowed:     allowed,
	}
	search.AgentTool = fantasy.NewParallelAgentTool(
		MCPToolSearchToolName,
		mcpToolSearchDescription,
		func(_ context.Context, params MCPToolSearchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse(formatMCPToolSearch(search.availableTools(), params)), nil
		},
	)
	return search
}

func (m *mcpToolSearchTool) availableTools() []*Tool {
	return allowedMCPTools(GetMCPTools(m.permissions, m.cfg, m.workingDir), m.allowed)
}

func (m *mcpToolSearchTool) ResolveDeferredTools(names []string) []fantasy.AgentTool {
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			wanted[name] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil
	}

	available := m.availableTools()
	sort.Slice(available, func(i, j int) bool {
		return available[i].Name() < available[j].Name()
	})
	resolved := make([]fantasy.AgentTool, 0, len(wanted))
	for _, tool := range available {
		if _, ok := wanted[strings.ToLower(tool.Name())]; ok {
			resolved = append(resolved, tool)
		}
	}
	return resolved
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
	if selected := DeferredToolSelectionNames(query); len(selected) > 0 {
		return formatSelectedMCPTools(available, selected)
	}

	limit := params.MaxResults
	if limit <= 0 {
		limit = params.Limit
	}
	if limit <= 0 {
		limit = defaultMCPToolMatches
	}
	offset := max(params.Offset, 0)

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
	if len(matches) == 0 {
		return "No usable MCP tools matched. Search with broader capability words, omit server, or inspect the connected-server inventory with an empty query."
	}
	totalMatches := len(matches)
	if offset >= totalMatches {
		return fmt.Sprintf("No matches at offset %d. total_matches: %d", offset, totalMatches)
	}
	end := min(offset+limit, totalMatches)
	matches = matches[offset:end]

	var output strings.Builder
	output.WriteString("Deferred MCP tool matches:\n")
	for _, match := range matches {
		info := match.tool.Info()
		fmt.Fprintf(&output, "- name: %s\n  server: %s\n  description: %s\n",
			match.tool.Name(), match.tool.MCP(), compactMCPDescription(info.Description))
	}
	fmt.Fprintf(&output, "total_matches: %d\nreturned: %d\noffset: %d\n", totalMatches, len(matches), offset)
	if end < totalMatches {
		fmt.Fprintf(&output, "next_offset: %d\n", end)
	}
	fmt.Fprintf(&output, "total_deferred_tools: %d\n", len(available))
	output.WriteString("Load a native schema with mcp_tool_search query select:<exact_name>. Multiple exact names may be comma-separated.")
	return strings.TrimSpace(output.String())
}

// DeferredToolSelectionNames parses the exact selection syntax shared by the
// discovery result and the per-step tool resolver.
func DeferredToolSelectionNames(query string) []string {
	query = strings.TrimSpace(query)
	const prefix = "select:"
	if len(query) < len(prefix) || !strings.EqualFold(query[:len(prefix)], prefix) {
		return nil
	}

	seen := make(map[string]struct{})
	var names []string
	for _, raw := range strings.Split(query[len(prefix):], ",") {
		name := strings.TrimSpace(raw)
		key := strings.ToLower(name)
		if name == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	return names
}

func formatSelectedMCPTools(available []*Tool, selected []string) string {
	wanted := make(map[string]struct{}, len(selected))
	for _, name := range selected {
		wanted[strings.ToLower(name)] = struct{}{}
	}

	var matches []*Tool
	for _, tool := range available {
		if _, ok := wanted[strings.ToLower(tool.Name())]; ok {
			matches = append(matches, tool)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Name() < matches[j].Name()
	})
	if len(matches) == 0 {
		return "No exact deferred MCP tool matched. Run a keyword search and select a returned native name exactly."
	}

	var output strings.Builder
	output.WriteString("Selected native MCP tools are available on the next model step. Call them directly by name.\n")
	for _, tool := range matches {
		info := tool.Info()
		schema, _ := json.Marshal(map[string]any{
			"type":       "object",
			"properties": info.Parameters,
			"required":   info.Required,
		})
		fmt.Fprintf(&output, "- name: %s\n  server: %s\n  description: %s\n  input_schema: %s\n",
			tool.Name(), tool.MCP(), compactMCPDescription(info.Description), schema)
	}
	output.WriteString(formatMatchedMCPInstructions(matches))
	fmt.Fprintf(&output, "\nselected: %d\ntotal_deferred_tools: %d", len(matches), len(available))
	return strings.TrimSpace(output.String())
}

func formatMatchedMCPInstructions(matches []*Tool) string {
	servers := make(map[string]bool)
	for _, match := range matches {
		servers[match.MCP()] = true
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
	output.WriteString("Search by capability before selecting a native tool.")
	return strings.TrimSpace(output.String())
}

func scoreMCPTool(tool *Tool, query string) int {
	if query == "" {
		return 1
	}
	query = strings.ToLower(query)
	server := strings.ToLower(tool.MCP())
	name := strings.ToLower(tool.MCPToolName())
	fullName := strings.ToLower(tool.Name())
	description := strings.ToLower(tool.Info().Description)
	score := 0
	if query == fullName {
		score += 200
	}
	if query == server || query == name {
		score += 100
	}
	if strings.Contains(name, query) || strings.Contains(fullName, query) {
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
	return description[:maxDescription-3] + "..."
}

func compactMCPInstructions(instructions string) string {
	instructions = strings.Join(strings.Fields(instructions), " ")
	const maxInstructions = 1_000
	if len(instructions) <= maxInstructions {
		return instructions
	}
	return instructions[:maxInstructions-3] + "..."
}
