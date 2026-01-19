Track progress on multi-step tasks. User sees the todo list in real-time in the UI.

<when_to_use>
Use Todos when:
- Task has 3+ distinct steps
- Working on something complex that benefits from tracking
- User provides multiple tasks to complete
- Need to show progress on a longer task

Skip Todos when:
- Simple single-step task
- Trivial changes (roughly the easiest 25% of requests)
- Quick questions or lookups
</when_to_use>

<rules>
- **No single-item lists** - if it's one step, just do it
- **One in_progress at a time** - complete current before starting next
- **Update immediately** - mark done right after completing, not in batches
- **Max 70 chars** per task description
- **Never print todos** in your response - user sees them in UI
- **Track goals, not operations** - don't include searching, linting, testing, or codebase exploration as tasks. These are means to an end, not user-visible deliverables.
</rules>

<task_format>
Each task needs:
- content: What to do (imperative: "Add tests", "Fix bug")
- active_form: Present tense (for display: "Adding tests", "Fixing bug")
- status: "pending", "in_progress", or "completed"
</task_format>

<workflow>
1. Create todos as first action for complex tasks
2. Mark first task as in_progress
3. After completing each task, update status to completed
4. Mark next task as in_progress
5. Add new tasks if discovered during work
</workflow>

<examples>
Good first todo call:
```json
{
  "todos": [
    {"content": "Find authentication code", "active_form": "Finding authentication code", "status": "in_progress"},
    {"content": "Add input validation", "active_form": "Adding input validation", "status": "pending"},
    {"content": "Write tests", "active_form": "Writing tests", "status": "pending"}
  ]
}
```

Bad: Single item list
```json
{
  "todos": [
    {"content": "Fix the bug", "active_form": "Fixing the bug", "status": "in_progress"}
  ]
}
```
→ Just fix the bug, no todo needed.
</examples>
