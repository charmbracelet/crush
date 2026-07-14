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
	Query      string `json:"query,omitempty" description:"Exact deferred tool selection such as select:mcp_github_get_me, or capability keywords to search the deferred tool names and descriptions."`
	Server     string `json:"server,omitempty" description:"Optional exact MCP server name used to restrict a keyword search."`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum fuzzy matches to load. Defaults to 5; exact select queries are not capped."`
}

type mcpToolMatch struct {
	tool  *Tool
	score int
}

// DeferredToolProvider resolves exact names or capability matches from a
// compact discovery tool. The agent exposes resolved tools natively on the
// following step.
type DeferredToolProvider interface {
	fantasy.AgentTool
	ResolveDeferredTools(names []string) []fantasy.AgentTool
	ResolveDeferredToolSearch(params MCPToolSearchParams) []fantasy.AgentTool
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

func (m *mcpToolSearchTool) Info() fantasy.ToolInfo {
	info := m.AgentTool.Info()
	if catalog := compactMCPToolCatalog(m.availableTools()); catalog != "" {
		info.Description = strings.TrimSpace(info.Description) + "\n\n" + catalog
	}
	return info
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

func (m *mcpToolSearchTool) ResolveDeferredToolSearch(params MCPToolSearchParams) []fantasy.AgentTool {
	if strings.TrimSpace(params.Query) == "" || len(DeferredToolSelectionNames(params.Query)) > 0 {
		return nil
	}
	matches, _ := rankedMCPToolMatches(m.availableTools(), params)
	resolved := make([]fantasy.AgentTool, 0, len(matches))
	for _, match := range matches {
		resolved = append(resolved, match.tool)
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

	if query == "" && server == "" {
		return formatMCPServerInventory(available)
	}
	if query == "" {
		return formatMCPServerSearchPrompt(available, server)
	}

	matches, totalMatches := rankedMCPToolMatches(available, params)
	if len(matches) == 0 {
		return "No usable MCP tools matched. Retry once with an exact tool name from the catalog or an empty query to inspect it. If the needed capability is still unclear, load the mcp-setup skill for recovery guidance."
	}

	var output strings.Builder
	output.WriteString("Matched native MCP tools are available on the next model step. Call the best match directly by name:\n")
	for _, match := range matches {
		info := match.tool.Info()
		fmt.Fprintf(&output, "- name: %s\n  server: %s\n  description: %s\n",
			match.tool.Name(), match.tool.MCP(), compactMCPDescription(info.Description))
	}
	fmt.Fprintf(&output, "total_matches: %d\nloaded: %d\ntotal_deferred_tools: %d", totalMatches, len(matches), len(available))
	return strings.TrimSpace(output.String())
}

func rankedMCPToolMatches(available []*Tool, params MCPToolSearchParams) ([]mcpToolMatch, int) {
	query := strings.TrimSpace(params.Query)
	server := strings.TrimSpace(params.Server)
	server, query = mcpSearchScope(available, server, query)
	limit := params.MaxResults
	if limit <= 0 {
		limit = defaultMCPToolMatches
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
	totalMatches := len(matches)
	return matches[:min(limit, totalMatches)], totalMatches
}

func mcpSearchScope(available []*Tool, explicitServer, query string) (string, string) {
	queryTerms := mcpSearchTerms(query)
	if len(queryTerms) == 0 {
		return explicitServer, query
	}

	server := explicitServer
	if server == "" {
		seen := make(map[string]bool)
		for _, tool := range available {
			name := tool.MCP()
			if seen[name] {
				continue
			}
			seen[name] = true
			serverTerms := meaningfulMCPServerTerms(name)
			if len(serverTerms) > 0 && containsAllMCPSearchTerms(queryTerms, serverTerms) {
				if server != "" {
					return explicitServer, query
				}
				server = name
			}
		}
	}
	if server == "" {
		return "", query
	}

	remove := mcpSearchTermSet(strings.Join(meaningfulMCPServerTerms(server), " "))
	remaining := make([]string, 0, len(queryTerms))
	for _, term := range queryTerms {
		if !remove[term] {
			remaining = append(remaining, term)
		}
	}
	return server, strings.Join(remaining, " ")
}

func meaningfulMCPServerTerms(server string) []string {
	terms := mcpSearchTerms(server)
	result := terms[:0]
	for _, term := range terms {
		if term != "mcp" {
			result = append(result, term)
		}
	}
	return result
}

func containsAllMCPSearchTerms(haystack, needles []string) bool {
	available := make(map[string]bool, len(haystack))
	for _, term := range haystack {
		available[term] = true
	}
	for _, term := range needles {
		if !available[term] {
			return false
		}
	}
	return true
}

func formatMCPServerSearchPrompt(available []*Tool, server string) string {
	var matches []*Tool
	for _, tool := range available {
		if strings.EqualFold(server, tool.MCP()) {
			matches = append(matches, tool)
		}
	}
	if len(matches) == 0 {
		return fmt.Sprintf("No usable MCP tools are connected for server %q. Inspect the connected-server inventory with an empty query.", server)
	}
	return compactMCPToolCatalog(matches) + "\nSelect an exact name or search this server with capability words to load matching schemas."
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
	if len(available) == 0 {
		return "No usable MCP tools are currently connected."
	}
	return compactMCPToolCatalog(available) + "\nSearch once with an exact tool name or a plain-language task; matching native schemas become callable on the next model step."
}

func compactMCPToolCatalog(available []*Tool) string {
	if len(available) == 0 {
		return ""
	}
	tools := slices.Clone(available)
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].MCP() != tools[j].MCP() {
			return tools[i].MCP() < tools[j].MCP()
		}
		return tools[i].Name() < tools[j].Name()
	})

	var output strings.Builder
	output.WriteString("Available deferred MCP tools. Exact names are visible now; full descriptions and input schemas load on demand:\n")
	currentServer := ""
	for _, tool := range tools {
		if tool.MCP() != currentServer {
			currentServer = tool.MCP()
			fmt.Fprintf(&output, "%s:\n", currentServer)
		}
		fmt.Fprintf(&output, "- %s\n", tool.Name())
	}
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
	nameTerms := mcpSearchTermSet(name)
	serverTerms := mcpSearchTermSet(server)
	descriptionTerms := mcpSearchTermSet(description)
	for _, token := range mcpSearchTerms(query) {
		if nameTerms[token] {
			score += 20
		}
		if serverTerms[token] {
			score += 8
		}
		if descriptionTerms[token] {
			score += 6
		}
	}
	return score
}

func mcpSearchTermSet(value string) map[string]bool {
	terms := mcpSearchTerms(value)
	result := make(map[string]bool, len(terms))
	for _, term := range terms {
		result[term] = true
	}
	return result
}

func mcpSearchTerms(value string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		token = normalizeMCPSearchTerm(token)
		if len(token) < 3 || lowSignalMCPSearchTerm(token) || seen[token] {
			continue
		}
		seen[token] = true
		result = append(result, token)
	}
	return result
}

func lowSignalMCPSearchTerm(term string) bool {
	switch term {
	case "get", "info", "information", "detail", "tool", "using", "with":
		return true
	default:
		return false
	}
}

func normalizeMCPSearchTerm(term string) string {
	switch {
	case len(term) > 4 && strings.HasSuffix(term, "ies"):
		return term[:len(term)-3] + "y"
	case len(term) > 4 && strings.HasSuffix(term, "ches"):
		return term[:len(term)-2]
	case len(term) > 4 && strings.HasSuffix(term, "shes"):
		return term[:len(term)-2]
	case len(term) > 3 && strings.HasSuffix(term, "s") && !strings.HasSuffix(term, "ss"):
		return term[:len(term)-1]
	default:
		return term
	}
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
