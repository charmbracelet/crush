package mcp

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
)

// ToolNamePrefix is what every MCP tool name starts with once it reaches
// the model. Checking the prefix rather than asserting a concrete tool type
// is what keeps callers working after hook wrapping, which hides a tool
// behind an unexported decorator.
const ToolNamePrefix = "mcp_"

// ToolName builds the model-facing name of a tool served by server.
func ToolName(server, tool string) string {
	return ToolNamePrefix + server + "_" + tool
}

// Search limits. DefaultSearchLimit mirrors the small result sets other
// agents settled on: enough candidates to choose from, few enough that a
// search cannot itself balloon the context it exists to protect.
const (
	DefaultSearchLimit = 8
	MaxSearchLimit     = 32
)

// ToolRef is a schema-free description of a single MCP tool. Dropping the
// parameter schema is the point: a ref is what Crush can afford to carry
// for tools the model has not asked for yet.
type ToolRef struct {
	// Server is the configured MCP server name.
	Server string
	// Tool is the tool name as the server knows it.
	Tool string
	// Name is the model-facing tool name, see ToolName.
	Name string
	// Description is the server-authored tool description.
	Description string
	// Required lists required input property names, so a search result
	// already says what a call needs.
	Required []string
	// Lazy reports whether the tool is hidden from context until a
	// session activates it.
	Lazy bool
}

// Catalog snapshots every tool the connected MCP servers have registered,
// sorted by model-facing name so results never depend on map order.
func Catalog(cfg *config.ConfigStore) []ToolRef {
	refs := make([]ToolRef, 0, 16)
	for server, tools := range allTools.Seq2() {
		lazy := isLazyServer(cfg, server)
		for _, tool := range tools {
			if tool == nil {
				continue
			}
			refs = append(refs, ToolRef{
				Server:      server,
				Tool:        tool.Name,
				Name:        ToolName(server, tool.Name),
				Description: tool.Description,
				Required:    RequiredInputs(tool.InputSchema),
				Lazy:        lazy,
			})
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	return refs
}

// Search ranks the catalog against query and returns at most limit tools,
// or DefaultSearchLimit when limit is not set.
//
// Two prefixes bypass ranking: "server:<name>" selects a whole server and
// "select:<name>,<name>" selects exact tools. Everything else is scored,
// with name matches outranking description matches.
func Search(cfg *config.ConfigStore, query string, limit int) []ToolRef {
	if limit <= 0 {
		limit = DefaultSearchLimit
	}
	if limit > MaxSearchLimit {
		limit = MaxSearchLimit
	}

	refs := Catalog(cfg)
	q := strings.TrimSpace(query)

	if rest, ok := selectionPrefix(q, "server:", "mcp:"); ok {
		return byServer(refs, rest, limit)
	}
	if rest, ok := selectionPrefix(q, "select:", "load:"); ok {
		return bySelection(refs, rest, limit)
	}

	type match struct {
		ref   ToolRef
		score int
	}
	matches := make([]match, 0, len(refs))
	for _, ref := range refs {
		if score := scoreRef(ref, q); score > 0 {
			matches = append(matches, match{ref: ref, score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].ref.Name < matches[j].ref.Name
	})

	results := make([]ToolRef, 0, min(len(matches), limit))
	for _, m := range matches {
		if len(results) == limit {
			break
		}
		results = append(results, m.ref)
	}
	return results
}

// PromptSections renders the MCP blocks appended to the system prompt:
// <mcp-instructions> from connected servers, then <available_mcp> naming the
// tools the model still has to load. Servers are visited in name order so a
// turn that changes nothing produces a byte-identical prompt. It returns an
// empty string when there is nothing to say, which keeps MCP-free setups
// paying nothing.
func PromptSections(cfg *config.ConfigStore, sessionID string) string {
	states := GetStates()
	names := make([]string, 0, len(states))
	for name := range states {
		names = append(names, name)
	}
	sort.Strings(names)

	var instructions strings.Builder
	for _, name := range names {
		server := states[name]
		if server.State != StateConnected || server.Client == nil {
			continue
		}
		if s := server.Client.InitializeResult().Instructions; s != "" {
			instructions.WriteString(s)
			instructions.WriteString("\n\n")
		}
	}

	var out strings.Builder
	if s := instructions.String(); s != "" {
		out.WriteString("<mcp-instructions>\n" + strings.TrimRight(s, "\n") + "\n</mcp-instructions>")
	}
	if lines := indexLines(cfg, sessionID); len(lines) > 0 {
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		// The block carries its own instructions: MCP goes unmentioned in
		// the system prompt until there is actually something to load.
		out.WriteString("<available_mcp>\n" +
			"MCP servers are connected, but their tool schemas are hidden to save context.\n" +
			"Call mcp_search with what you need, e.g. \"create a pull request\".\n" +
			"Loaded tools are callable on your next step, so continue the task rather than asking the user to proceed.\n" +
			strings.Join(lines, "\n") + "\n</available_mcp>")
	}
	return out.String()
}

// NamesForServers returns the model-facing tool names currently registered
// by the given servers. Names are derived from the registry instead of
// parsed apart, so servers and tools whose names contain underscores stay
// unambiguous.
func NamesForServers(servers []string) map[string]struct{} {
	wanted := make(map[string]struct{}, len(servers))
	for _, server := range servers {
		wanted[server] = struct{}{}
	}
	names := make(map[string]struct{})
	for server := range wanted {
		tools, ok := allTools.Get(server)
		if !ok {
			continue
		}
		for _, tool := range tools {
			if tool == nil {
				continue
			}
			names[ToolName(server, tool.Name)] = struct{}{}
		}
	}
	return names
}

// IndexLines renders the <available_mcp> block: one line per connected
// server whose tools are hidden, plus one per server waiting on OAuth. It
// is deliberately tiny — a name and a count is all the model needs to
// decide that searching is worth a step. Servers with nothing hidden are
// omitted, so an eager setup pays nothing extra.
func IndexLines(cfg *config.ConfigStore) []string {
	return indexLines(cfg, "")
}

func indexLines(cfg *config.ConfigStore, sessionID string) []string {
	hidden := make(map[string]int)
	for _, ref := range Catalog(cfg) {
		if ref.Lazy && !IsActivated(sessionID, ref.Name) {
			hidden[ref.Server]++
		}
	}

	lines := make([]string, 0, len(hidden)+1)
	for server, count := range hidden {
		lines = append(lines, fmt.Sprintf(
			"- %s: %d tool%s hidden, use mcp_search to load them",
			server, count, plural(count),
		))
	}
	sort.Strings(lines)

	// A server waiting on OAuth is worth naming: without it the model
	// searches, finds nothing, and cannot explain why.
	for _, name := range sortedNeedsAuth(cfg) {
		lines = append(lines, "- "+name+": needs authentication before its tools are available")
	}
	return lines
}

// RequiredInputs returns the required property names of an MCP tool input
// schema. The SDK delivers schemas as decoded JSON, so "required" is
// normally []any; []string shows up for schemas built in-process.
func RequiredInputs(inputSchema any) []string {
	input, ok := inputSchema.(map[string]any)
	if !ok {
		return nil
	}
	switch req := input["required"].(type) {
	case []any:
		required := make([]string, 0, len(req))
		for _, v := range req {
			if s, ok := v.(string); ok {
				required = append(required, s)
			}
		}
		return required
	case []string:
		return slices.Clone(req)
	default:
		return nil
	}
}

// isLazyServer resolves a server's laziness: the per-server flag wins,
// otherwise options.lazy_mcp decides.
func isLazyServer(cfg *config.ConfigStore, server string) bool {
	mcpCfg, ok := cfg.Config().MCP[server]
	if !ok {
		return false
	}
	return mcpCfg.IsLazy(cfg.Config().Options)
}

// scoreRef ranks one tool against a query. Zero means no match at all.
func scoreRef(ref ToolRef, query string) int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 1
	}
	tool := strings.ToLower(ref.Tool)
	full := strings.ToLower(ref.Name)
	server := strings.ToLower(ref.Server)
	desc := strings.ToLower(ref.Description)

	switch {
	case tool == q || full == q:
		return 1000
	case strings.HasPrefix(tool, q):
		return 800
	case strings.Contains(tool, q):
		return 600
	case strings.Contains(full, q):
		return 550
	case strings.Contains(server, q):
		return 400
	case strings.Contains(desc, q):
		return 300
	}

	// Nothing matched as a whole. "create pull request" should still find
	// create_pull_request, so fall back to counting word hits.
	score := 0
	for _, word := range strings.Fields(q) {
		if len(word) < 3 {
			continue
		}
		switch {
		case strings.Contains(tool, word), strings.Contains(server, word):
			score += 20
		case strings.Contains(desc, word):
			score += 5
		}
	}
	return score
}

func byServer(refs []ToolRef, server string, limit int) []ToolRef {
	want := strings.ToLower(strings.TrimSpace(server))
	results := make([]ToolRef, 0, min(len(refs), limit))
	for _, ref := range refs {
		if !strings.Contains(strings.ToLower(ref.Server), want) {
			continue
		}
		if len(results) == limit {
			break
		}
		results = append(results, ref)
	}
	return results
}

func bySelection(refs []ToolRef, selection string, limit int) []ToolRef {
	wanted := make(map[string]struct{})
	for _, name := range strings.Split(selection, ",") {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			wanted[name] = struct{}{}
		}
	}
	// The catalog is name-sorted, so results stay stable without depending
	// on the order the caller typed.
	results := make([]ToolRef, 0, min(len(wanted), limit))
	for _, ref := range refs {
		_, byFull := wanted[strings.ToLower(ref.Name)]
		_, byTool := wanted[strings.ToLower(ref.Tool)]
		if !byFull && !byTool {
			continue
		}
		if len(results) == limit {
			break
		}
		results = append(results, ref)
	}
	return results
}

// selectionPrefix reports whether q starts with one of the explicit
// selection prefixes, returning the remainder. A prefix with an empty
// remainder is not a selection: it should rank as an ordinary query instead
// of silently matching every tool.
func selectionPrefix(q string, prefixes ...string) (string, bool) {
	lower := strings.ToLower(q)
	for _, prefix := range prefixes {
		if _, ok := strings.CutPrefix(lower, prefix); !ok {
			continue
		}
		full := q[len(prefix):]
		if strings.TrimSpace(full) == "" {
			return "", false
		}
		return full, true
	}
	return "", false
}

func sortedNeedsAuth(cfg *config.ConfigStore) []string {
	var names []string
	for name, state := range GetStates() {
		if state.State != StateNeedsAuth {
			continue
		}
		if mcpCfg, ok := cfg.Config().MCP[name]; ok && mcpCfg.Disabled {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
