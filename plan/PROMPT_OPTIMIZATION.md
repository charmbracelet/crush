# Prompt Optimization — Implementation Plan - Implemented

## Goal

Minimize request payload and model distraction by removing duplicated
instructions and conditionally including tools, MCP instructions, and
historical context based on task relevance. Context-window budgeting
is out of scope except for defensive hard limits on unbounded content.

The objective is **not** token reduction — if tokens decrease as a side
effect, that's a bonus. The minimum goal is: **don't send repetitive,
stale, or irrelevant junk**.

## Problem

Crush sends the system prompt, tool descriptions, and context files on
every API call. Much of this content is:

- **Repetitive** — the same editing instructions duplicated 4 times
  across different sections of the system prompt.
- **Stale** — hard-coded "your todo list is currently empty" reminder
  even when a todo list exists.
- **Irrelevant** — MCP instructions for servers not in use, Git/PR
  walkthrough when the user isn't committing, all notebook entries
  regardless of relevance.

## Why prompt caching alone is insufficient

Prompt caching helps but is not the complete solution:

- It may reduce provider compute/cost.
- It usually **still sends the full request body** over the network.
- Cached content is still logically part of the model context, so it can
  still distract the model with irrelevant material.

The real solution is to send **relevant** content. Caching makes
repeated material cheaper; relevance selection removes material that
shouldn't be there at all.

## Provider caching

All major providers support prompt-prefix caching, not just Anthropic:

- OpenAI: automatic prompt-prefix caching for supported models.
- Gemini: implicit caching for recent models, plus explicit
  cached-content APIs.
- Anthropic: explicit/automatic prompt-prefix caching.

Crush already adds Anthropic-compatible cache controls to the last tool
(`agent.go:727-730`), system messages, and recent messages
(`agent.go:893-909`). It also sends deterministic session-affinity
headers (`agent.go:1609-1619`).

Stable prefix caching is still worth optimizing, but the bigger wins
come from removing irrelevant content entirely.

## What this plan does NOT touch

- Conversation history compaction — covered by `CONTEXT_NOTEBOOK.md`.
- Tool result truncation — already implemented (bash: 30K chars, view:
  200KB, grep: 100 results, etc.).
- Skills XML — already capped at 1024 chars per description, only 3
  builtin skills (~200 tokens total).
- Context-window budgeting — moved to `CONTEXT_WINDOW_SAFETY.md`. This
  plan uses only defensive hard limits where untrusted data can be
  unbounded.

## Out of scope (separate plan)

The following are scope expansion relative to the stated goal and
belong in `CONTEXT_WINDOW_SAFETY.md`:

- Central request-budget manager.
- Mandatory-section overflow handling.
- Context-file hard/soft token budgets.
- Whole-request fit calculations.
- Output-token and safety-margin allocation.
- Unknown context window fallback.
- Raw turn protection (compacted vs uncompacted).

Context-window budgeting solves overflow, but it does not directly
solve:

- Repeated bytes over the network.
- Irrelevant data distracting the model.

This plan keeps only simple defensive maximums where untrusted data can
be unbounded: MCP instructions, notebook injection, and recursively
loaded context files.

---

## Content classification

All prompt content falls into classes with different handling:

| Class                         | Examples                                      | Handling                         |
| ----------------------------- | --------------------------------------------- | -------------------------------- |
| Safety and permission rules   | Critical rules, security policy               | Always send                      |
| Core tool behavior            | view, edit, grep, glob, bash, todos, question | Always send, compactly           |
| Project instructions          | AGENTS.md, CRUSH.md policy                    | Always send if authoritative     |
| Git/PR workflow               | Commit format, PR template                    | Load only for Git/PR tasks       |
| Specialized tool instructions | notebook, MCP tools                           | Load when tool becomes available |
| Historical project knowledge  | architecture docs, design decisions           | Retrieve by relevance            |
| Old conversation details      | Past tool results, old decisions              | Notebook/retrieval               |
| Examples and tutorials        | JSON examples, bad/good pairs                 | Remove or load on demand         |

The immediate PRs (1-4) clean up always-sent content and conditionally
include content based on relevance. The deferred phases move content
from "always send" to "load on demand" or "retrieve by relevance."

---

## PR 1: Measure pollution

**No content removal.** Measure only what is necessary to prioritize
work.

### 1a: Logical prompt composition

`preparePrompt` (`agent.go:1639`) never sees the system prompt, tool
list, or MCP instructions — those are built in `Run` (`agent.go:706-738`).
Capture static measurements (system prompt, tools, MCP instructions) in
`Run`, and per-step measurements (messages, tools, notebook) in
`PrepareStep` (`agent.go:863-919`) where `prepared.Messages` and
`prepared.Tools` are finalized.

Log per request:

- System-prompt bytes (captured in `Run`).
- Built-in tool-schema bytes (captured in `Run` and `PrepareStep`).
- MCP tool-schema bytes (captured in `Run` and `PrepareStep`).
- MCP instruction bytes (captured in `Run`).
- Notebook bytes (captured in `PrepareStep`).
- Raw-history bytes (captured in `PrepareStep`).

Token estimates and cache-hit optimization are secondary because the
primary target is transmitted and irrelevant content. Include them
where cheap, but do not block on exact token attribution.

Crush already records `CacheCreationTokens` and `CacheReadTokens`
(`agent.go:2216-2233`). Expose these per session/turn, not just as
aggregate cost.

**Token accounting note**: `updateSessionTokenCounters` computes prompt
tokens as `InputTokens + CacheReadTokens` (`agent.go:2359`), while the
title/summarize path computes them as `InputTokens + CacheCreationTokens`
(`agent.go:2232`). These disagree on which cache tokens count toward
prompt tokens. Define a normalized `promptTokens` calculation and log
the raw provider numbers alongside it.

### 1b: Transport payload measurement

`PromptSection.Bytes` measures content strings, not bytes transmitted
over the network. The serialized request also includes JSON field
names, escaping, tool schemas, provider options, base64 attachments,
message-role wrappers, and HTTP headers.

`preparePrompt` and `PrepareStep` cannot observe the final
provider-specific HTTP payload. Add measurement at the transport layer:

- Final serialized request-body bytes where available.
- Response bytes.

Implement via an HTTP `RoundTripper` or provider transport wrapper
that wraps the existing HTTP client and records byte counts. A normal
`RoundTripper` can count request-body bytes consumed and response-body
bytes read. It cannot necessarily observe lower-level HTTP/2 framing or
bytes after compression beneath the transport wrapper.

**Provider capability**: Not all provider clients expose a usable HTTP
transport. Use a capability/interface and report "unavailable" where
instrumentation is not possible. Do not require all providers to support
transport instrumentation in the first PR.

### 1c: Section-level telemetry

Build and measure each section independently. Context files are
currently rendered into a single system-prompt string before `Run`
receives it (`prompt.go:83-98`). `Run` cannot separately measure
`core_policy`, `project_policy`, and other embedded sections unless
`prompt.Build` returns structured metadata:

```go
type BuiltPrompt struct {
    Text     string
    Sections []PromptSection
}
```

Similarly, classify built-in versus MCP tools during `buildTools`; do
not attempt to infer their origin later from flattened
`[]fantasy.AgentTool`.

```go
type PromptSection struct {
    Name       string
    Content    string
    Required   bool
    Bytes      int
    EstTokens  int64 // populated by approxTokenCount, same heuristic used elsewhere
    CacheClass CacheClass
}
```

`EstTokens` is populated by the same `approxTokenCount` heuristic used
by the context-file and notebook budget logic (see
`CONTEXT_WINDOW_SAFETY.md`). Storing the estimate on the section means
`prepareRequest` can sum per-section token counts directly instead of
re-estimating each section — avoiding duplicate work and inconsistent
estimates between the telemetry path and the budget path. `Bytes` is
kept for transport-layer measurement (which is byte-oriented) and for
sanity-checking `EstTokens`.

Possible sections:

```
core_tools
optional_tools
core_policy
project_policy
mcp_instructions
notebook_recent
notebook_relevant
raw_history
current_prompt
```

### 1d: Deterministic ordering

1. **Context files**: sort by path within priority groups. Currently
   `loadContextFiles` (`prompt.go:152-163`) returns a map keyed by path
   with nondeterministic iteration. Define precedence groups:
   1. Global context (e.g., `~/.config/crush/AGENTS.md`)
   2. Project context (e.g., `./AGENTS.md`)
   3. More-specific directory context

   Sort by normalized path only within each priority group. Otherwise
   lexical ordering may accidentally place broad instructions after
   more-specific ones.

2. **Built-in tools**: `buildTools` already sorts all tools by name
   (`coordinator.go:949-951`), but it sorts built-in, LSP, notebook, and
   MCP tools **together** into one alphabetical list. If an MCP server
   connects or its tool set changes, every built-in tool that comes
   after the new tool alphabetically shifts position, breaking the
   stable built-in prefix. PR 5's stable prefix work needs a
   **two-partition sort**:
   core built-ins sorted first, then optional/MCP tools sorted, with
   the cache breakpoint on the last core built-in. Origin marking is
   needed for the full built-in / LSP / notebook / MCP split, which
   `fantasy.AgentTool.Info()` does not currently expose — but MCP tools
   alone are already distinguishable today via type assertion to
   `*tools.Tool`, which exposes `MCP()` and `MCPToolName()`
   (`internal/agent/tools/mcp-tools.go:62-66`). A {everything-else} vs
   {MCP} partition can ship without touching fantasy.

3. **MCP servers**: MCP instructions are appended from
   `mcp.GetStates()` (`agent.go:713`). Sort by server name before
   concatenating.

PR 1d covers deterministic ordering. PR 5 (Stable prefix optimization)
builds on PR 1d by adding cache breakpoints to the stable ordering — it
is a continuation, not a replacement.

### Verification

- Run a 5-turn session and confirm the log shows per-component byte
  counts and transport bytes per turn.
- Verify transport wrapper records request/response bytes where the
  provider supports it; "unavailable" otherwise.
- Verify deterministic ordering by starting two sessions with the same
  config and confirming identical tool and context-file ordering.

---

## PR 2: Remove obvious redundancy

These are **low-risk behavioral prompt changes**, not "zero behavioral
change." They all change what the model sees. Require regression
evaluations, not only Go tests.

### Step 1: Deduplicate system prompt

**File**: `internal/agent/templates/coder.md.tpl`

#### Problem

The "read before editing + exact text matching" instructions are repeated
**4 times** across different sections:

| Section                           | Lines   | What it says                                                                           |
| --------------------------------- | ------- | -------------------------------------------------------------------------------------- |
| `<critical_rules>` #1, #5         | 6, 10   | "READ CONTEXT BEFORE EDITING", "USE EXACT MATCHES"                                     |
| `<editing_files>`                 | 136-186 | Full editing guide: read context, copy exact text, whitespace matters, common mistakes |
| `<whitespace_and_exact_matching>` | 188-215 | Entire section repeats `<editing_files>` whitespace guidance verbatim                  |
| `<error_handling>`                | 256-262 | "old_string not found" — repeats exact matching instructions a 4th time                |

Other overlaps:

- `<communication_style>` (23-53) + `<final_answers>` (341-369) — both define verbosity rules
- `<proactiveness>` (330-339) + `<decision_making>` (99-134) — both say "don't ask, just do it"
- `<workflow>` (61-97) + `<task_completion>` (217-238) — both cover "verify before finishing"
- `<code_conventions>` "check if library exists" (275) overlaps `<workflow>` "search codebase" (65)

#### Caveat

Redundant instructions are not always useless — repetition can increase
adherence. Four copies of exact-edit guidance is excessive, but measure
edit failure rate before and after to confirm no regression.

#### Changes

**Merge `<whitespace_and_exact_matching>` into `<editing_files>`**:

- Delete lines 188-215 (the entire `<whitespace_and_exact_matching>` section)
- The `<editing_files>` section already covers whitespace (lines 168-172) and
  common failures (lines 178-185). Add only the unique content from the deleted
  section:
  - The "func foo() { vs func foo(){" example (line 202)
  - The "If edit fails: try including entire function/block" (line 213)

**Merge `<error_handling>` edit-failure subsection into `<editing_files>`**:

- Delete lines 256-262 (the "Edit tool old_string not found" subsection)
- Move the one unique tip ("Count indentation spaces carefully") into the
  existing "Common mistakes to avoid" list at line 178

**Merge `<final_answers>` into `<communication_style>`**:

- Delete lines 341-369 (the entire `<final_answers>` section)
- Add the unique content to `<communication_style>`:
  - "Adapt verbosity to match work: default <4 lines, up to 10-15 for
    multi-file changes" (one bullet)
  - "What to avoid: preambles, postambles, full file contents" (already
    partially covered by existing bullets — dedupe)

**Merge `<task_completion>` into `<workflow>`**:

- Delete lines 217-238 (the entire `<task_completion>` section)
- The `<workflow>` "Before finishing" section (82-88) already covers
  verification. Add only:
  - "Think before acting: identify all components, consider edge cases"
    (one bullet under "Before acting")
  - "Implement end-to-end: wire fully, no TODOs, update all affected files"
    (one bullet under "While acting")

**Trim `<decision_making>` overlap with `<proactiveness>`**:

- Keep `<decision_making>` (more detailed with examples) and delete
  redundant lines from `<proactiveness>` (332-333, 336-337).
- Keep only the unique parts of `<proactiveness>`:
  - "When asked how to approach → explain first, don't auto-implement"
  - "After completing work → stop, don't explain"
  - "Don't surprise user with unexpected actions"

**Total: approximately 72 lines deduplicated. Measure actual impact with
the real tokenizer, not line-count estimates.**

#### Verification

- Measure rendered prompt bytes/tokens before and after (PR 1 telemetry)
- Run `go test ./internal/agent/...` — system prompt is embedded via
  `//go:embed`, so the template change is picked up at compile time
- Run `go build .` to confirm the embed works
- Manually verify the template still renders (no broken `{{}}` tags)
- Compare task/tool evaluations before and after (regression check)

### Step 2: Trim `bash.md.tpl` tool description

**File**: `internal/agent/tools/bash.md.tpl`

#### Problem

173 lines. The git commit walkthrough (lines 48-114) and PR creation
guide (lines 116-168) are only relevant when the user asks to commit or
create a PR, but they're sent as part of the tool definition on **every**
turn. This is irrelevant content for the vast majority of turns.

#### Changes

Trim verbose examples, but **preserve exact attribution syntax** for
both commits and PRs. The PR template at lines 152-162 conditionally
includes `{{ if .Attribution.GeneratedWith }}` — removing it loses
configuration-dependent behavior.

- `<git_message_quality>` (lines 48-56): Keep — 8 lines, useful rules
- `<commit_messages>` (lines 58-71): **Trim to 4 lines** — keep only
  "concise 1-2 sentence message focusing on why", "first line under 72
  chars", "add body only when needed". Remove the 4 bad/good examples
  (lines 67-70).
- `<git_commits>` (lines 73-114): **Trim to 8 lines** — keep the
  numbered steps, **keep the HEREDOC with attribution template** (lines
  93-107) since it contains configuration-dependent rendered fields.
  Remove the verbose `<commit_analysis>` explanation. Keep "Notes:".
- `<pull_requests>` (lines 116-168): **Trim to 8 lines** — keep the
  numbered steps, **keep the HEREDOC with attribution template** (lines
  152-162) since it contains `{{ if .Attribution.GeneratedWith }}`.
  Remove the verbose `<summary>` explanation and "Important:" footer.
- `<examples>` (lines 170-173): Keep — 4 lines, useful

#### Longer term

Commit/PR policy should not live in the Bash tool definition at all. A
dedicated built-in skill for Git operations is a better boundary: load
it only when the user asks to commit, push, or create a PR. This is
deferred to Phase 1.

**Decision: trim now, move to skill later.** The inline trim in Step 2
reduces pollution immediately. Phase 1 then moves the remaining Git/PR
content into a skill for further reduction. These are sequential, not
alternatives — do both.

#### Verification

- Run `go test ./internal/agent/tools/...`
- Run `go build .`
- Manually test that the model can still create commits with correct
  attribution trailers and PRs with correct `GeneratedWith` behavior
- Compare commit/PR generation quality before and after (regression check)

### Step 3: Trim `question.md` tool description

**File**: `internal/agent/tools/question.md`

#### Problem

98 lines. Two full JSON examples (lines 66-85) are redundant — the
schema already communicates structure. The "When to use" / "When NOT to
use" sections (lines 87-98) partially repeat the intro. The
"Confirmation screen" section (lines 48-57) is mostly UI mechanics, but
its `confirm_title`/`confirm_description` guidance is behavioral and
must be preserved in condensed form.

#### Changes

- **Remove the multi-question JSON example** (lines 74-85) — the single
  example (lines 66-72) is sufficient.
- **Merge "When to use" and "When NOT to use" into a single 4-line list**.
- **Remove the "Confirmation screen" section** (lines 48-57). The
  `confirm_title` and `confirm_description` fields are already in the
  JSON schema (`question.go:23-24`). Keep 1-2 sentences of behavioral
  guidance on writing `confirm_description` (e.g., "Write
  `confirm_description` as a prospective summary of the user's
  answers").
- **Trim "How it works"** (lines 4-8) from 5 lines to 2.

**Keep the semantic rules the schema cannot express**:

- When `yes_no` is appropriate (proposition, not A-vs-B).
- Choice limits (max 5 choices, max 5 questions).
- Required descriptions.
- The automatically provided custom-answer option (don't add "Other"
  manually).

#### Verification

- Run `go test ./internal/agent/tools/...`
- Run `go build .`
- Manually test that the model still asks structured questions correctly
- Compare question quality before and after (regression check)

### Step 4: Replace `system_reminder` with a stable system prompt rule

**File**: `internal/agent/agent.go:1639-1650`, `internal/agent/templates/coder.md.tpl`

#### Problem

A `<system_reminder>` about empty todo lists is injected as a **user
message** on every turn for non-sub-agents (`agent.go:1641-1650`). The
text is hard-coded to "your todo list is currently empty" — which is
wrong if a todo list exists. It's also noise after the first turn.

The same block also runs in the summarization path — `preparePrompt` is
called from the title/summary generator (`agent.go:1463`), so the stale
reminder currently pollutes summarize requests too. Removing the block
fixes both call sites.

#### Why a stable system prompt rule is the right approach

A synthetic reminder sent only on the first request would disappear
from the second request onward because it is not stored in conversation
history. Also, `len(msgs) == 0` means "the session has no messages," not
necessarily "there have been no real user turns." A stable rule in the
system prompt avoids both problems — it is always present and never
makes false claims about the todo list state.

#### Changes

**Remove the synthetic reminder entirely** from `preparePrompt`
(`agent.go:1641-1650`). Replace it with one short, truthful rule in the
stable system prompt (`coder.md.tpl`), inside `<critical_rules>`:

```
16. **USE TODOS FOR MULTI-STEP WORK**: Use the "todos" tool for non-trivial multi-step tasks. Skip it for simple tasks.
```

The synthetic reminder was gated by `!a.isSubAgent` (`agent.go:1641`),
so it only affected the coder agent. `task.md.tpl` sub-agents did not
receive it. Do **not** add the todo rule to `task.md.tpl` — sub-agents
handle focused tasks and don't need todo-list orchestration.

This instruction:

- Participates in prefix caching (stable, never changes between turns).
- Does not falsely claim the todo list is empty.
- Is concise (one rule, not a paragraph).
- Is always present (no first-turn detection needed).

#### Code change

In `agent.go:1641-1650`, remove the entire `if !a.isSubAgent` block:

```go
// Before:
func (a *sessionAgent) preparePrompt(...) {
	var history []fantasy.Message
	if !a.isSubAgent {
		history = append(history, fantasy.NewUserMessage(
			fmt.Sprintf(
				"<system_reminder>%s</system_reminder>",
				`This is a reminder that your todo list is currently empty...`,
			),
		))
	}

// After:
func (a *sessionAgent) preparePrompt(...) {
	var history []fantasy.Message
```

#### Verification

- Run `go test ./internal/agent/...` — no unit test asserts the
  reminder, but the VCR cassettes record it in request bodies and will
  fail to match; see "VCR cassette strategy" below.
- Run `go build .`
- Verify the model still uses the todos tool for multi-step tasks.
- Compare todo usage before and after (regression check).

---

## PR 3: Stop sending unrelated MCP content

This is probably the largest clean architectural win. Do not append
instructions from every connected MCP server. Send instructions only
for MCP servers whose tools are relevant or loaded.

### Problem

All connected MCP servers' `InitializeResult().Instructions` are
concatenated with no limit (`agent.go:711-725`). Unrelated MCP
instructions (Linear, GitHub, Slack, database) influence ordinary coding
tasks.

### Immediate: defensive hard limits

While the conditional loading architecture is being built, add
defensive hard limits to prevent unbounded MCP instructions from
flooding the prompt. Use **tokens** for prompt-content limits,
consistent with the existing codebase (`NotebookMaxTokens`,
`NotebookMaxEntryTokens`, `largeContextWindowThreshold` are all in
tokens):

1. **Hard safety limits** (defensive boundaries, not configurable):

   ```go
   const (
       maxMCPInstructionsPerServer = 1_000   // tokens per server
       maxTotalMCPInstructions      = 4_000   // tokens total
   )
   ```

2. **UTF-8-safe truncation** using `truncateToTokenLimit` (see
   "Defensive helpers" below). Since the caps are in tokens but
   `truncateUTF8Prefix` operates on bytes, use the
   `tokenLimitToBytes` / `truncateToTokenLimit` conversion helpers
   defined in `CONTEXT_WINDOW_SAFETY.md` (Byte/token conversion for
   soft budgets).

3. **Deterministic server ordering** — sort by server name before
   concatenating.

4. **Warning identifying the truncated server** (slog.Warn + TUI status).

5. **Prefer both beginning and end** rather than only the prefix.

6. **Enforcement location**: enforce the hard caps immediately in `Run`
   (`agent.go:711-725`) where MCP instructions are appended to the
   system prompt. When `prepareRequest` from `CONTEXT_WINDOW_SAFETY.md`
   is later implemented, it will subsume this enforcement as part of
   per-step request preparation. Until then, `Run` is the only place
   that sees MCP instructions.

### Architecture goal: conditional MCP instructions

The real fix is to not send MCP instructions by default. Instead:

- Keep a compact MCP server/tool catalog so the model knows what can be
  loaded.
- Send a server's instructions only when one or more tools from that
  server are loaded for the current step.
- Associate instructions with the corresponding server's tools.
- Remove generic onboarding prose from MCP instructions.

This requires session-scoped tool loading (see Phase 1 below).

### Verification

- Run `go test ./internal/agent/...`
- Test with a mock MCP server returning large instructions.
- Verify deterministic ordering by connecting servers in different
  orders and confirming the same output.

---

## PR 4: Relevance-select notebook entries

Instead of sending every notebook entry, select entries based on
relevance to the current task.

### Problem

`buildNotebookMessage` (`agent.go:1787-1833`) fetches all notebook
entries before the boundary turn and renders all of them via
`RenderEntries` (`agent.go:1827`). Sending all entries regardless of
relevance pollutes the model's context.

### Changes

1. **Always retain recent raw turns.** Do not drop raw history to make
   room for notebook entries. Recent context is more important than
   historical notebook content.

2. **Select notebook entries by relevance** rather than sending all
   entries newest-to-oldest. Build a context pack containing:
   - Active todos and unresolved decisions.
   - Entries matching files/symbols in the current request.
   - Entries matching current semantic tags.
   - Recent failures and test results.
   - A small chronological project summary.
   - Additional entries until a simple maximum is reached.

   **First-cut heuristic** (no semantic embedding needed):
   1. Collect explicit file paths from the current user message and any
      active todos. Active todos come from the session's todo list,
      which `buildNotebookMessage` can already reach: the sessionAgent
      holds `a.sessions` (`session.Service`, `agent.go:178`) and
      `buildNotebookMessage` already derives `sessionID` from the
      messages (`agent.go:1791-1798`). Fetch the session once and read
      `session.Todos` (`session.go:60`, `session.Todo` at
      `session.go:34`), keeping only
      items whose `Status` is `pending` or `in_progress`. Extract file
      paths from each todo's `Content` using the existing
      `extractExplicitFilePaths` helper (`agent.go:1843`). Do **not**
      pass a separate todo service — `session.Service` is the source of
      truth and is already in scope. If the session lookup fails or
      returns no todos, fall back to message paths only.
   2. Retrieve notebook entries whose `Tags` or `EntryText` contain
      those paths.
   3. Include entries from the current and previous turn
      (most-recent context).
   4. Fill remaining `maxNotebookInjectionTokens` with newest entries
      not already selected.
   5. Deduplicate by entry ID.

   This is intentionally simple — string matching on paths and tags.
   Phase 2 (Relevance-based context retrieval) replaces this with
   heading/symbol/keyword indexing and semantic retrieval.

3. **Simple maximum as a safety guard.** Not a context-window budget —
   just a defensive cap to prevent unbounded notebook injection. Use
   tokens, consistent with the existing notebook system
   (`NotebookMaxTokens = 100000`):

   ```go
   const maxNotebookInjectionTokens = 12_000 // safety guard
   ```

   This is deliberately a constant, not a config option — unlike
   `NotebookMaxTokens` (`notebook_max_tokens`, default 100000), which is
   a user-tunable retention budget, this cap is a defensive request-size
   guard and should not be user-configurable.

4. **Supersession.** When a newer entry contradicts or supersedes an
   older one, only inject the newest effective fact.

5. **Modify the existing filtering loop** in `buildNotebookMessage`
   (`agent.go:1813-1823`). The existing loop already filters by
   `boundaryTurn` and tracks `turnsWithEntries`. Add relevance-based
   selection after that filtering, then render the selected subset
   instead of all filtered entries.

### Verification

- Run `go test ./internal/agent/...`
- Run `go test ./internal/notebook/...`
- Test with a session that has many notebook entries — verify only
  relevant entries are sent.
- Test with a simple maximum — verify the cap is respected.
- Test that recent raw turns are always retained.

---

## PR 5: Stable prefix optimization

This is the cache/ordering extension of PR 1d. Ensure every request has
this shape, from most-stable to least-stable:

```
stable built-in tools (sorted deterministically)
optional/MCP tools (sorted deterministically)
stable core system prompt
stable project context (context files)
MCP instructions (sorted deterministically)
notebook context
conversation history (raw recent turns)
current user input
```

### Provider serialization constraint

Provider protocols commonly serialize **all tool definitions as one
request section** before system/messages. Crush cannot generally insert
the system prompt between built-in and MCP tools through Fantasy.

Therefore:

- **Stable built-in tool partition first** within the tools section.
- **Optional/MCP tools after built-ins** within the tools section.
- **Deterministic ordering within each partition.**
- **Cache breakpoint after the stable built-in partition** where
  providers permit it. Currently caching marks only the last tool
  (`agent.go:727-730`). If MCP tools change, that last-tool breakpoint
  and everything following it loses cache reuse. Add a breakpoint after
  the last built-in tool so the built-in prefix remains cacheable even
  when MCP tools change.
- **Provider-specific verification** — serialization and caching
  semantics differ.

### Multipart system prompt

A cache breakpoint after stable system/project policy requires
splitting the system prompt into structured blocks:

```text
system block 1: core policy
system block 2: project context
system block 3: volatile environment (date, git status)
system block 4: MCP instructions
```

Apply cache control after block 2 where supported. Crush currently
builds one system-prompt string and passes it through
`fantasy.WithSystemPrompt(systemPrompt)` (`agent.go:734`). A breakpoint
cannot be placed inside that single string.

If Fantasy cannot represent multipart system content, explicitly mark
this optimization **provider-dependent** and defer until Fantasy
supports structured system blocks.

### Rules

1. **Two-partition tool sort.** `buildTools` already sorts all tools by
   name (`coordinator.go:949-951`), but mixes built-in, LSP, notebook,
   and MCP tools together. Split into two partitions: core built-ins
   sorted first, then optional/MCP tools sorted. A full four-way origin
   split requires marking each tool's origin, which
   `fantasy.AgentTool.Info()` does not currently expose — but MCP tools
   can already be partitioned off via type assertion to `*tools.Tool`
   (`MCP()`/`MCPToolName()`, `internal/agent/tools/mcp-tools.go:62-66`),
   which is sufficient for the built-in/MCP cache boundary.

2. **Sort MCP servers deterministically.** Sort by server name before
   concatenating instructions.

3. **Move `<env>` after context files.** The system prompt template
   includes `{{.Date}}` and `{{.GitStatus}}` in the `<env>` block
   (`coder.md.tpl:371-381`). However, project context files
   (`coder.md.tpl:412-422`), global context files
   (`coder.md.tpl:423-434`), and the notebook block
   (`coder.md.tpl:435-447`) all render **after** `<env>`. Because
   `GitStatus` changes between sessions and the prompt is rebuilt per
   session, the context-file section cannot share a prefix cache with
   a previous session. To make project context cacheable, move `<env>`
   after context files or to the very end of the system prompt.

4. **Preserve exact bytes for system/context content between turns.**
   The system prompt is built once and cached on the agent. MCP
   instructions are appended per Run (`agent.go:711-725`). If MCP
   servers connect/disconnect between turns, the system prompt changes
   and cache misses. Consider building the full system prompt (including
   MCP) once and only rebuilding when the MCP server set changes.

5. **Track provider-reported cached tokens** to verify the result per
   provider, not against a universal threshold.

### Verification

- Log cache hit ratio before and after this phase per provider.
- Define provider-specific baselines (not a universal >80% target).
- Confirm cache hit ratio improves for turns 2+ within a session
  (assuming no MCP changes).

---

## Architecture roadmap (deferred phases)

The immediate PRs (1-4) clean up always-sent content and conditionally
include content based on relevance. The following phases move content
from "always send" to "load on demand" or "retrieve by relevance."

### Phase 1: Lazy tool loading (session-scoped)

Tool schemas and descriptions are the largest avoidable static cost
(estimated ~5,030 tokens for built-in tools alone, plus MCP tools —
confirm with PR 1 measurement).

#### Core tool set

Keep a small core tool set always loaded:

```
view, edit, grep, glob, bash, todos, question, tool_search, tool_load
```

**Include `todos` and `question` in the core set.** They are small,
common orchestration tools, and loading them through an extra round trip
would be inefficient. The system prompt permanently tells the model to
use `todos` — it must be available without a `tool_load` call.

#### Tools should be selected, not "budgeted"

Do not say "tools have an 8K budget, so drop whichever schemas do not
fit." Instead say "this task needs edit, view, grep, bash, and GitHub.
Send only those tools."

Selection should be based on capability relevance. A context limit can
act as a final safety check, but it should not drive normal tool
availability.

#### Session-scoped state

`sessionAgent.tools` is shared by all sessions using that agent
(`agent.go:175`). A direct `SetTools` mutation would expose loaded
tools to unrelated sessions and introduce races between concurrent
sessions.

The design needs session-scoped loaded-tool state:

```go
type SessionToolState struct {
    Loaded map[string]struct{}
}
```

Then `PrepareStep` should construct tools using:

```
core tools
+ tools loaded for call.SessionID
+ required tools for the current operation
```

Additional requirements:

- Persist or reconstruct loaded tools when restoring a session.
- Define what happens when MCP disconnects (loaded tools become
  unavailable — handle gracefully).
- Ensure a tool loaded during one step appears in the next `PrepareStep`.
- Keep tool-call history valid when a schema is no longer currently
  exposed (the model may still reference a previous tool call).
- Avoid unnecessary round trips for common tools (todos, question are
  in the core set for this reason).

#### Design

- Send a compact optional-tool catalog initially:
  ```
  github — issues, PRs, reviews
  notebook — historical context retrieval
  mcp:linear — Linear issue operations
  ```
- The model calls `tool_load(["github", "mcp:linear"])`.
- On the following agent step, add those full definitions.
- Keep loaded tools for the rest of the session so schemas remain stable.

#### MCP instructions

Send instructions only when one or more tools from that server are
loaded. Associate instructions with the server's tools. Remove generic
onboarding prose from MCP instructions.

#### Git/PR skill

Move commit/PR instructions out of the Bash tool definition into a
dedicated built-in skill. When the user asks to commit or create a PR:

1. Detect the explicit Git action (not loose keywords like "PR" —
   use explicit command intent or a small deterministic classifier).
2. Attach the Git workflow skill.
3. Include the exact attribution template and repository conventions.
4. Remove it again for unrelated future tasks, or retain it only for
   the current task.

Given the pollution goal, moving Git/PR into a skill is cleaner than
keeping it in the Bash tool. This is the continuation of Step 2's trim —
Step 2 reduces the inline content now, Phase 1 moves the remainder into
a skill.

### Phase 2: Relevance-based context retrieval

**Notebook**: full relevance-based selection (see PR 4 above).

**Context files**: for sources explicitly classified as knowledge (via
config), index by headings, symbols, file paths, and keywords. Retrieve
only relevant sections rather than entire files. For large policy files,
send a compact authoritative summary plus a `read_context` tool.

**Supersession**: track when newer entries supersede older ones. Only
inject the newest effective fact.

### Phase 3: Compact consumed tool results

When notebook mode is enabled, turns before the boundary are already
removed from raw history and represented through notebook entries
(`agent.go:1652-1682`). Therefore, "compact only at the notebook
boundary" is largely the current architecture.

Phase 3 is only needed for:

- Notebook-disabled sessions.
- Tool results that remain inside the raw window.
- Durable artifact retrieval.

#### Critical constraint

**Do not replace a result with an unavailable reference.** Any
compaction must store the full result in durable storage or the database
and provide a valid retrieval ID.

Three options:

1. **Store full results in durable content-addressed storage** and add
   a `read_artifact` tool. The summary references a valid artifact ID.
2. **Compact only at the notebook boundary** — when a turn crosses from
   raw history into notebook territory, losing exact raw context is
   already an explicit policy. Don't compact messages still inside the
   raw history window.
3. **Retain complete results in the database** but send a summary plus
   a valid retrieval ID (session ID + message ID) that can be used to
   fetch the full result via a tool.

Option 2 is the safest for the immediate implementation. Options 1 and
3 require additional infrastructure.

Keep full results only when:

- They are recent (inside the raw history window).
- The current task explicitly references them.
- The model has not yet produced a reliable interpretation.
- Exact output is necessary (compiler output, patches, IDs, line
  numbers).

### Phase 4: Network-specific provider optimization

Only after measuring actual payloads (PR 1):

- Gemini explicit cached-content references.
- Stateful conversation/interaction IDs where providers support them.
- Request compression where the provider accepts it.
- Content-addressed tool-result retrieval.

These can reduce actual network transmission.

Add capability flags to provider adapters:

```go
type ContextCapabilities struct {
    StatefulConversation   bool
    ExplicitCachedContent  bool
    RequestCompression     bool
}
```

Keep the current stateless request builder as the fallback. Stateful
provider APIs can reduce transmitted history but require handling:
branching conversations, retries/failover, model switching, server-side
expiration, privacy/retention, session recovery, and instructions/tools
not inherited by the stateful API.

---

## Defensive helpers

### UTF-8-safe truncation

For MCP instructions that may exceed the defensive hard limit, use
`truncateUTF8Prefix` to trim at a byte boundary while ensuring no
incomplete multi-byte rune is included. First normalize the entire
buffer for invalid UTF-8, then walk back from the cut point to the
nearest rune boundary:

```go
// truncateUTF8Prefix normalizes invalid UTF-8 and trims so that the
// result is valid UTF-8 no longer than maxBytes.
func truncateUTF8Prefix(s string, maxBytes int) string {
    s = strings.ToValidUTF8(s, "")
    if maxBytes >= len(s) {
        return s
    }
    for end := maxBytes; end > 0; end-- {
        if end == len(s) || utf8.RuneStart(s[end]) {
            return s[:end]
        }
    }
    return ""
}
```

The key detail: check `utf8.RuneStart(s[end])` — the first byte
**after** the cut — not `s[end-1]`. If `s[end]` is a continuation byte,
the rune starting before `end` is incomplete and must be excluded by
decrementing `end`.

### Bounded context-file reading

The 256KB hard read limit is a **process-protection** limit measured in
bytes. It guards against unbounded context files (recursively loaded
AGENTS.md, CRUSH.md, etc.) exhausting memory or producing oversized
requests. It is a defensive helper, not a token-budget mechanism — the
soft token-budget logic for context files lives in
`CONTEXT_WINDOW_SAFETY.md`.

```go
const maxContextFileReadSize = 256 * 1024 // 256KB hard process limit

// readBounded reads up to maxContextFileReadSize bytes from path. If
// the file is larger than the hard limit, it returns the truncated
// prefix and truncated=true. The result is always valid UTF-8.
func readBounded(path string) (content string, truncated bool, err error) {
    f, err := os.Open(path)
    if err != nil {
        return "", false, err
    }
    defer f.Close()

    buf := make([]byte, maxContextFileReadSize+1)
    n, err := io.ReadFull(f, buf)
    switch {
    case err == io.EOF || err == io.ErrUnexpectedEOF:
        return truncateUTF8Prefix(string(buf[:n]), n), false, nil
    case err != nil:
        return "", false, err
    default:
        // Read succeeded for max+1 bytes — file is larger than the hard limit.
        return truncateUTF8Prefix(string(buf[:maxContextFileReadSize]), maxContextFileReadSize), true, nil
    }
}
```

`readBounded` enforces only the hard byte limit. The soft token-budget
wrapper `readContextFile` (below) layers required/optional handling on
top of it.

> **Status:** `readContextFile`, `truncateToTokenLimit`, and
> `tokenLimitToBytes` are proposed helpers, not landed code. They were
> removed during review because they had no production callers; they
> should be reintroduced together with the context-file overflow
> handling in `CONTEXT_WINDOW_SAFETY.md` that consumes them. Only
> `readBounded` and `truncateUTF8Prefix`/`truncateUTF8Suffix` are
> implemented today.

### Context-file soft-budget wrapper (proposed)

`readBounded` knows nothing about required vs. optional files or token
budgets. The soft-budget logic that decides whether to prompt, error, or
truncate belongs with the context-file overflow handling, but the
wrapper itself is a defensive helper used by the context-file loader. It
calls `readBounded`, estimates tokens, and applies the required/optional
policy from `CONTEXT_WINDOW_SAFETY.md`'s overflow table:

```go
// readContextFile reads a context file enforcing both the hard byte
// limit (via readBounded) and a soft token budget. required controls
// overflow behavior:
//   - required, over soft budget: prompt interactively, or error in
//     non-interactive mode (crush run).
//   - optional, over soft budget: truncate via truncateToTokenLimit,
//     or exclude according to config.
//
// approxTokenCount and truncateToTokenLimit are defined in
// CONTEXT_WINDOW_SAFETY.md (Byte/token conversion for soft budgets).
// interactive is false during startup and `crush run`.
func readContextFile(path string, softTokenLimit int, required bool, interactive bool) (content string, truncated bool, err error) {
    raw, hardTruncated, err := readBounded(path)
    if err != nil {
        return "", false, err
    }
    if approxTokenCount(raw) <= softTokenLimit {
        return raw, hardTruncated, nil
    }
    if required {
        if interactive {
            // Prompt the user: keep full content, truncate to soft
            // budget, or skip the file. See CONTEXT_WINDOW_SAFETY.md
            // "Context-file overflow handling".
            return promptContextFileOverflow(path, raw, softTokenLimit)
        }
        return "", false, fmt.Errorf(
            "required context file %s exceeds soft token budget (%d tokens); "+
                "reduce the file or raise the budget", path, softTokenLimit,
        )
    }
    // Optional file over budget: truncate (or exclude per config).
    return truncateToTokenLimit(raw, softTokenLimit), true, nil
}
```

The interactive prompt and non-interactive error policy are specified
in `CONTEXT_WINDOW_SAFETY.md` (Context-file overflow handling). The
wrapper is listed here because it is the defensive entry point used by
the context-file loader in `prompt.go`; the budget values and overflow
table it consults live in `CONTEXT_WINDOW_SAFETY.md`.

---

## Implementation order

### Immediate

| PR  | Step                                 | Risk   | What it fixes                        | Ready?                                                                                |
| --- | ------------------------------------ | ------ | ------------------------------------ | ------------------------------------------------------------------------------------- |
| 1   | Measurement + deterministic assembly | Low    | Enables measurement, cache stability | Nearly — needs `BuiltPrompt`; MCP partition needs no fantasy changes (type assertion) |
| 2   | Step 1: Dedupe sys prompt            | Low    | Repetitive content                   | Yes                                                                                   |
| 2   | Step 2: Trim bash.md.tpl             | Low    | Irrelevant Git/PR content            | Yes                                                                                   |
| 2   | Step 3: Trim question.md             | Low    | Redundant examples                   | Yes                                                                                   |
| 2   | Step 4: Replace reminder             | Low    | Stale/false reminder                 | Yes                                                                                   |
| 3   | MCP defensive hard limits            | Low    | Unbounded MCP content                | Yes (isolated)                                                                        |
| 3   | Conditional MCP instructions         | Medium | Irrelevant MCP content               | Depends on Phase 1                                                                    |
| 4   | Relevance-select notebook entries    | Medium | Irrelevant notebook content          | Yes — first-cut heuristic (path/tag matching)                                         |
| 5   | Stable prefix optimization           | Medium | Cache loss on MCP tool churn         | MCP partition via type assertion; multipart system prompt is provider-dependent       |

Steps 1-4 are **low-risk behavioral prompt changes** (not "zero
behavioral change" — they all change what the model sees). Require
regression evaluations, not only Go tests.

### Recommended priority (including deferred phases)

1. PR 1: Measure pollution
2. PR 2: Remove obvious redundancy (Steps 1-4)
3. PR 3: Stop sending unrelated MCP content (defensive limits first,
   then conditional loading)
4. PR 4: Relevance-select notebook entries
5. PR 5: Stable prefix optimization (two-partition tool sort, `<env>`
   relocation, cache breakpoints)
6. Phase 1: Session-scoped lazy MCP/tool loading
7. Phase 1: Conditional MCP instructions (with server's tools)
8. Phase 1: Git/PR on-demand skill
9. Phase 2: Relevance-based context-file retrieval
10. Phase 3: Compact consumed tool results (at notebook boundary first)
11. Phase 4: Network-specific provider optimization

The highest-value architectural change is **session-scoped lazy
MCP/tool loading**, followed by **conditional MCP instructions**. Use
PR 1 measurement data to decide whether tool definitions, MCP content,
notebook entries, or raw results produce the most actual network and
attention waste.

## Testing

After all immediate PRs:

1. `go build .` — confirm compilation
2. `go test ./internal/agent/...` — agent and prompt tests
3. `go test ./internal/agent/tools/...` — tool tests
4. `go test ./internal/notebook/...` — notebook tests
5. `task lint` — linting
6. Verify PR 1 instrumentation shows per-component byte counts and
   transport bytes (where available) per turn.
7. Verify deterministic ordering by starting two sessions with the same
   config and confirming identical output.
8. Manual: start a session, verify the system prompt renders correctly,
   verify tools still work (edit, bash, question, commit)
9. Manual: test with an MCP server that has verbose instructions —
   verify defensive limits apply and warning is surfaced.
10. Regression: compare task completion, edit failure rate, commit/PR
    quality, and question quality before and after PR 2.
11. Manual: test notebook relevance selection — verify only relevant
    entries are sent and recent raw turns are retained.

### VCR cassette strategy

`internal/agent/testdata/TestCoderAgent/deepseek-v4/*.yaml` record
request bodies that include the complete system prompt and the
`<system_reminder>` user message.

`charm.land/x/vcr` matches on **request body**, not just method+URL.
Its matcher (`matcher.go`) compares normalized bodies and falls back to
deep JSON equality (`reflect.DeepEqual` after unmarshal) to tolerate
key reordering. Any change to the system prompt or message list —
Steps 1 and 4 included — breaks cassette matching. The recorder runs in
`ModeRecordOnce` by default, so re-recording means deleting the
affected cassette YAMLs and re-running the tests (or running with
`vcr.WithMode(recorder.ModeRecordOnly)`).

Before implementing Steps 1 and 4:

1. Re-record cassettes after the prompt changes (delete
   `internal/agent/testdata/TestCoderAgent/**/*.yaml` and re-run), or
   relax matching to method+URL for these tests.
2. Re-recorded cassettes only prove the request shape changed as
   intended — they replay canned responses and give no behavioral
   regression signal. Behavioral regression requires the eval harness
   below, not `go test`.

### New tests required

`internal/agent/prompt/` currently contains only `prompt.go` with no
test files. Running `go test ./internal/agent/prompt/...` will pass with
zero tests. Explicitly add new tests for:

- `readBounded` (defined in this document, Defensive helpers →
  Bounded context-file reading): truncation detection, UTF-8
  preservation, file-larger-than-limit, file-smaller-than-limit, read
  error.
- `readContextFile` (defined in this document, Defensive helpers →
  Context-file soft-budget wrapper): required-file over-budget errors
  in non-interactive mode, optional-file over-budget truncates,
  under-budget passthrough, hard-limit truncation propagates.
- `truncateToTokenLimit` (defined in `CONTEXT_WINDOW_SAFETY.md`,
  Byte/token conversion section): verify result token count does not
  exceed the limit, dense-token shrink behavior, very small
  tokenLimit (e.g. 1 with CJK) does not return empty string.
- Precedence-group ordering: global → project → directory-specific.
- `truncateUTF8Prefix`: valid UTF-8 passthrough, trailing incomplete
  multi-byte rune, invalid UTF-8 normalization, maxBytes boundary,
  empty string, maxBytes=0.
- `BuiltPrompt` section metadata: verify sections are populated
  correctly with names and byte counts.

### Regression eval harness

Go tests alone cannot verify that prompt changes don't regress model
behavior. Build a separate eval harness that:

- Runs a set of representative tasks (edit, multi-file refactor, commit,
  PR creation, question asking, multi-step todo usage).
- Compares model output quality before and after prompt changes.
- Reports edit failure rate, commit attribution correctness, question
  format compliance, and todo usage rate.
