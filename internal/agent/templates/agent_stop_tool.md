Stop a running subagent started with `agent_dispatch`.

Use it when a subagent is stuck, off task, or no longer needed. A stopped subagent sends no report. Its session is kept, so `agent_send` can resume it later with its work in context.

To redirect a subagent rather than stop it, use `agent_send`.
