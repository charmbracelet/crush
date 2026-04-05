package reducer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

type TaskResult struct {
	ID             string
	Description    string
	Status         message.ToolResultSubtaskStatus
	ChildSessionID string
	Content        string
}

func Reduce(results []TaskResult) message.ToolResultReducer {
	completed := 0
	failed := 0
	canceled := 0
	artifacts := make([]string, 0, len(results))
	risks := make([]string, 0)
	nextActions := make([]string, 0, 2)

	for _, result := range results {
		title := strings.TrimSpace(result.Description)
		if title == "" {
			title = result.ID
		}

		switch result.Status {
		case message.ToolResultSubtaskStatusFailed:
			failed++
			risks = append(risks, fmt.Sprintf("%s failed: %s", title, summarizeText(result.Content)))
		case message.ToolResultSubtaskStatusCanceled:
			canceled++
			risks = append(risks, fmt.Sprintf("%s canceled: %s", title, summarizeText(result.Content)))
		default:
			completed++
		}

		if strings.TrimSpace(result.ChildSessionID) != "" {
			artifacts = append(artifacts, fmt.Sprintf("%s session: %s", title, result.ChildSessionID))
		}
	}

	summary := fmt.Sprintf("Completed %d/%d subtasks.", completed, len(results))
	if failed > 0 || canceled > 0 {
		summary = fmt.Sprintf("Completed %d/%d subtasks (%d failed, %d canceled).", completed, len(results), failed, canceled)
	}

	hasFailures := failed > 0
	hasCancellations := canceled > 0
	if hasFailures || hasCancellations {
		nextActions = append(nextActions, "Address failed or canceled subtasks before finalizing.")
	}
	if completed > 0 {
		nextActions = append(nextActions, "Review child session outputs and integrate accepted results.")
	}
	if len(nextActions) == 0 {
		nextActions = append(nextActions, "Run subtasks to gather actionable output.")
	}

	confidence := "high"
	if hasFailures {
		confidence = "low"
	} else if hasCancellations {
		confidence = "medium"
	}

	return message.ToolResultReducer{
		Summary:     summary,
		Artifacts:   artifacts,
		Risks:       risks,
		NextActions: nextActions,
		Confidence:  confidence,
	}
}

func summarizeText(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "no additional details"
	}
	content = strings.Join(strings.Fields(content), " ")
	const maxLen = 160
	if len([]rune(content)) <= maxLen {
		return content
	}
	return string([]rune(content)[:maxLen]) + "..."
}
