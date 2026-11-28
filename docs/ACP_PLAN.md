# ACP (Agent Client Protocol) Implementation Plan

Crush implements the Agent Client Protocol as an **ACP Server** (Zed,
JetBrains, Neovim drive it) and, secondarily, as an **ACP Client**
(`crush acp connect` drives external agents). Spec:
https://agentclientprotocol.com/protocol/v1/ — the docs are vendored under
`docs/acp/` by `scripts/fetch_acp_docs/main.go`; the normative v1 prose is
now the 23 files in `docs/acp/protocol/v1/` (the rest of the tree is v2 drafts, RFDs,
announcements and community pages, which are informational only).

Status legend: ✅ done · ⚠️ partial/stub · ❌ not implemented.
"MUST/SHOULD/MAY" follow the v1 spec wording. Item IDs (R/S/O/G/OQ) are
stable: tests, audits and commits cite them.

## 1. Scope and library

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

Runtime SDK: `github.com/coder/acp-go-sdk` v0.20.0 (Coder; ACP schema
1.32.0, stdio + streaming helpers). `Tangerg/acp` v0.2.4 is a test-only
fixture (Zed 1.17.2 transcript + 154 cross-SDK fixtures).

Upstream shipped v0.14.0 through v0.20.0 on 2026-11-18, walking the ACP
schema from 0.13.5 to 1.32.0. The bump unblocks every item this plan used to
call SDK-gated (§5.1) and forces a mechanical type migration (§5.3). The
plan's conformance status below describes v0.13.5 code; §5.4 is the work
that brings it to v0.20.0.

## 2. Protocol-level rules

- All file paths in the protocol **MUST** be absolute; line numbers are
  **1-based** (`overview.md:160`).
- JSON-RPC messages **MUST** be UTF-8, newline-delimited, with no embedded
  newlines. The agent **MAY** write UTF-8 to stderr for logging.
- Stdio is a **MUST** baseline for every Agent; HTTP/SSE and custom
  transports are **MAY** and **MUST** preserve the JSON-RPC format.
- The session `cwd` **MUST** be absolute, **MUST** be used for the session
  regardless of spawn location, and **MUST** be the base for relative-path
  resolution. The effective root set `[cwd, ...additionalDirectories]`
  **SHOULD** bound tool file-system operations.
- Capabilities omitted in a peer's `initialize` are **UNSUPPORTED**
  (`initialization.md:108`); Crush reads what the client sent and gates on it.

## 3. Architecture

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

Supporting pieces: `modes.go` (ACP-only mode enum), `mcp.go` (per-session MCP
connect), `commands.go` (slash-command resolver), `event_bridge.go`
(events → updates, absolute paths, plan/usage), `permission.go` (options,
outcome mapping).

## 4. Conformance matrix

One row per spec requirement. "Where" names the symbol, not a line number,
so the reference survives edits.

### 4.1 Initialization (`initialize`, `authenticate`, `logout`)

| Requirement | Level | Status |
|---|---|---|
| Respond with chosen `protocolVersion` + `agentCapabilities` | MUST | ✅ `Initialize` |
| Provide `agentInfo` (name/title/version) | SHOULD | ✅ `Title:"Crush"` |
| `authMethods` present in response | MUST (field) | ✅ empty slice → `[]` |
| Omitted client capabilities treated as unsupported | MUST | ✅ fs/notice/compaction/terminal features gated on the matching client cap; elicitation is client-track only |
| `loadSession` capability | OPTIONAL | ✅ `true` |
| `promptCapabilities` (image/audio/embeddedContext) | OPTIONAL (text+resource_link MUST regardless) | ✅ all `true` |
| `mcpCapabilities` (http/sse) | OPTIONAL | ✅ both advertised and served (`acpMCPServer` maps stdio/http/sse → `config.MCPConfig`) |
| `sessionCapabilities` (close/list/resume/delete) | OPTIONAL | ✅ four advertised |
| `auth.logout` capability | OPTIONAL | ✅ advertised; `Logout` success no-op |
| `authenticate` method | only reachable if advertised; unadvertised SHOULD answer `-32601` (`error.md:46`) | ✅ `authMethods:[]` so nothing owed; stray call → `-32601` (`dispatcher.Authenticate`) |
| `systemPrompt` support | OPTIONAL | ✅ server advertises append-only; validates text-only content and stores it per session; client CLI sends append via flag/env |
| `additionalDirectories` support | OPTIONAL | ✅ advertised (`SessionCapabilities.AdditionalDirectories`); absolute paths validated on new/load/resume and merged into the tool root set |

### 4.2 Session lifecycle

| Method | Level | Status |
|---|---|---|
| `session/new` | MUST | ✅ creates session, returns `modes`+`configOptions`; cwd must be absolute, validated; `mcpServers` connected |
| `session/prompt` | MUST | ✅ runs coordinator, returns a spec `StopReason`; Fantasy errors/tool-call finishes map to `end_turn`. v1 has no `error` stop reason, and Crush has no request-count limiter to produce `max_turn_requests` |
| `session/cancel` (notification) | MUST | ✅ cancels coordinator |
| `session/update` baseline types (text/thought/tool) | MUST | ✅ see 4.3 |
| `session/close` | OPTIONAL | ✅ cancels work, frees resources (MCP teardown), idempotent empty result |
| `session/list` | OPTIONAL | ✅ `cwd`/`title`/`updatedAt`; pagination (O19); request `_meta` echoed on response; per-session `SessionInfo._meta` unpopulated |
| `session/load` | OPTIONAL | ✅ full replay (R7); returns modes/configOptions (O26/G7); `-32002` unknown |
| `session/resume` | OPTIONAL | ✅ Option B (O5): state, no replay; cwd validated (O22/G4) |
| `session/delete` | OPTIONAL | ✅ idempotent (O21/G2); active-session delete implementation-defined |
| `session/set_mode` | OPTIONAL (legacy) | ✅ real switch via `applyMode` (OQ-3); `-32602` unknown; `coder` alias |
| `session/set_config_option` | OPTIONAL | ✅ complete list returned (MUST); `currentValue` truthful; `-32602` unknown id/value |

Error-code duties (`error.md`): `-32600` before initialize ✅
(`requireInitialized`); `-32002`+`data.sessionId` on unknown session ✅
(`requireSession`, guards prompt/load/resume/set_mode/set_config_option; a
session the store knows but this connection never created, loaded or resumed
is also -32002 — the active set, error.md:56); `-32602` bad cursor / unknown
configId / unknown mode ✅; `-32601` unadvertised method ✅; only spec codes
used, none outside reserved ranges ✅.

### 4.3 Session updates (`session/update`)

20 variants exist; Crush emits 11 of the stable ones.

| Update | Level | Status |
|---|---|---|
| `user_message_chunk` | MUST on load replay | ✅ replay emits it (`UpdateUserMessageText`); live-turn echo skipped by decision (§6 S2) |
| `agent_message_chunk` | baseline | ✅ `subscribeMessages` |
| `agent_thought_chunk` | SHOULD | ✅ `streamThoughts` (S3) |
| `tool_call` | SHOULD | ✅ start with kind/title/locations/rawInput (S1/S10) |
| `tool_call_update` | SHOULD | ✅ pending→in_progress→completed/failed lifecycle (S10); live and replayed results carry `rawOutput` |
| `plan` | SHOULD | ✅ full-replace from `todos` results (S4) |
| `available_commands_update` | MAY | ✅ advertised (skills/custom/per-session MCP prompts, `input.hint`) + executable (O24) |
| `current_mode_update` | MAY | ✅ on mode change (S9) |
| `config_option_update` | MAY | ✅ on config change, complete state (S9) |
| `session_info_update` | MAY | ✅ on `session/new` and whenever the stored title changes (the auto-generated title is announced) |
| `usage_update` | MAY | ✅ at run completion; `used`/`size` non-null; cost has `amount`+`currency` |
| `notice` | MAY, gated on `clientCapabilities.session.notices` | ✅ gated live notices for agent errors, auth-required and cancellation; no replay; fallback agent message when gate absent |
| `compaction_update` / `compaction_summary_chunk` | MAY, gated on `session.compaction`; replayed by load | ✅ gated completed compaction update for stored summaries, replayed on load; no summary chunk stream because Crush stores a complete summary |
| `messageId` on chunks | MAY | ✅ stable per message (`withMessageID`); turn/load replay share the history id |

### 4.4 Content blocks

| Block | Level | Status |
|---|---|---|
| `text` | MUST | ✅ `buildPrompt` |
| `resource_link` | MUST | ✅ rendered inline |
| `image` | OPTIONAL | ✅ both directions (O7 input, O15 output) |
| `audio` | OPTIONAL | ✅ both directions (O7, O15) |
| `resource` (embedded) | OPTIONAL | ✅ text inlined; blob → media (MIME sniffed) or described placeholder (G8/O26) |
| `annotations` | OPTIONAL | ❌ ignored (O16) |

### 4.5 Tool calls and permissions

| Requirement | Level | Status |
|---|---|---|
| Emit `tool_call` + `tool_call_update` during execution | SHOULD | ✅ S1/S10 |
| Tool kind taxonomy (10 kinds) | OPTIONAL | ✅ `toolKindFor` incl. `switch_mode` |
| Tool content `content`/`diff`/`terminal` | SHOULD | ⚠️ `content` always; `diff` for edit/multiedit; client renders `terminal` refs, server-side terminal delegation deferred |
| Fields: `title` required; `name` SHOULD; `kind`/`status`/`locations`/`rawInput`/`rawOutput` optional | SHOULD (`name`) | ✅ `WithStartName`/`WithUpdateName` on live start, live update and load replay |
| All paths absolute (`tool-calls.md:307,353`) | MUST | ✅ `absPath`/`absPather` at `toolLocations`, `diffContentFor`, load replay |
| `session/request_permission` with 4 option kinds | MUST when needed | ✅ `buildPermissionOptions` (O20/G1) |
| Unrecognized outcome ≠ approval | SHOULD | ✅ `mapPermissionOutcome` allow-list (O20) |
| Remembered decisions owned by the Agent, applied by not asking again | SHOULD | ✅ always-kinds are remembered per session+tool and applied without re-asking, and the option names state the scope. Coarser than the spec's examples: the scope is the tool name, not a command or directory |
| Client answers pending permission `cancelled` on `session/cancel` | MUST (peer duty) | ⚠️ agent side correct: denial without error, no spurious `failed` |

### 4.6 Elicitation

OPTIONAL. ✅ Client track: `crush acp connect` advertises form and URL modes
and implements interactive CLI elicitation: review/modify flat form values,
decline/cancel controls, full URL display and explicit consent before opening,
outstanding-ID tracking for `elicitation/complete`, and no URL→form downgrade.
Server track has no elicitation surface; no Agent capability is advertised.
Spec duties that remain relevant: `mode` discriminator required (form/url, no
implicit default); form schemas MUST NOT request secrets; URL mode needs a
unique `elicitationId`, secure context, and user-identity verification;
outcomes are `accept`/`decline`/`cancel`; an unadvertised mode yields `-32602`.

### 4.7 File system

OPTIONAL, gated on `clientCapabilities.fs`. ✅ Client track: real
`os.ReadFile`/`os.WriteFile` with `line`/`limit`, mkdir-parent, create-if-missing
(spec MUST), `-32002`+`data.path` for a missing file (G10). Server track:
`fsReadTextFile`/`fsWriteTextFile` delegate when the client advertises fs,
else local FS (O12). ✅ The ACP server installs a `tools.ClientFS` adapter so
`view` reads and `write` writes through the connected client when it advertises
fs; other tools stay on the local FS. The client is responsible for its own
root-set checks (`file-system.md:127`).

### 4.8 Terminals

OPTIONAL, gated on `clientCapabilities.terminal`. ✅ Client track: all five
`terminal/*` methods are served from an in-memory `os/exec` registry with no
PTY and no new dependency; cwd must be absolute and inside the client root;
`create`/`output`/`wait`/`kill`/`release` are supported, unknown terminals
return `-32002`, output is truncated at rune boundaries, and released IDs are
invalid. The server still does not delegate `bash` through ACP terminals, so
terminal tool content is render-only on the client track. Spec duties noted for
any future work:
`create` params (`command`, `args`, `env`, absolute `cwd`, `outputByteLimit`
with character-boundary truncation), `output` fields (`output`, `truncated`,
`exitStatus` only when exited), release duty on the Agent, embedded
`terminal` content blocks in tool calls.

### 4.9 Cancellation

| Requirement | Level | Status |
|---|---|---|
| `session/cancel` notification cancels the turn | MUST | ✅ `Cancel` → coordinator |
| `$/cancel_request` answered with a valid response or `-32800` | MUST | ✅ SDK cancels the request ctx; `Prompt` maps it to `StopReasonCancelled`; `-32800` never needed |
| Aborts surface as `cancelled`, never an error | MUST | ✅ same mapping |
| Pending permission answered `cancelled` | MUST (peer) | ✅ client CLI answers `cancelled` on `session/cancel`; agent side correct |

### 4.10 MCP servers

| Requirement | Level | Status |
|---|---|---|
| stdio transport | MUST baseline | ✅ `connectSessionMCPServers` on new/load/resume |
| Connect all servers the client specifies | SHOULD | ✅; unsupported transports logged and skipped |
| Launch `command`+`args` without a shell | SHOULD | ✅ `exec.CommandContext` |
| Never echo `env`/`headers` into updates, errors, `_meta`, list, replay, model context, command lines | SHOULD | ✅ `config.MCPConfig` carries `env`, never serialized |
| Send `headers` only to the configured origin | SHOULD | ✅ headers are handed to the MCP client, which sends them to that server's URL only |
| HTTP/SSE transports | OPTIONAL, only if advertised | ✅ `mcpCapabilities{Http,Sse}` advertised and mapped to `config.MCPConfig` http/sse |

Collision safety: registry keys are namespaced `acp:<sessionID>:<name>`
(`mcp.ACPServerName`); `mcp.TeardownSession` closes a session's servers on
close/delete and replaces them on resume; `reconcile` skips `acp:` names so a
config reload cannot tear down a live session; `filterOtherSessionsMCPTools`
drops other sessions' tools from the palette. `context.WithoutCancel` lets
the connection outlive the setup request.

Limitations (recorded, not hidden): tool names embed the session id; ACP MCP
prompts are filtered per session, but MCP resource completions and the `/mcp`
UI read the config-file registry and do not surface ACP-injected servers;
`env` is additive to the inherited process env; OAuth,
`EnabledTools`/`DisabledTools` and channels have no ACP equivalent and are
not applied.

### 4.11 Extensibility (`_meta`)

`_meta` echoed on `session/prompt` responses, on every `session/update` of
that turn, on load replays, and on `session/list` responses ✅ (S7). A W3C
`traceparent` is generated per turn when the client did not supply one
(`withTraceMeta`), so a turn and its updates share a correlatable id;
`tracestate`/`baggage` stay untouched. `session/list` echoes only the request's
`_meta`; it does not synthesize per-session metadata. No custom root-level
fields are added; custom methods would be `_`-prefixed.

### 4.12 Transport

UTF-8 MUST, newline framing MUST, stdout carries only valid ACP messages
MUST, stderr logging MAY ✅ (SDK stdio loop). HTTP/SSE/custom transports are
MAY and unimplemented.

### 4.13 Security (`security.md`)

Trust model: the Agent runs with its process permissions; every
Agent-authored string/URI/path is untrusted, including replayed content;
`clientInfo`/`agentInfo` are self-reported and must not drive security
decisions.

| Agent duty | Spec | Status |
|---|---|---|
| Treat embedded resources, files, tool results, MCP results as data, not instructions | `security.md:50` | ⚠️ ACP prompt blocks carry explicit provenance delimiters; file/tool/MCP results still rely on prompt structure only |
| Permission before sending data off-machine after reading untrusted content | `security.md:51` | ❌ not distinguished from any other network tool |
| Permission before writing files that control execution or approval (VCS hooks, editor/agent settings, MCP config) | `security.md:52` | ✅ ACP permission titles label execution-control paths; behavior stays the normal write prompt |
| Apply the root set to URIs in prompts | `security.md:41` | ⚠️ root set now exists for path-touching tools; URI resolution in prompts still not applied |
| Scope `session/list`, load, resume, permissions, elicitation to the owning user | `security.md:60` | n/a — one user per process |

Client duties (peer side, recorded so they are not mistaken for our gaps):
the CLI client reveals/removes bidi/zero-width/tag characters and neutralizes
terminal control sequences before printing (`security.md:28-29`); because it
has no rich renderer, Markdown/HTML sanitization, no auto-fetch of Agent URIs,
active SVG blocking, and non-http(s) link confirmation remain peer/renderer
duties (`security.md:20-41`).

### 4.14 Capability advertising summary

Advertised: `loadSession:true`, `promptCapabilities{image,audio,
embeddedContext:true}`, `sessionCapabilities{close,delete,list,resume,
additionalDirectories}`, `mcpCapabilities{http,sse}`, `auth.logout`,
`authMethods:[]`. No advertised capability is behaviourally empty:
`additionalDirectories` is applied to the root set and reported back, and the
HTTP/SSE MCP transports map onto Crush's MCP client.

Not advertised: `systemPrompt.mode.override`, server elicitation (Agent never
initiates it), terminal auth, and client `subagents`/`plan`. Advertised:
`sessionCapabilities.systemPrompt` append-only, plus client fs, boolean config
options, elicitation form/url, notices, compaction and terminal support.

### 4.15 v1 spec coverage index

All 23 normative v1 files are accounted for here.

| Spec file | Plan coverage |
|---|---|
| `agent-plan.md` | §4.3 `plan`; §5.5 identified plans |
| `authentication.md` | §4.1 `authMethods`/`logout`, `authenticate`; §5.2 AUTH-1 |
| `cancellation.md` | §4.9 |
| `content.md` | §4.4 |
| `elicitation.md` | §4.6, OQ-4 |
| `error.md` | §4.2 error codes |
| `extensibility.md` | §4.11 |
| `file-system.md` | §4.7 |
| `initialization.md` | §4.1, §4.14 |
| `overview.md` | §2, §4.12 |
| `prompt-turn.md` | §4.2, §4.3, §4.5, §4.9 |
| `schema.md` | §1 SDK types plus every matrix section |
| `security.md` | §4.13 |
| `session-config-options.md` | §4.2, §4.3, M-SDK-2 |
| `session-delete.md` | §4.2 |
| `session-list.md` | §4.2, §4.3 |
| `session-modes.md` | §4.2, §4.3, OQ-3 |
| `session-setup.md` | §4.2, §4.10 |
| `slash-commands.md` | §4.3, O24/OQ-2 |
| `subagents.md` | §5.5 |
| `terminals.md` | §4.8 |
| `tool-calls.md` | §4.5 |
| `transports.md` | §4.12 |

Level wording is kept distinct from implementation posture: the spec says stdio
SHOULD be supported (`transports.md:13`) and permission MAY be requested
(`tool-calls.md:137`), while Crush enforces stdio and asks permission whenever
its safety policy requires it. HTTP MCP is MAY to advertise (`initialization.md:291`)
but SHOULD be supported by new Agents (`session-setup.md:460`); Crush supports
it and advertises it. Plans are MAY generally (`agent-plan.md:11`) and SHOULD
when the model creates one (`agent-plan.md:17`).

## 5. Open items

Grouped by the actual blocker.

### 5.1 SDK-gated — empty after the v0.20 bump

Every item this section used to list is now expressible in the SDK and implemented:

| Was blocked | Unblocked by | New item | Status |
|---|---|---|---|
| tool-call `name` | v0.17 (`ToolCall.Name`, `WithStartName`) | M-SDK-3 | ✅ done |
| `messageId` on chunks | v0.14 (stable `MessageId`) | M-SDK-3 | ✅ done |
| boolean config-option gate | v0.15 (`Session.ConfigOptions.Boolean`) | M-SDK-2 | ✅ done |
| `notice` gate | v0.17 (`Session.Notices`) | M-SDK-4 | ✅ gated live notices |
| `compaction` gate | v0.17 (`Session.Compaction`) | M-SDK-4 | ✅ completed summary update |
| elicitation | v0.16 (stable client methods) | OQ-4' | ✅ client CLI |
| `systemPrompt` | v0.18 (`Session.SystemPrompt`) | OQ-5 | ✅ append on both tracks |

Nothing is SDK-gated any more. O8 terminals were never SDK-gated: the SDK
ships protocol types, not a PTY.

### 5.2 Crush work

- **AUTH-1 `authenticate`** (conditional MUST): register provider auth
  methods (copilot, hyper, openai) as `authMethods` entries with
  `type:"agent"` and trigger OAuth. Nothing is owed while `authMethods:[]`.
  v0.20 adds an unstable `providers/*` surface
  (`AgentUnstableListProviders`/`SetProvider`/`DisableProvider`) that could
  drive this from the client, but it is outside v1 and not required.
- **O8 terminals** (OPTIONAL): no PTY dependency; could be backed by
  `internal/shell` instead — re-evaluate if `crush acp connect` grows a
  terminal pane.

### 5.3 SDK v0.20.0 migration (M-SDK) — landed 11-20

The bump was mechanical but wide: most breaks were compile errors in
`internal/acp`, a few changed wire behaviour silently and needed a check.

**M-SDK-1 — migrate to v0.20.0 types. ✅** No behaviour change.

| Break | Sites | Fix |
|---|---|---|
| `acp.Agent` requires `DeleteSession` | `agent.go` `UnstableDeleteSession`, `server.go` interface + dispatcher | rename; `Unstable*` types removed |
| `acp.Client` requires `CreateElicitation`/`CompleteElicitation` | `client.go` `crushClient` | CLI handler; `-32601` only when no handler is installed |
| `_meta` is `acp.Meta` (`map[string]json.RawMessage`) | `withTraceMeta`, `event_bridge` prompt-meta map, `LoadSession` `send` | `acp.NewMeta`, `Meta.Set`/`Get` |
| optional update fields are `acp.Patch[T]` | `permission.go` title, `sessionInfoUpdate`, `handleSessionEvent` | `acp.PatchSet`/`PatchFromPtr` |
| `rawInput`/`rawOutput` are `json.RawMessage` | `toolRawInput`, `parseToolCall` | pass raw JSON; drop the marshal round-trip |
| `*int` → `*int64` | `fsReadTextFile`, client `ReadTextFile` windowing | clamp in `int64` before slicing |
| `RequestError` literals | `sessionNotFound`, `LoadSession`, client fs read | `acp.NewRequestError` |
| `AgentExperimental`/`ClientExperimental` gone | none asserted today | one optional interface per method |

**Silent changes to verify:** `WithUpdateContent`/`WithUpdateLocations` with
an empty non-nil slice now sends `[]` (clears) — all call sites guard on
`len()>0`; `WithStartLocations` may inject `rawInput.path`; `WithStartRawInput`
no longer fails the send on an unencodable value; `RequestError` is no longer
comparable.

**M-SDK-2 — real boolean config option. ✅** `thinking` is
`type:"boolean"` when `clientCaps.Session.ConfigOptions.Boolean` is set, and
stays a `select` otherwise.

**M-SDK-3 — tool `name` + `messageId`. ✅** `WithStartName`/`WithUpdateName`
emitted on live start, live update and load replay; chunks carry a stable
`messageId` (`withMessageID`). Closes the last open SHOULD.

**M-SDK-4 — notice + compaction emission. ✅** Both are gated on the matching
`clientCaps.Session` gate, per the MUST-NOT rules. Notices cover agent errors,
re-auth requests and run cancellation; permission denial remains a failed tool
status. Compaction uses the stored summary message as a single completed
`compaction_update`, emitted live and replayed by load with the same ID.

**B2–B4 also landed:** ACP MCP `http`/`sse` advertised and served, and
`additionalDirectories` advertised, validated (absolute, on new/load/resume),
stored per session, reported in `session/list` and merged into the tool root
set via `tools.RootsContextKey`.

**Cleanup: ✅** `internal/acp/protocol/jsonrpc.go` deleted; superseded by the
SDK transport.

### 5.4 Deliberate decisions (not gaps)

- `additionalDirectories` is enforced, not retracted (supersedes O23):
  absolute-only on new/load/resume, reported in `session/list`, and merged
  into the tool root set so path tools see it.
- `thinking` is a real `boolean` when the client advertises
  `session.configOptions.boolean`, and a `select` with on/off otherwise — the
  spec's own fallback (`session-config-options.md:209`).
- No live-turn `user_message_chunk` echo (decision S2, §6).
- Shell builtins (`provider`, `model`, `mcp`, …) are not advertised: they
  mutate crushrc config rather than produce a prompt. Two designs were
  weighed — parse `/<builtin>` from prompt text and dispatch via `shell.Run`
  with the `ConfigBuilder` on context (spec-idiomatic), or wrap each as a
  `tool_call` with `kind:"other"` (faster, loses slash-command UX).
- YOLO as a config option, if ever added, would be a boolean once the client
  advertises `session.configOptions.boolean`, and a select otherwise
  (M-SDK-2).
- Mode switch is workspace-wide: two concurrent ACP sessions share one agent;
  `currentValue` stays truthful because it reads live state back. `plan`+
  `yolo` has no ACP id; selecting `plan` resets permissions to Normal, which
  raises oversight — the safe direction.

### 5.5 Unimplemented optional protocol families

These are optional v1 features. They are listed so the matrix does not imply
coverage that does not exist. None is owed while its client gate is absent.
They are also out of scope for this pass; §5.6 records the reasons.

- **Subagents** (`subagents.md`, OPTIONAL): no `subagents` gate read, no
  `subagent_update`, `session_message`, `session_message_chunk`, child
  work-state, child cancellation, or replay handling. SDK v0.20 still marks
  these update variants UNSTABLE.
- **Identified plans** (`agent-plan.md`, SHOULD when a plan exists): only the
  stable `plan` checklist is emitted. `plan_update`/`plan_removed` require the
  client `plan` gate and are not implemented.
- **Terminal auth** (`authentication.md`, MAY): no `auth.terminal` gate read
  and no terminal auth method advertised.
- **Elicitation** client track, **notices/compaction** (M-SDK-4) and
  **terminals** (O8) are tracked in their matrix rows.

### 5.6 Out of scope without new Crush or SDK work

Scope decision 11-25: implement only optional ACP features backed by existing
Crush surfaces and installed dependencies. Items below are **not in scope**,
not near-term TODOs. Each needs a new subsystem, a breaking prompt/model
change, SDK-unstable types, a transport rewrite, or work that belongs to the
ACP peer rather than this adapter.

| Feature | Why not in scope |
|---|---|
| `error` prompt stop reason | Not one of the five v1 `StopReason` values; emitting it would be a non-spec wire value |
| `max_turn_requests` prompt stop reason | SDK v0.20 has the constant, but Crush has no max-request limiter or result signal; the mapping alone would be fake |
| `notice` replay | Spec defines notices as live events; clients MUST NOT rely on replay |
| `compaction_summary_chunk` stream | Crush stores a complete summary; a chunk stream would have to invent boundaries |
| Command/directory-scoped remembered permissions | Remembered decisions remain per session+tool; execution-control-write classification does not add command or directory scopes |
| `annotations` surfacing | SDK has the type, but Crush messages and model prompts carry no audience/priority/lastModified field to surface it into |
| `systemPrompt.mode.override` | Not advertised in the SDK capability model; override would drop tool guidance and context files, so it needs prompt-assembly redesign |
| AUTH-1 `authenticate` and real `logout` effect | Existing `crush login` is CLI/TUI workspace setup; ACP needs advertising, provider selection, OAuth execution, token persistence, and config reload |
| Terminal auth (`auth.terminal`) | Requires client launch args/env policy, safety review of env changes, an interactive terminal, and reconnect after success |
| Server-side `bash` delegation through `terminal/*` | Client terminal methods exist, but routing the server's shell tool through them needs a session-scoped terminal bridge and lifecycle/cancel/release handling |
| Client terminal PTY | Deliberate no-dependency choice; adding PTY support needs a new terminal dependency/subsystem |
| Full elicitation security: isolated browser context and verified user identity | URL mode shows the full URL and asks consent, but Crush has no authenticated client identity or isolated browser context to verify |
| Form-elicitation secret refusal | The CLI cannot reliably infer credentials from arbitrary schema fields; URL mode is the spec's safe path for secrets |
| Subagents: `subagent_update`, `session_message`, `session_message_chunk`, child state/cancel/replay | Optional family, SDK v0.20 marks variants UNSTABLE, and Crush has no child-session update plumbing |
| Identified plans: `plan_update`, `plan_removed` | Requires the client `plan` gate, SDK-unstable update variants, and identified plan IDs that Crush todos do not have |
| General ACP HTTP/SSE/custom transports | Crush enforces stdio; SDK stdio loops are in use. MCP HTTP/SSE is a separate, already-supported feature |
| Unsupported MCP transports | Crush advertises and maps stdio/http/sse only; other ACP MCP transports are outside the advertised set |
| ACP MCP tool names embedding session id | Needed because Crush's MCP registries are keyed by name alone; removing it requires a registry key redesign to avoid cross-session collisions |
| `/mcp` UI surfacing and resource completions for ACP-injected servers | The UI and completion path read the config-file registry; only ACP prompt commands are filtered per session |
| MCP OAuth | ACP `McpServer` carries URL/headers only; OAuth needs a credential transport and auth URL flow with no ACP mapping |
| MCP `EnabledTools`/`DisabledTools` | ACP has no such fields; applying them needs per-session/injected-server policy separate from global config |
| MCP channels | Crush channels are config-store state; ACP injection bypasses config-file entries and has no per-session channel state |
| Root/fetch rules for URIs in prompts | `OutsideRoots` covers filesystem paths, not schemes, loopback/private targets, redirects, or response-size bounds |
| Taint-tracked off-machine-send permission after untrusted reads | Crush already prompts broadly for non-safe network operations; path classification and delimiters do not add taint tracking through tools and MCP results |
| Per-user scoping of sessions/permissions/elicitation | Crush is a one-user-per-process agent; scoping requires a multi-user deployment model |
| Full client rendering security: Markdown/HTML sanitization, no auto-fetch of Agent URIs, active SVG blocking, non-http(s) link confirmation | Peer/client renderer duty, not ACP adapter logic; CLI sanitization covers plain-text terminal output only, and the CLI client has no rich renderer to harden here |
| Workspace-local mode switch | `SetMainAgent` is workspace-wide; isolation needs a larger agent-state redesign |
| `plan` + `yolo` combined ACP mode | No ACP id exists; selecting `plan` resets permissions to Normal, which is the safe direction |
| `task` ACP session mode | `task` is a subagent profile; selecting it as the main agent would silently degrade the session |
| Server-side elicitation | ACP Agents do not initiate `elicitation/create`; the role/spec makes this N/A |
| One-user process model | Explicit deployment scope; changing it is outside ACP conformance work |
| Per-token `agent_message_chunk` cadence | PubSub delivers debounced snapshots; real deltas require a broker change, not an ACP bridge change |
| OpenAgents conformance harness in `test/acp/run.sh` | External Node/pnpm repository is not hermetic; keep the manual harness unless a pinned vendored harness is added |
| MCP prompt resolution end-to-end with a live MCP client | No in-repo live MCP fixture; current tests cover ACP prompt filtering and resolution logic, not a live MCP server |

## 6. Decision log

| ID | Date | Decision | Why |
|---|---|---|---|
| SDK | — | `coder/acp-go-sdk` runtime; Tangerg test-only | most complete v1 model, stdio + helpers |
| SDK-2 | 11-20 | bump to `acp-go-sdk` v0.20.0 (schema 1.32.0) | upstream stabilised every feature the plan called SDK-gated; one migration beats four |
| OQ-4 | 11-20 | elicitation: not implemented yet | superseded by OQ-4' on 11-25 |
| OQ-4' | 11-25 | client CLI elicitation with form/url consent | stable SDK + shared stdin dispatcher cover it without a new subsystem |
| OQ-5 | 11-20 | `systemPrompt`: unadvertised for now | superseded by OQ-5' on 11-25 |
| OQ-5' | 11-25 | advertise append only on both tracks | override would require prompt-assembly redesign and is not advertised |
| O2 | 11-17 | `thinking` as `select`, not `boolean` | superseded by SDK-2: gate was unreadable on v0.13.5; v0.20 reads it, so boolean returns and select becomes the fallback |
| O23 | 11-17 | retract `additionalDirectories` (supersedes O3) | an advertised capability with no behaviour behind it is worse than none |
| O23' | 11-20 | re-advertise and enforce `additionalDirectories` (supersedes O23) | B4 gave the field behaviour: absolute-only validation, storage, list reporting and root-set merge |
| O24 | 11-17 | ACP-local command resolver | keeps the TUI untouched; drift risk is OQ-2 |
| S2 | 09-17 | no live `user_message_chunk` echo | duplicates input the client holds; races the streaming path; load replay does emit (MUST) |
| O5 | 09-18 | resume = Option B | return mode/config state, no replay |
| O6 | 09-17 | `logout` success no-op | no auth session to end |
| O7 | 09-22 | prompt blocks → text + attachments; blobs → media or placeholder | image/audio/resources must reach the model, not be dropped |
| O8 | 09-17 | terminals deferred | superseded by O8' on 11-25 for the client track |
| O8' | 11-25 | client terminal methods use `os/exec`; server delegation stays deferred | no PTY dependency; external Agents can use `crush acp connect` terminals |
| O9 | 11-17 | connect `req.McpServers` per session | stdio is a MUST baseline; namespaced keys solve the name-keyed registry collision |
| O11 | 09-17 | render streaming chunks in `crush acp connect` | otherwise the CLI prints only `[stop: ...]` |
| O12 | 11-19 | wire fs into `view`/`write` via `tools.ClientFS` | client-side unsaved state must be visible to the tools |
| OQ-1 | 11-19 | enforce `[cwd]` root set in path tools; bash always prompts | spec SHOULD; permission request is the escape hatch (`security.md:451`) |
| O18 | 11-17 | spec error codes everywhere | `-32600`/`-32601`/`-32602`/`-32002` per `error.md` |
| O20 | 11-17 | all four option kinds; allow-list outcome map | unknown outcome must never approve |
| O21 | 11-17 | delete is idempotent | missing session = success (`session-delete.md:85`) |
| O22 | 11-17 | shared `validateCwd` on new/load/resume | cwd is part of the root set |
| O25/OQ-3 | 11-17 | four-mode enum; `applyMode` calls `SetMainAgent`+`SetPermissionMode` | `currentValue` MUST be the value the Agent operates with |
| AUTH | 09-18 | agent-type methods only | no terminal/side-auth flows advertised |
| cwd | 11-18 | reject relative cwd on setup and the list filter | protocol paths MUST be absolute (`overview.md:160`) |
| G12 | 11-18 | active-session set on the connection | `error.md:56` ties -32002 to activity, not store existence |
| scope | 11-18 | remember always-kinds per session+tool | `tool-calls.md:237-250`: the Agent owns scope; the option name states it |
| OQ-2 | 11-18 | share `commands.SubstituteArgs`; keep positional binding ACP-local | the only true duplication was the substitution rule |
| title | 11-18 | announce title changes via `session_info_update` | `session-list.md:219`: clients otherwise show the static new-session title |
| trace | 11-18 | generate a traceparent when the client sends none | `extensibility.md:40-44` reserves the key; a turn needs a correlatable id |

**OQ-1 — root set (RESOLVED 11-19, extended 11-20).** `tools.RootsContextKey`
carries the root set into every run via `sessionAgent.Run`. `OutsideRoots`
compares the resolved path by component and fails closed; `view`, `write`,
`edit` and `multiedit` ask permission before touching a path outside it.
`bash` always prompts when a root set is present: a shell cannot be confined
by a path check alone. Since B4 the set is `[cwd]` plus each session's
`additionalDirectories`.

**OQ-2 — RESOLVED 11-18: the substitution rule is shared.**
`commands.SubstituteArgs` in `internal/commands` is now the one rule both the
TUI dialog path and the ACP prompt path call. What stays ACP-local is the
part that legitimately differs: the TUI collects arguments through a dialog,
while a prompt binds them positionally from the trailing text (`splitCommand`,
`bindArgs` in `internal/acp/commands.go`). The drift risk OQ-2 recorded is
gone for substitution; argument collection is by design.

**OQ-4' — elicitation (RESOLVED 11-25).** `crush acp connect` now has a shared
stdin dispatcher, so it advertises form/url modes and prompts the user. URL mode
shows the full URL and requires consent before opening; unknown completion IDs
are ignored. The server track still advertises no elicitation because an ACP
Agent never initiates `elicitation/create`.

**OQ-5' — `systemPrompt` (RESOLVED 11-25).** Both tracks use append only.
Crush appends trusted text after built-in/MCP instructions and rejects override,
which would drop tool guidance and context files. The client sends append text
from `--system-prompt` or `CRUSH_ACP_SYSTEM_PROMPT`.

**OQ-3 — mode switching (RESOLVED 11-17).** The TUI's four-state ring is
flattened into an ACP-only enum:

| ACP mode id | Agent | PermissionMode |
|---|---|---|
| `code` | `coder` | Normal |
| `plan` | `plan` | Normal |
| `yolo` | `coder` | Yolo |
| `sysadmin` | `coder` | Sysadmin |

`task` is excluded (subagent profile; selecting it as main agent would
silently degrade the session). `coder` is accepted as a legacy alias for
`code`. `SetMainAgent` is workspace-wide, so with two concurrent ACP
sessions the last switch wins; `currentMode` reads live state back
(`Coordinator.MainAgentName`) so `currentValue` stays truthful.

## 7. Verification history

- **2026-09-17/22** — first full pass: R1–R7 closed (load replay, list
  fields, `authMethods`, cancellation, `$/cancel_request`, baseline updates,
  full replay); S1–S10 addressed (S2 decided not-echoed, S5/S6 SDK-gated);
  O1–O19 closed except O8/O16 (deferred) and O9 (open until the 11-17 batch).
- **2026-11-13** — O9/MCP was found still unimplemented behind a stale
  decision note; re-audited.
- **2026-11-17** — v1 re-audit of all 22 files: twelve gaps (G1–G12) found
  and closed the same day (O20–O26): four permission kinds + allow-list
  outcome map, idempotent delete, cwd validation on load/resume, capability
  retraction, runnable commands, mode/config parity with live `currentValue`,
  load returns setup state, blob decode, fs `-32002`, plus the SDK-correction
  that terminal auth *is* modelled.
- **2026-11-17** — second MUST pass: four unmet Agent MUSTs found and closed
  the same day: stdio MCP connect (`session-setup.md:458,466`), no boolean
  option unadvertised (`session-config-options.md:207`), absolute paths on
  the wire (`overview.md:160`, `tool-calls.md:307,353`), truthful
  `currentValue` (`session-config-options.md:245`).
- **2026-11-18** — this audit: re-read all 22 files against the code. No new
  code gap at MUST/SHOULD level except the cwd-absolute nit (closed below).
  Plan document restructured (one matrix instead of four duplicated status
  tables); stale rows fixed (O9, O25/Q2/Q3 tables, advisory-mode text). Also
  closed: relative cwd now rejected with -32602, the per-connection active
  session set is enforced (G12), and always-kind permission decisions are
  remembered at tool scope. Batch B landed the same day: the substitution
  rule moved to `commands.SubstituteArgs` (OQ-2 resolved), title changes are
  announced as `session_info_update`, and `session/prompt` now generates a
  W3C `traceparent` when the client sends none.
- **2026-11-19** — Batch C: `tools.RootsContextKey` now carries `[cwd]` into
every run, `OutsideRoots` bounds `view`/`write`/`edit`/`multiedit` (outside a
root needs a permission request) and `bash` always prompts when root-scoped
(OQ-1 resolved). ACP installs a `tools.ClientFS` so `view` reads and `write`
writes route through the client when it advertises fs (O12).
- **2026-11-20** — M-SDK landed (B1–B4). B1: `acp-go-sdk` v0.20.0, `_meta` →
  `acp.Meta`, `Patch[T]` fields, `json.RawMessage` raw I/O, `*int64` windows,
  `acp.NewRequestError`, `DeleteSession` rename, elicitation client methods
  answering `-32601`; the hand-rolled `protocol/jsonrpc.go` deleted. B2: tool
  `name` via `WithStartName`/`WithUpdateName` (live + replay) and a stable
  `messageId` per message. B3: `thinking` is a real boolean when the client
  advertises `session.configOptions.boolean`, `select` otherwise. B4: MCP
  `http`/`sse` advertised and served; `additionalDirectories` advertised,
  validated, stored, listed and merged into the tool root set. Conformance
  tests added for both capability groups.
- **2026-11-25** — v1 re-audit after refreshing the vendored docs: 23 v1
  files now exist. Added the §4.15 coverage index and §5.5 optional-family
  list; corrected the level wording that had conflated spec SHOULD/MAY with
  Crush's implementation posture. `session/new` now rejects an empty cwd with
  `-32602`, load replay emits `rawOutput` alongside tool result content, and
  the elicitation client callback actually returns `-32601` as recorded.
- **2026-11-25 (feature batch 1)** — `crush acp connect` gained a shared stdin
  reader, `--system-prompt`/`CRUSH_ACP_SYSTEM_PROMPT` append input, and the
  capability set for elicitation, notices, compaction and terminals. Agent
  message text stays on stdout; other updates render on stderr.
- **2026-11-25 (feature batches 2–3)** — `crush acp connect` now serves all
  five terminal methods from an `os/exec` registry, with root checks,
  rune-boundary truncation and release invalidation. Elicitation uses a shared
  stdin dispatcher for interactive form values and URL consent, and ignores
  unknown completion IDs.
- **2026-11-25 (feature batch 4)** — server advertises append-only
  `sessionCapabilities.systemPrompt`, accepts text-only append instructions on
  session setup, stores them per session, and injects them after built-in and
  MCP instructions. Override stays unadvertised and is rejected.
- **2026-11-25 (feature batches 5–6)** — server emits gated notices for agent
  errors, re-auth requests and cancellations, and emits/replays a single
  completed compaction update from stored summary messages.
- **2026-11-25 (minimal candidates)** — `session/list` echoes request `_meta`;
  the client CLI uses an interactive permission handler and answers pending
  permissions with `cancelled` on cancel; CLI output neutralizes terminal
  control and bidi/zero-width/tag characters; execution-control writes get a
  path label in ACP permission titles; ACP prompt content gets provenance
  delimiters; MCP prompt commands filter per session; blob attachments are
  verified through `Coordinator.Run`.

**Conformance position (current):** client-side terminals, elicitation,
systemPrompt, notice rendering and compaction rendering are implemented; server
append-only systemPrompt, notices and compaction are implemented. `session/list`
request `_meta` echo, client-side permission cancellation, CLI plain-text
sanitization, execution-control-write classification, ACP prompt provenance
delimiters, per-session MCP prompt command filtering and blob-to-coordinator
coverage are implemented. `thinking` is a real boolean where the client
advertises it, and the select is the documented fallback. The root set is
`[cwd]` plus each session's `additionalDirectories`, both enforced.
SDK-UNSTABLE subagents and identified plans remain skipped; AUTH-1 and terminal
auth remain deferred.

## 8. Tests

```bash
go test ./internal/acp/... -count=1            # unit + integration
go test ./test/acp/spec/... -count=1           # conformance, golden, Tangerg interop
CRUSH_BIN=$(go build -o /tmp/crush .) \
  go test -tags e2e ./test/acp/ -count=1       # subprocess e2e
./test/acp/run.sh all                          # full runner
```

The OpenAgents conformance harness is documented in `test/acp/README.md`; it
stays manual because Node/pnpm setup and the external repository are not
hermetic test-runner dependencies.

Coverage highlights: permission options + outcome mapping + remembered
scope; load replay; delete idempotency; cwd validation and absoluteness;
active-session gating; cursor handling; blob resources; fs `-32002`; mode
selection and current-mode truthfulness; command resolution; absolute-path
helpers; initialize-before-request and unknown session codes; golden JSON
fixtures (33) and the Tangerg/Zed interop fixtures. Added 11-20: tool `name`
and `messageId` presence on live and replayed updates, boolean vs select config
options behind the client gate, MCP `http`/`sse` advertising, and an
`additionalDirectories` round trip (apply → list → relative reject). Added
11-25: elicitation client `-32601`, empty-cwd rejection, replay `rawOutput`,
the complete Fantasy-to-ACP stop-reason table, `session/list` request `_meta`
echo, client permission cancellation, CLI plain-text sanitization,
execution-control-write and prompt-provenance handling, per-session MCP prompt
filtering, and blob attachments through `Coordinator.Run`.

Still uncovered: MCP prompt resolution end-to-end with a live MCP client.

## 9. Appendix — streaming investigations (historical)

**9.1 Emission gaps (superseded).** The 2026-09-17 gaps this section once
listed (thoughts, tool calls, plan, replay user chunks) are closed. The
finding that still holds: `agent_message_chunk` arrives in bursts because
`internal/pubsub` delivers debounced full-turn snapshots, not per-token
deltas; `splitDelta` + a 50 ms delay approximate cadence. The fix belongs
upstream in the broker, not in the ACP bridge.

**9.2 Fantasy transport finding.** `fantasy` streams per-delta over SSE
(`text/event-stream`) on OpenAI-compatible endpoints; no NDJSON anywhere.
So the burstiness in 9.1 is provably Crush-side, not provider-side.
