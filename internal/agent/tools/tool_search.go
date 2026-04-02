package tools

import (
	"context"
	"encoding/json"
	"strings"

	"charm.land/fantasy"
)

type ToolSearchParams struct {
	Query           string `json:"query" description:"Natural language or keyword query used to search available tool metadata"`
	Limit           int    `json:"limit,omitempty" description:"Maximum number of tool results to return (default: 10, max: 50)"`
	IncludeDeferred bool   `json:"include_deferred,omitempty" description:"Include deferred tools that are hidden from the default exposed tool set"`
	ExposedOnly     bool   `json:"exposed_only,omitempty" description:"Only include tools currently exposed to the agent"`
}

type ToolSearchResult struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Required    []string       `json:"required,omitempty"`
	Source      string         `json:"source,omitempty"`
	Exposed     bool           `json:"exposed"`
	Selected    bool           `json:"selected,omitempty"`
	Activated   bool           `json:"activated,omitempty"`
	Metadata    ToolMetadata   `json:"metadata,omitempty"`
}

type DeferredToolActivator func(ctx context.Context, toolNames []string) []string

func NewToolSearchTool(registry Registry, activateDeferred DeferredToolActivator) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		ToolSearchToolName,
		"Loads callable tool definitions from the local tool registry, including deferred tools that are hidden by default. Use 'select:tool_name' for direct activation, or keywords to search by name, description, and tags.",
		func(ctx context.Context, params ToolSearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = ctx
			_ = call
			if registry == nil {
				return fantasy.NewTextErrorResponse("tool registry is unavailable"), nil
			}
			limit := params.Limit
			if limit <= 0 {
				limit = 10
			} else if limit > 50 {
				limit = 50
			}

			query := strings.TrimSpace(params.Query)
			if selectedNames, selectQuery := parseToolSelectQuery(query); selectQuery {
				results := make([]ToolSearchResult, 0, len(selectedNames))
				deferredToActivate := make([]string, 0, len(selectedNames))
				for _, selectedName := range selectedNames {
					entry, ok := resolveRegistryEntryByName(registry, selectedName)
					if !ok {
						continue
					}
					results = append(results, toolSearchResultFromEntry(entry, true, false))
					if entry.Metadata.IsDeferred() {
						deferredToActivate = append(deferredToActivate, entry.Name)
					}
				}
				if len(deferredToActivate) > 0 && activateDeferred != nil {
					activated := activateDeferred(ctx, deferredToActivate)
					activatedSet := make(map[string]struct{}, len(activated))
					for _, name := range activated {
						activatedSet[strings.TrimSpace(name)] = struct{}{}
					}
					for i := range results {
						if _, ok := activatedSet[results[i].Name]; ok {
							results[i].Activated = true
						}
					}
				}
				return marshalToolSearchResults(results)
			}

			includeDeferred := params.IncludeDeferred
			if !params.ExposedOnly && !params.IncludeDeferred {
				includeDeferred = true
			}
			entries := registry.Search(query, RegistrySearchOptions{
				Limit:           limit,
				IncludeDeferred: includeDeferred,
				ExposedOnly:     params.ExposedOnly,
			})
			results := make([]ToolSearchResult, 0, len(entries))
			for _, entry := range entries {
				results = append(results, toolSearchResultFromEntry(entry, false, false))
			}
			return marshalToolSearchResults(results)
		},
	)
}

func parseToolSelectQuery(query string) ([]string, bool) {
	match := strings.TrimSpace(query)
	if !strings.HasPrefix(strings.ToLower(match), "select:") {
		return nil, false
	}
	requested := strings.Split(match[len("select:"):], ",")
	normalized := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, item := range requested {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, name)
	}
	return normalized, true
}

func resolveRegistryEntryByName(registry Registry, requestedName string) (RegistryEntry, bool) {
	name := strings.TrimSpace(requestedName)
	if name == "" {
		return RegistryEntry{}, false
	}
	if entry, ok := registry.Resolve(name); ok {
		return entry, true
	}
	entries := registry.Search("", RegistrySearchOptions{Limit: 10_000, IncludeDeferred: true})
	for _, entry := range entries {
		if strings.EqualFold(entry.Name, name) {
			return entry, true
		}
	}
	return RegistryEntry{}, false
}

func toolSearchResultFromEntry(entry RegistryEntry, selected bool, activated bool) ToolSearchResult {
	return ToolSearchResult{
		Name:        entry.Name,
		Description: entry.Description,
		Parameters:  cloneMap(entry.Parameters),
		Required:    append([]string(nil), entry.Required...),
		Source:      entry.Source,
		Exposed:     entry.Exposed,
		Selected:    selected,
		Activated:   activated,
		Metadata:    entry.Metadata,
	}
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func marshalToolSearchResults(results []ToolSearchResult) (fantasy.ToolResponse, error) {
	data, err := json.Marshal(results)
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	return fantasy.NewTextResponse(string(data)), nil
}
