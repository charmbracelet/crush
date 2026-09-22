# ACP (Agent Client Protocol) Implementation Plan

## 1. Overview

ACP standardizes editor↔agent communication over JSON-RPC 2.0 (stdio default).
Crush acts as an **ACP Server** (Zed, JetBrains, Neovim) and optionally as an
**ACP Client** (driving external agents). Spec: https://agentclientprotocol.com/protocol/v1/

### Protocol-level rules

- All file paths in the protocol **MUST** be absolute; line numbers are **1-based**.
- JSON-RPC messages **MUST** be UTF-8 encoded and delimited by newlines (`\n`), which **MUST NOT** contain embedded newlines. The agent **MAY** write UTF-8 to stderr for logging; the client **MAY** capture, forward, or ignore these logs.
- Stdio is a **MUST** baseline for every Agent; HTTP/SSE and custom transports are **MAY**. Custom transports **MUST** preserve the JSON-RPC message format and lifecycle.
- The session `cwd` **MUST** be absolute, **MUST** be used for the session regardless of where the Agent subprocess was spawned, and **MUST** remain the base for relative-path resolution. The effective root set `[cwd, ...additionalDirectories]` **SHOULD** bound tool file-system operations.

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
| `sessionCapabilities` (close/list/resume/delete/additionalDirectories) | OPTIONAL | ✅ `close`, `list`, `additionalDirectories`, `delete` advertised; `resume` intentionally not advertised (see §6.1) |
| `auth.logout` capability | OPTIONAL | ✅ advertised |
| `authenticate` method | MUST exist; return `auth_required`/error if unused | ⚠️ S2: register provider auth methods as `authMethods` entries (type `agent`); trigger OAuth on `authenticate` |
| `logout` method | MUST exist if `auth.logout` advertised | ✅ success no-op (Crush has no auth session) |

### 3.2 Session lifecycle methods

| Method | Level | Crush status |
|---|---|---|
| `session/new` | MUST | ✅ creates SQLite session, returns `sessionId` |
| `session/prompt` | MUST | ✅ runs coordinator, returns `StopReason` |
| `session/cancel` (notification) | MUST | ✅ cancels coordinator |
| `session/update` (notifications) | MUST | ✅ message/thought/tool_call/tool_call_update/usage emitted (see §3.3) |
| `session/close` | OPTIONAL (advertise `sessionCapabilities.close`) | ✅ implemented + advertised |
| `session/list` | OPTIONAL (advertise `sessionCapabilities.list`) | ✅ implemented + advertised; `cwd` set, `additionalDirectories` echoed back. **Pagination**: cursor-based; client MUST treat missing `nextCursor` as end of results, MUST treat cursors as opaque tokens. `SessionInfo` includes optional `_meta` for agent-specific metadata. |
| `session/load` | OPTIONAL (advertise `loadSession`) | ⚠️ B: implement — replays text only; tool/thought parts stored in `message.Parts` are skipped (see §4.1) |
| `session/resume` | OPTIONAL (advertise `sessionCapabilities.resume`) | ✅ implemented (Option B, §6.2 O5): returns mode/config state without replaying history; capability advertised |
| `session/delete` | OPTIONAL (advertise `sessionCapabilities.delete`) | ✅ implemented + advertised (`UnstableDeleteSession`). **Semantics**: deleting an already-deleted session SHOULD succeed silently; deleting an active session is implementation-defined. |
| `session/set_mode` | OPTIONAL (legacy; prefer config options; spec notes modes will be removed in a future version) | ✅ echoes `modeId` + emits `current_mode_update` (no real mode switch) |
| `session/set_config_option` | OPTIONAL | ✅ applies thinking/model, returns the complete `configOptions` list (spec MUST). Spec also: Agents **MUST** always provide a default value for every option (agent must operate correctly if the Client ignores/hides options or sends an unrecognized type). |

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
| `tool_call_update` | SHOULD (`pending`/`in_progress`/`completed`/`failed`) | ✅ full lifecycle: `pending` emitted by `CheckPermission` before the permission request; `in_progress`/`completed` by the event bridge; `failed` on tool errors and permission denial (S10) |
| `plan` | SHOULD | ❌ not emitted — no structured plan data model (SDK `UpdatePlan`/`SessionUpdatePlan` stable) |
| `available_commands_update` | MAY | ✅ emitted on `session/new` (S8). Covers **custom markdown commands** (`commands.LoadCustomCommands`) + MCP prompts only. **Shell builtins** (`provider`, `model`, `mcp`, `lsp`, `permissions`, `hook`, `option`) are **not** advertised as ACP commands — see §4.2 |
| `current_mode_update` | MAY | ✅ emitted on `session/set_mode` (S9) |
| `config_option_update` | MAY | ✅ emitted on config change (S9) |
| `session_info_update` | MAY (ties to `session/list`) | ✅ emitted on `session/new` (S9) |
| `usage_update` | MAY (context + cost) | ✅ emitted at run completion (O13). Spec field rules: `used`/`size` are required non-null token counts; if `cost` is present, `amount` + `currency` (ISO 4217) are required. |

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
| Emit `tool_call` + `tool_call_update` during execution | SHOULD | ✅ `StartToolCall`/`UpdateToolCall` from event bridge (S1); full status lifecycle (S10) |
| Tool kinds: `read/edit/delete/move/search/execute/think/fetch/switch_mode/other` | OPTIONAL taxonomy | ✅ `toolKindFor` maps Crush tools to all kinds incl. `switch_mode` |
| Tool content: `content` / `diff` / `terminal` | SHOULD | ⚠️ `content` (raw input as text) only; `diff`/`terminal` not emitted |
| `toolCallId`, `name`, `title`, `kind`, `status`, `locations`, `rawInput`, `rawOutput` | `title` required, rest optional; **all fields except `toolCallId` are optional in updates** | ✅ human-readable `title` (tool + primary argument), `kind`, `status`, `locations` (input `path`/`file_path`), `rawInput` (decoded JSON), `rawOutput` (tool result content). `name` still unset (no SDK helper); `diff`/`terminal` content not emitted |
| `session/request_permission` (4 option kinds: `allow_once`/`allow_always`/`reject_once`/`reject_always`) | MUST when needed | ✅ bridge implemented |
| Client MUST respond `cancelled` to pending permission on `session/cancel` | MUST (client-side duty) | ⚠️ applies to the peer client; agent-side only aborts + returns `cancelled` stop reason |

### 3.6 Elicitation (`elicitation/create`, `elicitation/complete`)

OPTIONAL (gated by `clientCapabilities.elicitation.{form,url}`).

Crush: ❌ not implemented on either side.

Spec essentials:
- `mode` discriminator is **required** (`form`/`url`); no implicit form default.
- Form: restricted JSON Schema in `requestedSchema`; MUST NOT request secrets.
- URL: unique `elicitationId` + `elicitation/complete` notification; when URL
  mode is required but unsupported the Agent MUST NOT downgrade to form mode
  (url→form fallback prohibited); client MUST show full URL + consent.
- Outcomes: `accept`/`decline`/`cancel` (not `cancelled`).
- Capability semantics: each mode (`form`/`url`) is advertised only via its own non-null field; `{}` advertises no modes (unlike MCP). Requesting an unadvertised mode produces JSON-RPC `-32602` (Invalid params).
- **User interaction**: Clients MUST clearly identify the Agent, respect user
  privacy, and provide clear decline and cancel controls. For form mode, Clients
  MUST let users review and modify responses before sending. For URL mode,
  Clients MUST display the target host and obtain consent before navigating.
- **URL security**: Agents MUST NOT put credentials/personal data in the URL
  and SHOULD use HTTPS outside development. Clients MUST NOT prefetch the URL
  or open it without explicit user consent; MUST show full URL before consent;
  MUST open in a secure context that prevents the Client or Agent's language
  model from inspecting the page or user input. Agent MUST verify the
  authenticated user who started the elicitation is the same user who
  completes the external interaction.

### 3.7 File system (`fs/read_text_file`, `fs/write_text_file`)

OPTIONAL (gated by `clientCapabilities.fs`).

Crush (client track): ✅ real `os.ReadFile`/`os.WriteFile` with `line`/`limit` and mkdir-parent.
Client **MUST** create the file if it doesn't exist (spec MUST).
Server track delegates to the connected client via the SDK.

### 3.8 Terminals (`terminal/*`)

OPTIONAL (gated by `clientCapabilities.terminal`).

Crush client track: ⚠️ all five methods (`create`/`output`/`wait_for_exit`/`kill`/`release`) stubbed "not supported".
Agent MUST `release` terminals it creates.

**Spec essentials** (all gated by `clientCapabilities.terminal`):
- `terminal/create` params: `sessionId` (required), `command` (required), `args`, `env` (name/value pairs), `cwd` (absolute path), `outputByteLimit` (bytes to retain; client truncates at character boundary).
- `terminal/output` returns: `output` (required), `truncated` (required), `exitStatus` (`exitCode`/`signal`, present only if command exited).
- Terminals CAN be embedded in tool calls as `terminal` content blocks with a `terminalId` — client displays live output even after release.
- Agent MUST NOT call terminal methods unless client advertises `terminal` capability.

### 3.9 Cancellation

- `session/cancel` is a **notification** (no response). ✅ cancels coordinator.
- `$/cancel_request` protocol-level cancel. ✅ handled — returns
  `StopReasonCancelled` as valid response (spec option 1); explicit `-32800`
  error NOT emitted (documented, not following).
- On abort the agent MUST catch errors and return `cancelled` stop reason, never
  an error. ✅ `Prompt` maps `ctx.Err()` → `StopReasonCancelled`.
- Pending `request_permission` MUST be answered `cancelled` on cancel. ⚠️ unverified.

### 3.10 Capability advertising summary

Advertised today: `loadSession:true`, `promptCapabilities{image,audio,embeddedContext:true}`,
`sessionCapabilities{close,list,additionalDirectories,delete,resume}`, `auth.logout`. **Missing
optional:** `authMethods:[]` is configured but serializes per SDK defaults.

### 3.11 Extensibility (`_meta`)

Every type has a `_meta` field. ⚠️ Crush echoes `_meta` on the message paths (S7):
`session/prompt` `_meta` is returned on the `PromptResponse` and attached to
every `session/update` notification for that turn; `session/load` replays echo
the load request's `_meta`. W3C trace-context values (`traceparent`/
`tracestate`/`baggage`) are reserved but never generated. Custom
capabilities advertised via `_meta` on capability objects; custom methods start
with `_`.

### 3.12 MCP servers

`session/new`/`load`/`resume` accept `mcpServers`.

The spec makes **stdio a MUST baseline for every Agent**; HTTP and SSE are optional (`mcpCapabilities.http`/
`.sse`, and SSE has since been deprecated by the MCP spec).

❌ Crush ignores `mcpServers` in `session/new`/`load` connect via `mcp.Initialize` at session start
(per-session, not startup-only).
Advertises real `mcpCapabilities`. Stdio is MUST baseline — conformance met via session-scoped initialization
(Option B: all servers known at session creation).

### 3.13 Transport Protocol

ACP uses JSON-RPC 2.0 for all message exchange. Key requirements:

- JSON-RPC messages **MUST** be UTF-8 encoded.
- Messages are delimited by newlines (`\n`) and **MUST NOT** contain embedded newlines.
- The agent **MAY** write UTF-8 strings to stderr for logging; the client **MAY** capture, forward, or ignore these logs.
- The agent **MUST NOT** write anything to `stdout` that is not a valid ACP message; the client **MUST NOT** write anything to the agent's `stdin` that is not a valid ACP message.
- Stdio is a **MUST** baseline for every Agent. HTTP and SSE are optional. Custom transports **MAY** be implemented provided the JSON-RPC message format and lifecycle requirements are preserved.

## 4. Verification & Conformance Gaps

### 4.1 Conformance gaps found in spec verification (2026-09-17)

Scraped the v1 spec end-to-end; items where the current implementation still
diverges from a spec requirement (independent of the §4 completion checkboxes):

- **Tool status lifecycle.** IMPLEMENTED (2026-09-22): `pending` emitted in `CheckPermission` before `RequestPermission`; `failed` emitted on permission denial and tool errors (with result content); `in_progress`/`completed` from the event bridge. See S10.
- **Tool `title`/`name`/`locations`/`rawInput`.** `title` is now human-readable (tool name + primary input argument); `locations` (input `path`/`file_path`), `rawInput` (decoded JSON), and `rawOutput` (tool result content) are populated. `name` is still not set (the SDK exposes no `WithStartName` helper), and `diff`/`terminal` tool content is never emitted.
- **Config option `category`.** ✅ IMPLEMENTED: `model`/`thought_level` categories on the model select and thinking boolean.
- **Boolean option client-capability gate.** Spec: Agent MUST NOT emit
  `type:"boolean"` options unless the client advertises
  `session.configOptions.boolean`. **SDK-blocked:** pinned `acp-go-sdk` v0.13.5
  `ClientCapabilities` has no `session` field, so the gate can't be read yet.
  Revisit on SDK bump (same class as `messageId`/elicitation).
- **MCP stdio connect** (see §3.12) — ✅ B: session-scoped `mcp.Initialize`
- **`session/load` replay completeness.** ✅ IMPLEMENTED (R7): full history replays — user/agent text, thought chunks, tool calls (kind/title/rawInput), and tool results with terminal status and content.
- **`$/cancel_request` / error `-32800`.** Relies on SDK context cancellation;
  Crush does not explicitly emit the `-32800` Request-Cancelled error.
- **`_meta` passthrough** + reserved W3C trace keys (see §3.11) — not propagated.
- **Terminal authentication** (`clientCapabilities.auth.terminal`, `type:"terminal"`
  auth methods) — not modelled by the pinned SDK; not handled.
- **`authenticate` method** (S2): register provider auth methods
  (copilot, hyper, openai) as `authMethods` entries with `type:"agent"`.
  Implement `Authenticate` to trigger OAuth flow per method ID. Decision
  recorded 2026-09-18: agent-type methods (not terminal, not side-auth).

### 4.2 Verification: commands, YOLO, and feature gaps (2026-09-17)

Answers to three verification questions asked on `feature/acp`:

#### Q1 — Are commands supported from ACP? How to implement Crush commands in the bridge?

**Partial.** The `available_commands_update` notification (§3 spec: **MAY**)
is implemented and advertised on `session/new` (`agent.go`
`availableCommandsUpdate`). Today it exposes **only custom markdown
commands** (loaded via `commands.LoadCustomCommands`) and MCP prompts —
**not** the shell builtins (`provider`, `model`, `mcp`, `lsp`,
`permissions`, `hook`, `option`). Those builtins are registered via
`shell.RegisterBuiltin` and are tied to the bash config system
(crushrc); they have no ACP-facing entry point.

To expose builtins as ACP slash commands, two natural paths:

1. **Parse `/<builtin>` from prompt text.** The spec says commands run
   "as part of regular `session/prompt` requests" (command spec §9).
   Add a prompt-prefix handler in the coordinator that recognizes
   `/<name> ...` and dispatches to the corresponding shell builtin
   via `shell.Run` with the `ConfigBuilder` on context. Advertise each
   builtin in `availableCommandsUpdate` with `Name`, `Description`, and
   `Input.Hint` (e.g. `"provider"` → hint `"add <id> [flags]"`).
2. **Tool-call wrapper.** Emit each builtin as a `tool_call` with
   `kind: other`; the agent executes it and returns output as a
   `tool_result`. This reuses the existing tool-call streaming path
   but loses slash-command UX (no `/` prefix, no client-side command
   palette entry).

Path 1 is spec-idiomatic. Path 2 is a faster interim if the client
already renders tool calls as interactive elements.

#### Q2 — Can YOLO be hooked as a boolean option, such as thinking?

**Yes — structurally identical to the `thinking` option.**
YOLO is `store.Overrides().SkipPermissionRequests` (`config/store.go:61`),
a plain boolean. Adding it as a config option requires:

- **Advertise** in `configOptionsFor` (`agent.go:455`):
  `SessionConfigOptionBoolean{Id:"yolo", Name:"YOLO", Type:"boolean", CurrentValue: cfg.Overrides().SkipPermissionRequests, Description: ..., Category: category("_yolo")}`.
- **Apply** in `SetSessionConfigOption` (`agent.go:255`): add a branch
  `if req.Boolean != nil && req.Boolean.ConfigId == "yolo" { store.Overrides().SkipPermissionRequests = req.Boolean.Value; ... }` mirroring the thinking path.
- **Category:** The spec category table (§4.1 O14) lists only
  `mode`, `model`, `model_config`, `thought_level`. Auto-approve
  permissions matches none. Per spec §4.1: "Category names beginning
  with `_` are free for custom use." Use `_yolo` — a custom category
  (UX-only; clients **MUST** handle unknown categories gracefully).
- **Gating caveat:** Identical to `thinking` — pinned `acp-go-sdk`
  v0.13.5 `ClientCapabilities` has no `session.configOptions.boolean`,
  so the spec's "MUST NOT emit `type:"boolean"` unless the client
  advertised support" gate cannot be read at runtime. Both `thinking`
  and `yolo` are currently emitted ungated; revisit on SDK bump.

#### Q3 — What features are missing, MUST → OPTIONAL? Can we implement them?

Status legend: ✅ done · ⚠️ partial · ❌ missing. "Implementable?" = do we
have the SDK surface + Crush infrastructure today?

**MUST (spec §3):**

| Requirement | Level | Crush status | Implementable? |
|---|---|---|---|
| `initialize` response (version + capabilities) | MUST | ✅ | — |
| `session/new` | MUST | ✅ | — |
| `session/prompt` | MUST | ✅ | — |
| `session/cancel` (notification) | MUST | ✅ | — |
| `session/update` baseline types (text/thought/tool) | MUST | ✅ | — |
| `text` / `resource_link` content in prompts | MUST | ✅ | — |
| `authenticate` method | MUST | ⚠️ S2: register provider auth methods as `authMethods` entries (type `agent`); trigger OAuth on `authenticate` | Yes — return `auth_required` |
| `logout` when `auth.logout` advertised | MUST | ✅ no-op | — |
| `session/load` when advertised | MUST | ✅ works; full replay (R7) | — |
| `authMethods:[]` in response | MUST (field present) | ✅ `[]` | — |
| Agent MUST respond with agreed protocol version | MUST | ✅ | — |

**MUST baseline gaps (genuine conformance failures):**

| Gap | Spec basis | Implementable? | Blocker |
|---|---|---|---|
| MCP stdio connect at session setup | §3.12: stdio is a MUST baseline for every Agent | **Yes** — B: `mcp.Initialize` at session start (all servers known upfront) | No dynamic add/remove after session start |
| `loadSession` capability match | If advertised, `session/load` must work | ✅ it works | Full history replay implemented (R7) |

**SHOULD (§3):**

| Feature | Status | Implementable? | Notes |
|---|---|---|---|
| `agent_thought_chunk` | ✅ `streamThoughts` | — | |
| `tool_call` | ✅ incl. lifecycle | — | S10: `pending`/`failed` status, human-readable `title`, `locations`, `rawInput`, `rawOutput` implemented; `name`/`diff`/`terminal` remain unset |
| `tool_call_update` | ✅ | — | Full status lifecycle implemented (S10) |
| `session/load` full replay | ✅ | — | R7: text/thought/tool_call/tool_result replayed |
| `user_message_chunk` on load | ✅ `UpdateUserMessageText` | — | |

**MAY (§3):**

| Feature | Status | Implementable? | Blocker |
|---|---|---|---|
| `available_commands_update` (builtins) | ⚠️ custom cmds only | **Yes** | Need prompt-prefix dispatch path (Q1 path 1) or tool-call wrapper |
| `usage_update` | ✅ | — | |
| `current_mode_update` / `config_option_update` / `session_info_update` | ✅ | — | |
| `session/resume` | ✅ (Option B) | — | Returns mode/config state without replay; capability advertised |
| Boolean config options (`type:"boolean"`) | ✅ (thinking+gated) | — | SDK gating: `ClientCapabilities.session.configOptions.boolean` missing in v0.13.5 |
| Config option `category` | ✅ | — | `model`/`thought_level` set (O14) |
| `_meta` passthrough | ✅ (echo) | — | Prompt/load `_meta` echoed (S7); W3C trace *generation* still needs new code |
| `messageId` on chunks | ❌ | No | SDK field UNSTABLE in v0.13.5 |
| Elicitation (`create`/`complete`) | ❌ | No | SDK has only `Unstable*` variants |
| `plan` updates | ❌ | No | Needs Crush plan data model; SDK `planCapabilities` UNSTABLE |
| Terminals (`terminal/*`) | ❌ | No (deferred) | No PTY lib; `connect` is stdio REPL with no terminal surface |
| Image/audio content on output | ✅ (O15) | — | Assistant `BinaryContent` parts stream as content blocks |
| Annotations on content blocks | ❌ (ignored) | Yes | Optional in spec |
| `$/cancel_request` → `-32800` | ❌ | No | SDK-level; relies on context cancellation |
| Terminal auth methods | ❌ | No | SDK doesn't model `auth.terminal` |

**Summary:** Most missing features are **implementable today** — the real
blockers are concentrated in three areas: (1) the pinned SDK lacks
`messageId`, `elicitation`, `boolean config gate`, and `planCapabilities`
(4 items); (2) missing Crush subsystems (MCP per-session connect, plan
data model, PTY for terminals — 3 items); (3) deliberate omissions
(resume, terminal auth). Everything else needs 1–2 files changed, no
SDK or subsystem work.

## 5. Implementation Roadmap (current state)

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
  full history before its response (superset of R1). Text chunks replay today;
  tool/thought parts do not — the remaining MUST gap is tracked as R7.
- [x] **R7. `session/load` full-history replay** — spec MUST: replay the *entire*
  conversation. ✅ IMPLEMENTED (2026-09-22): user/agent text chunks,
  `agent_thought_chunk`, `tool_call` starts (kind/title/rawInput), and tool
  results with terminal status + content, including synthesized starts for
  results whose call is missing from history. (`server.go` `LoadSession`.)
  Test: `TestLoadSessionReplaysFullHistory`.

**Tier 2 — Recommended (SHOULD): core UX**
- [x] **S1. Tool progress** — emit `tool_call` + `tool_call_update`.
  ✅ IMPLEMENTED, incl. full status lifecycle (S10). (`event_bridge.go`.)
  Test: `TestEventBridge_ToolCallEnrichment`, `TestEventBridge_ToolResultFailedStatus`.
- [ ] **S2. `user_message_chunk` live-turn echo** on `session/prompt`. → **NOT ECHOED** (I-1 race; the client already holds the input). Still emitted on `session/load` replay — see §6.1 S2.
- [x] **S3. `agent_thought_chunk`** for reasoning deltas. (`event_bridge.go`.)
- [ ] **S4. `plan` updates** — each notification is a FULL replace. (`event_bridge.go`.)
- [ ] **S5. `messageId`** on chunks (SDK field currently UNSTABLE; set once the SDK ships it).
- [ ] **S6. Elicitation** — `elicitation/create` + `elicitation/complete`
  (form+url, `accept`/`decline`/`cancel`, unique `elicitationId`, no url→form
  downgrade, no secrets in form). Client callback + server stub. (`client.go`, `agent.go`.)
  Test: `TestElicitationFormMode`.
- [x] **S7. `_meta` passthrough** — ✅ IMPLEMENTED: `session/prompt` `_meta` is
  echoed on the `PromptResponse` and attached to every `session/update`
  notification for that turn (cleared on run completion); `session/load` replays
  echo the load request's `_meta`. W3C trace keys are reserved (not generated).
- [x] **S8. `available_commands_update`** after session creation. (`agent.go`.)
- [x] **S9. Config/mode/info notifications** — `config_option_update` on
  `set_config_option`; `current_mode_update` on `set_mode`; `session_info_update`
  to keep `session/list` in sync. (`agent.go`.)
- [x] **S10. Enrich `tool_call` events** — ✅ IMPLEMENTED (2026-09-22): `pending`
  on permission wait (emitted by `CheckPermission`), `failed` on denial and tool
  errors, human-readable `title` (tool + primary argument), `rawInput`,
  `locations`, `rawOutput`. `name` and `diff`/`terminal` content remain unset
  (`name` lacks an SDK helper; `diff` needs old/new text plumbing).
  (`event_bridge.go`, `permission.go`.)

**Tier 3 — Optional (MAY): full parity**
- [x] **O1.** Return `modes` + `configOptions` in `session/new` response.
- [x] **O2.** Boolean config options — **IMPLEMENT** (`thinking` toggle on
  `SelectedModel.Think` via `UpdatePreferredModel` + `UpdateAgentModel`). See §6.1. (`agent.go`.)
- [x] **O3.** `additionalDirectories` capability + handling in `/new`, `/list` (echo back via `SessionInfo`). See §6.2. (`agent.go`.)
- [x] **O4.** `session/delete` + `sessionCapabilities.delete` — **IMPLEMENT** (`UnstableDeleteSession`). See §6.2. (`agent.go`, `server.go`.)
- [x] **O5.** `session/resume` decision: OPTION B (decided 2026-09-18) — ✅ IMPLEMENTED
  (2026-09-22): `ResumeSession` returns mode/config state without replaying;
  `sessionCapabilities.resume` advertised; conformance test updated to expect
  success. See §6.2. (`agent.go`.)
- [x] **O6.** `auth.logout` capability + `logout` behavior — **IMPLEMENT** (advertise + success no-op). See §6.2. (`agent.go`, `server.go`.)
- [x] **O7.** Image/audio/resource content parse + stream — **IMPLEMENT**
  (`buildPrompt` → `message.Attachment` → `Coordinator.Run`). See §6.1. (`agent.go`.)
- [ ] **O8.** Real terminal callbacks — **DEFER** (no PTY lib; `connect` is a stdio
  REPL with no terminal surface). Stays stubbed. See §6.1. (`client.go`.)
- [x] **O9.** MCP server connection at session setup — **B**: `mcp.Initialize`
  at session start (all servers known upfront). See §6.1. Documented: no dynamic
  add/remove after session start.
- [x] **O10.** `agentInfo.title`; `switch_mode` tool kind representation.
- [x] **O11.** Render streaming chunks in `crush acp connect` loop — **IMPLEMENT**
  (`Client.Subscribe` drain goroutine). See §6.1. (`acp_client.go`.)


> **Deferred** — grouped by the *actual* blocker (SDK vs missing Crush work):
> - **SDK-gated (UNSTABLE in pinned v0.13.5):** **S5** `messageId` — the field exists
>   on chunks and `PromptRequest`/`PromptResponse` but is flagged UNSTABLE; **S6**
>   elicitation — only `Unstable*` variants ship; the boolean config-option client
>   gate (`ClientCapabilities.session.configOptions.boolean`) and terminal auth
>   methods (`clientCapabilities.auth.terminal`, `type:"terminal"`) are likewise
>   not modelled. Revisit on the next SDK release.
> - **Missing Crush subsystem (NOT SDK-gated — implementable with product work):**
>   **S4** `plan` updates (no plan data model to emit), **O8** terminals (could be
>   backed by `internal/shell` instead of a PTY lib — re-evaluate the deferral),
>   **O9** per-session MCP — **decided Option B**: session-scoped `mcp.Initialize`
>   at session start. No dynamic add/remove after start.
> - **Conformance test adjustment needed for O5:** done 2026-09-22 (see below).
>
> - **Effort-deferred, NOT SDK-gated:** **S7** `_meta` — `Meta map[string]any` is
>   stable on every SDK type, so basic round-tripping is implementable today; only
>   W3C trace-context *generation* needs a tracing layer.
> - **Ready to implement now:** ~~**S10** tool enrichment~~ (done 2026-09-22),
>   ~~**O14** config `category`~~ (done 2026-09-22).
> - **Conformance test for O5:** updated — `TestConformanceSessionSetupAndUnsupportedMethods`
>   now expects `ResumeSession` to succeed (Option B).
> - **O12/O13** File System + Session Usage — implemented (see §6.2).

## 6. Decision Logs

### 6.1 Decisions log — config / modes / terminals group

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

- **O9 — MCP at session setup: OPTION B (decided).** `mcp.Initialize`
  called at `session/new` when `mcpServers` non-empty. All servers known at
  session creation, so no dynamic add/remove needed. Not startup-only anymore.
  Documented limitation: no add/remove mid-session.
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

### 6.2 Decisions log — optional group (O3–O6, O12–O13)

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

- **O5 — `session/resume`: OPTION B (DECISION 2026-09-18), IMPLEMENTED 2026-09-22.** `ResumeSession` returns the current mode/config state and succeeds without replaying history — `session/prompt` already continues any session id
  (same as `crush --continue`). `sessionCapabilities.resume` IS advertised.
  `TestConformanceSessionSetupAndUnsupportedMethods` now expects success.

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

- [x] **O12.** File System access — `fs/read_text_file` + `fs/write_text_file` — **IMPLEMENT** (gated `fsReadTextFile`/`fsWriteTextFile` helpers delegate to client when `clientCapabilities.fs` set, else local OS). See §6.2. (`agent.go`.)
  `clientCapabilities.fs` (read/write booleans); the Agent MUST NOT call them when
  unsupported. (`agent.go`, `event_bridge.go`.)
- [x] **O13.** Session Usage update — `usage_update` — **IMPLEMENT** (emitted at run completion with tokens-used + cost). See §6.2. (`event_bridge.go`.)
  and cumulative cost. (`event_bridge.go`.)
- [x] **O14. Config option `category`** — ✅ IMPLEMENTED: `category: "model"` on the
  model select and `category: "thought_level"` on the thinking boolean.
  (`agent.go` `configOptionsFor`.)
- [x] **O15. Image/audio content on output** — ✅ IMPLEMENTED (2026-09-22):
  assistant `BinaryContent` parts stream as image/audio `agent_message_chunk`
  content blocks, deduped per message. (`event_bridge.go` `streamAttachments`.)
- [ ] **O15. Image/audio content on output** — O7 covers input decode only;
  streaming image/audio blocks in output `agent_message_chunk`/tool content is
  not implemented (§4.2 MAY table: "input only"). (`event_bridge.go`.)
- [ ] **O16. Annotations on content blocks** — optional in spec; currently
  ignored (§3.4). Low priority; implement only if a client needs them.

**Tier 4 — Tests** (gate each Tier 1–3 item)
- [ ] Regression tests per missing update type; `TestConformanceSessionLoad`;
  elicitation roundtrip; `_meta` passthrough; cancellation `cancelled` outcome.

## 7. Test strategy

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

## 8. Unaddressed Points

Direct answer to "is any point unaddressed":

Resolved since this section was written (verified against code + v1 spec):
`session/load` replay, `SessionInfo.cwd`, `authMethods:[]`, `agentInfo.title`,
`auth.logout`, every `session/update` type except `plan`, `switch_mode` tool kind,
`available_commands_update`, mode/config/info notifications, and `user_message_chunk`
on load. See §3.3/§3.5 status and §4.

Genuinely still open:
- **MCP stdio connect at session setup** (§3.12) — spec MUST baseline, met via Option B (session-scoped `mcp.Initialize`).
- **`plan` updates** (S4) — plan mode exists (produces markdown via `plan` agent + `plan.md.tpl`), but no structured `PlanEntry{Content, Priority, Status}` data model feeds `acp.UpdatePlan`. Blocker is the missing structured data layer, not the SDK (both `UpdatePlan` and `SessionUpdatePlan` are stable).
- **Tool `name` field and `diff`/`terminal` tool content** (§3.5, §4.1) — `name` lacks an SDK helper; `diff` needs old/new text plumbing; `terminal` needs the deferred terminal subsystem.
- **Boolean client-capability gate** (SDK-blocked, §4.1).
- **`messageId`** on chunks (stable in spec; UNSTABLE in pinned SDK — S5).
- **Elicitation** (`elicitation/create`/`complete`) on either side (S6) — UNSTABLE in SDK.
- **W3C trace-context generation** (`traceparent`/`tracestate`/`baggage` values are reserved but never generated; `_meta` echo itself is implemented, S7).
- **`$/cancel_request` → `-32800`** explicit handling (§3.9).
- **Terminal callbacks** in client track (O8, deferred) + terminal-auth methods (§4.1).
- **Annotations on content blocks** (O16, MAY, low priority).

Resolved 2026-09-22: `session/load` full-history replay (R7, tool/thought/result parts), tool-call `pending`/`failed` lifecycle (S10), tool `title`/`locations`/`rawInput`/`rawOutput`, `_meta` echo (S7), `session/resume` (O5), config `category` (O14), output-side image/audio (O15).

All baseline methods, permission bridge, fs read/write, text/resource_link/embedded
content, `session/close`/`list`/`delete`, stdio transport, and capability advertising
are covered and conformant.

---

### 8.1 Current Streaming Behavior (post-Zed test, 2026-09-17)

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

## 9. Fantasy Streaming Investigation (2026-09-18)

Investigated the fantasy SDK streaming path to confirm the transport format
used for OpenAI-compatible endpoints, and how `fantasy.Agent.Stream` is
shaped. Findings:

### `fantasy.Agent.Stream` shape

- **Definition:** `charm.land/fantasy@v0.43.1/agent.go:902`
  `func (a *agent) Stream(ctx context.Context, opts AgentStreamCall) (*AgentResult, error)`.
- Drives the full agent loop (steps, tool dispatch, retries) and delegates
  per-step work to the model's `Stream` method.
- `AgentStreamCall` (agent.go:272–325) carries the full callback set:
  `OnChunk`, `OnTextStart/Delta/End`, `OnReasoningStart/Delta/End`,
  `OnToolInputStart/Delta/End`, `OnToolCall`, `OnToolResult`, `OnSource`,
  `OnStreamFinish`, plus lifecycle callbacks `OnAgentStart/Finish`,
  `OnStepStart/Finish`, `OnFinish`, `OnError`, `OnWarnings`.

### `StreamResponse` type

- **Definition:** `model.go:190`
  `type StreamResponse = iter.Seq[StreamPart]` — a Go 1.23 iterator
  (a `func(yield func(StreamPart) bool)` closure), **not** a channel.
- `StreamPart` (model.go:170–180) is a single typed delta:
  `Type`, `ID`, `ToolCallName`, `ToolCallInput`, `Delta`,
  `ReasoningDelta`, `Usage`, `FinishReason`, `Error`, `Warnings`,
  `ProviderMetadata`.
- `LanguageModel` interface (`model.go:258,261`) requires
  `Stream(ctx, Call) (StreamResponse, error)` and
  `StreamObject(ctx, ObjectCall) (ObjectStreamResponse, error)`.

### SSE on OpenAI-compatible endpoints: **yes**

- The `openaicompat` provider (`providers/openaicompat/openaicompat.go`)
  **does not override `Stream`**. It builds an `openai.New(...)` client and
  reuses `openai.languageModel.Stream` (`providers/openai/language_model.go:465`),
  which calls `client.Chat.Completions.NewStreaming` from
  `github.com/openai/openai-go/v3`.
- `openai-go/v3` uses **SSE** (`text/event-stream`) — confirmed by the
  provider's test fixtures (`openaicompat/header_test.go:50` and
  `replay_test.go:44` both set `Content-Type: text/event-stream`) and by
  the SDK's own `packages/ssestream` decoder, which parses SSE frames
  (`event:` / `data:` lines) with a `bufio.Scanner`.
- **No NDJSON / JSON-lines transport** is used anywhere in the openai or
  openaicompat provider code. The OpenAI API's streaming contract is SSE
  regardless of whether the endpoint is `openai.com` or a compat server
  (vLLM, llama.cpp, groq, xai, OpenRouter, etc. all return
  `text/event-stream` with `data:`-framed chunks).

### Implication for ACP streaming

The SSE transport is **per-delta and fine-grained** (one `data:` frame per
chunk, typically 1–few tokens). The "bulk snapshot" behavior observed in
`agent_message_chunk` (see §"Current Streaming Behavior") therefore does
**not** come from fantasy or the OpenAI SDK. It originates upstream in Crush's
`internal/pubsub` broker, which debounces `OnTextDelta` into full-turn
snapshots. The 50ms `chunkDeliveryDelay` in `streamText`/`streamThoughts`
approximates cadence over bulk data — the root cause is the pubsub
coalescing, not the SDK.

**Skipped:** no changes to fantasy, openai-go, or openaicompat. The fix for
fine-grained ACP chunking belongs in `internal/pubsub` + `internal/acp`
event bridge — revisit when investigating "OnTextDelta infrequent firing"
(investigation B).
