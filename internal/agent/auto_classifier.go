package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

//go:embed templates/auto_classifier_fast.md
var autoClassifierFastPrompt []byte

//go:embed templates/auto_classifier_reasoning.md
var autoClassifierReasoningPrompt []byte

var autoClassifierCodeFenceRegex = regexp.MustCompile("(?s)^```(?:json)?\\s*(.*?)\\s*```$")

type autoClassifierResponse struct {
	AllowAuto  bool                              `json:"allow_auto"`
	Reason     string                            `json:"reason"`
	Confidence permission.AutoApprovalConfidence `json:"confidence"`
}

func (c *coordinator) ClassifyPermission(ctx context.Context, req permission.PermissionRequest) (permission.AutoClassification, error) {
	model, providerCfg, err := c.selectedModel(ctx, config.SelectedModelTypeAutoClassifierFast, false)
	if err != nil {
		return permission.AutoClassification{}, err
	}

	sess, err := c.sessions.Get(ctx, req.SessionID)
	if err != nil {
		return permission.AutoClassification{}, fmt.Errorf("failed to load session for auto classification: %w", err)
	}
	msgs, err := c.messages.List(ctx, req.SessionID)
	if err != nil {
		return permission.AutoClassification{}, fmt.Errorf("failed to load messages for auto classification: %w", err)
	}

	prompt := buildAutoClassifierPrompt(c.cfg.Config(), c.cfg.WorkingDir(), sess, req, msgs)
	quickResult, err := c.runAutoGuardModel(ctx, model, providerCfg, string(autoClassifierFastPrompt), prompt, 8, 0.0)
	if err != nil {
		return permission.AutoClassification{}, err
	}
	if parseQuickClassifierDecision(quickResult) {
		return permission.AutoClassification{
			AllowAuto:  true,
			Reason:     "Quick classifier allowed this request.",
			Confidence: permission.AutoApprovalConfidenceMedium,
		}, nil
	}

	reasoningModel, reasoningProviderCfg, err := c.selectedModel(ctx, config.SelectedModelTypeAutoClassifierReasoning, false)
	if err != nil {
		return permission.AutoClassification{}, err
	}
	reasoningResult, err := c.runAutoGuardModel(ctx, reasoningModel, reasoningProviderCfg, string(autoClassifierReasoningPrompt), prompt, 512, 0.0)
	if err != nil {
		return permission.AutoClassification{}, err
	}
	return parseAutoClassification(reasoningResult)
}

func (c *coordinator) runAutoGuardModel(
	ctx context.Context,
	model Model,
	providerCfg config.ProviderConfig,
	systemPrompt string,
	prompt string,
	maxOutputTokens int64,
	temperature float64,
) (string, error) {
	agent := fantasy.NewAgent(
		model.Model,
		fantasy.WithSystemPrompt(systemPrompt),
		fantasy.WithMaxOutputTokens(maxOutputTokens),
		fantasy.WithUserAgent(userAgent),
	)

	resp, err := agent.Stream(copilot.ContextWithInitiatorType(ctx, copilot.InitiatorAgent), fantasy.AgentStreamCall{
		Prompt:          prompt,
		ProviderOptions: getProviderOptions(model, providerCfg),
		Temperature:     &temperature,
		PrepareStep: func(callCtx context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			callCtx = copilot.ContextWithInitiatorType(callCtx, copilot.InitiatorAgent)
			prepared.Messages = options.Messages
			if providerCfg.SystemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{
					fantasy.NewSystemMessage(providerCfg.SystemPromptPrefix),
				}, prepared.Messages...)
			}
			return callCtx, prepared, nil
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to classify permission request: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("auto classification returned no response")
	}
	return strings.TrimSpace(resp.Response.Content.Text()), nil
}

func buildAutoClassifierPrompt(cfg *config.Config, workingDir string, sess session.Session, req permission.PermissionRequest, msgs []message.Message) string {
	var sb strings.Builder
	sb.WriteString("Working directory:\n")
	sb.WriteString(strings.TrimSpace(workingDir))
	sb.WriteString("\n\nCollaboration mode:\n")
	sb.WriteString(string(sess.CollaborationMode))
	sb.WriteString("\n\nPermission mode:\n")
	sb.WriteString(string(sess.PermissionMode))
	sb.WriteString("\n\nPending permission request:\n")
	sb.WriteString("- tool: ")
	sb.WriteString(req.ToolName)
	sb.WriteString("\n- action: ")
	sb.WriteString(req.Action)
	sb.WriteString("\n- path: ")
	sb.WriteString(req.Path)
	sb.WriteString("\n- description: ")
	sb.WriteString(strings.TrimSpace(req.Description))
	sb.WriteString("\n- params: ")
	if data, err := json.Marshal(req.Params); err == nil {
		sb.Write(data)
	} else {
		sb.WriteString(fmt.Sprintf("%v", req.Params))
	}

	if cfg != nil && cfg.Permissions != nil && cfg.Permissions.AutoMode != nil {
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
		if len(cfg.Permissions.AutoMode.AllowExceptions) > 0 {
			sb.WriteString("\nAllow exceptions:\n")
			for _, item := range cfg.Permissions.AutoMode.AllowExceptions {
				sb.WriteString("- ")
				sb.WriteString(strings.TrimSpace(item))
				sb.WriteByte('\n')
			}
		}
	}

	sb.WriteString("\nRecent transcript:\n")
	for _, msg := range compactAutoClassifierMessages(msgs) {
		appendAutoClassifierMessage(&sb, msg)
	}

	return sb.String()
}

func compactAutoClassifierMessages(msgs []message.Message) []message.Message {
	const limit = 16

	filtered := make([]message.Message, 0, min(limit, len(msgs)))
	for i := len(msgs) - 1; i >= 0 && len(filtered) < limit; i-- {
		if msgs[i].IsSummaryMessage {
			continue
		}
		filtered = append(filtered, msgs[i])
	}
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	return filtered
}

func appendAutoClassifierMessage(sb *strings.Builder, msg message.Message) {
	switch msg.Role {
	case message.User:
		text := strings.TrimSpace(msg.Content().Text)
		if text == "" {
			return
		}
		sb.WriteString("- user: ")
		sb.WriteString(truncateAutoClassifierText(text, 500))
		sb.WriteByte('\n')
	case message.Assistant:
		for _, call := range msg.ToolCalls() {
			sb.WriteString("- tool_call ")
			sb.WriteString(call.Name)
			sb.WriteString(": ")
			sb.WriteString(truncateAutoClassifierText(call.Input, 300))
			sb.WriteByte('\n')
		}
	}
}

func truncateAutoClassifierText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

func parseQuickClassifierDecision(raw string) bool {
	raw = strings.TrimSpace(strings.ToUpper(raw))
	return raw == "ALLOW"
}

func parseAutoClassification(raw string) (permission.AutoClassification, error) {
	raw = strings.TrimSpace(raw)
	if matches := autoClassifierCodeFenceRegex.FindStringSubmatch(raw); len(matches) == 2 {
		raw = strings.TrimSpace(matches[1])
	}

	var payload autoClassifierResponse
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return permission.AutoClassification{}, fmt.Errorf("failed to parse auto classifier response: %w", err)
	}
	if payload.Confidence == "" {
		payload.Confidence = permission.AutoApprovalConfidenceLow
	}
	return permission.AutoClassification{
		AllowAuto:  payload.AllowAuto,
		Reason:     strings.TrimSpace(payload.Reason),
		Confidence: payload.Confidence,
	}, nil
}

func boolPtr(value bool) *bool {
	return &value
}
