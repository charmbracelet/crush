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
| Omitted client capabilities treated as unsupported | MUST | ✅ fs gated on `clientCaps.Fs`; no elicitation/terminal/notice/compaction use |
| `loadSession` capability | OPTIONAL | ✅ `true` |
| `promptCapabilities` (image/audio/embeddedContext) | OPTIONAL (text+resource_link MUST regardless) | ✅ all `true` |
| `mcpCapabilities` (http/sse) | OPTIONAL | ✅ both advertised and served (`acpMCPServer` maps stdio/http/sse → `config.MCPConfig`) |
| `sessionCapabilities` (close/list/resume/delete) | OPTIONAL | ✅ four advertised |
| `auth.logout` capability | OPTIONAL | ✅ advertised; `Logout` success no-op |
| `authenticate` method | only reachable if advertised; unadvertised SHOULD answer `-32601` (`error.md:46`) | ✅ `authMethods:[]` so nothing owed; stray call → `-32601` (`dispatcher.Authenticate`) |
| `systemPrompt` support | OPTIONAL | ✅ unadvertised; v0.20 models it, so this is OQ-5 (decision), not an SDK limit |
| `additionalDirectories` support | OPTIONAL | ✅ advertised (`SessionCapabilities.AdditionalDirectories`); absolute paths validated on new/load/resume and merged into the tool root set |

### 4.2 Session lifecycle

| Method | Level | Status |
|---|---|---|
| `session/new` | MUST | ✅ creates session, returns `modes`+`configOptions`; cwd must be absolute, validated; `mcpServers` connected |
| `session/prompt` | MUST | ✅ runs coordinator, returns a spec `StopReason`; Fantasy errors/tool-call finishes map to `end_turn` because SDK v0.20 has no `error`/`max_turn_requests` prompt stop reason |
| `session/cancel` (notification) | MUST | ✅ cancels coordinator |
| `session/update` baseline types (text/thought/tool) | MUST | ✅ see 4.3 |
| `session/close` | OPTIONAL | ✅ cancels work, frees resources (MCP teardown), idempotent empty result |
| `session/list` | OPTIONAL | ✅ `cwd`/`title`/`updatedAt`; pagination (O19); `_meta` unpopulated |
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
| `available_commands_update` | MAY | ✅ advertised (skills/custom/MCP prompts, `input.hint`) + executable (O24) |
| `current_mode_update` | MAY | ✅ on mode change (S9) |
| `config_option_update` | MAY | ✅ on config change, complete state (S9) |
| `session_info_update` | MAY | ✅ on `session/new` and whenever the stored title changes (the auto-generated title is announced) |
| `usage_update` | MAY | ✅ at run completion; `used`/`size` non-null; cost has `amount`+`currency` |
| `notice` | MAY, gated on `clientCapabilities.session.notices` | ⚠️ gate and update are modelled in SDK v0.20 (`ClientSessionCapabilities.Notices`, `SessionUpdateNotice`); not emitted yet (M-SDK-4) |
| `compaction_update` / `compaction_summary_chunk` | MAY, gated on `session.compaction`; replayed by load | ⚠️ gate and both updates modelled in SDK v0.20; not emitted or replayable yet (M-SDK-4) |
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
| Tool content `content`/`diff`/`terminal` | SHOULD | ⚠️ `content` always; `diff` for edit/multiedit; `terminal` deferred (O8) |
| Fields: `title` required; `name` SHOULD; `kind`/`status`/`locations`/`rawInput`/`rawOutput` optional | SHOULD (`name`) | ✅ `WithStartName`/`WithUpdateName` on live start, live update and load replay |
| All paths absolute (`tool-calls.md:307,353`) | MUST | ✅ `absPath`/`absPather` at `toolLocations`, `diffContentFor`, load replay |
| `session/request_permission` with 4 option kinds | MUST when needed | ✅ `buildPermissionOptions` (O20/G1) |
| Unrecognized outcome ≠ approval | SHOULD | ✅ `mapPermissionOutcome` allow-list (O20) |
| Remembered decisions owned by the Agent, applied by not asking again | SHOULD | ✅ always-kinds are remembered per session+tool and applied without re-asking, and the option names state the scope. Coarser than the spec's examples: the scope is the tool name, not a command or directory |
| Client answers pending permission `cancelled` on `session/cancel` | MUST (peer duty) | ⚠️ agent side correct: denial without error, no spurious `failed` |

### 4.6 Elicitation

OPTIONAL. ⚠️ Decision pending (OQ-4). The SDK made elicitation stable in
v0.16: `acp.Client` now requires `CreateElicitation`/`CompleteElicitation`,
so the client track must answer them whether or not the feature ships (a
`-32601` and a `nil` keep today's behaviour). Essentials for an
implementation: `mode` discriminator required (form/url, no implicit default);
form uses restricted JSON Schema and MUST NOT request secrets; URL mode needs
a unique `elicitationId` + `elicitation/complete`, no url→form downgrade,
full-URL display and consent before navigation, secure context, and
user-identity verification; outcomes are `accept`/`decline`/`cancel`; an
unadvertised mode yields `-32602`.

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

OPTIONAL, gated on `clientCapabilities.terminal`. ❌ all five methods stubbed
"not supported" (O8); no PTY library. Spec duties noted for any future work:
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
| Pending permission answered `cancelled` | MUST (peer) | ⚠️ peer duty; agent side correct |

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

Limitations (recorded, not hidden): tool names embed the session id; MCP
prompts/resource completions and the `/mcp` UI read the config-file registry
and do not surface ACP-injected servers; `env` is additive to the inherited
process env; OAuth, `EnabledTools`/`DisabledTools` and channels have no ACP
equivalent and are not applied.

### 4.11 Extensibility (`_meta`)

`_meta` echoed on `session/prompt` responses, on every `session/update` of
that turn, and on load replays ✅ (S7). A W3C `traceparent` is generated per
turn when the client did not supply one (`withTraceMeta`), so a turn and its
updates share a correlatable id; `tracestate`/`baggage` stay untouched. No
custom root-level fields are added; custom methods would be `_`-prefixed.

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
| Treat embedded resources, files, tool results, MCP results as data, not instructions | `security.md:50` | ⚠️ prompt structure only; no explicit delimiting |
| Permission before sending data off-machine after reading untrusted content | `security.md:51` | ❌ not distinguished from any other network tool |
| Permission before writing files that control execution or approval (VCS hooks, editor/agent settings, MCP config) | `security.md:52` | ❌ no such classification in the permission layer |
| Apply the root set to URIs in prompts | `security.md:41` | ⚠️ root set now exists for path-touching tools; URI resolution in prompts still not applied |
| Scope `session/list`, load, resume, permissions, elicitation to the owning user | `security.md:60` | n/a — one user per process |

Client duties (peer side, recorded so they are not mistaken for our gaps):
sanitize Agent Markdown/HTML, no auto-fetch of Agent URIs, confirm non-http(s)
links, reveal/remove bidi/zero-width/tag characters, neutralize terminal
control sequences (`security.md:20-41`).

### 4.14 Capability advertising summary

Advertised: `loadSession:true`, `promptCapabilities{image,audio,
embeddedContext:true}`, `sessionCapabilities{close,delete,list,resume,
additionalDirectories}`, `mcpCapabilities{http,sse}`, `auth.logout`,
`authMethods:[]`. No advertised capability is behaviourally empty:
`additionalDirectories` is applied to the root set and reported back, and the
HTTP/SSE MCP transports map onto Crush's MCP client.

Not advertised: `sessionCapabilities.systemPrompt` (OQ-5), elicitation and
terminal auth (Crush has no auth session to terminate). Client
`subagents`/`plan` capabilities are never needed by the unimplemented families
in §5.5.

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

Every item this section used to list is now expressible in the SDK, and all
but the two decision-gated ones are implemented:

| Was blocked | Unblocked by | New item | Status |
|---|---|---|---|
| tool-call `name` | v0.17 (`ToolCall.Name`, `WithStartName`) | M-SDK-3 | ✅ done |
| `messageId` on chunks | v0.14 (stable `MessageId`) | M-SDK-3 | ✅ done |
| boolean config-option gate | v0.15 (`Session.ConfigOptions.Boolean`) | M-SDK-2 | ✅ done |
| `notice` gate | v0.17 (`Session.Notices`) | M-SDK-4 | ⚠️ deferred |
| `compaction` gate | v0.17 (`Session.Compaction`) | M-SDK-4 | ⚠️ deferred |
| elicitation | v0.16 (stable client methods) | OQ-4 | ⚠️ declined |
| `systemPrompt` | v0.18 (`Session.SystemPrompt`) | OQ-5 | ⚠️ deferred |

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
| `acp.Client` requires `CreateElicitation`/`CompleteElicitation` | `client.go` `crushClient` | `-32601` + `nil` until OQ-4 decides |
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

**M-SDK-4 — notice + compaction emission.** ⚠️ Deferred. Only when the
corresponding `clientCaps.Session` gate is set, per the MUST-NOT rules. Notice
candidates: run cancellation and permission denial. Compaction needs a
Crush-side compaction event to forward; if none exists, the gate stays
unadvertised and emission is correctly omitted. Neither gate is advertised
today, so nothing is owed.

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

- **Subagents** (`subagents.md`, OPTIONAL): no `subagents` gate read, no
  `subagent_update`, `session_message`, `session_message_chunk`, child
  work-state, child cancellation, or replay handling. SDK v0.20 still marks
  these update variants UNSTABLE.
- **Identified plans** (`agent-plan.md`, SHOULD when a plan exists): only the
  stable `plan` checklist is emitted. `plan_update`/`plan_removed` require the
  client `plan` gate and are not implemented.
- **Terminal auth** (`authentication.md`, MAY): no `auth.terminal` gate read
  and no terminal auth method advertised.
- **`systemPrompt`** (OQ-5), **elicitation** (OQ-4), **notices/compaction**
  (M-SDK-4), **terminals** (O8): see the named items.

## 6. Decision log

| ID | Date | Decision | Why |
|---|---|---|---|
| SDK | — | `coder/acp-go-sdk` runtime; Tangerg test-only | most complete v1 model, stdio + helpers |
| SDK-2 | 11-20 | bump to `acp-go-sdk` v0.20.0 (schema 1.32.0) | upstream stabilised every feature the plan called SDK-gated; one migration beats four |
| OQ-4 | 11-20 | elicitation: not implemented yet | stable in the SDK, but Crush has no surface to ask a user; the client track answers `-32601` |
| OQ-5 | 11-20 | `systemPrompt`: unadvertised for now | v0.20 models append/override; implementing `append` is cheap, `override` needs prompt-assembly work |
| O2 | 11-17 | `thinking` as `select`, not `boolean` | superseded by SDK-2: gate was unreadable on v0.13.5; v0.20 reads it, so boolean returns and select becomes the fallback |
| O23 | 11-17 | retract `additionalDirectories` (supersedes O3) | an advertised capability with no behaviour behind it is worse than none |
| O23' | 11-20 | re-advertise and enforce `additionalDirectories` (supersedes O23) | B4 gave the field behaviour: absolute-only validation, storage, list reporting and root-set merge |
| O24 | 11-17 | ACP-local command resolver | keeps the TUI untouched; drift risk is OQ-2 |
| S2 | 09-17 | no live `user_message_chunk` echo | duplicates input the client holds; races the streaming path; load replay does emit (MUST) |
| O5 | 09-18 | resume = Option B | return mode/config state, no replay |
| O6 | 09-17 | `logout` success no-op | no auth session to end |
| O7 | 09-22 | prompt blocks → text + attachments; blobs → media or placeholder | image/audio/resources must reach the model, not be dropped |
| O8 | 09-17 | terminals deferred | no PTY lib; connect CLI has no terminal surface |
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

**OQ-4 — elicitation (OPEN 11-20).** Stable in SDK v0.16, so the blocker is
gone and the question is now a product one: Crush has no surface that asks a
user to fill a form or open a URL on the Agent's behalf. Until one exists the
client track answers `-32601` and the server track advertises no elicitation
capability, which is conformant. Revisit if `crush acp connect` grows a
prompting surface.

**OQ-5 — `systemPrompt` (OPEN 11-20).** v0.18 models `append` and `override`.
`append` is cheap: prepend the client's text to the assembled prompt.
`override` replaces the system prompt entirely, which would drop the tool
instructions and context files Crush depends on, so it needs its own design.
Recommendation: advertise `systemPrompt` with `append` only when the append
path is implemented.

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

**Conformance position (current, after M-SDK):** no MUST gap and no SHOULD
gap remains in the capabilities Crush advertises. `thinking` is a real boolean
where the client advertises it, and the select is the documented fallback. The
root set is `[cwd]` plus each session's `additionalDirectories`, both enforced.
Remaining open items are decisions (OQ-4 elicitation, OQ-5 systemPrompt), one
deferred optional emission pair (M-SDK-4 notices/compaction, gates
unadvertised), and the optional families in §5.5.

## 8. Tests

```bash
go test ./internal/acp/... -count=1            # unit + integration
go test ./test/acp/spec/... -count=1           # conformance, golden, Tangerg interop
CRUSH_BIN=$(go build -o /tmp/crush .) \
  go test -tags e2e ./test/acp/ -count=1       # subprocess e2e
./test/acp/run.sh all                          # full runner
```

The OpenAgents conformance harness is documented in `test/acp/README.md`;
informational only (Node/pnpm), not wired into `run.sh`.

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
and the complete Fantasy-to-ACP stop-reason table.

Still uncovered: MCP prompt resolution end-to-end (needs a live MCP client),
blob attachments flowing all the way through `Coordinator.Run`.

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
