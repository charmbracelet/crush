Creates and manages a structured task list for tracking progress when explicit,
manual task tracking is useful.

<when_to_use>
Use this tool when task tracking is specifically useful:

- User explicitly requests todo list management
- You need a persistent checklist for a long-running task in the current session
- You are already using the tool and need to update statuses accurately
</when_to_use>

<when_not_to_use>
Skip this tool when:

- Single, straightforward task
- Trivial task with no organizational benefit
- Purely conversational or informational request
- Independent tasks should be delegated to subagents instead of tracked here
- You can continue work directly without maintaining a manual checklist
</when_not_to_use>

<task_states>
- **pending**: Task not yet started
- **in_progress**: Currently working on (limit to ONE task at a time)
- **completed**: Task finished successfully

**IMPORTANT**: Each task requires two forms:
- **content**: Imperative form describing what needs to be done (e.g., "Run tests", "Build the project")
- **active_form**: Present continuous form shown during execution (e.g., "Running tests", "Building the project")
</task_states>

<task_management>
- Update task status in real-time as you work
- Mark tasks complete IMMEDIATELY after finishing (don't batch completions)
- Exactly ONE task must be in_progress at any time (not less, not more)
- Remove tasks that are no longer relevant from the list entirely
</task_management>

<completion_requirements>
ONLY mark a task as completed when you have FULLY accomplished it.

Never mark completed if:
- Tests are failing
- Implementation is partial
- You encountered unresolved errors
- You couldn't find necessary files or dependencies

If blocked:
- Keep task as in_progress
- Create new task describing what needs to be resolved
</completion_requirements>

<task_breakdown>
- Create specific, actionable items
- Break complex tasks into smaller, manageable steps
- Use clear, descriptive task names
- Always provide both content and active_form
</task_breakdown>

<examples>
✅ Good task:
```json
{
  "content": "Implement user authentication with JWT tokens",
  "status": "in_progress",
  "active_form": "Implementing user authentication with JWT tokens"
}
```

❌ Bad task (missing active_form):
```json
{
  "content": "Fix bug",
  "status": "pending"
}
```
</examples>

<output_behavior>
**NEVER** print or list todos in your response text. The user sees the todo list in real-time in the UI.
</output_behavior>

<tips>
- Use this tool only when the checklist itself adds value
- For parallel or independent work, prefer subagents over todo tracking
- Update immediately after state changes for accurate tracking
</tips>
