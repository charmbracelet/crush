package agent

import (
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/session"
)

const planModeSystemPrompt = `<collaboration_mode>
You are in Plan Mode.

Plan Mode rules override any conflicting instruction that tells you to execute changes immediately or to avoid asking questions.

In Plan Mode you must stay in read-only exploration and planning.
- Do not write files, edit files, run mutating commands, change configuration, or otherwise change repo-tracked or system state.
- Prefer understanding over speed: explore the codebase thoroughly before deciding on an implementation strategy.
- Look for existing patterns, similar features, reusable helpers, and architectural conventions before proposing new structures.
- Consider the main implementation options and their tradeoffs, then recommend one concrete approach.
- Keep planning until the task is decision-complete and implementation-ready.

Clarification rules:
- First try to resolve ambiguities by reading the repo and related context.
- Only if a material product or implementation decision remains unresolved, use the request_user_input tool.
- Do not ask low-value or easily-assumed questions.
- Do not ask the user to approve the plan in free-form text; the UI handles approval after you finish planning.

Output rules:
- If the user asks you to implement while Plan Mode is active, do not implement; continue planning instead.
- When the plan is implementation-ready, call the plan_exit tool.
- Your final textual answer must be exactly one <proposed_plan>...</proposed_plan> block and nothing else.
- The proposed plan should be concise but execution-ready.
- Include the key files or subsystems to change, the main steps, important reuse points, and the validation approach.
- Do not end a planning turn with a completed plan unless you also called plan_exit.
</collaboration_mode>`

func collaborationModePrompt(mode session.CollaborationMode) string {
	if mode == session.CollaborationModePlan {
		return planModeSystemPrompt
	}
	return ""
}

func buildSystemPromptForCollaborationMode(basePrompt string, mode session.CollaborationMode) string {
	sections := []string{basePrompt}
	if prompt := collaborationModePrompt(mode); prompt != "" {
		sections = append(sections, prompt)
	}

	filtered := make([]string, 0, len(sections))
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		filtered = append(filtered, section)
	}
	return strings.Join(filtered, "\n\n")
}

func filterToolsForCollaborationMode(toolNames []string, mode session.CollaborationMode) []string {
	if mode != session.CollaborationModePlan {
		return slices.Clone(toolNames)
	}

	allowed := map[string]struct{}{
		tools.BashToolName:             {},
		tools.FetchToolName:            {},
		tools.GlobToolName:             {},
		tools.GrepToolName:             {},
		tools.LSToolName:               {},
		tools.ViewToolName:             {},
		tools.DiagnosticsToolName:      {},
		tools.ReferencesToolName:       {},
		tools.ListMCPResourcesToolName: {},
		tools.ReadMCPResourceToolName:  {},
		tools.RequestUserInputToolName: {},
		tools.PlanExitToolName:         {},
		tools.SourcegraphToolName:      {},
	}

	filtered := make([]string, 0, len(toolNames))
	for _, toolName := range toolNames {
		if _, ok := allowed[toolName]; ok {
			filtered = append(filtered, toolName)
		}
	}
	if !slices.Contains(filtered, tools.PlanExitToolName) {
		filtered = append(filtered, tools.PlanExitToolName)
	}
	return filtered
}
