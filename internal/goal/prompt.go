package goal

import (
	"encoding/json"
	"fmt"
	"strings"
)

const maxFailureChars = 500

type Failure struct {
	Tool   string `json:"tool"`
	Class  string `json:"class,omitempty"`
	Output string `json:"output"`
}

func Context(state State, failures []Failure) string {
	var context strings.Builder
	context.WriteString("<goal_mode>\n")
	fmt.Fprintf(&context, "Objective: %s\nStatus: %s | Turn: %d\n", state.Objective, state.Status, state.Turns+1)
	if state.Summary != "" {
		fmt.Fprintf(&context, "Previous turn: %s\n", state.Summary)
	}
	context.WriteString("Work toward the objective in this turn. Use goal_status complete after verification, or blocked when user input is required. A normal reply ends the turn and preserves the goal.\n")
	if len(failures) > 0 {
		encoded, _ := json.Marshal(compactFailures(failures))
		context.WriteString("Recent failures: ")
		context.Write(encoded)
		context.WriteByte('\n')
	}
	context.WriteString("</goal_mode>")
	return context.String()
}

func compactFailures(failures []Failure) []Failure {
	if len(failures) > 3 {
		failures = failures[len(failures)-3:]
	}
	result := make([]Failure, len(failures))
	for i, failure := range failures {
		failure.Output = strings.Join(strings.Fields(failure.Output), " ")
		if len(failure.Output) > maxFailureChars {
			failure.Output = failure.Output[:maxFailureChars] + "..."
		}
		result[i] = failure
	}
	return result
}
