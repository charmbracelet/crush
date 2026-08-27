package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"charm.land/fantasy"
)

// ToolSearchToolName is the name of the meta-tool that lets the model discover
// deferred tools on demand instead of loading every MCP tool schema into
// context up front.
const ToolSearchToolName = "tool_search"

// defaultToolSearchThreshold is the number of MCP tools above which deferral
// kicks in. Below it, deferral is not worth the extra round-trip and the tools
// are loaded normally.
const defaultToolSearchThreshold = 20

// maxToolSearchResults caps how many tools a single search activates.
const maxToolSearchResults = 5

const toolSearchDescription = `Discover tools that are available but not currently loaded.

Many tools (typically from connected MCP servers) are deferred to keep the
context window small — their full schemas are not loaded until you ask for them.
Call this tool with keywords or a short description of the capability you need
(for example "query bigquery", "search dbt models", "read incident"). Matching
tools are loaded and become callable on your next turn. The response lists the
tools that were loaded, with their names and descriptions.`

// toolSearchInput is the input schema for the tool_search meta-tool.
type toolSearchInput struct {
	Query string `json:"query" description:"Keywords or a short natural-language description of the capability you need."`
}

// toolSearch holds a per-session catalog of deferred tools and the subset that
// has been activated (loaded) via the tool_search meta-tool. It is safe for
// concurrent use: the meta-tool's Run may mutate the activated set while the
// agent loop reads the effective set on each step.
type toolSearch struct {
	mu        sync.Mutex
	core      []fantasy.AgentTool          // always-loaded tools (built-ins + the search tool)
	catalog   []fantasy.AgentTool          // deferred tools, searchable but not loaded up front
	activated map[string]fantasy.AgentTool // name -> deferred tool that has been loaded
}

// newToolSearch builds a tool-search state over the given deferred catalog. If
// prev describes the same catalog (same set of tool names), its activated set
// is carried over so tools discovered earlier in a conversation stay loaded.
func newToolSearch(catalog []fantasy.AgentTool, prev *toolSearch) *toolSearch {
	ts := &toolSearch{
		catalog:   catalog,
		activated: make(map[string]fantasy.AgentTool),
	}
	if prev != nil && sameCatalog(prev.catalog, catalog) {
		prev.mu.Lock()
		for name, tool := range prev.activated {
			ts.activated[name] = tool
		}
		prev.mu.Unlock()
	}
	return ts
}

// effective returns the tools the model should see this turn: the core set plus
// any deferred tools that have been activated, sorted by name for a stable
// (cache-friendly) ordering.
func (ts *toolSearch) effective() []fantasy.AgentTool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	out := make([]fantasy.AgentTool, 0, len(ts.core)+len(ts.activated))
	out = append(out, ts.core...)
	for _, t := range ts.activated {
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b fantasy.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})
	return out
}

// activate matches catalog tools against query, marks the best matches as
// loaded, and returns the tools that were newly activated (for reporting to the
// model). Already-activated matches are skipped.
func (ts *toolSearch) activate(query string) []fantasy.AgentTool {
	ranked := rankTools(ts.catalog, query)
	ts.mu.Lock()
	defer ts.mu.Unlock()
	var loaded []fantasy.AgentTool
	for _, t := range ranked {
		if len(loaded) >= maxToolSearchResults {
			break
		}
		name := t.Info().Name
		if _, ok := ts.activated[name]; ok {
			continue
		}
		ts.activated[name] = t
		loaded = append(loaded, t)
	}
	return loaded
}

// tool returns the fantasy meta-tool. apply is invoked after each successful
// search with the new effective tool set, so the running agent picks up newly
// activated tools on its next step.
func (ts *toolSearch) tool(apply func([]fantasy.AgentTool)) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		ToolSearchToolName,
		toolSearchDescription,
		func(ctx context.Context, in toolSearchInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			query := strings.TrimSpace(in.Query)
			if query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}
			loaded := ts.activate(query)
			if apply != nil {
				apply(ts.effective())
			}
			if len(loaded) == 0 {
				return fantasy.NewTextResponse(fmt.Sprintf(
					"No tools matched %q. Try different or broader keywords.", query,
				)), nil
			}
			var b strings.Builder
			fmt.Fprintf(&b, "Loaded %d tool(s) — you can call them on your next turn:\n", len(loaded))
			for _, t := range loaded {
				info := t.Info()
				fmt.Fprintf(&b, "\n- %s: %s", info.Name, firstLine(info.Description))
			}
			return fantasy.NewTextResponse(b.String()), nil
		},
	)
}

// rankTools returns catalog tools that match query, best first. Matching is a
// dependency-free case-insensitive token-overlap over each tool's name and
// description; name matches are weighted more heavily. It is intentionally
// simple — enough for on-demand discovery without pulling in a search library.
func rankTools(catalog []fantasy.AgentTool, query string) []fantasy.AgentTool {
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	type scored struct {
		tool  fantasy.AgentTool
		score int
	}
	ranked := make([]scored, 0, len(catalog))
	for _, t := range catalog {
		info := t.Info()
		name := strings.ToLower(info.Name)
		hay := name + " " + strings.ToLower(info.Description)
		score := 0
		for _, term := range terms {
			if !strings.Contains(hay, term) {
				continue
			}
			score++
			if strings.Contains(name, term) {
				score += 2
			}
		}
		if score > 0 {
			ranked = append(ranked, scored{tool: t, score: score})
		}
	}
	slices.SortStableFunc(ranked, func(a, b scored) int { return b.score - a.score })
	out := make([]fantasy.AgentTool, len(ranked))
	for i, s := range ranked {
		out[i] = s.tool
	}
	return out
}

// tokenize lower-cases s and splits it into alphanumeric/`_`/`-` terms of length
// > 1, dropping trivial tokens.
func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	out := fields[:0]
	for _, f := range fields {
		if len(f) > 1 {
			out = append(out, f)
		}
	}
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// sameCatalog reports whether a and b contain the same set of tool names.
func sameCatalog(a, b []fantasy.AgentTool) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]struct{}, len(a))
	for _, t := range a {
		seen[t.Info().Name] = struct{}{}
	}
	for _, t := range b {
		if _, ok := seen[t.Info().Name]; !ok {
			return false
		}
	}
	return true
}
