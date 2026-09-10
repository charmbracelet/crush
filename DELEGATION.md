# Delegation: cheaper subagents for delegated subtasks

This fork adds one small feature on top of upstream crush: each agent can pin
its own model, so the built-in `agent` tool spawns its read-only subagents on a
cheaper/faster model while the main agent keeps running on your top-tier model.

Patched commit: `990d878a` ("feat: let agents pin their own model for cheaper
delegation"), rebased onto every upstream release via `scripts/update-crush.sh`.

## What changed (vs upstream v0.89.0)

| File | Change |
|---|---|
| `internal/config/config.go` | `Agents` map is now JSON-serializable (`agents,omitempty`); `SetupAgents` overlays user-defined agents from `crush.json` over the built-in defaults (name, description, model, allowed tools, context paths, MCP, disabled). |
| `internal/agent/coordinator.go` | `buildAgentModels` now resolves the agent's `Model` field against the `models` config map (custom keys allowed), falling back to the global `large` slot when the key is empty or unknown. |
| `internal/agent/agentic_fetch_tool.go` | Call-site updated for the new signature; behavior unchanged (still pinned to the small model). |

There is no behavior change unless you define agents or a custom model key in
your config.

## Configuration

`~/.config/crush/crush.json`:

```json
{
  "$schema": "https://charm.land/crush.json",
  "models": {
    "task": {
      "provider": "siliconflow",
      "model": "deepseek-ai/DeepSeek-V4-Flash-0731"
    }
  },
  "agents": {
    "task": {
      "name": "Task",
      "description": "Fast and cheap subagent for delegated subtasks.",
      "model": "task"
    }
  }
}
```

- `models.task` is a custom entry in the `models` map. It can be any key; it
  does not need to be called `task`.
- `agents.task.model` references that key. Set it to `large` or `small` to use
  a built-in slot instead.
- Any `agents.<id>` entry you add is treated as an override of the built-in
  agent of the same id; unknown ids are adopted as-is. The built-in ids are
  `coder` (main editing agent) and `task` (spawned by the `agent` tool; always
  read-only: glob/grep/ls/view plus LSP inspection tools, no bash/edit/write).

## Verifying the delegation

The main agent and the subagent must use different models. Run crush with a
fresh data dir and an explicit top-tier model for the main agent:

```bash
cd /path/to/some/repo
~/.local/bin/crush --debug --data-dir /tmp/dd \
  -m siliconflow/zai-org/GLM-5.2 \
  run "Use the agent tool to find where the function computeTotal is defined."
```

Then check the log:

```bash
grep "ModelProvider called" /tmp/dd/logs/crush.log \
  | sed 's/.*"provider":"\([^"]*\)","model":"\([^"]*\)".*/\1 \2/' | sort | uniq -c
```

Expected: one or two lines for `zai-org/GLM-5.2` (main agent) and several for
`deepseek-ai/DeepSeek-V4-Flash-0731` (subagent).

## Important caveats

- **Your persisted "large model" selection wins.** `~/.local/share/crush/
  crush.json` stores model selections made in the UI and overrides `model
  large` in `crushrc`. If you last selected flash as your large model there,
  the main agent will also run on flash — the whole setup saves nothing until
  you switch the large slot back to a top-tier model (e.g. GLM-5.2 or
  DeepSeek-V4-Pro-0813) via the UI, `crush models`, or by editing that file.
- **Subagents are read-only.** They cannot edit files, so delegate search,
  summarization, and analysis subtasks only — never implementation steps.
- **Base and this branch must stay in lockstep.** Quit the running crush
  process before updating (config reloading re-runs `SetupAgents`).

## Receiving upstream updates

```bash
scripts/update-crush.sh            # rebase onto latest upstream tag
scripts/update-crush.sh v0.90.0    # rebase onto a specific tag
```

The script fetches upstream tags, rebases commit `990d878a` from the base tag
onto the new tag, runs `go build` / `go vet` / `go test`, then rebuilds
`~/.local/bin/crush`. If the rebase conflicts (likely in
`internal/config/config.go` or `internal/agent/coordinator.go`), resolve,
`git rebase --continue`, and re-run the verify/build commands it prints.

The installed binary lives at `~/.local/bin/crush`; `~/.zshrc` puts
`~/.local/bin` ahead of Homebrew's `/usr/local/bin` so the patched build wins.