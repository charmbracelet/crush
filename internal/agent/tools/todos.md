Manage a structured task list for multi-step work. Each task has
`pending`, `in_progress`, or `completed` state.

## When to use

Use for complex multi-step work (roughly 3+ distinct steps), when the
user lists multiple tasks, or when careful tracking helps. Skip for
simple single-step or purely conversational requests.

## Rules

- Keep at most one task `in_progress` at a time.
- Mark a task `completed` immediately when that step is fully done.
  Do not leave finished work as `in_progress`.
- Before ending your turn with no further work remaining, every task
  must be `completed` (or removed if no longer relevant).
- When every task is `completed`, the list is cleared automatically
  and the UI pill goes away. You may also pass an empty `todos` array
  to clear early.
- Never mark a task completed if it is still blocked, partial, or
  failing; keep it `in_progress` and add a follow-up task if needed.
- Do not print the todo list in your reply text; the UI shows it live.

## Fields

- `content`: imperative form (e.g. "Run tests")
- `active_form`: present continuous form shown while working
  (e.g. "Running tests")
- `status`: `pending` | `in_progress` | `completed`
