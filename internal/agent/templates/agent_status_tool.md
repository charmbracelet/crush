Check on the sub-agents you dispatched with `agent_dispatch` in this conversation.

With no `label`, it lists every sub-agent: whether it is running, finished, failed, stopped, or was interrupted when Crush exited, how long it has been at it, how many tool calls it has made, and the call it made most recently.

With a `label`, it shows that one sub-agent in detail: the task it was given, its recent tool calls, the latest text it wrote, and its last report.

Use it when you need to decide something about a sub-agent: whether it is stuck, whether it has wandered off its scope, whether to send it a correction with `agent_send`, or whether to stop it with `agent_stop`. Do not poll it in a loop while you wait. Reports arrive on their own.
