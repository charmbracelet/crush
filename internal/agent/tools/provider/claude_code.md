Delegate a self-contained coding task to Claude Code, Anthropic's CLI agent,
running headlessly in the current working directory.

Claude Code runs with its own tools (read, edit, bash, ...) and its own
context. It cannot see this conversation, so the prompt must be complete on
its own: the goal, the relevant file paths, and any constraints. Use it when
the user asks to hand work to Claude Code or their Claude subscription, or
when an independent second implementation of a hard problem is useful.

Each call asks the user for permission first, showing the full prompt.
Once approved, Claude Code works in the project directory and may use its tools
without asking again, at the access level the user configured.

Parameters:

- prompt: the full task description for Claude Code. Required.
- resume: a session ID returned by an earlier claude_code call. Pass it to
  continue that Claude conversation instead of starting a new one.

The response is Claude Code's final report, followed by the session ID of
the delegation. Keep the session ID if follow-up work in the same Claude
conversation is likely, and pass it as resume next time.
