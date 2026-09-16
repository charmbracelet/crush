package flowrag

import (
	"context"
	"fmt"
	"strings"
)

type Retriever struct {
	store *VectorStore
}

func NewRetriever(store *VectorStore) *Retriever {
	return &Retriever{store: store}
}

type SearchResult struct {
	Workflow   WorkflowRecord `json:"workflow"`
	Similarity float64        `json:"-"`
}

func (r *Retriever) SearchSimilar(ctx context.Context, userPrompt string, topK int) ([]WorkflowRecord, error) {
	return r.store.Search(ctx, userPrompt, topK)
}

func (r *Retriever) BuildContextPrompt(records []WorkflowRecord) string {
	if len(records) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<past_successful_workflows>")
	sb.WriteString("\nBelow are similar tasks that were successfully completed in the past. You can reference their approaches.\n\n")

	for i, record := range records {
		sb.WriteString(fmt.Sprintf("--- Similar Workflow %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Original Request: %s\n", record.UserPrompt))
		sb.WriteString("Steps:\n")
		for _, step := range record.Steps {
			switch step.Role {
			case "tool_call":
				sb.WriteString(fmt.Sprintf("  [Tool: %s] Input: %s\n", step.Tool, step.Input))
			case "tool_result":
				sb.WriteString(fmt.Sprintf("  [Result: %s] %s\n", step.Tool, truncate(step.Output, 500)))
			default:
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("</past_successful_workflows>")
	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
