Check on subagents started with `agent_dispatch`.

With no `label`, it lists every subagent in this session: state (running, finished, failed, stopped, or interrupted by a restart), how long it has run, how many tool calls it has made, and its latest call.

With a `label`, it shows that subagent in detail: its task, recent tool calls, latest output, and last report.

Use it to answer a question about a subagent: whether it is stuck, what it is doing, why it stopped. It is not a way to wait for one. Reports arrive on their own and wake you, so once you have nothing else to do, end your turn rather than checking again.
