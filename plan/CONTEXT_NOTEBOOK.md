# Context Notebook — Per-Event Summarization

## Problem

Crush sends the **full conversation history** on every API call. In a 20-turn
session, 70% of tokens are old tool results the model already processed. With
1M context window, a single session can consume 5-10M tokens. Real-world
usage showed 46.6M tokens consumed in one night on the Alibaba Token Plan
(71.1% of the 10,000 weekly Credit quota).

### Measured data from real sessions

Data from local SQLite DB (107 sessions, 1453 messages):

| Metric                          | Measured value |
|---------------------------------|----------------|
| Total sessions                  | 107            |
| Total messages                  | 1,453          |
| Stored bytes (parts)            | 34,765         |
| Estimated stored tokens (~4 ch) | ~8,700         |
| Sum of last-call prompt_tokens  | 1,161,857      |
| Sum of completion_tokens         | 17,430         |

Note: `prompt_tokens` in the DB is the **last API call's** token count per
session, not cumulative. The 46.6M Alibaba figure reflects cumulative tokens
across all turns across all sessions. The DB cannot directly reproduce the
46.6M number because it doesn't accumulate per-turn token counts.

### Token breakdown by content type (estimated from session data)

| Content type                | % of tokens | Needed on next turn? |
|-----------------------------|-------------|----------------------|
| Old tool results            | ~70% (est)  | No — already read    |
| Old assistant responses     | ~15% (est)  | Sometimes            |
| System prompt               | ~10% (est)  | Yes — always         |
| Current turn                | ~5% (est)   | Yes                  |

These percentages are estimates based on the structure of typical coding
agent sessions (file reads, bash output, grep results dominate). They should
be validated by instrumenting `preparePrompt` to log actual token counts per
content type on 2-3 real sessions before implementation.

### Why reducing context window to 200K is not the solution

Reducing the context window only triggers auto-summarize sooner — it does not
fix the root cause. The root cause is sending junk (old tool output) every
turn. We need to improve the workflow itself.

## Relationship to Existing Summarizer

Crush already has auto-summarization (`agent.go:1329 Summarize()`). This plan
**replaces** it, not coexists with it.

### Current auto-summarize behavior

- **Reactive**: fires only when `remaining <= threshold` (agent.go:1053)
  - Large context (>200K): triggers at 20K remaining
  - Small context (<200K): triggers at 20% remaining
- **Lossy**: replaces entire history with one summary message
- **Expensive**: uses the **large** model to generate the summary
- **All-or-nothing**: no incremental compaction

### Notebook replaces auto-summarize

| Aspect                | Current `Summarize()`        | Notebook                     |
|-----------------------|------------------------------|------------------------------|
| Trigger               | Reactive (near overflow)     | Proactive (per significant event) |
| Model                 | Large (expensive)            | Small (cheap)                |
| Granularity           | Entire history → 1 summary    | Per-event → precise units    |
| Detail preservation   | Lossy (one-shot compression)  | Rich (one entry per event)   |
| Retrieval             | None (summary is final)      | Recall tool retrieves full   |
| Cross-session memory  | None                         | Via mem0 tags (optional)     |

### Migration plan

1. When notebook is **enabled**: disable `shouldSummarize` logic entirely
   (agent.go:1053). The notebook's compaction (Step 8) handles overflow.
2. When notebook is **disabled**: existing `Summarize()` remains as fallback.
3. The `disableAutoSummarize` config option still works — it disables both.
4. `summary_message_id` in sessions table is reused for notebook's active
   range marker (tracks which turns are notebook vs raw).

### What gets removed/modified

| Code path                        | Action |
|----------------------------------|--------|
| `shouldSummarize` (agent.go:1053) | Guarded by `if !notebookEnabled` |
| `Summarize()` (agent.go:1329)    | Kept as fallback when notebook disabled |
| `summaryPrompt` template         | Kept for fallback path |
| `preparePrompt()` (agent.go:1527) | Modified to inject notebook + adaptive raw window |

## Solution: Per-Event Context Notebook

After each turn, classify every tool call and assistant response as
**significant** or **trivial**. Generate one notebook entry per significant
event — not per turn. This gives maximum recall precision: each entry is
about exactly one thing (one file, one command, one decision).

### Event classification

```go
func isSignificant(toolCall message.ToolCall) bool {
    switch toolCall.Name {
    case "view", "read":
        return toolCall.OutputSize > 1000 // >100 lines
    case "edit", "write", "multiedit":
        return true // always significant
    case "bash":
        return true // always significant
    case "grep", "glob", "ls":
        return false // usually trivial exploration
    default:
        return true
    }
}
```

### Event types and handling

| Event type | Significant? | Entry | Size |
|------------|-------------|-------|------|
| File read > 100 lines | Yes | Own entry with key code | 500-1000 tok |
| File read ≤ 100 lines | No | Grouped into exploration mini-entry | ~100 tok |
| File edit/write | Always | Own entry with code diff | 500-1000 tok |
| Bash command | Always | Own entry with result | 500-1000 tok |
| grep/glob/ls | No | Grouped into exploration mini-entry | ~100 tok |
| Assistant decision | If contains decision | Own entry tagged #decision | 300-500 tok |
| Assistant "done" response | No | Skip | 0 |
| No tools + < 200 tok response | No | Skip entirely | 0 |

### What gets sent to the LLM each turn

```
[system prompt]
[notebook entries for older turns]     ← compacted, per-event
[raw recent turns within budget]        ← full recent context (adaptive)
[new user message]
```

The raw window is **adaptive by token budget**, not a fixed turn count.
Default budget: 25K tokens. The boundary walks backwards from the latest
message, accumulating tokens, and stops when the budget is exceeded —
always at a safe turn boundary (no split tool-call sequences).

| Session type | Avg tokens/turn | Adaptive sends | Fixed-5 would send |
|---|---|---|---|
| Chat-only (no tools) | ~800 | ~30 turns (24K) | 5 turns (4K) — wasteful cutoff |
| Light coding (small reads) | ~3K | ~8 turns (24K) | 5 turns (15K) — fine |
| Heavy coding (big files) | ~10K | ~2 turns (20K) | 5 turns (50K) — blows budget |
| Mixed | ~5K | ~5 turns (25K) | 5 turns (25K) — same |

Token usage stays **flat** instead of growing to 1M.

### Why per-event, not per-turn or per-phase

| Approach | Entries for 15-tool turn | Recall precision | Detail |
|----------|-------------------------|------------------|--------|
| Per-turn | 1 entry (all 15 tools compressed) | Low — `recall("file:auth.go")` returns entry about 5 files | Loses code snippets |
| Per-phase | 3 entries (exploration/modification/verification) | Medium — `recall("file:auth.go")` returns exploration phase with 5 files | Some code snippets |
| **Per-event** | ~6-8 entries (one per significant tool) | **High — `recall("file:auth.go")` returns entry only about auth.go** | **Full code snippets per file** |

Per-event gives **maximum recall precision**: each entry is about exactly
one thing. Searching for `#file:auth.go` returns the entry about auth.go,
not a phase summary that mentions auth.go alongside 4 other files.

### Why rich summaries, not 1-2 line summaries

A 10K-100K structured summary is more valuable than 1M tokens of raw history
where 70% is junk. Rich summaries preserve:

- PR numbers and project names
- File paths with key code snippets
- Decisions and rationale
- Open questions
- Tags for retrieval (`#pr:847`, `#file:auth.go`)

1-2 line summaries lose critical detail. Rich notebook entries preserve
everything important while staying 10-100× smaller than raw history.

### Notebook entry size: hard cap with smart truncation

Each entry has a **hard cap of 1000 tokens** (per-event entries are smaller
than per-turn entries because they cover less). The generation code enforces:

```go
const maxNotebookEntryTokens = 1000

// After generation, if entry exceeds cap:
if entryTokens > maxNotebookEntryTokens {
    // Truncate from the bottom (keep tags + key facts + code)
    // Add: "[Entry truncated. Use recall tool for full details.]"
}
```

This ensures the "flat ~10K-100K" claim is **guaranteed**, not aspirational.
A 100-turn session with ~2 events/turn × 1000 tokens = 200K max notebook
size, which compaction (Step 8) then compresses to 100K.

### Notebook entry format (per-event)

**Significant file read:**
```
## Turn 5.1 — Read auth.go
- internal/middleware/auth.go (120 lines)
- Contains: AuthMiddleware(), wraps ValidateToken()
- Bug: line 67, missing nil check on claims after error
#file:auth.go #pr:847 #phase:exploration
```

**File edit:**
```
## Turn 5.4 — Edit auth.go
- internal/middleware/auth.go line 67
- Added: if claims == nil { return ErrInvalidClaims }
#file:auth.go #pr:847 #phase:modification
```

**Command execution:**
```
## Turn 5.5 — Run tests
- go test ./internal/middleware/ → PASS (3 tests, 0.4s)
- go build ./... → success
#pr:847 #phase:verification #status:fixed
```

**Trivial exploration (grouped):**
```
## Turn 5.3 — Trivial exploration
- grep "ValidateToken" → 8 results across 4 files
- glob "internal/**/*.go" → 127 files
#pr:847 #phase:exploration
```

**Decision:**
```
## Turn 5.6 — Decision
- Chose nil check over error wrapping (minimal change for PR scope)
- Deferred RefreshToken nil check to PR #850
#pr:847 #decision
```

Each entry is a **precisely retrievable unit**. `recall("file:auth.go")`
returns only the auth.go entry, not a phase summary mentioning 5 files.

### When notebook exceeds 100K tokens

Old notebook entries get further compressed — but only the oldest:

```
Events from turns 1-50:   super-compressed (tags + 1 sentence) → ~5K
Events from turns 50-100: medium-compressed (key facts)         → ~20K
Events from turns 100+:   full entries (capped at 1000 tok)     → ~75K
Total: ~100K (bounded, guaranteed)
```

Full entries are always retrievable via the `recall` tool.

## Retrieval Flow — How the Model Reads from the Notebook

The notebook is useless if the model can't retrieve from it. There are
**three retrieval mechanisms**, ordered by reliability:

### Mechanism 1 (primary): Recall tool (model-driven)

This is the **primary and most reliable** retrieval mechanism. The model
decides what it needs and calls `recall()` explicitly. No guessing, no
regex, no false positives.

```
Model: "I need to check what we decided about RefreshToken"
Model calls: recall("decision") or recall("RefreshToken")
Recall returns: "Turn 5.6 — Decision: Deferred RefreshToken nil check to PR #850"
Model: "OK, I'll handle that in PR #850"
```

**Why this is primary:**
- The model knows what it needs better than any regex
- No false positives (model asks for exactly what it wants)
- No false negatives (model can search by concept, not just file path)
- Handles "the login bug we discussed" — model calls recall("login bug")
- Handles "that flaky test" — model calls recall("flaky test")

**Implementation:**

```go
// recall retrieves full content from notebook entries that were compacted.
// Use when you need exact code, file contents, or command output from
// previous turns. Each entry covers one specific event.
func recall(query string) string {
    // 1. Parse query type:
    //    "file:auth.go"  → search by tag
    //    "turn:5"        → search by turn number
    //    "command"       → search by event_type
    //    "auth.go"       → fuzzy match on entry text + tags
    //    "login bug"     → fuzzy match on entry text
    // 2. Return full (uncompacted) entries
}
```

**Cost:** one tool call (~500 tokens result). 16× cheaper than re-reading
a file (~8K tokens).

### Mechanism 2 (supporting): Notebook search (browse + query, one tool)

The model needs to know what entries exist before it can decide what to
recall. `notebook_search` serves both purposes: with no arguments it
lists all entries (titles + tags only); with a query it filters.

```
Model calls: notebook_search()
Returns:
  Turn 1.1 — Read config.go (#file:config.go #phase:exploration)
  Turn 1.2 — Edit config.go (#file:config.go #phase:modification)
  Turn 5.1 — Read auth.go (#file:auth.go #phase:exploration)
  Turn 5.2 — Read jwt.go (#file:jwt.go #phase:exploration)
  Turn 5.3 — Trivial exploration (#phase:exploration)
  Turn 5.4 — Edit auth.go (#file:auth.go #phase:modification)
  Turn 5.5 — Run tests (#phase:verification #status:fixed)
  Turn 5.6 — Decision (#pr:847 #decision)

Model: "I need jwt.go details"
Model calls: recall("file:jwt.go")

Model: "What decisions did we make?"
Model calls: notebook_search("decision")
Returns:
  Turn 5.6 — Decision (#pr:847 #decision)
  Turn 12.3 — Decision (#pr:850 #decision)
```

**Why one tool, not two:**
- `notebook_list()` and `search(query)` do the same thing — return
  matching entries. One takes no query, one does.
- Collapsing to one tool with an optional query reduces tool-surface
  bloat the model has to learn.
- The model doesn't need to decide "am I browsing or searching?" — it
  just calls `notebook_search` with or without a query.

**Implementation:**

```go
// notebook_search returns notebook entries for the current session.
// With no query: returns all entries (titles + tags only).
// With a query: returns entries matching the query (by tag, event_type,
// or entry text). Results are titles + tags only — use `recall` to get
// full content of any entry.
func notebookSearch(query string) string {
    entries := a.db.GetNotebookEntries(ctx, sessionID)
    if query != "" {
        entries = filterEntries(entries, query)
    }
    var sb strings.Builder
    for _, e := range entries {
        sb.WriteString(fmt.Sprintf("Turn %d.%d — %s %s\n",
            e.TurnNumber, e.EventNumber, e.Title, e.Tags))
    }
    return sb.String()
}
```

**Cost:** one tool call (~200 tokens for listing, ~300 for filtered).

### Mechanism 3 (experimental, opt-in): Auto-injection

**Status: experimental, best-effort, disabled by default.**

`preparePrompt` *may* scan the user's new message for explicit file path
references and upgrade matching compacted entries to full. This is a
convenience optimization, not a load-bearing mechanism. It will both
over-inject (incidental mentions like "compare auth.go to auth_test.go
from three different tasks 40 turns ago") and under-inject (misses "the
login bug we discussed," "that flaky test," function names without file
extensions).

**Strict limits to mitigate fragility:**
- Only matches **full file paths with path separators** (e.g.,
  `internal/middleware/auth.go`), not bare filenames (`auth.go`)
- Maximum **2 entries** auto-injected per turn
- Only upgrades entries from **compaction level > 0** (not already full)
- **Disabled by default** — must be explicitly enabled in config
- The model is instructed to use `recall` regardless — auto-injection is
  a bonus, not a dependency

```
User: "fix the bug in internal/middleware/auth.go we found earlier"

preparePrompt (if auto-injection enabled):
  1. Extract explicit file paths from user message:
     - "internal/middleware/auth.go" → tag #file:auth.go
  2. Search notebook for matching entries (max 2)
  3. Replace compacted entries with full (uncompacted) versions

Context sent to model:
  [system prompt]
  [notebook entries (compacted for old turns)]
  [AUTO-INJECTED: full entry for auth.go from turn 5]  ← upgraded, max 2
  [raw turns N-4..N]
  [new user message]
```

**What gets detected (conservative):**

| Pattern in user message | Tag extracted | Example |
|-------------------------|---------------|---------|
| Full file path with separator | `#file:{basename}` | "internal/middleware/auth.go" → `#file:auth.go` |
| Explicit PR reference | `#pr:{number}` | "PR #847" → `#pr:847` |

**What does NOT get detected (by design):**
- Bare filenames: "auth.go" (too many false positives)
- Conceptual references: "the login bug" (use recall instead)
- Function names: "ValidateToken" (use recall instead)
- Vague references: "that flaky test" (use recall instead)

**Implementation:**

```go
// autoInjectEnabled is a config flag, default false.
// extractExplicitFilePaths only matches paths with separators to
// reduce false positives. Bare filenames like "auth.go" are NOT matched.
func extractExplicitFilePaths(msg string) []string {
    var refs []string
    // Only match paths with at least one separator: "internal/middleware/auth.go"
    // Does NOT match bare "auth.go" — too many false positives
    if matches := fullPathRegex.FindAllString(msg, -1); matches != nil {
        for _, m := range matches {
            refs = append(refs, "file:"+filepath.Base(m))
        }
    }
    return refs
}

func (a *sessionAgent) maybeAutoInject(
    notebookMsg fantasy.Message,
    userMsg string,
    sessionID string,
) fantasy.Message {
    if !a.autoInjectEnabled {
        return notebookMsg // no-op when disabled
    }
    refs := extractExplicitFilePaths(userMsg)
    if len(refs) == 0 {
        return notebookMsg
    }
    fullEntries := a.db.SearchNotebookByTags(ctx, sessionID, refs)
    // Max 2 entries to avoid context inflation
    if len(fullEntries) > 2 {
        fullEntries = fullEntries[:2]
    }
    for _, entry := range fullEntries {
        if entry.CompressionLevel > 0 {
            notebookMsg = replaceEntryInMessage(notebookMsg, entry)
        }
    }
    return notebookMsg
}
```

**Cost:** zero tool calls, zero extra latency. Just a SQLite query during
`preparePrompt` (which already runs). But only runs when explicitly enabled.

### Full retrieval flow

```
┌─────────────────────────────────────────────────────────────┐
│ User sends: "fix the bug in internal/middleware/auth.go"    │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│ preparePrompt()                                              │
│                                                              │
│ 1. Build context:                                            │
│    [system prompt with notebook instructions]                │
│    [notebook entries (compacted, all old turns)]             │
│    [OPTIONAL auto-inject: if enabled + full path detected]    │
│    [raw recent turns within 25K budget]                     │
│    [new user message]                                        │
│                                                              │
│ Note: auto-injection is best-effort and disabled by default.│
│ The model is expected to use recall/notebook_list tools.     │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│ Model receives context                                       │
│                                                              │
│ - Sees compacted notebook for old turns                     │
│ - Sees recent raw turns within 25K token budget              │
│ - Sees system prompt: "Use recall before re-reading files"   │
│ - May see auto-injected entries (if enabled + matched)      │
│                                                              │
│ Model decides:                                               │
│ - "I need auth.go details from earlier"                     │
│ - Calls: recall("file:auth.go") → 500 tokens                │
│ - "I also need to check what we decided"                     │
│ - Calls: recall("decision") → 500 tokens                    │
│ - Now has enough context to work                             │
│                                                              │
│ If model doesn't know what's available:                      │
│ - Calls: notebook_list() → 200 tokens                        │
│ - Sees all entry titles + tags                               │
│ - Then calls recall for specific entries                     │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│ Model works on the task                                      │
│ - Uses recalled context                                       │
│ - Does NOT re-read files that have notebook entries          │
│ - Saves tokens by using recall instead of view tool          │
└──────────────────────────────────────────────────────────────┘
```

### System prompt update

The system prompt must instruct the model about the notebook:

```
You have a notebook of past events in this session. Each entry covers
one specific event (file read, file edit, command, decision).

- Use the `recall` tool to retrieve full details of any event.
  You can search by file name, tag, turn number, event type, or concept.
- Use `notebook_search` to browse all available entries or filter by
  a query (tag, event type, or text).
- Do NOT re-read files with notebook entries — use `recall` first.
  It is 16× cheaper than re-reading the file.
- If recall doesn't have what you need, then use `view` to re-read.
```

Note: the system prompt no longer mentions auto-injection as a
guaranteed mechanism. The model is instructed to use `recall` as the
primary retrieval path.

### Token cost comparison: retrieval methods

| Method | Tool calls | Tokens | Latency | Reliability |
|--------|-----------|--------|---------|-------------|
| `recall("file:auth.go")` | 1 | ~500 (result) | ~50ms (SQLite) | High — model asks for what it needs |
| `notebook_search()` | 1 | ~200 (listing) | ~50ms (SQLite) | High — complete listing |
| `notebook_search("decision")` | 1 | ~300 (filtered) | ~50ms (SQLite) | High — filtered listing |
| Auto-injection (if enabled) | 0 | ~500 (in context) | 0ms | Low — regex-based, best-effort |
| `view auth.go` (re-read) | 1 | ~8K (full file) | ~500ms | High but expensive |
| Without notebook (current) | 0 | ~8K (already in history) | 0ms but grows | — |

### Retrieval tools summary

| Tool | Purpose | When | Cost | Reliability |
|------|---------|------|------|------------|
| `recall(query)` | Retrieve full entry by tag/file/turn/concept | Model needs old context | ~500 tok | Primary — model-driven |
| `notebook_search(query?)` | Browse all or filter by query | Model doesn't know what's available | ~200-300 tok | Supporting — complete listing |
| Auto-injection | Upgrade compacted entries to full | User mentions explicit file path | Free | Experimental — opt-in, best-effort |
| mem0 sync | Cross-session search via mem0 | Need context from previous sessions | ~500 tok | Optional — requires mem0 |

## Turn Boundaries and Tool-Call Pairing

This is the trickiest part. Fantasy's message history has strict pairing
requirements: an assistant message with `tool_calls` must be immediately
followed by matching tool result messages. Cutting mid-sequence produces
an invalid message list that some providers reject.

### How Crush defines a "turn"

A turn is **not** a single message — it's a complete step sequence:

```
Turn = [user message] → [assistant + tool_calls] → [tool results] → ... → [assistant final response]
```

A turn may span multiple tool round-trips within one user request.

### Safe truncation rules

1. **Never split an assistant+tool_call from its tool_result**. The recency
   window boundary must fall between turns, not within a tool-call sequence.
2. A turn boundary is defined as: after an assistant message with
   `FinishReasonEndTurn` or `FinishReasonStop` (no pending tool calls).
3. If the recency window would split a tool-call sequence, extend the window
   to include the complete sequence.

### Implementation

```go
// findTurnBoundaryByTokenBudget walks backwards from the latest message,
// accumulating tokens until the budget is exceeded. The boundary always
// falls at a safe turn end (assistant message with no pending tool calls).
//
// This is adaptive: chat-only sessions get more turns (small tokens each),
// heavy-coding sessions get fewer turns (large tokens each). The token
// budget stays bounded regardless of session type.
func findTurnBoundaryByTokenBudget(
    msgs []message.Message,
    tokenBudget int,
    estimateTokens func([]message.Message) int,
) int {
    accumulated := 0
    for i := len(msgs) - 1; i >= 0; i-- {
        // Accumulate this message's tokens
        accumulated += estimateTokens(msgs[i:i+1])

        // Check if we've exceeded budget — if so, boundary is at i+1
        // (this message becomes the oldest raw message)
        if accumulated > tokenBudget {
            // Walk forward to find the next safe turn boundary
            // (don't split a tool-call sequence)
            return findNextSafeBoundary(msgs, i+1)
        }

        // If this is a complete turn end, we could stop here
        // but we keep going to maximize context within budget
    }
    return 0 // Entire history fits within budget — send all raw
}

// findNextSafeBoundary finds the earliest index >= start that is a safe
// turn boundary (after an assistant message with no pending tool calls).
// This ensures we never split a tool-call sequence even when the token
// budget would force a cut mid-sequence.
func findNextSafeBoundary(msgs []message.Message, start int) int {
    for i := start; i < len(msgs); i++ {
        if msgs[i].Role == message.Assistant && len(msgs[i].ToolCalls()) == 0 {
            return i + 1 // Start raw window after this complete turn
        }
    }
    return start // No safe boundary found — include from start
}
```

This reuses the same `ToolCalls()` / `ToolResults()` pattern that
`preparePrompt` already uses for orphan detection (agent.go:1550-1565).

**Why adaptive, not fixed turn count:**

| Session type | Avg tokens/turn | Adaptive (25K budget) | Fixed 5 turns |
|---|---|---|---|
| Chat-only (no tools) | ~800 | ~30 turns | 5 turns (wasteful) |
| Light coding | ~3K | ~8 turns | 5 turns (fine) |
| Heavy coding (big files) | ~10K | ~2 turns | 5 turns (50K, blows budget) |
| Mixed | ~5K | ~5 turns | 5 turns (same) |

Adaptive always stays within budget while maximizing recent context.

### Edge cases

| Case | Handling |
|------|----------|
| Entire history fits in token budget | Send all raw, no notebook injection |
| Budget exceeded mid-turn | Extend to next safe boundary (don't split) |
| Turn has 10+ tool calls | Include all as raw (don't split) |
| Cancelled mid-tool-call | `FinishReasonCanceled` counts as turn end |
| Sub-agent sessions | Same rules apply (sub-agent has own notebook) |
| Trivial turn (no tools, short response) | No notebook entries generated |
| Chat-only session (small turns) | Adaptive sends more turns (up to budget) |
| Heavy-coding session (large turns) | Adaptive sends fewer turns (within budget) |

## Notebook Generation Cost Analysis

Each significant event makes an **extra LLM call** to the small model — but
trivial events are skipped, and multiple events in one turn can be batched
into a single small model call.

### Per-event notebook generation cost

| Component | Tokens (est) |
|-----------|-------------|
| Input: event's tool call + result | ~2K-20K (varies by event) |
| Input: notebook template | ~300 |
| Output: notebook entry | ~300-1000 (capped) |
| **Total per event** | **~3K-21K** |

### Batching: one LLM call per turn, multiple entries out

Instead of one LLM call per event, batch all events in a turn into one
small model call:

```go
func (a *sessionAgent) generateNotebookEntries(
    ctx context.Context,
    sessionID string,
    turnNumber int,
    turnMessages []message.Message,
) error {
    // 1. Classify all tool calls in the turn
    events := classifyEvents(turnMessages)

    // 2. Skip if no significant events
    if len(events) == 0 {
        return nil
    }

    // 3. One small model call → multiple entries
    entries := a.callSmallModel(ctx, events, notebookTemplate)

    // 4. Store all entries + tags
    for _, entry := range entries {
        a.storeNotebookEntry(ctx, sessionID, turnNumber, entry)
    }
}
```

This means a turn with 8 significant events still costs **one** small model
call, not eight.

### Cost comparison: with vs without notebook

| Scenario | Without notebook (current) | With notebook |
|----------|---------------------------|---------------|
| 20-turn session | ~200K × 20 = **~4M** | Main: ~41K × 20 = 820K + Notebook: ~15K × 20 = 300K = **~1.1M** |
| 50-turn session | ~500K × 50 = **~25M** | Main: ~61K × 50 = 3.05M + Notebook: ~15K × 50 = 750K = **~3.8M** |
| 100-turn session | ~1M × 100 = **~100M** | Main: ~106K × 100 = 10.6M + Notebook: ~15K × 100 = 1.5M = **~12.1M** |

Main per-call = notebook (10-75K, compacted) + raw (25K budget) + system (~5K) + new msg (~1K).
Notebook gen = ~15K tokens/turn on small model.

Notebook generation uses the **small model** (deepseek-v4-flash, 0.17×
Credit multiplier) while main conversation uses the **large model**
(qwen3.8-max-preview, 0.1× with 10× promo). So notebook cost is:

```
Notebook Credits = 15K tokens × 100 turns × 0.17 ÷ 1000 = 255 Credits
Main Credits = 75K tokens × 100 turns × 0.1 ÷ 1000 = 750 Credits
Total = ~1,005 Credits (10.1% of weekly quota)
```

vs current:

```
Current Credits = 1M tokens × 100 turns × 0.1 ÷ 1000 = 10,000 Credits (100%)
```

**Net savings: ~90%** even accounting for notebook generation cost.

### Why per-event costs less than per-turn

| Approach | 100-turn session | Entries | Total notebook tokens | Gen cost |
|----------|-----------------|---------|----------------------|----------|
| Per-turn | 100 entries × 2000 tok | 200K | ~340 Credits |
| Per-phase | ~150 entries × 1000 tok | 150K | ~255 Credits |
| **Per-event** | ~200 entries × ~500 tok | **100K** | **~170 Credits** |

Per-event has **more entries but smaller entries** — total notebook size is
actually **smaller** because trivial tool calls (grep, glob, ls) get grouped
into tiny mini-entries instead of being part of a 2000-token per-turn entry.

### Latency: async generation

Notebook generation runs **asynchronously** after the model responds:

```go
// After turn completes, start notebook generation in background
go func() {
    if err := a.generateNotebookEntries(ctx, sessionID, turnNumber, turnMessages); err != nil {
        slog.Error("Failed to generate notebook entries", "error", err)
    }
}()
```

The user sees the response immediately. The notebook entries are ready
before the next turn starts (typically 5-30 seconds of user think time).

**Risk**: If the user sends the next message before notebook generation
completes, the notebook will be one turn stale. Mitigation: `preparePrompt`
checks if notebook is current; if not, it includes the last unprocessed
turn as raw (temporarily increasing the raw budget).

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  Turn N completes                                             │
│                                                               │
│  1. Classify each tool call: significant or trivial           │
│  2. Skip if no significant events (trivial turn)              │
│  3. Async: small model generates entries (batched, one call) │
│     - One entry per significant event (≤1000 tok each)        │
│     - One mini-entry for grouped trivial events               │
│  4. Full turn content stored in SQLite (for recall)          │
│  5. Entries stored in SQLite + tagged in join table           │
│  6. (Optional) Tags synced to mem0 MCP                       │
│                                                               │
│  Next turn (N+1):                                             │
│                                                               │
│  preparePrompt() builds:                                      │
│  [system prompt]                                              │
│  [notebook entries for older turns]  ← compacted              │
│  [raw recent turns within 25K budget] ← full (tool-safe)      │
│  [new user message]                                           │
│                                                               │
│  Raw budget: 25K tokens (adaptive, not fixed turn count)      │
│  Total: ~10K-100K (bounded, does not grow with session)       │
└──────────────────────────────────────────────────────────────┘
```

### Before vs After (estimated, needs validation)

| Turns | Current (raw history, est) | With notebook (est) | Notebook gen cost | Net total |
|-------|----------------------------|----------------------|-------------------|-----------|
| 5     | ~40K/call × 5 = 200K       | ~31K/call × 5 = 155K | ~15K × 5 = 75K   | ~230K     |
| 20    | ~200K/call × 20 = 4M       | ~41K/call × 20 = 820K | ~15K × 20 = 300K | ~1.1M     |
| 50    | ~500K/call × 50 = 25M      | ~61K/call × 50 = 3.05M | ~15K × 50 = 750K | ~3.8M    |
| 100   | ~1M/call × 100 = 100M     | ~106K/call × 100 = 10.6M | ~15K × 100 = 1.5M | ~12.1M  |

Main per-call = notebook (grows with session, capped at 100K) + raw (25K budget) + system (~5K) + new msg (~1K).
Early turns: notebook is small, most of the 25K budget is used for raw.
Later turns: notebook grows, raw stays at 25K, total capped by compaction.

These are **estimates** based on assumed ~10K tokens/turn average. They must
be validated by instrumenting `preparePrompt` on 2-3 real sessions before
implementation. The "Current" column assumes linear growth at ~10K/turn.

## Implementation Steps

### Step 0: Validate with real data (pre-implementation)

Before writing any code, instrument `preparePrompt` to log:

```go
slog.Info("preparePrompt token estimate",
    "session_id", sessionID,
    "message_count", len(msgs),
    "estimated_tokens", estimateTokens(msgs),
    "tool_result_bytes", toolResultBytes(msgs),
    "assistant_bytes", assistantBytes(msgs),
)
```

Run 2-3 real Crush sessions, then analyze the logs to validate:
- Average tokens per turn
- % of tokens from old tool results
- Actual growth curve
- Whether the notebook savings estimates hold

### Step 1: Database migration — notebook tables

**File**: `internal/db/migrations/20260909000000_add_notebook_table.sql`

Uses a **join table** for tags (not a JSON column) for proper indexing:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS notebook_entries (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    turn_number INTEGER NOT NULL,
    event_number INTEGER NOT NULL DEFAULT 0,
    event_type TEXT NOT NULL DEFAULT 'general',
    entry_text TEXT NOT NULL,
    token_count INTEGER NOT NULL DEFAULT 0,
    compression_level INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);
CREATE INDEX idx_notebook_session ON notebook_entries (session_id);
CREATE INDEX idx_notebook_turn ON notebook_entries (session_id, turn_number);
CREATE INDEX idx_notebook_event_type ON notebook_entries (session_id, event_type);

CREATE TABLE IF NOT EXISTS notebook_tags (
    entry_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES notebook_entries (id) ON DELETE CASCADE,
    PRIMARY KEY (entry_id, tag)
);
CREATE INDEX idx_notebook_tag ON notebook_tags (tag);

-- +goose Down
DROP TABLE IF EXISTS notebook_tags;
DROP TABLE IF EXISTS notebook_entries;
```

### Step 2: SQL queries — notebook CRUD

**File**: `internal/db/sql/notebook.sql`

Queries for:
- Insert notebook entry
- Insert notebook tags (batch)
- Get entries by session (ordered by turn, then event_number)
- Search by tag (join query)
- Search by event_type
- Delete by session
- Get total token count for a session
- Update entry compression level

### Step 3: Generated DB code — notebook.go

**File**: `internal/db/notebook.go`

Generated by sqlc from `notebook.sql`. Functions:
- `CreateNotebookEntry(ctx, params)`
- `CreateNotebookTags(ctx, params)`
- `GetNotebookEntries(ctx, sessionID)`
- `SearchNotebookByTag(ctx, sessionID, tag)`
- `SearchNotebookByEventType(ctx, sessionID, eventType)`
- `GetNotebookTokenCount(ctx, sessionID)`
- `UpdateNotebookCompression(ctx, entryID, level)`

### Step 4: Event classification and notebook entry generation

**File**: `internal/agent/notebook.go`

After each turn completes, classify events and generate notebook entries
**asynchronously** using the **small model** (cheap):

```go
const maxNotebookEntryTokens = 1000

// Event types.
const (
    EventFileRead      = "file_read"
    EventFileEdit      = "file_edit"
    EventCommand       = "command"
    EventDecision      = "decision"
    EventExploration   = "exploration" // grouped trivial tools
)

// isSignificant returns true if a tool call warrants its own notebook entry.
func isSignificant(toolCall message.ToolCall) bool {
    switch toolCall.Name {
    case "view", "read":
        return toolCall.OutputSize > 1000 // >100 lines
    case "edit", "write", "multiedit":
        return true
    case "bash":
        return true
    case "grep", "glob", "ls":
        return false
    default:
        return true
    }
}

func (a *sessionAgent) generateNotebookEntries(
    ctx context.Context,
    sessionID string,
    turnNumber int,
    turnMessages []message.Message,
) error {
    // 1. Classify all tool calls in the turn
    significant, trivial := classifyEvents(turnMessages)

    // 2. Skip if no significant events and no trivial events
    if len(significant) == 0 && len(trivial) == 0 {
        // Check if assistant response contains a decision
        if hasDecision(turnMessages) {
            significant = append(significant, decisionEvent(turnMessages))
        } else {
            return nil // Trivial turn — skip entirely
        }
    }

    // 3. Group trivial events into one mini-entry
    if len(trivial) > 0 {
        entry := buildTrivialExplorationEntry(trivial)
        a.storeNotebookEntry(ctx, sessionID, turnNumber, 0, EventExploration, entry)
    }

    // 4. One small model call → multiple entries (batched)
    entries := a.callSmallModel(ctx, significant, notebookTemplate)

    // 5. Store all entries + tags
    for i, entry := range entries {
        a.storeNotebookEntry(ctx, sessionID, turnNumber, i+1, entry.Type, entry)
    }

    // 6. Optionally sync tags to mem0
    if a.notebookSyncMem0 {
        a.syncToMem0(entries)
    }
}
```

**File**: `internal/agent/templates/notebook_entry.md.tpl`

The template for generating rich per-event notebook entries:

```
You are writing structured notebook entries for a coding agent's turn.
Each entry covers ONE significant event. Be precise and concise.
Maximum 1000 tokens per entry.

For each event, write:

## Turn {N}.{event_number} — {event_type}

{For file reads:}
- {file path} ({line count} lines)
- Contains: {key functions}
- Key: {important code snippet, max 10 lines}
- {notable findings: bugs, patterns, locations}

{For file edits:}
- {file path} line {line number}
- {what changed}
- {code snippet of the change}

{For commands:}
- {command} → {result: PASS/FAIL}
- {key output summary}

{For decisions:}
- {what was decided}
- {why}
- {what was deferred}

### Tags
[#pr:X #project:Y #file:Z #phase:exploration|modification|verification #type:{event_type}]

Write as if briefing a teammate. Include exact file paths, line numbers,
PR numbers. Be precise — each entry is about ONE thing. Stay under 1000
tokens per entry. Prioritize: tags > key facts > code > context.
```

### Step 5: Modify preparePrompt — notebook + recent turns

**File**: `internal/agent/agent.go` — modify `preparePrompt()`

Current behavior: sends ALL messages as raw history.

New behavior:

```go
func (a *sessionAgent) preparePrompt(
    msgs []message.Message,
    supportsImages bool,
    ...
) {
    if !a.notebookEnabled {
        // Fallback: existing behavior (full raw history)
        return existingPreparePrompt(msgs, supportsImages, ...)
    }

    // 1. Find safe turn boundary by token budget (adaptive, not fixed turns)
    //    Walks backwards, accumulates tokens, stops at 25K budget.
    //    Always falls at a safe turn end (no split tool-call sequences).
    boundary := findTurnBoundaryByTokenBudget(msgs, a.rawTokenBudget, estimateTokens)

    // 2. Build notebook message from DB state (always rebuild, never mutate)
    //    This avoids string-level surgery on a rendered markdown blob.
    //    The cost is one SQLite query per preparePrompt call — negligible.
    notebookMsg := a.buildNotebookMessage(ctx, sessionID, userMsg)

    // 3. Recent messages → raw (tool-call safe)
    recentMsgs := msgs[boundary:]

    // 4. Build: [system] + [notebook] + [recent raw] + [new msg]
    history = append(history, notebookMsg)
    history = append(history, recentMsgs...)
}

// buildNotebookMessage reconstructs the notebook system message from DB
// state every time. This avoids fragile string-level surgery on a
// rendered markdown blob — instead of find-and-replace, we just rebuild.
//
// If auto-injection is enabled and the user message contains explicit
// file paths, matching compacted entries are upgraded to full before
// rendering. This is done by setting a temporary compression override
// in the query, not by mutating the rendered string.
func (a *sessionAgent) buildNotebookMessage(
    ctx context.Context,
    sessionID string,
    userMsg string,
) fantasy.Message {
    // Get all entries with their current compression levels
    entries := a.db.GetNotebookEntries(ctx, sessionID)

    // Optional: auto-injection (experimental, disabled by default)
    // Instead of replacing entries in a rendered string, we set a
    // compression override on matching entries before rendering.
    overrides := make(map[string]int) // entryID → override compression level
    if a.autoInjectEnabled {
        refs := extractExplicitFilePaths(userMsg)
        if len(refs) > 0 {
            fullEntries := a.db.SearchNotebookByTags(ctx, sessionID, refs)
            count := 0
            for _, entry := range fullEntries {
                if entry.CompressionLevel > 0 && count < 2 {
                    overrides[entry.ID] = 0 // upgrade to full
                    count++
                }
            }
        }
    }

    // Render: iterate entries, apply overrides, concatenate
    var sb strings.Builder
    for _, e := range entries {
        level := e.CompressionLevel
        if override, ok := overrides[e.ID]; ok {
            level = override
        }
        text := e.EntryText
        if level > 0 {
            text = compressEntry(e, level) // tags + 1 sentence
        }
        sb.WriteString(text)
        sb.WriteString("\n\n")
    }
    return fantasy.NewSystemMessage(sb.String())
}
```

**Why rebuild instead of string surgery:**
- `replaceEntryInMessage` on a rendered markdown blob requires stable
  per-entry markers (e.g., `<!-- entry:id -->`) and find-and-replace —
  fiddly and error-prone.
- Rebuilding from DB is simpler: one query, iterate, render. No markers,
  no string splicing, no risk of partial replacements.
- The cost is one SQLite query per `preparePrompt` call, which already
  runs. Negligible.
- Compaction and auto-injection both work by adjusting what gets
  rendered, not by mutating a pre-rendered string.

**Auto-injection (experimental, opt-in):**

```go
// extractExplicitFilePaths only matches paths with separators to
// reduce false positives. Bare filenames like "auth.go" are NOT matched.
func extractExplicitFilePaths(msg string) []string {
    var refs []string
    if matches := fullPathRegex.FindAllString(msg, -1); matches != nil {
        for _, m := range matches {
            refs = append(refs, "file:"+filepath.Base(m))
        }
    }
    return refs
}
```

Logic:
- `findTurnBoundaryByTokenBudget` walks backwards from latest message,
  accumulates tokens, stops at 25K budget. Boundary always falls at a
  safe turn end (assistant message with no pending tool calls).
- Recent turns within budget: full raw messages (tool results, assistant
  responses) — guaranteed tool-call pairing intact. Adaptive: chat-only
  sessions get more turns, heavy-coding sessions get fewer.
- Older turns: replaced by per-event notebook entries
- Notebook entries sent as a single system message containing all entries
- Notebook message is **always rebuilt from DB state** — no string surgery
- Auto-injection is **experimental and disabled by default** — the model
  is expected to use `recall` and `notebook_search` tools for retrieval
- If notebook total > 100K tokens: compress oldest entries
  (keep tags + 1 sentence)

### Step 6: Recall tool — retrieve old full content

**File**: `internal/agent/tools/notebook/recall.go`
**File**: `internal/agent/tools/notebook/recall.md`

New tool the model can call to retrieve full content of old events:

```go
// recall retrieves full content from notebook entries that were compacted.
// Use when you need exact code, file contents, or command output from
// previous turns. Each entry covers one specific event.
func recall(query string) string {
    // 1. Parse query type:
    //    "file:auth.go"  → search by tag
    //    "turn:5"        → search by turn number
    //    "command"       → search by event_type
    //    "auth.go"       → fuzzy match on entry text + tags
    // 2. Return full (uncompacted) entries
}
```

Tool description:

```
# recall

Search and retrieve full details from previous events that were compacted
into the notebook. Each entry covers one specific event (file read, file
edit, command, decision).

Usage:
  recall("file:auth.go")      — retrieve all events that touched auth.go
  recall("pr:847")             — retrieve all events related to PR 847
  recall("bug:auth-nil-check") — retrieve events about this bug
  recall("turn:5")             — retrieve all events from turn 5
  recall("command")            — retrieve all command events
  recall("decision")           — retrieve all decision events

Returns: full notebook entries with all preserved details.
Each entry is about ONE event — no noise from unrelated events.

Do NOT re-read files with notebook entries — use recall first.
It is 16× cheaper than re-reading the file.
```

### Step 7: Notebook search tool — browse + query (one tool)

**File**: `internal/agent/tools/notebook/search.go`
**File**: `internal/agent/tools/notebook/search.md`

One tool serves both browsing and querying. With no args: lists all
entries (titles + tags only). With a query: filters by tag, event_type,
or entry text. Results are always titles + tags only — use `recall` for
full content.

```go
// notebook_search returns notebook entries for the current session.
// With no query: returns all entries (titles + tags only).
// With a query: returns entries matching the query (by tag, event_type,
// or entry text). Results are titles + tags only — use `recall` to get
// full content of any entry.
func notebookSearch(query string) string {
    entries := a.db.GetNotebookEntries(ctx, sessionID)
    if query != "" {
        entries = filterEntries(entries, query)
    }
    var sb strings.Builder
    for _, e := range entries {
        sb.WriteString(fmt.Sprintf("Turn %d.%d — %s %s\n",
            e.TurnNumber, e.EventNumber, e.Title, e.Tags))
    }
    return sb.String()
}
```

Tool description:

```
# notebook_search

List and search notebook entries for the current session. Returns titles
and tags only — use `recall` to get full content of any entry.

Usage:
  notebook_search()           — list all entries
  notebook_search("decision") — filter to decision entries
  notebook_search("file:auth.go") — filter to entries about auth.go
  notebook_search("command")  — filter to command entries

If configured with mem0, also searches across previous sessions.
```

**Why one tool, not two:**
- `notebook_list()` (no args) and `search(query)` (with args) do the same
  thing — return matching entries.
- Collapsing to one tool with an optional query reduces tool-surface
  bloat the model has to learn.
- The model doesn't need to decide "am I browsing or searching?" — it
  just calls `notebook_search` with or without a query.

### Step 8: System prompt update

**File**: `internal/agent/templates/coder.md.tpl`

Add notebook instructions to the system prompt:

```
You have a notebook of past events in this session. Each entry covers
one specific event (file read, file edit, command, decision).

- Use the `recall` tool to retrieve full details of any event.
  You can search by file name, tag, turn number, event type, or concept.
- Use `notebook_search` to browse all available entries or filter by
  a query (tag, event type, or text).
- Do NOT re-read files with notebook entries — use `recall` first.
  It is 16× cheaper than re-reading the file.
- If recall doesn't have what you need, then use `view` to re-read.
```

### Step 9: Compaction — when notebook exceeds 100K tokens

**File**: `internal/agent/notebook.go`

```go
func (a *sessionAgent) compactNotebook(
    ctx context.Context,
    sessionID string,
) error
```

Logic:
1. Get total token count for session's notebook
2. If <= 100K, do nothing
3. If > 100K, compress oldest entries:
   - Keep tags + 1 sentence per entry
   - Update `compression_level` in DB
   - Full entry remains in SQLite (still retrievable via recall tool)
   - Replace in-context entry with compressed version

Compression levels:
- 0: full entry (≤1000 tokens)
- 1: tags + 1 sentence (~100 tokens)
- 2: tags only (~20 tokens, for very old entries)

### Step 10: mem0 integration (optional)

**File**: `internal/agent/notebook.go`

After generating notebook entries, sync tags to mem0:

```go
func (a *sessionAgent) syncToMem0(entries []NotebookEntry) error
```

Calls mem0 MCP tool to add memory with tags. Enables cross-session recall:
"What did I do on PR 847 last week?"

Only runs when `notebook_sync_mem0` config is true.

### Step 11: Config option

**File**: `internal/config/config.go`

```go
type Config struct {
    // ... existing fields
    Notebook struct {
        Enabled           bool   `json:"notebook_enabled,omitempty"`
        RawTokenBudget    int    `json:"notebook_raw_token_budget,omitempty"`
        MaxNotebookTokens int64  `json:"notebook_max_tokens,omitempty"`
        MaxEntryTokens    int64  `json:"notebook_max_entry_tokens,omitempty"`
        SyncToMem0        bool   `json:"notebook_sync_mem0,omitempty"`
        AutoInject        bool   `json:"notebook_auto_inject,omitempty"`
    } `json:"notebook,omitempty"`
}
```

Defaults:
- `Enabled`: true
- `RawTokenBudget`: 25000 (adaptive, not fixed turn count)
- `MaxNotebookTokens`: 100000
- `MaxEntryTokens`: 1000
- `SyncToMem0`: false
- `AutoInject`: false (experimental, opt-in)

### Step 12: Tests

**File**: `internal/agent/notebook_test.go`

Tests using mock providers:
- Test event classification (significant vs trivial)
- Test that trivial turns generate no entries
- Test that trivial tool calls are grouped into exploration mini-entry
- Test notebook entry generation (mock small model returns structured entries)
- Test preparePrompt sends notebook + recent turns
- Test auto-injection: disabled by default (no-op)
- Test auto-injection: enabled + full path "internal/middleware/auth.go" → entry injected
- Test auto-injection: bare filename "auth.go" → NOT injected (too many false positives)
- Test auto-injection: max 2 entries per turn (not more)
- Test auto-injection: only upgrades compacted entries (not already full)
- Test `findTurnBoundaryByTokenBudget` never splits tool-call sequences
- Test adaptive boundary: small turns → more included, large turns → fewer included
- Test adaptive boundary: entire history within budget → all raw
- Test adaptive boundary: budget exceeded mid-turn → extends to safe boundary
- Test recall tool retrieves entries by tag
- Test recall tool retrieves entries by event_type
- Test recall tool with fuzzy query (no tag prefix)
- Test notebook_search with no query returns all entries (titles + tags only)
- Test notebook_search with query filters by tag/event_type/text
- Test notebook_search does not return full content (titles + tags only)
- Test compaction when notebook exceeds limit
- Test cross-session search via mem0
- Test fallback path when notebook disabled (existing Summarize still works)
- Test edge cases: <R turns, cancelled turns, 10+ tool calls per turn

## Commit Plan

| Commit | Description |
|--------|-------------|
| 0      | `chore: instrument preparePrompt to log token estimates` (temporary, for validation) |
| 1      | `feat: add notebook tables migration and SQL queries` |
| 2      | `feat: add event classification and notebook entry generation` |
| 3      | `feat: modify preparePrompt to rebuild notebook from DB + recent turns` |
| 4      | `feat: add recall tool for retrieving compacted context` |
| 5      | `feat: add notebook_search tool for browsing and querying entries` |
| 6      | `feat: update system prompt with notebook retrieval instructions` |
| 7      | `feat: add notebook compaction for large sessions` |
| 8      | `feat: add mem0 sync for cross-session notebook search` |
| 9      | `feat: add notebook config options` |
| 10     | `test: add notebook feature tests` |
| 11     | `docs: add notebook architecture documentation` |
| 12     | `refactor: remove temporary token instrumentation` |

## Expected Results (estimated — validate with Step 0)

| Metric                         | Current (est)    | With notebook (est) | Savings |
|--------------------------------|------------------|---------------------|---------|
| 20-turn session total tokens   | ~4M              | ~1.1M               | ~72%    |
| 50-turn session total tokens   | ~25M             | ~3.8M               | ~85%    |
| 100-turn session total tokens  | ~100M            | ~12.1M              | ~88%    |
| Credits/night (5M tokens)       | ~7,500 (71%)     | ~1,200 (12%)        | ~84%    |
| Context quality                | 70% junk         | 95% useful          | —       |
| Recall precision               | None             | Per-event (exact)   | —       |
| Cross-session memory           | None             | Via mem0 tags       | —       |

All numbers are **estimates** based on assumed ~10K tokens/turn average.
Must be validated by Step 0 instrumentation on real sessions.

## Risks and Mitigations

| Risk                              | Mitigation |
|-----------------------------------|------------|
| Notebook generation adds latency  | Async after response; uses small model (fast) |
| Notebook gen not ready before next turn | Include stale turn as raw (R+1 temporarily) |
| Summary loses critical detail     | Recall tool retrieves full content; 1000 token cap per entry |
| Model doesn't know it can recall   | System prompt instructs model; auto-injection handles common case |
| Model re-reads files instead of recalling | System prompt says "use recall first, 16× cheaper"; recall tool description reinforces |
| Notebook grows too large          | Compaction at 100K tokens; compression levels 0→1→2 |
| Tool-call sequence split          | `findTurnBoundaryByTokenBudget` + `findNextSafeBoundary` guarantee safe cut points |
| Token estimate inaccuracy         | Adaptive budget is approximate; over-estimate is safe (fewer raw turns), under-estimate sends slightly more |
| mem0 not configured               | Feature works without mem0 (SQLite-only mode) |
| Notebook gen cost offsets savings | Small model (0.17× multiplier); per-event is cheaper than per-turn; net savings ~87% |
| Existing Summarize conflicts      | Notebook replaces Summarize when enabled; fallback when disabled |
| Too many entries for complex turns | Batched into one small model call; trivial events grouped |
| Auto-injection injects too much   | Disabled by default; max 2 entries; only full paths with separators |
| Auto-injection misses references  | Expected — model uses recall tool instead; auto-injection is bonus |
| Auto-injection over-injects      | Only matches full paths with separators, not bare filenames |

## Open Questions (resolved)

1. **Granularity** — **per-event** (one entry per significant tool call or
   decision), not per-turn or per-phase. Trivial events grouped.
2. **Raw window** — **adaptive by token budget** (default 25K), not
   fixed turn count. Chat-only sessions get more turns, heavy-coding
   sessions get fewer. Budget stays bounded regardless of session type.
3. **Max notebook tokens** — **100K** (with compaction)
4. **Max entry tokens** — **1000** (hard cap, enforced by truncation)
5. **mem0 sync** — **optional**, disabled by default
6. **Notebook generation timing** — **async** (non-blocking, stale-tolerant)
7. **Replace vs coexist with Summarize** — **replace** when enabled,
   **fallback** when disabled
8. **Reasoning content in notebook** — **excluded** from entries (too
   verbose, low value for retrieval)
9. **Trivial turns** — **skip entirely** (no tools, < 200 tok response)
10. **Trivial tool calls** — **grouped** into one exploration mini-entry
11. **Retrieval mechanism** — **three-tier, ordered by reliability**:
    recall tool (primary, model-driven), notebook_search (supporting,
    browse + query in one tool), auto-injection (experimental, opt-in,
    disabled by default)
12. **System prompt** — **updated** to instruct model to use recall as
    primary retrieval path; auto-injection not mentioned as guaranteed
13. **Auto-injection** — **downgraded to experimental, opt-in, disabled
    by default**. Regex is fragile (over-injects on incidental mentions,
    under-injects on conceptual references). Only matches full file
    paths with separators, max 2 entries. Model uses recall instead.
14. **Notebook message rendering** — **always rebuild from DB state**,
    never mutate a rendered string. Avoids fragile string-level surgery
    (no per-entry markers, no find-and-replace). Cost is one SQLite query
    per preparePrompt call — negligible.
15. **Browse + search tools** — **collapsed into one tool**
    (`notebook_search`). No args → list all; with query → filter.
    Reduces tool-surface bloat.

## Remaining Open Questions

1. **Should the recall tool search mem0 cross-session by default, or only
   when explicitly requested?**
2. **Should notebook entries be visible in the TUI** (e.g., a `/notebook`
   command to view entries)?
3. **Should event classification thresholds be configurable?** (e.g.,
   minimum file size for "significant" read)
4. **Should auto-injection use semantic search instead of regex?** (e.g.,
   embed user message + notebook entries, match by similarity — would
   fix both over- and under-injection but adds embedding cost)
5. **Should recall support fuzzy/conceptual search by default?** (e.g.,
   "login bug" matches entries about auth nil check — requires either
   embedding or full-text search on entry text)
