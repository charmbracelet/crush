Launch a subagent to handle a bounded task autonomously.

Available subagent types:
{agents}

When to use the Agent tool:
- Open-ended codebase exploration, pattern hunting, and implementation lookup should usually use the `explore` subagent.
- Independent implementation tasks, test reproduction, or file-local refactors that can proceed without blocking your immediate next step should usually use the `general` subagent.
- If 2 or more substantial independent tasks can proceed in parallel, you should usually delegate them instead of doing them serially in the main thread.
- When there are multiple substantial independent tasks, launch multiple Agent tool calls in the same assistant message so they can run in parallel.
- If an `explore` subagent can gather context while you or another subagent handles implementation, start that delegated work immediately instead of waiting to do the search yourself first.
- Do not claim that you are delegating, spinning up subagents, or parallelizing work unless this response actually includes the corresponding `agent` tool calls.

When NOT to use the Agent tool:
- If the next step depends immediately on the result, do the work directly instead of delegating and waiting.
- Do not delegate tiny, tightly-coupled edits that are faster to do in the current thread.
- Do not delegate lightweight isolated single-file operations when direct tool calls are likely cheaper in tokens and just as fast.
- If several independent lightweight file operations can proceed in parallel, prefer multiple direct tool calls in one response instead of subagents.
- Do not use the main thread for broad implementation work just because you already know which files are involved. If those file changes are still separable, delegate them.

Usage notes:
1. Each subagent call is stateless and returns a single final report.
2. Your prompt must clearly state whether the subagent should only research or is allowed to modify code.
3. Tell the subagent exactly what output you need back, including relevant files, findings, and verification commands.
4. The subagent result is not shown to the user automatically; summarize the result yourself if needed.
5. The subagent's outputs should generally be trusted unless they conflict with stronger evidence in the current thread.
6. Do not treat this tool as a last resort. Prefer early delegation for bounded work that can unblock or parallelize the main task.
7. If you choose delegation, make the tool call first rather than narrating a future intention to delegate.
