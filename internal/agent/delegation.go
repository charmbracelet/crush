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
