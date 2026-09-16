package notebook

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
)

const (
	// mem0AgentID namespaces all crush-written memories inside the
	// mem0 store. Partitioning by project happens on the
	// working_dir metadata key, not the agent id.
	mem0AgentID = "crush"
	// mem0WorkingDirKey is the metadata key carrying the normalized
	// working directory path. It partitions memories per project so
	// a cross-session search in one repo can never surface another
	// repo's memories.
	mem0WorkingDirKey = "working_dir"
	// TagHydrated marks notebook entries seeded from mem0 during
	// session hydration. They must never re-sync — re-syncing would
	// duplicate the memory they came from, once per hydrated session.
	TagHydrated = "hydrated"
)

const (
	// mem0SearchTopK is the number of memories returned to the model.
	mem0SearchTopK = 10
	// mem0SearchFetchK is the over-fetch size used when the server's
	// search_memories cannot apply the working_dir filter itself.
	// Post-filtering a semantic top-10 would produce false negatives
	// — same-project memories that exist but didn't rank inside a
	// global top-10 — so the partition filter runs over a wide fetch
	// and the survivors are ranked and capped client-side.
	mem0SearchFetchK = 100
	// mem0SearchMaxTokens is the maximum token count for mem0 search
	// results returned to the model. Results beyond this are
	// truncated to avoid prompt inflation.
	mem0SearchMaxTokens = 4000
	// mem0SearchMaxChars is the byte budget matching
	// mem0SearchMaxTokens at the 4-chars-per-token estimate.
	mem0SearchMaxChars = mem0SearchMaxTokens * 4
)

// runMCPTool delegates to mcp.RunTool. It is a package-level var so
// tests can stub the MCP round-trip.
var runMCPTool = mcp.RunTool

// mem0LimitWarned records which servers have already logged the
// no-limit-argument warning.
var mem0LimitWarned = csync.NewMap[string, struct{}]()

// Mem0Sync provides cross-session memory sync via an MCP server
// (typically mem0). It is optional and only active when
// NotebookSyncMem0 is enabled in config.
type Mem0Sync struct {
	cfg        *config.ConfigStore
	serverName string
}

// NewMem0Sync creates a mem0 sync helper.
func NewMem0Sync(cfg *config.ConfigStore, serverName string) *Mem0Sync {
	return &Mem0Sync{
		cfg:        cfg,
		serverName: serverName,
	}
}

// SyncEntries syncs notebook entries to mem0 by calling the MCP
// server's add_memory tool. Each entry is stored with its tags as
// metadata so cross-session search can filter by tag. Memories are
// partitioned by the normalized working_dir metadata key so a search
// in one project never returns another project's entries.
func (m *Mem0Sync) SyncEntries(ctx context.Context, entries []Entry) {
	if m == nil || m.cfg == nil || m.serverName == "" || len(entries) == 0 {
		return
	}
	raw := m.cfg.WorkingDir()
	if raw == "" {
		// Without a partition key the memory would be unreachable by
		// every future filtered search — writing it is dead data.
		slog.Warn("Mem0 sync skipped: working directory is unknown", "server", m.serverName)
		return
	}
	workDir := canonicalizeWorkingDir(raw)
	for _, entry := range entries {
		if slices.Contains(entry.Tags, TagHydrated) {
			// Seeds hydrated from mem0 carry their origin session as a
			// tag; re-syncing them would compound one copy per session.
			continue
		}
		text := entry.EntryTextFull
		if text == "" {
			text = entry.EntryText
		}
		metadata := map[string]any{
			"session_id": entry.SessionID,
			// Always equal to session_id today; kept distinct so
			// hydrated entries (when hydration lands) can carry their
			// true origin session rather than the seeded one.
			"origin_session_id": entry.SessionID,
			"working_dir":       workDir,
			"turn_number":       entry.TurnNumber,
			"event_number":      entry.EventNumber,
			"event_type":        entry.EventType,
			"tags":              entry.Tags,
			"compression":       entry.CompressionLevel,
		}
		args := map[string]any{
			"text":     fmt.Sprintf("## %s\n%s", entry.Title, text),
			"agent_id": mem0AgentID,
			"metadata": metadata,
			"infer":    false,
		}
		input, _ := json.Marshal(args)
		_, err := runMCPTool(ctx, m.cfg, m.serverName, "add_memory", string(input))
		if err != nil {
			slog.Warn("Failed to sync entry to mem0",
				"error", err,
				"server", m.serverName,
				"turn", entry.TurnNumber,
			)
		}
	}
}

// SearchMem0 searches cross-session memories via the MCP server's
// search_memories tool, restricted to memories written from the
// current working directory. When the tool's schema accepts a
// filters argument the partition is applied server-side — a filtered
// call that errors retries unfiltered, since a server may declare
// the argument yet reject the grammar — and otherwise a wide fetch
// is partitioned and ranked client-side. Results that can neither
// be filtered server-side nor verified against working_dir metadata
// are dropped — an empty result beats a wrong-project one. The
// result text is truncated to mem0SearchMaxTokens to avoid prompt
// inflation.
func SearchMem0(ctx context.Context, cfg *config.ConfigStore, serverName, query string) (string, error) {
	if cfg == nil || serverName == "" || strings.TrimSpace(query) == "" {
		return "", nil
	}
	raw := cfg.WorkingDir()
	if raw == "" {
		slog.Warn("Mem0 search skipped: working directory is unknown", "server", serverName)
		return "", nil
	}
	workDir := canonicalizeWorkingDir(raw)
	caps := mem0SearchCapsFor(serverName)
	serverFiltered := caps.filters
	args := map[string]any{
		"query":    query,
		"agent_id": mem0AgentID,
	}
	if caps.limitArg != "" {
		limit := mem0SearchFetchK
		if serverFiltered {
			limit = mem0SearchTopK
		}
		args[caps.limitArg] = limit
	} else if !serverFiltered {
		// Once per server per process: a no-limit server keeps
		// under-returning for every search, but the warning only
		// needs to fire once.
		if _, warned := mem0LimitWarned.Get(serverName); !warned {
			mem0LimitWarned.Set(serverName, struct{}{})
			slog.Warn("Mem0 search cannot request an over-fetch: the server accepts no known limit argument",
				"server", serverName)
		}
	}
	if serverFiltered {
		args["filters"] = mem0WorkingDirFilter(workDir)
	}
	input, _ := json.Marshal(args)
	result, err := runMCPTool(ctx, cfg, serverName, "search_memories", string(input))
	if err != nil && serverFiltered {
		slog.Warn("Mem0 filtered search failed; retrying without server-side filter",
			"server", serverName,
			"error", err,
		)
		serverFiltered = false
		delete(args, "filters")
		if caps.limitArg != "" {
			args[caps.limitArg] = mem0SearchFetchK
		}
		input, _ = json.Marshal(args)
		result, err = runMCPTool(ctx, cfg, serverName, "search_memories", string(input))
	}
	if err != nil {
		return "", fmt.Errorf("mem0 search failed: %w", err)
	}
	filtered, ok := filterMem0Results(result.Content, workDir, serverName)
	if !ok {
		if serverFiltered {
			// The response isn't per-memory JSON we can re-check, so
			// the partition rests entirely on the declared filters
			// contract — a server that accepts the arg but silently
			// ignores it would still leak cross-project memories.
			// Dropping instead would break servers that legitimately
			// filter-and-return-prose.
			return truncateTextToTokens(result.Content, mem0SearchMaxTokens), nil
		}
		slog.Warn("Mem0 search dropped: response is not parseable and no server-side filter was applied",
			"server", serverName)
		return "", nil
	}
	return truncateTextToTokens(filtered, mem0SearchMaxTokens), nil
}

// canonicalizeWorkingDir canonicalizes a working directory for use as
// the mem0 partition key: filepath.Abs anchors it, EvalSymlinks
// collapses symlinked spellings (macOS /var → /private/var), and on
// case-insensitive filesystems the key is case-folded — the Abs +
// case-fold half matches the agent's normalizedPath — so symlinked
// or case-variant spellings share one partition instead of
// fragmenting it.
func canonicalizeWorkingDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = filepath.Clean(dir)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		abs = strings.ToLower(abs)
	}
	return abs
}

// mem0WorkingDirFilter builds a mem0 filter expression matching
// memories written by crush from the given working directory.
func mem0WorkingDirFilter(workDir string) map[string]any {
	return map[string]any{
		"AND": []any{
			map[string]any{"agent_id": mem0AgentID},
			map[string]any{"metadata." + mem0WorkingDirKey: workDir},
		},
	}
}

// mem0SearchCaps describes which search_memories arguments a server
// declares in its input schema.
type mem0SearchCaps struct {
	// filters reports whether the tool accepts a filters argument,
	// enabling server-side partitioning.
	filters bool
	// limitArg is the result-count argument the server accepts —
	// top_k, limit, or k — or empty when it declares none.
	limitArg string
}

// mem0SearchCapsFor inspects the server's registered search_memories
// tool schema. The registry is empty until the server connects and
// lists its tools — getOrRenewClient only renews a dead session, so
// an unconnected server yields zero caps: the client-side partition
// path runs without a limit arg, post-filtering the server's default
// top-N. The partition guarantee holds (no cross-project results),
// but that first search can silently under-return — same-project
// memories beyond the server's default page are missed. A
// package-level var so tests can force either path.
var mem0SearchCapsFor = func(serverName string) mem0SearchCaps {
	for name, tools := range mcp.Tools() {
		if name != serverName {
			continue
		}
		for _, tool := range tools {
			if tool != nil && tool.Name == "search_memories" {
				return mem0SearchCaps{
					filters:  schemaHasProperty(tool.InputSchema, "filters"),
					limitArg: mem0LimitArg(tool.InputSchema),
				}
			}
		}
	}
	return mem0SearchCaps{}
}

// mem0LimitArg returns the result-count argument a search_memories
// schema declares — top_k, limit, or k — or "" when it accepts none.
// Server vocabularies differ (the official mem0-mcp uses limit).
func mem0LimitArg(schema any) string {
	for _, key := range []string{"top_k", "limit", "k"} {
		if schemaHasProperty(schema, key) {
			return key
		}
	}
	return ""
}

// schemaHasProperty reports whether a JSON Schema object declares
// prop in its properties. The SDK hands the client the server's
// schema as a map[string]any; raw payloads may carry it as
// json.RawMessage, and in-process tool sources may hand over a
// marshalable schema struct — all are handled.
func schemaHasProperty(schema any, prop string) bool {
	var m map[string]any
	switch s := schema.(type) {
	case map[string]any:
		m = s
	case json.RawMessage:
		if err := json.Unmarshal(s, &m); err != nil {
			return false
		}
	default:
		raw, err := json.Marshal(schema)
		if err != nil || json.Unmarshal(raw, &m) != nil {
			return false
		}
	}
	props, _ := m["properties"].(map[string]any)
	_, ok := props[prop]
	return ok
}

// filterMem0Results parses a search_memories response and keeps only
// memories whose metadata.working_dir equals workDir; anything else —
// foreign partitions, missing keys, malformed metadata — is excluded
// and logged. Survivors are ranked by relevance score and capped at
// mem0SearchTopK. The boolean reports whether the payload parsed into
// per-memory items at all: false means the caller cannot prove the
// results are same-project.
func filterMem0Results(content, workDir, serverName string) (string, bool) {
	var payload any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return "", false
	}
	items, ok := mem0ResultItems(payload)
	if !ok {
		return "", false
	}
	var kept []map[string]any
	foreign, malformed := 0, 0
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			malformed++
			continue
		}
		wd, ok := mem0ItemWorkingDir(m)
		switch {
		case !ok:
			malformed++
		case wd != workDir:
			foreign++
		default:
			kept = append(kept, m)
		}
	}
	if malformed > 0 {
		slog.Warn("Excluded mem0 memories with missing or malformed working_dir metadata",
			"server", serverName,
			"excluded", malformed,
			"kept", len(kept),
		)
	}
	if foreign > 0 {
		// Foreign drops are the expected case on every partitioned
		// search in a multi-project store — not a warning.
		slog.Debug("Excluded mem0 memories from other working directories",
			"server", serverName,
			"excluded", foreign,
			"kept", len(kept),
		)
	}
	// The server ranked by semantic relevance across every project;
	// after partitioning, re-rank the survivors and cap at the
	// model-facing top_k.
	slices.SortStableFunc(kept, func(a, b map[string]any) int {
		return cmp.Compare(mem0Score(b), mem0Score(a))
	})
	if len(kept) > mem0SearchTopK {
		kept = kept[:mem0SearchTopK]
	}
	if len(kept) == 0 {
		return "", true
	}
	return renderMem0Results(kept), true
}

// renderMem0Results marshals kept memories to JSON that fits the
// token budget, dropping the lowest-ranked items rather than cutting
// mid-token. When items were dropped, a trailing note element reports
// the omission if it fits. A single oversized memory falls back to
// plain truncation so it still surfaces something.
func renderMem0Results(kept []map[string]any) string {
	dropped := 0
	for len(kept) > 1 {
		out, err := json.Marshal(kept)
		if err != nil {
			return ""
		}
		if len(out) <= mem0SearchMaxChars {
			break
		}
		kept = kept[:len(kept)-1]
		dropped++
	}
	out, err := json.Marshal(kept)
	if err != nil {
		return ""
	}
	if len(out) > mem0SearchMaxChars {
		return truncateTextToTokens(string(out), mem0SearchMaxTokens)
	}
	if dropped == 0 {
		return string(out)
	}
	noted := append(slices.Clone(kept), map[string]any{
		"note": fmt.Sprintf("%d additional same-project memories omitted to fit the token budget", dropped),
	})
	if out2, err := json.Marshal(noted); err == nil && len(out2) <= mem0SearchMaxChars {
		return string(out2)
	}
	return string(out)
}

// mem0ResultItems extracts the list of memory items from a parsed
// search_memories payload. mem0 servers variously return a bare JSON
// array or an object wrapping one under results/memories/data/items;
// a lone object carrying metadata is treated as a single result.
func mem0ResultItems(payload any) ([]any, bool) {
	switch p := payload.(type) {
	case []any:
		return p, true
	case map[string]any:
		for _, key := range []string{"results", "memories", "data", "items"} {
			v, exists := p[key]
			if !exists {
				continue
			}
			switch t := v.(type) {
			case []any:
				return t, true
			case map[string]any:
				return []any{t}, true
			default:
				// A wrapper key holding neither a list nor a single
				// memory means the envelope is malformed — don't
				// mistake the envelope itself for a memory.
				return nil, false
			}
		}
		if _, ok := p["metadata"]; ok {
			return []any{p}, true
		}
	}
	return nil, false
}

// mem0ItemWorkingDir returns the metadata.working_dir of one memory
// item, reporting false when the metadata is absent or the key is
// missing, empty, or not a string.
func mem0ItemWorkingDir(item map[string]any) (string, bool) {
	meta, ok := item["metadata"].(map[string]any)
	if !ok {
		return "", false
	}
	wd, ok := meta[mem0WorkingDirKey].(string)
	return wd, ok && wd != ""
}

// mem0Score extracts a relevance score from a memory item for
// client-side ranking; absent or unparseable scores rank last.
func mem0Score(item map[string]any) float64 {
	for _, key := range []string{"score", "similarity", "relevance"} {
		switch v := item[key].(type) {
		case float64:
			return v
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return 0
}

// truncateTextToTokens truncates text to approximately maxTokens by
// using a rough 4-chars-per-token estimate.
func truncateTextToTokens(text string, maxTokens int) string {
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + "\n\n[Results truncated to stay within token budget]"
}
