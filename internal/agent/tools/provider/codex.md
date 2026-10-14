Delegate a self-contained coding task to Codex, OpenAI's CLI agent, running
headlessly in the current working directory.

Codex runs with its own tools (shell, file edits, ...) and its own context.
It cannot see this conversation, so the prompt must be complete on its own:
the goal, the relevant file paths, and any constraints. Use it when the user
asks to hand work to Codex or their ChatGPT plan, or when an independent
second implementation of a hard problem is useful. Codex must be signed in
on this machine ("codex" once, interactively); no API key is involved.

The permission prompt for this tool call is the trust boundary: approving
the delegation lets Codex run commands with full access inside the project
directory without asking again.

Parameters:

- prompt: the full task description for Codex. Required.
- resume: a thread ID returned by an earlier codex call. Pass it to continue
  that Codex conversation instead of starting a new one.

The response is Codex's final report, followed by the thread ID of the
delegation. Keep the thread ID if follow-up work in the same Codex
conversation is likely, and pass it as resume next time.
