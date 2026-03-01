Creates a fresh session context to continue the current task with a clean slate.

<when_to_use>
Use this tool when you are working on a long-running task and want to avoid using too much context.
A `<context_status>` block is injected on every turn containing `used_pct`, `remaining_tokens`, and `context_window`.

- By default, invoke `new_session` when `used_pct >= 75` (context is 75% full).
- The user may override the conditions for when this tool is called (e.g. "start a new session when there's only 5000 tokens remaining").
- The user may also override how the summary context that's passed in to this tool is generated.
- When approaching the threshold, proactively wrap up current work and call `new_session` with a comprehensive summary.
</when_to_use>

<usage>
- Provide a comprehensive description of the user's original request and what's being worked on.
- Provide a detailed summary of what has been accomplished so far.
- Provide a detailed summary of what still needs to be done.
- Include any critical context, file paths, or findings that the new session will need.
- Preserve user directives, such as instructions on how and when to run the new_session tool and any other tools.
- If any skills were loaded this session, always tell the new session to load those same skills
</usage>

<notes>
- Once invoked, the current session will terminate and a new session will launch immediately with your summary as its starting point.
- Ensure your summary is comprehensive enough that the new session won't need to re-discover basic project structure.
</notes>
