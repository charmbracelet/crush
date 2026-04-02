Sends a mailbox message to a running task graph.

Use this tool when you need to notify another task (or all tasks) during agent delegation. If no mailbox_id is provided, the current task graph mailbox from context is used automatically.

Parameters:
- mailbox_id (optional): Mailbox identifier to deliver messages to. If omitted, defaults to the current task graph mailbox from context.
- message (required): Message content to enqueue.
- task_id (optional): Target task ID. Omit to broadcast to all tasks in the mailbox.

Notes:
- Messages are delivered best-effort while the mailbox is active.
- Unknown mailbox or task IDs return an error response.
