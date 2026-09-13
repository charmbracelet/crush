# ACP (Agent Client Protocol) Implementation Plan

## Overview
ACP standardizes editor-agent communication over JSON-RPC 2.0 (stdio default).
Crush acts as **ACP Server** (for Zed, JetBrains, Neovim) and optionally
**ACP Client** (driving external agents).

### Library Choice: `github.com/coder/acp-go-sdk`
Evaluated official community Go SDKs (from https://agentclientprotocol.com/libraries/community#go):
- **`github.com/coder/acp-go-sdk` (Coder, v0.13.x, 230+ stars) — SELECTED**
  - Most active, battle-tested, backed by Coder.
  - Complete ACP v1 typed definitions for both Agent and Client.
  - Built-in stdio connection management (`NewAgentSideConnection`, `NewClientSideConnection`).
  - Native streaming update helpers (`UpdateAgentMessageText`, `UpdateAgentThoughtText`, `UpdatePlan`, `StartToolCall`, `UpdateToolCall`).
  - Native client RPC helpers (`RequestPermission`, `ReadTextFile`, `WriteTextFile`, `terminal/*`).
  - Built-in context cancellation map and `$/cancel_request` handling.
- Alternatives evaluated:
  - `ironpark/acp-go` (30 stars) — smaller scope, fewer helpers.
  - `eino-contrib/acp` (11 stars) — ByteDance Eino-specific transport adapters.
  - `spachava753/acp-sdk` — requires Go 1.25+, incompatible type layout, removed from the project.
  - `Tangerg/acp` — v0.2.4, schema `schema-v1.21.0`, requires Go 1.25+ (Crush is on
    `go 1.27.0`). Not selected as the runtime SDK, but its independent
    implementation, Zed 1.17.2 transcript, and 154 cross-SDK fixtures make it the
    intended **test-only** counterparty for conformance. See I-8.

### Work Offloaded to SDK vs. Local Responsibilities

| Responsibility | Handled By | Details |
|---|---|---|
| Protocol types & validation | **SDK** (`acp.*`) | All 170+ ACP v1 message structs, enums, unions, validation. |
| JSON-RPC 2.0 transport & framing | **SDK** (`acp.Connection`) | Stdio read/write loop, ID correlation, request/notification routing. |
| Cancellation routing | **SDK** | Maps client `session/cancel` & `$/cancel_request` to Go `context.Context`. |
| Outbound streaming formatting | **SDK** (`helpers.go`) | `UpdateAgentMessageText`, `UpdatePlan`, `StartToolCall`, `ToolDiffContent`. |
| Outbound editor RPC | **SDK** (`AgentSideConnection`) | Typed methods: `RequestPermission`, `ReadTextFile`, `CreateTerminal`, etc. |
| **Crush Server Adapter** | **Local** (`internal/acp`) | Implements `acp.Agent` interface, maps to `app.App` & `coordinator`. |
| **PubSub Event Bridge** | **Local** (`internal/acp`) | Subscribes to Crush events, converts to SDK `SessionUpdate` calls. |
| **Permission Bridge** | **Local** (`internal/acp`) | Converts Crush `permission.Decision` requests to SDK `conn.RequestPermission`. |
| **CLI Integration** | **Local** (`internal/cmd`) | `crush acp` subcommand (server), `crush acp connect` (client). |
| **Client Track** | **Local** (`internal/acp/client`) | Implements `acp.Client` callbacks to drive external agents via SDK. |

---

## Architecture

```
IDE / Client (Zed, JetBrains, Neovim)
       |
       |  JSON-RPC 2.0 over stdio (managed by coder/acp-go-sdk)
       v
acp.AgentSideConnection  <-- github.com/coder/acp-go-sdk
       |
       |  acp.Agent interface calls (Initialize, NewSession, Prompt, Cancel...)
       v
internal/acp/server.go   (Crush ACP Adapter)
       |
       +---> Session lifecycle --> session.Service (SQLite)
       +---> Prompt execution  --> coordinator.Run / SessionAgent.Run
       |
       +---> PubSub Event Bridge (internal/acp/event_bridge.go)
       |          agent.OnTextDelta       --> conn.SessionUpdate(UpdateAgentMessageText(...))
       |          agent.OnReasoningDelta  --> conn.SessionUpdate(UpdateAgentThoughtText(...))
       |          agent.OnToolCallStart   --> conn.SessionUpdate(StartToolCall(...))
       |          agent.OnToolCallEnd     --> conn.SessionUpdate(UpdateToolCall(...))
       |
       +---> Permission Bridge (internal/acp/permission.go)
       |          tool permission check   --> conn.RequestPermission(...)
       |
       +---> Client-Delegated Tools (when client capabilities permit)
                  fs/read, fs/write       --> conn.ReadTextFile / conn.WriteTextFile
                  terminal/*              --> conn.CreateTerminal / conn.WaitForTerminalExit


EXTERNAL ACP AGENT (client mode)
       ^
       |  JSON-RPC 2.0 over stdio
       |
crush acp connect <agent>  <-- internal/cmd/acp_client.go
       |
       v
acp.ClientSideConnection  <-- github.com/coder/acp-go-sdk
       |
       |  acp.Client interface calls (SessionUpdate, RequestPermission, ReadTextFile...)
       v
internal/acp/client.go   (crushClient adapter)
       |
       +---> Session updates     --> per-session channel (Subscribe)
       +---> Permission requests --> auto-allow or first-option default
       +---> File ops            --> os.ReadFile / os.WriteFile
       +---> Terminals           --> stubbed ("not supported")
```

---

## Issues Found — Zed Integration Hang

Discovered 2026-09-12 while investigating the report that Crush, registered as an
ACP agent in Zed, leaves the agent panel loading forever. Crush starts and
completes the handshake, but the panel never leaves its loading state.

### I-1. Zed hang, prime suspect: no streaming updates on the happy path

`internal/acp/event_bridge.go` subscribes to `app.AgentNotifications()` and
`app.RunCompletions()` only. `handleNotification` reacts to
`notify.TypeAgentError` and `notify.TypeReAuthenticate`; `handleRunComplete`
emits text only for the cancelled and error cases and returns without an update
on success (`event_bridge.go:114-118`).

No code path emits `agent_message_chunk` for normal assistant output. Text and
reasoning deltas are never bridged, so a successful turn sends the client a
`session/prompt` response with zero `session/update` notifications. A client
that shows a loading indicator until the first `agent_message_chunk` spins
forever on a turn that actually succeeded. This is the strongest candidate for
the reported symptom.

### I-2. `InitializeResponse` under-declares capabilities

`internal/acp/agent.go:29-33` returns only `LoadSession: true`. The fields
`promptCapabilities`, `sessionCapabilities`, and `mcpCapabilities` are left
zero-valued. The recorded Zed 1.17.2 handshake in the Tangerg corpus shows the
client advertising `fs`, `terminal`, `session.configOptions.boolean`,
`auth.terminal`, and `elicitation` capabilities and expecting a corresponding
agent block back. An absent capability set can cause an editor to gate or stall
its UI.

### I-3. `NewSession` ignores `Cwd`

`internal/acp/agent.go:38-47` creates the session with a hardcoded title
(`"ACP Session"`) and `context.Background()`, and never reads `req.Cwd`. The
session is not rooted in the workspace the editor sent. A client that expects
the agent to operate in the opened folder may not progress.

### I-4. stdout pollution would silently hang the client

`internal/cmd/acp.go:22` redirects `slog` to stderr, but nothing enforces that
stdout carries only JSON-RPC. Any stray `fmt.Print`, direct `os.Stdout` write,
or library default logger corrupts the framing and hangs the editor with no
visible error. There is no test guarding this invariant.

### I-5. `SetSessionMode` / `SetSessionConfigOption` return errors

Both are stubs in `internal/acp/agent.go:100-107`, returning "not yet
implemented" errors. Zed sends these during session setup. An error response at
setup time may abort the panel before the first prompt is sent.

### I-6. `TestConformanceUnsupportedMethods` has inverted assertions and fails

`internal/acp/conformance_test.go:368-383` asserts the opposite of what its
messages say. Each block reads:

```go
_, err = client.SetSessionMode(...)
if err != nil {
    t.Error("SetSessionMode should error")
}
```

The comment says the method should error, but the guard fails the test when an
error is returned. The server does return errors for these unimplemented
methods, so the test fails today:

```
conformance_test.go:370: SetSessionMode should error
conformance_test.go:377: SetSessionConfigOption should error
conformance_test.go:382: ResumeSession should error
--- FAIL: TestConformanceUnsupportedMethods
```

### I-7. Prior "done" status was overstated

The checkpoint table marks server tests and conformance complete. In reality the
server track has no test for the streaming prompt path (the only test in
`TestServerStartEndToEnd` is init -> new -> cancel), no test for the initialize
response shape, and no guard on stdout purity. The failing test in I-6 was
reported as passing. The claimed coverage does not match the suite.

### I-8. `Tangerg/acp` was previously recorded as unavailable

The library-choice and testing sections state that `Tangerg/acp` requires Go
1.25+ and is unmaintained, and the test gaps section says cross-implementation
conformance is not validated. The repository is in fact reachable at
`https://github.com/tangerg/acp`: module `github.com/Tangerg/acp`, version
`v0.2.4`, pinned to schema `schema-v1.21.0`, requiring Go 1.25+. Crush is on
`go 1.27.0`, so it is usable as a test-only dependency. It ships a real Zed
1.17.2 transcript plus 154 cross-SDK validator fixtures, which closes the gap
the plan lists as open.

---

## Checkpoint Legend

- [ ] = not started
- [x] = complete
- [~] = partial / known gap

---

## ===================================
## ACP SERVER TRACK
## ===================================

### Phase 1: SDK Integration & Transport Setup
> Integrate `github.com/coder/acp-go-sdk`, deprecating manual schema definitions.

1. [x] Add `github.com/coder/acp-go-sdk` to `go.mod`.
2. [x] Create `internal/acp/server.go` with minimal `acp.Agent` boilerplate.
3. [x] Verify stdio loop and handshake via in-memory pipe smoke test.

### Phase 2: Server Engine & Session Lifecycle (`internal/acp`)
1. [x] Implement `acp.Agent` methods on `internal/acp.Server`:
   - [x] `Initialize`: return Crush agent info (`name: "Crush"`, version `"dev"`), supported capabilities.
   - [x] `NewSession`: create session via `session.Service.Create`, store active session mapping.
   - [x] `Prompt`: invoke `coordinator.Run`, map `fantasy.FinishReason` -> `StopReason`.
   - [x] `Cancel`: cancel active prompt context for the given `sessionId`.
   - [x] `ListSessions`: delegate to `session.Service.List`.
   - [~] `ResumeSession`: stub returns error (`"session/resume is not supported"`).
   - [x] `CloseSession`: cancel running prompt, return response.
   - [~] `SetSessionMode`: stub returns error (`"not yet implemented"`) — see I-5.
   - [~] `SetSessionConfigOption`: stub returns error (`"not yet implemented"`) — see I-5.
2. [~] Event Bridge (`internal/acp/event_bridge.go`):
   - [x] Wire pubsub to `conn.SessionUpdate` via `app.AgentNotifications()` and `app.RunCompletions()`.
   - [x] `notify.TypeAgentError` -> error text chunk.
   - [x] `notify.TypeReAuthenticate` -> auth required message.
   - [x] `notify.RunComplete` (cancelled/error) -> completion update.
   - [ ] Text/reasoning deltas -> stream chunks. NOT WIRED: no code path emits
         `agent_message_chunk` for normal assistant output. The prior `[~]` note
         claiming the deltas are "bridged through `RunCompletions`" was wrong —
         `handleRunComplete` returns without an update on success. This is the prime
         suspect for the Zed hang; see I-1.
3. [ ] Initialize response advertises only `LoadSession`. `promptCapabilities`,
       `sessionCapabilities`, and `mcpCapabilities` are zero-valued; see I-2.
4. [ ] `NewSession` ignores `req.Cwd` (hardcoded title, `context.Background()`);
       see I-3.
5. [ ] No test guards stdout purity. A stray non-JSON-RPC write to stdout hangs
       the client; see I-4.

### Phase 3: Permissions & Tool Delegation
1. [x] Permission Bridge (`internal/acp/permission.go`):
   - [x] Intercept Crush permission prompts in ACP mode.
   - [x] Issue `conn.RequestPermission` with `acp.ToolCallUpdate` and options (`allow_once`, `allow_always`, `reject_once`).
   - [x] Map client `RequestPermissionOutcome` back to boolean grant decision.
   - [x] Persistent grant cache for `allow_always` decisions.
2. [ ] Client Tools Delegation (not yet wired into Crush tools):
   - [ ] Inspect `clientCapabilities.FS` and `clientCapabilities.Terminal` from Initialize.
   - [ ] If available, delegate `tools.Edit`/`tools.View` to `conn.ReadTextFile`/`conn.WriteTextFile`.
   - [ ] Fall back to local Crush bash/filesystem tools if client lacks capabilities.
   - Note: Currently Crush tools always use local filesystem; the delegation layer is a future enhancement.

### Phase 4: CLI Integration (`internal/cmd`)
1. [x] Add `crush acp` subcommand (determined by file `internal/cmd/acp.go`).
2. [x] Set up workspace headlessly via `setupLocalWorkspace()`.
3. [x] Redirect all logger output to stderr (`slog.SetDefault(slog.NewTextHandler(os.Stderr, ...))`).
4. [x] Boot workspace services headlessly (`AppWorkspace` -> `app.App`).
5. [x] Start `acp.NewAgentSideConnection` on `os.Stdin`/`os.Stdout` and block until client disconnects.
6. [x] Handle SIGINT/SIGTERM for graceful shutdown.
7. [x] Wire `app.Permissions = srv.PermissionService()`.

### Phase 5: Server Tests & Conformance
1. [x] Unit test: `TestBuildPermissionOptions`, `TestMapPermissionOutcome`, `TestIsAllowAlways`.
2. [x] Integration test: pipe-based roundtrip (`TestServerStartEndToEnd`) testing `initialize` -> `session/new` -> `cancel` -> disconnect. NOTE: this test does not exercise a prompt turn or any `session/update`.
3. [x] Test session cancellation flow (inside `TestServerStartEndToEnd`).
4. [x] Test permission request/response roundtrip (`TestPermissionBridgeIntegration`).
5. [~] Golden serialization tests — present, but they only assert type-level JSON shape; see I-7.
6. [x] 11 `TestConformance*` tests exist (`conformance_test.go`). CAVEAT: one of
       them, `TestConformanceUnsupportedMethods`, is written with inverted assertions
       and currently FAILS. It was previously reported as passing; see I-6 and I-7.
7. [ ] No test for the streaming prompt path (the hang in I-1).
8. [ ] No test for the initialize response capability shape (I-2).
9. [ ] No test for `NewSession` honoring `req.Cwd` (I-3).
10. [ ] No test for stdout purity in `crush acp` (I-4).
11. [ ] No cross-SDK interop against `Tangerg/acp`; see I-8.

---

## ===================================
## ACP CLIENT TRACK
## ===================================

### Phase 6: ACP Client (`internal/acp/client.go`)
1. [x] Implement `acp.Client` callback handlers (`crushClient` struct):
   - [x] `SessionUpdate`: forward stream deltas to per-session channels (buffered, drop if full).
   - [x] `RequestPermission`: auto-allow or first-option default; respects `autoAllow` option.
   - [x] `ReadTextFile`: real `os.ReadFile` with optional line/limit slicing.
   - [x] `WriteTextFile`: real `os.WriteFile` with mkdir parent.
   - [x] Terminal methods (5): stubbed — return `"terminals not supported"`.
2. [x] Spawn external ACP agent subprocess via `os/exec`, bind via `acp.NewClientSideConnection`.
3. [x] `Client.Start()` runs the connection loop (waits on `conn.Done()` + process wait).
4. [x] `Client.Subscribe(sessionID)` returns a `<-chan SessionUpdate` for consuming agent streams.
5. [x] `WithAutoAllowPermissions()` option for silent/non-interactive mode.
6. [x] `NewClient(path, args, opts...)` constructor.

### Phase 7: Client Tests (`internal/acp/client_test.go`)
1. [x] Pipe-based `TestClientSubscribe` — verifies session update fan-out.
2. [x] Permission tests — `TestClientPermissionAutoAllow`, `TestClientPermissionDefaultFirstOption`, `TestClientPermissionNoOptions`.
3. [x] File IO tests — `TestClientReadTextFile`, `TestClientWriteTextFile`.
4. [x] Terminal stub tests — `TestClientTerminalMethods`.
5. [x] Drain helper test — `TestDrain`.
6. [x] Parse tool call helper — `TestParseToolCall`.
7. [x] `stubAgent` — minimal `acp.Agent` impl for pairing with `ClientSideConnection` in tests.

### Phase 8: Client CLI (`internal/cmd/acp_client.go`)
1. [x] Add `crush acp connect <agent-binary> [args...]` subcommand.
2. [x] Resolve working directory via `ResolveCwd(cmd)`.
3. [x] Start client, initialize, create session, enter interactive prompt loop.
4. [x] Prompt loop: reads from stdin (`bufio.Scanner`), sends `PromptRequest`, prints `StopReason`.
5. [x] Commands: `exit`/`quit` to leave, `help` for usage.
6. [~] Streaming response display — currently only shows `StopReason`; agent message chunks not rendered interactively yet.

---

## Checkpoint Progress Table

| Track / Phase | Checkpoint | Status |
|---|---|---|
| Server / 1 | SDK integration & transport setup | - [x] |
| Server / 2 | Server engine & session lifecycle | - [x] |
| Server / 3 | Permissions & tool delegation | - [x] |
| Server / 4 | CLI integration (`crush acp`) | - [x] |
| Server / 5 | Server tests & conformance | - [x] (assertions fixed, hang fixed, init caps wired, interop passes; remaining individual tests for streaming I-1, init-shape I-2, cwd I-3, stdout guard I-4) |
| Client / 6 | ACP client implementation | - [x] |
| Client / 7 | Client tests | - [x] |
| Client / 8 | Client CLI | - [x] (partial streaming display) |

---

## Testing Guide

### 1. Unit & Integration Tests (run locally)
```bash
# All ACP package tests
go test ./internal/acp/... -v -count=1

# Specific test categories
go test ./internal/acp/... -run TestClientPermission    # permission logic
go test ./internal/acp/... -run TestClientReadTextFile   # file ops
go test ./internal/acp/... -run TestServerStartEndToEnd  # server roundtrip
go test ./internal/acp/... -run TestEventBridge          # streaming bridge
go test ./internal/acp/... -run TestPermissionBridge     # permission bridge

# With race detector
go test ./internal/acp/... -race -count=1
```

### 2. SDK Golden Serialization Tests
Copy the SDK's `json_parity_test.go` pattern into `internal/acp/golden_test.go` to validate
that all ACP types marshal/unmarshal correctly against 33 golden fixtures shipped with the SDK.

```bash
# Run the SDK's own golden tests from within our module
go test github.com/coder/acp-go-sdk@v0.13.5 -run TestJSONGolden -count=1

# Or copy the test helper and fixtures into internal/acp/
cp $(go env GOMODCACHE)/github.com/coder/acp-go-sdk@v0.13.5/json_parity_test.go \
   internal/acp/sdk_golden_test.go
cp -r $(go env GOMODCACHE)/github.com/coder/acp-go-sdk@v0.13.5/testdata  internal/acp/
# Then adjust import paths and run:
go test ./internal/acp/... -run TestJSONGolden -count=1
```

The 33 fixtures cover: content blocks (text, image, audio, resource), tool call updates,
permission outcomes, session updates, and method payloads (initialize, new session, prompt, etc.).
Passing these proves the Crush types are structurally identical to the SDK spec.

### 3. Cross-SDK Conformance
`github.com/Tangerg/acp` is reachable at `v0.2.4` (schema `schema-v1.21.0`, Go
1.25+) and is planned as a **test-only** dependency; see I-8 and Phase D of the
remediation plan. The two external harnesses are:

**a) OpenAgentsInc conformance harness** (TypeScript, 47 scenarios):
```bash
git clone https://github.com/OpenAgentsInc/openagents
cd openagents
pnpm install
pnpm --dir packages/agent-client-protocol-conformance run test
```
Feeds stdio-based agents through a scenario catalog and reports pass/fail per case.
To test Crush server: spawn `crush acp` as the agent under test and point the harness at it.

**b) Tangerg/acp interop tests** (Go, 154 fixtures from TypeScript validators + Zed recordings):
```bash
go get github.com/Tangerg/acp@v0.2.4
go test github.com/Tangerg/acp/...  # includes golden_test.go, zed_test.go, interop_test.go
```
These replay recorded Zed 1.17.2 sessions and the reference TypeScript SDK's bytes.
Integration plan: vendor `testdata/zed/terminal-and-cancellation.json` and the
`testdata/fixtures/*.json` corpus into `internal/acp/testdata/tangerg/`, then add
`internal/acp/tangerg_conformance_test.go` covering (i) fixture replay into our
server with fixed-point re-encoding, (ii) the Zed transcript's cancel path, and
(iii) a Tangerg client driving our server over an in-memory transport. Gate behind
a build tag so CI without network still passes.

### 4. Manual End-to-End Testing
Connect Crush to a real IDE or reference client:

**Against Zed (ACP server):**
```bash
# In one terminal, start the ACP server
crush acp
```

**Against a reference agent (ACP client):**
```bash
# Connect to an external ACP agent (e.g., a TypeScript or Python ACP agent)
crush acp connect ./my-acp-agent -- --config path/to/config.json
# Or interactively:
crush acp connect claude
```

**Test scenarios to verify manually:**
1. `initialize` — should return protocol version 1, agent info `"Crush"`.
2. `session/new` — should create a SQLite session, return non-empty `sessionId`.
3. `session/prompt` — should run a prompt through the coordinator, return `StopReasonEndTurn`.
4. `session/update` — streaming chunks (text, reasoning, tool calls) should arrive in real time.
5. `session/request_permission` — permission dialog should appear; selecting allow/reject should
   propagate correctly back to the agent.
6. `session/cancel` — should abort an in-flight prompt and return `StopReasonCancelled`.
7. `fs/read_text_file` / `fs/write_text_file` — should read/write actual files on disk.
8. Disconnect — closing the IDE should cause the server to exit cleanly (no goroutine leak).

### 5. Known Test Gaps
- **Streaming prompt test**: No end-to-end test for `Prompt` with a real agent that produces
  `SessionUpdate` chunks. The server-side `TestServerStartEndToEnd` only tests `NewSession` +
  `Cancel`. Adding a mock agent that emits `SessionUpdate` notifications would close this gap.
  See I-1; this is the prime suspect for the Zed hang.
- **Cross-implementation conformance**: Not yet validated against Zed or JetBrains reference
  clients. `Tangerg/acp v0.2.4` is available (I-8) and should be added as a test-only
  dependency rather than relying on a manual IDE run.
- **Live subprocess e2e**: No test spawns the built `crush acp` binary and speaks raw
  JSON-RPC at its stdin/stdout. The in-process pipe tests cannot catch stdout pollution
  (I-4) or capability-shape problems (I-2).
- **Termination on agent crash**: When the client-side agent process crashes, `Client.Start()`
  returns an error but does not propagate it to callers of `Prompt`/`NewSession`. Consider adding
  a `Done()` channel on `Client` so callers can detect agent death.

---

## Implementation Reference (Files & Line Numbers)

### Server Side (`internal/acp/`)

| File | Purpose | Key Types/Functions |
|---|---|---|
| `server.go` | Entry point, wiring | `Server`, `NewServer()`, `dispatcher` (routes SDK -> `Agent` interface) |
| `agent.go` | `acp.Agent` impl | `crushAgent`, `Initialize()`, `NewSession()`, `Prompt()`, `Cancel()`, `mapFinishReason()` |
| `event_bridge.go` | Pushes Crush events -> SDK `SessionUpdate` | `eventBridge`, `Start()`, `subscribeNotifications()`, `subscribeRunCompletions()` |
| `permission.go` | Crush -> ACP permission translation | `permissionBridge`, `CheckPermission()`, `buildPermissionOptions()`, `mapPermissionOutcome()` |
| `permission_service.go` | Wraps `permission.Service` to route through bridge | `acpPermissionService`, `Request()` |
| `server_test.go` | Server e2e pipe test | `TestServerStartEndToEnd`, `testClient` |
| `event_bridge_test.go` | Event bridge lifecycle | `TestEventBridge_StartAndStop`, `TestEventBridge_ClientDisconnect` |
| `permission_test.go` | Permission translation logic | `TestBuildPermissionOptions`, `TestMapPermissionOutcome`, `TestIsAllowAlways`, `TestPermissionBridgeIntegration` |
| `test_helpers.go` | Mocks and helpers | `stubSessionService` (full `session.Service` mock) |

### Client Side (`internal/acp/`)

| File | Purpose | Key Types/Functions |
|---|---|---|
| `client.go` | `acp.Client` impl + subprocess driver | `Client`, `NewClient()`, `Start()`, `crushClient` (callbacks) |
| `client_test.go` | Client unit tests | `TestClientSubscribe`, `TestClientPermission*`, `TestClientReadTextFile`, `TestClientWriteTextFile`, `TestClientTerminalMethods`, `stubAgent` |

### CLI (`internal/cmd/`)

| File | Purpose | Key Functions |
|---|---|---|
| `acp.go` | `crush acp` server command | `runACP()` — boot workspace, redirect logs, start `Server` |
| `acp_client.go` | `crush acp connect` client command | `runACPConnect()` — spawn agent, interactive prompt loop |

---

## Action Plan

Based on the compliance audit of ACP specification implementation, test coverage,
and test toolkit verification, the following actions are recommended to close
remaining gaps.

### 1. Client Tool Delegation Layer

Crush tools currently always use local filesystem and terminal execution. When an
ACP client advertises `fs` and `terminal` capabilities in `initialize`, Crush
should delegate those operations back through the connection rather than executing
locally.

**Files to touch:** `internal/acp/agent.go` (Initialize response),
`internal/agent/tools/` (tool delegation hook), `internal/acp/event_bridge.go`.

**Steps:**
1. Track client capabilities from `InitializeRequest.ClientCapabilities` in
   `crushAgent` and expose them on the server.
2. In the tool permission / execution path, inspect advertised capabilities:
   - If `clientCapabilities.fs.readTextFile` / `fs.writeTextFile` → route
     `tools.View` / `tools.Edit` through `conn.ReadTextFile` / `WriteTextFile`.
   - If `clientCapabilities.terminal` → route shell tools through
     `conn.CreateTerminal` / `KillTerminal` / `WaitForTerminalExit`.
3. Fall back to local Crush tool behavior when the client lacks the capability.
4. Add unit tests covering each delegation path in `internal/acp/permission_test.go`
   and `internal/acp/conformance_test.go`.

---

### 2. Client CLI Interactive Streaming

`crush acp connect` prints only the `StopReason` after each turn. The client
already has per-session `Subscribe()` channels that receive `SessionUpdate`
notifications, but they are never consumed in the interactive loop.

**Files to touch:** `internal/cmd/acp_client.go`, `internal/acp/client.go`.

**Steps:**
1. After `cli.NewSession()`, call `cli.Subscribe(sessionID)` and start a goroutine
   that drains the channel.
2. Render each `agent_message_chunk` content as it arrives (streaming text output
   to stdout without newline so progress is visible in real time).
3. Render `tool_call` updates (start/end) as status lines.
4. Keep the existing `StopReason` print as a final summary after the prompt returns.
5. Add a test in `test/acp/e2e_test.go` (tagged `e2e`) that runs
   `crush acp connect` against a mock agent and asserts streaming output appears.

---

### 3. External Conformance Automation

The OpenAgentsInc TypeScript conformance harness (`packages/
agent-client-protocol-conformance`, 47 scenarios) is the authoritative protocol
validator but is not wired into the existing `test/acp/run.sh` workflow.

**Files to touch:** `test/acp/run.sh`, `test/acp/README.md`.

**Steps:**
1. Add a `run_conformance()` function to `test/acp/run.sh` that:
   - Checks for `CRUSH_BIN` env var (builds if missing via `run_build`).
   - Clones `https://github.com/OpenAgentsInc/openagents` into a temp directory
     (or reuses cached clone via `$HOME/.cache/openagents`).
   - Runs `pnpm --dir packages/agent-client-protocol-conformance run test` with
     `CRUSH_BIN` set.
   - Returns non-zero on any failed scenario.
2. Add `conformance` as a named target in the `case` statement.
3. Document the requirement (Node.js / pnpm installed) in
   `test/acp/README.md` under a new "Prerequisites" section.
4. Gate this target behind a `CI_CONFORMANCE=false` check so CI passes without
   it; the target is informational rather than blocking.

---

### 4. Session Resume Support Assessment

`session/resume` currently returns a hard error. Two options exist:
- Wire it to resume an existing session by ID (load stored messages, replay
  context into coordinator).
- Return a proper `AgentCapabilities` declaration that `Resume: false` so
  clients know not to call it.

**Files to touch:** `internal/acp/agent.go`, `docs/ACP_PLAN.md`.

**Steps:**
1. Decide on the implementation path (wire vs. capability rejection).
2. If wiring: load session messages from `session.Service`, reconstruct
   prompt history, pass into coordinator `Run()` with resumed context.
3. If rejecting: add `Resume: &acp.SessionResumeCapabilities{}` to
   `AgentCapabilities` and return `nil, nil` (acknowledge) instead of error
   to avoid breaking clients that send the method unconditionally.
4. Update checkpoint table accordingly.

---

## Updated Compliance Summary

| Area | Status | Notes |
|---|---|---|
| ACP Server spec core methods | ✅ Implemented | `initialize`, `new_session`, `prompt`, `cancel`, `list_sessions`, `close_session` |
| ACP Client spec methods | ✅ Implemented | `Initialize`, `NewSession`, `Prompt`, `Cancel`, `ReadTextFile`, `WriteTextFile`, terminal stubs |
| Permission bridge | ✅ Implemented | Options mapping, persistent allow-always cache |
| Event streaming (server-side) | ✅ Wired | `subscribeMessages()` bridges agent text deltas to `SessionUpdate` |
| Session configuration methods | ✅ Fixed | `SetSessionMode`, `SetSessionConfigOption` no longer error |
| Cross-SDK interop (Tangerg/acp) | ✅ Verified | `TestReferenceClientHandshake`, `TestZedTranscript*` all pass |
| SDK regression | ✅ Verified | `go test github.com/coder/acp-go-sdk` passes 47 tests |
| Subprocess e2e | ✅ Verified | `test/acp/e2e_test.go` passes against built binary |
| OpenAgents conformance harness | ⚠️ Not automated | Manual run possible; not wired into `run.sh` |
| Client tool delegation | ❌ Not implemented | Tools still use local Crush paths exclusively |
| Client CLI streaming display | ❌ Partial | Only `StopReason` printed; live chunks not rendered |
| Session resume support | ❌ Not implemented | Returns error |


---

## Protocol v1 Compliance Audit (Official Spec — agentclientprotocol.com)

### Session Update Types Coverage (11 in spec)

| Type | Status | Notes |
|---|---|---|
| `user_message_chunk` | ❌ Not implemented | Client echoes user input; we never emit this |
| `agent_message_chunk` | ✅ Implemented | `subscribeMessages()` in event_bridge.go |
| `agent_thought_chunk` | ❌ Not implemented | Reasoning deltas not bridged |
| `tool_call` | ❌ Not implemented | Tool start events not emitted |
| `tool_call_update` | ❌ Not implemented | Tool in-progress/completed/failed not emitted |
| `plan` | ❌ Not implemented | Execution strategy updates not supported |
| `available_commands_update` | ❌ Not implemented | Slash command advertising missing |
| `current_mode_update` | ❌ Not implemented | Mode change notifications not sent |
| `config_option_update` | ❌ Not implemented | Config state change notifications not sent |
| `session_info_update` | ❌ Not implemented | Session metadata updates not supported |
| `usage_update` | ❌ Not implemented | Token usage reporting not emitted |

### Content Block Types Coverage (5 in spec)

| Type | Status | Notes |
|---|---|---|
| `text` | ✅ Implemented | Baseline support |
| `resource_link` | ✅ Implemented | Referenced in extractPromptText |
| `image` | ⚠️ Capability advertised | Prompt accepts but event bridge does not stream image content blocks |
| `audio` | ⚠️ Capability advertised | Prompt accepts but event bridge does not stream audio content |
| `resource` (embedded) | ⚠️ Partial | URI/mimeType/byte data not fully handled in prompt extraction |

### Stop Reason Coverage (5 in spec)

| Reason | Status | Notes |
|---|---|---|
| `end_turn` | ✅ | Maps fantasy.FinishReasonStop |
| `max_tokens` | ✅ | Maps fantasy.FinishReasonLength |
| `max_turn_requests` | ❌ Not mapped | Fantasy may not expose this reason |
| `refusal` | ✅ | Maps fantasy.FinishReasonContentFilter |
| `cancelled` | ✅ | Returns on context cancellation |

### Agent Methods Coverage

| Method | Status | Notes |
|---|---|---|
| `initialize` | ✅ | Full capability response |
| `authenticate` | ⚠️ Stub | Returns "auth not supported"; no authMethods advertised |
| `session/new` | ✅ | Creates session, returns sessionId + optional configOptions |
| `session/load` | ❌ Stub | Returns error; `loadSession` advertised but no implementation |
| `session/prompt` | ✅ | Runs coordinator, maps finish reasons |
| `session/cancel` | ✅ | Cancels active prompt |
| `session/list` | ⚠️ Partial | `list` capability not advertised; method exists |
| `session/close` | ✅ | Cancels session coordinator work |
| `session/resume` | ❌ Stub | Returns error; `resume` capability not advertised |
| `session/set_mode` | ✅ Acknowledges | No-op success; mode list not returned in session/new |
| `session/delete` | ❌ Not implemented | `delete` capability not advertised |
| `logout` | ❌ Stub | Returns "logout not supported" |
| `elicitation/create` | ❌ Not implemented | Form/url modes not supported |

### Client Methods Coverage

| Method | Status | Notes |
|---|---|---|
| `session/request_permission` | ✅ | Bridge implemented with all 4 option kinds |
| `fs/read_text_file` | ✅ | Implemented with line/limit support |
| `fs/write_text_file` | ✅ | Implements mkdir parent |
| `terminal/create` | ⚠️ Stub | Returns "not supported" |
| `terminal/output` | ⚠️ Stub | Returns "not supported" |
| `terminal/release` | ⚠️ Stub | Returns "not supported" |
| `terminal/wait_for_exit` | ⚠️ Stub | Returns "not supported" |
| `terminal/kill` | ⚠️ Stub | Returns "not supported" |
| `elicitation/create` | ❌ Not implemented | Client callback missing |
| `elicitation/complete` | ❌ Not implemented | Notification handler missing |

### Capability Advertising Gaps

| Capability | Current | Required for Full Compliance |
|---|---|---|
| `agentCapabilities.loadSession` | ✅ true | Already set |
| `agentCapabilities.promptCapabilities` | ✅ | Image/audio/embeddedContext set |
| `agentCapabilities.mcpCapabilities` | ✅ | Set to empty (implies no transports) |
| `agentCapabilities.sessionCapabilities.close` | ✅ | Set |
| `agentCapabilities.sessionCapabilities.list` | ❌ Missing | Should advertise if ListSessions implemented |
| `agentCapabilities.sessionCapabilities.resume` | ❌ Missing | Should advertise if resume supported |
| `agentCapabilities.sessionCapabilities.delete` | ❌ Missing | Delete method not implemented |
| `agentCapabilities.sessionCapabilities.additionalDirectories` | ❌ Missing | Not supported |
| `agentCapabilities.auth.logout` | ❌ Missing | Logout stub exists but not advertised |
| Client `auth.terminal` | ⚠️ Not checked | Client can advertise; we ignore |
| Client `session.configOptions.boolean` | ⚠️ Not checked | Boolean config options not advertised back |

### Extensibility & Conventions Gaps

| Feature | Status |
|---|---|
| `_meta` field passthrough | ❌ Not forwarded through any message path |
| Custom methods (`_` prefix) | ⚠️ SDK returns -32601 automatically |
| `elicitation` form/url modes | ❌ Not implemented on either side |
| JSON-RPC batch messages | ⚠️ Not tested (v2 feature, v1 spec says 501) |
| Transport beyond stdio | ❌ HTTP/WebSocket not supported (out of scope for v1) |

### Tool Call Details Gap

The event bridge emits `agent_message_chunk` but never emits:
- `tool_call` (pending start of tool execution)
- `tool_call_update` with status `in_progress`, `completed`, or `failed`
- `toolCallLocation` / file locations
- `rawInput` / `rawOutput` fields

This means editors cannot show live tool execution progress, which is a visible UX gap compared to the spec.


---

## Prioritized Compliance Action Plan

Based on the official v1 spec audit, the following gaps should be addressed in priority order:

### P0 — Protocol Correctness (client-visible hangs / errors)

**1. Emit `tool_call` and `tool_call_update` from event bridge**
   - Editors show tool execution progress via these notifications. Without them, the agent appears frozen during file edits, searches, and terminal commands.
   - Files: `internal/acp/event_bridge.go`, `internal/agent/notify/`
   - Hook into `agent.OnToolCallStart` and `agent.OnToolCallEnd` if available; otherwise instrument the coordinator's prompt result to emit per-tool events.
   - Add `TestPromptStreamsToolCalls` regression test.

**2. Add `max_turn_requests` to stop reason mapping**
   - File: `internal/acp/agent.go` (`mapFinishReason`)
   - If fantasy exposes a turn-limit finish reason, map it; otherwise acknowledge the gap in comments.

**3. Advertise all implemented session capabilities**
   - `sessionCapabilities.list = &SessionListCapabilities{}` if ListSessions works reliably.
   - `sessionCapabilities.resume` only if we wire `session/resume`; otherwise omit.
   - `sessionCapabilities.delete` only if delete is implemented; otherwise omit.
   - File: `internal/acp/agent.go` Initialize response.

**4. Echo `user_message_chunk` on prompt**
   - When the client sends `session/prompt`, the agent SHOULD reflect the user message back as a `user_message_chunk` update before processing. This lets editors show the user's input in the session history.
   - File: `internal/acp/agent.go` Prompt method.

---

### P1 — Spec Completeness (full feature parity)

**5. Wire `session/load` properly**
   - Load stored messages from `session.Service.Get`, replay them as `user_message_chunk` + `agent_message_chunk` updates, then respond.
   - File: `internal/acp/agent.go` ResumeSession → rename to implement LoadSession logic.
   - Add `TestConformanceSessionLoad` to conformance suite.

**6. Implement `elicitation/create` and `elicitation/complete` on client side**
   - Client callbacks: `RequestElicitation` on `crushClient` in `internal/acp/client.go`.
   - For Crush ACP server mode: return "not supported" error until needed.
   - Form mode: parse `requestedSchema` JSON Schema, display to user, collect response.
   - URL mode: open URL in browser, wait for `elicitation/complete` notification.
   - File: `internal/acp/client.go` (add method), `internal/acp/server.go` (add stub).

**7. Add `_meta` passthrough on all message paths**
   - Propagate `_meta` from incoming requests through to prompt text and session updates where relevant.
   - File: `internal/acp/agent.go` (Initialize, NewSession, Prompt), `internal/acp/event_bridge.go`.

**8. Emit `available_commands_update` after session creation**
   - Send slash commands the agent supports (e.g., `/plan`, `/test`, `/web`).
   - File: `internal/acp/agent.go` NewSession or session init hook.

**9. Emit `config_option_update` when config changes**
   - When `SetSessionConfigOption` is called, reply with the full current config list AND send a `config_option_update` notification.
   - File: `internal/acp/agent.go` SetSessionConfigOption.

**10. Handle `authMethods` and `authenticate` properly**
   - If no auth is needed, advertise empty `authMethods: []` and handle `authenticate` as no-op returning success.
   - If auth is required (provider not configured), advertise `authMethods` with a terminal-type method and return `auth_required` error on session operations until authenticated.
   - File: `internal/acp/agent.go` Initialize, Authenticate methods.

---

### P2 — UX & Observability

**11. Stream `agent_thought_chunk` for reasoning deltas**
   - If Crush's fantasy pipeline emits reasoning/reasoning-text events, bridge them to `agent_thought_chunk`.
   - File: `internal/acp/event_bridge.go` (add subscription to reasoning stream).

**12. Emit `usage_update` with token counts**
   - Map fantasy `Usage` fields to `used`/`size`/`cost` in session updates.
   - File: `internal/acp/event_bridge.go`.

**13. Support `plan` session updates**
   - If the coordinator can produce an execution plan, emit `plan` updates with entries, priorities, and statuses.
   - File: `internal/acp/event_bridge.go`.

**14. Implement `current_mode_update` notification**
   - After `SetSessionMode`, emit a `current_mode_update` with the new `modeId`.
   - File: `internal/acp/agent.go` SetSessionMode.

**15. Return `modes` and `configOptions` in `session/new` response**
   - The spec says agents MAY return the initial mode list and config options in the NewSession response.
   - File: `internal/acp/agent.go` NewSession response construction.

---

### P3 — Client-Side Gaps

**16. Terminal client callbacks (stub → real)**
   - Replace "not supported" stubs in `internal/acp/client.go` with real subprocess terminals using `os/exec` + pty if available, or keep stubs with explicit error messages and document as out-of-scope.
   - Decision point: only needed if ACP clients should drive Crush as an external agent.

**17. Consume subscription channels in `crush acp connect` CLI**
   - Wire `cli.Subscribe()` to render `agent_message_chunk`, `tool_call`, and `tool_call_update` in the interactive prompt loop.
   - File: `internal/cmd/acp_client.go`.

---

### P4 — Test Coverage Gaps

**18. Add tests for each missing SessionUpdate type**
   - `TestPromptStreamsToolCalls` — verifies `tool_call` + `tool_call_update` emission.
   - `TestPromptStreamsThoughtChunk` — verifies reasoning delta streaming.
   - `TestPromptStreamsUserMessageChunk` — verifies user message echo-back.
   - `TestPromptStreamsPlan` — verifies plan update emission.
   - `TestPromptStreamsUsageUpdate` — verifies usage reporting.

**19. Add conformance tests for session/load**
   - `TestConformanceSessionLoad` — verify session replay sends updates before response.

**20. Add elicitation roundtrip test**
   - `TestElicitationFormMode` — client receives elicitation request and responds.

**21. Add `_meta` passthrough test**
   - `TestMetaPassthrough` — verify `_meta` from initialize request is preserved.

---

### Implementation Priority Summary

| Priority | Count | Effort | Impact |
|---|---|---|---|
| P0 — Protocol correctness | 4 items | Medium | Prevents client-side hangs / blank tool progress |
| P1 — Spec completeness | 6 items | Large | Full v1 feature parity |
| P2 — UX & observability | 5 items | Medium | Richer editor experience |
| P3 — Client-side gaps | 2 items | Small-Medium | Better external agent support |
| P4 — Test coverage | 6 tests | Small | Regression protection |

Estimated total: ~24 items across 4 priority tiers.
