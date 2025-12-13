Creates and manages a structured task list for tracking progress on complex, multi-step coding tasks.

<when_to_use>
Use this tool proactively in these scenarios:

- Complex multi-step tasks requiring 3+ distinct steps or actions
- Non-trivial tasks requiring careful planning or multiple operations
- User explicitly requests todo list management
- User provides multiple tasks (numbered or comma-separated list)
- After receiving new instructions to capture requirements
- When starting work on a task (mark as in_progress BEFORE beginning)
- After completing a task (mark completed and add new follow-up tasks)
</when_to_use>

<when_not_to_use>
Skip this tool when:

- Single, straightforward task
- Trivial task with no organizational benefit
- Task completable in less than 3 trivial steps
- Purely conversational or informational request
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
- Complete current tasks before starting new ones
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
- When in doubt, use this tool - being proactive demonstrates attentiveness
- One task in_progress at a time keeps work focused
- Update immediately after state changes for accurate tracking
</tips>
