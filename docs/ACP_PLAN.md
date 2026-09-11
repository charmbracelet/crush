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
  - `spachava753/acp-sdk` & `Tangerg/acp` — require Go 1.25+, unmaintained.

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
| **CLI Integration** | **Local** (`internal/cmd`) | `--acp` flag, headless workspace boot, redirect logs to stderr. |
| **Client Track** (optional) | **Local** (`internal/acp/client`) | Implements `acp.Client` callbacks to drive external agents via SDK. |

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
       +---> PubSub Event Bridge (internal/acp/bridge.go)
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
```

---

## Checkpoint Legend

- [ ] = not started
- [x] = complete

---

## ===================================
## ACP SERVER TRACK
## ===================================

### Phase 1: SDK Integration & Transport Setup
> Integrate `github.com/coder/acp-go-sdk`, deprecating manual schema definitions.

1. [ ] Add `github.com/coder/acp-go-sdk` to `go.mod`.
2. [ ] Create `internal/acp/server.go` with minimal `acp.Agent` boilerplate.
3. [ ] Verify stdio loop and handshake via in-memory pipe smoke test.

### Phase 2: Server Engine & Session Lifecycle (`internal/acp`)
1. [ ] Implement `acp.Agent` methods on `internal/acp.Server`:
   - [ ] `Initialize`: return Crush agent info (`name: "Crush"`, current version), supported capabilities (`loadSession: false`, prompt/MCP/session capabilities).
   - [ ] `NewSession`: create session via `session.Service.Create`, scope working directory (`cwd`), store active session mapping.
   - [ ] `Prompt`: invoke `coordinator.Run` or `SessionAgent.Run`. Await turn completion, return `StopReasonEndTurn`, `StopReasonCancelled`, etc.
   - [ ] `Cancel`: cancel active prompt context for the given `sessionId`.
   - [ ] `ListSessions`: delegate to `session.Service.List`.
   - [ ] `ResumeSession`: restore session state without replaying message history.
   - [ ] `CloseSession`: cancel running prompt if any, free session resources.
   - [ ] `SetSessionMode`: map mode ("coder", "task") to Crush coordinator mode.
2. [ ] Event Bridge:
   - [ ] Wire pubsub / callbacks to `conn.SessionUpdate`:
     - Text deltas -> `acp.UpdateAgentMessageText`.
     - Reasoning deltas -> `acp.UpdateAgentThoughtText`.
     - Tool lifecycle -> `acp.StartToolCall` / `acp.UpdateToolCall`.
     - Plan updates -> `acp.UpdatePlan`.

### Phase 3: Permissions & Tool Delegation
1. [ ] Permission Bridge:
   - [ ] Intercept Crush permission prompts in ACP mode.
   - [ ] Issue `conn.RequestPermission` with `acp.ToolCallUpdate` and permission options (`allow_once`, `allow_always`, `reject_once`).
   - [ ] Map client `RequestPermissionOutcome` back to Crush `permission.Decision`.
2. [ ] Client Tools Delegation (optional enhancement):
   - [ ] Inspect `clientCapabilities.FS` and `clientCapabilities.Terminal`.
   - [ ] If available, delegate `tools.Edit`/`tools.View` to `conn.ReadTextFile`/`conn.WriteTextFile`.
   - [ ] Fall back to local Crush bash/filesystem tools if client lacks capabilities.

### Phase 4: CLI Integration (`internal/cmd`)
1. [ ] Add `--acp` persistent flag to root command (`internal/cmd/root.go`).
2. [ ] Add `crush acp` subcommand (alias/dedicated command).
3. [ ] If `--acp` active:
   - [ ] Suppress Bubble Tea TUI.
   - [ ] Redirect all logger output (slog / charm.land/log) to stderr or file (stdout strictly reserved for JSON-RPC).
   - [ ] Boot workspace services headlessly.
   - [ ] Start `acp.NewAgentSideConnection` on `os.Stdin`/`os.Stdout` and block until client disconnects.

### Phase 5: Server Tests & Conformance
1. [ ] Unit test: `Server` initialization and capability negotiation.
2. [ ] Integration test: pipe-based roundtrip (`NewClientSideConnection` <-> `NewAgentSideConnection`) testing `initialize` -> `session/new` -> `session/prompt` -> `session/update` streaming -> response.
3. [ ] Test session cancellation flow.
4. [ ] Test permission request/response roundtrip.

---

## ===================================
## ACP CLIENT TRACK (Optional Follow-On)
## ===================================

### Phase 6: ACP Client (`internal/acp/client`)
1. [ ] Implement `acp.Client` handlers using SDK:
   - [ ] `SessionUpdate`: forward stream deltas to Crush UI/events.
   - [ ] `RequestPermission`: prompt user in Crush TUI for permission decision.
   - [ ] `ReadTextFile` / `WriteTextFile`: read/write local filesystem.
   - [ ] `CreateTerminal` / `TerminalOutput` / etc.: execute commands via `internal/shell`.
2. [ ] Spawn external ACP agent subprocess, bind via `acp.NewClientSideConnection`.
3. [ ] Implement Crush agent provider interface wrapping the ACP client connection.

### Phase 7: Client Tests
1. [ ] In-memory pipe test driving mock ACP agent.
2. [ ] E2E test launching an external ACP agent process.

---

## Checkpoint Progress Table

| Track / Phase | Checkpoint | Status |
|---|---|---|
| Server / 1 | SDK integration & transport setup | - [x] |
| Server / 2 | Server engine & session lifecycle | - [x] |
| Server / 3 | Permissions & tool delegation | - [ ] |
| Server / 4 | CLI integration (`--acp`) | - [x] |
| Server / 5 | Server tests & conformance | - [ ] |
| Client / 6 | ACP client implementation | - [ ] |
| Client / 7 | Client tests | - [ ] |
