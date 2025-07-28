package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/llm/prompt"
	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
)

// Common errors
var (
	ErrRequestCancelled = errors.New("request canceled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
)

type AgentEventType string

const (
	AgentEventTypeError     AgentEventType = "error"
	AgentEventTypeResponse  AgentEventType = "response"
	AgentEventTypeSummarize AgentEventType = "summarize"
)

type AgentEvent struct {
	Type    AgentEventType
	Message message.Message
	Error   error

	// When summarizing
	SessionID string
	Progress  string
	Done      bool
}

type Service interface {
	pubsub.Suscriber[AgentEvent]
	Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	Summarize(ctx context.Context, sessionID string) error
	SetDebug(debug bool)
	UpdateModels(large, small Model) error
	// for now, not really sure how to handle this better
	WithAgentTool() error

	ModelConfig() Model
	Model() *catwalk.Model
	Provider() *provider.Config
}

type Model struct {
	// The model id as used by the provider API.
	// Required.
	Model string `json:"model"`
	// The model provider, same as the key/id used in the providers config.
	// Required.
	Provider string `json:"provider"`

	// Only used by models that use the openai provider and need this set.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`

	// Overrides the default model configuration.
	MaxTokens int64 `json:"max_tokens,omitempty"`

	// Used by anthropic models that can reason to indicate if the model should think.
	Think bool `json:"think,omitempty"`
}

type agent struct {
	*pubsub.Broker[AgentEvent]
	ctx          context.Context
	cwd          string
	systemPrompt string
	providers    map[string]provider.Config

	sessions session.Service
	messages message.Service

	toolsRegistry tools.Registry

	large, small      Model
	provider          provider.Provider
	titleProvider     provider.Provider
	summarizeProvider provider.Provider

	activeRequests *csync.Map[string, context.CancelFunc]

	debug bool
}

func NewAgent(
	ctx context.Context,
	cwd string,
	systemPrompt string,
	toolsRegistry tools.Registry,
	providers map[string]provider.Config,

	smallModel Model,
	largeModel Model,

	sessions session.Service,
	messages message.Service,
) (Service, error) {
	agent := &agent{
		Broker:         pubsub.NewBroker[AgentEvent](),
		ctx:            ctx,
		providers:      providers,
		cwd:            cwd,
		systemPrompt:   systemPrompt,
		toolsRegistry:  toolsRegistry,
		small:          smallModel,
		large:          largeModel,
		messages:       messages,
		sessions:       sessions,
		activeRequests: csync.NewMap[string, context.CancelFunc](),
	}

	err := agent.setProviders()
	return agent, err
}

func (a *agent) ModelConfig() Model {
	return a.large
}

func (a *agent) Model() *catwalk.Model {
	return a.provider.Model(a.large.Model)
}

func (a *agent) Provider() *provider.Config {
	for _, provider := range a.providers {
		if provider.ID == a.large.Provider {
			return &provider
		}
	}
	return nil
}

func (a *agent) Cancel(sessionID string) {
	// Cancel regular requests
	if cancel, exists := a.activeRequests.Take(sessionID); exists {
		slog.Info("Request cancellation initiated", "session_id", sessionID)
		cancel()
	}

	// Also check for summarize requests
	if cancel, exists := a.activeRequests.Take(sessionID + "-summarize"); exists {
		slog.Info("Summarize cancellation initiated", "session_id", sessionID)
		cancel()
	}
}

func (a *agent) IsBusy() bool {
	busy := false
	for cancelFunc := range a.activeRequests.Seq() {
		if cancelFunc != nil {
			busy = true
			break
		}
	}
	return busy
}

func (a *agent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Get(sessionID)
	return busy
}

func (a *agent) generateTitle(ctx context.Context, sessionID string, content string) error {
	if content == "" {
		return nil
	}
	if a.titleProvider == nil {
		return nil
	}
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	parts := []message.ContentPart{message.TextContent{
		Text: fmt.Sprintf("Generate a concise title for the following content:\n\n%s", content),
	}}

	response := a.titleProvider.Stream(
		ctx,
		a.small.Model,
		[]message.Message{
			{
				Role:  message.User,
				Parts: parts,
			},
		},
		nil,
	)

	var finalResponse *provider.ProviderResponse
	for r := range response {
		if r.Error != nil {
			return r.Error
		}
		finalResponse = r.Response
	}

	if finalResponse == nil {
		return fmt.Errorf("no response received from title provider")
	}

	title := strings.TrimSpace(strings.ReplaceAll(finalResponse.Content, "\n", " "))
	if title == "" {
		return nil
	}

	session.Title = title
	_, err = a.sessions.Save(ctx, session)
	return err
}

func (a *agent) err(err error) AgentEvent {
	return AgentEvent{
		Type:  AgentEventTypeError,
		Error: err,
	}
}

func (a *agent) Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error) {
	if !a.Model().SupportsImages && attachments != nil {
		attachments = nil
	}
	events := make(chan AgentEvent)
	if a.IsSessionBusy(sessionID) {
		return nil, ErrSessionBusy
	}

	genCtx, cancel := context.WithCancel(ctx)

	a.activeRequests.Set(sessionID, cancel)
	go func() {
		slog.Debug("Request started", "sessionID", sessionID)
		defer log.RecoverPanic("agent.Run", func() {
			events <- a.err(fmt.Errorf("panic while running the agent"))
		})
		var attachmentParts []message.ContentPart
		for _, attachment := range attachments {
			attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
		}
		result := a.processGeneration(genCtx, sessionID, content, attachmentParts)
		if result.Error != nil && !errors.Is(result.Error, ErrRequestCancelled) && !errors.Is(result.Error, context.Canceled) {
			slog.Error(result.Error.Error())
		}
		slog.Debug("Request completed", "sessionID", sessionID)
		a.activeRequests.Del(sessionID)
		cancel()
		a.Publish(pubsub.CreatedEvent, result)
		events <- result
		close(events)
	}()
	return events, nil
}

func (a *agent) processGeneration(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) AgentEvent {
	// List existing messages; if none, start title generation asynchronously.
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return a.err(fmt.Errorf("failed to list messages: %w", err))
	}
	if len(msgs) == 0 {
		go func() {
			defer log.RecoverPanic("agent.Run", func() {
				slog.Error("panic while generating title")
			})
			titleErr := a.generateTitle(context.Background(), sessionID, content)
			if titleErr != nil && !errors.Is(titleErr, context.Canceled) && !errors.Is(titleErr, context.DeadlineExceeded) {
				slog.Error("failed to generate title", "error", titleErr)
			}
		}()
	}
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return a.err(fmt.Errorf("failed to get session: %w", err))
	}
	if session.SummaryMessageID != "" {
		summaryMsgInex := -1
		for i, msg := range msgs {
			if msg.ID == session.SummaryMessageID {
				summaryMsgInex = i
				break
			}
		}
		if summaryMsgInex != -1 {
			msgs = msgs[summaryMsgInex:]
			msgs[0].Role = message.User
		}
	}

	userMsg, err := a.createUserMessage(ctx, sessionID, content, attachmentParts)
	if err != nil {
		return a.err(fmt.Errorf("failed to create user message: %w", err))
	}
	// Append the new user message to the conversation history.
	msgHistory := append(msgs, userMsg)

	for {
		// Check for cancellation before each iteration
		select {
		case <-ctx.Done():
			return a.err(ctx.Err())
		default:
			// Continue processing
		}
		agentMessage, toolResults, err := a.streamAndHandleEvents(ctx, sessionID, msgHistory)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				agentMessage.AddFinish(message.FinishReasonCanceled, "Request cancelled", "")
				a.messages.Update(context.Background(), agentMessage)
				return a.err(ErrRequestCancelled)
			}
			return a.err(fmt.Errorf("failed to process events: %w", err))
		}
		if a.debug {
			slog.Info("Result", "message", agentMessage.FinishReason(), "toolResults", toolResults)
		}
		if (agentMessage.FinishReason() == message.FinishReasonToolUse) && toolResults != nil {
			// We are not done, we need to respond with the tool response
			msgHistory = append(msgHistory, agentMessage, *toolResults)
			continue
		}
		return AgentEvent{
			Type:    AgentEventTypeResponse,
			Message: agentMessage,
			Done:    true,
		}
	}
}

func (a *agent) createUserMessage(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) (message.Message, error) {
	parts := []message.ContentPart{message.TextContent{Text: content}}
	parts = append(parts, attachmentParts...)
	return a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	})
}

func (a *agent) streamAndHandleEvents(ctx context.Context, sessionID string, msgHistory []message.Message) (message.Message, *message.Message, error) {
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
	eventChan := a.provider.Stream(ctx, a.large.Model, msgHistory, a.toolsRegistry.GetAllTools())

	assistantMsg, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:     message.Assistant,
		Parts:    []message.ContentPart{},
		Model:    a.large.Model,
		Provider: a.large.Provider,
	})
	if err != nil {
		return assistantMsg, nil, fmt.Errorf("failed to create assistant message: %w", err)
	}

	// Add the session and message ID into the context if needed by tools.
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, assistantMsg.ID)

	// Process each event in the stream.
	for event := range eventChan {
		if processErr := a.processEvent(ctx, sessionID, &assistantMsg, event); processErr != nil {
			if errors.Is(processErr, context.Canceled) {
				a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled, "Request cancelled", "")
			} else {
				a.finishMessage(ctx, &assistantMsg, message.FinishReasonError, "API Error", processErr.Error())
			}
			return assistantMsg, nil, processErr
		}
		if ctx.Err() != nil {
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled, "Request cancelled", "")
			return assistantMsg, nil, ctx.Err()
		}
	}

	toolResults := make([]message.ToolResult, len(assistantMsg.ToolCalls()))
	toolCalls := assistantMsg.ToolCalls()
	for i, toolCall := range toolCalls {
		select {
		case <-ctx.Done():
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled, "Request cancelled", "")
			// Make all future tool calls cancelled
			for j := i; j < len(toolCalls); j++ {
				toolResults[j] = message.ToolResult{
					ToolCallID: toolCalls[j].ID,
					Content:    "Tool execution canceled by user",
					IsError:    true,
				}
			}
			goto out
		default:
			// Continue processing
			var tool tools.BaseTool
			for _, availableTool := range a.toolsRegistry.GetAllTools() {
				if availableTool.Info().Name == toolCall.Name {
					tool = availableTool
					break
				}
			}

			// Tool not found
			if tool == nil {
				toolResults[i] = message.ToolResult{
					ToolCallID: toolCall.ID,
					Content:    fmt.Sprintf("Tool not found: %s", toolCall.Name),
					IsError:    true,
				}
				continue
			}

			// Run tool in goroutine to allow cancellation
			type toolExecResult struct {
				response tools.ToolResponse
				err      error
			}
			resultChan := make(chan toolExecResult, 1)

			go func() {
				response, err := tool.Run(ctx, tools.ToolCall{
					ID:    toolCall.ID,
					Name:  toolCall.Name,
					Input: toolCall.Input,
				})
				resultChan <- toolExecResult{response: response, err: err}
			}()

			var toolResponse tools.ToolResponse
			var toolErr error

			select {
			case <-ctx.Done():
				a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled, "Request cancelled", "")
				// Mark remaining tool calls as cancelled
				for j := i; j < len(toolCalls); j++ {
					toolResults[j] = message.ToolResult{
						ToolCallID: toolCalls[j].ID,
						Content:    "Tool execution canceled by user",
						IsError:    true,
					}
				}
				goto out
			case result := <-resultChan:
				toolResponse = result.response
				toolErr = result.err
			}

			if toolErr != nil {
				slog.Error("Tool execution error", "toolCall", toolCall.ID, "error", toolErr)
				if errors.Is(toolErr, permission.ErrorPermissionDenied) {
					toolResults[i] = message.ToolResult{
						ToolCallID: toolCall.ID,
						Content:    "Permission denied",
						IsError:    true,
					}
					for j := i + 1; j < len(toolCalls); j++ {
						toolResults[j] = message.ToolResult{
							ToolCallID: toolCalls[j].ID,
							Content:    "Tool execution canceled by user",
							IsError:    true,
						}
					}
					a.finishMessage(ctx, &assistantMsg, message.FinishReasonPermissionDenied, "Permission denied", "")
					break
				}
			}
			toolResults[i] = message.ToolResult{
				ToolCallID: toolCall.ID,
				Content:    toolResponse.Content,
				Metadata:   toolResponse.Metadata,
				IsError:    toolResponse.IsError,
			}
		}
	}
out:
	if len(toolResults) == 0 {
		return assistantMsg, nil, nil
	}
	parts := make([]message.ContentPart, 0)
	for _, tr := range toolResults {
		parts = append(parts, tr)
	}
	msg, err := a.messages.Create(context.Background(), assistantMsg.SessionID, message.CreateMessageParams{
		Role:     message.Tool,
		Parts:    parts,
		Model:    a.large.Model,
		Provider: a.large.Provider,
	})
	if err != nil {
		return assistantMsg, nil, fmt.Errorf("failed to create cancelled tool message: %w", err)
	}

	return assistantMsg, &msg, err
}

func (a *agent) finishMessage(ctx context.Context, msg *message.Message, finishReason message.FinishReason, message, details string) {
	msg.AddFinish(finishReason, message, details)
	_ = a.messages.Update(ctx, *msg)
}

func (a *agent) processEvent(ctx context.Context, sessionID string, assistantMsg *message.Message, event provider.ProviderEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue processing.
	}

	switch event.Type {
	case provider.EventThinkingDelta:
		assistantMsg.AppendReasoningContent(event.Thinking)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventSignatureDelta:
		assistantMsg.AppendReasoningSignature(event.Signature)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventContentDelta:
		assistantMsg.FinishThinking()
		assistantMsg.AppendContent(event.Content)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventToolUseStart:
		assistantMsg.FinishThinking()
		slog.Info("Tool call started", "toolCall", event.ToolCall)
		assistantMsg.AddToolCall(*event.ToolCall)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventToolUseDelta:
		assistantMsg.AppendToolCallInput(event.ToolCall.ID, event.ToolCall.Input)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventToolUseStop:
		slog.Info("Finished tool call", "toolCall", event.ToolCall)
		assistantMsg.FinishToolCall(event.ToolCall.ID)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventError:
		return event.Error
	case provider.EventComplete:
		assistantMsg.FinishThinking()
		assistantMsg.SetToolCalls(event.Response.ToolCalls)
		assistantMsg.AddFinish(event.Response.FinishReason, "", "")
		if err := a.messages.Update(ctx, *assistantMsg); err != nil {
			return fmt.Errorf("failed to update message: %w", err)
		}
		model := a.Model()
		if model == nil {
			return nil
		}
		return a.TrackUsage(ctx, sessionID, *model, event.Response.Usage)
	}

	return nil
}

func (a *agent) TrackUsage(ctx context.Context, sessionID string, model catwalk.Model, usage provider.TokenUsage) error {
	sess, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	cost := model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		model.CostPer1MIn/1e6*float64(usage.InputTokens) +
		model.CostPer1MOut/1e6*float64(usage.OutputTokens)

	sess.Cost += cost
	sess.CompletionTokens = usage.OutputTokens + usage.CacheReadTokens
	sess.PromptTokens = usage.InputTokens + usage.CacheCreationTokens

	_, err = a.sessions.Save(ctx, sess)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

func (a *agent) Summarize(ctx context.Context, sessionID string) error {
	if a.summarizeProvider == nil {
		return fmt.Errorf("summarize provider not available")
	}

	// Check if session is busy
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	// Create a new context with cancellation
	summarizeCtx, cancel := context.WithCancel(ctx)

	// Store the cancel function in activeRequests to allow cancellation
	a.activeRequests.Set(sessionID+"-summarize", cancel)

	go func() {
		defer a.activeRequests.Del(sessionID + "-summarize")
		defer cancel()
		event := AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Starting summarization...",
		}

		a.Publish(pubsub.CreatedEvent, event)
		// Get all messages from the session
		msgs, err := a.messages.List(summarizeCtx, sessionID)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to list messages: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		summarizeCtx = context.WithValue(summarizeCtx, tools.SessionIDContextKey, sessionID)

		if len(msgs) == 0 {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("no messages to summarize"),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}

		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Analyzing conversation...",
		}
		a.Publish(pubsub.CreatedEvent, event)

		// Add a system message to guide the summarization
		summarizePrompt := "Provide a detailed but concise summary of our conversation above. Focus on information that would be helpful for continuing the conversation, including what we did, what we're doing, which files we're working on, and what we're going to do next."

		// Create a new message with the summarize prompt
		promptMsg := message.Message{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: summarizePrompt}},
		}

		// Append the prompt to the messages
		msgsWithPrompt := append(msgs, promptMsg)

		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Generating summary...",
		}

		a.Publish(pubsub.CreatedEvent, event)

		// Send the messages to the summarize provider
		response := a.summarizeProvider.Stream(
			summarizeCtx,
			a.large.Model,
			msgsWithPrompt,
			nil,
		)
		var finalResponse *provider.ProviderResponse
		for r := range response {
			if r.Error != nil {
				event = AgentEvent{
					Type:  AgentEventTypeError,
					Error: fmt.Errorf("failed to summarize: %w", err),
					Done:  true,
				}
				a.Publish(pubsub.CreatedEvent, event)
				return
			}
			finalResponse = r.Response
		}

		summary := strings.TrimSpace(finalResponse.Content)
		if summary == "" {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("empty summary returned"),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		shell := shell.GetPersistentShell(a.cwd)
		summary += "\n\n**Current working directory of the persistent shell**\n\n" + shell.GetWorkingDir()
		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Creating new session...",
		}

		a.Publish(pubsub.CreatedEvent, event)
		oldSession, err := a.sessions.Get(summarizeCtx, sessionID)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to get session: %w", err),
				Done:  true,
			}

			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		// Create a message in the new session with the summary
		msg, err := a.messages.Create(summarizeCtx, oldSession.ID, message.CreateMessageParams{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: summary},
				message.Finish{
					Reason: message.FinishReasonEndTurn,
					Time:   time.Now().Unix(),
				},
			},
			Model:    a.large.Model,
			Provider: a.large.Provider,
		})
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to create summary message: %w", err),
				Done:  true,
			}

			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		oldSession.SummaryMessageID = msg.ID
		oldSession.CompletionTokens = finalResponse.Usage.OutputTokens
		oldSession.PromptTokens = 0
		model := a.summarizeProvider.Model(a.large.Model)
		usage := finalResponse.Usage
		cost := model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
			model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
			model.CostPer1MIn/1e6*float64(usage.InputTokens) +
			model.CostPer1MOut/1e6*float64(usage.OutputTokens)
		oldSession.Cost += cost
		_, err = a.sessions.Save(summarizeCtx, oldSession)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to save session: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
		}

		event = AgentEvent{
			Type:      AgentEventTypeSummarize,
			SessionID: oldSession.ID,
			Progress:  "Summary complete",
			Done:      true,
		}
		a.Publish(pubsub.CreatedEvent, event)
		// Send final success event with the new session ID
	}()

	return nil
}

func (a *agent) CancelAll() {
	if !a.IsBusy() {
		return
	}
	for key := range a.activeRequests.Seq2() {
		a.Cancel(key) // key is sessionID
	}

	timeout := time.After(5 * time.Second)
	for a.IsBusy() {
		select {
		case <-timeout:
			return
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (a *agent) UpdateModels(small, large Model) error {
	a.small = small
	a.large = large
	return a.setProviders()
}

func (a *agent) SetDebug(debug bool) {
	a.debug = debug
	if a.provider != nil {
		a.provider.SetDebug(debug)
	}
	if a.titleProvider != nil {
		a.titleProvider.SetDebug(debug)
	}
	if a.summarizeProvider != nil {
		a.summarizeProvider.SetDebug(debug)
	}
}

func (a *agent) setProviders() error {
	opts := []provider.Option{
		provider.WithSystemMessage(a.systemPrompt),
		provider.WithThinking(a.large.Think),
	}

	if a.large.MaxTokens > 0 {
		opts = append(opts, provider.WithMaxTokens(a.large.MaxTokens))
	}
	if a.large.ReasoningEffort != "" {
		opts = append(opts, provider.WithReasoningEffort(a.large.ReasoningEffort))
	}

	providerCfg, ok := a.providers[a.large.Provider]
	if !ok {
		return fmt.Errorf("provider %s not found in config", a.large.Provider)
	}
	var err error
	a.provider, err = provider.NewProvider(providerCfg, opts...)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	titleOpts := []provider.Option{
		provider.WithSystemMessage(prompt.TitlePrompt()),
		provider.WithMaxTokens(40),
	}

	titleProviderCfg, ok := a.providers[a.small.Provider]
	if !ok {
		return fmt.Errorf("small model provider %s not found in config", a.small.Provider)
	}

	a.titleProvider, err = provider.NewProvider(titleProviderCfg, titleOpts...)
	if err != nil {
		return err
	}
	summarizeOpts := []provider.Option{
		provider.WithSystemMessage(prompt.SummarizerPrompt()),
	}
	a.summarizeProvider, err = provider.NewProvider(providerCfg, summarizeOpts...)
	if err != nil {
		return err
	}

	if _, ok := a.toolsRegistry.GetTool(AgentToolName); ok {
		// reset the agent tool
		a.WithAgentTool()
	}

	a.SetDebug(a.debug)
	return nil
}

func (a *agent) WithAgentTool() error {
	agent, err := NewAgent(
		a.ctx,
		a.cwd,
		prompt.TaskPrompt(a.cwd),
		NewTaskTools(a.cwd),
		a.providers,
		a.small,
		a.large,
		a.sessions,
		a.messages,
	)
	if err != nil {
		return err
	}

	agentTool := NewAgentTool(
		agent,
		a.sessions,
		a.messages,
	)

	a.toolsRegistry.SetTool(AgentToolName, agentTool)
	return nil
}
