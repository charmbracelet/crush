# Notebook vs Summary — Comparison

## How each works (from actual code)

### Existing Summarize() (agent.go:1329)

```
1. Trigger: reactive — only when context is near full (agent.go:1053)
   remaining = context_window - (prompt_tokens + completion_tokens)
   if remaining <= 20K (large context) or 20% (small context):
       shouldSummarize = true

2. Model: LARGE model (expensive) — agent.go:1383
   a.largeModel.Get().Model

3. Input: ALL messages in session (agent.go:1351)
   aiMsgs, _ = a.preparePrompt(msgs, ...)
   → sends entire conversation history to generate one summary

4. Output: ONE summary message (agent.go:1369)
   Created with IsSummaryMessage: true
   No length limit (template says "No limit. Err on the side of too much detail")

5. Replacement: ALL history replaced by summary (agent.go:1695)
   getSessionMessages() truncates to start from summary message:
   msgs = msgs[summaryMsgIndex:]
   → everything before the summary is GONE from context
   → summary message role changed to "user"

6. Retrieval: NONE
   Old messages still exist in DB but are never sent again
   No tool to retrieve them
```

### Proposed Notebook

```
1. Trigger: proactive — after every turn (async)
   No threshold check — always generates

2. Model: SMALL model (cheap) — 0.17× Credit multiplier
   a.smallModel.Get().Model

3. Input: only that turn's messages (~5K-50K tokens)
   Not the entire history

4. Output: one entry per turn, 2000 token hard cap
   Stored in notebook_entries table with tags

5. Replacement: only old turns replaced (agent.go preparePrompt modified)
   Last 5 turns: full raw messages (tool-call safe)
   Older turns: notebook entries as system message
   Old full content stays in DB, retrievable via recall tool

6. Retrieval: recall tool
   recall("file:auth.go") → returns full notebook entries
   search("pr:847") → searches across sessions via mem0
```

## Side-by-side comparison

| Aspect | Summarize() | Notebook |
|--------|-------------|----------|
| **Trigger** | Reactive (near overflow) | Proactive (every turn) |
| **When it fires** | ~980K of 1M context used | After every turn, always |
| **Model used** | Large (expensive) | Small (cheap, 0.17× multiplier) |
| **Input size** | Entire conversation (~1M tokens) | One turn (~5K-50K tokens) |
| **Output** | One summary message | One entry per turn |
| **Output size** | No limit (template says "no limit") | 2000 token hard cap per entry |
| **What gets replaced** | ALL history → one summary | Only turns older than 5 → notebook entries |
| **Recent context** | Lost (summary only) | Last 5 turns full raw |
| **Retrieval** | None — old context gone | recall tool retrieves full content |
| **Cross-session** | None | mem0 tags (optional) |
| **Frequency** | Once per session (maybe) | Every turn |
| **Latency** | Blocking (user waits) | Async (non-blocking) |
| **Cost per invocation** | ~1M tokens × large model | ~20K tokens × small model |
| **Detail preservation** | Lossy — one-shot compression | Rich — incremental per turn |
| **Tool-call safety** | N/A (replaces everything) | findTurnBoundary guarantees safe cuts |

## What the summary actually looks like

The existing `summary.md` template produces:

```markdown
## Current State
- What task is being worked on (exact user request)
- Current progress and what's been completed
- What's being worked on right now (incomplete work)
- What remains to be done (specific next steps, not vague)

## Files & Changes
- Files that were modified (with brief description of changes)
- Files that were read/analyzed (why they're relevant)
- Key files not yet touched but will need changes
- File paths and line numbers for important code locations

## Technical Context
- Architecture decisions made and why
- Patterns being followed (with examples)
- Libraries/frameworks being used
- Commands that worked (exact commands with context)
- Commands that failed (what was tried and why it didn't work)
- Environment details (language versions, dependencies, etc.)

## Strategy & Approach
- Overall approach being taken
- Why this approach was chosen over alternatives
- Key insights or gotchas discovered
- Assumptions made
- Any blockers or risks identified

## Exact Next Steps
1. Add JWT middleware to src/middleware/auth.js:15
2. Update login handler in src/routes/user.js:45 to return token
3. Test with: npm test -- auth.test.js
```

### Problems with this format

1. **One-shot**: summarizes everything at once — early turns get compressed
   as much as recent turns. No granularity.
2. **No tags**: no way to search or retrieve specific topics later.
3. **No retrieval**: once summarized, old context is gone. If the model
   needs a file it read 50 turns ago, it has to re-read it.
4. **Expensive**: uses the large model to process the entire conversation.
5. **No length limit**: template says "No limit. Err on the side of too
   much detail" — same problem the notebook plan originally had.
6. **Replaces everything**: `getSessionMessages()` truncates to start
   from the summary. Even recent context is lost.

## What the notebook entry looks like

```markdown
## Turn 5 — 2026-09-09 11:30

### Task
Fix authentication bug in nexus-backend PR #847

### Files examined
- internal/auth/jwt.go (450 lines) — ValidateToken(), RefreshToken()
  Key code: func ValidateToken(token string) (claims *Claims, err error) {
    ...returns ErrExpiredToken after 24h...
  }
- internal/middleware/auth.go (120 lines) — wraps ValidateToken
  Bug: line 67, missing nil check on claims

### Changes made
- internal/middleware/auth.go line 67: added nil check
  ```go
  if claims == nil { return ErrInvalidClaims }
  ```

### Commands run
- go test ./internal/middleware/ → PASS (3 tests)
- go build ./... → success

### Tags
#pr:847 #project:nexus-backend #file:auth.go #bug:auth-nil-check

### Decisions
- Chose nil check over error wrapping for minimal change

### Open questions
- Should RefreshToken also check nil claims? (deferred to PR #850)
```

### Advantages of this format

1. **Per-turn**: each turn gets its own entry — early turns preserved
   with same fidelity as recent turns.
2. **Tagged**: `#pr:847`, `#file:auth.go` — searchable and retrievable.
3. **Retrievable**: recall tool fetches full content if needed.
4. **Cheap**: small model processes only one turn, not entire history.
5. **Hard cap**: 2000 tokens per entry — bounded, guaranteed.
6. **Recent context preserved**: last 5 turns stay raw.

## Cost comparison for a 100-turn session

### Summarize() (fires once at ~980K tokens)

| Step | Tokens sent | Model | Cost |
|------|-------------|-------|------|
| Turns 1-99 (growing history) | ~50M cumulative | Large | ~7,500 Credits |
| Summarize (turn 100) | ~1M input + ~50K output | Large | ~1,575 Credits |
| Turn 101+ (summary + new) | ~50K per call | Large | — |
| **Total to summarize point** | **~51M** | | **~9,075 Credits** |

### Notebook (every turn)

| Step | Tokens sent | Model | Cost |
|------|-------------|-------|------|
| Turns 1-100 main (flat ~75K) | ~7.5M cumulative | Large (0.1× promo) | ~750 Credits |
| Notebook gen (100 turns × ~20K) | ~2M cumulative | Small (0.17×) | ~340 Credits |
| **Total** | **~9.5M** | | **~1,090 Credits** |

### Difference

| | Summarize() | Notebook | Savings |
|---|---|---|---|
| Total tokens sent | ~51M | ~9.5M | ~81% |
| Credits used | ~9,075 | ~1,090 | ~88% |
| Context after 100 turns | One summary (~50K) | Notebook (~100K) + 5 raw turns | Richer |
| Can retrieve old context | No | Yes (recall tool) | — |

## Quality comparison

### Scenario: Model needs file content from turn 10 (at turn 90)

| | Summarize() | Notebook |
|---|---|---|
| **Is the file content available?** | No — summarized away | No — in notebook entry (summarized) |
| **Can it be retrieved?** | No — must re-read the file | Yes — `recall("file:auth.go")` returns full entry |
| **Cost to recover** | Full tool call + tool result in context (~8K tokens) | recall tool call (~500 tokens) |
| **Detail level** | Whatever the one-shot summary preserved | Structured per-turn entry with code snippets |

### Scenario: Model needs to recall a decision from turn 5

| | Summarize() | Notebook |
|---|---|---|
| **Is the decision available?** | Maybe — if summary included it | Yes — in notebook entry for turn 5 |
| **How to find it** | Scan the summary text | `recall("turn:5")` or `recall("decision")` |
| **Context cost** | Whatever the summary included | ~500 tokens from recall tool |

### Scenario: Starting a new session on same project

| | Summarize() | Notebook |
|---|---|---|
| **Cross-session memory** | None | mem0 tags (if enabled) |
| **Can find last session's work?** | No | `search("pr:847")` via mem0 |
| **Context from previous session** | Lost | Retrieved via search tool |

## When Summarize() is better

| Scenario | Why |
|---|---|
| **Short session (< 10 turns)** | Notebook overhead not worth it; summarize never fires |
| **Simple task (1-3 tool calls)** | No need for per-turn entries |
| **No mem0 configured** | Notebook still works (SQLite-only) but cross-session lost |
| **Emergency context overflow** | Summarize is a safety net if notebook fails |

## When Notebook is better

| Scenario | Why |
|---|---|
| **Long session (20+ turns)** | Flat token usage vs exponential growth |
| **Complex multi-file work** | Per-turn entries preserve what happened when |
| **Need to recall old decisions** | recall tool retrieves specific turns |
| **Multiple sessions on same project** | mem0 tags enable cross-session search |
| **Cost-sensitive (Token Plan)** | 88% Credit savings on long sessions |

## Recommendation: hybrid approach

Keep `Summarize()` as a **fallback safety net**. Use Notebook as the
**primary context management**:

```go
if notebookEnabled {
    // Use notebook: per-turn entries + recent raw turns
    // Compaction handles overflow at 100K notebook tokens
} else {
    // Fallback: existing Summarize() behavior
    // Fires reactively at context window threshold
}
```

This means:
- Notebook enabled (default): flat token usage, rich context, recall tool
- Notebook disabled: existing behavior, no changes
- Notebook + emergency: if notebook compaction fails, Summarize fires as
  last resort
