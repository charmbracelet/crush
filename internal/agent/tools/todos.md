Manage a structured task list for multi-step work; each task has pending/in_progress/completed state. Keep exactly one task in_progress at a time. Skip for simple or single-step tasks.

<task_fields>
- content: Imperative form (e.g., "Run tests")
- active_form: Present continuous form (e.g., "Running tests")
- status: pending | in_progress | completed
</task_fields>

<output_behavior>
**NEVER** list todos in your response text. The user sees them in the UI.
</output_behavior>
