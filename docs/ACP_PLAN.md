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
| Provide `agentInfo` (name/title/version) | SHOULD (required in future) | ⚠️ sets name+version, **omits `title`** |
| `authMethods` present in response (default `[]`) | MUST (field present) | ⚠️ not set → serializes as `null`, not `[]` |
| `loadSession` capability | OPTIONAL | ✅ `true` |
| `promptCapabilities` (image/audio/embeddedContext) | OPTIONAL (MUST support Text+ResourceLink in prompts regardless) | ✅ all `true` |
| `mcpCapabilities` (http/sse) | OPTIONAL | ✅ empty (no MCP transport) |
| `sessionCapabilities` (close/list/resume/delete/additionalDirectories) | OPTIONAL | ⚠️ only `close` set |
| `auth.logout` capability | OPTIONAL | ❌ not advertised |
| `authenticate` method | MUST exist; return `auth_required`/error if unused | ⚠️ stub "auth not supported" |
| `logout` method | MUST exist if `auth.logout` advertised | ⚠️ stub "logout not supported" (not advertised, so safe) |

### 3.2 Session lifecycle methods

| Method | Level | Crush status |
|---|---|---|
| `session/new` | MUST | ✅ creates SQLite session, returns `sessionId` |
| `session/prompt` | MUST | ✅ runs coordinator, returns `StopReason` |
| `session/cancel` (notification) | MUST | ✅ cancels coordinator |
| `session/update` (notifications) | MUST | ⚠️ only `agent_message_chunk` emitted |
| `session/close` | OPTIONAL (advertise `sessionCapabilities.close`) | ✅ implemented + advertised |
| `session/list` | OPTIONAL (advertise `sessionCapabilities.list`) | ⚠️ implemented but **not advertised**; `SessionInfo` **omits required `cwd`** |
| `session/load` | OPTIONAL (advertise `loadSession`) | ❌ **advertised `true` but method returns error — incomplete** |
| `session/resume` | OPTIONAL (advertise `sessionCapabilities.resume`) | ❌ returns error (not advertised — acceptable) |
| `session/delete` | OPTIONAL (advertise `sessionCapabilities.delete`) | ❌ not implemented (not advertised) |
| `session/set_mode` | OPTIONAL (legacy; prefer config options) | ✅ acknowledges (no-op), no `current_mode_update` |
| `session/set_config_option` | OPTIONAL | ✅ acknowledges, returns empty `configOptions` |

### 3.3 Prompt turn & streaming (`session/update` types)

All are **notifications** carried in `{"sessionId", "update": {"sessionUpdate": <type>, ...}}`.
The spec distinguishes `session/load` (MUST replay history via these) from
`session/resume` (MUST NOT replay).

| Update type | Level | Crush status |
|---|---|---|
| `user_message_chunk` | SHOULD (echo user input) | ❌ not emitted |
| `agent_message_chunk` | baseline | ✅ emitted on happy path (`subscribeMessages`) |
| `agent_thought_chunk` | SHOULD (reasoning) | ❌ not emitted |
| `tool_call` | SHOULD | ❌ not emitted |
| `tool_call_update` | SHOULD (in_progress/completed/failed) | ❌ not emitted |
| `plan` | SHOULD | ❌ not emitted |
| `available_commands_update` | MAY | ❌ not emitted |
| `current_mode_update` | MAY | ❌ not emitted |
| `config_option_update` | MAY | ❌ not emitted |
| `session_info_update` | MAY (ties to `session/list`) | ❌ not emitted |
| `usage_update` | MAY (context + cost) | ❌ not emitted |

`messageId` (per-message opaque id; chunks sharing it belong to one message) is a
**MAY** field on `agent_message_chunk`/`user_message_chunk` — **Crush never sets
it** (SDK v0.13.5 still marks `MessageId` UNSTABLE; track on SDK bump).

### 3.4 Content blocks (prompts & outputs)

| Block | Level | Crush status |
|---|---|---|
| `text` | MUST | ✅ |
| `resource_link` | MUST | ✅ (in `extractPromptText`) |
| `image` | OPTIONAL (gated by `promptCapabilities.image`) | ⚠️ capability advertised, not parsed/streamed |
| `audio` | OPTIONAL | ⚠️ advertised, not handled |
| `resource` (embedded) | OPTIONAL (gated by `embeddedContext`) | ⚠️ advertised, not handled |
| `annotations` on blocks | OPTIONAL | ❌ ignored |

### 3.5 Tool calls

| Requirement | Level | Crush status |
|---|---|---|
| Emit `tool_call` + `tool_call_update` during execution | SHOULD | ❌ not emitted |
| Tool kinds: `read/edit/delete/move/search/execute/think/fetch/other` (+`switch_mode`) | OPTIONAL taxonomy | ❌ no tool events at all |
| Tool content: `content` / `diff` / `terminal` | SHOULD | ❌ |
| `toolCallId`, `title`, `kind`, `status`, `locations`, `rawInput`, `rawOutput` | — | ❌ |
| `session/request_permission` (4 option kinds: `allow_once`/`allow_always`/`reject_once`/`reject_always`) | MUST when needed | ✅ bridge implemented |
| Client MUST respond `cancelled` to pending permission on `session/cancel` | MUST | ⚠️ not verified |

### 3.6 Elicitation (`elicitation/create`, `elicitation/complete`)

OPTIONAL (gated by `clientCapabilities.elicitation.{form,url}`). Crush: ❌ not
implemented on either side. Spec essentials:
- `mode` discriminator is **required** (`form`/`url`); no implicit form default.
- Form: restricted JSON Schema in `requestedSchema`; MUST NOT request secrets.
- URL: unique `elicitationId` + `elicitation/complete` notification; MUST NOT
  fall back form→url; client MUST show full URL + consent.
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
`sessionCapabilities.close`. **Missing optional:** `sessionCapabilities.list`,
`.resume`, `.delete`, `.additionalDirectories`, `auth.logout`, `authMethods:[]`.

### 3.11 Extensibility (`_meta`)

Every type has a `_meta` field. ❌ Crush does not propagate it. Reserved root
keys `traceparent`/`tracestate`/`baggage` are for W3C trace context. Custom
capabilities advertised via `_meta` on capability objects; custom methods start
with `_`.

### 3.12 MCP servers

`session/new`/`load`/`resume` accept `mcpServers` (stdio/http/sse). ❌ Crush does
not connect to MCP servers during session setup (capabilities advertise none).

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

### Gaps to close (prioritized)

**P0 — correctness / visible hangs**
1. Stream `tool_call` + `tool_call_update` (in_progress/completed/failed). Editors
   show tool progress; without it the agent looks frozen during edits/searches.
   Files: `event_bridge.go`, `notify` subscription. Test: `TestPromptStreamsToolCalls`.
2. Fix `loadSession:true` vs unimplemented `session/load` — either implement
   replay or set `loadSession:false`. Currently advertises a capability we don't serve.
3. `ListSessions` MUST populate `SessionInfo.cwd` (required, currently `""`).
4. `Initialize` MUST serialize `authMethods` as `[]`, not `null`.

**P1 — spec completeness**
5. Echo `user_message_chunk` on `session/prompt`.
6. Implement `session/load` replay (or drop capability). Decide `session/resume`
   (wire vs capability rejection).
7. Implement `elicitation/create` + `elicitation/complete` (form+url, `accept`/
   `decline`/`cancel`, `elicitationId` lifecycle) — client callback + server stub.
8. `_meta` passthrough on message paths; reserve W3C trace keys.
9. Emit `available_commands_update` after session creation.
10. `config_option_update` on config change; support `sessionCapabilities.list`
    advertisement + `session_info_update` metadata sync.
11. `authMethods:[]` + `agentInfo.title`; `auth.logout` capability if `logout` wired.

**P2 — UX / observability**
12. `agent_thought_chunk` for reasoning deltas.
13. `usage_update` (context + cost).
14. `plan` updates (each update is a FULL replace).
15. `current_mode_update` after `set_mode`; return `modes`/`configOptions` in
    `session/new` response.
16. Boolean config options (`type:"boolean"`, gated on
    `clientCapabilities.session.configOptions.boolean`) + `category` field.
17. `additionalDirectories` capability + handling in `/new`, `/load`, `/resume`.

**P3 — client-side**
18. Real terminal callbacks (or document out-of-scope).
19. Render streaming chunks in `crush acp connect` loop.

**P4 — tests**
20. Regression tests for each missing update type; `TestConformanceSessionLoad`;
    elicitation roundtrip; `_meta` passthrough.

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

- **`session/load` inconsistency** — capability advertised `true` but method
  errors. Highest-priority correctness gap (P0 #2).
- **Mandatory `SessionInfo.cwd`** missing in `ListSessions` (P0 #3).
- **`authMethods` null vs `[]`** serialization (P0 #4).
- All streaming update types beyond `agent_message_chunk` (P0 #1, P2 #12–17).
- `user_message_chunk` echo, `messageId` propagation (P1 #5).
- Elicitation fully unspecified in code (P1 #7).
- `_meta` passthrough + reserved trace keys (P1 #8).
- `session/delete`, `session/resume` wiring decision (P1 #6).
- MCP server connection at session setup (§3.12).
- `additionalDirectories` capability (P2 #17).
- Boolean config options + `category` (P2 #16).
- `$/cancel_request` handling + permission-`cancelled` on cancel (§3.9).
- `switch_mode` tool kind not represented in any tool event.
- Client terminal callbacks + `connect` streaming display (P3).
- `agentInfo.title` omitted; `auth.logout` capability not advertised (§3.1).

All other spec areas (baseline methods, permission bridge, fs client ops, text/
resource_link content, `session/close`, stdio transport, `_meta` field presence)
are covered.
