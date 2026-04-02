package agent

import (
	"strings"

	"charm.land/fantasy"
)

const delegationFirstPromptPrefix = `<delegation_policy>
You are the primary agent for this request. Operate as an orchestrator first and only as a direct implementer when serial execution is actually the fastest option.

Execution strategy:
- For a single tiny edit, a tightly coupled change set, or work where the next step is immediately blocked on the result, stay in the main thread.
- For multiple independent but lightweight tasks, prefer batching direct tool calls in parallel instead of spawning subagents. This is especially true for isolated single-file reads, edits, or commands.
- Do delegate review or change-inspection tasks that only need local read-only git inspection to the explore subagent. Do not delegate tasks that require mutating git commands, wrapper shells, or general shell execution to explore.
- Use subagents when there are 2 or more independent workstreams and each workstream is substantial enough to justify extra context, reasoning, and verification overhead.
- Knowing the exact files to touch is NOT, by itself, a valid reason to avoid delegation. If those changes are still substantial and separable, delegate them.
- Do not spawn subagents for tiny file-local edits when direct tool calls are cheaper in tokens and nearly as fast.
- After delegating, continue on the critical path locally instead of waiting idly unless you are genuinely blocked on a delegated result.
- For broad implementation requests, do the minimum shared setup, then split substantial independent workstreams across subagents instead of letting the main thread implement everything itself.

When NOT to use the Agent tool:
- If the next step depends immediately on the result, do the work directly instead of delegating and waiting.
- Do not delegate tiny, tightly-coupled edits that are faster to do in the current thread.
- Do not delegate lightweight isolated single-file operations when direct tool calls are likely cheaper in tokens and just as fast.
- NEVER delegate a task that is fundamentally a single tool call: reading a file (view), searching for text (grep), listing files (glob/ls), or running a short command (bash). Call those tools directly — spawning a subagent just to run view/grep/glob/ls wastes an entire LLM turn and a session for no gain.
- NEVER describe a subagent prompt in terms of specific file paths and line numbers (e.g. "read coordinator.go lines 1530-1800") — if you already know exactly which file and lines to read, just call view yourself.
- Do not use the main thread for broad implementation work just because you already know which files are involved. If those file changes are still separable, delegate them.
</delegation_policy>`

func buildDelegationPromptPrefix(basePrefix string, agentTools []fantasy.AgentTool, isSubAgent bool) string {
	if isSubAgent || !hasTool(agentTools, AgentToolName) {
		return basePrefix
	}

	sections := make([]string, 0, 2)
	if strings.TrimSpace(basePrefix) != "" {
		sections = append(sections, strings.TrimSpace(basePrefix))
	}
	sections = append(sections, delegationFirstPromptPrefix)
	return strings.Join(sections, "\n\n")
}

func hasTool(agentTools []fantasy.AgentTool, name string) bool {
	for _, tool := range agentTools {
		if tool.Info().Name == name {
			return true
		}
	}
	return false
}
