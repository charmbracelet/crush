# Context Window Safety — Implementation Plan

## Goal

Prevent request overflow when the sum of system prompt, tools, MCP
instructions, raw history, notebook entries, and current prompt exceeds
the model's context window. This is a safety plan, not a pollution
reduction plan.

## Relationship to Prompt Optimization

This plan is split from `PROMPT_OPTIMIZATION.md`. The split is
intentional:

- **Prompt Optimization** reduces what is sent (relevance, deduplication,
  conditional loading). It targets repeated bytes and model distraction.
- **Context Window Safety** ensures the request fits when all required
  content is present. It targets overflow.

Context-window budgeting solves overflow, but it does not directly
solve:

- Repeated bytes over the network.
- Irrelevant data distracting the model.

Do not implement this plan until Prompt Optimization PRs 1-4 are
measured. The measurement data will show whether overflow is a real
problem or a theoretical one.

## Problem

Crush assembles requests across two functions:

- `Run` builds the system prompt, copies tools, and appends MCP
  instructions (`agent.go:706-725`), creates the Fantasy agent
  (`agent.go:732-737`), and calls `agent.Stream` (`agent.go:851`).
- `preparePrompt` (`agent.go:1639`) receives only old messages, image
  support, and attachments. It does **not** receive the current system
  prompt, tools, MCP instructions, current prompt, or output reserve.
- `buildNotebookMessage` runs inside `preparePrompt` and cannot access
  the budget inputs it needs.

Without a single function owning all request components, no component
can reliably measure the total request size or handle overflow.

## Required design: per-step request preparation

`Agent.Stream` invokes `PrepareStep` (`agent.go:863-934`) for **every
step** of the tool-call loop. `PrepareStep` folds queued prompts, copies
the latest tool list (`agent.go:870`), and the provider's message list
grows with each tool-result round. The budget calculation (notebook
drop, MCP caps, raw-turn protection) therefore needs to happen **inside
`PrepareStep`**, not once at `Run` start.

### `prepareRequest` subsumes `preparePrompt`

Today `preparePrompt` runs once at `Run` start (`agent.go:838`). For
per-step budgeting, request assembly must move into `PrepareStep`, which
means `prepareRequest` takes over **all** of `preparePrompt`'s
responsibilities, not just notebook selection:

- Turn-boundary computation and the stale-turn fallback
  (`agent.go:1662-1679`).
- Orphaned tool-call/tool-result repair (`agent.go:1704-1752`).
- `supportsImages` file-part filtering (`agent.go:1738-1744`).
- The todo-reminder system message (`agent.go:1641-1650`).
- Notebook auto-injection via `maybeAutoInject` (`agent.go:1687-1698`).

`options.Messages` inside `PrepareStep` is `[]fantasy.Message`, but turn
classification (compacted vs. uncompacted) requires the DB
`[]message.Message` list. `PrepareStep` must therefore re-fetch session
messages each step via `getSessionMessages` so that queued prompts
folded during the step and tool results persisted by the loop are
included, then pass them to `prepareRequest` as
`RequestPreparation.Messages`. `PreparedRequest.Messages` then replaces
`prepared.Messages`, and `workaroundProviderMediaLimitations`
(`agent.go:893`) must be applied to the result.

Two ordering requirements follow:

- Folded queued prompts are persisted by `createUserMessage`
  (`agent.go:886`) and must be persisted **before** the re-fetch so
  they appear exactly once in the rebuilt list.
- The cache-control marking loop (`agent.go:895-909`) and the
  `ProviderOptions = nil` strip (`agent.go:865-867`) must run **after**
  the rebuild and after the `promptPrefix` prepend, or prompt caching
  regresses.

The re-fetch is per-step, so the cost is O(steps × history): a full
message `List` + JSON deserialize, `GetEntries`, `SearchByTag` per
auto-inject tag, and an O(history) token estimate each step. That is
fine for typical sessions; for long ones, memoize on message count —
when `len(msgs)` is unchanged since the last step, the boundary, the
entry selection, and the raw-history estimate can all be reused.

### DB rebuild vs. `options.Messages` equivalence

Rebuilding from the DB is not perfectly equivalent to
`options.Messages`. Fantasy appends `toResponseMessages(stepContent)`
per step (`fantasy/agent.go:1073`), which can contain parts Crush never
persists: `fantasy.SourceContent` has no `message.ContentPart`
equivalent (it is token-counted in `usage_fallback.go:72-75` but never
stored), and `OnToolCall` hardcodes `ProviderExecuted: false`
(`agent.go:1014`). Wholesale replacement therefore silently drops
source citations and any provider-executed content on steps > 0.

Low impact today, but the rebuild must either:

- **Merge**: splice non-persistable parts observed in
  `options.Messages` into the rebuilt list, or
- **Assert**: verify the rebuilt set covers the persisted set and
  diff-log the parts being dropped so the loss is explicit.

### The current prompt is already in `Messages`

`createUserMessage` persists the prompt before `agent.Stream`
(`agent.go:768`), so the per-step re-fetch already includes the
current-turn user message — text attachments included (`ToAIMessage`
re-inlines them identically to `PromptWithTextAttachments`). Measuring
the prompt as a separate mandatory section would count it twice; it is
covered by the `Messages` measurement. Likewise `PreparedRequest` has
no `Prompt`/`Files` fields: `PrepareStepResult` exposes no such sink
(`fantasy/agent.go:107-115`) — `createPrompt` folds
`AgentStreamCall.Prompt`/`Files` into `options.Messages` before
`PrepareStep` ever runs (`fantasy/agent.go:929,1247-1286`).

### Never set `prepared.System`

`options.Messages[0]` is the system prompt, and crush prepends
`promptPrefix` at index 0 (`agent.go:911-913`). If `prepareRequest`
ever sets `prepared.System`, fantasy replaces `stepInputMessages[0]`
(`fantasy/agent.go:990-997`) — clobbering the prefix while the old
system message remains at index 1, yielding two system messages. Any
mid-run system-prompt change (e.g. dropping MCP instructions) must
mutate the system element inside `Messages` in place. State this
explicitly in the implementation.

```go
type RequestPreparation struct {
    Model           Model
    SystemPrompt    string
    Tools           []fantasy.AgentTool
    MCPInstructions string
    Messages        []message.Message // per-step re-fetch; includes current prompt
    MaxOutputTokens int64
}

type PreparedRequest struct {
    Messages       []fantasy.Message
    Tools          []fantasy.AgentTool
    SystemPrompt   string
    Sections       []PromptSection     // for telemetry; new type, defined with prepareRequest
    TotalEstTokens int64
}

// prepareRequest is called from PrepareStep with the actual
// messages/tools for that step, not once at Run start. It subsumes
// preparePrompt: boundary computation, stale-turn fallback, orphan
// tool-call repair, image filtering, and auto-inject all live here.
func (a *sessionAgent) prepareRequest(req RequestPreparation) (PreparedRequest, error) {
    // 1. Calculate input budget:
    //    input budget = context window - effective output - safety margin
    //    safety margin = max(4K, contextWindow * 5%)
    //
    // 2. Measure mandatory sections (system prompt + promptPrefix,
    //    core tools, MCP instructions, raw history — which already
    //    includes the persisted current-turn user message).
    //
    // 3. If mandatory sections exceed input budget AND the context
    //    window is known, return an actionable error (do not silently
    //    truncate policy). For cw == 0, warn and continue — see
    //    "Unknown context window fallback".
    //
    // 4. Calculate remaining budget for optional content (notebook
    //    entries, compacted raw turns, MCP catalog/prose).
    //
    // 5. Select notebook entries within remaining budget using
    //    Entry.TokenCount, newest-to-oldest, skip-and-continue
    //    (lowest-relevance-first once PR4 relevance scoring exists).
    //
    // 6. Assemble final messages (notebook system message + raw
    //    history). The current prompt needs no special handling — it
    //    is already the last persisted user message in Messages.
    //
    // 7. Perform final whole-request estimate. If it exceeds the
    //    input budget, drop optional content in priority order (see
    //    the table below): MCP catalog/prose first, then notebook
    //    entries lowest-value-first, then compacted raw turns
    //    oldest-first (only turns whose covering entries are
    //    rendered). Never drop uncompacted raw turns or mandatory
    //    sections.
    //
    // 8. Return PreparedRequest with section measurements.
}
```

## Section priorities

Define explicit priorities for overflow handling:

| Priority                    | Section                                                         | Overflow behavior       |
| --------------------------- | --------------------------------------------------------------- | ----------------------- |
| 1 (never drop)              | Safety/core policy (system prompt, promptPrefix, context files) | Error if exceeds budget |
| 2 (never drop)              | Current user prompt + attachments                               | Error if exceeds budget |
| 3 (never drop)              | Required tool schemas (core tools + loaded MCP tools)           | Error if exceeds budget |
| 4 (never drop)              | Uncompacted raw turns (no rendered notebook entry)              | Error if exceeds budget |
| 5 (preserve where possible) | Compacted raw turns (entry selected for rendering)              | Drop oldest first       |
| 6                           | Relevant notebook entries (incl. auto-injected full entries)    | Drop lowest-value first |
| 7                           | General notebook history                                        | Drop lowest-value first |
| 8 (drop first)              | Unloaded MCP catalog / general server prose                     | Drop first              |

Priorities 6-7 are "oldest first" until Prompt Optimization PR4 lands
relevance scoring; after that, drop **lowest-relevance-first** so a
highly relevant old entry is not evicted by a marginally relevant new
one. The current-prompt row (2) is measured as the persisted last user
message inside `Messages`, not a separate section.

`promptPrefix` (added per step at `agent.go:911-913`) and the
auto-injected full-entry message (`agent.go:1687-1698`) are part of the
request even though the original `preparePrompt` didn't budget them;
the table accounts for both.

### Raw turn protection

Raw turns cannot be silently dropped merely because the budget is
exceeded. Existing notebook logic deliberately keeps old turns that do
not yet have notebook entries to protect against asynchronous notebook
generation falling behind (`agent.go:1662-1679`).

Classify raw turns as:

- **Compacted**: a valid notebook entry exists **and is selected into
  the rendered notebook message** for this step; the raw turn may be
  removed. An entry that exists but was skipped by the notebook budget
  does **not** make its turn compacted — dropping the raw turn while
  its entry is absent silently loses the content, which is exactly
  what this classification exists to prevent.
- **Uncompacted**: no notebook entry exists, or the covering entry was
  dropped by the budget; do not silently remove.
- **Partially compacted**: some events are missing; preserve until complete.

If uncompacted required history causes overflow, either:

1. Synchronously compact enough turns.
2. Wait briefly for in-flight notebook generation.
3. Return an actionable context-overflow error.

Do not silently drop uncompacted turns.

### MCP content classification

MCP content is not uniformly optional. Split into:

- **Unloaded MCP catalog**: optional; drop first.
- **Loaded tool schemas**: required for the current step.
- **Operational instructions required by loaded tools**: required.
- **General server prose**: optional.

Never remove a loaded tool between tool-call generation and execution
merely to fit the budget.

If mandatory sections (1-4) alone exceed the model window, return an
actionable error instead of sending an oversized request — but only
when the context window is known; see "Unknown context window
fallback". Do not solve that case by silently truncating project
policy.

Reserve for provider-specific hidden overhead and estimation error:
`max(4K, contextWindow * 5%)`.

## Context-file overflow handling

| Condition                                   | Behavior                                                                                                                            |
| ------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Required file exceeds 256KB hard read limit | Error; do not run                                                                                                                   |
| Required file exceeds soft token budget     | Prompt user interactively; error in non-interactive mode (`crush run`)                                                              |
| Optional file exceeds soft token budget     | Truncate/exclude according to config                                                                                                |
| Invalid UTF-8 anywhere in file              | Normalize with `strings.ToValidUTF8` then trim trailing incomplete sequence via `truncateUTF8Prefix` (see `PROMPT_OPTIMIZATION.md`) |

A TUI question cannot be used during startup or headless `crush run`
without a non-interactive policy. In non-interactive mode, soft
overflow on required files is an error, not a silent truncation.

Note that the interactive branch needs plumbing that does not exist
today: `loadContextFiles` runs inside `promptData` during system-prompt
`Build` (`prompt/prompt.go:152-172`), far below any TUI question
machinery. The options are:

- Have `processContextPath`/`promptData` return a structured
  soft-overflow signal, surface the question at the app/TUI layer, then
  rebuild the prompt with the user's decision; or
- Resolve the policy at config load time instead of at Build.

Either way this is new plumbing, not just a policy branch — call it out
as such when scoping the work.

**Do not truncate required project policy solely to make room for
optional notebook or MCP content.** Budget optional content first.

### Bounded reading

The 256KB hard read limit is a **process-protection** limit measured in
bytes. The soft budget is measured in **tokens**. The hard byte limit
(`readBounded` / `maxContextFileReadSize`) and the soft-budget wrapper
(`readContextFile`) are defined in `PROMPT_OPTIMIZATION.md` (Defensive
helpers) because they are defensive process guards that do not depend on
the request-budget manager. This plan defines only the soft token-budget
values, the required/optional overflow policy, and the conversion
helpers that `readContextFile` consumes.

> **Status:** `readContextFile`, `tokenLimitToBytes`, and
> `truncateToTokenLimit` below are proposed, not implemented — they
> were dropped during review for lack of production callers and should
> land together with this overflow-handling work. `readBounded` and
> `truncateUTF8Prefix`/`truncateUTF8Suffix` are the only implemented
> helpers.

`truncateUTF8Prefix` is also defined in `PROMPT_OPTIMIZATION.md`
(Defensive helpers). It takes a `string` and `maxBytes` and returns
valid UTF-8 no longer than `maxBytes`.

### Byte/token conversion for soft budgets

Defensive caps and soft budgets are specified in **tokens**, but
`truncateUTF8Prefix` operates on **bytes**. Derive a conservative
`maxBytes` from the token limit and verify after truncation:

```go
// approxTokenCount uses ~4 bytes per token (matching the existing
// heuristic in the codebase). For defensive caps, derive maxBytes
// from the token limit, truncate, then verify.
func tokenLimitToBytes(tokenLimit int) int {
    return tokenLimit * 4
}

// truncateToTokenLimit truncates s so that approxTokenCount(result) <= tokenLimit.
func truncateToTokenLimit(s string, tokenLimit int) string {
    maxBytes := tokenLimitToBytes(tokenLimit)
    result := truncateUTF8Prefix(s, maxBytes)
    // If the result still exceeds the token limit (dense tokens),
    // shrink and re-truncate. Use a 64-byte step while maxBytes is
    // large, then switch to a 1-byte step once below the step size so
    // that very small tokenLimits (e.g. 1 with CJK text) do not drop
    // maxBytes below the first rune and return an empty string.
    for approxTokenCount(result) > tokenLimit && maxBytes > 0 {
        if maxBytes >= 64 {
            maxBytes -= 64
        } else {
            maxBytes--
        }
        result = truncateUTF8Prefix(s, maxBytes)
    }
    return result
}
```

For the documented caps (1K, 4K, 12K tokens) the 64-byte step is always
used and the small-limit path is never hit. The 1-byte fallback exists
only to keep `truncateToTokenLimit` correct for arbitrarily small
limits; callers should not rely on sub-rune precision.

The linear shrink is O((Δbytes/64) × len) — on a 256KB dense-token file
this can approach ~750MB of `ToValidUTF8` passes. Bounded and
defensive-path-only, but a binary search over `maxBytes` in
`[0, tokenLimitToBytes(tokenLimit)]` costs nothing extra and removes
the quadratic behavior; prefer it at implementation time.

This applies to MCP instruction caps in `PROMPT_OPTIMIZATION.md` and
context-file soft budgets here.

## Unknown context window fallback

For an unknown context window (`cw == 0`), use a conservative fallback
of **8K tokens** — not 50K — for budgeting optional content. An unknown
model might have only a 16K or 32K context window.

`prepareRequest` computes the input budget as
`context window - output - safety margin`. For `cw == 0` the fallback
is applied **before** that subtraction, and the result is clamped at
zero:

```go
knownWindow := req.Model.CatwalkCfg.ContextWindow
contextWindow := knownWindow
if contextWindow == 0 {
    contextWindow = 8_192 // conservative fallback for unknown models
}
safetyMargin := max(4_096, int64(float64(contextWindow)*0.05))
effectiveOutputMax := req.MaxOutputTokens
if effectiveOutputMax == 0 {
    effectiveOutputMax = req.Model.CatwalkCfg.DefaultMaxTokens
}
if effectiveOutputMax == 0 {
    effectiveOutputMax = defaultOutputReserve // 4_096, see below
}
inputBudget := max(0, contextWindow-effectiveOutputMax-safetyMargin)
```

With the 8K fallback and a 4K output reserve, `inputBudget` is 0: an
unknown model gets no notebook injection budget.

**However, the mandatory-section overflow error must only fire when the
context window is known (`knownWindow > 0`).** The 8K figure is a
budgeting assumption, not a measured limit — a custom or local model
whose real window is 16K or 32K would hard-error on every request if
the check compared mandatory sections against an `inputBudget` of 0.
Today such models work (auto-summarize deliberately skips `cw == 0` at
`agent.go:1097-1100`); turning a budgeting guess into a guaranteed
failure would be a regression. For `cw == 0`, send mandatory content
with a warning and let the provider enforce its real limit.

The same applies to known but tiny windows: if `inputBudget` computes
to 0 for a known `cw` (e.g. a 4K-window model), the mandatory check
compares against `contextWindow - effectiveOutputMax` (the window minus
real output needs) rather than the clamped budget, so the error only
fires when the request genuinely cannot fit.

## Notebook budget calculation

```go
// input budget =
//     context window (8K fallback when unknown)
//   - effective output maximum (call → DefaultMaxTokens → 4K)
//   - safety margin (max(4K, contextWindow * 5%))
//
// notebook budget =
//     input budget
//   - core prompt + promptPrefix (measured)
//   - tool schemas (measured)
//   - MCP instructions (measured)
//   - raw history incl. current-turn user message (measured)
```

### MaxOutputTokens = 0 fallback

The current code does not send `MaxOutputTokens` when it is 0
(`agent.go:846-850`). A flat fallback under-reserves for models whose
catalog entry declares a larger output budget, so fall back through
`CatwalkCfg.DefaultMaxTokens` first — the same chain already used at
`coordinator.go:324` and `agent.go:2160` — and only then to a constant:

```go
const defaultOutputReserve = 4_096 // tokens

effectiveOutputMax := req.MaxOutputTokens
if effectiveOutputMax == 0 {
    effectiveOutputMax = req.Model.CatwalkCfg.DefaultMaxTokens
}
if effectiveOutputMax == 0 {
    effectiveOutputMax = defaultOutputReserve
}
```

This prevents the input budget from consuming the entire context window
and leaving no room for model output.

Walk entries newest-to-oldest, accumulating `Entry.TokenCount`
(`notebook.go:48`). This modifies the existing filtering loop in
`buildNotebookMessage` (`agent.go:1813-1823`) — the existing loop
already filters by `boundaryTurn` and tracks `turnsWithEntries`. Add
budget accumulation after that filtering. If one entry exceeds the
remaining budget, **skip it and continue** (don't `break`):

```go
// Existing loop at agent.go:1813-1823 already builds `filtered` by
// boundaryTurn and tracks turnsWithEntries. After that loop, apply
// the notebook budget:
var selected []notebook.Entry
var accumulated int64
for i := len(filtered) - 1; i >= 0; i-- {
    if accumulated+filtered[i].TokenCount > notebookBudget {
        continue // skip large entry, try smaller ones
    }
    selected = append(selected, filtered[i])
    accumulated += filtered[i].TokenCount
}
slices.Reverse(selected)
// Then render `selected` instead of `filtered`.
```

Account for `RenderEntries` formatting overhead — `TokenCount`
(`classify.go:270`) measures uncompressed `EntryText` while
`RenderEntries` may render `compressEntry` output. After rendering, if
the result exceeds the budget, remove oldest entries until it fits.

After assembling the full request, perform a final whole-request
estimate. If it exceeds the input budget, drop optional content in the
priority order from the table above: MCP catalog/prose first, then
notebook entries oldest-first, then compacted raw turns oldest-first —
only turns whose covering entries are in the rendered set. Never drop
uncompacted raw turns or mandatory sections; if the request still does
not fit, that is the actionable overflow error.

## Interaction with auto-summarize

The existing `StopWhen` condition (`agent.go:1094-1115`) triggers
summarization when remaining window drops below a threshold (20K buffer
for windows over 200K, else 20% of the window) — but only when
`!a.notebookEnabled && !a.disableAutoSummarize` (`agent.go:1110`).
Three consequences:

- With notebook enabled, or with auto-summarize disabled via config,
  there is currently **no** overflow protection at all; this plan is
  the only guard in both modes.
- With notebook disabled and summarize enabled, the summarize threshold
  (20%/20K) is larger than the safety margin (5%/4K), so summarization
  normally triggers before the budget overflows — the two mechanisms
  are complementary.
- Reconcile the constants when implementing so the relationship stays
  intentional rather than incidental.

## When to implement

Do not implement this plan until:

1. Prompt Optimization PR 1 (measurement) is complete.
2. The measurement data confirms overflow is a real problem.
3. Prompt Optimization PRs 2-4 (deduplication, conditional MCP,
   relevance notebook selection) are complete — these reduce content
   and may make overflow rare enough that budgeting is unnecessary.

If measurement shows overflow never happens in practice, this plan may
not be needed. The defensive hard limits in the Prompt Optimization
plan (MCP instruction caps, notebook injection cap, context file read
limit) may be sufficient.

## Verification

- Run `go test ./internal/agent/...`
- Run `go test ./internal/notebook/...`
- Test with a session that has many notebook entries and a small model
  context window — verify budget is respected and the request fits.
- Test with unknown context window (cw == 0) — verify the conservative
  fallback (8K, not 50K) yields no notebook budget, and that mandatory
  content is still sent with a warning rather than a guaranteed error.
- Test with a known tiny window (e.g. 4K) — verify the error fires only
  when mandatory sections genuinely cannot fit.
- Test with one very large entry — verify it is skipped and smaller
  entries are still included.
- Test final whole-request estimate — verify overflow drops content in
  priority order (MCP catalog/prose, then notebook entries, then
  compacted raw turns), never mandatory content or uncompacted turns.
- Test that a raw turn whose covering entry was skipped by the budget
  is treated as uncompacted, not dropped.
- Test a multi-step tool-call loop — verify the budget is re-evaluated
  inside `PrepareStep` as `options.Messages` grows with tool results.
- Test with queued prompts folded during `PrepareStep` — verify they
  appear exactly once in the rebuilt list (persisted before re-fetch).
- Test that the current prompt is counted once — it arrives via the
  persisted user message, not a separate section.
- Test `prepared.System` is never set — verify `promptPrefix` survives
  at index 0 and exactly one system message ordering holds when MCP
  instructions are dropped mid-run.
- Test non-persisted parts (`fantasy.SourceContent`, provider-executed
  tool content) — verify they are either merged into the rebuild or
  explicitly diff-logged as dropped.
- Test with mandatory content exceeding context window — verify
  actionable error, not silent truncation.
- Test with required context file exceeding soft budget in
  non-interactive mode — verify error, not silent truncation.
