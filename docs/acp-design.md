# Crush ACP Integration

`crush acp` exposes Crush as an Agent Client Protocol (ACP) agent. An ACP
client can create or load a Crush session, send prompts, receive streaming text
and tool updates, and service agent-to-client requests for file reads, file
writes, terminals, and permission decisions.

This implementation was exercised while prototyping a Go-based TUI and coding
agent control surface. Crush already had the useful pieces for that trial:
terminal-first interaction, persisted sessions, model/tool orchestration, and a
Bubble Tea UI. The ACP work keeps those pieces intact and adds a protocol bridge
around the existing coordinator.

## Launch Modes

| Command | Transport | UI | Purpose |
| --- | --- | --- | --- |
| `crush acp` | stdio | no TUI | ACP client launches Crush as a subprocess |
| `crush acp --tui` | none | TUI | Standalone terminal session |
| `crush acp --tui-acp` | Unix socket | TUI | TUI plus an external ACP client |
| `crush --tui` | Unix socket | TUI | Shortcut for TUI plus ACP socket mode |

The socket mode keeps ACP traffic off stdin/stdout so Bubble Tea can own the
terminal cleanly.

## Agent To Client Flow

Crush streams runtime activity through a `RunObserver` interface. The ACP
adapter implements that observer and forwards events as `session/update`
notifications:

| Crush event | ACP update |
| --- | --- |
| text delta | agent message chunk |
| reasoning delta | agent thought chunk |
| tool input begins | tool call start |
| tool execution begins | tool call in progress |
| tool execution completes | tool call completed |
| plan changes | plan update |
| title changes | session info update |

This avoids database polling and preserves live tool progress for clients that
render tool cards or progress indicators.

## Client Backed Tools

When `options.acp.zed_control` is enabled, Crush registers ACP-backed variants
of core workspace tools:

| Tool | ACP request |
| --- | --- |
| `zed_view` | `ReadTextFile` |
| `zed_write` | `WriteTextFile` |
| `zed_bash` | `CreateTerminal`, `WaitForTerminalExit`, `TerminalOutput` |

The names are intentionally compatibility-oriented because current clients
already recognize the Zed visual command metadata described below. The tool
descriptions tell the model to prefer these tools when an ACP client is
connected, but native tools remain available.

## Visual Command Metadata

Some ACP clients can perform workspace UI actions from tool-call metadata. This
branch emits `_zed_visual_command` metadata from optional tools such as
`zed_visual`, `zed_pane`, `zed_panel`, and `zed_batch`.

That metadata is treated as an extension: clients that recognize it can open
files, move panes, or focus panels; clients that do not recognize it can still
display the tool calls normally.

## Configuration

ACP-backed workspace tools are off by default. Enable them with:

```json
{
  "options": {
    "acp": {
      "zed_control": true
    }
  }
}
```

## Implementation Notes

- `internal/cmd/acp.go` owns launch modes and socket setup.
- `internal/acp/adapter.go` implements the ACP agent adapter.
- `internal/agent/agent.go` defines the observer callbacks emitted by the agent
  runtime.
- `internal/agent/coordinator.go` wires the adapter into sessions, tools, title
  updates, plan updates, and config reloads.
- `internal/agent/tools/acp.go` contains the ACP-backed tool definitions.
- `internal/config/config.go` adds the optional ACP configuration.

## Validation

Targeted validation for this branch:

```sh
go test ./internal/acp ./internal/cmd ./internal/agent ./internal/agent/tools
go build ./...
```

At the time this was prepared, `go test ./...` also exercised the branch but
failed in existing config-store tests that require provider model data not
available in the local test environment.
