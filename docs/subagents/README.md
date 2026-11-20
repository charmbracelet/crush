# Subagents

> [!NOTE]
> This document was designed for both humans and agents.

Subagents let the main agent delegate work to child agents that run in their
own context windows and hand back a single report. This keeps the main
conversation small and lets independent work run in parallel.

## What a subagent is

A subagent is a child agent with its own context window. It receives only the
prompt you send it, plus Crush's task system prompt: it cannot see your
conversation, other subagents, or your full global rules. When it finishes, it
returns a report.

## Tools

| Tool            | Description |
| --------------- | ----------- |
| `agent`         | Synchronous, blocking variant. Returns the subagent's answer within the current turn. |
| `agent_dispatch` | Starts a subagent and returns immediately with a generated name (`<first-name>-<label>`, for example `betty-date-impl`), not the result. |
| `agent_send`    | Sends a message to an existing subagent, keeping its session. Use it to narrow scope, correct course, or ask for follow-up. Can also move the subagent to a different model when it is not running. |
| `agent_status`  | Inspects a subagent's state, tool calls, and last report. It is not a way to wait for a result. |
| `agent_stop`    | Cancels a running subagent. It emits no report; its session is kept so you can resume it later. |

`agent_dispatch` accepts these parameters: `prompt`, `label`, `access`,
`model`, `provider`, and `reasoning`.

## Access: read vs write

- `read` is the default.
- `read` uses the task agent's tools: `bash`, `glob`, `grep`, `ls`, `view`,
  `sourcegraph`, `lsp_symbols`, `lsp_definition`, and `lsp_call_hierarchy`.
- `write` uses the same task agent, given the coder's tool set (`edit`,
  `multiedit`, `write`, and so on).
- `read` removes `edit`/`write` but keeps `bash`, so it is not a hard sandbox.
- In plan mode, subagents are read-only.

## Choosing a model

- By default a subagent runs on the session's large model.
- `model`, `provider`, and `reasoning` override it per dispatch.
- The override is remembered in that subagent's follow-ups.

## Lifecycle

1. `agent_dispatch` returns the subagent's name and the turn continues.
2. The parent keeps working on whatever does not depend on the report.
3. The report arrives as a `<subagent-report>` and wakes a new turn.
4. When everything left depends on the report, end the turn; do not poll with
   `agent_status`.
5. Use `agent_send` for follow-ups and `agent_stop` to cancel.

The dispatch is recorded in the parent's transcript. If Crush restarts with a
dispatch that never reported, it appears as interrupted and can be resumed
with `agent_send`.

## Limits

- No nesting: subagents do not receive the delegation tools.
- Subagents do not inherit MCP or LSP servers; they only get the LSP lookups.
- Hooks do not fire inside a subagent; they do fire on the outer call.
- There is no documented concurrency limit.
- Cost scales: every subagent makes its own model calls.

## Examples

Parallel read-only exploration:

```text
Search for where authentication tokens are generated and validated, and report
the files and functions involved.
```

Read-only review of a diff:

```text
Review the current uncommitted changes for correctness and obvious bugs. Report
each finding with the file path and what is wrong.
```

Write implementation with disjoint scopes:

```text
Implement the `User` model in `pkg/models/user.go` and its constructor. Do not
touch any other file, and report what you changed.
```

## Customizing the subagent prompt

`option prompt task <path>` replaces the task agent's system prompt (the worker
behind every subagent). `coder` and `plan` prompts also exist. The replacement
applies to every dispatch.

## Permissions

`permissions allow agent agent_dispatch agent_send` avoids permission prompts
for the delegation tools. `permissions deny` hides tools. `crush run` propagates
the parent's approval to its subagents.
