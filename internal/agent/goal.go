package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
)

const (
	maxGoalContinuations = 8
	maxGoalFailures      = 3
	maxGoalFailureChars  = 500
)

type goalFailure struct {
	Tool   string `json:"tool"`
	Class  string `json:"class,omitempty"`
	Output string `json:"output"`
}

func goalModeContext(iteration int, failures []goalFailure) string {
	var context strings.Builder
	context.WriteString("<goal_mode>\n")
	context.WriteString("Continue autonomously toward the user's exact objective. Choose tools from current evidence; no tool is forced. ")
	context.WriteString("Use goal_status with complete after verification, or blocked when external input or state is required. ")
	context.WriteString("A normal final response also ends the run; do not repeat completed work only to report status. ")
	context.WriteString("Do not call goal_status for an intermediate failure.\n")
	fmt.Fprintf(&context, "Continuation %d of %d.\n", iteration, maxGoalContinuations)
	if len(failures) > 0 {
		encoded, _ := json.Marshal(failures)
		context.WriteString("Failures observed in the previous step: ")
		context.Write(encoded)
		context.WriteByte('\n')
	}
	context.WriteString("</goal_mode>")
	return context.String()
}

func goalNeedsContinuation(steps []fantasy.StepResult) bool {
	if len(steps) == 0 {
		return false
	}
	return steps[len(steps)-1].FinishReason == fantasy.FinishReasonLength
}

func prepareGoalContinuation(call SessionAgentCall, steps []fantasy.StepResult) SessionAgentCall {
	call.originalIntent = originalIntent(call)
	call.Prompt = call.originalIntent
	call.Attachments = nil
	call.Accepted = nil
	call.skipUserMessage = true
	call.goalIteration++
	call.TransientContext = appendTransientContext(call.goalBaseContext, goalModeContext(call.goalIteration, observedGoalFailures(steps)))
	return call
}

func terminalGoalStatus(steps []fantasy.StepResult) (string, bool) {
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
			if status == "complete" || status == "blocked" {
				return status, true
			}
		}
	}
	return "", false
}

func observedGoalFailures(steps []fantasy.StepResult) []goalFailure {
	var failures []goalFailure
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
			output = compactGoalFailure(output)
			key := strings.ToLower(name + "\x00" + class + "\x00" + output)
			if seen[key] {
				continue
			}
			seen[key] = true
			failures = append(failures, goalFailure{Tool: name, Class: class, Output: output})
		}
	}
	if len(failures) > maxGoalFailures {
		failures = failures[len(failures)-maxGoalFailures:]
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

func compactGoalFailure(output string) string {
	output = strings.Join(strings.Fields(output), " ")
	if len(output) <= maxGoalFailureChars {
		return output
	}
	return output[:maxGoalFailureChars] + "..."
}
