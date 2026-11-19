# ACP (Agent Client Protocol) Implementation Plan

Crush implements the Agent Client Protocol as an **ACP Server** (Zed,
JetBrains, Neovim drive it) and, secondarily, as an **ACP Client**
(`crush acp connect` drives external agents). Spec:
https://agentclientprotocol.com/protocol/v1/ — the v1 docs are vendored in
`docs/acp/protocol/v1/` (22 files, scraped via `scripts/fetch_acp_docs.go`).

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

Runtime SDK: `github.com/coder/acp-go-sdk` v0.13.5 (Coder; most active,
complete v1 typed model, stdio + streaming helpers). `Tangerg/acp` v0.2.4 is
a test-only fixture (Zed 1.17.2 transcript + 154 cross-SDK fixtures).

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
| `mcpCapabilities` (http/sse) | OPTIONAL | ✅ empty |
| `sessionCapabilities` (close/list/resume/delete) | OPTIONAL | ✅ four advertised |
| `auth.logout` capability | OPTIONAL | ✅ advertised; `Logout` success no-op |
| `authenticate` method | only reachable if advertised; unadvertised SHOULD answer `-32601` (`error.md:46`) | ✅ `authMethods:[]` so nothing owed; stray call → `-32601` (`dispatcher.Authenticate`) |
| `systemPrompt` support | OPTIONAL | ✅ unadvertised; SDK drops the field before Crush sees it (G11) |
| `additionalDirectories` support | OPTIONAL | ✅ unadvertised; field ignored (O23) |

### 4.2 Session lifecycle

| Method | Level | Status |
|---|---|---|
| `session/new` | MUST | ✅ creates session, returns `modes`+`configOptions`; cwd must be absolute, validated; `mcpServers` connected |
| `session/prompt` | MUST | ✅ runs coordinator, returns `StopReason` |
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

14 variants exist; Crush emits 11.

| Update | Level | Status |
|---|---|---|
| `user_message_chunk` | MUST on load replay | ✅ replay emits it (`UpdateUserMessageText`); live-turn echo skipped by decision (§6 S2) |
| `agent_message_chunk` | baseline | ✅ `subscribeMessages` |
| `agent_thought_chunk` | SHOULD | ✅ `streamThoughts` (S3) |
| `tool_call` | SHOULD | ✅ start with kind/title/locations/rawInput (S1/S10) |
| `tool_call_update` | SHOULD | ✅ pending→in_progress→completed/failed lifecycle (S10) |
| `plan` | SHOULD | ✅ full-replace from `todos` results (S4) |
| `available_commands_update` | MAY | ✅ advertised (skills/custom/MCP prompts, `input.hint`) + executable (O24) |
| `current_mode_update` | MAY | ✅ on mode change (S9) |
| `config_option_update` | MAY | ✅ on config change, complete state (S9) |
| `session_info_update` | MAY | ✅ on `session/new` and whenever the stored title changes (the auto-generated title is announced) |
| `usage_update` | MAY | ✅ at run completion; `used`/`size` non-null; cost has `amount`+`currency` |
| `notice` | MAY, gated on `clientCapabilities.session.notices` | ❌ not emitted; SDK has no `session` field, and the MUST-NOT-without-gate rule is satisfied by omission |
| `compaction_update` / `compaction_summary_chunk` | MAY, gated on `session.compaction`; replayed by load | ❌ not emitted, not replayable; same SDK gate |
| `messageId` on chunks | MAY | ❌ SDK field exists but UNSTABLE |

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
| Fields: `title` required; `name` SHOULD; `kind`/`status`/`locations`/`rawInput`/`rawOutput` optional | SHOULD (`name`) | ✅ all except `name` — the SDK types have no `Name` field (verified `types_gen.go`), so it needs an SDK bump |
| All paths absolute (`tool-calls.md:307,353`) | MUST | ✅ `absPath`/`absPather` at `toolLocations`, `diffContentFor`, load replay |
| `session/request_permission` with 4 option kinds | MUST when needed | ✅ `buildPermissionOptions` (O20/G1) |
| Unrecognized outcome ≠ approval | SHOULD | ✅ `mapPermissionOutcome` allow-list (O20) |
| Remembered decisions owned by the Agent, applied by not asking again | SHOULD | ✅ always-kinds are remembered per session+tool and applied without re-asking, and the option names state the scope. Coarser than the spec's examples: the scope is the tool name, not a command or directory |
| Client answers pending permission `cancelled` on `session/cancel` | MUST (peer duty) | ⚠️ agent side correct: denial without error, no spurious `failed` |

### 4.6 Elicitation

OPTIONAL. ❌ neither track: the SDK ships only `Unstable*` types and the
client track does not implement `ClientExperimental`. Essentials for a future
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
| Send `headers` only to the configured origin | SHOULD | n/a (no HTTP/SSE advertised) |

Collision safety: registry keys are namespaced `acp:<sessionID>:<name>`
(`mcp.ACPServerName`); `mcp.TeardownSession` closes a session's servers on
close/delete and replaces them on resume; `reconcile` skips `acp:` names so a
config reload cannot tear down a live session; `filterOtherSessionsMCPTools`
drops other sessions' tools from the palette. `context.WithoutCancel` lets
the connection outlive the setup request.

Limitations (recorded, not hidden): tool names embed the session id; MCP
prompts/resource completions and the `/mcp` UI read the config-file registry
and do not surface ACP-injected servers; `env` is additive to the inherited
process env; OAuth, HTTP/SSE headers, `EnabledTools`/`DisabledTools` and
channels have no ACP equivalent and are not applied — none are advertised.

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
embeddedContext:true}`, `sessionCapabilities{close,delete,list,resume}`,
`auth.logout`, `authMethods:[]`. No advertised capability is behaviourally
empty; `additionalDirectories` was retracted (O23) precisely because it had
no enforcement behind it.

Not advertised, correctly so: `sessionCapabilities.systemPrompt` (SDK drops
the field, G11), `additionalDirectories` (O23/OQ-1), any `mcpCapabilities`
(no HTTP/SSE), elicitation and terminal auth (SDK-gated, and Crush has no
auth session to terminate).

## 5. Open items

Grouped by the actual blocker.

### 5.1 SDK-gated (pinned v0.13.5)

- Tool-call `name` field (SHOULD, 4.5) — no `Name` on SDK types.
- `messageId` on chunks (MAY) — field UNSTABLE.
- Elicitation (OPTIONAL) — only `Unstable*` variants.
- `notice`/`compaction` gates (MAY) — `ClientCapabilities.session` not
  modelled; omission keeps us conformant.
- Boolean config-option client gate (MUST-not-emit) — unreadable; the spec's
  `select` fallback is used instead, so the MUST holds.

### 5.2 Crush work

- **AUTH-1 `authenticate`** (conditional MUST): register provider auth
  methods (copilot, hyper, openai) as `authMethods` entries with
  `type:"agent"` and trigger OAuth. Nothing is owed while `authMethods:[]`.
- **O8 terminals** (OPTIONAL): no PTY dependency; could be backed by
  `internal/shell` instead — re-evaluate if `crush acp connect` grows a
  terminal pane.

### 5.3 Deliberate decisions (not gaps)

- `additionalDirectories` retracted rather than half-enforced (O23).
- `thinking` is a `select` with on/off — the spec's own fallback
  (`session-config-options.md:209`); `req.Boolean` stays for cached clients.
- No live-turn `user_message_chunk` echo (decision S2, §6).
- Shell builtins (`provider`, `model`, `mcp`, …) are not advertised: they
  mutate crushrc config rather than produce a prompt. Two designs were
  weighed — parse `/<builtin>` from prompt text and dispatch via `shell.Run`
  with the `ConfigBuilder` on context (spec-idiomatic), or wrap each as a
  `tool_call` with `kind:"other"` (faster, loses slash-command UX).
- YOLO as a config option, if ever added, must use the same select treatment
  (it cannot ship as a boolean gate).
- Mode switch is workspace-wide: two concurrent ACP sessions share one agent;
  `currentValue` stays truthful because it reads live state back. `plan`+
  `yolo` has no ACP id; selecting `plan` resets permissions to Normal, which
  raises oversight — the safe direction.

## 6. Decision log

| ID | Date | Decision | Why |
|---|---|---|---|
| SDK | — | `coder/acp-go-sdk` v0.13.5 runtime; Tangerg test-only | most complete v1 model, stdio + helpers |
| O2 | 11-17 | `thinking` as `select`, not `boolean` | boolean gate unreadable; spec names the select fallback; MUST holds |
| O23 | 11-17 | retract `additionalDirectories` (supersedes O3) | an advertised capability with no behaviour behind it is worse than none |
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

**OQ-1 — root set (RESOLVED 11-19).** `tools.RootsContextKey` carries `[cwd]`
into every run via `sessionAgent.Run`. `OutsideRoots` compares the resolved
path by component and fails closed; `view`, `write`, `edit` and `multiedit`
ask permission before touching a path outside it. `bash` always prompts when
a root set is present: a shell cannot be confined by a path check alone.
`additionalDirectories` stays retracted (O23), so the set is just `[cwd]`.

**OQ-2 — RESOLVED 11-18: the substitution rule is shared.**
`commands.SubstituteArgs` in `internal/commands` is now the one rule both the
TUI dialog path and the ACP prompt path call. What stays ACP-local is the
part that legitimately differs: the TUI collects arguments through a dialog,
while a prompt binds them positionally from the trailing text (`splitCommand`,
`bindArgs` in `internal/acp/commands.go`). The drift risk OQ-2 recorded is
gone for substitution; argument collection is by design.

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

**Conformance position:** no MUST gap remains. The only SHOULD still open is
`tool name` (SDK-gated). Two behavioural caveats remain: the select fallback
for `thinking`, and remembered permission scope being the tool name rather
than the command or directory. The root set is now enforced for path-touching
tools, so `additionalDirectories` stays retracted (O23) and the set is `[cwd]`.

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
fixtures (33) and the Tangerg/Zed interop fixtures.

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
