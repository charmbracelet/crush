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
| `sessionCapabilities` (close/list/resume/delete/additionalDirectories) | OPTIONAL | ✅ all five advertised, including `resume` (`agent.go:70-76`) |
| `auth.logout` capability | OPTIONAL | ✅ advertised |
| `authenticate` method | Only reachable for advertised methods; a capability-gated method the Agent did not advertise SHOULD answer `-32601` (`error.md:46`) | ⚠️ `authMethods:[]` means clients MUST NOT call it, so nothing is owed today. Crush answers a stray call with a generic error → JSON-RPC `-32603`, not `-32601` (`server.go:81-83`). S2 (register provider auth methods, type `agent`, trigger OAuth) is the real work |
| `logout` method | MUST exist if `auth.logout` advertised | ✅ success no-op (Crush has no auth session) |

### 3.2 Session lifecycle methods

| Method | Level | Crush status |
|---|---|---|
| `session/new` | MUST | ✅ creates SQLite session, returns `sessionId` |
| `session/prompt` | MUST | ✅ runs coordinator, returns `StopReason` |
| `session/cancel` (notification) | MUST | ✅ cancels coordinator |
| `session/update` (notifications) | MUST | ✅ message/thought/tool_call/tool_call_update/usage emitted (see §3.3) |
| `session/close` | OPTIONAL (advertise `sessionCapabilities.close`) | ✅ implemented + advertised. Spec: closing MUST cancel ongoing work, is idempotent, and a non-existent session SHOULD still return the empty result; clients SHOULD treat `-32002` as success. Crush cancels the coordinator and always returns `{}`, which satisfies both |
| `session/list` | OPTIONAL (advertise `sessionCapabilities.list`) | ⚠️ implemented + advertised; `cwd`, `title`, `updatedAt`, `additionalDirectories` echoed. ❌ **Pagination not implemented**: `ListSessions` ignores `req.Cursor` and never sets `NextCursor`, so results are unbounded (spec-legal while one page suffices; a bad cursor is not rejected with `-32602`). `_meta` is not populated per `SessionInfo`. |
| `session/load` | OPTIONAL (advertise `loadSession`) | ✅ full-history replay (R7): user/agent text, `agent_thought_chunk`, `tool_call` starts (kind/title/rawInput), tool results with terminal status + content + `diff`, and `plan` updates. Synthesizes a start when a result has no matching call (`server.go:132-140`) |
| `session/resume` | OPTIONAL (advertise `sessionCapabilities.resume`) | ✅ implemented (Option B, §6.2 O5): returns mode/config state without replaying history; capability advertised |
| `session/delete` | OPTIONAL (advertise `sessionCapabilities.delete`) | ✅ implemented + advertised (`UnstableDeleteSession`). **Semantics**: deleting an already-deleted session SHOULD succeed silently; deleting an active session is implementation-defined. |
| `session/set_mode` | OPTIONAL (legacy; prefer config options; spec notes modes will be removed in a future version) | ✅ echoes `modeId` + emits `current_mode_update` (no real mode switch) |
| `session/set_config_option` | OPTIONAL | ✅ applies thinking/model and returns the complete `configOptions` list (spec MUST, `session-config-options.md:304`). Spec: every option's `currentValue` is the default the Agent operates with (`:245`). ❌ Unknown `configId`/value SHOULD answer `-32602` (`error.md:50`); Crush logs a warning and returns success with unchanged options (`agent.go:269-284`). |

### 3.3 Prompt turn & streaming (`session/update` types)

All are **notifications** carried in `{"sessionId", "update": {"sessionUpdate": <type>, ...}}`.
The spec distinguishes `session/load` (MUST replay history via these) from
`session/resume` (MUST NOT replay).

| Update type | Level | Crush status |
|---|---|---|
| `user_message_chunk` | MUST be emitted on `session/load` replay (`session-setup.md:135`). The v1 spec states **no** rule against echoing it during a live prompt | ✅ emitted during load replay (`UpdateUserMessageText`). Live-prompt echo is skipped for an internal reason (races the streaming path), not a spec one — see §6.1 S2 |
| `agent_message_chunk` | baseline | ✅ emitted on happy path (`subscribeMessages`) |
| `agent_thought_chunk` | SHOULD (reasoning) | ✅ emitted via `streamThoughts` (S3) |
| `tool_call` | SHOULD | ✅ emitted on tool start (S1) |
| `tool_call_update` | SHOULD (`pending`/`in_progress`/`completed`/`failed`) | ✅ full lifecycle: `pending` emitted by `CheckPermission` before the permission request; `in_progress`/`completed` by the event bridge; `failed` on tool errors and permission denial (S10) |
| `plan` | SHOULD | ✅ emitted as a full-replace update whenever the `todos` tool runs (S4): entries map from `session.Todo{Content, Status}` with `priority: medium` (todos carry no priority) |
| `available_commands_update` | MAY | ⚠️ emitted on `session/new` (S8) but **incomplete**: only custom markdown commands (`commands.LoadCustomCommands`) are listed; MCP prompts (`commands.LoadMCPPrompts`) and skills (`commands.FromSkillCatalog`) are **not**, despite §4.2 claiming they were. `input.hint` is never populated (`AvailableCommandInput.Unstructured.Hint` is required once `input` is present), and **shell builtins** (`provider`, `model`, `mcp`, `lsp`, `permissions`, `hook`, `option`) are not advertised — see §4.2. (`agent.go:438-455`) |
| `current_mode_update` | MAY | ✅ emitted on `session/set_mode` (S9) |
| `config_option_update` | MAY | ✅ emitted on config change (S9) |
| `session_info_update` | MAY (ties to `session/list`) | ✅ emitted on `session/new` (S9). Fields are `title`, `updatedAt`, `_meta`; Crush sets title + timestamp |
| `usage_update` | MAY (context + cost) | ✅ emitted at run completion (O13). Spec field rules (`prompt-turn.md:453`): `used`/`size` are required non-null token counts; if `cost` is present, `amount` + `currency` (an ISO 4217 code, or another unit such as `credits`) are required. |
| `notice` | MAY, **gated on `clientCapabilities.session.notices`** | ❌ not emitted. Agents MUST NOT send notices without that capability, and the pinned SDK has no `session` field on `ClientCapabilities`, so the gate cannot be read. Not a gap today (MAY), but see §8 |
| `compaction_update` / `compaction_summary_chunk` | MAY, **gated on `clientCapabilities.session.compaction`** | ❌ not emitted and not replayable. Spec also requires compaction to be replayed by `session/load` (`session-setup.md:137-140`); without emitting it, Crush has nothing to replay. SDK-gated for the same reason as `notice` |

`messageId` (per-message opaque id; chunks sharing it belong to one message) is a
**MAY** field on `agent_message_chunk`/`user_message_chunk` — **Crush never sets
it** (SDK v0.13.5 ships the field but flags it UNSTABLE; track on SDK bump).

The `sessionUpdate` variant set above is complete for v1
(`prompt-turn.md:17-33`): 13 values, of which Crush emits 11. The two missing
(`notice`, `compaction_update`/`compaction_summary_chunk`) are both
capability-gated on client fields the pinned SDK does not model.

### 3.4 Content blocks (prompts & outputs)

| Block | Level | Crush status |
|---|---|---|
| `text` | MUST | ✅ |
| `resource_link` | MUST | ✅ rendered inline by `buildPrompt` |
| `image` | OPTIONAL (gated by `promptCapabilities.image`) | ✅ both directions: decoded to `message.Attachment` on input (O7); streamed back as `agent_message_chunk` image blocks on output (O15, `streamAttachments`) |
| `audio` | OPTIONAL | ✅ both directions: decoded on input (O7); streamed back as audio content blocks on output (O15) |
| `resource` (embedded) | OPTIONAL (gated by `embeddedContext`) | ✅ parsed into prompt (O7) |
| `annotations` on blocks | OPTIONAL | ❌ ignored |

### 3.5 Tool calls

| Requirement | Level | Crush status |
|---|---|---|
| Emit `tool_call` + `tool_call_update` during execution | SHOULD | ✅ `StartToolCall`/`UpdateToolCall` from event bridge (S1); full status lifecycle (S10) |
| Tool kinds: `read/edit/delete/move/search/execute/think/fetch/switch_mode/other` | OPTIONAL taxonomy | ✅ `toolKindFor` maps Crush tools to all kinds incl. `switch_mode` |
| Tool content: `content` / `diff` / `terminal` | SHOULD | ⚠️ `content` always; `diff` emitted for `edit`/`multiedit` results (from old/new content metadata); `terminal` not emitted (deferred, O8) |
| `toolCallId`, `name`, `title`, `kind`, `status`, `locations`, `rawInput`, `rawOutput` | `title` required; `name` SHOULD when available; rest optional; **all fields except `toolCallId` are optional in updates** | ✅ `title` (human-readable: tool + primary argument), `kind`, `status`, `locations` (input `path`/`file_path`), `rawInput` (decoded JSON), `rawOutput` (tool result content), and `diff` content for `edit`/`multiedit`. ❌ `name` cannot be emitted: the SDK's `SessionUpdateToolCall`/`ToolCall` **have no `Name` field at all** (verified in v0.13.5 `types_gen.go`), so this needs an SDK bump, not a helper |
| `session/request_permission` (4 option kinds: `allow_once`/`allow_always`/`reject_once`/`reject_always`) | MUST when needed | ✅ bridge implemented |
| Client MUST respond `cancelled` to pending permission on `session/cancel` | MUST (client-side duty) | ⚠️ applies to the peer client; agent-side only aborts + returns `cancelled` stop reason |

### 3.6 Elicitation (`elicitation/create`, `elicitation/complete`)

OPTIONAL (gated by `clientCapabilities.elicitation.{form,url}`).

Crush: ❌ not implemented on **either** side. The docs describe it as a v1
method (`elicitation.md`), but the pinned SDK types it only as `Unstable*`
(`UnstableCreateElicitationRequest`, `UnstableCompleteElicitationNotification`)
and only wires it through `ClientExperimental`, which `crushClient` does not
implement — so an agent-initiated `elicitation/create` currently dies as
`-32601 Method not found`. Same blocker for both tracks.

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

- `session/cancel` is a **notification** (no response). ✅ cancels coordinator
  (`agent.go:222-230`).
- `$/cancel_request`. The SDK cancels the in-flight request's
  `context.Context` on receipt (`connection.go` `handleCancelRequest` →
  `inflight[idKey]`), so a cancelled `session/prompt` unwinds. Spec says the
  agent **MUST** answer with *either* a valid response (`cancelled` stop reason)
  or `-32800`. ✅ Crush takes option 1 — `Prompt` maps `ctx.Err()` →
  `StopReasonCancelled`; `-32800` is never emitted, which is conformant.
- On abort the agent MUST catch errors and return `cancelled` stop reason, never
  an error. ✅ `Prompt` maps `ctx.Err()` → `StopReasonCancelled`.
- Pending `request_permission` MUST be answered `cancelled` **by the client**
  (peer duty). ⚠️ Agent side is correct: `CheckPermission` sees `ctx.Err()` and
  returns a denial without error, and `permission.go` emits no spurious `failed`
  status when the outcome is `cancelled`. What is unverified is only whether the
  peer client answers in time; that is not our conformance obligation.

### 3.10 Capability advertising summary

Advertised today: `loadSession:true`, `promptCapabilities{image,audio,embeddedContext:true}`,
`sessionCapabilities{close,list,additionalDirectories,delete,resume}`, `auth.logout`,
and `authMethods:[]` (explicitly set to an empty, non-nil slice at
`agent.go:58`, so it serializes as `[]`). Nothing is withheld from the
advertising set; the gaps below are behavioural, not advertising.

Agent capabilities **not** advertised, and correctly so:
`sessionCapabilities.systemPrompt` (no system-prompt injection support) and
any `mcpCapabilities`. Their absence means clients must not send
`systemPrompt` or assume HTTP/SSE MCP — both hold.

### 3.11 Extensibility (`_meta`)

Every type has a `_meta` field. ⚠️ Crush echoes `_meta` on the message paths (S7):
`session/prompt` `_meta` is returned on the `PromptResponse` and attached to
every `session/update` notification for that turn; `session/load` replays echo
the load request's `_meta`. W3C trace-context values (`traceparent`/
`tracestate`/`baggage`) are reserved but never generated. Custom
capabilities advertised via `_meta` on capability objects; custom methods start
with `_`.

### 3.12 MCP servers

`session/new`/`load`/`resume` accept `mcpServers`, and the spec's lifecycle
diagrams show "Connect to MCP servers" inside all three
(`session-setup.md:22-41`).

The spec makes **stdio a MUST baseline for every Agent**; HTTP and SSE are optional (`mcpCapabilities.http`/
`.sse`, and SSE has since been deprecated by the MCP spec). The SDK labels
`McpServer.Stdio` "All Agents MUST support this transport"
(`types_gen.go:2716-2719`).

❌ **NOT DONE — the plan previously claimed this was met by Option B; it was not.**
Re-audited 2026-11-13: `NewSession`/`LoadSession`/`ResumeSession` never read
`req.McpServers`; no `mcp.Initialize` call exists in `internal/acp` (only the
browser-client `agent/tools/mcp` package, wired at app startup). Crush connects
the MCP servers it knows from config at startup, already-initialised, so a
client-supplied server list is silently dropped. This is the one remaining
MUST-level gap. `McpCapabilities` is advertised empty, which is consistent
(no HTTP/SSE), but the stdio baseline is not honoured for
client-supplied servers.

**Shape of the fix** (verified against the packages, not guessed):
`internal/agent/tools/mcp` already has everything needed — `createSession`
takes a `config.MCPConfig` (which is exactly `command`/`args`/`env`/`type`),
`registerSessionTools` writes the tools into the global registry that
`tools.GetMCPTools` iterates at the start of every turn, and `InitializeSingle`
already wires both. The missing piece is a session-scoped lifecycle: today
the registry is keyed by server *name* only and `session.Service` has no MCP
ownership, so two ACP sessions supplying different servers with the same
name would collide. Options: (a) name-mangle with the session id as the
registry key, (b) keep a per-session allow-list of server names and filter
`GetMCPTools` by it. `internal/app` already imports `tools/mcp`, so the
connection itself can live in the app layer and be exposed to `internal/acp`
as a narrow interface, avoiding an import cycle.

Security constraints the spec adds (`session-setup.md:200-210`): launch with
`command` and `args` as separate arguments (never through a shell); never
echo `env` or `headers` values into session updates, errors, `_meta`,
`session/list`, replayed history, model context, or command lines; never log
or persist them.

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
- **Tool `title`/`name`/`locations`/`rawInput`.** `title` is human-readable (tool name + primary input argument); `locations` (input `path`/`file_path`), `rawInput` (decoded JSON), `rawOutput` (tool result content), and `diff` content for `edit`/`multiedit` are populated. `name` is **unreachable**: `SessionUpdateToolCall`, `ToolCall`, and `SessionToolCallUpdate` have no `Name` field in v0.13.5 and there is no start/update helper — needs an SDK bump. `terminal` content still awaits O8.
- **Config option `category`.** ✅ IMPLEMENTED: `model`/`thought_level` categories on the model select and thinking boolean.
- **Boolean option client-capability gate.** Spec: Agent MUST NOT emit
  `type:"boolean"` options unless the client advertises
  `session.configOptions.boolean` (`initialization.md:179-185`,
  `session-config-options.md:207-209`). **SDK-blocked:** pinned `acp-go-sdk`
  v0.13.5 `ClientCapabilities` has no `session` field, so the gate can't be
  read through typed fields. Options: (a) wait for the SDK, (b) unmarshal
  `InitializeRequest` params a second time into a local struct with the extra
  field. Recorded, not fixing today; `thinking` is emitted ungated meanwhile.
- **MCP stdio connect** (see §3.12) — ❌ NOT IMPLEMENTED. `req.McpServers` is
  never read; no per-session `mcp.Initialize`. See §3.12.
- **`session/load` replay completeness.** ✅ IMPLEMENTED (R7): full history replays — user/agent text, thought chunks, tool calls (kind/title/rawInput), and tool results with terminal status and content.
- **`$/cancel_request` / error `-32800`.** ✅ CONFORMANT, not a gap. `cancellation.md:23-25`
  requires the implementation to send *either* a valid response (e.g.
  `session/prompt` with `cancelled`) *or* an error with `-32800`. The SDK
  cancels the in-flight request's `context.Context`, `Prompt` maps that to
  `StopReasonCancelled`, and `-32800` is never needed. See §3.9.
- **`_meta` passthrough** + reserved W3C trace keys (see §3.11) — not propagated.
- **Terminal authentication.** **The earlier "not modelled by the pinned SDK"
  claim is WRONG** (re-audited 2026-11-13): `acp-go-sdk` v0.13.5 *does* model
  it — `ClientCapabilities.Auth.Terminal bool` and `AuthMethod.Terminal`
  (`AuthMethodTerminalInline`) exist, both flagged UNSTABLE. Not implemented in
  Crush, but the blocker is a decision (Crush advertises no auth methods at
  all), not the SDK.
- **`authenticate` method** (S2): register provider auth methods
  (copilot, hyper, openai) as `authMethods` entries with `type:"agent"`.
  Implement `Authenticate` to trigger OAuth flow per method ID. Decision
  recorded 2026-09-18: agent-type methods (not terminal, not side-auth).
- **Error codes for unknown/unsupported input** (SHOULD-level, re-audited
  2026-11-13). The spec pins specific codes (`error.md:29-60`):
  - unknown session on a session-scoped request → `-32002` with
    `data.sessionId`. ❌ Crush falls through to the generic wrapper, so a
    missing session surfaces as `-32603` internal error.
  - invalid `session/list` cursor, or an unknown `configId`/value in
    `session/set_config_option`, or an unknown mode in `session/set_mode`
    → `-32602`. ❌ `ListSessions` ignores the cursor entirely;
    `SetSessionConfigOption` and `SetSessionMode` accept anything and
    return success.
  - a request other than `initialize` before initialization completes →
    `-32600`. ❌ no guard in `internal/acp`; the SDK's dispatch does not
    enforce ordering either.
  - a method the Agent did not advertise → `-32601`. ⚠️ `dispatcher.Authenticate`
    returns a plain `errors.New`, which `toReqErr` maps to `-32603`
    (`server.go:81-83`).
- **`usage_update` cost fields.** Spec requires `amount` + `currency` when
  `cost` is present and non-null `used`/`size` always. ✅ emitted with both
  token counts; cost is omitted rather than sent partially
  (`event_bridge.go:698-729`).

### 4.2 Verification: commands, YOLO, and feature gaps (2026-09-17)

Answers to three verification questions asked on `feature/acp`:

#### Q1 — Are commands supported from ACP? How to implement Crush commands in the bridge?

**Partial.** The `available_commands_update` notification (§3 spec: **MAY**)
is implemented and advertised on `session/new` (`agent.go`
`availableCommandsUpdate`). Today it exposes **only custom markdown
commands** (loaded via `commands.LoadCustomCommands`) — **not** MCP prompts
(`commands.LoadMCPPrompts` exists but is not called here), not user-invocable
skills (`commands.FromSkillCatalog`), and not the shell builtins
(`provider`, `model`, `mcp`, `lsp`, `permissions`, `hook`, `option`). Those
builtins are registered via `shell.RegisterBuiltin` and are tied to the bash
config system (crushrc); they have no ACP-facing entry point. `input.hint`
is never set even though the spec marks it required whenever `input` is
present (`slash-commands.md:70-72`).

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
| MCP stdio connect at session setup | §3.12: stdio is a MUST baseline for every Agent | **Yes** — `internal/agent/tools/mcp` already exposes `createSession` + `registerSessionTools`; needs a session-scoped registry (see §3.12) | No dynamic add/remove today; two sessions supplying the same server name would collide in the name-keyed registry |
| `loadSession` capability match | If advertised, `session/load` must work | ✅ it works | Full history replay implemented (R7) |

**SHOULD (§3):**

| Feature | Status | Implementable? | Notes |
|---|---|---|---|
| `agent_thought_chunk` | ✅ `streamThoughts` | — | |
| `tool_call` | ✅ incl. lifecycle | — | S10: `pending`/`failed` status, human-readable `title`, `locations`, `rawInput`, `rawOutput`, `diff` implemented; `name` is unreachable (no SDK field) and `terminal` content awaits O8 |
| `tool_call_update` | ✅ | — | Full status lifecycle implemented (S10) |
| `session/load` full replay | ✅ | — | R7: text/thought/tool_call/tool_result replayed |
| `user_message_chunk` on load | ✅ `UpdateUserMessageText` | — | |

**MAY (§3):**

| Feature | Status | Implementable? | Blocker |
|---|---|---|---|
| `available_commands_update` | ⚠️ custom cmds only, no `input.hint` | **Yes** | O17: add MCP prompts + skills and populate hints; builtins need the prompt-prefix dispatch path (Q1 path 1) |
| `usage_update` | ✅ | — | |
| `current_mode_update` / `config_option_update` / `session_info_update` | ✅ | — | |
| `session/resume` | ✅ (Option B) | — | Returns mode/config state without replay; capability advertised |
| Boolean config options (`type:"boolean"`) | ⚠️ emitted ungated | Not with this SDK | SDK gating: `ClientCapabilities.session.configOptions.boolean` is missing in v0.13.5, so the spec's MUST-NOT-until-advertised gate cannot be read (O2 caveat) |
| Config option `category` | ✅ | — | `model`/`thought_level` set (O14) |
| `_meta` passthrough | ✅ (echo) | — | Prompt/load `_meta` echoed (S7); W3C trace *generation* still needs new code |
| `messageId` on chunks | ❌ | No | SDK field UNSTABLE in v0.13.5 |
| Elicitation (`create`/`complete`) | ❌ | No | SDK has only `Unstable*` variants |
| `plan` updates | ✅ (S4) | — | Emitted from `todos` tool results, live and on replay (`planUpdateFor`) |
| Terminals (`terminal/*`) | ❌ | No (deferred) | No PTY lib; `connect` is stdio REPL with no terminal surface |
| Image/audio content on output | ✅ (O15) | — | Assistant `BinaryContent` parts stream as content blocks |
| Annotations on content blocks | ❌ (ignored) | Yes | Optional in spec |
| `$/cancel_request` → `-32800` | ✅ conformant | — | Spec allows either a `cancelled` response *or* `-32800` (`cancellation.md:23-25`); the SDK cancels the in-flight request ctx and Crush returns `StopReasonCancelled` |
| `session/list` pagination | ⚠️ single page | Yes | Ignores `cursor`, never sets `nextCursor`, no `-32602` on a bad cursor; legal while results fit one page |
| Terminal auth methods | ❌ | Yes | SDK **does** model `auth.terminal` + `AuthMethod.Terminal`; Crush advertises no auth methods |
| `notice` | ❌ | Not with this SDK | Gated on `clientCapabilities.session.notices`, which v0.13.5 does not model |
| `compaction_update` / `compaction_summary_chunk` | ❌ | Not with this SDK | Gated on `clientCapabilities.session.compaction`; also must be replayed by `session/load` |

**Summary:** Everything left is either a small Crush-side change or an SDK
limitation. Blockers: (1) pinned SDK lacks `messageId`, non-`Unstable`
elicitation, the `session.configOptions.boolean` client gate, and a `name`
field on tool calls (4 items); (2) missing Crush work: per-session MCP connect
(the only MUST gap), PTY for terminals, prompt-prefix dispatch for builtins;
(3) deliberate omissions (terminal auth, `-32800`).

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
- [x] **S4. `plan` updates** — ✅ IMPLEMENTED (2026-09-22): the `todos` tool's
  result metadata carries the full `session.Todo` list; the bridge emits it as a
  full-replace `plan` update (entries map to `PlanEntry{Content, Priority:
  medium, Status}`), both live and during `session/load` replay.
  (`event_bridge.go` `planUpdateFor`, `server.go`.)
  Test: `TestEventBridge_PlanUpdateFromTodos`.
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
  `locations`, `rawOutput`, and `diff` content for `edit`/`multiedit` results.
  `name` cannot be sent at all: the SDK struct has no such field, so this is
  SDK-bump-gated, not helper-gated. `terminal` content awaits O8.
  (`event_bridge.go`, `permission.go`.)

**Tier 3 — Optional (MAY): full parity**
- [x] **O1.** Return `modes` + `configOptions` in `session/new` response.
- [x] **O2.** Boolean config options — **IMPLEMENT** (`thinking` toggle on
  `SelectedModel.Think` via `UpdatePreferredModel` + `UpdateAgentModel`). See §6.1. (`agent.go`.)
- [x] **O3.** `additionalDirectories` capability + handling in `/new`, `/list` (echo back via `SessionInfo`). See §6.2. (`agent.go`.)
- [ ] **O4.** `session/delete` + `sessionCapabilities.delete` — ✅ IMPLEMENT
  (`UnstableDeleteSession`). See §6.2. (`agent.go`, `server.go`.)
- [x] **O5.** `session/resume` decision: OPTION B (decided 2026-09-18) — ✅ IMPLEMENTED
  (2026-09-22): `ResumeSession` returns mode/config state without replaying;
  `sessionCapabilities.resume` advertised; conformance test updated to expect
  success. See §6.2. (`agent.go`.)
- [x] **O6.** `auth.logout` capability + `logout` behavior — **IMPLEMENT** (advertise + success no-op). See §6.2. (`agent.go`, `server.go`.)
- [x] **O7.** Image/audio/resource content parse + stream — **IMPLEMENT**
  (`buildPrompt` → `message.Attachment` → `Coordinator.Run`). See §6.1. (`agent.go`.)
- [ ] **O8.** Real terminal callbacks — **DEFER** (no PTY lib; `connect` is a stdio
  REPL with no terminal surface). Stays stubbed. See §6.1. (`client.go`.)
- [ ] **O9.** MCP server connection at session setup — ❌ **NOT DONE.** The
  "Option B" entry below records a decision that was never implemented, and its
  premise (`mcp.Initialize` at session start) does not match the API. See §3.12
  for the re-audit and the shape of the fix.
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
>   ~~**S4** `plan` updates~~ (done 2026-09-22 — the existing `session.Todo`
>   data model feeds `acp.UpdatePlan`), **O8** terminals (could be
>   backed by `internal/shell` instead of a PTY lib — re-evaluate the deferral),
>   **O9** per-session MCP — ❌ **not implemented**; the "Option B" note below is
>   a stale decision record. See §3.12 for what the connect would actually need.
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
  Correction from spec audit: `user_message_chunk` is *not* client→agent only, and
  the v1 spec states **no** rule against echoing the user's message during a live
  turn. On `session/load` the Agent MUST replay history including the user's
  messages (as `user_message_chunk`, `session-setup.md:135`), and `server.go`
  `LoadSession` does so via `UpdateUserMessageText`. What is avoided is echoing
  it inside a live `session/prompt` handler, where it would duplicate input the
  client already holds and interleave with the streaming path. That is an
  internal UX decision, not a conformance requirement.
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

- **O9 — MCP at session setup: ❌ NOT IMPLEMENTED (decision reversed
  2026-11-13).** The "Option B" note that used to sit here was a decision with
  no code behind it, and described an API that does not exist
  (`mcp.Initialize(ctx, permissions, cfg *config.ConfigStore, ...)` takes a
  config store, not an `acp.McpServer`). Ground truth: the `mcp` package runs
  at app startup off the config store; there is **no per-session / runtime
  "add MCP server" API**, and the tool registry is keyed by server name only.
  The building blocks for a fix are `createSession(ctx, cfg, name,
  config.MCPConfig, resolver, channelOptIn)` and `registerSessionTools`, but
  they assume one global config store, so a session-scoped connect needs
  either name-mangling by session id or a per-session allow-list filter in
  `tools.GetMCPTools`. See §3.12.

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
- [ ] **O16. Annotations on content blocks** — optional in spec; currently
  ignored (§3.4). Low priority; implement only if a client needs them.
- [ ] **O17. `available_commands_update` completeness** — populate
  `input.hint` from `CustomCommand.Arguments` / `MCPPrompt.Arguments`, and add
  MCP prompts (`commands.LoadMCPPrompts`) and skills
  (`commands.FromSkillCatalog`) to the advertised list. See §3.3. (`agent.go`
  `availableCommandsUpdate`.)
- [ ] **O18. Spec error codes** — `-32002` for unknown sessions, `-32602` for a
  bad `session/list` cursor and for unknown `configId`/value/mode, `-32600`
  for requests before `initialize`, `-32601` for an unadvertised
  `authenticate`. All SHOULD-level. See §4.1.

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
`auth.logout`, `plan`, `switch_mode` tool kind, `available_commands_update`,
mode/config/info notifications, `user_message_chunk` on load, tool `diff`
content, and output-side image/audio. See §3.3/§3.5 status and §4.

Genuinely still open:
- **MCP stdio connect at session setup** (§3.12) — **spec MUST baseline, NOT met.** `req.McpServers` is ignored; no per-session `mcp.Initialize`. Highest-priority remaining gap.
- **Tool `name` field and `terminal` tool content** (§3.5, §4.1) — `name` is SHOULD on the wire: the SDK's `SessionUpdateToolCall`/`ToolCall` have no `Name` field at all and no start/update helper, so it cannot be emitted with v0.13.5; `terminal` needs the deferred terminal subsystem (O8).
- **Boolean client-capability gate** (SDK-blocked, §4.1) — `ClientCapabilities` has no `session` field in v0.13.5, so a `type:"boolean"` option is emitted regardless of client support (spec: MAY only after advertisement).
- **`messageId`** on chunks (MAY in spec; UNSTABLE in pinned SDK — S5).
- **Elicitation** (`elicitation/create`/`complete`) on either side (S6) — only `Unstable*` types ship; server-side is agent-initiated, client-side unimplemented.
- **W3C trace-context generation** (`traceparent`/`tracestate`/`baggage` values are reserved but never generated; `_meta` echo itself is implemented, S7).
- **`$/cancel_request` → `-32800`** explicit handling (§3.9) — not a gap: the
  spec accepts `StopReasonCancelled` in place of the error, which is what Crush
  returns when the SDK cancels the request context.
- **Terminal callbacks** in client track (O8, deferred) + terminal-auth methods (§4.1 — SDK *does* model these; not implemented).
- **Annotations on content blocks** (O16, MAY, low priority).
- **Session list pagination** (SHOULD) — `ListSessions` ignores `req.Cursor` and sets no `NextCursor`; spec-legal while everything fits in one page, but unbounded, and a bad cursor is not rejected with `-32602` (`session-list.md:170-177`).
- **`available_commands_update` is incomplete** (MAY) — only custom markdown commands are listed; MCP prompts + user-invocable skills are missing and `input.hint` is never populated (§3.3, §4.2 Q1, O17).
- **Spec error codes** (SHOULD) — `-32002` for an unknown session, `-32602` for a bad cursor / unknown `configId`, value or mode, `-32600` for requests before `initialize`, `-32601` for the unadvertised `authenticate`. All currently fall through to `-32603` (§4.1, O18).
- **Session compaction / notices** (`compaction_update`, `compaction_summary_chunk`, `notice`) — MAY update types in v1, each gated on a `clientCapabilities.session.*` field that the pinned SDK does not model. Not emitted; also not replayable on `session/load` (§3.3).

Resolved 2026-09-22: `session/load` full-history replay (R7, tool/thought/result parts), tool-call `pending`/`failed` lifecycle (S10), tool `title`/`locations`/`rawInput`/`rawOutput`/`diff`, `plan` updates from todos results (S4), `_meta` echo (S7), `session/resume` (O5), config `category` (O14), output-side image/audio (O15).

All baseline methods, permission bridge, fs read/write, text/resource_link/embedded
content, `session/close`/`list`/`delete`, stdio transport, and capability advertising
are covered and conformant — **except the MCP stdio baseline** (§3.12), the one
remaining MUST-level gap. The caveat on "capability advertising" is that two
advertised-or-emitted behaviours are sanctioned by omission rather than by
checking the peer: `thinking` ships as an ungated `type:"boolean"` option
because the client gate is unreadable in v0.13.5, and no other optional update
type is gated on client advertisement either (the SDK exposes no client-session
capability surface at all). Both are recorded above; neither is a MUST violation
under the spec's "omitted means unsupported" rule only if the client tolerates
the extra updates, which a v1 client must.

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
