Send a message to a subagent started with `agent_dispatch`.

Use it to narrow a scope, pass along something you learned, or ask a follow-up about a report. The subagent keeps its session, so it answers with its earlier work in context.

If the subagent is still running, the message reaches it at its next step and the answer folds into its report. If it has finished, it picks the message up as a follow-up and sends a new report. This call returns only a delivery confirmation; the answer arrives later as a `<subagent-report>` under the same name.

`model` moves the subagent to a different model before it gets the message, with the same `provider` and `reasoning` options and the same model list as `agent_dispatch`. Use it when a subagent failed because its model refused or could not do the task: it keeps its whole session, so it picks up where it stopped. Only a subagent that is not running can be moved; stop a running one with `agent_stop` first. The new model sticks for later follow-ups.
