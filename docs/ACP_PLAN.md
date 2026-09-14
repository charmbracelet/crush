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
| Provide `agentInfo` (name/title/version) | SHOULD | ✅ |
| `authMethods` present in response | MUST | ✅ |
| `loadSession` capability | OPTIONAL | ✅ |
| `promptCapabilities` | OPTIONAL | ✅ |
| `mcpCapabilities` | OPTIONAL | ✅ |
| `sessionCapabilities` | OPTIONAL | ✅ |

### 3.2 Session lifecycle methods

| Method | Level | Crush status |
|---|---|---|
| `session/new` | MUST | ✅ |
| `session/prompt` | MUST | ✅ |
| `session/cancel` | MUST | ✅ |
| `session/update` | MUST | ✅ |
| `session/close` | OPTIONAL | ✅ |
| `session/list` | OPTIONAL | ✅ |
| `session/load` | OPTIONAL | ✅ |
| `session/resume` | OPTIONAL | ❌ |
| `session/delete` | OPTIONAL | ❌ |
| `session/set_mode` | OPTIONAL | ✅ |
| `session/set_config_option` | OPTIONAL | ✅ |

### 3.3 Prompt turn & streaming

| Update type | Level | Crush status |
|---|---|---|
| `user_message_chunk` | SHOULD | ❌ |
| `agent_message_chunk` | baseline | ✅ |
| `agent_thought_chunk` | SHOULD | ✅ |
| `tool_call` | SHOULD | ✅ |
| `tool_call_update` | SHOULD | ✅ |

## 4. Prioritized Roadmap (Tier 1-4)

- [x] **Tier 1 (R1-R6)**: R1(Session Load/Replay), R2(Session List: cwd/time), R3(AuthMethods []), R4(Cancel Denial), R5(Cancel Request SDK-native), R6(LoadSession stream).
- [x] **Tier 2 (S1, S3)**: S1(ToolCall/Update streaming), S3(AgentThought streaming).
- [ ] **Tier 3 (S4, S6, S8, S9)**: S4(Plan updates), S6(Elicitation), S8(available_commands_update), S9(session_info_update).
- [ ] **Tier 4 (S5, S7, O1-O11)**: S5(messageId), S7(_meta/trace), O1-O11 (parity).
