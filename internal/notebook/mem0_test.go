package notebook

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// mem0TestStore builds a ConfigStore rooted at workDir, isolated from
// the developer's real global config. No t.Parallel(): it sets env vars.
func mem0TestStore(t *testing.T, workDir string) *config.ConfigStore {
	t.Helper()
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))
	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)
	return store
}

// stubRunMCPTool swaps the MCP round-trip for a canned implementation.
// No t.Parallel(): it mutates a package global.
func stubRunMCPTool(t *testing.T, fn func(ctx context.Context, cfg *config.ConfigStore, name, toolName, input string) (mcp.ToolResult, error)) {
	t.Helper()
	orig := runMCPTool
	runMCPTool = fn
	t.Cleanup(func() { runMCPTool = orig })
}

// stubSearchCaps forces the search_memories schema capability check.
// No t.Parallel(): it mutates a package global.
func stubSearchCaps(t *testing.T, caps mem0SearchCaps) {
	t.Helper()
	orig := mem0SearchCapsFor
	mem0SearchCapsFor = func(string) mem0SearchCaps { return caps }
	t.Cleanup(func() { mem0SearchCapsFor = orig })
}

func TestSearchMem0_PartitionsByWorkingDir(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	cfgA := mem0TestStore(t, dirA)
	stubSearchCaps(t, mem0SearchCaps{limitArg: "limit"})

	// Metadata carries the normalized spelling — the same key
	// SyncEntries writes.
	payload := fmt.Sprintf(`{"results":[
		{"id":"1","memory":"dir A fact","metadata":{"working_dir":%q,"session_id":"sA"}},
		{"id":"2","memory":"dir B secret","metadata":{"working_dir":%q,"session_id":"sB"}},
		{"id":"3","memory":"legacy memory","metadata":{"session_id":"sOld"}},
		{"id":"4","memory":"malformed","metadata":"not-an-object"}
	]}`, canonicalizeWorkingDir(dirA), canonicalizeWorkingDir(dirB))

	var gotTool, gotInput string
	stubRunMCPTool(t, func(_ context.Context, _ *config.ConfigStore, name, toolName, input string) (mcp.ToolResult, error) {
		gotTool, gotInput = toolName, input
		return mcp.ToolResult{Type: "text", Content: payload}, nil
	})

	result, err := SearchMem0(context.Background(), cfgA, "mem0", "fact")
	require.NoError(t, err)
	require.Equal(t, "search_memories", gotTool)
	require.Contains(t, result, "dir A fact")
	require.NotContains(t, result, "dir B secret")
	require.NotContains(t, result, "legacy memory")
	require.NotContains(t, result, "malformed")

	// With a schema declaring a limit argument but no filters, the
	// request over-fetches so the client-side partition doesn't drop
	// same-project memories that didn't rank in a global top-10.
	var args map[string]any
	require.NoError(t, json.Unmarshal([]byte(gotInput), &args))
	require.Equal(t, mem0AgentID, args["agent_id"])
	require.Equal(t, float64(mem0SearchFetchK), args["limit"])
	_, hasTopK := args["top_k"]
	require.False(t, hasTopK, "the limit arg name comes from the server schema")
	_, hasFilters := args["filters"]
	require.False(t, hasFilters)
}

func TestSearchMem0_ServerSideFilter(t *testing.T) {
	workDir := t.TempDir()
	cfg := mem0TestStore(t, workDir)
	stubSearchCaps(t, mem0SearchCaps{filters: true, limitArg: "top_k"})

	var gotInput string
	stubRunMCPTool(t, func(_ context.Context, _ *config.ConfigStore, _, _, input string) (mcp.ToolResult, error) {
		gotInput = input
		// Not per-memory JSON — the server-side filter is trusted, so
		// the raw response is returned rather than dropped.
		return mcp.ToolResult{Type: "text", Content: "Found 2 memories:\n- a\n- b"}, nil
	})

	result, err := SearchMem0(context.Background(), cfg, "mem0", "fact")
	require.NoError(t, err)
	require.Contains(t, result, "Found 2 memories")

	var args map[string]any
	require.NoError(t, json.Unmarshal([]byte(gotInput), &args))
	require.Equal(t, float64(mem0SearchTopK), args["top_k"])
	require.Equal(t, map[string]any{
		"AND": []any{
			map[string]any{"agent_id": mem0AgentID},
			map[string]any{"metadata.working_dir": canonicalizeWorkingDir(workDir)},
		},
	}, args["filters"])
}

func TestSearchMem0_FilteredErrorRetriesUnfiltered(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	cfg := mem0TestStore(t, dirA)
	stubSearchCaps(t, mem0SearchCaps{filters: true, limitArg: "top_k"})

	var inputs []string
	stubRunMCPTool(t, func(_ context.Context, _ *config.ConfigStore, _, _, input string) (mcp.ToolResult, error) {
		inputs = append(inputs, input)
		if len(inputs) == 1 {
			// Server declares filters but rejects the grammar.
			return mcp.ToolResult{}, fmt.Errorf("unknown filter operator")
		}
		payload := fmt.Sprintf(`[{"memory":"dir A fact","metadata":{"working_dir":%q}},
			{"memory":"dir B secret","metadata":{"working_dir":%q}}]`,
			canonicalizeWorkingDir(dirA), canonicalizeWorkingDir(dirB))
		return mcp.ToolResult{Type: "text", Content: payload}, nil
	})

	result, err := SearchMem0(context.Background(), cfg, "mem0", "fact")
	require.NoError(t, err)
	require.Len(t, inputs, 2, "a failed filtered call must retry unfiltered")
	require.Contains(t, result, "dir A fact")
	require.NotContains(t, result, "dir B secret")

	var retryArgs map[string]any
	require.NoError(t, json.Unmarshal([]byte(inputs[1]), &retryArgs))
	_, hasFilters := retryArgs["filters"]
	require.False(t, hasFilters, "retry drops the rejected filters arg")
	require.Equal(t, float64(mem0SearchFetchK), retryArgs["top_k"])
}

func TestSearchMem0_UnparseableUnfilteredReturnsEmpty(t *testing.T) {
	cfg := mem0TestStore(t, t.TempDir())

	stubRunMCPTool(t, func(_ context.Context, _ *config.ConfigStore, _, _, _ string) (mcp.ToolResult, error) {
		// Plain-text response carrying a foreign memory; with no
		// server-side filter available the partition can't be proven.
		return mcp.ToolResult{Type: "text", Content: "Found memories: other repo's secret"}, nil
	})

	result, err := SearchMem0(context.Background(), cfg, "mem0", "anything")
	require.NoError(t, err)
	require.Equal(t, "", result, "unverifiable results must not leak cross-project memories")
}

func TestSearchMem0_NoLimitArgStillPartitions(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	cfg := mem0TestStore(t, dirA)
	// Server declares neither filters nor a limit argument — the
	// client's only leverage is post-filtering the default page.
	stubSearchCaps(t, mem0SearchCaps{})

	var gotInput string
	stubRunMCPTool(t, func(_ context.Context, _ *config.ConfigStore, _, _, input string) (mcp.ToolResult, error) {
		gotInput = input
		payload := fmt.Sprintf(`[{"memory":"dir A fact","metadata":{"working_dir":%q}},
			{"memory":"dir B secret","metadata":{"working_dir":%q}}]`,
			canonicalizeWorkingDir(dirA), canonicalizeWorkingDir(dirB))
		return mcp.ToolResult{Type: "text", Content: payload}, nil
	})

	result, err := SearchMem0(context.Background(), cfg, "mem0", "fact")
	require.NoError(t, err)
	require.Contains(t, result, "dir A fact")
	require.NotContains(t, result, "dir B secret")

	var args map[string]any
	require.NoError(t, json.Unmarshal([]byte(gotInput), &args))
	for _, key := range []string{"top_k", "limit", "k", "filters"} {
		_, present := args[key]
		require.False(t, present, "args must not send %q the server never declared", key)
	}
}

func TestSearchMem0_PropagatesToolError(t *testing.T) {
	cfg := mem0TestStore(t, t.TempDir())

	stubRunMCPTool(t, func(_ context.Context, _ *config.ConfigStore, _, _, _ string) (mcp.ToolResult, error) {
		return mcp.ToolResult{}, fmt.Errorf("server unreachable")
	})

	_, err := SearchMem0(context.Background(), cfg, "mem0", "anything")
	require.Error(t, err)
	require.Contains(t, err.Error(), "mem0 search failed")
}

func TestSyncEntries_WorkingDirMetadata(t *testing.T) {
	workDir := t.TempDir()
	cfg := mem0TestStore(t, workDir)

	var inputs []string
	stubRunMCPTool(t, func(_ context.Context, _ *config.ConfigStore, name, toolName, input string) (mcp.ToolResult, error) {
		require.Equal(t, "mem0", name)
		require.Equal(t, "add_memory", toolName)
		inputs = append(inputs, input)
		return mcp.ToolResult{Type: "text", Content: "ok"}, nil
	})

	m := NewMem0Sync(cfg, "mem0")
	m.SyncEntries(context.Background(), []Entry{
		{
			ID:          "e1",
			SessionID:   "session1",
			Title:       "Read auth.go",
			EntryText:   "read auth middleware",
			EventType:   EventFileRead,
			TurnNumber:  3,
			EventNumber: 1,
			Tags:        []string{"file:auth.go"},
		},
		{
			// Hydrated seed — must never re-sync or it would compound
			// one copy of the origin memory per hydrated session.
			ID:        "e2",
			SessionID: "session1",
			Title:     "Seed",
			EntryText: "hydrated from prior session",
			Tags:      []string{TagHydrated, "origin:session0"},
		},
	})

	require.Len(t, inputs, 1, "hydrated entries must not re-sync")
	var args map[string]any
	require.NoError(t, json.Unmarshal([]byte(inputs[0]), &args))
	require.Equal(t, mem0AgentID, args["agent_id"])
	metadata, ok := args["metadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, canonicalizeWorkingDir(workDir), metadata["working_dir"])
	require.Equal(t, "session1", metadata["session_id"])
	require.Equal(t, "session1", metadata["origin_session_id"])
}

func TestSyncEntries_SkipsWhenWorkingDirUnknown(t *testing.T) {
	cfg := mem0TestStore(t, "")

	called := false
	stubRunMCPTool(t, func(_ context.Context, _ *config.ConfigStore, _, _, _ string) (mcp.ToolResult, error) {
		called = true
		return mcp.ToolResult{Type: "text", Content: "ok"}, nil
	})

	NewMem0Sync(cfg, "mem0").SyncEntries(context.Background(), []Entry{{ID: "e1"}})
	require.False(t, called, "entries without a partition key must not sync")
}

func TestCanonicalizeWorkingDir(t *testing.T) {
	dir := t.TempDir()

	// A symlinked spelling resolves to the same partition.
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(dir, link))
	require.Equal(t, canonicalizeWorkingDir(dir), canonicalizeWorkingDir(link))

	// On case-insensitive filesystems a case-variant spelling aliases.
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		require.Equal(t, canonicalizeWorkingDir(dir), canonicalizeWorkingDir(strings.ToUpper(dir)))
		require.Equal(t, strings.ToLower(canonicalizeWorkingDir(dir)), canonicalizeWorkingDir(dir))
	}
}

func TestFilterMem0Results(t *testing.T) {
	const dirA = "/repo/a"

	tests := []struct {
		name    string
		content string
		want    []string // substrings expected in the output
		notWant []string
		ok      bool
	}{
		{
			name:    "bare array keeps same-dir only",
			content: `[{"memory":"a1","metadata":{"working_dir":"/repo/a"}},{"memory":"b1","metadata":{"working_dir":"/repo/b"}}]`,
			want:    []string{"a1"},
			notWant: []string{"b1"},
			ok:      true,
		},
		{
			name:    "results wrapper",
			content: `{"results":[{"memory":"a1","metadata":{"working_dir":"/repo/a"}}]}`,
			want:    []string{"a1"},
			ok:      true,
		},
		{
			name:    "memories wrapper",
			content: `{"memories":[{"memory":"a1","metadata":{"working_dir":"/repo/a"}}]}`,
			want:    []string{"a1"},
			ok:      true,
		},
		{
			name:    "single object with metadata",
			content: `{"memory":"a1","metadata":{"working_dir":"/repo/a"}}`,
			want:    []string{"a1"},
			ok:      true,
		},
		{
			name:    "missing working_dir excluded",
			content: `[{"memory":"a1","metadata":{"session_id":"s1"}}]`,
			notWant: []string{"a1"},
			ok:      true,
		},
		{
			name:    "missing metadata excluded",
			content: `[{"memory":"a1"}]`,
			notWant: []string{"a1"},
			ok:      true,
		},
		{
			name:    "non-string working_dir excluded",
			content: `[{"memory":"a1","metadata":{"working_dir":42}}]`,
			notWant: []string{"a1"},
			ok:      true,
		},
		{
			name:    "empty working_dir excluded",
			content: `[{"memory":"a1","metadata":{"working_dir":""}}]`,
			notWant: []string{"a1"},
			ok:      true,
		},
		{
			name:    "empty results parses",
			content: `{"results":[]}`,
			ok:      true,
		},
		{
			name:    "non-JSON is not ok",
			content: "Found memories: stuff",
			ok:      false,
		},
		{
			name:    "JSON without memory list is not ok",
			content: `{"detail":"nope"}`,
			ok:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, ok := filterMem0Results(tc.content, dirA, "mem0")
			require.Equal(t, tc.ok, ok)
			for _, w := range tc.want {
				require.Contains(t, out, w)
			}
			for _, nw := range tc.notWant {
				require.NotContains(t, out, nw)
			}
		})
	}

	// Foreign-only results keep nothing.
	out, ok := filterMem0Results(`[{"memory":"b1","metadata":{"working_dir":"/repo/b"}}]`, dirA, "mem0")
	require.True(t, ok)
	require.Equal(t, "", out)
}

func TestFilterMem0Results_SortsByScoreAndCaps(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[")
	for i := range mem0SearchTopK + 5 {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"memory":"m%d","score":%d,"metadata":{"working_dir":"/repo/a"}}`, i, i)
	}
	sb.WriteString(`,{"memory":"foreign","score":9999,"metadata":{"working_dir":"/repo/b"}}]`)

	out, ok := filterMem0Results(sb.String(), "/repo/a", "mem0")
	require.True(t, ok)

	var kept []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &kept))
	require.Len(t, kept, mem0SearchTopK, "results are capped at the model-facing top_k")
	// The high-scoring foreign memory was excluded before ranking.
	for _, m := range kept {
		require.NotEqual(t, "foreign", m["memory"])
	}
	// Survivors rank by score descending.
	for i := range len(kept) - 1 {
		require.GreaterOrEqual(t, mem0Score(kept[i]), mem0Score(kept[i+1]))
	}
	require.Equal(t, "m14", kept[0]["memory"], "highest-scoring same-dir memory ranks first")
}

func TestFilterMem0Results_EnvelopeEdgeCases(t *testing.T) {
	// A wrapper key holding neither a list nor a memory is a
	// malformed envelope, not a single memory carrying metadata.
	out, ok := filterMem0Results(`{"results":"oops","metadata":{"working_dir":"/repo/a"}}`, "/repo/a", "mem0")
	require.False(t, ok)
	require.Equal(t, "", out)

	// A wrapper key holding one memory object unwraps it.
	out, ok = filterMem0Results(`{"results":{"memory":"a1","metadata":{"working_dir":"/repo/a"}}}`, "/repo/a", "mem0")
	require.True(t, ok)
	require.Contains(t, out, "a1")
}

func TestRenderMem0Results_FitsBudget(t *testing.T) {
	// Many same-project memories, best first (the order
	// filterMem0Results produces): the output must stay within the
	// budget and remain valid JSON — items are dropped, never cut.
	var kept []map[string]any
	for i := range 500 {
		kept = append(kept, map[string]any{
			"memory": strings.Repeat("x", 500),
			"score":  float64(500 - i),
		})
	}
	out := renderMem0Results(kept)
	require.LessOrEqual(t, len(out), mem0SearchMaxChars)
	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &items))
	require.Less(t, len(items), len(kept))

	var scores []float64
	noted := false
	for _, m := range items {
		if _, ok := m["note"]; ok {
			noted = true
			continue
		}
		s, ok := m["score"].(float64)
		require.True(t, ok)
		scores = append(scores, s)
	}
	require.True(t, noted, "dropped memories should leave a note element")
	require.Equal(t, float64(500), scores[0], "best-ranked memories survive")
}

func TestRenderMem0Results_SingleOversizedFallsBack(t *testing.T) {
	kept := []map[string]any{{"memory": strings.Repeat("x", mem0SearchMaxChars*2)}}
	out := renderMem0Results(kept)
	require.LessOrEqual(t, len(out), mem0SearchMaxChars+200)
	require.Contains(t, out, "truncated")
}

func TestSchemaHasProperty(t *testing.T) {
	require.True(t, schemaHasProperty(map[string]any{
		"type":       "object",
		"properties": map[string]any{"query": map[string]any{}, "filters": map[string]any{}},
	}, "filters"))
	require.False(t, schemaHasProperty(map[string]any{
		"type":       "object",
		"properties": map[string]any{"query": map[string]any{}},
	}, "filters"))
	require.True(t, schemaHasProperty(json.RawMessage(`{"properties":{"filters":{}}}`), "filters"))
	require.False(t, schemaHasProperty(json.RawMessage(`{broken`), "filters"))
	// Marshalable schema structs (in-process tool sources) work too.
	require.True(t, schemaHasProperty(struct {
		Properties map[string]any `json:"properties"`
	}{map[string]any{"filters": map[string]any{}}}, "filters"))
	require.False(t, schemaHasProperty(nil, "filters"))
	require.False(t, schemaHasProperty("not a schema", "filters"))
}

func TestMem0WorkingDirFilter(t *testing.T) {
	f := mem0WorkingDirFilter("/repo/a")
	and, ok := f["AND"].([]any)
	require.True(t, ok)
	require.Len(t, and, 2)
	require.Equal(t, map[string]any{"agent_id": mem0AgentID}, and[0])
	require.Equal(t, map[string]any{"metadata.working_dir": "/repo/a"}, and[1])
}
