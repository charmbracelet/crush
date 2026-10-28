Send a message to a subagent started with `agent_dispatch`.

Use it to narrow a scope, pass along something you learned, or ask a follow-up about a report. The subagent keeps its session, so it answers with its earlier work in context.

If the subagent is still running, the message reaches it at its next step and the answer folds into its report. If it has finished, it picks the message up as a follow-up and sends a new report. This call returns only a delivery confirmation; the answer arrives later as a `<subagent-report>` under the same label.
