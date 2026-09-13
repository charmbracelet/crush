Stop a running sub-agent started with `agent_dispatch`.

Use it when a sub-agent is stuck, off task, or no longer needed. A stopped sub-agent sends no report. Its session is kept, so `agent_send` can resume it later with its work in context.

To redirect a sub-agent rather than stop it, use `agent_send`.
