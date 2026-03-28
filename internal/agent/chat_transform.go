package agent

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/plugin"
	"github.com/charmbracelet/crush/internal/session"
)

type sessionCompactingPurposeContextKey struct{}

type chatRequestState struct {
	Messages     []message.Message
	History      []fantasy.Message
	Files        []fantasy.FilePart
	SystemPrompt string
	PromptPrefix string
}

type chatRequestStateInput struct {
	SessionID    string
	Agent        string
	Model        Model
	Provider     plugin.ProviderContext
	Purpose      plugin.ChatTransformPurpose
	Messages     []message.Message
	Message      message.Message
	Attachments  []message.Attachment
	SystemPrompt string
	PromptPrefix string
	PermissionMode session.PermissionMode
}

func withSessionCompactingPurpose(ctx context.Context, purpose plugin.ChatTransformPurpose) context.Context {
	if purpose == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionCompactingPurposeContextKey{}, purpose)
}

func sessionCompactingPurposeFromContext(ctx context.Context) plugin.ChatTransformPurpose {
	purpose, ok := ctx.Value(sessionCompactingPurposeContextKey{}).(plugin.ChatTransformPurpose)
	if !ok || purpose == "" {
		return plugin.ChatTransformPurposeSummarize
	}
	return purpose
}

func cloneMessages(msgs []message.Message) []message.Message {
	cloned := make([]message.Message, len(msgs))
	for i := range msgs {
		cloned[i] = msgs[i].Clone()
	}
	return cloned
}

func agentModelInfo(model Model) plugin.ModelInfo {
	return plugin.ModelInfo{
		ProviderID:    model.ModelCfg.Provider,
		ModelID:       model.ModelCfg.Model,
		ContextWindow: int64(model.CatwalkCfg.ContextWindow),
	}
}

func defaultProviderContext() plugin.ProviderContext {
	return plugin.ProviderContext{Source: "config", Options: map[string]any{}}
}

func transientUserMessage(sessionID, prompt string, attachments []message.Attachment) message.Message {
	parts := []message.ContentPart{message.TextContent{Text: prompt}}
	for _, attachment := range attachments {
		parts = append(parts, message.BinaryContent{
			Path:     attachment.FilePath,
			MIMEType: attachment.MimeType,
			Data:     attachment.Content,
		})
	}
	return message.Message{
		SessionID: sessionID,
		Role:      message.User,
		Parts:     parts,
	}
}

func joinSystemSections(sections []string) string {
	filtered := make([]string, 0, len(sections))
	for _, section := range sections {
		if strings.TrimSpace(section) == "" {
			continue
		}
		filtered = append(filtered, section)
	}
	return strings.Join(filtered, "\n")
}

func (a *sessionAgent) transformSessionMessages(ctx context.Context, input chatRequestStateInput) ([]message.Message, error) {
	transformed, err := plugin.TriggerChatMessagesTransform(ctx, plugin.ChatMessagesTransformInput{
		SessionID: input.SessionID,
		Agent:     input.Agent,
		Model:     agentModelInfo(input.Model),
		Provider:  input.Provider,
		Purpose:   input.Purpose,
		Message:   input.Message,
	}, plugin.ChatMessagesTransformOutput{Messages: cloneMessages(input.Messages)})
	if err != nil {
		return nil, err
	}
	return transformed.Messages, nil
}

func (a *sessionAgent) transformSystemPrompt(ctx context.Context, input chatRequestStateInput) (string, string, error) {
	transformed, err := plugin.TriggerChatSystemTransform(ctx, plugin.ChatSystemTransformInput{
		SessionID: input.SessionID,
		Agent:     input.Agent,
		Model:     agentModelInfo(input.Model),
		Provider:  input.Provider,
		Purpose:   input.Purpose,
		Message:   input.Message,
	}, plugin.ChatSystemTransformOutput{System: []string{input.SystemPrompt}, Prefix: input.PromptPrefix})
	if err != nil {
		return "", "", err
	}
	return joinSystemSections(transformed.System), transformed.Prefix, nil
}

func (a *sessionAgent) buildChatRequestState(ctx context.Context, input chatRequestStateInput) (chatRequestState, error) {
	transformedMessages, err := a.transformSessionMessages(ctx, input)
	if err != nil {
		return chatRequestState{}, err
	}
	systemPrompt, promptPrefix, err := a.transformSystemPrompt(ctx, input)
	if err != nil {
		return chatRequestState{}, err
	}
	if autoModePrompt, ok := pendingAutoModePromptText(transformedMessages, input.PermissionMode); ok {
		systemPrompt = joinSystemSections([]string{systemPrompt, autoModePrompt})
	}
	history, files := a.preparePrompt(transformedMessages, input.Attachments...)
	return chatRequestState{
		Messages:     transformedMessages,
		History:      history,
		Files:        files,
		SystemPrompt: systemPrompt,
		PromptPrefix: promptPrefix,
	}, nil
}

func buildSessionCompactingPrompt(todos []session.Todo, extraContext []string, promptOverride string) string {
	base := buildSummaryPrompt(todos)
	if promptOverride != "" {
		base = promptOverride
	}
	if len(extraContext) == 0 {
		return base
	}

	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n\n## Additional Context\n\n")
	for _, item := range extraContext {
		fmt.Fprintf(&sb, "- %s\n", item)
	}
	return sb.String()
}
