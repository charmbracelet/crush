Launch a new agent that runs the given task with its own tool set and context window, and returns a final report when done.

By default the agent has read-only tools (glob, grep, ls, view, sourcegraph, and LSP lookups) for searching for a keyword or file and reading code. Set write_tools to true to also give it bash, edit, multiedit, write, and download, so it can modify files and run commands; it still must pass the same permission checks.

Use this tool when a task requires many independent tool calls, like searching for a keyword when you are not confident you will find the right match on the first try, finding implementations across many files, or performing a focused, self-contained piece of work that would otherwise clutter the main conversation.

The agent runs on the currently selected model by default. Pass model (and optionally provider) to run it with a different model, e.g. model="claude-sonnet-4", provider="anthropic". Only use a different model when it is a clearly better fit for the task.

Prefer this tool for focused, self-contained tasks; for ambiguous questions you can answer faster yourself, do that instead.
