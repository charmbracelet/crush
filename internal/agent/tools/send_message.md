Sends a control message to a running task graph, or queues a follow-up prompt
for a background agent.

Use this tool when you need to notify another task (or all tasks) during agent
delegation, or when you want a background agent to continue working after it
finishes its current run. If no mailbox_id is provided, the current task graph
mailbox from context is used automatically.

Parameters:
- agent_id (optional): Background agent ID or name to continue with a follow-up prompt. You can use either the agent ID (e.g., "a-abc123") or a human-readable name (e.g., "researcher-xyz").
- mailbox_id (optional): Mailbox identifier to deliver messages to. If omitted, defaults to the current task graph mailbox from context.
- message (required): Message content to enqueue.
- task_id (optional): Target task ID. Omit to broadcast to all tasks in the mailbox.

Targeting rules and precedence:
- If `agent_id` is provided, the message targets that background agent and `mailbox_id`/`task_id` are ignored.
- If `agent_id` is omitted, `mailbox_id` + optional `task_id` target task-graph mailbox delivery.
- If only `mailbox_id` is provided, message is broadcast in that mailbox.
- If both `mailbox_id` and `task_id` are provided, message is sent only to that task.
- If neither `agent_id` nor `mailbox_id` is provided, the current delegation mailbox from context is used.
- If `task_id` is provided while both `agent_id` and `mailbox_id` are omitted, the message targets that task in the context default mailbox.

Examples:
- Background agent by name: `{ "agent_id": "researcher", "message": "continue investigation" }`
- Background agent by id: `{ "agent_id": "a-abc123", "message": "run extra tests" }`
- Mailbox broadcast: `{ "mailbox_id": "mb-1", "message": "sync status" }`
- Mailbox targeted task: `{ "mailbox_id": "mb-1", "task_id": "task-a", "message": "retry with logs" }`

Notes:
- Agents can be addressed by name or ID. Name-based addressing makes it easier to continue specific agents without tracking random IDs.
- Prefer `agent_id` for resumable background agents and `mailbox_id` for task graph coordination.
- Messages are delivered best-effort while the mailbox is active.
- Unknown mailbox, agent, or task IDs return an error response.
- If an agent is still running, the message is queued and delivered on its next turn.
- Attempts to send `agent_id` messages to stopped or non-resumable agents return an error. Use the `subtask_result` tool to fetch final output.
