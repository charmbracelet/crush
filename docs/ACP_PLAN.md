# ACP (Agent Client Protocol) Implementation Plan

## Overview
ACP standardizes editor-agent communication over JSON-RPC 2.0 (stdio default).
Crush acts as **ACP Server** (for Zed, JetBrains, Neovim) and optionally
**ACP Client** (driving external agents).

This plan is split into two parallel, independently-trackable tracks:
- [ ] **ACP Server Track** (`internal/acp/server`) — Crush as ACP server over stdio.
- [ ] **ACP Client Track** (`internal/acp/client`) — Crush as ACP client to external agents.

> The two tracks share the `internal/acp/protocol` package (Phase 1). The server
> track is the MVP and is expected to be implemented first; the client track is
> an optional follow-on that unlocks external-agent delegation.

---

## Architecture & Mapping

```
IDE / Client (Zed, JetBrains)
       |
       |  JSON-RPC 2.0 (stdio)
       v
internal/acp/server.go
       |
       +---> initialize / session/*
       |          |
       |          v
       |    internal/app (Workspace, Session Service)
       |          |
       |          v
       |    internal/agent (Coordinator / SessionAgent)
       |
       +---> Streaming Events --> session/update (Text, Reasoning, ToolCalls, Plan)
       |
       +---> Client Delegation (when negotiated):
       |          +---> session/request_permission
       |          +---> fs/read_text_file & fs/write_text_file
       |          +---> terminal/create, terminal/output, terminal/wait_for_exit
       |
       +---> ACP Client (optional, separate track)
                  internal/acp/client.go  <-- drives external agent process
```

### Protocol Mapping to Crush Primitives

| ACP Method / Direction | Crush Internal Subsystem | Notes |
|---|---|---|
| `initialize` (C -> S) | Version & Capabilities handshake | Advertise tools, streaming, modes (`coder`, `task`), prompt options. |
| `session/new` (C -> S) | `session.Service.Create` + workspace setup | Scopes session to client `cwd`. Initialises DB record. |
| `session/prompt` (C -> S) | `coordinator.Run` / `SessionAgent.Run` | Streams turn; blocks until completion or returns terminal status. |
| `session/cancel` (C -> S) | Context cancellation (`csync` / `dispatchMu`) | Cancels in-flight prompt context for `sessionId`. |
| `session/update` (S -> C) | PubSub: `agent.OnTextDelta`, `OnReasoningDelta`, tool events | Emits chunks, reasoning deltas, tool status, and plan updates. |
| `session/request_permission` (S -> C) | `internal/permission` engine | Hooks into Crush tool permission check before running command. |
| `fs/*`, `terminal/*` (S -> C) | `tools.Bash`, `tools.Edit`, `tools.View` | Delegated to editor if client capabilities permit; fallback to local tools. |

---

## Checkpoint Legend

Each checkable item below is a **checkpoint**. When implementation of that item
is complete, change `- [ ]` to `- [x]` (or mark it done in the checklist table
at the bottom of this file). Checkpoints are grouped per track and per phase.

- [ ] = not started
- [x] = complete

---

## ===================================
## ACP SERVER TRACK
## ===================================

### Phase 1: Shared Protocol Types & Transport (`internal/acp/protocol`)
> Shared with the ACP Client Track. Implemented once, consumed by both.

1. [ ] Create `internal/acp/protocol/`:
   - [ ] JSON-RPC 2.0 message envelope structs (`Request`, `Response`, `Notification`, `Error`).
   - [ ] ACP v1 schema definitions:
     - [ ] Initialization (`InitializeRequest`, `InitializeResponse`, capabilities).
     - [ ] Session types (`NewSessionRequest`, `PromptRequest`, `SessionUpdateParams`, `PromptResponse`).
     - [ ] Content blocks (`TextContent`, `ImageContent`, `ResourceContent`).
     - [ ] Permissions (`RequestPermissionRequest`, `RequestPermissionResponse`).
2. [ ] Build stdio transport loop using `bufio.Reader` and
       `json.Decoder`/`Encoder` (newline-delimited JSON or Content-Length
       framing per spec).

### Phase 2: ACP Server Engine (`internal/acp/server`)
1. [ ] Implement `Server` struct wrapping Crush `*app.App` or workspace services.
2. [ ] Dispatch handlers:
   - [ ] `initialize`: report agent info (`name: "Crush"`, `version`), supported capabilities.
   - [ ] `session/new`: create session in DB, bind working directory.
   - [ ] `session/prompt`: invoke `coordinator.Run`. Map streaming callbacks
         (`OnTextDelta`, `OnReasoningDelta`, `OnToolInputStart`) to outbound
         `session/update` notifications.
   - [ ] `session/cancel`: trigger cancellation of the active run context
         for the session.
   - [ ] `session/list`, `session/load`, `session/delete`: wire directly to
         `session.Service`.
   - [ ] `session/close`, `session/set_mode`, `session/set_config_option`:
         wire to coordinator / config.

### Phase 3: Permissions & Tool Execution
1. [ ] Wrap permission handler:
   - [ ] When tool requires approval and running in ACP server mode, issue
         `session/request_permission` to client.
   - [ ] Map response (`allow_once`, `allow_always`, `reject_once`) to Crush
         `permission.Decision`.
2. [ ] Client-delegated tools vs. local tools:
   - [ ] If client advertises `fs` / `terminal` capabilities, map tool calls
         to client-side `fs/read_text_file`, `fs/write_text_file`,
         `terminal/create`.
   - [ ] If client lacks capabilities, execute tools via Crush's internal
         runtime (`bash`, native tools).

### Phase 4: CLI Integration (`internal/cmd`)
1. [ ] Add `--acp` persistent flag to `crush` command in `internal/cmd/root.go`.
2. [ ] Add dedicated `crush acp` subcommand (alias/wrapper).
3. [ ] If --acp supplied:
   - [ ] Suppress Bubble Tea TUI startup.
   - [ ] Redirect all application logs (slog) to file or stderr (stdout
       reserved for JSON-RPC).
   - [ ] Boot workspace headlessly and start stdio ACP server.

### Phase 5: Server Tests & Conformance
1. [ ] Unit tests for JSON-RPC serialization and deserialization.
2. [ ] In-memory round-trip test harness (`testnet` / pipes) simulating
       client queries (`initialize`, `session/new`, `session/prompt`).
3. [ ] Integration test with Zed / official ACP test harness.

---

## ===================================
## ACP CLIENT TRACK (Optional)
## ===================================

### Phase 6: ACP Client (`internal/acp/client`)
1. [ ] Implement `Client` struct:
   - [ ] JSON-RPC 2.0 stdio transport (reuses `protocol` package).
   - [ ] Outbound request helpers for all client -> agent methods:
         `initialize`, `session/new`, `session/prompt`, `session/cancel`,
         `session/load`, `session/list`, `session/delete`, `session/resume`,
         `session/close`, `session/set_mode`, `session/set_config_option`.
   - [ ] Notification handlers for agent -> client messages:
         `session/update` (stream decoding), `session/request_permission`,
         `fs/read_text_file`, `fs/write_text_file`, `terminal/create`,
         `terminal/output`, `terminal/wait_for_exit`, `terminal/kill`,
         `elicitation/create`, `elicitation/complete`.
   - [ ] Responders:
         [ ] Permission responder (emit `session/request_permission` answers).
         [ ] File system responder (`fs/*` -> local filesystem reads/writes).
         [ ] Terminal responder (`terminal/*` -> `internal/shell` subprocess).
         [ ] Elicitation responder (`elicitation/create` -> prompt user).
2. [ ] Wire external ACP agent process as an LLM provider or specialized
       agent delegate in Crush (`internal/agent` / `internal/app`).
   - [ ] `Client.Run` / `Client.Stream` mirroring `SessionAgent` interface
         so it can slot into `coordinator`.

### Phase 7: Client Tests & Conformance
1. [ ] Unit tests for client JSON-RPC framing and message parsing.
2. [ ] Round-trip test harness (pipes) simulating an ACP agent server
       responding to a `session/prompt`.
3. [ ] Conformance against the official ACP test harness / external agent.

---

## Checkpoint Progress Table

| Track / Phase | Checkpoint | Status |
|---|---|---|
| Server / 1 | Protocol types & transport | - [ ] |
| Server / 2 | Server engine + dispatch | - [ ] |
| Server / 3 | Permissions & tool execution | - [ ] |
| Server / 4 | CLI integration (`--acp`) | - [ ] |
| Server / 5 | Server tests & conformance | - [ ] |
| Client / 6 | ACP client implementation | - [ ] |
| Client / 7 | Client tests & conformance | - [ ] |
