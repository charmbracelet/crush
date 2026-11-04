Stop a running background agent started with `agent` and `background: true`.

Use it when an agent is stuck, off task, or no longer needed. Its session is kept, so `agent_send` can reopen it later. To redirect an agent instead of stopping it, use `agent_send`.
