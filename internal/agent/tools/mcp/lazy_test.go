package mcp

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func lazy(v bool) *bool { return &v }

func mcpStore(servers map[string]config.MCPConfig, globalLazy *bool) *config.ConfigStore {
	options := &config.Options{LazyMCP: globalLazy}
	return config.NewTestStore(&config.Config{MCP: servers, Options: options})
}

// The MCP tool registry and state map are process-global, so no test in
// this file runs in parallel: each one seeds and clears shared maps.

// seedTools registers fake tools for a server, as a connected client would.
func seedTools(t *testing.T, server string, names ...string) {
	t.Helper()
	t.Cleanup(func() { allTools.Del(server) })

	tools := make([]*Tool, 0, len(names))
	for _, name := range names {
		tools = append(tools, &mcp.Tool{
			Name:        name,
			Description: "does " + name,
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"id": map[string]any{"type": "string"}},
				"required":   []any{"id"},
			},
		})
	}
	allTools.Set(server, tools)
}

func TestToolName(t *testing.T) {
	require.Equal(t, "mcp_github_create_issue", ToolName("github", "create_issue"))
	require.True(t, strings.HasPrefix(ToolName("github", "x"), ToolNamePrefix))
}

func TestNamesForServers(t *testing.T) {
	seedTools(t, "nf-a", "one", "two")
	seedTools(t, "nf-b", "three")

	names := NamesForServers([]string{"nf-a", "missing"})
	require.Equal(t, map[string]struct{}{
		"mcp_nf-a_one": {},
		"mcp_nf-a_two": {},
	}, names)
}

func TestRequiredInputs(t *testing.T) {
	require.Equal(t, []string{"a", "b"}, RequiredInputs(map[string]any{
		"required": []any{"a", "b", 1},
	}))
	require.Equal(t, []string{"a"}, RequiredInputs(map[string]any{"required": []string{"a"}}))
	require.Nil(t, RequiredInputs(map[string]any{"type": "object"}))
	require.Nil(t, RequiredInputs("not a schema"))
}

func TestCatalogIsSortedAndMarksLazy(t *testing.T) {
	seedTools(t, "cat-b", "zeta")
	seedTools(t, "cat-a", "alpha", "beta")
	cfg := mcpStore(map[string]config.MCPConfig{
		"cat-a": {Type: config.MCPStdio, Lazy: lazy(false)},
		"cat-b": {Type: config.MCPStdio},
	}, nil)

	refs := Catalog(cfg)
	require.Len(t, refs, 3)
	// Sorted by model-facing name, never by registry order.
	require.Equal(t, "mcp_cat-a_alpha", refs[0].Name)
	require.Equal(t, "mcp_cat-a_beta", refs[1].Name)
	require.Equal(t, "mcp_cat-b_zeta", refs[2].Name)

	for _, ref := range refs {
		switch ref.Server {
		case "cat-a":
			require.False(t, ref.Lazy, "a pinned server is not lazy")
		default:
			require.True(t, ref.Lazy, "unset follows the global default")
		}
		require.Equal(t, []string{"id"}, ref.Required)
	}
}

func TestSearchRanking(t *testing.T) {
	seedTools(t, "rank", "create_issue", "list_issues", "close_pr")
	cfg := mcpStore(map[string]config.MCPConfig{"rank": {Type: config.MCPStdio}}, nil)

	got := Search(cfg, "create_issue", 0)
	require.Equal(t, []string{"mcp_rank_create_issue"}, names(got))

	// A tool-name match outranks a description-only match.
	got = Search(cfg, "issue", 1)
	require.Equal(t, []string{"mcp_rank_create_issue"}, names(got))

	// Word-overlap fallback: no single substring of "make pr" matches
	// close_pr, but "pr" does.
	got = Search(cfg, "close the pr", 2)
	require.Equal(t, []string{"mcp_rank_close_pr"}, names(got)[:1])

	require.Empty(t, Search(cfg, "zzzzznope", 0))
}

func TestSearchExplicitSelections(t *testing.T) {
	seedTools(t, "sel", "one", "two")
	cfg := mcpStore(map[string]config.MCPConfig{"sel": {Type: config.MCPStdio}}, nil)

	require.Equal(t, []string{"mcp_sel_one", "mcp_sel_two"}, names(Search(cfg, "server:sel", 0)))
	require.Equal(t, []string{"mcp_sel_two"}, names(Search(cfg, "select:mcp_sel_two", 0)))
	// Bare tool names work too.
	require.Equal(t, []string{"mcp_sel_one"}, names(Search(cfg, "select:one", 0)))
	// A prefix with nothing after it must not match everything.
	require.Empty(t, Search(cfg, "server:", 0))
}

func TestSearchLimitClamp(t *testing.T) {
	seedTools(t, "many", "a_tool", "b_tool", "c_tool")
	cfg := mcpStore(map[string]config.MCPConfig{"many": {Type: config.MCPStdio}}, nil)

	require.Len(t, Search(cfg, "tool", 2), 2)
	require.Len(t, Search(cfg, "tool", 1000), 3, "the clamp is above the catalog size")
	require.Len(t, Search(cfg, "", 0), 3, "an empty query lists everything up to the default")
}

func TestIndexLines(t *testing.T) {
	seedTools(t, "idx-hidden", "one", "two")
	seedTools(t, "idx-pinned", "three")
	cfg := mcpStore(map[string]config.MCPConfig{
		"idx-hidden": {Type: config.MCPStdio},
		"idx-pinned": {Type: config.MCPStdio, Lazy: lazy(false)},
	}, nil)

	lines := IndexLines(cfg)
	require.Equal(t, []string{"- idx-hidden: 2 tools hidden, use mcp_search to load them"}, lines)

	// An eager setup contributes nothing, so the prompt block stays absent.
	pinned := mcpStore(map[string]config.MCPConfig{
		"idx-hidden": {Type: config.MCPStdio, Lazy: lazy(false)},
		"idx-pinned": {Type: config.MCPStdio, Lazy: lazy(false)},
	}, nil)
	require.Empty(t, IndexLines(pinned))
}

func TestIndexLinesCountsNeedsAuth(t *testing.T) {
	const name = "idx-auth"
	t.Cleanup(func() { states.Del(name) })
	states.Set(name, ClientInfo{Name: name, State: StateNeedsAuth})

	cfg := mcpStore(map[string]config.MCPConfig{name: {Type: config.MCPHttp}}, nil)
	require.Equal(t,
		[]string{"- idx-auth: needs authentication before its tools are available"},
		IndexLines(cfg),
	)

	// Disabled servers are not worth nagging about.
	disabled := mcpStore(map[string]config.MCPConfig{name: {Type: config.MCPHttp, Disabled: true}}, nil)
	require.Empty(t, IndexLines(disabled))
}

func TestPromptSectionsQuietWithoutMCP(t *testing.T) {
	// Nothing connected and nothing configured: no block at all, which is
	// what keeps MCP-free workspaces byte-identical.
	empty := mcpStore(map[string]config.MCPConfig{}, nil)
	require.Empty(t, PromptSections(empty, "sess-quiet"))
}

func TestDeferredNamesOnlyHidesLazyUnactivated(t *testing.T) {
	seedTools(t, "def-a", "one", "two")
	seedTools(t, "def-b", "three")
	cfg := mcpStore(map[string]config.MCPConfig{
		"def-a": {Type: config.MCPStdio},
		"def-b": {Type: config.MCPStdio, Lazy: lazy(false)},
	}, nil)

	const session = "sess-deferred"
	defer ClearSession(session)

	deferred := DeferredNames(cfg, session)
	require.Equal(t, map[string]struct{}{
		"mcp_def-a_one": {},
		"mcp_def-a_two": {},
	}, deferred)

	Activate(session, "mcp_def-a_one")
	deferred = DeferredNames(cfg, session)
	require.Equal(t, map[string]struct{}{"mcp_def-a_two": {}}, deferred)
	require.Equal(t, 1, DeferredCount(cfg, session))
}

func TestDeferredNamesNilWhenNothingHidden(t *testing.T) {
	seedTools(t, "def-off", "one")
	cfg := mcpStore(map[string]config.MCPConfig{"def-off": {Type: config.MCPStdio}}, lazy(false))
	require.Nil(t, DeferredNames(cfg, "sess-nil"))
}

func TestActivateReturnsOnlyNewNames(t *testing.T) {
	const session = "sess-new"
	defer ClearSession(session)

	require.Equal(t, []string{"a", "b"}, Activate(session, "a", "b"))
	require.Empty(t, Activate(session, "a"), "re-activating is not new")
	require.Equal(t, []string{"mcp_x_a"}, Activate(session, "mcp_x_a", "a"), "distinct names are distinct")
}

func TestActivationIsPerSession(t *testing.T) {
	seedTools(t, "iso", "one")
	cfg := mcpStore(map[string]config.MCPConfig{"iso": {Type: config.MCPStdio}}, nil)

	Activate("iso-first", "mcp_iso_one")
	require.True(t, IsActivated("iso-first", "mcp_iso_one"))
	require.False(t, IsActivated("iso-second", "mcp_iso_one"),
		"one session must not reveal another session's tools")
	require.Len(t, DeferredNames(cfg, "iso-second"), 1)
}

func TestActivateEvictsLeastRecentlyUsed(t *testing.T) {
	const session = "sess-cap"
	defer ClearSession(session)

	first := make([]string, 0, MaxActivatedPerSession)
	for i := range MaxActivatedPerSession {
		first = append(first, string(rune('a'+i))+"_tool")
	}
	Activate(session, first...)
	require.Len(t, ActivatedTools(session), MaxActivatedPerSession)

	// Pushing one more evicts the oldest, not the newest.
	Activate(session, "overflow_tool")
	activated := ActivatedTools(session)
	require.Len(t, activated, MaxActivatedPerSession)
	require.False(t, containsName(activated, first[0]))
	require.True(t, containsName(activated, first[1]))
	require.True(t, containsName(activated, "overflow_tool"))
}

func TestReActivateDefersEviction(t *testing.T) {
	const session = "sess-refresh"
	defer ClearSession(session)

	first := make([]string, 0, MaxActivatedPerSession)
	for i := range MaxActivatedPerSession {
		first = append(first, string(rune('a'+i))+"_tool")
	}
	Activate(session, first...)

	// Touching the oldest entry makes it the last candidate to be evicted.
	Activate(session, first[0])
	Activate(session, "another_tool")

	require.True(t, IsActivated(session, first[0]), "a refreshed tool survives")
	require.False(t, IsActivated(session, first[1]), "the next-oldest goes away")
}

func TestClearSessionDropsActivations(t *testing.T) {
	const session = "sess-clear"
	Activate(session, "a", "b")
	ClearSession(session)
	require.Empty(t, ActivatedTools(session))
	require.False(t, IsActivated(session, "a"))
}

func TestActivateWithoutSessionIsNoop(t *testing.T) {
	require.Empty(t, Activate("", "a"), "an anonymous caller must not create a global bucket")
	require.False(t, IsActivated("", "a"))
}

func TestExposedNames(t *testing.T) {
	seedTools(t, "exp", "hidden_tool", "loaded_tool")
	cfg := mcpStore(map[string]config.MCPConfig{"exp": {Type: config.MCPStdio}}, nil)

	const session = "sess-exposed"
	defer ClearSession(session)

	candidates := []string{"bash", "mcp_exp_hidden_tool", "mcp_exp_loaded_tool", "write"}

	exposed, ok := ExposedNames(cfg, session, candidates)
	require.True(t, ok)
	require.Equal(t, []string{"bash", "write"}, exposed)

	Activate(session, "mcp_exp_loaded_tool")
	exposed, ok = ExposedNames(cfg, session, candidates)
	require.True(t, ok)
	require.Equal(t, []string{"bash", "mcp_exp_loaded_tool", "write"}, exposed)

	// Nothing to hide means the caller leaves ActiveTools unset.
	eager := mcpStore(map[string]config.MCPConfig{
		"exp": {Type: config.MCPStdio, Lazy: lazy(false)},
	}, nil)
	_, ok = ExposedNames(eager, session, candidates)
	require.False(t, ok)

	// A step with no MCP tools in it is still filtered normally.
	exposed, ok = ExposedNames(cfg, session, []string{"bash"})
	require.True(t, ok)
	require.Equal(t, []string{"bash"}, exposed)

	// If every candidate were hidden, filtering would leave an empty list —
	// and fantasy reads an empty ActiveTools as "everything is active", so
	// the caller has to be told not to set it.
	_, ok = ExposedNames(cfg, session, []string{"mcp_exp_hidden_tool"})
	require.False(t, ok)
}

func names(refs []ToolRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.Name)
	}
	return out
}

// TestLazyFlowWithLiveServer drives the whole path against a real in-memory
// MCP server, so the schema shapes come from the SDK rather than fixtures.
func TestLazyFlowWithLiveServer(t *testing.T) {
	const name = "live-lazy"
	t.Cleanup(func() {
		allTools.Del(name)
		states.Del(name)
	})

	sess, ctx := liveSession(t, "send_message")
	t.Cleanup(func() { _ = sess.Close() })

	cfg := mcpStore(map[string]config.MCPConfig{name: {Type: config.MCPStdio}}, nil)
	count, err := registerSessionTools(ctx, cfg, name, sess)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	const session = "live-session"
	defer ClearSession(session)

	toolName := ToolName(name, "send_message")
	require.Equal(t, map[string]struct{}{toolName: {}}, DeferredNames(cfg, session))
	require.Contains(t, PromptSections(cfg, session), "mcp_search")

	refs := Search(cfg, "send", 0)
	require.Equal(t, []string{toolName}, names(refs))

	Activate(session, names(refs)...)
	require.Nil(t, DeferredNames(cfg, session))
	require.NotContains(t, PromptSections(cfg, session), "hidden")

	// Another session in the same workspace stays narrow.
	require.Len(t, DeferredNames(cfg, "other-session"), 1)
}

func containsName(haystack []string, want string) bool {
	for _, name := range haystack {
		if name == want {
			return true
		}
	}
	return false
}

func TestPromptSectionsTeachesSearch(t *testing.T) {
	seedTools(t, "ps", "one", "two")
	cfg := mcpStore(map[string]config.MCPConfig{"ps": {Type: config.MCPStdio}}, nil)

	const session = "sess-prompt"
	defer ClearSession(session)

	block := PromptSections(cfg, session)
	require.Contains(t, block, "<available_mcp>")
	// The block is the model's only cue that hidden tools exist at all, so
	// it has to name the remedy.
	require.Contains(t, block, "mcp_search")
	require.Contains(t, block, "- ps: 2 tools hidden, use mcp_search to load them")
	require.NotContains(t, block, "<mcp-instructions>", "nothing is connected")

	// Once the session has loaded everything, the index has nothing to say.
	Activate(session, "mcp_ps_one", "mcp_ps_two")
	require.NotContains(t, PromptSections(cfg, session), "ps:")
}
