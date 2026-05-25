Launch a specialized subagent to perform delegated work.

## When to use
- Tasks that benefit from focused, parallel execution (e.g., searching one area while you work on another)
- Multi-step work that would consume many turns if done sequentially
- Complex investigations that require synthesizing information from many files or sources
- Delegating well-scoped work where you can provide clear context and expectations

## When NOT to use
- **Simple lookups** that you can do in 1-2 direct tool calls → use the tool directly
- **Tasks requiring your judgment or approval** at every step → do it yourself
- **Work you haven't thought through** → first understand the problem, then delegate

## Parallel usage
You can launch multiple agents in parallel for independent tasks. For example, one agent searches the backend while another searches the frontend.

## Output
The agent returns a concise report of what it found or accomplished. If you need it to read specific sections in more detail, follow up with another agent call or use direct tools.

## Guidance
Use this tool when the task is clear enough to hand off, but large enough that doing it yourself would take many turns. Be specific about what the agent should deliver.

## How to prompt
Be specific about what to accomplish. Good prompts explain the goal, the relevant context, and what success looks like. Vague prompts like "fix the bug" waste turns.

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
