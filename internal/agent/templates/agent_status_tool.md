Check on subagents started with `agent_dispatch`.

With no `label`, it lists every subagent in this session: state (running, finished, failed, stopped, or interrupted by a restart), how long it has run, how many tool calls it has made, and its latest call.

With a `label`, it shows that subagent in detail: its task, recent tool calls, latest output, and last report.

Reports arrive on their own, so there is no need to poll.
