package tools

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
)

type ToolSearchParams struct {
	Query           string `json:"query" description:"Natural language or keyword query used to search available tool metadata"`
	Limit           int    `json:"limit,omitempty" description:"Maximum number of tool results to return (default: 10, max: 50)"`
	MaxResults      int    `json:"max_results,omitempty" description:"Alias for limit. Maximum number of tool results to return (default: 10, max: 50)"`
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

type ToolSearchResponse struct {
	Query             string             `json:"query"`
	Matches           []string           `json:"matches"`
	Results           []ToolSearchResult `json:"results,omitempty"`
	TotalDeferred     int                `json:"total_deferred_tools"`
	PendingMCPServers []string           `json:"pending_mcp_servers,omitempty"`
}

type (
	DeferredToolActivator  func(ctx context.Context, toolNames []string) []string
	PendingServersProvider func() []string
)

func NewToolSearchTool(registry Registry, activateDeferred DeferredToolActivator, pendingProviders ...PendingServersProvider) fantasy.AgentTool {
	pendingServersProvider := detectPendingMCPServers
	if len(pendingProviders) > 0 && pendingProviders[0] != nil {
		pendingServersProvider = pendingProviders[0]
	}

	return fantasy.NewParallelAgentTool(
		ToolSearchToolName,
		"Loads callable tool definitions from the local tool registry, including deferred tools that are hidden by default. Use 'select:tool_name' for direct activation, or keywords to search by name, description, and tags. Deferred matches from non-empty keyword queries are activated automatically. Returns {matches, results, total_deferred_tools, pending_mcp_servers}.",
		func(ctx context.Context, params ToolSearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = ctx
			_ = call
			if registry == nil {
				return fantasy.NewTextErrorResponse("tool registry is unavailable"), nil
			}
			limit := params.Limit
			if limit <= 0 {
				limit = params.MaxResults
			}
			if limit <= 0 {
				limit = 10
			} else if limit > 50 {
				limit = 50
			}

			query := strings.TrimSpace(params.Query)
			totalDeferred := countDeferredRegistryEntries(registry)
			if selectedNames, selectQuery := parseToolSelectQuery(query); selectQuery {
				results := selectToolEntries(ctx, registry, activateDeferred, selectedNames)
				pendingMCP := noMatchPendingServers(results, pendingServersProvider)
				return marshalToolSearchResponse(buildToolSearchResponse(query, results, totalDeferred, pendingMCP))
			}

			if query != "" {
				if entry, ok := resolveRegistryEntryByName(registry, query); ok && isRegistryEntrySearchable(entry) {
					results := selectToolEntries(ctx, registry, activateDeferred, []string{entry.Name})
					pendingMCP := noMatchPendingServers(results, pendingServersProvider)
					return marshalToolSearchResponse(buildToolSearchResponse(query, results, totalDeferred, pendingMCP))
				}
			}

			includeDeferred := params.IncludeDeferred
			if !params.ExposedOnly && !params.IncludeDeferred {
				includeDeferred = true
			}
			entries := searchableRegistryEntries(registry, RegistrySearchOptions{
				Limit:           10_000,
				IncludeDeferred: includeDeferred,
				ExposedOnly:     params.ExposedOnly,
			})
			entries = rankRegistryEntries(entries, query, limit)

			results := make([]ToolSearchResult, 0, len(entries))
			deferredToActivate := make([]string, 0, len(entries))
			for _, entry := range entries {
				results = append(results, toolSearchResultFromEntry(entry, false, false))
				if query != "" && entry.Metadata.IsDeferred() {
					deferredToActivate = append(deferredToActivate, entry.Name)
				}
			}
			if len(deferredToActivate) > 0 && activateDeferred != nil {
				activatedSet := makeActivatedSet(activateDeferred(ctx, deferredToActivate))
				for i := range results {
					if _, ok := activatedSet[results[i].Name]; ok {
						results[i].Activated = true
					}
				}
			}
			pendingMCP := noMatchPendingServers(results, pendingServersProvider)
			return marshalToolSearchResponse(buildToolSearchResponse(query, results, totalDeferred, pendingMCP))
		},
	)
}

func selectToolEntries(ctx context.Context, registry Registry, activateDeferred DeferredToolActivator, selectedNames []string) []ToolSearchResult {
	results := make([]ToolSearchResult, 0, len(selectedNames))
	deferredToActivate := make([]string, 0, len(selectedNames))
	for _, selectedName := range selectedNames {
		entry, ok := resolveRegistryEntryByName(registry, selectedName)
		if !ok || !isRegistryEntrySearchable(entry) {
			continue
		}
		results = append(results, toolSearchResultFromEntry(entry, true, false))
		if entry.Metadata.IsDeferred() {
			deferredToActivate = append(deferredToActivate, entry.Name)
		}
	}

	if len(deferredToActivate) == 0 || activateDeferred == nil {
		return results
	}
	activatedSet := makeActivatedSet(activateDeferred(ctx, deferredToActivate))
	for i := range results {
		if _, ok := activatedSet[results[i].Name]; ok {
			results[i].Activated = true
		}
	}
	return results
}

func makeActivatedSet(activated []string) map[string]struct{} {
	set := make(map[string]struct{}, len(activated))
	for _, name := range activated {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func countDeferredRegistryEntries(registry Registry) int {
	entries := registry.Search("", RegistrySearchOptions{Limit: 10_000, IncludeDeferred: true})
	count := 0
	for _, entry := range entries {
		if entry.Metadata.IsDeferred() {
			count++
		}
	}
	return count
}

func detectPendingMCPServers() []string {
	states := mcp.GetStates()
	if len(states) == 0 {
		return nil
	}
	pending := make([]string, 0, len(states))
	for name, state := range states {
		if state.State == mcp.StateStarting {
			pending = append(pending, name)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	sort.Strings(pending)
	return pending
}

func noMatchPendingServers(results []ToolSearchResult, provider PendingServersProvider) []string {
	if len(results) > 0 || provider == nil {
		return nil
	}
	pending := provider()
	if len(pending) == 0 {
		return nil
	}
	return append([]string(nil), pending...)
}

func searchableRegistryEntries(registry Registry, opts RegistrySearchOptions) []RegistryEntry {
	entries := registry.Search("", opts)
	filtered := make([]RegistryEntry, 0, len(entries))
	for _, entry := range entries {
		if !isRegistryEntrySearchable(entry) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func rankRegistryEntries(entries []RegistryEntry, query string, limit int) []RegistryEntry {
	if len(entries) == 0 || limit <= 0 {
		return nil
	}

	trimmedQuery := normalizeSearchQuery(query)
	if trimmedQuery == "" {
		if len(entries) <= limit {
			return entries
		}
		return append([]RegistryEntry(nil), entries[:limit]...)
	}

	requiredTerms, optionalTerms := parseSearchTerms(trimmedQuery)
	scoringTerms := optionalTerms
	if len(scoringTerms) == 0 {
		scoringTerms = requiredTerms
	}

	type scoredEntry struct {
		entry RegistryEntry
		score int
	}

	ranked := make([]scoredEntry, 0, len(entries))
	for _, entry := range entries {
		index := buildRegistrySearchIndex(entry)
		if !matchAllTerms(index, requiredTerms) {
			continue
		}

		score := scoreRegistryEntry(index, scoringTerms)
		if score == 0 {
			continue
		}
		ranked = append(ranked, scoredEntry{entry: entry, score: score})
	}

	if len(ranked) == 0 {
		return nil
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].entry.Exposed != ranked[j].entry.Exposed {
			return ranked[i].entry.Exposed
		}
		return ranked[i].entry.Name < ranked[j].entry.Name
	})

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	results := make([]RegistryEntry, 0, len(ranked))
	for _, item := range ranked {
		results = append(results, item.entry)
	}
	return results
}

func parseSearchTerms(query string) (required []string, optional []string) {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return nil, nil
	}

	for _, raw := range terms {
		term := strings.TrimSpace(raw)
		if term == "" {
			continue
		}
		if strings.HasPrefix(term, "+") && len(term) > 1 {
			required = append(required, term[1:])
			continue
		}
		optional = append(optional, term)
	}
	return dedupeTerms(required), dedupeTerms(optional)
}

func dedupeTerms(terms []string) []string {
	if len(terms) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(terms))
	normalized := make([]string, 0, len(terms))
	for _, term := range terms {
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		normalized = append(normalized, term)
	}
	return normalized
}

type registrySearchIndex struct {
	name        string
	nameFull    string
	nameParts   []string
	isMCP       bool
	hint        string
	description string
	source      string
	tags        []string
	blob        string
}

func buildRegistrySearchIndex(entry RegistryEntry) registrySearchIndex {
	nameFull, nameParts, isMCP := parseRegistryEntryName(entry.Name)

	terms := entry.Metadata.SearchTerms(entry.Name, entry.Description)
	blobParts := make([]string, 0, len(terms)+2)
	for _, term := range terms {
		trimmed := strings.TrimSpace(strings.ToLower(term))
		if trimmed != "" {
			blobParts = append(blobParts, trimmed)
		}
	}

	if source := strings.TrimSpace(strings.ToLower(entry.Source)); source != "" {
		blobParts = append(blobParts, source)
	}

	if data, err := json.Marshal(entry.Parameters); err == nil {
		text := strings.TrimSpace(strings.ToLower(string(data)))
		if text != "" {
			blobParts = append(blobParts, text)
		}
	}

	tags := make([]string, 0, len(entry.Metadata.SearchTags))
	for _, tag := range entry.Metadata.SearchTags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag != "" {
			tags = append(tags, tag)
		}
	}

	return registrySearchIndex{
		name:        strings.ToLower(entry.Name),
		nameFull:    nameFull,
		nameParts:   nameParts,
		isMCP:       isMCP,
		hint:        strings.ToLower(entry.Metadata.SearchHint),
		description: strings.ToLower(entry.Description),
		source:      strings.ToLower(entry.Source),
		tags:        tags,
		blob:        strings.Join(blobParts, "\n"),
	}
}

func parseRegistryEntryName(name string) (full string, parts []string, isMCP bool) {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if normalized == "" {
		return "", nil, false
	}

	if strings.HasPrefix(normalized, "mcp_") {
		isMCP = true
		normalized = strings.TrimPrefix(normalized, "mcp_")
	}

	replacer := strings.NewReplacer("__", " ", "_", " ", "-", " ", "/", " ")
	normalized = replacer.Replace(normalized)
	rawParts := strings.Fields(normalized)

	deduped := make([]string, 0, len(rawParts))
	seen := make(map[string]struct{}, len(rawParts))
	for _, part := range rawParts {
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		deduped = append(deduped, part)
	}

	return strings.Join(deduped, " "), deduped, isMCP
}

func normalizeSearchQuery(query string) string {
	normalized := strings.TrimSpace(strings.ToLower(query))
	if normalized == "" {
		return ""
	}
	replacer := strings.NewReplacer("mcp__", "mcp_", "__", "_", "-", " ")
	normalized = replacer.Replace(normalized)
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

func matchAllTerms(index registrySearchIndex, required []string) bool {
	if len(required) == 0 {
		return true
	}
	for _, term := range required {
		if !strings.Contains(index.blob, term) {
			return false
		}
	}
	return true
}

func scoreRegistryEntry(index registrySearchIndex, terms []string) int {
	if len(terms) == 0 {
		return 0
	}
	score := 0
	for _, term := range terms {
		if term == "" {
			continue
		}
		if index.name == term {
			score += 30
		}
		if index.nameFull == term {
			score += 20
		}
		for _, part := range index.nameParts {
			if part == term {
				if index.isMCP {
					score += 14
				} else {
					score += 12
				}
				continue
			}
			if strings.Contains(part, term) {
				if index.isMCP {
					score += 7
				} else {
					score += 5
				}
			}
		}
		if strings.Contains(index.nameFull, term) {
			score += 3
		}
		if strings.Contains(index.name, term) {
			score += 12
		}
		for _, tag := range index.tags {
			if tag == term {
				score += 8
				break
			}
			if strings.Contains(tag, term) {
				score += 4
			}
		}
		if strings.Contains(index.hint, term) {
			score += 6
		}
		if strings.Contains(index.description, term) {
			score += 4
		}
		if strings.Contains(index.source, term) {
			score += 2
		}
		if strings.Contains(index.blob, term) {
			score += 1
		}
	}
	return score
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
	candidates := lookupNameCandidates(name)
	for _, candidate := range candidates {
		if entry, ok := registry.Resolve(candidate); ok {
			return entry, true
		}
	}
	entries := registry.Search("", RegistrySearchOptions{Limit: 10_000, IncludeDeferred: true})
	for _, entry := range entries {
		for _, candidate := range candidates {
			if strings.EqualFold(entry.Name, candidate) {
				return entry, true
			}
		}
	}
	return RegistryEntry{}, false
}

func lookupNameCandidates(requested string) []string {
	trimmed := strings.TrimSpace(requested)
	if trimmed == "" {
		return nil
	}

	candidates := make([]string, 0, 4)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		candidates = append(candidates, value)
	}

	add(trimmed)
	add(strings.ReplaceAll(trimmed, "__", "_"))

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "mcp__") {
		rest := trimmed[len("mcp__"):]
		add("mcp_" + strings.ReplaceAll(rest, "__", "_"))
	}
	if strings.HasPrefix(lower, "mcp_") {
		rest := trimmed[len("mcp_"):]
		add("mcp__" + strings.ReplaceAll(rest, "_", "__"))
	}

	return candidates
}

func isRegistryEntrySearchable(entry RegistryEntry) bool {
	if entry.Exposed {
		return true
	}
	return entry.Metadata.IsDeferred()
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

func buildToolSearchResponse(query string, results []ToolSearchResult, totalDeferred int, pendingMCP []string) ToolSearchResponse {
	matches := make([]string, 0, len(results))
	for _, result := range results {
		matches = append(matches, result.Name)
	}

	response := ToolSearchResponse{
		Query:         query,
		Matches:       matches,
		Results:       results,
		TotalDeferred: totalDeferred,
	}
	if len(pendingMCP) > 0 {
		response.PendingMCPServers = append([]string(nil), pendingMCP...)
	}
	return response
}

func marshalToolSearchResponse(response ToolSearchResponse) (fantasy.ToolResponse, error) {
	data, err := json.Marshal(response)
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	return fantasy.NewTextResponse(string(data)), nil
}
