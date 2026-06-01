package flowrag

import (
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

type WorkflowStep struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Input   string `json:"input,omitempty"`
	Output  string `json:"output,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

type Workflow struct {
	UserPrompt string         `json:"user_prompt"`
	Steps      []WorkflowStep `json:"steps"`
	SessionID  string         `json:"session_id"`
}

type Segmenter struct{}

func NewSegmenter() *Segmenter {
	return &Segmenter{}
}

func (s *Segmenter) Segment(userPrompt string, messages []message.Message) *Workflow {
	steps := make([]WorkflowStep, 0)
	errorToolCallIDs := make(map[string]bool)

	for _, msg := range messages {
		if msg.Role == message.Tool {
			for _, tr := range msg.ToolResults() {
				if tr.IsError {
					errorToolCallIDs[tr.ToolCallID] = true
				}
			}
		}
	}

	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			if len(steps) == 0 {
				continue
			}
			steps = append(steps, WorkflowStep{
				Role:    "user",
				Content: msg.Content().Text,
			})
		case message.Assistant:
			textContent := msg.Content().Text
			if textContent != "" {
				steps = append(steps, WorkflowStep{
					Role:    "assistant",
					Content: textContent,
				})
			}
			for _, tc := range msg.ToolCalls() {
				if errorToolCallIDs[tc.ID] {
					continue
				}
				steps = append(steps, WorkflowStep{
					Role:  "tool_call",
					Tool:  tc.Name,
					Input: tc.Input,
				})
			}
		case message.Tool:
			for _, tr := range msg.ToolResults() {
				if tr.IsError {
					continue
				}
				if errorToolCallIDs[tr.ToolCallID] {
					continue
				}
				steps = append(steps, WorkflowStep{
					Role:    "tool_result",
					Tool:    tr.Name,
					Output:  tr.Content,
					IsError: false,
				})
			}
		}
	}

	return &Workflow{
		UserPrompt: userPrompt,
		Steps:      steps,
	}
}

func (w *Workflow) ToText() string {
	var sb strings.Builder
	sb.WriteString("User: ")
	sb.WriteString(w.UserPrompt)
	sb.WriteString("\n")
	for _, step := range w.Steps {
		switch step.Role {
		case "user":
			sb.WriteString("User: ")
			sb.WriteString(step.Content)
			sb.WriteString("\n")
		case "assistant":
			sb.WriteString("Assistant: ")
			sb.WriteString(step.Content)
			sb.WriteString("\n")
		case "tool_call":
			sb.WriteString("Tool Call: ")
			sb.WriteString(step.Tool)
			if step.Input != "" {
				sb.WriteString("(")
				sb.WriteString(step.Input)
				sb.WriteString(")")
			}
			sb.WriteString("\n")
		case "tool_result":
			sb.WriteString("Tool Result: ")
			sb.WriteString(step.Tool)
			sb.WriteString(" -> ")
			sb.WriteString(step.Output)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
