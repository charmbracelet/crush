package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/coder/acp-go-sdk"
)

// Agent implements the acp.Agent interface to handle ACP protocol methods.
type Agent struct {
	app   *app.App
	conn  *acp.AgentSideConnection
	sinks *csync.Map[string, *Sink]
}

// Compile-time interface checks.
var (
	_ acp.Agent             = (*Agent)(nil)
	_ acp.AgentLoader       = (*Agent)(nil)
	_ acp.AgentExperimental = (*Agent)(nil)
)

// NewAgent creates a new ACP agent backed by a Crush app instance.
func NewAgent(app *app.App) *Agent {
	return &Agent{
		app:   app,
		sinks: csync.NewMap[string, *Sink](),
	}
}

// SetAgentConnection stores the connection for sending notifications.
func (a *Agent) SetAgentConnection(conn *acp.AgentSideConnection) {
	a.conn = conn
}

// Initialize handles the ACP initialize request.
func (a *Agent) Initialize(ctx context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	slog.Debug("ACP Initialize", "protocol_version", params.ProtocolVersion)
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: true,
			McpCapabilities: acp.McpCapabilities{
				Http: false,
				Sse:  false,
			},
			PromptCapabilities: acp.PromptCapabilities{
				EmbeddedContext: true,
				Audio:           false,
				Image:           false,
			},
		},
	}, nil
}

// Authenticate handles authentication requests (stub for local stdio).
func (a *Agent) Authenticate(ctx context.Context, params acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	slog.Debug("ACP Authenticate")
	return acp.AuthenticateResponse{}, nil
}

// NewSession creates a new Crush session.
func (a *Agent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	slog.Info("ACP NewSession", "cwd", params.Cwd)

	sess, err := a.app.Sessions.Create(ctx, "ACP Session")
	if err != nil {
		return acp.NewSessionResponse{}, err
	}

	// Create and start the event sink to stream updates to this session.
	// Use a background context since the sink needs to outlive the NewSession
	// request.
	sink := NewSink(context.Background(), a.conn, sess.ID)
	sink.Start(a.app.Messages, a.app.Permissions, a.app.Sessions)
	a.sinks.Set(sess.ID, sink)

	return acp.NewSessionResponse{
		SessionId: acp.SessionId(sess.ID),
		Models:    a.buildSessionModelState(),
	}, nil
}

// LoadSession loads an existing session to resume a previous conversation.
func (a *Agent) LoadSession(ctx context.Context, params acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	sessionID := string(params.SessionId)
	slog.Info("ACP LoadSession", "session_id", sessionID)

	// Verify the session exists.
	session, err := a.app.Sessions.Get(ctx, sessionID)
	if err != nil {
		return acp.LoadSessionResponse{}, err
	}

	// Create and start the event sink for future updates.
	sink := NewSink(context.Background(), a.conn, session.ID)
	sink.Start(a.app.Messages, a.app.Permissions, a.app.Sessions)
	a.sinks.Set(session.ID, sink)

	// Load and replay historical messages to the client.
	messages, err := a.app.Messages.List(ctx, sessionID)
	if err != nil {
		return acp.LoadSessionResponse{}, err
	}

	for _, msg := range messages {
		if err := a.replayMessage(ctx, sessionID, msg); err != nil {
			slog.Error("Failed to replay message", "message_id", msg.ID, "error", err)
		}
	}

	return acp.LoadSessionResponse{
		Models: a.buildSessionModelState(),
	}, nil
}

// SetSessionMode handles mode switching (stub - Crush doesn't have modes yet).
func (a *Agent) SetSessionMode(ctx context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	slog.Debug("ACP SetSessionMode", "mode_id", params.ModeId)
	return acp.SetSessionModeResponse{}, nil
}

// SetSessionModel handles model switching by parsing the model ID and updating
// the agent's active model.
func (a *Agent) SetSessionModel(ctx context.Context, params acp.SetSessionModelRequest) (acp.SetSessionModelResponse, error) {
	slog.Info("ACP SetSessionModel", "session_id", params.SessionId, "model_id", params.ModelId)

	// Parse model ID (format: "provider:model").
	parts := strings.SplitN(string(params.ModelId), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return acp.SetSessionModelResponse{}, fmt.Errorf("invalid model ID format %q: expected provider:model", params.ModelId)
	}
	providerID, modelID := parts[0], parts[1]

	// Validate that the model exists.
	cfg := config.Get()
	if cfg.GetModel(providerID, modelID) == nil {
		return acp.SetSessionModelResponse{}, fmt.Errorf("model %q not found for provider %q", modelID, providerID)
	}

	// Check if the agent is busy.
	if a.app.AgentCoordinator.IsBusy() {
		return acp.SetSessionModelResponse{}, fmt.Errorf("agent is busy, cannot switch models")
	}

	// Update the preferred model in config.
	selectedModel := config.SelectedModel{
		Provider: providerID,
		Model:    modelID,
	}
	if err := cfg.UpdatePreferredModel(config.SelectedModelTypeLarge, selectedModel); err != nil {
		return acp.SetSessionModelResponse{}, fmt.Errorf("failed to update preferred model: %w", err)
	}

	// Apply the model change to the agent.
	if err := a.app.UpdateAgentModel(ctx); err != nil {
		return acp.SetSessionModelResponse{}, fmt.Errorf("failed to apply model change: %w", err)
	}

	slog.Info("ACP SetSessionModel completed", "provider", providerID, "model", modelID)
	return acp.SetSessionModelResponse{}, nil
}

// Prompt handles a prompt request by running the agent.
func (a *Agent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	slog.Info("ACP Prompt", "session_id", params.SessionId)

	// Extract text from content blocks.
	var prompt string
	for _, block := range params.Prompt {
		if block.Text != nil {
			prompt += block.Text.Text
		}
	}

	if prompt == "" {
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	}

	// Check for slash commands before sending to the agent.
	if strings.HasPrefix(prompt, "/") {
		if resp, handled := a.handleCommand(ctx, string(params.SessionId), prompt); handled {
			return resp, nil
		}
	}

	// Run the agent.
	result, err := a.app.AgentCoordinator.Run(ctx, string(params.SessionId), prompt)
	if err != nil {
		// Permission denial is a normal user choice, not an error.
		if errors.Is(err, permission.ErrorPermissionDenied) {
			return acp.PromptResponse{StopReason: acp.StopReasonRefusal}, nil
		}
		// Context cancellation means the user cancelled the request.
		if errors.Is(err, context.Canceled) {
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
		}
		// Other errors are actual errors.
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, err
	}

	// Map the agent's finish reason to an ACP stop reason.
	if result != nil && result.Response.FinishReason == fantasy.FinishReasonLength {
		return acp.PromptResponse{StopReason: acp.StopReasonMaxTokens}, nil
	}

	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

// Cancel handles cancellation of an in-flight prompt.
func (a *Agent) Cancel(ctx context.Context, params acp.CancelNotification) error {
	slog.Info("ACP Cancel", "session_id", params.SessionId)
	a.app.AgentCoordinator.Cancel(string(params.SessionId))
	return nil
}

// handleCommand checks if the prompt is a slash command and handles it.
// Returns the response and true if handled, otherwise returns an empty
// response and false.
func (a *Agent) handleCommand(ctx context.Context, sessionID, prompt string) (acp.PromptResponse, bool) {
	// Parse command name and args: "/command arg1 arg2".
	parts := strings.Fields(prompt)
	if len(parts) == 0 {
		return acp.PromptResponse{}, false
	}

	cmd := strings.TrimPrefix(parts[0], "/")
	args := parts[1:]

	var response string
	var err error

	switch cmd {
	case "toggle_yolo":
		response = a.cmdToggleYolo()
	case "toggle_thinking":
		response, err = a.cmdToggleThinking(ctx)
	case "set_reasoning_effort":
		response, err = a.cmdSetReasoningEffort(ctx, args)
	case "summarize":
		response, err = a.cmdSummarize(ctx, sessionID)
	default:
		// Not a recognized command; pass through to agent.
		return acp.PromptResponse{}, false
	}

	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	// Send the response as an agent text message.
	a.sendCommandResponse(ctx, sessionID, response)

	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, true
}

// sendCommandResponse sends a text response for a command to the ACP client.
func (a *Agent) sendCommandResponse(ctx context.Context, sessionID, text string) {
	update := acp.UpdateAgentMessageText(text)
	if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: acp.SessionId(sessionID),
		Update:    update,
	}); err != nil {
		slog.Error("Failed to send command response", "error", err)
	}
}

// cmdToggleYolo toggles auto-approve mode for tool calls.
func (a *Agent) cmdToggleYolo() string {
	current := a.app.Permissions.SkipRequests()
	a.app.Permissions.SetSkipRequests(!current)
	if !current {
		return "YOLO mode enabled: tool calls will be auto-approved."
	}
	return "YOLO mode disabled: tool calls will require approval."
}

// cmdToggleThinking toggles thinking mode for Anthropic/Hyper reasoning models.
func (a *Agent) cmdToggleThinking(ctx context.Context) (string, error) {
	cfg := config.Get()
	agentCfg := cfg.Agents[config.AgentCoder]

	// Validate that the current model supports thinking toggle.
	providerCfg := cfg.GetProviderForModel(agentCfg.Model)
	model := cfg.GetModelByType(agentCfg.Model)
	if providerCfg == nil || model == nil {
		return "", fmt.Errorf("could not determine current model configuration")
	}

	if !model.CanReason {
		return "", fmt.Errorf("current model does not support reasoning")
	}

	// Thinking toggle is only for Anthropic/Hyper models.
	if providerCfg.Type != catwalk.TypeAnthropic && providerCfg.Type != catwalk.Type(hyper.Name) {
		return "", fmt.Errorf("toggle_thinking is only supported for Anthropic models; use /set_reasoning_effort for other providers")
	}

	currentModel := cfg.Models[agentCfg.Model]
	currentModel.Think = !currentModel.Think

	if err := cfg.UpdatePreferredModel(agentCfg.Model, currentModel); err != nil {
		return "", fmt.Errorf("failed to update model config: %w", err)
	}

	// Apply the change to the agent.
	if err := a.app.UpdateAgentModel(ctx); err != nil {
		return "", fmt.Errorf("failed to apply model change: %w", err)
	}

	if currentModel.Think {
		return "Extended thinking enabled.", nil
	}
	return "Extended thinking disabled.", nil
}

// cmdSummarize triggers session summarization.
func (a *Agent) cmdSummarize(ctx context.Context, sessionID string) (string, error) {
	if err := a.app.AgentCoordinator.Summarize(ctx, sessionID); err != nil {
		return "", fmt.Errorf("summarization failed: %w", err)
	}
	return "Session summarized successfully.", nil
}

// cmdSetReasoningEffort sets the reasoning effort level for OpenAI-style models.
func (a *Agent) cmdSetReasoningEffort(ctx context.Context, args []string) (string, error) {
	cfg := config.Get()
	agentCfg := cfg.Agents[config.AgentCoder]
	model := cfg.GetModelByType(agentCfg.Model)

	if model == nil || len(model.ReasoningLevels) == 0 {
		return "", fmt.Errorf("current model does not support reasoning effort levels")
	}

	if len(args) == 0 {
		currentModel := cfg.Models[agentCfg.Model]
		current := currentModel.ReasoningEffort
		if current == "" {
			current = "default"
		}
		return fmt.Sprintf("Current reasoning effort: %s\nAvailable levels: %s",
			current, strings.Join(model.ReasoningLevels, ", ")), nil
	}

	effort := strings.ToLower(args[0])

	// Validate the effort level.
	valid := false
	for _, level := range model.ReasoningLevels {
		if strings.EqualFold(level, effort) {
			effort = level // Use the canonical casing.
			valid = true
			break
		}
	}
	if !valid {
		return "", fmt.Errorf("invalid reasoning effort %q; valid levels: %s",
			effort, strings.Join(model.ReasoningLevels, ", "))
	}

	currentModel := cfg.Models[agentCfg.Model]
	currentModel.ReasoningEffort = effort

	if err := cfg.UpdatePreferredModel(agentCfg.Model, currentModel); err != nil {
		return "", fmt.Errorf("failed to update model config: %w", err)
	}

	if err := a.app.UpdateAgentModel(ctx); err != nil {
		return "", fmt.Errorf("failed to apply model change: %w", err)
	}

	return fmt.Sprintf("Reasoning effort set to %q.", effort), nil
}

// replayMessage sends a historical message to the client via session updates.
func (a *Agent) replayMessage(ctx context.Context, sessionID string, msg message.Message) error {
	for _, part := range msg.Parts {
		update := a.translateHistoryPart(msg.Role, part)
		if update == nil {
			continue
		}

		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: acp.SessionId(sessionID),
			Update:    *update,
		}); err != nil {
			return err
		}
	}
	return nil
}

// translateHistoryPart converts a message part to an ACP session update for
// history replay. Unlike streaming updates, this sends full content rather
// than deltas.
func (a *Agent) translateHistoryPart(role message.MessageRole, part message.ContentPart) *acp.SessionUpdate {
	switch p := part.(type) {
	case message.TextContent:
		if p.Text == "" {
			return nil
		}
		var update acp.SessionUpdate
		if role == message.User {
			update = acp.UpdateUserMessageText(p.Text)
		} else {
			update = acp.UpdateAgentMessageText(p.Text)
		}
		return &update

	case message.ReasoningContent:
		if p.Thinking == "" {
			return nil
		}
		update := acp.UpdateAgentThoughtText(p.Thinking)
		return &update

	case message.ToolCall:
		// For history replay, send the tool call as completed with full input.
		opts := []acp.ToolCallStartOpt{
			acp.WithStartStatus(acp.ToolCallStatusCompleted),
			acp.WithStartKind(toolKind(p.Name)),
		}
		if input := parseToolInput(p.Input); input != nil {
			if input.Path != "" {
				opts = append(opts, acp.WithStartLocations([]acp.ToolCallLocation{{Path: input.Path}}))
			}
			opts = append(opts, acp.WithStartRawInput(input.Raw))
		}
		title := p.Name
		if input := parseToolInput(p.Input); input != nil && input.Title != "" {
			title = input.Title
		}
		update := acp.StartToolCall(acp.ToolCallId(p.ID), title, opts...)
		return &update

	case message.ToolResult:
		status := acp.ToolCallStatusCompleted
		if p.IsError {
			status = acp.ToolCallStatusFailed
		}
		content := []acp.ToolCallContent{acp.ToolContent(acp.TextBlock(p.Content))}
		update := acp.UpdateToolCall(
			acp.ToolCallId(p.ToolCallID),
			acp.WithUpdateStatus(status),
			acp.WithUpdateContent(content),
		)
		return &update

	default:
		return nil
	}
}

// buildSessionModelState constructs the model state for session responses,
// listing all available models and the currently selected one.
func (a *Agent) buildSessionModelState() *acp.SessionModelState {
	cfg := config.Get()
	if cfg == nil {
		return nil
	}

	var availableModels []acp.ModelInfo
	for providerID, providerConfig := range cfg.Providers.Seq2() {
		if providerConfig.Disable {
			continue
		}
		providerName := providerConfig.Name
		if providerName == "" {
			providerName = providerID
		}
		for _, model := range providerConfig.Models {
			modelID := acp.ModelId(providerID + ":" + model.ID)
			modelName := model.Name
			if modelName == "" {
				modelName = model.ID
			}
			availableModels = append(availableModels, acp.ModelInfo{
				ModelId: modelID,
				Name:    providerName + " / " + modelName,
			})
		}
	}

	// Get current model.
	currentModel := cfg.Models[config.SelectedModelTypeLarge]
	currentModelID := acp.ModelId(currentModel.Provider + ":" + currentModel.Model)

	return &acp.SessionModelState{
		AvailableModels: availableModels,
		CurrentModelId:  currentModelID,
	}
}
