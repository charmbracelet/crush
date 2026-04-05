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
	Artifacts      []string
	FilesTouched   []string
	PatchPlan      []string
	TestResults    []string
	Followups      []string
}

func Reduce(results []TaskResult) message.ToolResultReducer {
	completed := 0
	failed := 0
	canceled := 0
	artifacts := make([]string, 0, len(results))
	filesTouched := make([]string, 0, len(results))
	patchPlan := make([]string, 0, len(results))
	testResults := make([]string, 0, len(results))
	followupQuestions := make([]string, 0, len(results))
	risks := make([]string, 0)
	nextActions := make([]string, 0, 2)
	seenArtifacts := make(map[string]struct{}, len(results)*2)
	seenFiles := make(map[string]struct{}, len(results)*2)
	seenPatchPlan := make(map[string]struct{}, len(results)*2)
	seenTests := make(map[string]struct{}, len(results)*2)
	seenFollowups := make(map[string]struct{}, len(results)*2)
	addArtifact := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seenArtifacts[value]; ok {
			return
		}
		seenArtifacts[value] = struct{}{}
		artifacts = append(artifacts, value)
	}
	addFile := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seenFiles[value]; ok {
			return
		}
		seenFiles[value] = struct{}{}
		filesTouched = append(filesTouched, value)
	}
	addPatchPlan := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seenPatchPlan[value]; ok {
			return
		}
		seenPatchPlan[value] = struct{}{}
		patchPlan = append(patchPlan, value)
	}
	addTestResult := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seenTests[value]; ok {
			return
		}
		seenTests[value] = struct{}{}
		testResults = append(testResults, value)
	}
	addFollowup := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seenFollowups[value]; ok {
			return
		}
		seenFollowups[value] = struct{}{}
		followupQuestions = append(followupQuestions, value)
	}

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
			addArtifact(fmt.Sprintf("%s session: %s", title, result.ChildSessionID))
		}
		for _, artifact := range result.Artifacts {
			addArtifact(artifact)
		}
		for _, filePath := range result.FilesTouched {
			addFile(filePath)
		}
		for _, step := range result.PatchPlan {
			addPatchPlan(step)
		}
		for _, test := range result.TestResults {
			addTestResult(test)
		}
		for _, question := range result.Followups {
			addFollowup(question)
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
		Summary:           summary,
		Artifacts:         artifacts,
		FilesTouched:      filesTouched,
		PatchPlan:         patchPlan,
		TestResults:       testResults,
		FollowupQuestions: followupQuestions,
		Risks:             risks,
		NextActions:       nextActions,
		Confidence:        confidence,
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
