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
| `sessionCapabilities` (close/list/resume/delete) | OPTIONAL | ✅ four advertised: `close`, `delete`, `list`, `resume` (`agent.go`). `additionalDirectories` is **deliberately not advertised** — Crush has no per-session root set to back it, so the capability was retracted rather than promised and ignored (§3.14, O23). |
| `auth.logout` capability | OPTIONAL | ✅ advertised |
| `authenticate` method | Only reachable for advertised methods; a capability-gated method the Agent did not advertise SHOULD answer `-32601` (`error.md:46`) | ⚠️ `authMethods:[]` means clients MUST NOT call it, so nothing is owed today. Crush now answers a stray call with `-32601` (`server.go:81-87`). S2 (register provider auth methods, type `agent`, trigger OAuth) is the real work |
| `logout` method | MUST exist if `auth.logout` advertised | ✅ success no-op (Crush has no auth session) |

### 3.2 Session lifecycle methods

| Method | Level | Crush status |
|---|---|---|
| `session/new` | MUST | ✅ creates SQLite session, returns `sessionId`, `modes` and `configOptions`. The `cwd` is validated against the process working directory (mismatch is rejected). `mcpServers` are connected for this session only (§3.12). `additionalDirectories` is ignored: the capability is not advertised (§3.14, O23) |
| `session/prompt` | MUST | ✅ runs coordinator, returns `StopReason` |
| `session/cancel` (notification) | MUST | ✅ cancels coordinator |
| `session/update` (notifications) | MUST | ✅ message/thought/tool_call/tool_call_update/usage emitted (see §3.3) |
| `session/close` | OPTIONAL (advertise `sessionCapabilities.close`) | ✅ implemented + advertised. Spec: closing MUST cancel ongoing work, is idempotent, and a non-existent session SHOULD still return the empty result; clients SHOULD treat `-32002` as success. Crush cancels the coordinator and always returns `{}`, which satisfies both |
| `session/list` | OPTIONAL (advertise `sessionCapabilities.list`) | ✅ implemented + advertised. `cwd`, `title`, `updatedAt` populated. ✅ **Pagination implemented**: `req.Cursor` is honoured (opaque base64 offset), `NextCursor` is set while more results remain, a malformed cursor is rejected with `-32602`, and pages are bounded at `listPageSize` (50). A `cwd` filter naming a different root returns an empty list (Crush has one workspace root). `_meta` on `SessionInfo` is still unpopulated. |
| `session/load` | OPTIONAL (advertise `loadSession`) | ✅ full-history replay (R7): user/agent text, `agent_thought_chunk`, `tool_call` starts (kind/title/rawInput), tool results with terminal status + content + `diff`, and `plan` updates. Synthesizes a start when a result has no matching call. ✅ Returns `modes` and `configOptions` like `session/resume` (O26/G7), validates `req.Cwd`, and answers `-32002` for an unknown session. `req.AdditionalDirectories` is not applied: the capability is not advertised |
| `session/resume` | OPTIONAL (advertise `sessionCapabilities.resume`) | ✅ implemented (Option B, §6.2 O5): returns mode/config state without replaying history; capability advertised. ✅ Validates `req.Cwd` and answers `-32002` for an unknown session (O22/O18). `req.AdditionalDirectories` is not applied |
| `session/delete` | OPTIONAL (advertise `sessionCapabilities.delete`) | ✅ implemented + advertised (`UnstableDeleteSession`) and **idempotent** (O21): a missing session is treated as `sql.ErrNoRows` and succeeds silently per `session-delete.md:85`, while a real store failure still surfaces as an internal error. Deleting an active session is implementation-defined |
| `session/set_mode` | OPTIONAL (legacy; prefer config options; spec notes modes will be removed in a future version) | ✅ **switches for real** (2026-11-17): the requested id is resolved against an ACP-only four-mode enum and applied by calling `SetMainAgent` for the agent half and `SetPermissionMode` for the permission half. Emits `current_mode_update`. An unknown id is `-32602`; `coder` is accepted as a legacy alias for `code` |
| `session/set_config_option` | OPTIONAL | ✅ applies thinking/model/mode and returns the complete `configOptions` list (spec MUST, `session-config-options.md:304`). Every option's `currentValue` is read from live state, so it is the value the Agent operates with (`:245`). ✅ Unknown `configId`/value answers `-32602` via `validateConfigOption` (`error.md:50`). ✅ Answers `-32002` for an unknown session |

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
| `available_commands_update` | MAY | ✅ emitted on `session/new` (S8, O17) and **executable** (O24): a leading `/name` is resolved against the same sources before `Coordinator.Run` (`internal/acp/commands.go`). Covers **user-invocable skills** (`commands.FromSkillCatalog`), **custom markdown commands** (`commands.LoadCustomCommands`), and **MCP prompts** (`commands.LoadMCPPrompts`). Commands that take arguments carry an unstructured `input.hint`; argument-less commands omit `input` entirely, since the spec makes `hint` required once `input` is present. **Shell builtins** (`provider`, `model`, `mcp`, `lsp`, `permissions`, `hook`, `option`) are **not** advertised — see §4.2. The advisory resolver, its longest-prefix matching and its argument binding are documented in §4.2 Q1 |
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
(`prompt-turn.md:17-33`): 14 values, of which Crush emits 11. The three missing
(`notice`, `compaction_update`, `compaction_summary_chunk`) are all
capability-gated on client fields the pinned SDK does not model.

### 3.4 Content blocks (prompts & outputs)

| Block | Level | Crush status |
|---|---|---|
| `text` | MUST | ✅ |
| `resource_link` | MUST | ✅ rendered inline by `buildPrompt` |
| `image` | OPTIONAL (gated by `promptCapabilities.image`) | ✅ both directions: decoded to `message.Attachment` on input (O7); streamed back as `agent_message_chunk` image blocks on output (O15, `streamAttachments`) |
| `audio` | OPTIONAL | ✅ both directions: decoded on input (O7); streamed back as audio content blocks on output (O15) |
| `resource` (embedded) | OPTIONAL (gated by `embeddedContext`) | ✅ text resources are inlined and **blob resources** are handled: image/audio blobs attach as media (MIME sniffed when undeclared), other blobs become a described placeholder (O26/G8) |
| `annotations` on blocks | OPTIONAL | ❌ ignored |

### 3.5 Tool calls

| Requirement | Level | Crush status |
|---|---|---|
| Emit `tool_call` + `tool_call_update` during execution | SHOULD | ✅ `StartToolCall`/`UpdateToolCall` from event bridge (S1); full status lifecycle (S10) |
| Tool kinds: `read/edit/delete/move/search/execute/think/fetch/switch_mode/other` | OPTIONAL taxonomy | ✅ `toolKindFor` maps Crush tools to all kinds incl. `switch_mode` |
| Tool content: `content` / `diff` / `terminal` | SHOULD | ⚠️ `content` always; `diff` emitted for `edit`/`multiedit` results (from old/new content metadata); `terminal` not emitted (deferred, O8) |
| `toolCallId`, `name`, `title`, `kind`, `status`, `locations`, `rawInput`, `rawOutput` | `title` required; `name` SHOULD when available; rest optional; **all fields except `toolCallId` are optional in updates** | ✅ `title` (human-readable: tool + primary argument), `kind`, `status`, `locations` (input `path`/`file_path`), `rawInput` (decoded JSON), `rawOutput` (tool result content), and `diff` content for `edit`/`multiedit`. ❌ `name` cannot be emitted: the SDK's `SessionUpdateToolCall`/`ToolCall` **have no `Name` field at all** (verified in v0.13.5 `types_gen.go`), so this needs an SDK bump, not a helper |
| `session/request_permission` (4 option kinds: `allow_once`/`allow_always`/`reject_once`/`reject_always`) | MUST when needed | ✅ all **4** kinds offered (`buildPermissionOptions`, `permission.go`). `reject_always` is offered but **not remembered**: the grant cache is keyed by `sessionID+toolCallID`, so a remembered decision cannot outlive its own call. That limitation is pre-existing and documented, not a spec violation — the spec only asks that the Agent decides the scope of a remembered decision |
| Unrecognised permission outcomes | Agents **SHOULD NOT** treat an outcome they don't recognise as approval (`tool-calls.md:248`) | ✅ `mapPermissionOutcome` (`permission.go`) grants on an explicit allow-list (`allow_once`, `allow_always`) and denies everything else, including `reject_always` and any unknown `optionId` (O20) |
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

Crush (client track): ⚠️ real `os.ReadFile`/`os.WriteFile` with `line`/`limit` and mkdir-parent.
Client **MUST** create the file if it doesn't exist (spec MUST) — this holds.
⚠️ Two gaps: a missing file surfaces as a generic error (`-32603`) rather than `-32002`, which the spec prescribes for a `fs/read_text_file` naming a nonexistent file (`error.md:60`); and neither method checks the requested path against the session's root set (`file-system.md:127`).
Server track delegates to the connected client via the SDK when `clientCaps.Fs` allows, otherwise falls back to the local filesystem (`agent.go:776-820`).

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
  (`agent.go:284-291`).
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
`sessionCapabilities{close,delete,list,resume}`, `auth.logout`,
and `authMethods:[]` (explicitly set to an empty, non-nil slice, so it serializes
as `[]`).

✅ No advertised capability is behaviourally empty. `additionalDirectories` was
removed from the advertised set on 2026-11-17 (§3.14, O23) precisely because it
had no enforcement behind it. What remains unimplemented is behavioural and
unadvertised: no per-session root set exists (§3.14, OQ-1), and a mode selection
is recorded and echoed without switching agents (§4.2, OQ-3).

Agent capabilities **not** advertised, and correctly so:
`sessionCapabilities.systemPrompt` (no system-prompt injection support) and
any `mcpCapabilities`. Their absence means clients must not send
`systemPrompt` or assume HTTP/SSE MCP. For `systemPrompt` the pinned SDK is the
reason it cannot be honoured: no request type carries the field and
`SessionCapabilities` has no `SystemPrompt` member, so the value is dropped at
the SDK boundary before Crush sees it (§4.1b G11).

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

✅ **DONE (2026-11-17).** `NewSession`, `LoadSession` and `ResumeSession` now
read `req.McpServers` and connect each stdio server through
`mcp.InitializeSingleWithConfig`, a new entry point that takes a
`config.MCPConfig` directly instead of requiring a store entry.

**How the collision was solved.** Every registry in `internal/agent/tools/mcp`
(`sessions`, `states`, `allTools`, `allPrompts`, `allResources`, `gens`,
`renewMus`, `authURLs`) is keyed by server name alone, so two sessions asking for
a server called `github` would have overwritten each other's connection and
leaked the loser's stdio child. Rather than re-keying eight maps and ~40
functions, the registry name embeds the owning session
(`mcp.ACPServerName` → `acp:<sessionID>:<name>`), which makes each entry
single-tenant. `mcp.ACPSessionOf` recovers the owner.

Three supporting pieces make that safe:

- `mcp.TeardownSession(sessionID)` closes every server a session injected. Called
  from `session/close` and `session/delete`, so a closed session frees its
  processes (`session-setup.md:313`).
- `reconcile` skips `acp:`-prefixed names. Without this a config reload would
  have emitted `reinitRemove` for servers that are not in the config store by
  design, tearing down a live session's connection — a correctness bug, not a
  gap.
- `filterOtherSessionsMCPTools` runs in `PrepareStep` and drops tools belonging
  to a *different* ACP session. The tool palette is process-wide, so without it
  a run could offer a tool backed by another session's connection.

The connection uses `context.WithoutCancel(ctx)`: the `session/new` request
context is cancelled the moment the response is written, and the MCP session
must outlive it.

**Known limitations, recorded rather than hidden.** Tool names carry the
registry key, so the model sees `mcp_acp_<sessionID>_github_search`. MCP prompts
(`availableCommandsUpdate`), resource completions and the `/mcp` UI all read the
config-file registry and so do not surface ACP-injected servers. OAuth, HTTP/SSE
headers, `EnabledTools`/`DisabledTools` and the channel subsystem have no ACP
equivalent and are not applied — none of them are advertised over ACP anyway.
`env` is additive to the inherited process environment, matching
`createTransport`, rather than an exclusive set.

Security constraints the spec adds (`session-setup.md:200-210`): launch with
`command` and `args` as separate arguments (never through a shell); never
echo `env` or `headers` values into session updates, errors, `_meta`,
`session/list`, replayed history, model context, or command lines; never log
or persist them. `config.MCPConfig` carries `env`, so it holds no secrets on the
wire.

### 3.13 Transport Protocol

ACP uses JSON-RPC 2.0 for all message exchange. Key requirements:

- JSON-RPC messages **MUST** be UTF-8 encoded.
- Messages are delimited by newlines (`\n`) and **MUST NOT** contain embedded newlines.
- The agent **MAY** write UTF-8 strings to stderr for logging; the client **MAY** capture, forward, or ignore these logs.
- The agent **MUST NOT** write anything to `stdout` that is not a valid ACP message; the client **MUST NOT** write anything to the agent's `stdin` that is not a valid ACP message.
- Stdio is a **MUST** baseline for every Agent. HTTP and SSE are optional. Custom transports **MAY** be implemented provided the JSON-RPC message format and lifecycle requirements are preserved.

### 3.14 Security (`security.md`, workspace roots)

Added 2026-11-17: `security.md` (79 lines) had no section in this plan, so its
agent-side duties were never tracked. Most are SHOULD-level and are recorded
here once, not re-litigated.

**Trust model** (`security.md:11-18`): the Agent runs with the permissions of
its process; ACP does not sandbox it. Every Agent-authored string, URI and path
is untrusted, *including replayed content*. `clientInfo`/`agentInfo` are
self-reported and MUST NOT drive security decisions.

**The root set: advertised capability retracted, enforcement still open.**
`session-setup.md:444-452` defines the session's effective root set as
`[cwd, ...additionalDirectories]` and says it SHOULD bound tool filesystem
operations. Crush has no per-session root concept at all, so the earlier state
advertised `sessionCapabilities.additionalDirectories` while ignoring it.

Resolved 2026-11-17 by **retraction** (O23), not enforcement:

- `sessionCapabilities.additionalDirectories` is no longer advertised;
- the request field is ignored rather than half-applied on
  `session/new`/`load`/`resume`;
- the `session/list` echo and its `sync.Map` storage were removed;
- `cwd` **is** validated and compared against this process's workspace on all
  three setup methods, since it is part of the same root set.

The advertisement therefore no longer promises a scope that does not exist.
What remains open is enforcement itself (OQ-1): `internal/agent/tools` still has
no per-session root set, so `bash`, `view`, `edit` and friends operate on the
whole filesystem. The spec's check is a SHOULD, and a retracted capability is an
honest position while Crush serves one workspace per process.

**Agent duties Crush does not implement** (all SHOULD, none blocking):

| Duty | Spec | Status |
|---|---|---|
| Treat embedded resources, files, tool results and MCP results as data, not instructions | `security.md:50` | ⚠️ prompt structure only; no explicit delimiting |
| Request permission before sending data off-machine after reading untrusted content | `security.md:51` | ❌ not distinguished from any other network tool |
| Request permission before writing files that control execution or approval (VCS hooks, editor/agent settings, MCP config) | `security.md:52` | ❌ no such classification in the permission layer |
| Apply the root set to URIs in prompts | `security.md:41` | ❌ still open — no root set exists to apply (OQ-1) |
| Scope `session/list`, loading and resumption to the owning user | `security.md:60` | N/A today — Crush serves one user per process |

**Client duties** (peer side; recorded so they are not mistaken for our gaps):
sanitize Agent-authored Markdown/HTML, do not auto-fetch Agent-supplied remote
URIs, do not open non-`http(s)` links without confirmation, reveal or remove
bidi/zero-width/tag characters, neutralize terminal control sequences
(`security.md:20-41`).

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
  `session-config-options.md:207-209`). ✅ **RESOLVED without the SDK (2026-11-17)**
  by taking the escape hatch the spec names one line later: "Agents that need to
  support older Clients should omit the boolean option or provide a `select`
  fallback" (`:209`). `thinking` is now a `select` with `on`/`off`, so no
  boolean option is ever emitted and the MUST cannot be violated. The `req.Boolean`
  branch is kept in `validateConfigOption` so a client that cached the old
  advertisement still works. Same treatment will be needed if `_yolo` (§4.2 Q2)
  is ever added — it cannot ship as a boolean gate either.
- **MCP stdio connect** (see §3.12) — ✅ IMPLEMENTED (2026-11-17).
- **`session/load` replay completeness.** ✅ IMPLEMENTED (R7): full history replays — user/agent text, thought chunks, tool calls (kind/title/rawInput), and tool results with terminal status and content.
- **`$/cancel_request` / error `-32800`.** ✅ CONFORMANT, not a gap. `cancellation.md:23-25`
  requires the implementation to send *either* a valid response (e.g.
  `session/prompt` with `cancelled`) *or* an error with `-32800`. The SDK
  cancels the in-flight request's `context.Context`, `Prompt` maps that to
  `StopReasonCancelled`, and `-32800` is never needed. See §3.9.
- **`_meta` passthrough** (see §3.11) — ✅ IMPLEMENTED (S7): `session/prompt`
  `_meta` is echoed on the `PromptResponse` and on every `session/update` for
  that turn, and `session/load` replays echo the load request's `_meta`. W3C
  trace-context values (`traceparent`/`tracestate`/`baggage`) are reserved but
  never *generated*; that remains open.
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
  2026-11-13, mostly implemented same day). The spec pins specific codes
  (`error.md:29-60`):
  - unknown session on a session-scoped request → `-32002` with
    `data.sessionId`. ✅ `session/prompt` calls `requireSession` and answers
    `-32002` when the store has no such session; previously the store error
    surfaced as `-32603`. ⚠️ Narrower than the spec: `error.md:56` also covers a
    session "not active on the connection", and `requireSession` checks DB
    existence only, so any session id known to the store is accepted from any
    connection. No per-connection active set exists. ✅ The guard now covers
    `session/prompt`, `session/load`, `session/resume`, `session/set_mode` and
    `session/set_config_option` (O18); only the per-connection active-set idea
    remains (G12).
  - invalid `session/list` cursor, or an unknown `configId`/value in
    `session/set_config_option`, or an unknown mode in `session/set_mode`
    → `-32602`. ✅ implemented: cursors are validated in `ListSessions`,
    `validateConfigOption` checks all three option families (thinking, model,
    mode), and `SetSessionMode` rejects an unknown or non-selectable mode id.
  - a request other than `initialize` before initialization completes →
    `-32600`. ✅ implemented (O18): `dispatcher.requireInitialized` gates every
    request method; `session/cancel` is a notification and is exempt.
  - a method the Agent did not advertise → `-32601`. ✅
    `dispatcher.Authenticate` now returns `acp.NewMethodNotFound`.
- **`usage_update` cost fields.** Spec requires `amount` + `currency` when
  `cost` is present and non-null `used`/`size` always. ✅ emitted with both
  token counts; cost is omitted rather than sent partially
  (`event_bridge.go:698-729`).

### 4.1b Conformance gaps found in the 2026-11-17 audit

Re-read all 22 files in `docs/acp/protocol/v1/` against the code. Items below
were previously marked done or simply unlisted; each is a real divergence.

| # | Gap | Spec | Status |
|---|---|---|---|
| G1 | ~~`reject_always` is never offered; an unrecognised `optionId` is read as **approval**~~ | `tool-calls.md:232-248` | ✅ **fixed (O20)**: all four kinds offered; `mapPermissionOutcome` uses an allow-list so an unknown outcome denies |
| G2 | ~~`session/delete` is not idempotent~~ | `session-delete.md:85` | ✅ **fixed (O21)**: a missing session (`sql.ErrNoRows`) is success; other store errors still surface |
| G3 | ~~`additionalDirectories` advertised but never enforced or applied~~ | `session-setup.md:359,444-452` | ✅ **resolved by retraction (O23)**: the capability is no longer advertised and the field is ignored. Enforcement remains open — see OQ-1 |
| G4 | ~~`cwd` not validated on `session/load` / `session/resume`~~ | `session-setup.md:359` | ✅ **fixed (O22)**: `validateCwd` is shared by `session/new`, `session/load` and `session/resume` |
| G5 | ~~No `category:"mode"` config option alongside `modes`~~ | `session-config-options.md:462-471` | ✅ **fixed (O25)**: a `mode` select carries `category:"mode"` and its selection is recorded per session |
| G6 | ~~`currentModeId` hardcoded to `"coder"`~~ | `session-modes.md:58-64` | ✅ **fixed (O25)**: derived from the session's selection, always one of `availableModes`; `task` excluded; an empty or stale selection falls back to a real mode |
| G7 | ~~`session/load` returns an empty result~~ | `session-setup.md:200` | ✅ **fixed (O26)**: returns `modes` and `configOptions`. ✅ The field is implementable — `LoadSessionResponse` has both |
| G8 | ~~Embedded `BlobResourceContents` dropped from the prompt~~ | `content.md:148-160` | ✅ **fixed (O26)**: decoded; image/audio blobs attach as media (sniffed when MIME is undeclared), anything else becomes a described placeholder |
| G9 | ~~Advertised slash commands are not runnable~~ | `slash-commands.md:99` | ✅ **fixed (O24)**: a leading `/name` resolves against skills, custom commands and MCP prompts. ACP-local resolver; see OQ-2 |
| G10 | ~~Client-track `fs` returns `-32603` for a missing file~~ | `error.md:60` | ✅ **fixed (O26)**: `fs.ErrNotExist` answers `-32002` with `data.path` |
| G11 | `systemPrompt` not supported | `session-setup.md:363-421` | **SDK-blocked, not a Crush decision.** `SystemPrompt` appears on no request type (`NewSessionRequest`/`LoadSessionRequest`/`ResumeSessionRequest` carry only `Meta`, `AdditionalDirectories`, `Cwd`, `McpServers`) and `SessionCapabilities` has no `SystemPrompt` field in v0.13.5. The field is dropped at the SDK boundary, so Crush neither sees it nor needs to reject it. Correctly unadvertised |
| G12 | `active session on the connection` not tracked | `error.md:56` | ⚠️ `requireSession` checks DB existence only. ✅ It now guards `prompt`, `load`, `resume`, `set_mode` and `set_config_option` (O18); the per-connection active-set idea is still unimplemented |


### 4.2 Verification: commands, YOLO, and feature gaps (2026-09-17)

Answers to three verification questions asked on `feature/acp`:

#### Q1 — Are commands supported from ACP? How to implement Crush commands in the bridge?

**Resolved 2026-11-17 (O24): the gap was execution, not advertising.** The
`available_commands_update` notification (§3 spec: **MAY**) is emitted on
`session/new` (`agent.go` `availableCommandsUpdate`) and covers three sources:
user-invocable skills (`commands.FromSkillCatalog`), custom markdown commands
(`commands.LoadCustomCommands`) and MCP prompts (`commands.LoadMCPPrompts`).
Commands that take arguments carry an `input.hint`; argument-less ones omit
`input` entirely, since the spec requires `hint` whenever `input` is present
(`slash-commands.md:70-72`).

The spec says commands "are run as part of regular `session/prompt` requests
where the Client includes the command text in the prompt", and that "the Agent
recognizes the command prefix and processes it accordingly"
(`slash-commands.md:9,99`). `Prompt` now resolves a leading `/name` against the
same three sources before `Coordinator.Run`, in `internal/acp/commands.go`:

- **skill** → `Skill.FormatInvocation()`, plus any trailing argument text;
- **custom markdown command** → `$ARG_*` substitution, arguments bound
  positionally from the trailing text;
- **MCP prompt** → `commands.GetMCPPrompt(store, clientID, promptID, args)`.

Matching is longest-prefix, because command ids embed `:` prefixes and path
separators (`user:foo`, `project:sub/bar`). An unknown `/name` falls through to
the model unchanged, and so does a command whose required arguments are missing —
sending a template with `$ARG_X` still in it would be worse than the raw text.

The resolver is an **ACP-local copy** of the TUI's logic rather than a shared
helper; the duplication and its drift risk are recorded in **OQ-2**. The TUI
resolves commands in `internal/ui/model` and can prompt for arguments through a
dialog, which a prompt has no equivalent of.

**Shell builtins** (`provider`, `model`, `mcp`, `lsp`, `permissions`, `hook`,
`option`) stay out of scope. They are registered via `shell.RegisterBuiltin`
against the bash config system (crushrc) and mutate configuration rather than
producing a prompt. Exposing them would need an ACP-facing entry point of their
own; the two candidate designs are recorded below for completeness.

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

- **Advertise** in `configOptionsFor` (`agent.go:687`):
  `SessionConfigOptionBoolean{Id:"yolo", Name:"YOLO", Type:"boolean", CurrentValue: cfg.Overrides().SkipPermissionRequests, Description: ..., Category: category("_yolo")}`.
- **Apply** in `SetSessionConfigOption` (`agent.go:374`): add a branch
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
| `authenticate` method | Conditional on advertisement — only clients whose `agentCapabilities.auth` covers it may call it; an unadvertised capability-gated method `SHOULD` answer `-32601` (`error.md:46`) | ⚠️ S2: register provider auth methods as `authMethods` entries (type `agent`); trigger OAuth on `authenticate`. Today `authMethods:[]`, so nothing is owed and the call answers `-32601` | Yes — return `auth_required` |
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
| `available_commands_update` | ✅ advertised (skills + custom cmds + MCP prompts, with `input.hint`) **and executable** | — | O17 + O24: a leading `/name` resolves against the same three sources in `internal/acp/commands.go` (Q1, OQ-2) |
| `usage_update` | ✅ | — | |
| `current_mode_update` / `config_option_update` / `session_info_update` | ✅ | — | |
| `session/resume` | ✅ (Option B) | — | Returns mode/config state without replay; capability advertised. Validates `cwd` and answers `-32002` for an unknown session (G4/O18); `additionalDirectories` is not applied because the capability is not advertised |
| Config option with `category:"mode"` alongside `modes` | ❌ | Yes | `session-config-options.md:462-471` SHOULD; Crush sends only `modes` (G5) |
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
| `session/list` pagination | ✅ | — | O19: opaque offset cursors, `nextCursor` while more remain, 50-entry pages, `-32602` on a malformed cursor, `cwd` filter |
| Terminal auth methods | ❌ | Yes | SDK **does** model `auth.terminal` + `AuthMethod.Terminal`; Crush advertises no auth methods |
| `notice` | ❌ | Not with this SDK | Gated on `clientCapabilities.session.notices`, which v0.13.5 does not model |
| `compaction_update` / `compaction_summary_chunk` | ❌ | Not with this SDK | Gated on `clientCapabilities.session.compaction`; also must be replayed by `session/load` |

**Summary:** Everything left is either a small Crush-side change or an SDK
limitation. Blockers: (1) pinned SDK lacks `messageId`, non-`Unstable`
elicitation, the `session.configOptions.boolean` client gate, and a `name`
field on tool calls (4 items); (2) missing Crush work: per-session MCP connect
(the only MUST gap), PTY for terminals, and the slash-command recogniser in
`session/prompt` (which blocks every advertised command, not just builtins);
(3) deliberate omissions (terminal auth, `-32800`).

## 5. Implementation Roadmap (current state)

Phases reflect **verified code state** on branch `feature/acp`.

### Server — done
- [x] **Phase 1** SDK integration + transport (`server.go`, `NewServer`).
- [x] **Phase 2** Agent methods: `Initialize`, `NewSession`, `Prompt`,
  `Cancel`, `ListSessions`, `CloseSession`, `ResumeSession`,
  `SetSessionMode`, `SetSessionConfigOption` — all implemented (the last three
  were stubs when this line was first written).
- [x] **Phase 3** Permission bridge (`allow_always` cache). ✅ All four spec
  option kinds are offered and an unrecognised outcome denies (G1, O20).
  Limitation: the grant cache is keyed by session+toolCall, so a remembered
  decision cannot outlive the call it was granted for.
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

### Open questions (raised 2026-11-17 when Tier 3b landed)

Each is a decision the Tier 3b batch deliberately did not make. They are recorded
here rather than in a separate file so they travel with the plan they belong to.

**OQ-1 — Should the root set be enforced?**

Tier 3b **retracted** `sessionCapabilities.additionalDirectories` instead of
enforcing it, because Crush has no per-session root concept in
`internal/agent/tools` and a retracted capability is an honest position while
Crush serves one workspace per process. What is still unaddressed is the spec's
SHOULD at `session-setup.md:444-452`.

Enforcement would need:

1. a `RootsContextKey` beside `SessionIDContextKey` in
   `internal/agent/tools/tools.go:22-33`, populated where the tool context is
   built (`internal/agent/agent.go:807`, `:949`);
2. a consumer in every path-touching tool: `view.go:113`, `write.go:66`,
   `edit.go:72`, `multiedit.go:77` (and a documented decision for `bash`, which a
   path check cannot confine without a sandbox);
3. a session→roots lookup on `app.App`, since the current storage was deleted;
4. comparison by path component on the **resolved** path, failing closed
   (`session-setup.md:452`).

The question for the maintainer is whether a single-workspace ACP server should
carry this at all, or whether the retraction is the correct resting state.

**OQ-2 — Where should the slash-command resolver live?**

Tier 3b added an **ACP-local** resolver in `internal/acp/commands.go`. The TUI
keeps its own copy in `internal/ui/model/ui.go:2783-2838`.

The duplication is real and bounded:

| Piece | TUI | ACP |
|---|---|---|
| Argument substitution | `substituteArgs` (`ui.go:2832-2838`) | `substituteArgs` (`commands.go`) |
| Argument collection | `dialog.NewArguments` — asks the user | positional binding from trailing text |
| Skill invocation | `Skill.FormatInvocation()` | identical |
| MCP prompt | `runMCPPrompt` | `commands.GetMCPPrompt` |

The documented drift risk: if the TUI's `$NAME` rule or the argument-collection
semantics change, the ACP copy will not follow. The alternative is one exported
resolver in `internal/commands` that both paths call, which was declined for this
batch to keep the TUI untouched.

**OQ-3 — RESOLVED 2026-11-17: `session/set_mode` now switches for real.**

The earlier Tier 3b batch made mode switching advisory. That left a real MUST
violation: `session-config-options.md:245` requires an option's `currentValue`
to be the value the Agent operates with, and the advertised value was a
per-session record while the agent never moved.

The fix flattens Crush's mode ring into an ACP-only four-mode enum, which is
what makes a single-value `modes` field able to express the TUI's two-axis
state:

| ACP mode id | Agent | PermissionMode |
|---|---|---|
| `code` | `coder` | Normal |
| `plan` | `plan` | Normal |
| `yolo` | `coder` | Yolo |
| `sysadmin` | `coder` | Sysadmin |

Verified premise: YOLO and Sysadmin really are "coder with a permission toggle
on" — `setYolo` only calls `setPermissionMode` and leaves the agent alone
(`ui.go:4873-4877`), and `switchPlanToYolo` forces `uiInputModeCode`
(`ui.go:5048-5057`). `SetPermissionMode` was already on the
`permission.Service` interface and already delegated by
`acpPermissionService`, so the permission half needed no new plumbing.

`SetSessionMode` and the `mode` config option both funnel through `applyMode`,
which calls `SetMainAgent` and `SetPermissionMode` and then echoes
`current_mode_update`. `currentMode` reads the answer back from the live
coordinator (`Coordinator.MainAgentName`, added for this) and permission service
rather than a private map, so `currentValue` stays truthful even when the other
session or the TUI moved the workspace-wide agent.

Two limitations recorded rather than hidden:

1. `SetMainAgent` is still workspace-wide. With two concurrent ACP sessions the
   last switch wins the agent for both, so the second session's `currentValue`
   reflects reality — it reports what the agent is doing — but the user's
   earlier choice is not per-session honoured. Real per-session agents need
   coordinator work, not an adapter change.
2. `plan` + `yolo` is reachable in the TUI but has no ACP id. Selecting `plan`
   resets the permission mode to Normal, which raises oversight rather than
   lowering it, so the degradation is in the safe direction.

#### Preliminary research: what "mode" means in Crush

Three different concepts share the word. Only the first is what ACP's `modes`
field means; the third is a separate axis the flattened enum above absorbs.

| Layer | Values | Where | ACP-visible |
|---|---|---|---|
| **Selectable agents** (`modes`) | `coder`, `plan` | `config.go:62-63`; built in `SetupAgents` (`config.go:1155-1191`) | ✅ yes, via `modeState` |
| **Agents, internal** | `task` | `config.go:64`; `SetupAgents:1168`; consumed by `internal/agent/agent_tool.go:36` | ⚠️ **leaked** before Tier 3b — `modeState` and `SetSessionMode` accepted it; now excluded by `selectableModes` |
| **Custom agents** | any key in `cfg.Agents` | user config, `Disabled` filtered | ✅ yes — the set is open-ended |
| **TUI input mode** | `uiInputModeCode`, `uiInputModePlan` | `internal/ui/model/ui.go:136-137`; maps to `coder`/`plan` via `setInputMode` (`ui.go:5067-5075`) | ❌ a UI affordance over the same agents |
| **Permission mode** | `PermissionModeNormal`, `PermissionModeYolo`, `PermissionModeSysadmin` | `internal/permission/permission.go:41-46` | ❌ orthogonal state on `permission.Service`, not an agent |

Findings carried into the implementation:

1. **`task` must never be a session mode.** It is the subagent profile with
   `resolveTaskTools` and no MCP/LSP access; selecting it as the main agent would
   silently degrade the session. Fixed in `selectableModes`.
2. **YOLO is not a mode.** It lives on `permission.Service`; §4.2 Q2 already
   scopes it correctly as an `_yolo` boolean config option.
3. **A real switch is workspace-wide.** Wiring `set_mode` to `SetMainAgent`
   needs per-session agent selection in the coordinator first. That is the open
   question, not a missing field.

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
- [x] **O3.** `additionalDirectories` capability + handling in `/new`, `/list` (echo back via `SessionInfo`). See §6.2. ✅ **Superseded 2026-11-17 by O23**: the capability was retracted rather than left unenforced, so there is no echo to maintain. The decision record in §6.2 is kept for history.
- [x] **O4.** `session/delete` + `sessionCapabilities.delete` — ✅ IMPLEMENT
  (`UnstableDeleteSession`). See §6.2. ⚠️ Not idempotent: a missing session
  answers `-32603` instead of succeeding silently or answering `-32002` (G2).
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

**Tier 3b — Gaps found in the 2026-11-17 audit (§4.1b)**
- [x] **O20. Permission option kinds and outcomes (G1).** ✅ IMPLEMENTED
  (2026-11-17): `buildPermissionOptions` emits all four spec kinds, and
  `mapPermissionOutcome` grants only on an explicit allow-list, so `reject_always`
  and any unrecognised `optionId` deny (`tool-calls.md:248`). `reject_always` is
  not remembered: the grant cache is keyed by `sessionID+toolCallID`. Tests:
  `TestBuildPermissionOptions`, `TestMapPermissionOutcome`.
- [x] **O21. `session/delete` idempotency (G2).** ✅ IMPLEMENTED (2026-11-17):
  `UnstableDeleteSession` treats `sql.ErrNoRows` as success and still surfaces
  other store errors (`session-delete.md:85`). Fixed at the agent, not in
  `session.Service.Delete`, so `crush session rm` and the HTTP path are
  unchanged. Tests: `TestUnstableDeleteSession`,
  `TestConformanceDeleteIsIdempotent`.
- [x] **O22. `cwd` validation on load/resume (G4).** ✅ IMPLEMENTED
  (2026-11-17): `validateCwd` is shared by `session/new`, `session/load` and
  `session/resume`, and `requireSession` answers `-32002` on all three. The
  `additionalDirectories` half of this item was resolved by retraction (O23).
- [x] **O23. Root set advertising (G3, §3.14).** ✅ RESOLVED BY RETRACTION
  (2026-11-17): `sessionCapabilities.additionalDirectories` is no longer
  advertised, the `sync.Map` echo storage is gone, and the request field is
  ignored rather than half-applied. Enforcement is still open — see **OQ-1**.
- [x] **O24. Slash-command recogniser (G9).** ✅ IMPLEMENTED (2026-11-17):
  `internal/acp/commands.go` resolves a leading `/name` against skills, custom
  commands and MCP prompts before `Coordinator.Run`, with longest-prefix
  matching, positional argument binding and pass-through for unknown names.
  ACP-local copy, not shared with the TUI — see **OQ-2**. Tests:
  `internal/acp/commands_test.go`.
- [x] **O25. Mode/config parity (G5, G6).** ✅ IMPLEMENTED (2026-11-17) as an
  **advisory** mode: a `category:"mode"` select is emitted alongside `modes`,
  its selection is recorded per session, `currentModeId` is derived from that
  selection (never a hardcoded `"coder"`), `selectableModes` sorts and excludes
  `config.AgentTask`, and `validateConfigOption` accepts `configId:"mode"`.
  No call to `SetMainAgent`. Remaining question: **OQ-3**.
- [x] **O26. Small conformance leftovers.** ✅ IMPLEMENTED (2026-11-17):
  `session/load` returns `modes`/`configOptions` (G7); embedded
  `BlobResourceContents` is decoded and attached as media or described as a
  placeholder (G8); the client-track `fs` answers `-32002` for a missing file
  (G10). Tests: `TestBuildPromptBlobResource`, `TestClientReadTextFile`.


> **Deferred** — grouped by the *actual* blocker (SDK vs missing Crush work):
> - **SDK-gated (UNSTABLE in pinned v0.13.5):** **S5** `messageId` — the field exists
>   on chunks and `PromptRequest`/`PromptResponse` but is flagged UNSTABLE; **S6**
>   elicitation — only `Unstable*` variants ship; terminal auth
>   methods (`clientCapabilities.auth.terminal`, `type:"terminal"`) are likewise
>   not modelled. The boolean config-option client gate
>   (`ClientCapabilities.session.configOptions.boolean`) is still unreadable, but
>   it no longer blocks anything: the spec's `select` fallback is used instead
>   (§4.1). Revisit on the next SDK release.
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

- **O25 — mode switching stays advisory.** The `mode` config option and
  `session/set_mode` both record the selection in `crushAgent.activeMode` and
  echo it, but neither calls `SetMainAgent`. Reason: `Coordinator.SetMainAgent`
  (`internal/agent/coordinator.go:337-347`) mutates the shared coordinator, so a
  switch is workspace-wide rather than per-session — harmless under `crush acp`
  (one session per process), wrong behind the HTTP backend, and against
  `tool-calls.md:251` ("should not switch to a mode that reduces user
  oversight without user action"). Open question tracked as OQ-3.

- **O23 — retract rather than enforce.** Advertised-but-unenforced
  `additionalDirectories` was removed from `Initialize`, along with the
  echo-only `sync.Map` and the `session/list` field. A capability the Agent
  cannot honour is worse than an absent one, and enforcement needs a root set
  the tool layer does not have. Open question tracked as OQ-1.

- **O24 — ACP-local resolver, not a shared one.** `internal/acp/commands.go`
  duplicates the TUI's `substituteArgs` and skill/MCP resolution rather than
  exporting a shared helper from `internal/commands`. The TUI's path runs
  through `dialog.ActionRunCustomCommand` and can ask the user for arguments;
  a prompt cannot, so the two differ in argument collection anyway. Drift risk
  recorded as OQ-2.

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

- **O3 — `additionalDirectories`: SUPERSEDED 2026-11-17 by O23.** The original
  decision was to advertise `sessionCapabilities.additionalDirectories`, accept
  `NewSessionRequest.AdditionalDirectories`, store them per session in a
  `sync.Map` and echo them back via `SessionInfo.AdditionalDirectories` in
  `ListSessions`, with the note that Crush has no multi-root model so they were
  persisted for round-tripping only. That left an advertised capability with no
  behaviour behind it, so the capability, the storage and the echo were all
  removed in Tier 3b. Enforcement is tracked as OQ-1.

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
  answers `acp.NewMethodNotFound` (`-32601`), which is correct because no auth
  method is advertised (`server.go:81-87`).

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
- [x] **O17. `available_commands_update` completeness** — ✅ IMPLEMENTED
  (2026-11-13): user-invocable skills, custom markdown commands and MCP
  prompts are all advertised, and commands with arguments carry an
  unstructured `input.hint` built from `Argument.Title`/`ID`
  (`newAvailableCommand`, `commandHint`). Shell builtins stay out on purpose.
  (`agent.go` `availableCommandsUpdate`.) ⚠️ Advertising only: the prompt-side
  recogniser is missing, so the commands cannot actually run (O24, G9).
- [x] **O18. Spec error codes** — ✅ IMPLEMENTED (2026-11-17): `-32602` for a
  bad `session/list` cursor and an unknown `configId`/value/mode, `-32601` for
  `authenticate`, `-32600` for any request before `initialize`
  (`dispatcher.requireInitialized`), and `-32002` for a missing session on
  `prompt`/`load`/`resume`/`set_mode`/`set_config_option`. Remaining and
  un-implemented: the per-connection active-session concept that `error.md:56`
  assumes but `requireSession` does not implement (G12). All SHOULD-level.
  See §4.1, §4.1b. Tests: `TestConformanceRejectsRequestBeforeInitialize`,
  `TestConformanceUnknownSessionOnSetupMethods`.
- [x] **O19. `session/list` pagination** — ✅ IMPLEMENTED (2026-11-13):
  opaque offset cursors, `nextCursor` while more remain, 50-entry pages,
  `-32602` on a malformed cursor, and `cwd` filtering. See §3.2.
  (`agent.go` `ListSessions`, `encodeListCursor`, `decodeListCursor`.)
  Tests: `TestListSessionsPaginates`, `TestListSessionsFiltersByCwd`,
  `TestListSessionsEmptyReturnsEmptySlice`.

**Tier 4 — Tests** (gate each Tier 1–3 item)
- [ ] Regression tests per missing update type; elicitation roundtrip; `_meta`
  passthrough; cancellation `cancelled` outcome.
  Already present and passing: `TestConformanceInitialize/NewSession/PromptEndTurn/
  SessionSetupAndUnsupportedMethods/SessionLifecycle/SessionLoad/CancelDuringPrompt/
  MultipleSessions/SessionUpdates/ContentBlockVariants/InitializeCapabilities/
  InvalidSessionError` and `TestGolden*`, `TestZed*`, `TestStdio*` in `test/acp/spec/`.
- [x] Tier 3b coverage added 2026-11-17:
  `TestConformanceRejectsRequestBeforeInitialize` (`-32600`),
  `TestConformanceUnknownSessionOnSetupMethods` (`-32002` on load/resume/
  set_mode/set_config_option), `TestConformanceDeleteIsIdempotent`,
  `TestBuildPermissionOptions`/`TestMapPermissionOutcome` (four kinds,
  allow-list), `TestInitializeOmitsAdditionalDirectories`,
  `TestNewSessionIgnoresAdditionalDirectories`, `TestValidateCwd`,
  `TestUnstableDeleteSession`, `TestConfigOptionsIncludesMode`,
  `TestSelectableModesExcludesTaskAndDisabled`,
  `TestDefaultModeFallsBackWhenCoderMissing`, `TestModeStateReportsCurrentMode`,
  `TestBuildPromptBlobResource`, and the whole of
  `internal/acp/commands_test.go` for the slash-command resolver.
- [ ] Still uncovered: the per-connection active-session concept (G12), MCP
  prompt resolution end-to-end (needs a live MCP client), and blob attachments
  flowing all the way through `Coordinator.Run`.

## 7. Test strategy

```bash
go test ./internal/acp/... -count=1            # unit + integration
go test ./test/acp/spec/... -count=1           # conformance, golden, Tangerg interop
CRUSH_BIN=$(go build -o /tmp/crush .) \
  go test -tags e2e ./test/acp/ -count=1       # subprocess e2e
./test/acp/run.sh all                          # full runner (unit/spec/interop/sdk/e2e)
```
- Unit: permission logic (four kinds + allow-list), server roundtrip
  (`TestServerStartEndToEnd`), event bridge lifecycle, client
  subscribe/permission/IO, mode selection helpers, and the slash-command resolver
  (`internal/acp/commands_test.go`).
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
- **Tool `name` field and `terminal` tool content** (§3.5, §4.1) — `name` is SHOULD on the wire: the SDK's `SessionUpdateToolCall`/`ToolCall` have no `Name` field at all and no start/update helper, so it cannot be emitted with v0.13.5; `terminal` needs the deferred terminal subsystem (O8).
- **`messageId`** on chunks (MAY in spec; UNSTABLE in pinned SDK — S5).
- **Elicitation** (`elicitation/create`/`complete`) on either side (S6) — only `Unstable*` types ship; server-side is agent-initiated, client-side unimplemented.
- **W3C trace-context generation** (`traceparent`/`tracestate`/`baggage` values are reserved but never generated; `_meta` echo itself is implemented, S7).
- **`$/cancel_request` → `-32800`** explicit handling (§3.9) — not a gap: the
  spec accepts `StopReasonCancelled` in place of the error, which is what Crush
  returns when the SDK cancels the request context.
- **Terminal callbacks** in client track (O8, deferred) + terminal-auth methods (§4.1 — SDK *does* model these; not implemented).
- **Annotations on content blocks** (O16, MAY, low priority).
- **`systemPrompt`** (`session-setup.md:363-421`) — correctly unadvertised and
  **SDK-blocked**: no request type carries the field and `SessionCapabilities`
  has no `SystemPrompt` member in v0.13.5 (G11).
- **Session list pagination** (SHOULD) — ✅ DONE (O19): cursors, `nextCursor`, page bound and `-32602` on a bad cursor are all implemented; only `SessionInfo._meta` remains unpopulated.
- **`available_commands_update`** (MAY) — ✅ DONE (O17 + O24): skills, custom markdown commands and MCP prompts are advertised with `input.hint` **and** resolved on the prompt path. Shell builtins are still not exposed at all.
- **Spec error codes** (SHOULD) — ✅ DONE (O18/O20/O21): `-32602` (bad cursor, unknown `configId`/value/mode), `-32601` (`authenticate`), `-32002` on `prompt`/`load`/`resume`/`set_mode`/`set_config_option`, `-32600` before initialize, and `-32002` for a missing file in the client fs track. Remaining: the per-connection active-session concept (G12).
- **Mode switching is workspace-wide** (O25/OQ-3) — `set_mode` really switches now, but `SetMainAgent` is process-wide, so two concurrent ACP sessions share one agent. `currentValue` is read back from live state, so it stays truthful.
- **Slash-command resolver duplication** (O24/OQ-2) — the ACP copy in `internal/acp/commands.go` and the TUI copy in `internal/ui/model` are separate.
- **ACP-injected MCP servers are keys, not config entries** (§3.12) — their tool names embed the session id, and MCP prompts / resource completions / the `/mcp` UI read the config-file registry, so they do not surface them.
- **Root-set enforcement** (OQ-1) — the capability is no longer advertised (§3.14), but no tool-layer root set exists either.
- **Session compaction / notices** (`compaction_update`, `compaction_summary_chunk`, `notice`) — MAY update types in v1, each gated on a `clientCapabilities.session.*` field that the pinned SDK does not model. Not emitted; also not replayable on `session/load` (§3.3).

Resolved 2026-09-22: `session/load` full-history replay (R7, tool/thought/result parts), tool-call `pending`/`failed` lifecycle (S10), tool `title`/`locations`/`rawInput`/`rawOutput`/`diff`, `plan` updates from todos results (S4), `_meta` echo (S7), `session/resume` (O5), config `category` (O14), output-side image/audio (O15).

Resolved 2026-11-13: `session/list` pagination (O19) and command advertisement
completeness for skills/custom commands/MCP prompts with `input.hint` (O17).

Audited 2026-11-17: all 22 spec files re-read against the code. That pass found
twelve gaps the plan had either marked closed or never listed — see §4.1b and
Tier 3b (O20–O26). All seven Tier 3b items were implemented the same day.

A second pass the same day enumerated every Agent-side MUST in the v1 prose and
found four unmet. All four are now closed:

| MUST | Spec | Resolution |
|---|---|---|
| stdio MCP support | `session-setup.md:458,466` | `req.McpServers` connected per session (§3.12) |
| no boolean option unadvertised | `session-config-options.md:207` | `thinking` became a `select` (§4.1) |
| every path absolute | `overview.md:160`, `tool-calls.md:307,353` | `absPath` on tool locations and diff paths |
| `currentValue` is what the Agent uses | `session-config-options.md:245` | four-mode enum applied for real; value read from live state |

All baseline methods, the permission bridge, fs read/write, text/resource_link
content, `session/close`/`list`/`delete`, stdio transport, per-session MCP, and
capability advertising are present. **No MUST-level gap remains.** Three caveats
are behavioural rather than structural:

1. `thinking` is a `select` rather than a `boolean`. The spec forbids a boolean
   unless the client advertises support and the pinned SDK cannot read that
   capability, so the `select` fallback the spec names
   (`session-config-options.md:209`) is used instead. Functionally identical.
2. `reject_always` and `allow_always` are remembered only for the call they were
   granted for (`permissionBridge.permitted` is keyed by session+toolCall). An
   honest limitation of the bridge, not an advertised capability that does
   nothing.
3. `additionalDirectories` is no longer advertised at all, so there is no
   capability promising a scope Crush does not enforce. Enforcement itself
   remains open (OQ-1).

---

### 8.1 Current Streaming Behavior (post-Zed test, 2026-09-17)

**Superseded.** The emission gaps this section listed are closed: `agent_thought_chunk`
(S3), `tool_call`/`tool_call_update` (S1/S10), `plan` (S4) and `user_message_chunk`
on replay (R1/R6) all ship. The finding that still holds is the *shape* of
`agent_message_chunk` delivery, and its root cause is diagnosed in §9:

- The message broker (`internal/pubsub`) delivers **debounced full-turn
  snapshots**, not per-token deltas. `OnTextDelta` fires only on full snapshot
  changes, not continuously during generation.
- `streamText`/`streamThoughts` therefore receive bulk data and re-chunk it with
  `splitDelta` plus a 50ms `chunkDeliveryDelay`, which approximates streaming
  cadence rather than reporting it.
- Observed in `crush_debug.jsonl`: one burst carrying all 88 words, then spaced
  chunks — a long initial wait followed by progressive output.

The real fix is upstream, in `internal/pubsub` coalescing, not in the ACP bridge;
see §9 for the verification that neither fantasy nor the OpenAI SDK is the cause.

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
