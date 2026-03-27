package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

//go:embed templates/auto_tool_output_guard.md
var autoToolOutputGuardPrompt []byte

//go:embed templates/auto_handoff_guard.md
var autoHandoffGuardPrompt []byte

type toolOutputGuardResponse struct {
	Suspicious bool                              `json:"suspicious"`
	Reason     string                            `json:"reason"`
	Confidence permission.AutoApprovalConfidence `json:"confidence"`
}

func (c *coordinator) reviewToolResultForPromptInjection(ctx context.Context, sessionID string, toolResult message.ToolResult, permissionMode session.PermissionMode) (message.ToolResult, error) {
	if permissionMode != session.PermissionModeAuto {
		return toolResult, nil
	}
	if toolResult.Data != "" || strings.TrimSpace(toolResult.Content) == "" {
		return toolResult, nil
	}

	model, providerCfg, err := c.selectedModel(ctx, config.SelectedModelTypeAutoClassifierReasoning, false)
	if err != nil {
		return toolResult.WithAutoReview(message.ToolResultAutoReview{
			Suspicious:     true,
			Sanitized:      true,
			DetectorFailed: true,
			Reason:         "Tool output review failed before model selection.",
			Confidence:     string(permission.AutoApprovalConfidenceLow),
		}), err
	}

	prompt := buildToolOutputGuardPrompt(c.cfg.Config(), toolResult)
	raw, err := c.runAutoGuardModel(ctx, model, providerCfg, string(autoToolOutputGuardPrompt), prompt, 256, 0.0)
	if err != nil {
		return toolResult.WithAutoReview(message.ToolResultAutoReview{
			Suspicious:     true,
			Sanitized:      true,
			DetectorFailed: true,
			Reason:         "Tool output review failed; output withheld from the model.",
			Confidence:     string(permission.AutoApprovalConfidenceLow),
		}), nil
	}

	review, err := parseToolOutputGuard(raw)
	if err != nil {
		return toolResult.WithAutoReview(message.ToolResultAutoReview{
			Suspicious:     true,
			Sanitized:      true,
			DetectorFailed: true,
			Reason:         "Tool output review returned an invalid response; output withheld from the model.",
			Confidence:     string(permission.AutoApprovalConfidenceLow),
		}), nil
	}

	return toolResult.WithAutoReview(message.ToolResultAutoReview{
		Suspicious: review.Suspicious,
		Sanitized:  review.Suspicious,
		Reason:     strings.TrimSpace(review.Reason),
		Confidence: string(review.Confidence),
	}), nil
}

func buildToolOutputGuardPrompt(cfg *config.Config, toolResult message.ToolResult) string {
	var sb strings.Builder
	sb.WriteString("Tool output to review:\n")
	sb.WriteString("- tool: ")
	sb.WriteString(toolResult.Name)
	sb.WriteString("\n- error: ")
	if toolResult.IsError {
		sb.WriteString("true")
	} else {
		sb.WriteString("false")
	}
	sb.WriteString("\n- output:\n")
	sb.WriteString(toolResult.Content)

	if cfg != nil && cfg.Permissions != nil && cfg.Permissions.AutoMode != nil && len(cfg.Permissions.AutoMode.BlockRules) > 0 {
		sb.WriteString("\n\nBlock rules:\n")
		for _, item := range cfg.Permissions.AutoMode.BlockRules {
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(item))
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func parseToolOutputGuard(raw string) (toolOutputGuardResponse, error) {
	raw = strings.TrimSpace(raw)
	if matches := autoClassifierCodeFenceRegex.FindStringSubmatch(raw); len(matches) == 2 {
		raw = strings.TrimSpace(matches[1])
	}
	var payload toolOutputGuardResponse
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return toolOutputGuardResponse{}, fmt.Errorf("failed to parse tool output guard response: %w", err)
	}
	if payload.Confidence == "" {
		payload.Confidence = permission.AutoApprovalConfidenceLow
	}
	return payload, nil
}

func (c *coordinator) reviewHandoffText(ctx context.Context, sess session.Session, title, content string) (permission.AutoClassification, error) {
	model, providerCfg, err := c.selectedModel(ctx, config.SelectedModelTypeAutoClassifierReasoning, false)
	if err != nil {
		return permission.AutoClassification{}, err
	}

	var sb strings.Builder
	sb.WriteString("Session title:\n")
	sb.WriteString(strings.TrimSpace(sess.Title))
	sb.WriteString("\n\nCollaboration mode:\n")
	sb.WriteString(string(sess.CollaborationMode))
	sb.WriteString("\n\nPermission mode:\n")
	sb.WriteString(string(sess.PermissionMode))
	sb.WriteString("\n\nCandidate handoff title:\n")
	sb.WriteString(strings.TrimSpace(title))
	sb.WriteString("\n\nCandidate handoff content:\n")
	sb.WriteString(strings.TrimSpace(content))

	if cfg := c.cfg.Config(); cfg != nil && cfg.Permissions != nil && cfg.Permissions.AutoMode != nil {
		if len(cfg.Permissions.AutoMode.Environment) > 0 {
			sb.WriteString("\n\nEnvironment:\n")
			for _, item := range cfg.Permissions.AutoMode.Environment {
				sb.WriteString("- ")
				sb.WriteString(strings.TrimSpace(item))
				sb.WriteByte('\n')
			}
		}
		if len(cfg.Permissions.AutoMode.BlockRules) > 0 {
			sb.WriteString("\nBlock rules:\n")
			for _, item := range cfg.Permissions.AutoMode.BlockRules {
				sb.WriteString("- ")
				sb.WriteString(strings.TrimSpace(item))
				sb.WriteByte('\n')
			}
		}
	}

	raw, err := c.runAutoGuardModel(ctx, model, providerCfg, string(autoHandoffGuardPrompt), sb.String(), 256, 0.0)
	if err != nil {
		return permission.AutoClassification{}, err
	}
	return parseAutoClassification(raw)
}
