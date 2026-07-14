package agent

import (
	"encoding/json"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/goal"
)

func prepareGoalContinuation(call SessionAgentCall, state goal.State, steps []fantasy.StepResult) SessionAgentCall {
	call.originalIntent = originalIntent(call)
	call.Prompt = call.originalIntent
	call.Attachments = nil
	call.Accepted = nil
	call.skipUserMessage = true
	call.goalState = state
	call.TransientContext = appendTransientContext(call.goalBaseContext, goal.Context(state, observedGoalFailures(steps)))
	call.TransientContext = appendTransientContext(call.TransientContext, "<goal_continuation>Do not repeat the previous response. Perform a new concrete action, or call goal_status blocked if progress requires external input.</goal_continuation>")
	return call
}

func goalTurnMadeProgress(steps []fantasy.StepResult) bool {
	for _, step := range steps {
		for _, call := range step.Content.ToolCalls() {
			if !call.Invalid {
				return true
			}
		}
	}
	return false
}

func terminalGoalStatus(steps []fantasy.StepResult) (goal.Status, string, bool) {
	for i := len(steps) - 1; i >= 0; i-- {
		calls := make(map[string]tools.GoalStatusParams)
		for _, call := range steps[i].Content.ToolCalls() {
			if call.ToolName != tools.GoalStatusToolName {
				continue
			}
			var params tools.GoalStatusParams
			if json.Unmarshal([]byte(call.Input), &params) == nil {
				calls[call.ToolCallID] = params
			}
		}
		for _, result := range steps[i].Content.ToolResults() {
			params, ok := calls[result.ToolCallID]
			if !ok {
				continue
			}
			if _, isError := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result.Result); isError {
				continue
			}
			status := strings.ToLower(strings.TrimSpace(params.Status))
			switch status {
			case string(goal.StatusComplete):
				return goal.StatusComplete, params.Summary, true
			case string(goal.StatusBlocked):
				return goal.StatusBlocked, params.Summary, true
			}
		}
	}
	return "", "", false
}

func observedGoalFailures(steps []fantasy.StepResult) []goal.Failure {
	var failures []goal.Failure
	seen := make(map[string]bool)
	for _, step := range steps {
		toolNames := make(map[string]string)
		for _, call := range step.Content.ToolCalls() {
			toolNames[call.ToolCallID] = call.ToolName
		}
		for _, result := range step.Content.ToolResults() {
			output := strings.TrimSpace(toolResultOutputString(result.Result))
			class := failureClass(output)
			_, isError := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result.Result)
			if !isError && class == "" && !looksLikeFailedCommand(output) {
				continue
			}
			name := toolNames[result.ToolCallID]
			if name == "" {
				name = result.ToolName
			}
			output = strings.Join(strings.Fields(output), " ")
			key := strings.ToLower(name + "\x00" + class + "\x00" + output)
			if seen[key] {
				continue
			}
			seen[key] = true
			failures = append(failures, goal.Failure{Tool: name, Class: class, Output: output})
		}
	}
	return failures
}

func looksLikeFailedCommand(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "exit code 1") ||
		strings.Contains(lower, "exit code 2") ||
		strings.Contains(lower, "exit code 127") ||
		strings.HasPrefix(lower, "error:") ||
		strings.HasPrefix(lower, "error ")
}
