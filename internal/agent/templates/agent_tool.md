Launch a specialized read-only agent to thoroughly search and explore the code and documentation.

This agent has access to `glob`, `grep`, `ls`, `view`, and `sourcegraph`. It is optimized for finding files, searching code and documentation, and analyzing architecture — but it **cannot edit files or run commands**.

## When to use
- Complex multi-step searches (finding a pattern across many files)
- Understanding codebase architecture ("how does the auth system work?")
- Finding all references to a function or type
- Exploring an unfamiliar codebase
- Cross-referencing between multiple packages/modules

## When NOT to use
- **Reading a specific file you already know the path to** → use `view` directly
- **Searching for a specific function or string in code** → use `grep` directly
- **Searching within 2-3 known files** → use `view` on each file directly
- **Simple lookups** that don't require exploration or synthesis

## Parallel usage
You can launch multiple agents in parallel for independent searches. For example, one agent searches for frontend code while another searches for backend code.

## Output
The agent returns a concise report with absolute file paths and line numbers. If you need the agent to read specific sections in more detail, follow up with another agent call or use `view` directly.

## Guidance
Use this tool only when a simple, directed search proves insufficient or when the task will clearly require more than 3 separate queries.

## How to prompt
Be specific about what to find. Good prompts name the target (function, type, pattern) and what the caller needs to know about it. Vague prompts like "search for auth" waste turns.

## Writing the prompt
When spawning an agent, it starts with zero context. Brief the agent like a smart colleague who just walked into the room — it hasn't seen this conversation, doesn't know what you've tried, doesn't understand why this task matters.
- Explain what you're trying to accomplish and why.
- Describe what you've already learned or ruled out.
- Give enough context about the surrounding problem that the agent can make judgment calls rather than just following a narrow instruction.
- If you need a short response, say so ("report in under 200 words").
- Lookups: hand over the exact command. Investigations: hand over the question — prescribed steps become dead weight when the premise is wrong.

Terse command-style prompts produce shallow, generic work.

## Never delegate understanding
Don't write "based on your findings, fix the bug" or "based on the research, implement it." Those phrases push synthesis onto the agent instead of doing it yourself. Write prompts that prove you understood: include file paths, line numbers, what specifically to change.
