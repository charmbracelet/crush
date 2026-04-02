package agent

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"charm.land/fantasy"
	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
)

type toolRegistry struct {
	entries  map[string]agenttools.RegistryEntry
	invokers map[string]func(context.Context, map[string]any, fantasy.ToolCall) (fantasy.ToolResponse, error)
}

func newToolRegistry() *toolRegistry {
	return &toolRegistry{
		entries:  map[string]agenttools.RegistryEntry{},
		invokers: map[string]func(context.Context, map[string]any, fantasy.ToolCall) (fantasy.ToolResponse, error){},
	}
}

func (r *toolRegistry) register(entry agenttools.RegistryEntry, invoker func(context.Context, map[string]any, fantasy.ToolCall) (fantasy.ToolResponse, error)) {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		return
	}
	entry.Name = name
	r.entries[name] = entry
	if invoker != nil {
		r.invokers[name] = invoker
	}
}

func (r *toolRegistry) Search(query string, opts agenttools.RegistrySearchOptions) []agenttools.RegistryEntry {
	entries := make([]agenttools.RegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return agenttools.SearchRegistryEntries(entries, query, opts)
}

func (r *toolRegistry) Resolve(name string) (agenttools.RegistryEntry, bool) {
	entry, ok := r.entries[strings.TrimSpace(name)]
	return entry, ok
}

func (r *toolRegistry) Invoke(ctx context.Context, name string, args map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	invoker, ok := r.invokers[strings.TrimSpace(name)]
	if !ok {
		return fantasy.NewTextErrorResponse("tool is not invokable from registry"), nil
	}
	return invoker(ctx, args, call)
}

func buildRegistryEntryFromTool(tool fantasy.AgentTool, source string, metadata agenttools.ToolMetadata, exposed bool) agenttools.RegistryEntry {
	info := tool.Info()
	entry := agenttools.RegistryEntry{
		Name:        info.Name,
		Description: info.Description,
		Parameters:  copyMap(info.Parameters),
		Required:    append([]string(nil), info.Required...),
		Source:      source,
		Metadata:    metadata.Normalized(isFantasyToolParallelSafe(tool)),
		Exposed:     exposed,
	}
	return entry
}

func invokeFantasyTool(tool fantasy.AgentTool) func(context.Context, map[string]any, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return func(ctx context.Context, args map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
		payload, err := json.Marshal(args)
		if err != nil {
			return fantasy.ToolResponse{}, err
		}
		forwarded := fantasy.ToolCall{ID: call.ID, Name: tool.Info().Name, Input: string(payload)}
		return tool.Run(ctx, forwarded)
	}
}

func copyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func isFantasyToolParallelSafe(tool fantasy.AgentTool) bool {
	if tool == nil {
		return false
	}
	name := strings.TrimSpace(tool.Info().Name)
	switch name {
	case agenttools.FetchToolName,
		agenttools.GlobToolName,
		agenttools.GrepToolName,
		agenttools.LSToolName,
		agenttools.ViewToolName,
		agenttools.HistorySearchToolName,
		agenttools.DiagnosticsToolName,
		agenttools.ReferencesToolName,
		agenttools.LSPDeclarationToolName,
		agenttools.LSPDefinitionToolName,
		agenttools.LSPImplementationToolName,
		agenttools.LSPTypeDefinitionToolName,
		agenttools.LSPHoverToolName,
		agenttools.LSPDocumentSymbolsToolName,
		agenttools.LSPWorkspaceSymbolsToolName,
		agenttools.ListMCPResourcesToolName,
		agenttools.ReadMCPResourceToolName,
		agenttools.ToolSearchToolName:
		return true
	default:
		return false
	}
}
