# ACP (Agent Client Protocol) Implementation Plan

## 1. Overview

ACP standardizes editor↔agent communication over JSON-RPC 2.0 (stdio default).
Crush acts as an **ACP Server** (Zed, JetBrains, Neovim) and optionally as an
**ACP Client** (driving external agents). Spec: https://agentclientprotocol.com/protocol/v1/

### Library choice: `github.com/coder/acp-go-sdk` v0.13.5

Selected SDK (Coder, most active, complete v1 typed model, built-in stdio
connection + streaming helpers). `Tangerg/acp` v0.2.4 (schema `schema-v1.21.0`,
Go 1.25+) is used **as a test-only fixture** (Zed 1.17.2 transcript + 154
cross-SDK fixtures), not as the runtime SDK.

### Work split

| Responsibility | Handled by | Notes |
|---|---|---|
| Protocol types & validation | SDK (`acp.*`) | 170+ v1 structs/enums/unions |
| JSON-RPC transport & framing | SDK | stdio loop, ID correlation |
| Cancellation routing | SDK | `session/cancel` + `$/cancel_request` → `context.Context` |
| Outbound streaming helpers | SDK | `UpdateAgentMessageText`, `UpdateAgentThoughtText`, `UpdatePlan`, `StartToolCall`, … |
| Outbound client RPC | SDK | `RequestPermission`, `ReadTextFile`, `CreateTerminal`, … |
| Crush Server Adapter | `internal/acp` | implements `acp.Agent`, maps to `app.App`/`coordinator` |
| PubSub Event Bridge | `internal/acp/event_bridge.go` | subscribes Crush events → `SessionUpdate` |
| Permission Bridge | `internal/acp/permission*.go` | `permission.Decision` → `conn.RequestPermission` |
| CLI Integration | `internal/cmd` | `crush acp` (server), `crush acp connect` (client) |
| Client Track | `internal/acp/client.go` | `acp.Client` callbacks to drive external agents |

## 2. Architecture

```
IDE / Client (Zed, JetBrains, Neovim)
   ^  JSON-RPC 2.0 over stdio (coder/acp-go-sdk)
   v
acp.AgentSideConnection
   ^  acp.Agent interface (Initialize, NewSession, Prompt, Cancel, ...)
   v
internal/acp/server.go  ->  coordinator.Run / session.Service (SQLite)
   +-- Event Bridge: assistant text -> agent_message_chunk
   +-- Permission Bridge: RequestPermission <-> allow/deny
   +-- Client-delegated fs/terminal (when client capabilities permit)

EXTERNAL ACP AGENT (client mode)
crush acp connect <agent>  ->  acp.ClientSideConnection  ->  internal/acp/client.go
   +-- SessionUpdate fan-out (per-session channel)
   +-- RequestPermission (auto-allow / first-option default)
   +-- ReadTextFile/WriteTextFile (os.Read/WriteFile)
   +-- Terminals: stubbed ("not supported")
```

## 3. Specification Requirements — Mandatory vs Optional

Status legend: ✅ done · ⚠️ partial/stub · ❌ not implemented.
"MUST/SHOULD/MAY" follow the v1 spec wording.

### 3.1 Initialization (`initialize`, `authenticate`, `logout`)

| Requirement | Level | Crush status |
|---|---|---|
| Respond with chosen `protocolVersion` + `agentCapabilities` | MUST | ✅ |
| Provide `agentInfo` (name/title/version) | SHOULD (required in future) | ✅ sets name+title+version (`Title:"Crush"`) |
| `authMethods` present in response (default `[]`) | MUST (field present) | ✅ empty slice → serializes as `[]` |
| `loadSession` capability | OPTIONAL | ✅ `true` |
| `promptCapabilities` (image/audio/embeddedContext) | OPTIONAL (MUST support Text+ResourceLink in prompts regardless) | ✅ all `true` |
| `mcpCapabilities` (http/sse) | OPTIONAL | ✅ empty (no MCP transport) |
| `sessionCapabilities` (close/list/resume/delete/additionalDirectories) | OPTIONAL | ✅ `close`, `list`, `additionalDirectories`, `delete` advertised; `resume` intentionally not advertised (see §4.1) |
| `auth.logout` capability | OPTIONAL | ✅ advertised |
| `authenticate` method | MUST exist; return `auth_required`/error if unused | ⚠️ stub "auth not supported" |
| `logout` method | MUST exist if `auth.logout` advertised | ✅ success no-op (Crush has no auth session) |

### 3.2 Session lifecycle methods

| Method | Level | Crush status |
|---|---|---|
| `session/new` | MUST | ✅ creates SQLite session, returns `sessionId` |
| `session/prompt` | MUST | ✅ runs coordinator, returns `StopReason` |
| `session/cancel` (notification) | MUST | ✅ cancels coordinator |
| `session/update` (notifications) | MUST | ✅ message/thought/tool_call/tool_call_update/usage emitted (see §3.3) |
| `session/close` | OPTIONAL (advertise `sessionCapabilities.close`) | ✅ implemented + advertised |
| `session/list` | OPTIONAL (advertise `sessionCapabilities.list`) | ✅ implemented + advertised; `cwd` set, `additionalDirectories` echoed back |
| `session/load` | OPTIONAL (advertise `loadSession`) | ✅ implemented (`server.go` `LoadSession`); replays history as `user_message_chunk`/`agent_message_chunk` before responding (R1/R6) |
| `session/resume` | OPTIONAL (advertise `sessionCapabilities.resume`) | ❌ returns error, capability not advertised (deliberate — see §4.1 O5) |
| `session/delete` | OPTIONAL (advertise `sessionCapabilities.delete`) | ✅ implemented + advertised (`UnstableDeleteSession`) |
| `session/set_mode` | OPTIONAL (legacy; prefer config options) | ✅ echoes `modeId` + emits `current_mode_update` (no real mode switch) |
| `session/set_config_option` | OPTIONAL | ✅ applies thinking/model, returns the complete `configOptions` list (spec MUST) |

### 3.3 Prompt turn & streaming (`session/update` types)

All are **notifications** carried in `{"sessionId", "update": {"sessionUpdate": <type>, ...}}`.
The spec distinguishes `session/load` (MUST replay history via these) from
`session/resume` (MUST NOT replay).

| Update type | Level | Crush status |
|---|---|---|
| `user_message_chunk` | agent MUST emit on `session/load` replay; SHOULD NOT echo on live prompt | ✅ emitted during load replay (`UpdateUserMessageText`) |
| `agent_message_chunk` | baseline | ✅ emitted on happy path (`subscribeMessages`) |
| `agent_thought_chunk` | SHOULD (reasoning) | ✅ emitted via `streamThoughts` (S3) |
| `tool_call` | SHOULD | ✅ emitted on tool start (S1) |
| `tool_call_update` | SHOULD (`pending`/`in_progress`/`completed`/`failed`) | ⚠️ emitted on status change; only `in_progress`/`completed` used — `pending`/`failed` never sent (see §3.5) |
| `plan` | SHOULD | ❌ not emitted (S4 open) |
| `available_commands_update` | MAY | ✅ emitted on `session/new` (S8) |
| `current_mode_update` | MAY | ✅ emitted on `session/set_mode` (S9) |
| `config_option_update` | MAY | ✅ emitted on config change (S9) |
| `session_info_update` | MAY (ties to `session/list`) | ✅ emitted on `session/new` (S9) |
| `usage_update` | MAY (context + cost) | ✅ emitted at run completion (O13) |

`messageId` (per-message opaque id; chunks sharing it belong to one message) is a
**MAY** field on `agent_message_chunk`/`user_message_chunk` — **Crush never sets
it** (SDK v0.13.5 still marks `MessageId` UNSTABLE; track on SDK bump).

### 3.4 Content blocks (prompts & outputs)

| Block | Level | Crush status |
|---|---|---|
| `text` | MUST | ✅ |
| `resource_link` | MUST | ✅ rendered inline by `buildPrompt` |
| `image` | OPTIONAL (gated by `promptCapabilities.image`) | ✅ base64-decoded to `message.Attachment` on input (O7); not streamed on output |
| `audio` | OPTIONAL | ✅ base64-decoded to `message.Attachment` on input (O7); not streamed on output |
| `resource` (embedded) | OPTIONAL (gated by `embeddedContext`) | ✅ parsed into prompt (O7) |
| `annotations` on blocks | OPTIONAL | ❌ ignored |

### 3.5 Tool calls

| Requirement | Level | Crush status |
|---|---|---|
| Emit `tool_call` + `tool_call_update` during execution | SHOULD | ✅ `StartToolCall`/`UpdateToolCall` from the event bridge (S1) |
| Tool kinds: `read/edit/delete/move/search/execute/think/fetch/switch_mode/other` | OPTIONAL taxonomy | ✅ `toolKindFor` maps Crush tools to all kinds incl. `switch_mode` |
| Tool content: `content` / `diff` / `terminal` | SHOULD | ⚠️ `content` (raw input as text) only; `diff`/`terminal` not emitted |
| `toolCallId`, `name`, `title`, `kind`, `status`, `locations`, `rawInput`, `rawOutput` | `title` required, rest optional | ⚠️ id/kind/status/content sent; `title` carries the tool's programmatic name (not human-readable). No `name` (no SDK helper), `locations`, `rawInput`, or `rawOutput` |
| `session/request_permission` (4 option kinds: `allow_once`/`allow_always`/`reject_once`/`reject_always`) | MUST when needed | ✅ bridge implemented |
| Client MUST respond `cancelled` to pending permission on `session/cancel` | MUST (client-side duty) | ⚠️ applies to the peer client; agent-side only aborts + returns `cancelled` stop reason |

### 3.6 Elicitation (`elicitation/create`, `elicitation/complete`)

OPTIONAL (gated by `clientCapabilities.elicitation.{form,url}`). Crush: ❌ not
implemented on either side. Spec essentials:
- `mode` discriminator is **required** (`form`/`url`); no implicit form default.
- Form: restricted JSON Schema in `requestedSchema`; MUST NOT request secrets.
- URL: unique `elicitationId` + `elicitation/complete` notification; when URL
  mode is required but unsupported the Agent MUST NOT downgrade to form mode
  (url→form fallback prohibited); client MUST show full URL + consent.
- Outcomes: `accept`/`decline`/`cancel` (not `cancelled`).

### 3.7 File system (`fs/read_text_file`, `fs/write_text_file`)

OPTIONAL (gated by `clientCapabilities.fs`). Crush (client track): ✅ real
`os.ReadFile`/`os.WriteFile` with `line`/`limit` and mkdir-parent. Server track
delegates to the connected client via the SDK.

### 3.8 Terminals (`terminal/*`)

OPTIONAL (gated by `clientCapabilities.terminal`). Crush client track: ⚠️ all
five methods (`create`/`output`/`wait_for_exit`/`kill`/`release`) stubbed
"not supported". Agent MUST `release` terminals it creates.

### 3.9 Cancellation

- `session/cancel` is a **notification** (no response). ✅ cancels coordinator.
- `$/cancel_request` protocol-level cancel. ❌ not handled explicitly (relies on
  SDK context cancellation).
- On abort the agent MUST catch errors and return `cancelled` stop reason, never
  an error. ✅ `Prompt` maps `ctx.Err()` → `StopReasonCancelled`.
- Pending `request_permission` MUST be answered `cancelled` on cancel. ⚠️ unverified.

### 3.10 Capability advertising summary

Advertised today: `loadSession:true`, `promptCapabilities{image,audio,embeddedContext:true}`,
`sessionCapabilities{close,list,additionalDirectories,delete}`, `auth.logout`. **Missing
optional:** `sessionCapabilities.resume`, `authMethods:[]` (configured but serializes per SDK defaults).

### 3.11 Extensibility (`_meta`)

Every type has a `_meta` field. ❌ Crush does not propagate it. Reserved root
keys `traceparent`/`tracestate`/`baggage` are for W3C trace context. Custom
capabilities advertised via `_meta` on capability objects; custom methods start
with `_`.

### 3.12 MCP servers

`session/new`/`load`/`resume` accept `mcpServers`. The spec makes **stdio a MUST
baseline for every Agent**; HTTP and SSE are optional (`mcpCapabilities.http`/
`.sse`, and SSE has since been deprecated by the MCP spec). ❌ Crush ignores
`mcpServers` in `session/new`/`load` (no per-session connect — `mcp.Initialize`
is startup-only) and advertises empty `mcpCapabilities`. Because stdio is a MUST
baseline, this is a genuine conformance gap (O9), not a purely optional omission.

### 3.13 Conformance gaps found in spec verification (2026-09-17)

Scraped the v1 spec end-to-end; items where the current implementation still
diverges from a spec requirement (independent of the §4 completion checkboxes):

- **Tool status lifecycle.** Agent never sends `pending` (first report is already
  `in_progress`) and never sends `failed` — a finished-but-errored tool is reported
  `completed`. Spec status enum is `pending`/`in_progress`/`completed`/`failed`.
- **Tool `title`/`name`/`locations`/`rawInput`.** `title` (required) is populated
  with the programmatic tool name, not a human-readable description; `name`,
  `locations`, and `rawInput` are not set (the SDK's own `read`/`edit` helpers
  show `ToolCallStatusPending` + `WithStartLocations` + `WithStartRawInput`).
  `diff`/`terminal` tool content never emitted.
- **Config option `category`.** `model`/`thinking` options omit `category`
  (`model`/`thought_level`); optional but improves client UX.
- **Boolean option client-capability gate.** Spec: Agent MUST NOT emit
  `type:"boolean"` options unless the client advertises
  `session.configOptions.boolean`. **SDK-blocked:** pinned `acp-go-sdk` v0.13.5
  `ClientCapabilities` has no `session` field, so the gate can't be read yet.
  Revisit on SDK bump (same class as `messageId`/elicitation).
- **MCP stdio connect** (see §3.12) — MUST baseline not met.
- **`session/load` replay completeness.** Replays text only; tool calls/thoughts/
  plan are not replayed, though spec says replay the *entire* conversation.
- **`$/cancel_request` / error `-32800`.** Relies on SDK context cancellation;
  Crush does not explicitly emit the `-32800` Request-Cancelled error.
- **`_meta` passthrough** + reserved W3C trace keys (see §3.11) — not propagated.
- **Terminal authentication** (`clientCapabilities.auth.terminal`, `type:"terminal"`
  auth methods) — not modelled by the pinned SDK; not handled.

## 4. Implementation Roadmap (current state)

Phases reflect **verified code state** on branch `feature/acp-client`.

### Server — done
- [x] **Phase 1** SDK integration + transport (`server.go`, `NewServer`).
- [x] **Phase 2** Agent methods: `Initialize`, `NewSession`, `Prompt`,
  `Cancel`, `ListSessions`, `CloseSession`; stubs `ResumeSession`,
  `SetSessionMode`, `SetSessionConfigOption`.
- [x] **Phase 3** Permission bridge (4 option kinds, `allow_always` cache).
- [x] **Phase 4** CLI `crush acp` (headless workspace, stderr logging, SIGINT).
- [x] **Phase 5** Tests: `TestServerStartEndToEnd`, permission roundtrip,
  conformance (moved to `test/acp/spec/`), Tangerg interop, golden.
- [x] **Event bridge**: `agent_message_chunk` wired on the happy path
  (`subscribeMessages`); `AgentError`/`ReAuthenticate`→text chunk;
  `RunComplete` cancelled/error→text chunk.

### Client — done
- [x] **Phase 6** `acp.Client` callbacks; subprocess spawn; `Subscribe`.
- [x] **Phase 7** Client unit tests.
- [x] **Phase 8** `crush acp connect` CLI + interactive loop (prints `StopReason`).

### Roadmap: remaining spec requirements (prioritized)

Ordered **required-first**. Each item cites the spec clause and target files.
"MUST / SHOULD / MAY" follow the v1 spec wording. Completion reflects the
current code state from §3.

**Tier 1 — Required (MUST): conformance & correctness**
- [x] **R1. `session/load` consistency** — capability is advertised `true` but the
  method errors. Either implement history replay (`user_message_chunk` +
  `agent_message_chunk`) or set `loadSession:false`. (`agent.go` `ResumeSession`,
  `event_bridge.go`.) Test: `TestConformanceSessionLoad`.
- [x] **R2. `ListSessions` required fields** — populate `SessionInfo.cwd` (MUST,
  currently `""`) and `updatedAt`; advertise `sessionCapabilities.list`.
  (`agent.go` `ListSessions`.)
- [x] **R3. `Initialize` `authMethods`** — serialize as `[]`, not `null`
  (`json:"authMethods"` is non-omittable). (`agent.go` `Initialize`.)
- [x] **R4. Cancellation semantics** — on `session/cancel`, answer pending
  `request_permission` with the `cancelled` outcome and ensure aborts map to
  `StopReasonCancelled` (never an error). (`event_bridge.go`, `permission.go`.)
- [x] **R5. `$/cancel_request`** — ensure SDK-level request cancellation cascades
  and terminates the prompt turn cleanly.
- [x] **R6. `session/update` baseline on replay** — `session/load` MUST stream the
  full history before its response (superset of R1).

**Tier 2 — Recommended (SHOULD): core UX**
- [x] **S1. Tool progress** — emit `tool_call` + `tool_call_update`
  (in_progress/completed/failed; `kind`, `content`/`diff`/`terminal`, `locations`,
  `rawInput`/`rawOutput`). Most visible "frozen agent" gap. (`event_bridge.go`.)
  Test: `TestPromptStreamsToolCalls`.
- [ ] **S2. `user_message_chunk` live-turn echo** on `session/prompt`. → **NOT ECHOED** (I-1 race; the client already holds the input). Still emitted on `session/load` replay — see §4.1 S2.
- [x] **S3. `agent_thought_chunk`** for reasoning deltas. (`event_bridge.go`.)
- [ ] **S4. `plan` updates** — each notification is a FULL replace. (`event_bridge.go`.)
- [ ] **S5. `messageId`** on chunks (SDK field currently UNSTABLE; set once the SDK ships it).
- [ ] **S6. Elicitation** — `elicitation/create` + `elicitation/complete`
  (form+url, `accept`/`decline`/`cancel`, unique `elicitationId`, no url→form
  downgrade, no secrets in form). Client callback + server stub. (`client.go`, `agent.go`.)
  Test: `TestElicitationFormMode`.
- [ ] **S7. `_meta` passthrough** on message paths; reserve W3C trace keys
  `traceparent`/`tracestate`/`baggage`. (`agent.go`, `event_bridge.go`.)
- [x] **S8. `available_commands_update`** after session creation. (`agent.go`.)
- [x] **S9. Config/mode/info notifications** — `config_option_update` on
  `set_config_option`; `current_mode_update` on `set_mode`; `session_info_update`
  to keep `session/list` in sync. (`agent.go`.)
- [ ] **S10. Enrich `tool_call` events** — send `pending` on the first report, `failed`
  on error (correlate `message.ToolResult`), a human-readable `title`, plus
  `rawInput`, `locations`, and `diff`/`terminal` content. The SDK already exposes
  `ToolCallStatusPending/Failed`, `WithStartLocations`, `WithStartRawInput` — this is
  **not** SDK-gated, just unimplemented. (`event_bridge.go`.)

**Tier 3 — Optional (MAY): full parity**
- [x] **O1.** Return `modes` + `configOptions` in `session/new` response.
- [x] **O2.** Boolean config options — **IMPLEMENT** (`thinking` toggle on
  `SelectedModel.Think` via `UpdatePreferredModel` + `UpdateAgentModel`). See §4.1. (`agent.go`.)
- [x] **O3.** `additionalDirectories` capability + handling in `/new`, `/list` (echo back via `SessionInfo`). See §4.2. (`agent.go`.)
- [x] **O4.** `session/delete` + `sessionCapabilities.delete` — **IMPLEMENT** (`UnstableDeleteSession`). See §4.2. (`agent.go`, `server.go`.)
- [x] **O5.** `session/resume` decision: **capability rejection** (keep returns error; do not advertise `resume`). See §4.2. (`agent.go`.)
- [x] **O6.** `auth.logout` capability + `logout` behavior — **IMPLEMENT** (advertise + success no-op). See §4.2. (`agent.go`, `server.go`.)
- [x] **O7.** Image/audio/resource content parse + stream — **IMPLEMENT**
  (`buildPrompt` → `message.Attachment` → `Coordinator.Run`). See §4.1. (`agent.go`.)
- [ ] **O8.** Real terminal callbacks — **DEFER** (no PTY lib; `connect` is a stdio
  REPL with no terminal surface). Stays stubbed. See §4.1. (`client.go`.)
- [ ] **O9.** MCP server connection at session setup — **DEFER** (`mcp.Initialize`
  is startup-only; no per-session add API). See §4.1.
- [x] **O10.** `agentInfo.title`; `switch_mode` tool kind representation.
- [x] **O11.** Render streaming chunks in `crush acp connect` loop — **IMPLEMENT**
  (`Client.Subscribe` drain goroutine). See §4.1. (`acp_client.go`.)


> **Deferred** — grouped by the *actual* blocker (SDK vs missing Crush work):
> - **SDK-gated (UNSTABLE in pinned v0.13.5):** **S5** `messageId` — the field exists
>   on chunks and `PromptRequest`/`PromptResponse` but is flagged UNSTABLE; **S6**
>   elicitation — only `Unstable*` variants ship. Revisit on the next SDK release.
> - **Missing Crush subsystem (NOT SDK-gated — implementable with product work):**
>   **S4** `plan` updates (no plan data model to emit), **O8** terminals (could be
>   backed by `internal/shell` instead of a PTY lib — re-evaluate the deferral),
>   **O9** per-session MCP (`mcp.Initialize` is startup-only).
> - **Effort-deferred, NOT SDK-gated:** **S7** `_meta` — `Meta map[string]any` is
>   stable on every SDK type, so basic round-tripping is implementable today; only
>   W3C trace-context *generation* needs a tracing layer.
> - **Ready to implement now:** **S10** tool enrichment, **O14** config `category`
>   (both fully SDK-supported).
> - **O12/O13** File System + Session Usage — implemented (see §4.2).

### 4.1 Decisions log — config / modes / terminals group

Per-item research + decisions for the O2/O7–O9/O11 cluster (the next
implementation batch). Each entry records the finding that drove the call.

- **SDK bump (S5/S6 gate).** No version newer than `coder/acp-go-sdk@v0.13.5`
  exists in the module proxy. S5/S6 stay SDK-gated; revisit on next SDK release.

- **S2 — `user_message_chunk` on live prompt: NOT ECHOED; load replay DOES emit.**
  Correction from spec scrape: `user_message_chunk` is *not* client→agent only —
  on `session/load` the Agent MUST replay history including the user's messages
  (as `user_message_chunk`), and `server.go` `LoadSession` does so via
  `UpdateUserMessageText`. What is avoided is echoing it inside a live
  `session/prompt` handler, which trips the I-1 conformance barrier and races the
  streaming stream (same reason S8/S9 are safe — they fire outside a prompt).
  Decision: no live-turn echo; keep load-replay emission (satisfies R1/R6).

- **O2 — Boolean config option (`thinking`): IMPLEMENT.** Crush stores the flag
  on `config.SelectedModel.Think`. Runtime toggle path already exists and is what
  the TUI uses: read the current agent's model (`cfg.Agents[coder].Model` →
  `cfg.Models[type]`), flip `.Think`, persist via
  `ConfigStore.UpdatePreferredModel(ScopeGlobal, type, model)`, then
  `App.UpdateAgentModel(ctx)`. Advertise as `SessionConfigOptionBoolean{Id:"thinking"}`
  and apply in `SetSessionConfigOption` when `req.Boolean.ConfigId == "thinking"`.
  *Ceiling:* toggle is global (persisted), not per-session. Make per-session once
  `coordinator` accepts a runtime think override.

- **O7 — Image/audio/resource content: IMPLEMENT.** ACP `ContentBlockImage` /
  `ContentBlockAudio` carry base64 `Data` + `MimeType`; `ContentBlockResource`
  carries embedded bytes. Map each to a `message.Attachment{Content, MimeType,
  FileName}` and pass to `Coordinator.Run(ctx, sid, prompt, attachments...)`
  (the existing `attachments ...message.Attachment` variadic). `extractPromptText`
  is split into `buildPrompt` returning `(text, attachments)`. Text and
  `resource_link` keep current behavior (text concatenated; link rendered inline).

- **O8 — Real terminals: DEFER + document.** `crush acp connect` is a stdio REPL;
  it has **no interactive terminal surface** to forward a PTY into, and Crush
  carries **no PTY abstraction** (`creack/pty` is not a dependency). The five
  `terminal/*` client callbacks stay stubbed "not supported" and the connect
  `ClientCapabilities` does **not** advertise `terminal`. Revisit when/if the
  connect CLI gains a terminal pane or a PTY helper is added.

- **O9 — MCP at session setup: DEFER + document.** `mcp.Initialize(ctx,
  permissions, store)` runs **once at app startup** and reads the config store;
  there is **no per-session / runtime "add MCP server" API** (`mcp` package has
  only startup `Initialize`/`InitializeSingle`/`WaitForInit*`). Wiring
  `UnstableConnectMcp` would require new infrastructure (per-session MCP manager +
  dynamic tool registration on the coordinator). Out of scope for this batch;
  implement only after that manager lands.

- **O11 — Render streaming chunks in `crush acp connect`: IMPLEMENT.**
  `Client.Subscribe(sid)` already returns the per-session `SessionUpdate` channel
  that `crushClient.SessionUpdate` feeds. Start a drain goroutine before
  `Prompt` and render `AgentMessageChunk`, `AgentThoughtChunk`, and `ToolCall`
  updates to stdout; otherwise the CLI only prints `[stop: ...]` and the user
  sees no streaming.

### 4.2 Decisions log — optional group (O3–O6, O12–O13)

- **O3 — `additionalDirectories`: IMPLEMENT (advertise + echo).** Advertise
  `sessionCapabilities.additionalDirectories` and accept `NewSessionRequest.AdditionalDirectories`,
  storing them per session in a `sync.Map` and echoing them back via
  `SessionInfo.AdditionalDirectories` in `ListSessions`. Crush has no multi-root
  model, so they are persisted for round-tripping only; they do not change the
  agent's filesystem scope.

- **O4 — `session/delete`: IMPLEMENT.** Advertise `sessionCapabilities.delete`
  (SDK `SessionDeleteCapabilities`, UNSTABLE) and implement `UnstableDeleteSession`:
  cancel the coordinator, delete the session from the store, and clear the stored
  additional-directories key. The SDK dispatches `session/delete` only when the
  agent implements the `UnstableDeleteSession` method (interface assertion in
  `agent_gen.go`).

- **O5 — `session/resume`: capability rejection (DECISION).** The conformance suite
  `TestConformanceSessionSetupAndUnsupportedMethods` pins resume as *unsupported*
  (must error). Rather than wire a half-baked resume, keep `ResumeSession`
  returning an error and do **not** advertise the `resume` capability. Crush
  already continues any session id via `session/prompt` without replaying history,
  so a client that truly needs resume can prompt against the existing session.

- **O6 — `auth.logout`: IMPLEMENT (advertise + no-op).** Advertise
  `agentCapabilities.auth.logout` and make `Logout` a successful no-op — Crush
  has no auth session to terminate, so there is nothing to do. `Authenticate`
  remains a "not supported" stub, which is correct because no auth method is
  advertised.

- **O12 — File System (`fs`): IMPLEMENT (gated helpers).** Capture
  `clientCapabilities.fs` at `Initialize`. Add `fsReadTextFile`/`fsWriteTextFile`
  helpers on the agent that delegate to the connected client (`conn.ReadTextFile`
  / `conn.WriteTextFile`) when the client supports fs, otherwise fall back to the
  local OS (`os.ReadFile`/`WriteFile` + mkdir-parent). The client track already
  answers `fs/read_text_file`/`fs/write_text_file` from its own OS. *Ceiling:*
  server-track helpers are not yet wired into Crush's tool layer (edit/bash tools
  use the local FS); wire them when an agent-initiated, client-side file read is
  needed.

- **O13 — Session Usage (`usage_update`): IMPLEMENT.** At run completion the
  bridge emits a `usage_update` carrying the session's cumulative token usage
  (`PromptTokens + CompletionTokens`), the active model's context window size
  (best-effort via `Config.GetModelByType(coder).ContextWindow`), and cumulative
  cost. It is a best-effort signal: missing data is simply omitted.

- [x] **O12.** File System access — `fs/read_text_file` + `fs/write_text_file` — **IMPLEMENT** (gated `fsReadTextFile`/`fsWriteTextFile` helpers delegate to client when `clientCapabilities.fs` set, else local OS). See §4.2. (`agent.go`.)
  `clientCapabilities.fs` (read/write booleans); the Agent MUST NOT call them when
  unsupported. (`agent.go`, `event_bridge.go`.)
- [x] **O13.** Session Usage update — `usage_update` — **IMPLEMENT** (emitted at run completion with tokens-used + cost). See §4.2. (`event_bridge.go`.)
  and cumulative cost. (`event_bridge.go`.)
- [ ] **O14. Config option `category`** — set `category: "model"` on the model select
  and `category: "thought_level"` on the thinking boolean so clients place/icon them
  consistently. The SDK exposes `SessionConfigOptionCategory{Mode,Model,ThoughtLevel}`
  — **not** SDK-gated. (`agent.go` `configOptionsFor`.)

**Tier 4 — Tests** (gate each Tier 1–3 item)
- [ ] Regression tests per missing update type; `TestConformanceSessionLoad`;
  elicitation roundtrip; `_meta` passthrough; cancellation `cancelled` outcome.

## 5. Test strategy

```bash
go test ./internal/acp/... -count=1            # unit + integration
go test ./test/acp/spec/... -count=1           # conformance, golden, Tangerg interop
CRUSH_BIN=$(go build -o /tmp/crush .) \
  go test -tags e2e ./test/acp/ -count=1       # subprocess e2e
./test/acp/run.sh all                          # full runner (unit/spec/interop/sdk/e2e)
```
- Unit: permission logic, server roundtrip (`TestServerStartEndToEnd`),
  event bridge lifecycle, client subscribe/permission/IO.
- Golden: 33 JSON fixtures in `test/acp/spec/testdata/json_golden`.
- Cross-SDK: Tangerg/acp fixtures + Zed transcript in `test/acp/spec/testdata/tangerg`.
- OpenAgents conformance harness: documented in `test/acp/README.md`, not wired
  into `run.sh` (Node/pnpm; informational).

## 6. Unaddressed points

Direct answer to "is any point unaddressed":

Resolved since this section was written (verified against code + v1 spec):
`session/load` replay, `SessionInfo.cwd`, `authMethods:[]`, `agentInfo.title`,
`auth.logout`, every `session/update` type except `plan`, `switch_mode` tool kind,
`available_commands_update`, mode/config/info notifications, and `user_message_chunk`
on load. See §3.3/§3.5 status and §4.

Genuinely still open:
- **MCP stdio connect at session setup** (§3.12) — spec MUST baseline, unmet (O9).
- **`plan` updates** (S4) — needs a Crush plan subsystem; gated on the SDK's
  UNSTABLE `planCapabilities`.
- **Tool `pending`/`failed` status**, `locations`/`rawInput`, and `diff`/`terminal`
  tool content (§3.5, §3.13).
- **Config option `category`** and the boolean client-capability gate (SDK-blocked, §3.13).
- **`load` replay completeness** — text only, not tool/thought/plan entries (§3.13).
- **`messageId`** on chunks (stable in spec; UNSTABLE in pinned SDK — S5).
- **Elicitation** (`elicitation/create`/`complete`) on either side (S6) — UNSTABLE in SDK.
- **`_meta`/trace passthrough** (S7, §3.11).
- **`$/cancel_request` → `-32800`** explicit handling (§3.9).
- **Terminal callbacks** in client track (O8, deferred) + terminal-auth methods (§3.13).

All baseline methods, permission bridge, fs read/write, text/resource_link/embedded
content, `session/close`/`list`/`delete`, stdio transport, and capability advertising
are covered and conformant.

---

## Current Streaming Behavior (post-Zed test, 2026-09-17)

**Status: IMPROVED BUT STILL LIMITED.**

After fixing `splitDelta` (using `strings.Fields` + chunk spacing) and adding
`chunkDeliveryDelay` (50ms) in `streamText`/`streamThoughts`, the ACP stream
works for text that contains whitespace-separated tokens. However:

- The message broker (`internal/pubsub`) delivers **debounced full-turn
  snapshots**, not per-token deltas. `OnTextDelta` (in `agent.go`) fires only
  on full snapshot changes, not continuously during generation.
- As a result, `streamText` receives large deltas at turn boundaries rather
  than fine-grained per-word updates. The 50ms spacing approximates streaming
  cadence, but the underlying data is bulk.
- Tests (`crush_debug.jsonl`) show single-burst `agent_message_chunk`
  notifications (e.g., all 88 words delivered in one chunk at `12:37:38.814Z`)
  followed by spaced chunks — a long initial wait then progressive output.
- `agent_thought_chunk`, `tool_call`, `plan`, and `user_message_chunk`
  updates are **not emitted** (§3.3), which is the primary upstream gap
  (documented in Tier 1 / P2 S1). The upstream `OnTextDelta` infrequent firing
  (investigation B) should be tracked separately.
