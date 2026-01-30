Manage a task list for tracking progress on multi-step tasks. Each task needs `content` (imperative: "Run tests"), `active_form` (continuous: "Running tests"), and `status` (pending/in_progress/completed).

<rules>
- Use for 3+ step tasks, skip for trivial work
- Keep exactly ONE task in_progress at a time
- Mark complete IMMEDIATELY after finishing, not in batches
- Only mark completed when FULLY done (tests pass, no errors)
- Never print todos in response—user sees them in UI
</rules>
