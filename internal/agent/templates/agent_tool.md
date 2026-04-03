Launch a subagent to handle a bounded task autonomously.

Available subagent types:
{agents}

When to use the Agent tool:
- Open-ended codebase exploration, pattern hunting, and implementation lookup should usually use the `explore` subagent.
- The `explore` subagent is read-only and has a restricted `bash` tool for direct local read-only git inspection only. It is suitable for `git diff`, `git status`, `git log`, `git show`, `git blame`, `git rev-parse`, `git merge-base`, and `git ls-files`, but not for mutating git commands, wrapper shells, or general shell work.
- Independent implementation tasks, test reproduction, or file-local refactors that can proceed without blocking your immediate next step should usually use the `general` subagent.
- If 2 or more substantial independent tasks can proceed in parallel, you should usually delegate them instead of doing them serially in the main thread.
- **When there are multiple substantial independent tasks, use a single Agent call with the `tasks` array** so they run in parallel with unified tracking, shared budget control, and a single consolidated result.
- If an `explore` subagent can gather context while you or another subagent handles implementation, start that delegated work immediately instead of waiting to do the search yourself first.
- Do not claim that you are delegating, spinning up subagents, or parallelizing work unless this response actually includes the corresponding `agent` tool calls.

When NOT to use the Agent tool:
- If the next step depends immediately on the result, do the work directly instead of delegating and waiting.
- Do not delegate tiny, tightly-coupled edits that are faster to do in the current thread.
- Do not delegate lightweight isolated single-file operations when direct tool calls are likely cheaper in tokens and just as fast.
- If several independent lightweight file operations can proceed in parallel, prefer multiple direct tool calls in one response instead of subagents.
- Do not use the main thread for broad implementation work just because you already know which files are involved. If those file changes are still separable, delegate them.
- **NEVER spawn a subagent whose sole job is to read files and return their contents.** If you already know the file paths, call `view`/`grep`/`glob` directly in the main thread — in parallel if needed. Spawning a subagent just to call `view` wastes a full LLM turn and a session context for zero benefit.

Usage notes:
1. Each subagent call is stateless and returns a single final report.
2. Your prompt must clearly state whether the subagent should only research or is allowed to modify code.
3. Tell the subagent exactly what output you need back, including relevant files, findings, and verification commands.
4. The subagent result is not shown to the user automatically; summarize the result yourself if needed.
5. The subagent's outputs should generally be trusted unless they conflict with stronger evidence in the current thread.
6. Do not treat this tool as a last resort. Prefer early delegation for bounded work that can unblock or parallelize the main task.
7. If you choose delegation, make the tool call first rather than narrating a future intention to delegate.
8. **Use the `tasks` array for 2+ tasks.** Tasks with no `depends_on` run in parallel; tasks with dependencies run after their prerequisites complete. The `tasks` array provides budget control, concurrency limiting, retry support, and a unified result — always prefer it over launching multiple separate Agent calls.
