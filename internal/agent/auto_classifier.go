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

//go:embed templates/auto_classifier.md
var autoClassifierPrompt []byte

var autoClassifierCodeFenceRegex = regexp.MustCompile("(?s)^```(?:json)?\\s*(.*?)\\s*```$")

type autoClassifierResponse struct {
	AllowAuto  bool                              `json:"allow_auto"`
	Reason     string                            `json:"reason"`
	Confidence permission.AutoApprovalConfidence `json:"confidence"`
}

func (c *coordinator) ClassifyPermission(ctx context.Context, req permission.PermissionRequest) (permission.AutoClassification, error) {
	model, providerCfg, err := c.autoClassifierModel(ctx)
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

	maxOutputTokens := int64(512)
	if model.ModelCfg.MaxTokens > 0 && model.ModelCfg.MaxTokens < maxOutputTokens {
		maxOutputTokens = model.ModelCfg.MaxTokens
	}
	if model.CatwalkCfg.DefaultMaxTokens > 0 && model.CatwalkCfg.DefaultMaxTokens < maxOutputTokens {
		maxOutputTokens = model.CatwalkCfg.DefaultMaxTokens
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = 512
	}

	providerOptions, temperature, topP, topK, freqPenalty, presPenalty := mergeCallOptions(model, providerCfg)
	if temperature == nil {
		defaultTemperature := 0.0
		temperature = &defaultTemperature
	}

	agent := fantasy.NewAgent(
		model.Model,
		fantasy.WithSystemPrompt(string(autoClassifierPrompt)),
		fantasy.WithMaxOutputTokens(maxOutputTokens),
		fantasy.WithUserAgent(userAgent),
	)

	resp, err := agent.Stream(copilot.ContextWithInitiatorType(ctx, copilot.InitiatorAgent), fantasy.AgentStreamCall{
		Prompt:           buildAutoClassifierPrompt(c.cfg.WorkingDir(), sess.CollaborationMode, req, msgs),
		ProviderOptions:  providerOptions,
		Temperature:      temperature,
		TopP:             topP,
		TopK:             topK,
		FrequencyPenalty: freqPenalty,
		PresencePenalty:  presPenalty,
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
		return permission.AutoClassification{}, fmt.Errorf("failed to classify permission request: %w", err)
	}
	if resp == nil {
		return permission.AutoClassification{}, fmt.Errorf("auto classification returned no response")
	}

	return parseAutoClassification(resp.Response.Content.Text())
}

func (c *coordinator) autoClassifierModel(ctx context.Context) (Model, config.ProviderConfig, error) {
	selectedModel, ok := c.cfg.Config().Models[config.SelectedModelTypeAutoClassifier]
	if !ok {
		return Model{}, config.ProviderConfig{}, fmt.Errorf("model type %q not configured", config.SelectedModelTypeAutoClassifier)
	}

	providerCfg, ok := c.cfg.Config().Providers.Get(selectedModel.Provider)
	if !ok {
		return Model{}, config.ProviderConfig{}, errModelProviderNotConfigured
	}

	catwalkModel, err := c.lookupCatwalkModel(selectedModel)
	if err != nil {
		return Model{}, config.ProviderConfig{}, err
	}

	thinkingDisabled := true
	provider, err := c.buildProvider(providerCfg, catwalkModel, false, thinkingDisabled)
	if err != nil {
		return Model{}, config.ProviderConfig{}, err
	}

	modelID := selectedModel.Model
	if selectedModel.Provider == "openrouter" && isExactoSupported(modelID) {
		modelID += ":exacto"
	}

	languageModel, err := provider.LanguageModel(ctx, modelID)
	if err != nil {
		return Model{}, config.ProviderConfig{}, err
	}

	selectedModel.Think = boolPtr(false)

	return Model{
		Model:      languageModel,
		CatwalkCfg: catwalkModel,
		ModelCfg:   selectedModel,
	}, providerCfg, nil
}

func buildAutoClassifierPrompt(workingDir string, mode session.CollaborationMode, req permission.PermissionRequest, msgs []message.Message) string {
	var sb strings.Builder
	sb.WriteString("Working directory:\n")
	sb.WriteString(strings.TrimSpace(workingDir))
	sb.WriteString("\n\nCollaboration mode:\n")
	sb.WriteString(string(mode))
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

	sb.WriteString("\n\nRecent transcript:\n")
	for _, msg := range compactAutoClassifierMessages(msgs) {
		appendAutoClassifierMessage(&sb, msg)
	}

	return sb.String()
}

func compactAutoClassifierMessages(msgs []message.Message) []message.Message {
	const limit = 12

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
		for _, result := range msg.ToolResults() {
			sb.WriteString("- tool_result ")
			sb.WriteString(result.Name)
			if result.IsError {
				sb.WriteString(" (error)")
			}
			sb.WriteString(": ")
			sb.WriteString(truncateAutoClassifierText(result.Content, 300))
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
