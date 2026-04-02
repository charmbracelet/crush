package agent

import (
	"slices"
	"strings"

	"charm.land/fantasy"
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

const autoModeSystemPrompt = `<permission_mode>
You are in Auto Mode.

Auto Mode rules override any conflicting instruction that would otherwise cause unnecessary permission-related interruptions.

In Auto Mode you should:
- Execute autonomously and keep moving when the request is clear.
- Minimize interruptions and prefer reasonable assumptions over low-value clarification questions.
- Expect some sensitive actions to still require manual confirmation when the safety classifier is unsure.
- Prefer safe local actions and incremental progress over broad risky changes.
- Be thorough: complete the task end-to-end, including verification, unless a hard blocker requires user input.
</permission_mode>`

const yoloModeSystemPrompt = `<permission_mode>
You are in YOLO Mode.

YOLO Mode auto-approves permission checks.

Proceed without waiting for permission prompts, but still avoid pointless risk and stay aligned with the user's request.
</permission_mode>`

const defaultModeSystemPrompt = `<permission_mode>
Auto Mode is not active.

Do not assume permission-requiring actions will be auto-approved. When manual confirmation is required, wait for it instead of assuming it has already been granted.
</permission_mode>`

type toolRiskLevel string

const (
	toolRiskRead       toolRiskLevel = "read"
	toolRiskWrite      toolRiskLevel = "write"
	toolRiskExecute    toolRiskLevel = "execute"
	toolRiskNetwork    toolRiskLevel = "network"
	toolRiskDelegation toolRiskLevel = "delegation"
)

var toolRiskLevels = map[string]toolRiskLevel{
	AgentToolName:                     toolRiskDelegation,
	tools.AgenticFetchToolName:        toolRiskNetwork,
	tools.BashToolName:                toolRiskExecute,
	tools.JobOutputToolName:           toolRiskExecute,
	tools.JobWaitToolName:             toolRiskExecute,
	tools.JobKillToolName:             toolRiskExecute,
	tools.DownloadToolName:            toolRiskNetwork,
	tools.EditToolName:                toolRiskWrite,
	tools.MultiEditToolName:           toolRiskWrite,
	tools.FetchToolName:               toolRiskNetwork,
	tools.GlobToolName:                toolRiskRead,
	tools.GrepToolName:                toolRiskRead,
	tools.LSToolName:                  toolRiskRead,
	tools.SourcegraphToolName:         toolRiskNetwork,
	tools.HistorySearchToolName:       toolRiskRead,
	tools.LongTermMemoryToolName:      toolRiskWrite,
	tools.TodosToolName:               toolRiskWrite,
	tools.SendMessageToolName:         toolRiskWrite,
	tools.TaskStopToolName:            toolRiskWrite,
	tools.ViewToolName:                toolRiskRead,
	tools.WriteToolName:               toolRiskWrite,
	tools.ListMCPResourcesToolName:    toolRiskNetwork,
	tools.ReadMCPResourceToolName:     toolRiskNetwork,
	tools.DiagnosticsToolName:         toolRiskRead,
	tools.ReferencesToolName:          toolRiskRead,
	tools.LSPDeclarationToolName:      toolRiskRead,
	tools.LSPDefinitionToolName:       toolRiskRead,
	tools.LSPImplementationToolName:   toolRiskRead,
	tools.LSPTypeDefinitionToolName:   toolRiskRead,
	tools.LSPHoverToolName:            toolRiskRead,
	tools.LSPDocumentSymbolsToolName:  toolRiskRead,
	tools.LSPWorkspaceSymbolsToolName: toolRiskRead,
	tools.LSPCodeActionToolName:       toolRiskWrite,
	tools.LSPRenameToolName:           toolRiskWrite,
	tools.LSPFormatToolName:           toolRiskWrite,
	tools.LSPRestartToolName:          toolRiskExecute,
	tools.RequestUserInputToolName:    toolRiskRead,
	tools.PlanExitToolName:            toolRiskRead,
	tools.ToolSearchToolName:          toolRiskRead,
}

var planModeReadToolNames = map[string]struct{}{
	tools.GlobToolName:                {},
	tools.GrepToolName:                {},
	tools.LSToolName:                  {},
	tools.ViewToolName:                {},
	tools.HistorySearchToolName:       {},
	tools.DiagnosticsToolName:         {},
	tools.ReferencesToolName:          {},
	tools.LSPDeclarationToolName:      {},
	tools.LSPDefinitionToolName:       {},
	tools.LSPImplementationToolName:   {},
	tools.LSPTypeDefinitionToolName:   {},
	tools.LSPHoverToolName:            {},
	tools.LSPDocumentSymbolsToolName:  {},
	tools.LSPWorkspaceSymbolsToolName: {},
}

func collaborationModePrompt(mode session.CollaborationMode) string {
	switch mode {
	case session.CollaborationModePlan:
		return planModeSystemPrompt
	default:
		return ""
	}
}

func permissionModePrompt(mode session.PermissionMode) string {
	switch mode {
	case session.PermissionModeAuto:
		return autoModeSystemPrompt
	case session.PermissionModeYolo:
		return yoloModeSystemPrompt
	default:
		return defaultModeSystemPrompt
	}
}

func buildSystemPromptForModes(basePrompt string, mode session.CollaborationMode, permissionMode session.PermissionMode) string {
	sections := []string{basePrompt}
	if prompt := collaborationModePrompt(mode); prompt != "" {
		sections = append(sections, prompt)
	}
	if prompt := permissionModePrompt(permissionMode); prompt != "" {
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

func riskLevelForTool(toolName string) toolRiskLevel {
	if level, ok := toolRiskLevels[toolName]; ok {
		return level
	}
	return toolRiskExecute
}

func isPlanModeToolAllowed(toolName string) bool {
	if toolName == tools.RequestUserInputToolName || toolName == tools.PlanExitToolName {
		return true
	}
	if riskLevelForTool(toolName) != toolRiskRead {
		return false
	}
	_, ok := planModeReadToolNames[toolName]
	return ok
}

func deduplicateToolNames(toolNames []string) []string {
	filtered := make([]string, 0, len(toolNames))
	seen := make(map[string]struct{}, len(toolNames))
	for _, toolName := range toolNames {
		if _, ok := seen[toolName]; ok {
			continue
		}
		seen[toolName] = struct{}{}
		filtered = append(filtered, toolName)
	}
	return filtered
}

func removeDisabledToolNames(toolNames []string, disabledToolNames []string) []string {
	if len(disabledToolNames) == 0 {
		return toolNames
	}
	filtered := make([]string, 0, len(toolNames))
	for _, toolName := range toolNames {
		if slices.Contains(disabledToolNames, toolName) {
			continue
		}
		filtered = append(filtered, toolName)
	}
	return filtered
}

func filterToolsForRiskPolicy(toolNames []string, mode session.CollaborationMode, disabledToolNames []string) []string {
	filtered := deduplicateToolNames(toolNames)
	if mode != session.CollaborationModePlan {
		return removeDisabledToolNames(filtered, disabledToolNames)
	}

	planModeTools := make([]string, 0, len(filtered)+2)
	for _, toolName := range filtered {
		if isPlanModeToolAllowed(toolName) {
			planModeTools = append(planModeTools, toolName)
		}
	}
	if !slices.Contains(planModeTools, tools.RequestUserInputToolName) {
		planModeTools = append(planModeTools, tools.RequestUserInputToolName)
	}
	if !slices.Contains(planModeTools, tools.PlanExitToolName) {
		planModeTools = append(planModeTools, tools.PlanExitToolName)
	}
	return planModeTools
}

func filterToolsForCollaborationMode(toolNames []string, mode session.CollaborationMode) []string {
	return filterToolsForRiskPolicy(toolNames, mode, nil)
}

func filterToolsByNames(toolsList []fantasy.AgentTool, allowedNames []string) []fantasy.AgentTool {
	if len(allowedNames) == 0 {
		return toolsList
	}
	allowedSet := make(map[string]bool, len(allowedNames))
	for _, name := range allowedNames {
		allowedSet[name] = true
	}
	filtered := make([]fantasy.AgentTool, 0, len(toolsList))
	for _, tool := range toolsList {
		if allowedSet[tool.Info().Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}
