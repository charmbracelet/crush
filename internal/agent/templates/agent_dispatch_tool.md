Launch a sub-agent that runs detached and reports back on its own.

This tool returns the moment the sub-agent starts. Its result arrives later as a `<sub-agent-report>` message in the conversation, labeled with the name you gave it. Reports arrive one at a time, as each sub-agent finishes, so you read and act on the first result without waiting for the rest.

Use this when you want work happening alongside your own:
- Fan out a review, a search, or an investigation across several scopes at once, then handle each report as it lands.
- Hand off reading that would otherwise fill your context. The sub-agent burns its own context and reports back only its conclusion, which extends how long this session runs before it needs compaction.

Give every sub-agent a short `label`. The label names it in `agent_send` and titles its report, so make it descriptive of the scope: `auth-review`, `race-conditions`, `db-layer`. Two sub-agents in the same session cannot share a label.

Set `access` to `read` (the default) for a sub-agent that can inspect and run things but not edit files, or `write` for one that can carry out a change. Parallel `write` sub-agents must be given scopes that do not overlap, or they will clobber each other's edits.

Write the prompt as a standalone brief. Sub-agents cannot see your conversation, each other, or anything you did not put in the prompt. Say what to examine, what to do, and what shape the answer should take.

Do not sit idle waiting for reports. If you have nothing else to do, say what you dispatched and end your turn; the reports will wake you when they arrive.

A sub-agent stays answerable after it reports. Use `agent_send` with its label to ask a follow-up, and it replies with its earlier work still in context.

For a quick lookup whose answer you need before you can do anything else, use the `agent` tool instead, which blocks and hands the answer straight back.
