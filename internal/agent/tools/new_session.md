Creates a fresh session context to continue the current task with a clean slate.

<when_to_use>
Use this tool when you are working on a long-running task and want to avoid getting too close to the context limit.
A `<context_status>` block in the system prompt contains `used_pct`, `remaining_tokens`, and `context_window`.

- By default, invoke `new_session` when `used_pct >= 75` (context is 75% full) per the instructions in the usage section.
- The user may override the conditions for when and how this tool is called (e.g. "start a new session when there's only 5000 tokens remaining").
- The user may also override how the summary context that's passed in to this tool is generated.
</when_to_use>

<usage>
Unless the user asked you to invoke the new_session tool with directives that contradict the instructions below, invoke the tool with a summary according to these rules:

1. A comprehensive description of the task that was worked on during this session
2. What was accomplished during this session
3. A detailed summary of what remains to be done in the new session

- Include any critical context, file paths, or findings that the new session will need.
- Preserve user directives, such as instructions on how and when to run the new_session tool and any other tools.
- If any skills were loaded during this session, always tell the new session to load those same skills at the start of the new session
</usage>

<notes>
- Once invoked, the current session will terminate and a new session will launch immediately with your summary as its starting point.
- Ensure your summary is comprehensive enough that the new session won't need to re-discover basic project structure.
</notes>
