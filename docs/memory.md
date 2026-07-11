# Memory Architecture

Crush memory is a passive, local subsystem. It is separate from chat history,
summarization, review, and context compaction.

## Lifecycle

1. Recall selects a small set of relevant active memories before a normal turn.
2. The normal agent runs without memory text being persisted into chat history.
3. After a successful idle turn, the summary model receives a bounded user and
   assistant transcript with no tools.
4. Extracted observations enter SQLite as active or pending records according
   to policy and confidence.
5. Human-readable projections and a bounded `MEMORY.md` index are synchronized
   from SQLite. Manual edits reconcile back into the database.
6. Maintenance creates a verified backup before pruning old telemetry,
   tombstones, and session state.

Each session is independently eligible for recording. A session may be
`enabled`, `disabled`, or `polluted`. Recall remains independent, so excluding
a session from recording does not remove useful context from that session.

## Record Types

- `user`: stable role, goals, responsibilities, or expertise.
- `feedback`: explicit corrections and confirmed operating preferences.
- `project`: non-derivable coordination context scoped to a canonical Git root.
- `reference`: durable pointers to external information, not copied content.

Records also carry provenance, confidence, status, replacement links, recall
count, last-recalled time, and pin state. Secrets and facts derivable from the
workspace are rejected at the storage boundary.

## Source Comparison

The design adapts compatible ideas from the supplied Claude Code architecture
study and the current OpenAI Codex source without copying either runtime:

- Claude-inspired: four observation types, derivability filtering, Git-root
  project identity, index-plus-detail retrieval, transparent files, no-tool
  background extraction, and serialized consolidation/maintenance.
- Codex-inspired: separate generate/use controls, per-session recording mode,
  optional external-context pollution, provenance and usage tracking, no-op as
  the preferred low-signal outcome, and reinjection of critical project state
  after context compaction or overflow.

Crush deliberately keeps one SQLite database and one passive post-turn worker.
Pending records serve as the review boundary instead of introducing Codex's
larger rollout-claim and global consolidation-agent pipeline. This matches a
single-user local client while preserving a future migration path.

## Configuration

Memory settings live under `options.memory`:

```json
{
  "enabled": true,
  "recorder_enabled": true,
  "recall_enabled": true,
  "disable_on_external_context": false,
  "auto_approve_confidence": 0.88,
  "max_recall": 5,
  "max_index_entries": 80,
  "max_backups": 5
}
```

MCP servers default to externally sourced output. A trusted local server can
opt out with `"pollutes_memory": false`. This matters only when
`disable_on_external_context` is enabled.

The command palette's `Memory` dialog provides explicit remember, approve,
pin, forget, global recorder/recall toggles, per-session recording eligibility,
and maintenance. Database work is executed asynchronously outside the TUI
update path.

## Context Recovery

When a provider rejects a request for exceeding its context window, Crush
still uses a compact retry. The retry omits the long normal prompt, provider
prefix, skills catalog, MCP instructions, and most history, but retains a
bounded working-directory, platform, and context-file fragment. This preserves
project identity and instructions without recreating the original overflow.
