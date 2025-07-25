package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/llm/prompt"
	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/lsp"
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
	Model() catwalk.Model
	Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	Summarize(ctx context.Context, sessionID string) error
	UpdateModel() error
}

type agent struct {
	*pubsub.Broker[AgentEvent]
	agentCfg config.Agent
	sessions session.Service
	messages message.Service

	tools *csync.LazySlice[tools.BaseTool]

	provider   provider.Provider
	providerID string

	titleProvider       provider.Provider
	summarizeProvider   provider.Provider
	summarizeProviderID string

	activeRequests sync.Map
}

var agentPromptMap = map[string]prompt.PromptID{
	"coder": prompt.PromptCoder,
	"task":  prompt.PromptTask,
}

func NewAgent(
	agentCfg config.Agent,
	// These services are needed in the tools
	permissions permission.Service,
	sessions session.Service,
	messages message.Service,
	history history.Service,
	lspClients map[string]*lsp.Client,
) (Service, error) {
	ctx := context.Background()
	cfg := config.Get()
	var agentTool tools.BaseTool
	if agentCfg.ID == "coder" {
		taskAgentCfg := config.Get().Agents["task"]
		if taskAgentCfg.ID == "" {
			return nil, fmt.Errorf("task agent not found in config")
		}
		taskAgent, err := NewAgent(taskAgentCfg, permissions, sessions, messages, history, lspClients)
		if err != nil {
			return nil, fmt.Errorf("failed to create task agent: %w", err)
		}

		agentTool = NewAgentTool(taskAgent, sessions, messages)
	}

	providerCfg := config.Get().GetProviderForModel(agentCfg.Model)
	if providerCfg == nil {
		return nil, fmt.Errorf("provider for agent %s not found in config", agentCfg.Name)
	}
	model := config.Get().GetModelByType(agentCfg.Model)

	if model == nil {
		return nil, fmt.Errorf("model not found for agent %s", agentCfg.Name)
	}

	promptID := agentPromptMap[agentCfg.ID]
	if promptID == "" {
		promptID = prompt.PromptDefault
	}
	opts := []provider.ProviderClientOption{
		provider.WithModel(agentCfg.Model),
		provider.WithSystemMessage(prompt.GetPrompt(promptID, providerCfg.ID, config.Get().Options.ContextPaths...)),
	}
	agentProvider, err := provider.NewProvider(*providerCfg, opts...)
	if err != nil {
		return nil, err
	}

	smallModelCfg := cfg.Models[config.SelectedModelTypeSmall]
	var smallModelProviderCfg *config.ProviderConfig
	if smallModelCfg.Provider == providerCfg.ID {
		smallModelProviderCfg = providerCfg
	} else {
		smallModelProviderCfg = cfg.GetProviderForModel(config.SelectedModelTypeSmall)

		if smallModelProviderCfg.ID == "" {
			return nil, fmt.Errorf("provider %s not found in config", smallModelCfg.Provider)
		}
	}
	smallModel := cfg.GetModelByType(config.SelectedModelTypeSmall)
	if smallModel.ID == "" {
		return nil, fmt.Errorf("model %s not found in provider %s", smallModelCfg.Model, smallModelProviderCfg.ID)
	}

	titleOpts := []provider.ProviderClientOption{
		provider.WithModel(config.SelectedModelTypeSmall),
		provider.WithSystemMessage(prompt.GetPrompt(prompt.PromptTitle, smallModelProviderCfg.ID)),
	}
	titleProvider, err := provider.NewProvider(*smallModelProviderCfg, titleOpts...)
	if err != nil {
		return nil, err
	}
	summarizeOpts := []provider.ProviderClientOption{
		provider.WithModel(config.SelectedModelTypeSmall),
		provider.WithSystemMessage(prompt.GetPrompt(prompt.PromptSummarizer, smallModelProviderCfg.ID)),
	}
	summarizeProvider, err := provider.NewProvider(*smallModelProviderCfg, summarizeOpts...)
	if err != nil {
		return nil, err
	}

	toolFn := func() []tools.BaseTool {
		slog.Info("Initializing agent tools", "agent", agentCfg.ID)
		defer func() {
			slog.Info("Initialized agent tools", "agent", agentCfg.ID)
		}()

		cwd := cfg.WorkingDir()
		allTools := []tools.BaseTool{
			tools.NewBashTool(permissions, cwd),
			tools.NewDownloadTool(permissions, cwd),
			tools.NewEditTool(lspClients, permissions, history, cwd),
			tools.NewMultiEditTool(lspClients, permissions, history, cwd),
			tools.NewFetchTool(permissions, cwd),
			tools.NewGlobTool(cwd),
			tools.NewGrepTool(cwd),
			tools.NewLsTool(cwd),
			tools.NewSourcegraphTool(),
			tools.NewViewTool(lspClients, cwd),
			tools.NewWriteTool(lspClients, permissions, history, cwd),
		}

		mcpTools := GetMCPTools(ctx, permissions, cfg)
		allTools = append(allTools, mcpTools...)

		if len(lspClients) > 0 {
			allTools = append(allTools, tools.NewDiagnosticsTool(lspClients))
		}

		if agentTool != nil {
			allTools = append(allTools, agentTool)
		}

		if agentCfg.AllowedTools == nil {
			return allTools
		}

		var filteredTools []tools.BaseTool
		for _, tool := range allTools {
			if slices.Contains(agentCfg.AllowedTools, tool.Name()) {
				filteredTools = append(filteredTools, tool)
			}
		}
		return filteredTools
	}

	return &agent{
		Broker:              pubsub.NewBroker[AgentEvent](),
		agentCfg:            agentCfg,
		provider:            agentProvider,
		providerID:          string(providerCfg.ID),
		messages:            messages,
		sessions:            sessions,
		titleProvider:       titleProvider,
		summarizeProvider:   summarizeProvider,
		summarizeProviderID: string(smallModelProviderCfg.ID),
		activeRequests:      sync.Map{},
		tools:               csync.NewLazySlice(toolFn),
	}, nil
}

func (a *agent) Model() catwalk.Model {
	return *config.Get().GetModelByType(a.agentCfg.Model)
}

func (a *agent) Cancel(sessionID string) {
	// Cancel regular requests
	if cancelFunc, exists := a.activeRequests.LoadAndDelete(sessionID); exists {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			slog.Info("Request cancellation initiated", "session_id", sessionID)
			cancel()
		}
	}

	// Also check for summarize requests
	if cancelFunc, exists := a.activeRequests.LoadAndDelete(sessionID + "-summarize"); exists {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			slog.Info("Summarize cancellation initiated", "session_id", sessionID)
			cancel()
		}
	}
}

func (a *agent) IsBusy() bool {
	busy := false
	a.activeRequests.Range(func(key, value any) bool {
		if cancelFunc, ok := value.(context.CancelFunc); ok {
			if cancelFunc != nil {
				busy = true
				return false
			}
		}
		return true
	})
	return busy
}

func (a *agent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Load(sessionID)
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

	// Use streaming approach like summarization
	response := a.titleProvider.StreamResponse(
		ctx,
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

	a.activeRequests.Store(sessionID, cancel)
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
		a.activeRequests.Delete(sessionID)
		cancel()
		a.Publish(pubsub.CreatedEvent, result)
		events <- result
		close(events)
	}()
	return events, nil
}

func (a *agent) processGeneration(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) AgentEvent {
	cfg := config.Get()
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
		agentMessage, toolResults, err := a.streamAndHandleEvents(ctx, sessionID, msgHistory)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				agentMessage.AddFinish(message.FinishReasonCanceled, "Request cancelled", "")
				a.messages.Update(context.Background(), agentMessage)
				return a.err(ErrRequestCancelled)
			}
			return a.err(fmt.Errorf("failed to process events: %w", err))
		}
		if cfg.Options.Debug {
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
	eventChan := a.provider.StreamResponse(ctx, msgHistory, slices.Collect(a.tools.Seq()))

	assistantMsg, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:     message.Assistant,
		Parts:    []message.ContentPart{},
		Model:    a.Model().ID,
		Provider: a.providerID,
	})
	if err != nil {
		return assistantMsg, nil, fmt.Errorf("failed to create assistant message: %w", err)
	}

	// Add the session and message ID into the context if needed by tools.
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, assistantMsg.ID)

	toolResults := sync.Map{}
	processedTool := make(map[string]bool)
	msg, err := a.messages.Create(context.Background(), assistantMsg.SessionID, message.CreateMessageParams{
		Role:     message.Tool,
		Parts:    []message.ContentPart{},
		Provider: a.providerID,
	})
	hasPermissionDenied := atomic.Bool{}
	wg := sync.WaitGroup{}

	// Helper function to handle cancellation
	handleCancellation := func(ctx context.Context, reason string) error {
		slog.Info("Request cancelled by user", "sessionID", sessionID, "reason", reason)
		for toolCallID, processed := range processedTool {
			_, ok := toolResults.Load(toolCallID)
			if !ok || !processed {
				toolResults.Store(toolCallID, message.ToolResult{
					ToolCallID: toolCallID,
					Content:    "Tool execution cancelled by user",
					IsError:    true,
				})
			}
		}
		parts := make([]message.ContentPart, 0)
		toolResults.Range(func(key, value any) bool {
			if tr, ok := value.(message.ToolResult); ok {
				parts = append(parts, tr)
			}
			return true
		})
		msg.Parts = parts
		if updateErr := a.messages.Update(context.Background(), msg); updateErr != nil {
			slog.Error("Failed to update tool message", "error", updateErr)
			a.finishMessage(ctx, &assistantMsg, message.FinishReasonError, "Failed to update tool message", updateErr.Error())
			return updateErr
		}
		a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled, "Request cancelled", "")
		return ctx.Err()
	}

	// Process each event in the stream.
	for event := range eventChan {
		if processErr := a.processEvent(ctx, sessionID, &assistantMsg, event); processErr != nil {
			if errors.Is(processErr, context.Canceled) {
				return assistantMsg, nil, handleCancellation(ctx, "process event")
			} else {
				a.finishMessage(ctx, &assistantMsg, message.FinishReasonError, "API Error", processErr.Error())
			}
			return assistantMsg, nil, processErr
		}
		if ctx.Err() != nil {
			return assistantMsg, nil, handleCancellation(ctx, "context cancelled during event processing")
		}

		for _, toolCall := range assistantMsg.ToolCalls() {
			if !toolCall.Finished {
				continue
			}
			if _, exists := processedTool[toolCall.ID]; exists {
				continue
			}
			processedTool[toolCall.ID] = false
			wg.Add(1)
			go func(toolCall message.ToolCall) {
				defer wg.Done()
				var tool tools.BaseTool
				defer func() { processedTool[toolCall.ID] = true }()
				for availableTool := range a.tools.Seq() {
					if availableTool.Info().Name == toolCall.Name {
						tool = availableTool
						break
					}
				}
				// Tool not found
				if tool == nil {
					toolResults.Store(toolCall.ID, message.ToolResult{
						ToolCallID: toolCall.ID,
						Content:    fmt.Sprintf("Tool not found: %s", toolCall.Name),
						IsError:    true,
					})
					return
				}
				response, err := tool.Run(ctx, tools.ToolCall{
					ID:    toolCall.ID,
					Name:  toolCall.Name,
					Input: toolCall.Input,
				})
				if ctx.Err() != nil {
					toolResults.Store(toolCall.ID, message.ToolResult{
						ToolCallID: toolCall.ID,
						Content:    "Tool execution canceled by user",
						IsError:    true,
					})
					return
				}
				if err != nil {
					slog.Error("Tool execution error", "toolCall", toolCall.ID, "error", err)
					if errors.Is(err, permission.ErrorPermissionDenied) {
						toolResults.Store(toolCall.ID, message.ToolResult{
							ToolCallID: toolCall.ID,
							Content:    "Permission denied",
							IsError:    true,
						})
						hasPermissionDenied.Store(true)
					}
				} else {
					toolResults.Store(toolCall.ID, message.ToolResult{
						ToolCallID: toolCall.ID,
						Content:    response.Content,
						Metadata:   response.Metadata,
						IsError:    response.IsError,
					})
				}
				parts := make([]message.ContentPart, 0)
				toolResults.Range(func(key, value any) bool {
					if tr, ok := value.(message.ToolResult); ok {
						parts = append(parts, tr)
					}
					return true
				})
				msg.Parts = parts
				if updateErr := a.messages.Update(ctx, msg); updateErr != nil {
					slog.Error("Failed to update tool message", "error", updateErr)
					a.finishMessage(ctx, &assistantMsg, message.FinishReasonError, "Failed to update tool message", updateErr.Error())
				}
			}(toolCall)
			break
		}
	}

	// We need this to be a go routine to avoid blocking so that if the context is cancelled we can
	// cancel the tool calls.
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-ctx.Done():
		return assistantMsg, nil, handleCancellation(ctx, "context cancelled during tool execution")
	case <-doneChan:
		slog.Debug("All tool calls processed", "sessionID", sessionID, "finished", assistantMsg.FinishReason())
		// Intentionally did not add a timeout here, users can forget that they started a request and it will sit
		// while permissions are requested.
	}

	if hasPermissionDenied.Load() {
		a.finishMessage(ctx, &assistantMsg, message.FinishReasonPermissionDenied, "Permission denied", "")
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
		return a.TrackUsage(ctx, sessionID, a.Model(), event.Response.Usage)
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
	a.activeRequests.Store(sessionID+"-summarize", cancel)

	go func() {
		defer a.activeRequests.Delete(sessionID + "-summarize")
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
		response := a.summarizeProvider.StreamResponse(
			summarizeCtx,
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
		shell := shell.GetPersistentShell(config.Get().WorkingDir())
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
			Model:    a.summarizeProvider.Model().ID,
			Provider: a.summarizeProviderID,
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
		model := a.summarizeProvider.Model()
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
	a.activeRequests.Range(func(key, value any) bool {
		a.Cancel(key.(string)) // key is sessionID
		return true
	})

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

func (a *agent) UpdateModel() error {
	cfg := config.Get()

	// Get current provider configuration
	currentProviderCfg := cfg.GetProviderForModel(a.agentCfg.Model)
	if currentProviderCfg == nil || currentProviderCfg.ID == "" {
		return fmt.Errorf("provider for agent %s not found in config", a.agentCfg.Name)
	}

	// Check if provider has changed
	if string(currentProviderCfg.ID) != a.providerID {
		// Provider changed, need to recreate the main provider
		model := cfg.GetModelByType(a.agentCfg.Model)
		if model.ID == "" {
			return fmt.Errorf("model not found for agent %s", a.agentCfg.Name)
		}

		promptID := agentPromptMap[a.agentCfg.ID]
		if promptID == "" {
			promptID = prompt.PromptDefault
		}

		opts := []provider.ProviderClientOption{
			provider.WithModel(a.agentCfg.Model),
			provider.WithSystemMessage(prompt.GetPrompt(promptID, currentProviderCfg.ID, cfg.Options.ContextPaths...)),
		}

		newProvider, err := provider.NewProvider(*currentProviderCfg, opts...)
		if err != nil {
			return fmt.Errorf("failed to create new provider: %w", err)
		}

		// Update the provider and provider ID
		a.provider = newProvider
		a.providerID = string(currentProviderCfg.ID)
	}

	// Check if small model provider has changed (affects title and summarize providers)
	smallModelCfg := cfg.Models[config.SelectedModelTypeSmall]
	var smallModelProviderCfg config.ProviderConfig

	for _, p := range cfg.Providers.Seq2() {
		if p.ID == smallModelCfg.Provider {
			smallModelProviderCfg = p
			break
		}
	}

	if smallModelProviderCfg.ID == "" {
		return fmt.Errorf("provider %s not found in config", smallModelCfg.Provider)
	}

	// Check if summarize provider has changed
	if string(smallModelProviderCfg.ID) != a.summarizeProviderID {
		smallModel := cfg.GetModelByType(config.SelectedModelTypeSmall)
		if smallModel == nil {
			return fmt.Errorf("model %s not found in provider %s", smallModelCfg.Model, smallModelProviderCfg.ID)
		}

		// Recreate title provider
		titleOpts := []provider.ProviderClientOption{
			provider.WithModel(config.SelectedModelTypeSmall),
			provider.WithSystemMessage(prompt.GetPrompt(prompt.PromptTitle, smallModelProviderCfg.ID)),
			// We want the title to be short, so we limit the max tokens
			provider.WithMaxTokens(40),
		}
		newTitleProvider, err := provider.NewProvider(smallModelProviderCfg, titleOpts...)
		if err != nil {
			return fmt.Errorf("failed to create new title provider: %w", err)
		}

		// Recreate summarize provider
		summarizeOpts := []provider.ProviderClientOption{
			provider.WithModel(config.SelectedModelTypeSmall),
			provider.WithSystemMessage(prompt.GetPrompt(prompt.PromptSummarizer, smallModelProviderCfg.ID)),
		}
		newSummarizeProvider, err := provider.NewProvider(smallModelProviderCfg, summarizeOpts...)
		if err != nil {
			return fmt.Errorf("failed to create new summarize provider: %w", err)
		}

		// Update the providers and provider ID
		a.titleProvider = newTitleProvider
		a.summarizeProvider = newSummarizeProvider
		a.summarizeProviderID = string(smallModelProviderCfg.ID)
	}

	return nil
}
