// Package agent is the core orchestration layer for Crush AI agents.
//
// It provides session-based AI agent functionality for managing
// conversations, tool execution, and message handling. It coordinates
// interactions between language models, messages, sessions, and tools while
// handling features like automatic summarization, queuing, and token
// management.
package agent

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openrouter"
	"charm.land/fantasy/providers/vercel"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/checkpoint"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/plugin"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/x/exp/charmtone"
)

const (
	DefaultSessionName = "Untitled Session"

	// Constants for auto-summarization thresholds
	autoSummarizeReserveTokens        = 20_000
	autoSummarizeToolReserveMax       = 8_000
	autoSummarizeToolReserveMin       = 2_000
	autoSummarizeSafetyReserveMin     = 2_000
	autoSummarizeSoftLimitNumerator   = 9
	autoSummarizeSoftLimitDenominator = 10

	joinActiveRunMaxInjectedCalls  = 2
	joinActiveRunPromptCharsBudget = 1_600
)

var userAgent = fmt.Sprintf("Charm-Crush/%s (https://charm.land/crush)", version.Version)

//go:embed templates/title.md
var titlePrompt []byte

//go:embed templates/summary.md
var summaryPrompt []byte

// Used to remove <think> tags from generated titles.
var thinkTagRegex = regexp.MustCompile(`(?s)<think>.*?</think>`)

const autoResumePromptPrefix = "The previous session was interrupted because it got too long, the initial user request was: `"

type sessionCompactionTrigger string

const (
	sessionCompactionTriggerNone      sessionCompactionTrigger = ""
	sessionCompactionTriggerNormal    sessionCompactionTrigger = "normal_summarize"
	sessionCompactionTriggerRecover   sessionCompactionTrigger = "recover_summarize"
	sessionCompactionTriggerProactive sessionCompactionTrigger = "proactive_compact"
)

func (t sessionCompactionTrigger) Purpose() plugin.ChatTransformPurpose {
	switch t {
	case sessionCompactionTriggerRecover:
		return plugin.ChatTransformPurposeRecover
	case sessionCompactionTriggerProactive:
		return plugin.ChatTransformPurposeProactiveCompact
	default:
		return plugin.ChatTransformPurposeSummarize
	}
}

func proactiveCompactionTrigger(contextUsed, contextWindow, maxOutputTokens int64) sessionCompactionTrigger {
	if shouldAutoSummarize(contextUsed, contextWindow, maxOutputTokens) {
		return sessionCompactionTriggerProactive
	}
	return sessionCompactionTriggerNone
}

func shouldCollapseMessages(purpose plugin.ChatTransformPurpose) bool {
	switch purpose {
	case plugin.ChatTransformPurposeSummarize,
		plugin.ChatTransformPurposeRecover,
		plugin.ChatTransformPurposeProactiveCompact:
		return true
	default:
		return false
	}
}

func shouldReactiveCompactMessages(purpose plugin.ChatTransformPurpose) bool {
	switch purpose {
	case plugin.ChatTransformPurposeRecover,
		plugin.ChatTransformPurposeProactiveCompact:
		return true
	default:
		return false
	}
}

func shouldAutoCompactMessages(purpose plugin.ChatTransformPurpose, msgs []message.Message) bool {
	if len(msgs) < 3 {
		return false
	}
	switch purpose {
	case plugin.ChatTransformPurposeSummarize,
		plugin.ChatTransformPurposeRecover,
		plugin.ChatTransformPurposeProactiveCompact:
		return true
	default:
		return false
	}
}

func (a *sessionAgent) microCompactSessionMessages(ctx context.Context, sessionID string, model Model, providerCtx plugin.ProviderContext, msgs []message.Message) ([]message.Message, error) {
	result, err := a.transformSessionMessages(ctx, chatRequestStateInput{
		SessionID: sessionID,
		Agent:     "session",
		Model:     model,
		Provider:  providerCtx,
		Purpose:   plugin.ChatTransformPurposeMicroCompact,
		Messages:  msgs,
		Message:   message.Message{SessionID: sessionID, Role: message.User},
	})
	if err != nil {
		return nil, err
	}
	return builtinMicroCompactMessages(result), nil
}

func (a *sessionAgent) collapseSessionMessages(ctx context.Context, sessionID string, model Model, providerCtx plugin.ProviderContext, msgs []message.Message) ([]message.Message, error) {
	return a.transformSessionMessages(ctx, chatRequestStateInput{
		SessionID: sessionID,
		Agent:     "session",
		Model:     model,
		Provider:  providerCtx,
		Purpose:   plugin.ChatTransformPurposeCollapse,
		Messages:  msgs,
		Message:   message.Message{SessionID: sessionID, Role: message.User},
	})
}

func (a *sessionAgent) reactiveCompactSessionMessages(ctx context.Context, sessionID string, model Model, providerCtx plugin.ProviderContext, msgs []message.Message) ([]message.Message, error) {
	return a.transformSessionMessages(ctx, chatRequestStateInput{
		SessionID: sessionID,
		Agent:     "session",
		Model:     model,
		Provider:  providerCtx,
		Purpose:   plugin.ChatTransformPurposeReactiveCompact,
		Messages:  msgs,
		Message:   message.Message{SessionID: sessionID, Role: message.User},
	})
}

func (a *sessionAgent) autoCompactSessionMessages(ctx context.Context, sessionID string, model Model, providerCtx plugin.ProviderContext, msgs []message.Message) ([]message.Message, error) {
	result, err := a.transformSessionMessages(ctx, chatRequestStateInput{
		SessionID: sessionID,
		Agent:     "session",
		Model:     model,
		Provider:  providerCtx,
		Purpose:   plugin.ChatTransformPurposeAutoCompact,
		Messages:  msgs,
		Message:   message.Message{SessionID: sessionID, Role: message.User},
	})
	if err != nil {
		return nil, err
	}
	return builtinAutoCompactMessages(result), nil
}

func (a *sessionAgent) postCompactSessionMessages(ctx context.Context, sessionID string, model Model, providerCtx plugin.ProviderContext, msgs []message.Message) ([]message.Message, error) {
	return a.transformSessionMessages(ctx, chatRequestStateInput{
		SessionID: sessionID,
		Agent:     "session",
		Model:     model,
		Provider:  providerCtx,
		Purpose:   plugin.ChatTransformPurposePostCompact,
		Messages:  msgs,
		Message:   message.Message{SessionID: sessionID, Role: message.User},
	})
}

func (a *sessionAgent) autoRecallBlock(ctx context.Context, sessionID, prompt string) string {
	if a.autoRecall == nil {
		return ""
	}
	return strings.TrimSpace(a.autoRecall(ctx, sessionID, prompt))
}

type SessionAgentCall struct {
	SessionID     string
	Prompt        string
	Purpose       plugin.ChatTransformPurpose
	InitiatorType string
	// JoinActiveRun allows this queued call to be injected into the active
	// run's next provider step instead of waiting for the current run to
	// finish.
	JoinActiveRun    bool
	ProviderOptions  fantasy.ProviderOptions
	Attachments      []message.Attachment
	MaxOutputTokens  int64
	Temperature      *float64
	TopP             *float64
	TopK             *int64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	NonInteractive   bool
	// OnProgress is called after each completed LLM step with accumulated
	// tool call count and the name of the last tool invoked.
	OnProgress func(toolUses int, lastTool string)
}

type SessionAgent interface {
	Run(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)
	EstimateSessionPromptTokensForModel(context.Context, string, Model) (int64, error)
	SetModels(large Model, small Model)
	SetTools(tools []fantasy.AgentTool)
	SetSystemPrompt(systemPrompt string)
	SetSystemPromptPrefix(systemPromptPrefix string)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	QueuedPromptsList(sessionID string) []string
	RemoveQueuedPrompt(sessionID string, index int) bool
	ClearQueue(sessionID string)
	Summarize(context.Context, string, fantasy.ProviderOptions) error
	Model() Model
	PauseQueue(sessionID string)
	ResumeQueue(sessionID string)
	IsQueuePaused(sessionID string) bool
	PrioritizeQueuedPrompt(sessionID string, index int) bool
}

type Model struct {
	Model      fantasy.LanguageModel
	CatwalkCfg catwalk.Model
	ModelCfg   config.SelectedModel
}

type sessionAgent struct {
	largeModel         *csync.Value[Model]
	smallModel         *csync.Value[Model]
	systemPromptPrefix *csync.Value[string]
	systemPrompt       *csync.Value[string]
	workingDir         string
	tools              *csync.Slice[fantasy.AgentTool]
	agentFactory       func(model fantasy.LanguageModel, opts ...fantasy.AgentOption) fantasy.Agent
	autoRecall         func(context.Context, string, string) string

	refreshCallConfig    func(context.Context) (sessionAgentRuntimeConfig, error)
	isSubAgent           bool
	sessions             session.Service
	messages             message.Service
	memory               memory.Service
	backgroundModel      *backgroundModel
	reviewToolResult     func(context.Context, string, message.ToolResult, session.PermissionMode) (message.ToolResult, error)
	disableAutoSummarize bool
	disableAutoMemory    bool
	isYolo               bool
	notify               pubsub.Publisher[notify.Notification]
	hookManager          *hooks.Manager
	filetracker          filetracker.Service
	checkpoint           checkpoint.Service

	queueMu        sync.Mutex
	messageQueue   *csync.Map[string, []SessionAgentCall]
	activeRequests *csync.Map[string, context.CancelFunc]
	pausedQueues   *csync.Map[string, bool]

	extractionMu        sync.Mutex
	pendingExtractions  map[string][]context.CancelFunc
	extractionTurnCount map[string]int
}

type SessionAgentOptions struct {
	LargeModel           Model
	SmallModel           Model
	SystemPromptPrefix   string
	SystemPrompt         string
	WorkingDir           string
	AgentFactory         func(model fantasy.LanguageModel, opts ...fantasy.AgentOption) fantasy.Agent
	AutoRecall           func(context.Context, string, string) string
	RefreshCallConfig    func(context.Context) (sessionAgentRuntimeConfig, error)
	IsSubAgent           bool
	DisableAutoSummarize bool
	DisableAutoMemory    bool
	IsYolo               bool
	Sessions             session.Service
	Messages             message.Service
	Memory               memory.Service
	BackgroundModel      *backgroundModel
	ReviewToolResult     func(context.Context, string, message.ToolResult, session.PermissionMode) (message.ToolResult, error)
	Tools                []fantasy.AgentTool
	Notify               pubsub.Publisher[notify.Notification]
	HookManager          *hooks.Manager
	Filetracker          filetracker.Service
	Checkpoint           checkpoint.Service
}

type sessionAgentRuntimeConfig struct {
	ProviderOptions    fantasy.ProviderOptions
	MaxOutputTokens    int64
	Temperature        *float64
	TopP               *float64
	TopK               *int64
	FrequencyPenalty   *float64
	PresencePenalty    *float64
	SystemPrompt       *string
	SystemPromptPrefix *string
	CollaborationMode  session.CollaborationMode
	PermissionMode     session.PermissionMode
	AllowedToolNames   []string
	Tools              []fantasy.AgentTool
}

type sessionAgentRuntimeConfigContextKey struct{}

func NewSessionAgent(
	opts SessionAgentOptions,
) SessionAgent {
	agentFactory := opts.AgentFactory
	if agentFactory == nil {
		agentFactory = fantasy.NewAgent
	}
	return &sessionAgent{
		largeModel:           csync.NewValue(opts.LargeModel),
		smallModel:           csync.NewValue(opts.SmallModel),
		systemPromptPrefix:   csync.NewValue(opts.SystemPromptPrefix),
		systemPrompt:         csync.NewValue(opts.SystemPrompt),
		workingDir:           opts.WorkingDir,
		agentFactory:         agentFactory,
		autoRecall:           opts.AutoRecall,
		refreshCallConfig:    opts.RefreshCallConfig,
		isSubAgent:           opts.IsSubAgent,
		sessions:             opts.Sessions,
		messages:             opts.Messages,
		memory:               opts.Memory,
		backgroundModel:      opts.BackgroundModel,
		reviewToolResult:     opts.ReviewToolResult,
		disableAutoSummarize: opts.DisableAutoSummarize,
		disableAutoMemory:    opts.DisableAutoMemory,
		tools:                csync.NewSliceFrom(opts.Tools),
		isYolo:               opts.IsYolo,
		notify:               opts.Notify,
		hookManager:          opts.HookManager,
		filetracker:          opts.Filetracker,
		checkpoint:           opts.Checkpoint,
		messageQueue:         csync.NewMap[string, []SessionAgentCall](),
		activeRequests:       csync.NewMap[string, context.CancelFunc](),
		pausedQueues:         csync.NewMap[string, bool](),
		pendingExtractions:   make(map[string][]context.CancelFunc),
		extractionTurnCount:  make(map[string]int),
	}
}

func (a *sessionAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	if call.InitiatorType != "" {
		ctx = copilot.ContextWithInitiatorType(ctx, call.InitiatorType)
	} else if a.isSubAgent {
		ctx = copilot.ContextWithInitiatorType(ctx, copilot.InitiatorAgent)
	}

	// isUserInitiatedRequest is true only for the very first step of a real user
	// prompt. All tool-call continuations, auto-resume prompts, sub-agent
	// requests, and any call with an explicit InitiatorAgent type are free
	// (X-Initiator: agent).
	isUserInitiatedRequest := call.InitiatorType == copilot.InitiatorUser ||
		(call.InitiatorType == "" && !a.isSubAgent)
	firstRequestStep := true

	if call.Prompt == "" && !message.ContainsTextAttachment(call.Attachments) {
		return nil, ErrEmptyPrompt
	}
	if call.SessionID == "" {
		return nil, ErrSessionMissing
	}

	// Queue the message if busy
	if a.IsSessionBusy(call.SessionID) {
		a.enqueueQueuedCall(call.SessionID, call)
		return nil, nil
	}

	if a.hookManager != nil {
		a.hookManager.RunSessionStart(ctx, call.SessionID)
		defer a.hookManager.RunSessionEnd(ctx, call.SessionID)
	}

	if a.hookManager != nil && call.Prompt != "" {
		a.hookManager.RunUserPromptSubmit(ctx, call.SessionID, call.Prompt)
	}

	if a.checkpoint != nil && !a.isSubAgent {
		if _, cpErr := a.checkpoint.CreateCheckpoint(ctx, call.SessionID, ""); cpErr != nil {
			slog.Warn("Failed to create checkpoint", "error", cpErr, "session_id", call.SessionID)
		}
	}

	runtimeConfig, err := a.refreshCallConfigIfNeeded(ctx, &call)
	if err != nil {
		return nil, err
	}

	// Copy mutable fields under lock to avoid races with SetTools/SetModels.
	agentTools := a.tools.Copy()
	largeModel := a.largeModel.Get()
	systemPrompt := a.systemPrompt.Get()
	promptPrefix := a.systemPromptPrefix.Get()
	if runtimeConfig != nil {
		if runtimeConfig.SystemPrompt != nil {
			systemPrompt = *runtimeConfig.SystemPrompt
		}
		if runtimeConfig.SystemPromptPrefix != nil {
			promptPrefix = *runtimeConfig.SystemPromptPrefix
		}
		if len(runtimeConfig.Tools) > 0 {
			agentTools = append([]fantasy.AgentTool(nil), runtimeConfig.Tools...)
		}
		if len(runtimeConfig.AllowedToolNames) > 0 {
			agentTools = filterToolsByNames(agentTools, runtimeConfig.AllowedToolNames)
		}
	}
	var instructions strings.Builder

	for _, server := range mcp.GetStates() {
		if server.State != mcp.StateConnected {
			continue
		}
		if s := server.Client.InitializeResult().Instructions; s != "" {
			instructions.WriteString(s)
			instructions.WriteString("\n\n")
		}
	}

	if s := instructions.String(); s != "" {
		systemPrompt += "\n\n<mcp-instructions>\n" + s + "\n</mcp-instructions>"
	}

	if !a.isSubAgent {
		if recall := a.autoRecallBlock(ctx, call.SessionID, call.Prompt); recall != "" {
			systemPrompt += "\n\n<auto_recall>\n" + recall + "\n</auto_recall>"
		}
	}

	providerCtx := defaultProviderContext()
	requestPurpose := call.Purpose
	if requestPurpose == "" {
		requestPurpose = plugin.ChatTransformPurposeRequest
	}

	var preflightSummarized bool
	sessionLock := sync.Mutex{}
	currentSession, err := a.sessions.Get(ctx, call.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return nil, fmt.Errorf("failed to get session messages: %w", err)
	}
	promptPrefix = buildDelegationPromptPrefix(promptPrefix, agentTools, a.isSubAgent)
	if len(msgs) > 0 {
		microCompacted, compactErr := a.microCompactSessionMessages(ctx, call.SessionID, largeModel, providerCtx, msgs)
		if compactErr != nil {
			return nil, compactErr
		}
		if len(microCompacted) > 0 {
			msgs = microCompacted
		}
		collapsed, collapseErr := a.collapseSessionMessages(ctx, call.SessionID, largeModel, providerCtx, msgs)
		if collapseErr != nil {
			return nil, collapseErr
		}
		if len(collapsed) > 0 {
			msgs = collapsed
		}
	}

	preflightState, err := a.buildChatRequestState(ctx, chatRequestStateInput{
		SessionID:      call.SessionID,
		Agent:          "session",
		Model:          largeModel,
		Provider:       providerCtx,
		Purpose:        plugin.ChatTransformPurposePreflightEstimate,
		Messages:       msgs,
		Message:        transientUserMessage(call.SessionID, call.Prompt, call.Attachments),
		Attachments:    call.Attachments,
		SystemPrompt:   systemPrompt,
		PromptPrefix:   promptPrefix,
		PermissionMode: currentSession.PermissionMode,
	})
	if err != nil {
		return nil, err
	}
	if !a.disableAutoSummarize && len(msgs) > 0 {
		// Estimate input tokens only. The shouldAutoSummarize function handles
		// output token reservation internally, so we don't need to add maxOutputTokens here.
		// This prevents double-counting the output reservation for large context models.
		estimatedInput := a.estimateSessionPromptTokens(preflightState.History, call.Prompt, call.Attachments, agentTools, preflightState.SystemPrompt, preflightState.PromptPrefix)
		estimatedInput = max(estimatedInput, currentSession.LastInputTokens())
		if trigger := proactiveCompactionTrigger(estimatedInput, effectiveContextWindow(largeModel), call.MaxOutputTokens); trigger != sessionCompactionTriggerNone {
			if truncErr := a.truncateOversizedToolResults(ctx, call.SessionID); truncErr != nil {
				slog.Warn("Failed to truncate oversized tool results before preflight summarization", "error", truncErr, "session_id", call.SessionID)
			}
			if summarizeErr := a.Summarize(withSessionCompactingPurpose(copilot.ContextWithInitiatorType(ctx, copilot.InitiatorAgent), trigger.Purpose()), call.SessionID, call.ProviderOptions); summarizeErr != nil {
				return nil, summarizeErr
			}
			preflightSummarized = true
			currentSession, err = a.sessions.Get(ctx, call.SessionID)
			if err != nil {
				return nil, fmt.Errorf("failed to reload session after summarization: %w", err)
			}
			msgs, err = a.getSessionMessages(ctx, currentSession)
			if err != nil {
				return nil, fmt.Errorf("failed to reload session messages after summarization: %w", err)
			}
		}
	}

	var wg sync.WaitGroup
	if !call.NonInteractive && shouldGenerateSessionTitle(currentSession.Title) {
		titlePrompt := titlePromptFromCallOrHistory(call.Prompt, msgs)
		if titlePrompt != "" {
			titleCtx := copilot.ContextWithInitiatorType(ctx, copilot.InitiatorAgent)
			wg.Go(func() {
				a.generateTitle(titleCtx, call.SessionID, titlePrompt, &sessionLock)
			})
		}
	}
	defer wg.Wait()

	// Add the user message to the session.
	userMessage, err := a.createUserMessage(ctx, call)
	if err != nil {
		return nil, err
	}

	// Add the session to the context.
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, call.SessionID)
	ctx = context.WithValue(ctx, tools.SessionServiceContextKey, a.sessions)

	genCtx, cancel := context.WithCancel(ctx)
	a.activeRequests.Set(call.SessionID, cancel)

	defer cancel()
	defer a.activeRequests.Del(call.SessionID)

	requestState, err := a.buildChatRequestState(genCtx, chatRequestStateInput{
		SessionID:      call.SessionID,
		Agent:          "session",
		Model:          largeModel,
		Provider:       providerCtx,
		Purpose:        requestPurpose,
		Messages:       msgs,
		Message:        userMessage,
		Attachments:    call.Attachments,
		SystemPrompt:   systemPrompt,
		PromptPrefix:   promptPrefix,
		PermissionMode: currentSession.PermissionMode,
	})
	if err != nil {
		return nil, err
	}
	if len(agentTools) > 0 {
		// Add Anthropic caching to the last tool.
		agentTools[len(agentTools)-1].SetProviderOptions(a.getCacheControlOptions())
	}
	agent := a.agentFactory(
		retryableStreamModel{largeModel.Model},
		fantasy.WithSystemPrompt(requestState.SystemPrompt),
		fantasy.WithTools(agentTools...),
		fantasy.WithUserAgent(userAgent),
	)

	startTime := time.Now()
	a.eventPromptSent(call.SessionID)

	var shouldSummarize bool
	var compactionTrigger sessionCompactionTrigger
	var contextWindowExceeded bool
	contextWindowAutoResumeAllowed := true
	var currentAssistant *message.Message
	var currentStepToolMessageIDs []string
	var currentStepToolResultChars int
	var allRunMessageIDs []string
	var estimatedPromptTokens int64
	var completedStepsThisRun int
	var runToolUses int
	var runLastTool string
	runStream := func(providerOptions fantasy.ProviderOptions, billFirstStepAsUser bool) (*fantasy.AgentResult, error) {
		currentAssistant = nil
		currentStepToolMessageIDs = nil
		currentStepToolResultChars = 0
		allRunMessageIDs = nil
		estimatedPromptTokens = 0
		shouldSummarize = false
		completedStepsThisRun = 0
		runToolUses = 0
		runLastTool = ""
		firstRequestStep = billFirstStepAsUser

		if err := plugin.TriggerChatBeforeRequest(genCtx, plugin.ChatBeforeRequestInput{
			SessionID: call.SessionID,
			Agent:     "session",
			Model: plugin.ModelInfo{
				ProviderID: largeModel.ModelCfg.Provider,
				ModelID:    largeModel.ModelCfg.Model,
			},
			Provider: providerCtx,
			Message:  userMessage,
		}); err != nil {
			return nil, err
		}

		result, err := agent.Stream(genCtx, fantasy.AgentStreamCall{
			Prompt:           message.PromptWithTextAttachments(call.Prompt, call.Attachments),
			Files:            requestState.Files,
			Messages:         requestState.History,
			ProviderOptions:  providerOptions,
			MaxOutputTokens:  &call.MaxOutputTokens,
			TopP:             call.TopP,
			Temperature:      call.Temperature,
			PresencePenalty:  call.PresencePenalty,
			TopK:             call.TopK,
			FrequencyPenalty: call.FrequencyPenalty,
			PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
				// Explicitly tag every LLM request with the correct X-Initiator value
				// so GitHub Copilot billing is correct regardless of how the fantasy
				// framework propagates the outer context. Only the first step of a
				// real user-initiated request is billable; tool-call loops,
				// sub-agent steps, and continuations are always free.
				if isUserInitiatedRequest && firstRequestStep {
					callContext = copilot.ContextWithInitiatorType(callContext, copilot.InitiatorUser)
				} else {
					callContext = copilot.ContextWithInitiatorType(callContext, copilot.InitiatorAgent)
				}
				firstRequestStep = false

				stepRuntimeConfig := runtimeConfig
				if a.refreshCallConfig != nil {
					refreshed, refreshErr := a.refreshCallConfig(callContext)
					if refreshErr != nil {
						slog.Warn("Failed to refresh runtime config for step", "error", refreshErr, "session_id", call.SessionID)
					} else {
						stepRuntimeConfig = &refreshed
					}
				}

				prepared.Tools = a.tools.Copy()
				if stepRuntimeConfig != nil && len(stepRuntimeConfig.Tools) > 0 {
					prepared.Tools = append([]fantasy.AgentTool(nil), stepRuntimeConfig.Tools...)
				}
				if stepRuntimeConfig != nil && len(stepRuntimeConfig.AllowedToolNames) > 0 {
					prepared.Tools = filterToolsByNames(prepared.Tools, stepRuntimeConfig.AllowedToolNames)
				}
				// Add Anthropic caching to the last tool.
				if len(prepared.Tools) > 0 {
					prepared.Tools[len(prepared.Tools)-1].SetProviderOptions(a.getCacheControlOptions())
				}

				prepared.Messages = options.Messages
				for i := range prepared.Messages {
					prepared.Messages[i].ProviderOptions = nil
				}

				queuedCalls := a.takeJoinActiveRunCalls(call.SessionID)
				remainingJoinBudget := joinActiveRunPromptCharsBudget

				type selectedCall struct {
					index int
					call  SessionAgentCall
				}
				var selected []selectedCall
				for i := len(queuedCalls) - 1; i >= 0; i-- {
					queued := queuedCalls[i]
					if len(selected) >= joinActiveRunMaxInjectedCalls || remainingJoinBudget <= 0 {
						a.enqueueQueuedCall(call.SessionID, queued)
						continue
					}
					prompt := strings.TrimSpace(queued.Prompt)
					if prompt == "" {
						a.enqueueQueuedCall(call.SessionID, queued)
						continue
					}
					promptRunes := []rune(prompt)
					if len(promptRunes) > remainingJoinBudget {
						if remainingJoinBudget <= 1 {
							a.enqueueQueuedCall(call.SessionID, queued)
							continue
						}
						prompt = string(promptRunes[:remainingJoinBudget-1]) + "…"
					}
					queued.Prompt = prompt
					selected = append(selected, selectedCall{index: i, call: queued})
					remainingJoinBudget -= len([]rune(prompt))
				}

				for s := len(selected) - 1; s >= 0; s-- {
					userMessage, createErr := a.createUserMessage(callContext, selected[s].call)
					if createErr != nil {
						return callContext, prepared, createErr
					}
					prepared.Messages = append(prepared.Messages, userMessage.ToAIMessage()...)
				}

				prepared.Messages = a.workaroundProviderMediaLimitations(prepared.Messages, largeModel)

				lastSystemRoleInx := 0
				systemMessageUpdated := false
				for i, msg := range prepared.Messages {
					// Only add cache control to the last message.
					if msg.Role == fantasy.MessageRoleSystem {
						lastSystemRoleInx = i
					} else if !systemMessageUpdated {
						prepared.Messages[lastSystemRoleInx].ProviderOptions = a.getCacheControlOptions()
						systemMessageUpdated = true
					}
					// Than add cache control to the last 2 messages.
					if i > len(prepared.Messages)-3 {
						prepared.Messages[i].ProviderOptions = a.getCacheControlOptions()
					}
				}

				if requestState.PromptPrefix != "" {
					prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(requestState.PromptPrefix)}, prepared.Messages...)
				}

				var assistantMsg message.Message
				assistantMsg, err = a.messages.Create(callContext, call.SessionID, message.CreateMessageParams{
					Role:     message.Assistant,
					Parts:    []message.ContentPart{},
					Model:    largeModel.ModelCfg.Model,
					Provider: largeModel.ModelCfg.Provider,
				})
				if err != nil {
					return callContext, prepared, err
				}
				callContext = context.WithValue(callContext, tools.MessageIDContextKey, assistantMsg.ID)
				callContext = context.WithValue(callContext, tools.SupportsImagesContextKey, largeModel.CatwalkCfg.SupportsImages)
				callContext = context.WithValue(callContext, tools.ModelNameContextKey, largeModel.CatwalkCfg.Name)
				callContext = context.WithValue(callContext, tools.SessionServiceContextKey, a.sessions)
				currentAssistant = &assistantMsg
				currentStepToolMessageIDs = nil
				currentStepToolResultChars = 0
				allRunMessageIDs = append(allRunMessageIDs, assistantMsg.ID)

				estimatedPromptTokens = estimatePromptTokens(prepared.Messages, prepared.Tools)
				return callContext, prepared, err
			},
			OnReasoningStart: func(id string, reasoning fantasy.ReasoningContent) error {
				currentAssistant.AppendReasoningContent(reasoning.Text)
				return a.messages.Update(genCtx, *currentAssistant)
			},
			OnReasoningDelta: func(id string, text string) error {
				currentAssistant.AppendReasoningContent(text)
				return a.messages.Update(genCtx, *currentAssistant)
			},
			OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
				// handle anthropic signature
				if anthropicData, ok := reasoning.ProviderMetadata[anthropic.Name]; ok {
					if reasoning, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok {
						currentAssistant.AppendReasoningSignature(reasoning.Signature)
					}
				}
				if googleData, ok := reasoning.ProviderMetadata[google.Name]; ok {
					if reasoning, ok := googleData.(*google.ReasoningMetadata); ok {
						currentAssistant.AppendThoughtSignature(reasoning.Signature, reasoning.ToolID)
					}
				}
				if openaiData, ok := reasoning.ProviderMetadata[openai.Name]; ok {
					if reasoning, ok := openaiData.(*openai.ResponsesReasoningMetadata); ok {
						currentAssistant.SetReasoningResponsesData(reasoning)
					}
				}
				currentAssistant.FinishThinking()
				return a.messages.Update(genCtx, *currentAssistant)
			},
			OnTextDelta: func(id string, text string) error {
				// Strip leading newline from initial text content. This is is
				// particularly important in non-interactive mode where leading
				// newlines are very visible.
				if len(currentAssistant.Parts) == 0 {
					text = strings.TrimPrefix(text, "\n")
				}

				currentAssistant.AppendContent(text)
				return a.messages.Update(genCtx, *currentAssistant)
			},
			OnToolInputStart: func(id string, toolName string) error {
				toolCall := message.ToolCall{
					ID:               id,
					Name:             toolName,
					ProviderExecuted: false,
					Finished:         false,
				}
				currentAssistant.AddToolCall(toolCall)
				return a.messages.Update(ctx, *currentAssistant)
			},
			OnRetry: func(providerErr *fantasy.ProviderError, delay time.Duration) {
				slog.Info("Retrying after network error", "error", providerErr.Error(), "delay", delay)
				if currentAssistant == nil {
					return
				}
				if err := a.resetRetriedStep(ctx, currentAssistant, currentStepToolMessageIDs); err != nil {
					slog.Warn("Failed to reset step state before retry", "error", err, "session_id", currentAssistant.SessionID, "message_id", currentAssistant.ID)
					return
				}
				currentStepToolMessageIDs = nil
				currentStepToolResultChars = 0
			},
			OnToolCall: func(tc fantasy.ToolCallContent) error {
				toolCall := message.ToolCall{
					ID:               tc.ToolCallID,
					Name:             tc.ToolName,
					Input:            tc.Input,
					ProviderExecuted: false,
					Finished:         true,
				}
				currentAssistant.AddToolCall(toolCall)
				runToolUses++
				runLastTool = tc.ToolName
				return a.messages.Update(ctx, *currentAssistant)
			},
			OnToolResult: func(result fantasy.ToolResultContent) error {
				toolResult := a.convertToToolResult(result)
				toolResult, additionalMedia := a.extractAdditionalMCPMedia(toolResult)
				if runtimeConfig != nil {
					toolResult = a.applyToolResultReview(genCtx, currentAssistant.SessionID, toolResult, runtimeConfig.PermissionMode)
				}
				toolResult = a.enforceStepToolResultBudget(currentAssistant.SessionID, toolResult, &currentStepToolResultChars)
				if truncatedResult, truncated := a.truncateToolResult(currentAssistant.SessionID, toolResult); truncated {
					toolResult = truncatedResult
				}
				toolMsg, createMsgErr := a.messages.Create(ctx, currentAssistant.SessionID, message.CreateMessageParams{
					Role: message.Tool,
					Parts: []message.ContentPart{
						toolResult,
					},
				})
				if createMsgErr != nil {
					return createMsgErr
				}
				currentStepToolMessageIDs = append(currentStepToolMessageIDs, toolMsg.ID)
				allRunMessageIDs = append(allRunMessageIDs, toolMsg.ID)

				if len(additionalMedia) > 0 {
					parts := make([]message.ContentPart, 0, len(additionalMedia)+1)
					parts = append(parts, message.TextContent{Text: "Additional media content from the tool result:"})
					for _, mediaPart := range additionalMedia {
						parts = append(parts, mediaPart)
					}
					additionalMsg, additionalErr := a.messages.Create(ctx, currentAssistant.SessionID, message.CreateMessageParams{
						Role:  message.User,
						Parts: parts,
					})
					if additionalErr != nil {
						return additionalErr
					}
					currentStepToolMessageIDs = append(currentStepToolMessageIDs, additionalMsg.ID)
					allRunMessageIDs = append(allRunMessageIDs, additionalMsg.ID)
				}
				return nil
			},
			OnStepFinish: func(stepResult fantasy.StepResult) error {
				finishReason := message.FinishReasonUnknown
				switch stepResult.FinishReason {
				case fantasy.FinishReasonLength:
					finishReason = message.FinishReasonMaxTokens
				case fantasy.FinishReasonStop:
					finishReason = message.FinishReasonEndTurn
				case fantasy.FinishReasonToolCalls:
					finishReason = message.FinishReasonToolUse
				}
				currentAssistant.AddFinish(finishReason, "", "")
				sessionLock.Lock()
				defer sessionLock.Unlock()

				updatedSession, getSessionErr := a.sessions.Get(ctx, call.SessionID)
				if getSessionErr != nil {
					return getSessionErr
				}
				a.updateSessionUsage(largeModel, &updatedSession, stepResult.Usage, a.openrouterCost(stepResult.ProviderMetadata), estimatedPromptTokens)
				_, sessionErr := a.sessions.Save(ctx, updatedSession)
				if sessionErr != nil {
					return sessionErr
				}
				completedStepsThisRun++
				currentSession = updatedSession
				updateErr := a.messages.Update(genCtx, *currentAssistant)
				if call.OnProgress != nil {
					call.OnProgress(runToolUses, runLastTool)
				}
				return updateErr
			},
			StopWhen: []fantasy.StopCondition{
				func(_ []fantasy.StepResult) bool {
					projectedPromptTokens, estimateErr := a.estimateNextStepPromptTokens(genCtx, call.SessionID, agentTools, systemPrompt, promptPrefix, largeModel, providerCtx)
					if estimateErr != nil {
						slog.Warn("Failed to estimate next-step prompt tokens", "error", estimateErr, "session_id", call.SessionID)
						// Fallback: use the higher of LastInputTokens or the current step's estimatedPromptTokens.
						// estimatedPromptTokens is set during PrepareStep and reflects the actual messages
						// that will be sent to the LLM, making it more accurate than LastInputTokens.
						fallbackTokens := currentSession.LastInputTokens()
						if estimatedPromptTokens > fallbackTokens {
							fallbackTokens = estimatedPromptTokens
							slog.Info("Using current step's estimatedPromptTokens as fallback", "estimatedPromptTokens", estimatedPromptTokens, "lastInputTokens", currentSession.LastInputTokens(), "session_id", call.SessionID)
						} else {
							slog.Info("Using LastInputTokens as fallback", "lastInputTokens", fallbackTokens, "session_id", call.SessionID)
						}
						projectedPromptTokens = fallbackTokens
					}
					if !preflightSummarized {
						projectedPromptTokens = max(projectedPromptTokens, currentSession.LastInputTokens())
					}
					// Pass input-only estimate to shouldAutoSummarize. The function
					// handles output token reservation internally to avoid double-counting.
					if shouldAutoSummarize(projectedPromptTokens, effectiveContextWindow(largeModel), call.MaxOutputTokens) && !a.disableAutoSummarize {
						shouldSummarize = true
						return true
					}
					return false
				},
				func(steps []fantasy.StepResult) bool {
					return hasRepeatedToolCalls(steps, loopDetectionWindowSize, loopDetectionMaxRepeats)
				},
			},
		})
		if err == nil {
			hydrateAgentResultFromAssistantMessage(result, currentAssistant)
		}
		if hookErr := plugin.TriggerChatAfterResponse(genCtx, plugin.ChatAfterResponseInput{
			SessionID: call.SessionID,
			Agent:     "session",
			Model: plugin.ModelInfo{
				ProviderID: largeModel.ModelCfg.Provider,
				ModelID:    largeModel.ModelCfg.Model,
			},
			Purpose: requestPurpose,
			Result:  result,
			Error:   err,
		}); hookErr != nil {
			if err != nil {
				return nil, fmt.Errorf("stream error: %w; hook error: %w", err, hookErr)
			}
			return nil, hookErr
		}
		return result, err
	}

	providerOptions := call.ProviderOptions
	var result *fantasy.AgentResult
	var retryAttempt int
	for {
		result, err = runStream(providerOptions, retryAttempt == 0)

		if err != nil && isRetriableError(err) && !a.disableAutoSummarize {
			observedPromptTokens := max(currentSession.LastInputTokens(), estimatedPromptTokens)
			if shouldAutoSummarize(observedPromptTokens, effectiveContextWindow(largeModel), call.MaxOutputTokens) {
				slog.Warn("Near context limit during transient failure; forcing summarization to recover",
					"error", err,
					"session_id", call.SessionID,
					"model", largeModel.ModelCfg.Model,
					"provider", largeModel.ModelCfg.Provider,
					"observed_prompt_tokens", observedPromptTokens,
				)
				if truncErr := a.truncateOversizedToolResults(ctx, call.SessionID); truncErr != nil {
					slog.Warn("Failed to truncate oversized tool results before retry summarization", "error", truncErr, "session_id", call.SessionID)
				}
				if currentAssistant != nil {
					currentAssistant.FinishThinking()
					currentAssistant.AddFinish(
						message.FinishReasonError,
						"Context limit reached",
						"The conversation history is near this model's context window limit and the request is repeatedly failing. Auto-summarizing the session to continue the task…",
					)
					if updateErr := a.messages.Update(ctx, *currentAssistant); updateErr != nil {
						slog.Warn("Failed to update assistant message before retry summarization", "error", updateErr, "session_id", call.SessionID)
					}
				}
				compactionTrigger = sessionCompactionTriggerRecover
				shouldSummarize = true
				err = nil
				result = &fantasy.AgentResult{}
				break
			}
		}

		// Check for retriable errors (429, 503, network issues).
		if err != nil && isRetriableError(err) && retryAttempt < maxRetriableAttempts {
			if completedStepsThisRun > 0 {
				// Steps already completed — only clean up the current
				// incomplete step to avoid re-executing tool side
				// effects. Completed steps' messages stay in the DB.
				if cleanupErr := a.cleanupFailedAttempt(ctx, currentAssistant, currentStepToolMessageIDs); cleanupErr != nil {
					slog.Warn("Failed to clean up incomplete step during retry",
						"error", cleanupErr, "session_id", call.SessionID)
					break
				}
				// Re-fetch history from DB so the retry includes the
				// completed steps' messages.
				retryMsgs, getMsgsErr := a.getSessionMessages(ctx, currentSession)
				if getMsgsErr != nil {
					slog.Warn("Failed to re-fetch messages for retry",
						"error", getMsgsErr, "session_id", call.SessionID)
					break
				}
				retryState, buildErr := a.buildChatRequestState(genCtx, chatRequestStateInput{
					SessionID:      call.SessionID,
					Agent:          "session",
					Model:          largeModel,
					Provider:       providerCtx,
					Purpose:        requestPurpose,
					Messages:       retryMsgs,
					Message:        userMessage,
					Attachments:    call.Attachments,
					SystemPrompt:   systemPrompt,
					PromptPrefix:   promptPrefix,
					PermissionMode: currentSession.PermissionMode,
				})
				if buildErr != nil {
					slog.Warn("Failed to rebuild request state for retry",
						"error", buildErr, "session_id", call.SessionID)
					break
				}
				requestState = retryState
				// Reset retry budget — completed steps prove the
				// previous attempt made progress, so this is a new
				// transient failure that deserves full retries.
				retryAttempt = 0
			} else {
				// No steps completed — clean up all messages so the
				// retry starts from a clean slate.
				for _, id := range allRunMessageIDs {
					if delErr := a.messages.Delete(ctx, id); delErr != nil {
						slog.Warn("Failed to delete message during retry cleanup",
							"error", delErr, "message_id", id)
					}
				}
			}
			retryAttempt++
			delay := retryDelay(retryAttempt, retryAfterFromError(err))
			slog.Warn("Retrying after transient error",
				"error", err,
				"attempt", retryAttempt,
				"delay", delay,
				"completed_steps", completedStepsThisRun,
				"session_id", call.SessionID,
				"model", largeModel.ModelCfg.Model,
				"provider", largeModel.ModelCfg.Provider,
			)

			// Show a temporary message in the chat so the user knows
			// a retry is in progress and how long it will take.
			retryText := fmt.Sprintf(
				"Service temporarily unavailable. Retrying in %d seconds... (attempt %d/%d)",
				int(delay.Seconds()), retryAttempt, maxRetriableAttempts,
			)
			retryMsg, retryMsgErr := a.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.TextContent{Text: retryText},
				},
				Model:    largeModel.ModelCfg.Model,
				Provider: largeModel.ModelCfg.Provider,
			})

			select {
			case <-ctx.Done():
				// Clean up the retry message before returning.
				if retryMsgErr == nil {
					_ = a.messages.Delete(ctx, retryMsg.ID)
				}
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// Remove the temporary retry message before the next attempt.
			if retryMsgErr == nil {
				_ = a.messages.Delete(ctx, retryMsg.ID)
			}
			continue
		}
		break
	}

	if shouldRetryWithoutAnthropicThinking(err, providerOptions) {
		slog.Warn(
			"Retrying request without Anthropic thinking after provider rejected unsigned reasoning content",
			"session_id", call.SessionID,
			"model", largeModel.ModelCfg.Model,
			"provider", largeModel.ModelCfg.Provider,
			"completed_steps", completedStepsThisRun,
		)
		if cleanupErr := a.cleanupFailedAttempt(ctx, currentAssistant, currentStepToolMessageIDs); cleanupErr != nil {
			return nil, cleanupErr
		}
		currentAssistant = nil
		currentStepToolMessageIDs = nil
		if completedStepsThisRun > 0 {
			retryMsgs, getMsgsErr := a.getSessionMessages(ctx, currentSession)
			if getMsgsErr != nil {
				return nil, getMsgsErr
			}
			retryState, buildErr := a.buildChatRequestState(genCtx, chatRequestStateInput{
				SessionID:      call.SessionID,
				Agent:          "session",
				Model:          largeModel,
				Provider:       providerCtx,
				Purpose:        requestPurpose,
				Messages:       retryMsgs,
				Message:        userMessage,
				Attachments:    call.Attachments,
				SystemPrompt:   systemPrompt,
				PromptPrefix:   promptPrefix,
				PermissionMode: currentSession.PermissionMode,
			})
			if buildErr != nil {
				return nil, buildErr
			}
			requestState = retryState
		}
		providerOptions, _ = disableAnthropicThinking(providerOptions)
		result, err = runStream(providerOptions, false)
	}

	alreadyRecoveringFromContextWindow := strings.HasPrefix(call.Prompt, contextWindowResumePromptPrefix)
	contextWindowErr := isContextWindowExceededError(err) || isContextLengthError(err)
	if contextWindowErr {
		estimatedInput := max(currentSession.LastInputTokens(), estimatedPromptTokens)
		if estimatedInput > currentSession.LastPromptTokens {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cleanupCancel()
			updatedSession, getSessionErr := a.sessions.Get(cleanupCtx, call.SessionID)
			if getSessionErr != nil {
				slog.Warn("Failed to load session for context-window token update", "error", getSessionErr, "session_id", call.SessionID)
			} else if estimatedInput > updatedSession.LastPromptTokens {
				updatedSession.LastPromptTokens = estimatedInput
				sessionLock.Lock()
				if _, saveSessionErr := a.sessions.Save(cleanupCtx, updatedSession); saveSessionErr != nil {
					slog.Warn("Failed to persist context-window token estimate", "error", saveSessionErr, "session_id", call.SessionID)
				} else {
					currentSession = updatedSession
				}
				sessionLock.Unlock()
			}
		}
	}
	if contextWindowErr && !a.disableAutoSummarize && !alreadyRecoveringFromContextWindow {
		contextWindowAutoResumeAllowed = completedStepsThisRun == 0
		slog.Warn("Context window exceeded; forcing summarization to recover",
			"session_id", call.SessionID,
			"model", largeModel.ModelCfg.Model,
			"provider", largeModel.ModelCfg.Provider,
			"completed_steps", completedStepsThisRun,
			"auto_resume_allowed", contextWindowAutoResumeAllowed,
		)
		if truncErr := a.truncateOversizedToolResults(ctx, call.SessionID); truncErr != nil {
			slog.Warn("Failed to truncate oversized tool results", "error", truncErr)
		}
		if currentAssistant != nil {
			currentAssistant.FinishThinking()
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cleanupCancel()
			msgs, listErr := a.messages.List(cleanupCtx, currentAssistant.SessionID)
			if listErr != nil {
				return nil, listErr
			}
			for _, tc := range currentAssistant.ToolCalls() {
				if !tc.Finished {
					tc.Finished = true
					tc.Input = "{}"
					currentAssistant.AddToolCall(tc)
				}
				found := false
				for _, msg := range msgs {
					if msg.Role != message.Tool {
						continue
					}
					for _, tr := range msg.ToolResults() {
						if tr.ToolCallID == tc.ID {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if found {
					continue
				}
				toolResult := message.ToolResult{
					ToolCallID: tc.ID,
					Name:       tc.Name,
					Content:    "There was an error while executing the tool",
					IsError:    true,
				}
				if subtaskStatus, ok := syntheticSubtaskStatusForTool(tc.Name, false, false); ok {
					toolResult = toolResult.WithSubtaskResult(message.ToolResultSubtaskResult{
						ParentToolCallID: tc.ID,
						Status:           subtaskStatus,
					})
				}
				if _, createErr := a.messages.Create(cleanupCtx, currentAssistant.SessionID, message.CreateMessageParams{
					Role: message.Tool,
					Parts: []message.ContentPart{
						toolResult,
					},
				}); createErr != nil {
					return nil, createErr
				}
			}
			finishDescription := "The conversation history reached this model's context window limit. Auto-summarizing the session to continue the task…"
			if !contextWindowAutoResumeAllowed {
				finishDescription = "The conversation history reached this model's context window limit after tools already ran. Auto-summarizing the session now; re-run your last request to continue safely without replaying completed tool calls."
			}
			currentAssistant.AddFinish(
				message.FinishReasonError,
				"Context limit reached",
				finishDescription,
			)
			if updateErr := a.messages.Update(cleanupCtx, *currentAssistant); updateErr != nil {
				slog.Warn("Failed to update failed assistant message after context-window error", "error", updateErr)
			}
		}
		contextWindowExceeded = contextWindowAutoResumeAllowed
		compactionTrigger = sessionCompactionTriggerRecover
		shouldSummarize = true
		err = nil
		result = &fantasy.AgentResult{}
	} else if contextWindowErr && alreadyRecoveringFromContextWindow {
		slog.Warn("Context window exceeded again after recover attempt; returning provider error",
			"session_id", call.SessionID,
			"model", largeModel.ModelCfg.Model,
			"provider", largeModel.ModelCfg.Provider,
		)
	}

	a.eventPromptResponded(call.SessionID, time.Since(startTime).Truncate(time.Second))

	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		isPermissionErr := permission.IsPermissionError(err)
		permissionErr, hasPermissionErr := permission.AsPermissionError(err)
		if currentAssistant == nil {
			return result, err
		}
		// Ensure we finish thinking on error to close the reasoning state.
		currentAssistant.FinishThinking()
		toolCalls := currentAssistant.ToolCalls()
		// Use a detached context for cleanup DB operations. Both ctx and
		// genCtx may be cancelled (e.g. ACP session/cancel cancels the
		// parent runCtx which propagates to both). We must still persist
		// tool-result messages so the conversation history stays valid.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		msgs, createErr := a.messages.List(cleanupCtx, currentAssistant.SessionID)
		if createErr != nil {
			return nil, createErr
		}
		for _, tc := range toolCalls {
			if !tc.Finished {
				tc.Finished = true
				tc.Input = "{}"
				currentAssistant.AddToolCall(tc)
				updateErr := a.messages.Update(cleanupCtx, *currentAssistant)
				if updateErr != nil {
					return nil, updateErr
				}
			}

			found := false
			for _, msg := range msgs {
				if msg.Role == message.Tool {
					for _, tr := range msg.ToolResults() {
						if tr.ToolCallID == tc.ID {
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
			if found {
				continue
			}
			content := "There was an error while executing the tool"
			if isCancelErr {
				content = "Error: user cancelled assistant tool calling"
			} else if isPermissionErr {
				if hasPermissionErr && permissionErr.Kind == permission.PermissionErrorKindPolicyDenied {
					content = cmp.Or(permissionErr.Message, "Permission blocked by safety policy")
				} else {
					content = "User denied permission"
				}
			}
			toolResult := message.ToolResult{
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Content:    content,
				IsError:    true,
			}
			if subtaskStatus, ok := syntheticSubtaskStatusForTool(tc.Name, isCancelErr, isPermissionErr); ok {
				toolResult = toolResult.WithSubtaskResult(message.ToolResultSubtaskResult{
					ParentToolCallID: tc.ID,
					Status:           subtaskStatus,
				})
			}
			_, createErr = a.messages.Create(cleanupCtx, currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			})
			if createErr != nil {
				return nil, createErr
			}
		}
		var fantasyErr *fantasy.Error
		var providerErr *fantasy.ProviderError
		const defaultTitle = "Provider Error"
		linkStyle := lipgloss.NewStyle().Foreground(charmtone.Guac).Underline(true)
		if isCancelErr {
			currentAssistant.AddFinish(message.FinishReasonCanceled, "User canceled request", "")
		} else if isPermissionErr {
			if hasPermissionErr && permissionErr.Kind == permission.PermissionErrorKindPolicyDenied {
				currentAssistant.AddFinish(
					message.FinishReasonPermissionDenied,
					"Permission blocked",
					permissionErr.Details,
				)
			} else {
				currentAssistant.AddFinish(message.FinishReasonPermissionDenied, "User denied permission", "")
			}
		} else if errors.Is(err, hyper.ErrNoCredits) {
			url := hyper.BaseURL()
			link := linkStyle.Hyperlink(url, "id=hyper").Render(url)
			currentAssistant.AddFinish(message.FinishReasonError, "No credits", "You're out of credits. Add more at "+link)
		} else if errors.As(err, &providerErr) {
			if providerErr.Message == "The requested model is not supported." {
				url := "https://github.com/settings/copilot/features"
				link := linkStyle.Hyperlink(url, "id=copilot").Render(url)
				currentAssistant.AddFinish(
					message.FinishReasonError,
					"Copilot model not enabled",
					withRetryFailureDetails(
						fmt.Sprintf("%q is not enabled in Copilot. Go to the following page to enable it. Then, wait 5 minutes before trying again. %s", largeModel.CatwalkCfg.Name, link),
						retryAttempt,
					),
				)
			} else {
				currentAssistant.AddFinish(
					message.FinishReasonError,
					cmp.Or(stringext.Capitalize(providerErr.Title), defaultTitle),
					withRetryFailureDetails(providerErr.Message, retryAttempt),
				)
			}
		} else if errors.As(err, &fantasyErr) {
			currentAssistant.AddFinish(
				message.FinishReasonError,
				cmp.Or(stringext.Capitalize(fantasyErr.Title), defaultTitle),
				withRetryFailureDetails(fantasyErr.Message, retryAttempt),
			)
		} else {
			currentAssistant.AddFinish(
				message.FinishReasonError,
				defaultTitle,
				withRetryFailureDetails(err.Error(), retryAttempt),
			)
		}
		// Use the detached cleanup context to ensure the assistant message
		// (with its finish reason) is always persisted.
		updateErr := a.messages.Update(cleanupCtx, *currentAssistant)
		if updateErr != nil {
			return nil, updateErr
		}
		return nil, err
	}

	// Send notification that agent has finished its turn (skip for
	// nested/non-interactive sessions).
	// NOTE: This is done after checking for summarization and queued messages
	// to avoid sending a spurious "agent finished" notification when the agent
	// is about to continue working.
	if a.hookManager != nil && !shouldSummarize {
		a.hookManager.RunStop(ctx, call.SessionID)
	}

	queuedMessages, ok := a.messageQueue.Get(call.SessionID)
	hasQueuedMessages := ok && len(queuedMessages) > 0
	if !call.NonInteractive && a.notify != nil && !shouldSummarize && !hasQueuedMessages {
		a.notify.Publish(pubsub.CreatedEvent, notify.Notification{
			SessionID:    call.SessionID,
			SessionTitle: currentSession.Title,
			Type:         notify.TypeAgentFinished,
		})
	}

	if shouldSummarize {
		a.activeRequests.Del(call.SessionID)
		if compactionTrigger == sessionCompactionTriggerNone {
			compactionTrigger = sessionCompactionTriggerNormal
		}
		if summarizeErr := a.Summarize(withSessionCompactingPurpose(copilot.ContextWithInitiatorType(genCtx, copilot.InitiatorAgent), compactionTrigger.Purpose()), call.SessionID, call.ProviderOptions); summarizeErr != nil {
			return nil, summarizeErr
		}
		hasPendingToolCalls := currentAssistant != nil && len(currentAssistant.ToolCalls()) > 0
		shouldAutoResume := hasPendingToolCalls
		if compactionTrigger == sessionCompactionTriggerRecover {
			shouldAutoResume = contextWindowAutoResumeAllowed
		}
		if shouldAutoResume {
			resumePrefix := autoResumePromptPrefix
			if contextWindowExceeded {
				resumePrefix = contextWindowResumePromptPrefix
			}
			call.Prompt = fmt.Sprintf(resumePrefix+"%s`", call.Prompt)
			if compactionTrigger == sessionCompactionTriggerRecover {
				call.Purpose = plugin.ChatTransformPurposeRecover
			}
			call.InitiatorType = copilot.InitiatorAgent
			a.enqueueQueuedCall(call.SessionID, call)
		}
	}

	// Release active request before processing queued messages.
	a.activeRequests.Del(call.SessionID)
	cancel()
	wg.Wait()

	if !a.isSubAgent && a.memory != nil && a.backgroundModel != nil && !a.disableAutoMemory && !shouldSummarize && a.QueuedPrompts(call.SessionID) == 0 {
		a.extractionMu.Lock()
		a.extractionTurnCount[call.SessionID]++
		turns := a.extractionTurnCount[call.SessionID]
		if shouldExtractMemories(turns) {
			a.extractionTurnCount[call.SessionID] = 0
			historyForExtraction := a.getHistoryForMemoryExtraction(ctx, call.SessionID)
			// Track pending extraction for graceful shutdown
			a.pendingExtractions[call.SessionID] = append(a.pendingExtractions[call.SessionID], cancel)
			go func() {
				extractMemories(context.Background(), a.memory, a.backgroundModel, call.SessionID, call.Prompt, historyForExtraction)
				a.extractionMu.Lock()
				delete(a.pendingExtractions, call.SessionID)
				a.extractionMu.Unlock()
			}()
		}
		a.extractionMu.Unlock()
	}

	if a.QueuedPrompts(call.SessionID) == 0 {
		return result, err
	}
	// Don't auto-process the next queued message while the queue is paused.
	if a.IsQueuePaused(call.SessionID) {
		return result, err
	}
	// There are queued messages restart the loop.
	firstQueuedMessage, ok := a.popNextQueuedCall(call.SessionID)
	if !ok {
		return result, err
	}
	ctx = context.WithValue(ctx, sessionAgentRuntimeConfigContextKey{}, (*sessionAgentRuntimeConfig)(nil))
	return a.Run(ctx, firstQueuedMessage)
}

func syntheticSubtaskStatusForTool(toolName string, isCancelErr, isPermissionErr bool) (message.ToolResultSubtaskStatus, bool) {
	switch toolName {
	case AgentToolName, tools.AgenticFetchToolName:
		if isCancelErr || isPermissionErr {
			return message.ToolResultSubtaskStatusCanceled, true
		}
		return message.ToolResultSubtaskStatusFailed, true
	default:
		return "", false
	}
}

func hydrateAgentResultFromAssistantMessage(result *fantasy.AgentResult, assistant *message.Message) {
	if result == nil || assistant == nil {
		return
	}

	text := assistant.Content().Text
	if strings.TrimSpace(text) == "" {
		return
	}

	if result.Response.Content.Text() != "" {
		return
	}

	textPart := fantasy.TextContent{Text: text}
	result.Response.Content = append(fantasy.ResponseContent{textPart}, result.Response.Content...)

	if len(result.Steps) == 0 {
		return
	}
	last := &result.Steps[len(result.Steps)-1]
	if last.Content.Text() == "" {
		last.Content = append(fantasy.ResponseContent{textPart}, last.Content...)
	}
}

// isContextLengthError checks if the error is due to context length exceeding the model's limit.
func isContextLengthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for common context length error patterns from various providers.
	// These patterns are specifically chosen to avoid matching rate limits or other errors.
	contextLengthIndicators := []string{
		"context window",
		"context length",
		"maximum context length",
		"range of input length",
		"context_too_long",
		"context window exceeded",
		"input length exceeds",
		"prompt is too long",
		"token limit exceeded",
		"request body too large",
	}
	lowerErr := strings.ToLower(errStr)
	for _, indicator := range contextLengthIndicators {
		if strings.Contains(lowerErr, indicator) {
			return true
		}
	}
	return false
}

// truncateMessagesToFit truncates messages to fit within the specified token limit.
// It keeps the most recent messages and removes older ones until the estimated
// token count is below the limit.
// Note: System messages are excluded from the result. The caller is responsible
// for adding appropriate system messages (e.g., via PrepareStep).
func truncateMessagesToFit(msgs []fantasy.Message, maxTokens int64) []fantasy.Message {
	if len(msgs) == 0 {
		return msgs
	}

	// Always keep at least the last 2 messages (user request + assistant response).
	minMessagesToKeep := 2
	if len(msgs) <= minMessagesToKeep {
		return msgs
	}

	// Calculate tokens for all messages.
	type msgInfo struct {
		index  int
		tokens int64
	}

	msgInfos := make([]msgInfo, len(msgs))
	var totalTokens int64
	for i, msg := range msgs {
		tokens := estimateSingleMessageTokens(msg)
		msgInfos[i] = msgInfo{index: i, tokens: tokens}
		totalTokens += tokens
	}

	// If already under limit, return as-is.
	if totalTokens <= maxTokens {
		return msgs
	}

	// Skip system messages at the beginning (they will be re-added by the caller's PrepareStep).
	startIdx := 0
	for startIdx < len(msgs) && msgs[startIdx].Role == fantasy.MessageRoleSystem {
		startIdx++
	}

	// Subtract tokens for messages we already skipped (system messages).
	for i := 0; i < startIdx; i++ {
		totalTokens -= msgInfos[i].tokens
	}

	// Store original token count before removal.
	originalTokens := totalTokens

	// Remove from the oldest non-system message first.
	for totalTokens > maxTokens && startIdx < len(msgs)-minMessagesToKeep {
		totalTokens -= msgInfos[startIdx].tokens
		startIdx++
	}

	slog.Info("Truncated messages for summarization",
		"original_count", len(msgs),
		"new_count", len(msgs)-startIdx,
		"removed_count", startIdx,
		"original_tokens", originalTokens,
		"new_tokens", totalTokens)

	return msgs[startIdx:]
}

func (a *sessionAgent) applyToolResultReview(ctx context.Context, sessionID string, toolResult message.ToolResult, permissionMode session.PermissionMode) message.ToolResult {
	if a.reviewToolResult == nil {
		return toolResult
	}

	reviewed, err := a.reviewToolResult(ctx, sessionID, toolResult, permissionMode)
	if err != nil {
		slog.Warn("Failed to review tool result for Auto Mode", "error", err, "tool_name", toolResult.Name, "session_id", sessionID)
	}
	return reviewed
}

// estimateSingleMessageTokens estimates tokens for a single fantasy.Message.
func estimateSingleMessageTokens(msg fantasy.Message) int64 {
	return estimateMessageContentTokens(msg.Content)
}

func (a *sessionAgent) Summarize(ctx context.Context, sessionID string, opts fantasy.ProviderOptions) error {
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}
	if a.refreshCallConfig != nil {
		runtimeConfig, err := a.refreshCallConfig(ctx)
		if err != nil {
			return err
		}
		if runtimeConfig.ProviderOptions != nil {
			opts = runtimeConfig.ProviderOptions
		}
	}

	// Copy mutable fields under lock to avoid races with SetModels.
	largeModel := a.largeModel.Get()
	systemPromptPrefix := a.systemPromptPrefix.Get()
	providerCtx := defaultProviderContext()
	compactingPurpose := sessionCompactingPurposeFromContext(ctx)

	currentSession, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	if truncErr := a.truncateOversizedToolResults(ctx, sessionID); truncErr != nil {
		slog.Warn("Failed to truncate oversized tool results before summarization", "error", truncErr, "session_id", sessionID)
	}
	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		// Nothing to summarize.
		return nil
	}

	if a.hookManager != nil {
		a.hookManager.RunPreCompact(ctx, sessionID)
	}

	microCompacted, err := a.microCompactSessionMessages(ctx, sessionID, largeModel, providerCtx, msgs)
	if err != nil {
		return err
	}
	if len(microCompacted) > 0 {
		msgs = microCompacted
	}
	if shouldCollapseMessages(compactingPurpose) {
		collapsed, collapseErr := a.collapseSessionMessages(ctx, sessionID, largeModel, providerCtx, msgs)
		if collapseErr != nil {
			return collapseErr
		}
		if len(collapsed) > 0 {
			msgs = collapsed
		}
	}
	if shouldReactiveCompactMessages(compactingPurpose) {
		reactiveCompacted, reactiveErr := a.reactiveCompactSessionMessages(ctx, sessionID, largeModel, providerCtx, msgs)
		if reactiveErr != nil {
			return reactiveErr
		}
		if len(reactiveCompacted) > 0 {
			msgs = reactiveCompacted
		}
	}
	if shouldAutoCompactMessages(compactingPurpose, msgs) {
		autoCompacted, autoCompactErr := a.autoCompactSessionMessages(ctx, sessionID, largeModel, providerCtx, msgs)
		if autoCompactErr != nil {
			return autoCompactErr
		}
		if len(autoCompacted) > 0 {
			msgs = autoCompacted
		}
		postCompacted, postCompactErr := a.postCompactSessionMessages(ctx, sessionID, largeModel, providerCtx, msgs)
		if postCompactErr != nil {
			return postCompactErr
		}
		if len(postCompacted) > 0 {
			msgs = postCompacted
		}
	}

	transformedMsgs, err := a.transformSessionMessages(ctx, chatRequestStateInput{
		SessionID: sessionID,
		Agent:     "session",
		Model:     largeModel,
		Provider:  providerCtx,
		Purpose:   compactingPurpose,
		Messages:  msgs,
		Message:   message.Message{SessionID: sessionID, Role: message.User},
	})
	if err != nil {
		return err
	}
	aiMsgs, _ := a.preparePrompt(transformedMsgs)
	compacting, err := plugin.TriggerSessionCompacting(ctx, plugin.SessionCompactingInput{
		SessionID: sessionID,
		Agent:     "session",
		Model:     agentModelInfo(largeModel),
		Purpose:   compactingPurpose,
	}, plugin.SessionCompactingOutput{})
	if err != nil {
		return err
	}

	if a.filetracker != nil {
		fileContext := a.buildRecentFileContext(ctx, sessionID, int64(largeModel.CatwalkCfg.ContextWindow))
		compacting.Context = append(compacting.Context, fileContext...)
	}

	genCtx, cancel := context.WithCancel(ctx)
	genCtx = copilot.ContextWithInitiatorType(genCtx, copilot.InitiatorAgent)
	a.activeRequests.Set(sessionID, cancel)
	defer a.activeRequests.Del(sessionID)
	defer cancel()

	agent := a.agentFactory(retryableStreamModel{largeModel.Model},
		fantasy.WithSystemPrompt(string(summaryPrompt)),
		fantasy.WithUserAgent(userAgent),
	)
	summaryMessage, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:             message.Assistant,
		Model:            largeModel.ModelCfg.Model,
		Provider:         largeModel.ModelCfg.Provider,
		IsSummaryMessage: true,
	})
	if err != nil {
		return err
	}

	summaryPromptText := buildSessionCompactingPrompt(currentSession.Todos, compacting.Context, compacting.Prompt)

	// Check if aiMsgs exceeds context window limit and truncate if necessary.
	// This prevents 400 Bad Request when the context is too large for the model.
	contextWindow := int64(largeModel.CatwalkCfg.ContextWindow)
	if contextWindow > 0 {
		estimatedTokens := estimatePromptTokens(aiMsgs, nil)
		// Leave room for system prompt, summary prompt, and output tokens.
		maxAllowedTokens := contextWindow - 4000 // Reserve 4k for safety.
		if maxAllowedTokens < 0 {
			maxAllowedTokens = contextWindow * 3 / 4 // Fallback: use 75% of context window.
		}
		if estimatedTokens > maxAllowedTokens {
			slog.Warn("Messages exceed context window, truncating before summarization",
				"estimated_tokens", estimatedTokens,
				"max_allowed", maxAllowedTokens,
				"context_window", contextWindow,
				"session_id", sessionID)
			aiMsgs = truncateMessagesToFit(aiMsgs, maxAllowedTokens)
		}
	}

	resp, err := agent.Stream(genCtx, fantasy.AgentStreamCall{
		Prompt:          summaryPromptText,
		Messages:        aiMsgs,
		ProviderOptions: opts,
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			// Summarization is always agent-initiated (never billable).
			callContext = copilot.ContextWithInitiatorType(callContext, copilot.InitiatorAgent)
			prepared.Messages = options.Messages
			if systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
		OnReasoningDelta: func(id string, text string) error {
			summaryMessage.AppendReasoningContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			// Handle anthropic signature.
			if anthropicData, ok := reasoning.ProviderMetadata["anthropic"]; ok {
				if signature, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok && signature.Signature != "" {
					summaryMessage.AppendReasoningSignature(signature.Signature)
				}
			}
			summaryMessage.FinishThinking()
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnTextDelta: func(id, text string) error {
			summaryMessage.AppendContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
	})
	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		isContextLengthErr := isContextLengthError(err)
		if isCancelErr || isContextLengthErr {
			// User cancelled or context too long - remove the summary message.
			deleteErr := a.messages.Delete(ctx, summaryMessage.ID)
			if isContextLengthErr {
				if deleteErr != nil {
					slog.Warn("Failed to delete summary message after context length error", "error", deleteErr, "session_id", sessionID, "message_id", summaryMessage.ID)
				}
				return fmt.Errorf("context too long for summarization: %w", err)
			}
			return deleteErr
		}
		return err
	}

	summaryMessage.AddFinish(message.FinishReasonEndTurn, "", "")
	err = a.messages.Update(genCtx, summaryMessage)
	if err != nil {
		return err
	}

	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	// Compute an estimate so the fallback in updateSessionUsage can correct
	// for proxies that under-report input tokens during summarization.
	summarizeEstimatedPromptTokens := estimatePromptTokens(aiMsgs, nil)
	a.updateSessionUsage(largeModel, &currentSession, resp.TotalUsage, openrouterCost, summarizeEstimatedPromptTokens)

	currentSession.SummaryMessageID = summaryMessage.ID
	_, err = a.sessions.Save(genCtx, currentSession)
	if err == nil && a.hookManager != nil {
		a.hookManager.RunPostCompact(ctx, sessionID)
	}
	return err
}

func (a *sessionAgent) getCacheControlOptions() fantasy.ProviderOptions {
	if t, _ := strconv.ParseBool(os.Getenv("CRUSH_DISABLE_ANTHROPIC_CACHE")); t {
		return fantasy.ProviderOptions{}
	}
	return fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
		bedrock.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
		vercel.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
	}
}

func (a *sessionAgent) createUserMessage(ctx context.Context, call SessionAgentCall) (message.Message, error) {
	parts := []message.ContentPart{message.TextContent{Text: call.Prompt}}
	var attachmentParts []message.ContentPart
	for _, attachment := range call.Attachments {
		attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
	}
	parts = append(parts, attachmentParts...)
	msg, err := a.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	})
	if err != nil {
		return message.Message{}, fmt.Errorf("failed to create user message: %w", err)
	}
	return msg, nil
}

func (a *sessionAgent) preparePrompt(msgs []message.Message, attachments ...message.Attachment) ([]fantasy.Message, []fantasy.FilePart) {
	var history []fantasy.Message
	// Build a set of tool-call IDs that already have a tool-result so we can
	// detect orphaned tool_use blocks below.
	toolResultIDs := make(map[string]bool)
	for _, m := range msgs {
		if m.Role == message.Tool {
			for _, tr := range m.ToolResults() {
				toolResultIDs[tr.ToolCallID] = true
			}
		}
	}

	for _, m := range msgs {
		if len(m.Parts) == 0 {
			continue
		}
		// Assistant message without content or tool calls (cancelled before it
		// returned anything).
		if m.Role == message.Assistant && len(m.ToolCalls()) == 0 && m.Content().Text == "" && m.ReasoningContent().String() == "" {
			continue
		}
		history = append(history, m.ToAIMessage()...)

		// Defensive: if this assistant message contains tool_use blocks
		// without corresponding tool_result messages anywhere in the
		// session, inject synthetic error results so the provider never
		// rejects the request with a "missing tool_result" error.
		if m.Role == message.Assistant {
			var missingParts []fantasy.MessagePart
			for _, tc := range m.ToolCalls() {
				if !toolResultIDs[tc.ID] {
					slog.Warn("Injecting synthetic tool_result for orphaned tool_use",
						"tool_call_id", tc.ID, "tool_name", tc.Name)
					missingOutput := fantasy.ToolResultOutputContentError{
						Error: fmt.Errorf("tool execution was interrupted"),
					}
					missingPart := fantasy.ToolResultPart{
						ToolCallID: tc.ID,
						Output:     missingOutput,
					}
					missingParts = append(missingParts, missingPart)
				}
			}
			if len(missingParts) > 0 {
				history = append(history, fantasy.Message{
					Role:    fantasy.MessageRoleTool,
					Content: missingParts,
				})
			}
		}
	}

	var files []fantasy.FilePart
	for _, attachment := range attachments {
		if attachment.IsText() {
			continue
		}
		files = append(files, fantasy.FilePart{
			Filename:  attachment.FileName,
			Data:      attachment.Content,
			MediaType: attachment.MimeType,
		})
	}

	return history, files
}

func disableAnthropicThinking(opts fantasy.ProviderOptions) (fantasy.ProviderOptions, bool) {
	anthropicOpts, ok := opts[anthropic.Name].(*anthropic.ProviderOptions)
	if !ok || anthropicOpts == nil || anthropicOpts.Thinking == nil {
		return opts, false
	}

	cloned := make(fantasy.ProviderOptions, len(opts))
	for k, v := range opts {
		cloned[k] = v
	}
	sanitized := *anthropicOpts
	sanitized.Thinking = nil
	cloned[anthropic.Name] = &sanitized
	return cloned, true
}

func shouldRetryWithoutAnthropicThinking(err error, opts fantasy.ProviderOptions) bool {
	anthropicOpts, ok := opts[anthropic.Name].(*anthropic.ProviderOptions)
	if !ok || anthropicOpts == nil || anthropicOpts.Thinking == nil {
		return false
	}
	var providerErr *fantasy.ProviderError
	if !errors.As(err, &providerErr) || providerErr == nil {
		return false
	}
	if providerErr.StatusCode != 400 {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(providerErr.Message))
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "thinking is enabled but reasoning_content is missing") {
		return true
	}
	hasThinking := strings.Contains(msg, "thinking")
	hasReasoning := strings.Contains(msg, "reasoning_content") || strings.Contains(msg, "reasoning content")
	hasMissing := strings.Contains(msg, "missing") || strings.Contains(msg, "required")
	isToolContext := strings.Contains(msg, "tool call") || strings.Contains(msg, "tool_use") || strings.Contains(msg, "tool use")
	return hasThinking && hasReasoning && hasMissing && isToolContext
}

func (a *sessionAgent) cleanupFailedAttempt(ctx context.Context, assistant *message.Message, toolMessageIDs []string) error {
	for _, toolMessageID := range toolMessageIDs {
		if err := a.messages.Delete(ctx, toolMessageID); err != nil {
			return err
		}
	}
	if assistant == nil {
		return nil
	}
	return a.messages.Delete(ctx, assistant.ID)
}

func (a *sessionAgent) getSessionMessages(ctx context.Context, session session.Session) ([]message.Message, error) {
	msgs, err := a.messages.List(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	if session.SummaryMessageID != "" {
		summaryMsgIndex := -1
		for i, msg := range msgs {
			if msg.ID == session.SummaryMessageID {
				summaryMsgIndex = i
				break
			}
		}
		if summaryMsgIndex != -1 {
			msgs = msgs[summaryMsgIndex:]
			msgs[0].Role = message.User
		}
	}
	return filterAutoModePromptMessages(msgs, session.PermissionMode), nil
}

func (a *sessionAgent) getHistoryForMemoryExtraction(ctx context.Context, sessionID string) []string {
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return nil
	}

	var history []string
	for _, msg := range msgs {
		switch msg.Role {
		case message.User:
			if text := msg.Content().Text; text != "" {
				history = append(history, "USER: "+text)
			}
		case message.Assistant:
			if text := msg.Content().Text; text != "" {
				history = append(history, "ASSISTANT: "+text)
			}
		}
	}
	return history
}

func shouldGenerateSessionTitle(title string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		return true
	}
	if strings.EqualFold(title, "New Session") {
		return true
	}
	return strings.EqualFold(title, DefaultSessionName)
}

func titlePromptFromCallOrHistory(prompt string, history []message.Message) string {
	if titlePrompt := titleUserPromptFromCall(prompt); titlePrompt != "" {
		return titlePrompt
	}
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role != message.User {
			continue
		}
		if titlePrompt := titleUserPromptFromCall(msg.Content().Text); titlePrompt != "" {
			return titlePrompt
		}
	}
	return ""
}

// generateTitle generates a session titled based on the initial prompt.
func titleUserPromptFromCall(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ""
	}
	for _, prefix := range []string{autoResumePromptPrefix, contextWindowResumePromptPrefix} {
		if !strings.HasPrefix(prompt, prefix) {
			continue
		}
		trimmed := strings.TrimPrefix(prompt, prefix)
		if end := strings.LastIndex(trimmed, "`"); end >= 0 {
			trimmed = trimmed[:end]
		}
		return strings.TrimSpace(trimmed)
	}
	return prompt
}

func (a *sessionAgent) generateTitle(ctx context.Context, sessionID string, userPrompt string, sessionLock *sync.Mutex) {
	userPrompt = titleUserPromptFromCall(userPrompt)
	if userPrompt == "" {
		return
	}

	smallModel := a.smallModel.Get()
	largeModel := a.largeModel.Get()
	systemPromptPrefix := a.systemPromptPrefix.Get()

	const maxOutputTokens int64 = 40

	newAgent := func(m fantasy.LanguageModel, p []byte, tok int64) fantasy.Agent {
		return fantasy.NewAgent(m,
			fantasy.WithSystemPrompt(string(p)+"\n/no_think"),
			fantasy.WithMaxOutputTokens(tok),
			fantasy.WithUserAgent(userAgent),
		)
	}

	var streamedTitle strings.Builder
	streamCall := fantasy.AgentStreamCall{
		Prompt: userPrompt,
		PrepareStep: func(callCtx context.Context, opts fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			// Title generation is always agent-initiated (never billable).
			callCtx = copilot.ContextWithInitiatorType(callCtx, copilot.InitiatorAgent)
			prepared.Messages = opts.Messages
			if systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{
					fantasy.NewSystemMessage(systemPromptPrefix),
				}, prepared.Messages...)
			}
			return callCtx, prepared, nil
		},
		OnTextDelta: func(_ string, text string) error {
			streamedTitle.WriteString(text)
			return nil
		},
	}

	// Use the small model to generate the title.
	model := smallModel
	agent := newAgent(model.Model, titlePrompt, maxOutputTokens)
	titleCtx := copilot.ContextWithInitiatorType(ctx, copilot.InitiatorAgent)
	resp, err := agent.Stream(titleCtx, streamCall)
	if err == nil {
		// We successfully generated a title with the small model.
		slog.Debug("Generated title with small model")
	} else {
		// It didn't work. Let's try with the big model.
		slog.Error("Error generating title with small model; trying big model", "err", err)
		model = largeModel
		agent = newAgent(model.Model, titlePrompt, maxOutputTokens)
		streamedTitle.Reset()
		resp, err = agent.Stream(titleCtx, streamCall)
		if err == nil {
			slog.Debug("Generated title with large model")
		} else {
			// Welp, the large model didn't work either. Use the default
			// session name and return.
			slog.Error("Error generating title with large model", "err", err)
			if sessionLock != nil {
				sessionLock.Lock()
				defer sessionLock.Unlock()
			}
			saveErr := a.sessions.Rename(ctx, sessionID, DefaultSessionName)
			if saveErr != nil {
				slog.Error("Failed to save session title", "error", saveErr)
			}
			return
		}
	}

	if resp == nil {
		// Actually, we didn't get a response so we can't. Use the default
		// session name and return.
		slog.Error("Response is nil; can't generate title")
		if sessionLock != nil {
			sessionLock.Lock()
			defer sessionLock.Unlock()
		}
		saveErr := a.sessions.Rename(ctx, sessionID, DefaultSessionName)
		if saveErr != nil {
			slog.Error("Failed to save session title", "error", saveErr)
		}
		return
	}

	// Clean up title.
	title := streamedTitle.String()
	if strings.TrimSpace(title) == "" {
		title = resp.Response.Content.Text()
	}
	title = strings.ReplaceAll(title, "\n", " ")

	// Remove thinking tags if present.
	title = thinkTagRegex.ReplaceAllString(title, "")

	title = strings.TrimSpace(title)
	title = cmp.Or(title, DefaultSessionName)

	// Calculate usage and cost.
	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	modelConfig := model.CatwalkCfg
	cost := modelConfig.CostPer1MInCached/1e6*float64(resp.TotalUsage.CacheCreationTokens) +
		modelConfig.CostPer1MOutCached/1e6*float64(resp.TotalUsage.CacheReadTokens) +
		modelConfig.CostPer1MIn/1e6*float64(resp.TotalUsage.InputTokens) +
		modelConfig.CostPer1MOut/1e6*float64(resp.TotalUsage.OutputTokens)

	// Use override cost if available (e.g., from OpenRouter).
	if openrouterCost != nil {
		cost = *openrouterCost
	}

	promptTokens := promptTokensForUsage(resp.TotalUsage, usageProvider(model))
	completionTokens := resp.TotalUsage.OutputTokens

	// Atomically update only title and usage fields to avoid overriding other
	// concurrent session updates.
	if sessionLock != nil {
		sessionLock.Lock()
		defer sessionLock.Unlock()
	}
	saveErr := a.sessions.UpdateTitleAndUsage(ctx, sessionID, title, promptTokens, completionTokens, cost)
	if saveErr != nil {
		slog.Error("Failed to save session title and usage", "error", saveErr)
		return
	}
}

func (a *sessionAgent) openrouterCost(metadata fantasy.ProviderMetadata) *float64 {
	openrouterMetadata, ok := metadata[openrouter.Name]
	if !ok {
		return nil
	}

	opts, ok := openrouterMetadata.(*openrouter.ProviderMetadata)
	if !ok {
		return nil
	}
	return &opts.Usage.Cost
}

func usageProvider(model Model) string {
	if model.Model != nil {
		if provider := model.Model.Provider(); provider != "" {
			return provider
		}
	}
	return model.ModelCfg.Provider
}

func effectiveContextWindow(model Model) int64 {
	window := int64(model.CatwalkCfg.ContextWindow)
	options := model.CatwalkCfg.Options.ProviderOptions
	if options == nil {
		return window
	}
	value, ok := options["max_prompt_tokens"]
	if !ok {
		return window
	}
	maxPromptTokens, ok := int64ProviderOptionValue(value)
	if !ok || maxPromptTokens <= 0 {
		return window
	}
	if window <= 0 {
		return maxPromptTokens
	}
	return min(window, maxPromptTokens)
}

func int64ProviderOptionValue(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case json.Number:
		parsed, err := v.Int64()
		if err == nil {
			return parsed, true
		}
		f, ferr := v.Float64()
		if ferr != nil {
			return 0, false
		}
		return int64(f), true
	default:
		return 0, false
	}
}

func isAnthropicStyleUsageProvider(providerID string) bool {
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	if providerID == "" {
		return false
	}
	switch providerID {
	case "anthropic", "anthropic-proxy", "bedrock":
		return true
	default:
		return strings.Contains(providerID, "anthropic") || strings.Contains(providerID, "bedrock")
	}
}

func promptTokensForUsage(usage fantasy.Usage, providerID string) int64 {
	// Anthropic and Bedrock report InputTokens WITHOUT cached tokens.
	// OpenAI and other providers report InputTokens INCLUDING cached tokens.
	// See: https://github.com/vercel/ai/issues/8794
	if isAnthropicStyleUsageProvider(providerID) {
		return usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens
	}
	// For OpenAI, Google, etc., InputTokens already includes cached tokens.
	// Only add CacheCreationTokens (rare).
	// Note: ReasoningTokens are output tokens (part of completion_tokens), not input tokens.
	return usage.InputTokens + usage.CacheCreationTokens
}

func totalTokensForUsage(usage fantasy.Usage, providerID string) int64 {
	return promptTokensForUsage(usage, providerID) + usage.OutputTokens
}

func autoSummarizeReservedTokens(maxOutputTokens int64) int64 {
	if maxOutputTokens <= 0 {
		return autoSummarizeReserveTokens
	}
	return min(autoSummarizeReserveTokens, maxOutputTokens)
}

func autoSummarizeToolReserveTokens(contextWindow int64) int64 {
	if contextWindow <= 0 {
		return 0
	}
	return min(autoSummarizeToolReserveMax, max(autoSummarizeToolReserveMin, contextWindow/10))
}

func autoSummarizeSafetyReserveTokens(contextWindow int64) int64 {
	if contextWindow <= 0 {
		return 0
	}
	return max(autoSummarizeSafetyReserveMin, contextWindow/50)
}

func shouldAutoSummarize(contextUsed, contextWindow, maxOutputTokens int64) bool {
	if contextWindow <= 0 {
		slog.Warn("ShouldAutoSummarize: contextWindow <= 0, returning false", "contextWindow", contextWindow)
		return false
	}
	reserved := autoSummarizeReservedTokens(maxOutputTokens)
	toolReserve := autoSummarizeToolReserveTokens(contextWindow)
	safetyReserve := autoSummarizeSafetyReserveTokens(contextWindow)
	hardLimit := contextWindow - reserved - toolReserve - safetyReserve
	softLimit := contextWindow * autoSummarizeSoftLimitNumerator / autoSummarizeSoftLimitDenominator
	usable := min(hardLimit, softLimit)

	slog.Info("ShouldAutoSummarize calculation",
		"contextUsed", contextUsed,
		"contextWindow", contextWindow,
		"maxOutputTokens", maxOutputTokens,
		"reserved", reserved,
		"toolReserve", toolReserve,
		"safetyReserve", safetyReserve,
		"hardLimit", hardLimit,
		"softLimit", softLimit,
		"usable", usable,
		"shouldSummarize", contextUsed >= usable)

	if usable <= 0 {
		slog.Warn("ShouldAutoSummarize: usable <= 0, forcing summarize", "usable", usable)
		return true
	}
	return contextUsed >= usable
}

func estimateStringTokens(s string) int64 {
	if s == "" {
		return 0
	}
	return estimateTextTokens(s, true)
}

func estimateTextTokens(s string, roundUpASCII bool) int64 {
	if s == "" {
		return 0
	}
	asciiBytes, nonASCIIRunes := estimateTextTokenUnits(s)
	return estimateTextTokensFromUnits(asciiBytes, nonASCIIRunes, roundUpASCII)
}

func estimateTextTokenUnits(s string) (asciiBytes int64, nonASCIIRunes int64) {
	for _, r := range s {
		if r < utf8.RuneSelf {
			asciiBytes++
			continue
		}
		nonASCIIRunes++
	}
	return asciiBytes, nonASCIIRunes
}

func estimateTextTokensFromUnits(asciiBytes, nonASCIIRunes int64, roundUpASCII bool) int64 {
	asciiTokens := asciiBytes / 4
	if roundUpASCII && asciiBytes%4 != 0 {
		asciiTokens++
	}
	return asciiTokens + nonASCIIRunes
}

// estimatedImageTokens is a rough estimate for a compressed image.
// Anthropic's vision token calculation is based on resolution:
// ~1333x1000 pixels ≈ 1500 tokens. Since images are compressed to max
// 2048px dimension, we use 2000 as a reasonable upper bound estimate.
// This is far more accurate than treating base64 bytes as text (which
// would estimate ~250,000 tokens for an 800KB image).
const estimatedImageTokens = 2000

func estimateMessageContentTokens(parts []fantasy.MessagePart) int64 {
	var asciiBytes int64
	var nonASCIIRunes int64
	var imageCount int64

	accumulate := func(s string) {
		partASCII, partNonASCII := estimateTextTokenUnits(s)
		asciiBytes += partASCII
		nonASCIIRunes += partNonASCII
	}

	accumulateData := func(data []byte, mediaType string) {
		if len(data) == 0 {
			return
		}
		if isImageMediaType(mediaType) {
			imageCount++
		} else {
			asciiBytes += int64(len(data))
		}
	}

	for _, part := range parts {
		switch p := part.(type) {
		case fantasy.TextPart:
			accumulate(p.Text)
		case fantasy.ReasoningPart:
			accumulate(p.Text)
		case fantasy.ToolCallPart:
			accumulate(p.Input)
		case fantasy.FilePart:
			accumulateData(p.Data, p.MediaType)
			accumulate(p.Filename)
			accumulate(p.MediaType)
		case fantasy.ToolResultPart:
			if txt, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](p.Output); ok {
				accumulate(txt.Text)
			} else if errOut, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](p.Output); ok && errOut.Error != nil {
				accumulate(errOut.Error.Error())
			} else if mediaOut, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](p.Output); ok {
				accumulateData([]byte(mediaOut.Data), mediaOut.MediaType)
				accumulate(mediaOut.MediaType)
				accumulate(mediaOut.Text)
			}
		}
	}

	textTokens := estimateTextTokensFromUnits(asciiBytes, nonASCIIRunes, false)
	return textTokens + (imageCount * estimatedImageTokens)
}

func isImageMediaType(mediaType string) bool {
	return mediaType == "image/png" ||
		mediaType == "image/jpeg" ||
		mediaType == "image/gif" ||
		mediaType == "image/webp" ||
		mediaType == "image/bmp"
}

func (a *sessionAgent) estimateSessionPromptTokens(history []fantasy.Message, prompt string, attachments []message.Attachment, tools []fantasy.AgentTool, systemPrompt string, promptPrefix string) int64 {
	total := estimatePromptTokens(history, tools)
	total += estimateStringTokens(systemPrompt)
	total += estimateStringTokens(promptPrefix)
	total += estimateStringTokens(message.PromptWithTextAttachments(prompt, attachments))
	return total
}

func (a *sessionAgent) estimateNextStepPromptTokens(ctx context.Context, sessionID string, tools []fantasy.AgentTool, systemPrompt string, promptPrefix string, model Model, provider plugin.ProviderContext) (int64, error) {
	currentSession, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return 0, err
	}
	state, err := a.buildChatRequestState(ctx, chatRequestStateInput{
		SessionID:      sessionID,
		Agent:          "session",
		Model:          model,
		Provider:       provider,
		Purpose:        plugin.ChatTransformPurposeNextStepEstimate,
		Messages:       msgs,
		Message:        message.Message{SessionID: sessionID, Role: message.User},
		SystemPrompt:   systemPrompt,
		PromptPrefix:   promptPrefix,
		PermissionMode: currentSession.PermissionMode,
	})
	if err != nil {
		return 0, err
	}
	return a.estimateSessionPromptTokens(state.History, "", nil, tools, state.SystemPrompt, state.PromptPrefix), nil
}

func (a *sessionAgent) EstimateSessionPromptTokensForModel(ctx context.Context, sessionID string, model Model) (int64, error) {
	return a.estimateNextStepPromptTokens(
		ctx,
		sessionID,
		a.tools.Copy(),
		a.systemPrompt.Get(),
		a.systemPromptPrefix.Get(),
		model,
		defaultProviderContext(),
	)
}

func applyRuntimeConfig(call *SessionAgentCall, runtimeConfig sessionAgentRuntimeConfig) {
	if runtimeConfig.ProviderOptions != nil {
		call.ProviderOptions = runtimeConfig.ProviderOptions
	}
	if runtimeConfig.MaxOutputTokens > 0 {
		call.MaxOutputTokens = runtimeConfig.MaxOutputTokens
	}
	if runtimeConfig.Temperature != nil {
		call.Temperature = runtimeConfig.Temperature
	}
	if runtimeConfig.TopP != nil {
		call.TopP = runtimeConfig.TopP
	}
	if runtimeConfig.TopK != nil {
		call.TopK = runtimeConfig.TopK
	}
	if runtimeConfig.FrequencyPenalty != nil {
		call.FrequencyPenalty = runtimeConfig.FrequencyPenalty
	}
	if runtimeConfig.PresencePenalty != nil {
		call.PresencePenalty = runtimeConfig.PresencePenalty
	}
}

func (a *sessionAgent) refreshCallConfigIfNeeded(ctx context.Context, call *SessionAgentCall) (*sessionAgentRuntimeConfig, error) {
	if runtimeConfig, ok := ctx.Value(sessionAgentRuntimeConfigContextKey{}).(*sessionAgentRuntimeConfig); ok && runtimeConfig != nil {
		applyRuntimeConfig(call, *runtimeConfig)
		return runtimeConfig, nil
	}
	if runtimeConfig, ok := ctx.Value(sessionAgentRuntimeConfigContextKey{}).(sessionAgentRuntimeConfig); ok {
		applyRuntimeConfig(call, runtimeConfig)
		return &runtimeConfig, nil
	}
	if a.refreshCallConfig == nil {
		return nil, nil
	}
	runtimeConfig, err := a.refreshCallConfig(ctx)
	if err != nil {
		return nil, err
	}
	applyRuntimeConfig(call, runtimeConfig)
	return &runtimeConfig, nil
}

func (a *sessionAgent) resetRetriedStep(ctx context.Context, assistant *message.Message, toolMessageIDs []string) error {
	for _, toolMessageID := range toolMessageIDs {
		if err := a.messages.Delete(ctx, toolMessageID); err != nil {
			return err
		}
	}
	assistant.Parts = nil
	return a.messages.Update(ctx, *assistant)
}

func withRetryFailureDetails(details string, retryAttempt int) string {
	details = strings.TrimSpace(details)
	if retryAttempt <= 0 {
		return details
	}

	summary := fmt.Sprintf("Retried %d %s, but the request still failed.", retryAttempt, cmp.Or(pluralizeRetryAttempt(retryAttempt), "times"))
	if details == "" {
		return summary
	}
	return summary + " " + details
}

func pluralizeRetryAttempt(retryAttempt int) string {
	if retryAttempt == 1 {
		return "time"
	}
	return "times"
}

// estimatePromptTokens estimates the prompt token count from message content
// and tool definitions. This serves as a fallback when providers (e.g., some
// Anthropic-compatible proxies) don't report input tokens in streaming mode.
//
// All message part types are counted:
//   - TextPart / ReasoningPart: plain text bytes
//   - ToolCallPart: Input JSON string bytes
//   - ToolResultPart: text output bytes
//
// Tool definitions include the JSON-encoded parameter schema. ASCII-heavy
// content is estimated at roughly 4 bytes per token, while non-ASCII runes
// count as at least one token each so CJK-heavy prompts are not badly
// under-estimated.
func estimatePromptTokens(messages []fantasy.Message, tools []fantasy.AgentTool) int64 {
	var totalTokens int64
	for _, msg := range messages {
		totalTokens += estimateMessageContentTokens(msg.Content)
	}
	for _, tool := range tools {
		info := tool.Info()
		totalTokens += estimateTextTokens(info.Name, false)
		totalTokens += estimateTextTokens(info.Description, false)
		if schemaJSON, err := json.Marshal(info.Parameters); err == nil {
			totalTokens += estimateTextTokens(string(schemaJSON), false)
		} else {
			totalTokens += 75
		}
	}
	return totalTokens
}

func (a *sessionAgent) updateSessionUsage(model Model, session *session.Session, usage fantasy.Usage, overrideCost *float64, estimatedPromptTokens int64) {
	modelConfig := model.CatwalkCfg
	cost := modelConfig.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		modelConfig.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		modelConfig.CostPer1MIn/1e6*float64(usage.InputTokens) +
		modelConfig.CostPer1MOut/1e6*float64(usage.OutputTokens)

	a.eventTokensUsed(session.ID, model, usage, cost)

	if overrideCost != nil {
		session.Cost += *overrideCost
	} else {
		session.Cost += cost
	}

	promptTokens := promptTokensForUsage(usage, usageProvider(model))
	// Some providers (e.g., Anthropic-compatible proxies) under-report or
	// return stale input token counts in streaming mode — they may report
	// only user-message tokens, omit system prompt and tool definitions, or
	// return a constant value that does not grow across tool-call steps.
	// Use the higher of the API-reported value and the byte-based estimate
	// so the context-window display keeps pace with the actual conversation
	// size. The estimate is rough (total bytes / 4) but directionally
	// correct and guaranteed to grow as messages accumulate.
	if estimatedPromptTokens > 0 && promptTokens < estimatedPromptTokens {
		promptTokens = estimatedPromptTokens
	}

	session.CompletionTokens += usage.OutputTokens
	session.PromptTokens += promptTokens
	session.LastPromptTokens = promptTokens
	session.LastCompletionTokens = usage.OutputTokens
}

func (a *sessionAgent) Cancel(sessionID string) {
	// Cancel regular requests. Don't use Take() here - we need the entry to
	// remain in activeRequests so IsBusy() returns true until the goroutine
	// fully completes (including error handling that may access the DB).
	// The defer in processRequest will clean up the entry.
	if cancel, ok := a.activeRequests.Get(sessionID); ok && cancel != nil {
		slog.Debug("Request cancellation initiated", "session_id", sessionID)
		cancel()
	}

	// Also check for summarize requests.
	if cancel, ok := a.activeRequests.Get(sessionID + "-summarize"); ok && cancel != nil {
		slog.Debug("Summarize cancellation initiated", "session_id", sessionID)
		cancel()
	}

	if a.QueuedPrompts(sessionID) > 0 {
		slog.Debug("Clearing queued prompts", "session_id", sessionID)
		a.clearQueuedCalls(sessionID)
	}
	a.pausedQueues.Del(sessionID)
}

func (a *sessionAgent) RemoveQueuedPrompt(sessionID string, index int) bool {
	if !a.removeQueuedCall(sessionID, index) {
		return false
	}

	slog.Debug("Removing queued prompt", "session_id", sessionID, "index", index)
	if a.QueuedPrompts(sessionID) == 0 {
		a.pausedQueues.Del(sessionID)
	}
	return true
}

func (a *sessionAgent) ClearQueue(sessionID string) {
	if a.QueuedPrompts(sessionID) > 0 {
		slog.Debug("Clearing queued prompts", "session_id", sessionID)
		a.clearQueuedCalls(sessionID)
	}
	// Auto-unpause when the queue is cleared.
	a.pausedQueues.Del(sessionID)
}

func (a *sessionAgent) PrioritizeQueuedPrompt(sessionID string, index int) bool {
	if !a.prioritizeQueuedCall(sessionID, index) {
		return false
	}
	slog.Debug("Prioritizing queued prompt", "session_id", sessionID, "index", index)
	return true
}

// PauseQueue pauses automatic processing of queued prompts for the session.
// The current request (if any) continues, but the next queued prompt won't
// be automatically started. Use this to stop the queue without clearing it.
func (a *sessionAgent) PauseQueue(sessionID string) {
	a.pausedQueues.Set(sessionID, true)
	slog.Debug("Queue paused", "session_id", sessionID)
}

// ResumeQueue resumes automatic processing of queued prompts for the session.
// If there are queued prompts and no active request, it starts the next one.
func (a *sessionAgent) ResumeQueue(sessionID string) {
	a.pausedQueues.Del(sessionID)
	slog.Debug("Queue resumed", "session_id", sessionID)

	if a.IsSessionBusy(sessionID) {
		return
	}
	firstQueuedMessage, ok := a.popNextQueuedCall(sessionID)
	if !ok {
		return
	}
	go func(call SessionAgentCall) {
		if _, err := a.Run(context.Background(), call); err != nil {
			slog.Warn("Failed to resume queued prompt", "session_id", sessionID, "error", err)
		}
	}(firstQueuedMessage)
}

// IsQueuePaused reports whether the queue is paused for the session.
func (a *sessionAgent) IsQueuePaused(sessionID string) bool {
	paused, _ := a.pausedQueues.Get(sessionID)
	return paused
}

func (a *sessionAgent) CancelAll() {
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

	a.extractionMu.Lock()
	pending := len(a.pendingExtractions)
	a.extractionMu.Unlock()
	if pending > 0 {
		slog.Debug("Waiting for pending memory extractions", "count", pending)
		time.Sleep(2 * time.Second)
	}
}

func (a *sessionAgent) IsBusy() bool {
	var busy bool
	for cancelFunc := range a.activeRequests.Seq() {
		if cancelFunc != nil {
			busy = true
			break
		}
	}
	return busy
}

func (a *sessionAgent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Get(sessionID)
	return busy
}

func (a *sessionAgent) QueuedPrompts(sessionID string) int {
	return len(a.queuedCallsSnapshot(sessionID))
}

func (a *sessionAgent) QueuedPromptsList(sessionID string) []string {
	l := a.queuedCallsSnapshot(sessionID)
	prompts := make([]string, len(l))
	for i, call := range l {
		prompts[i] = call.Prompt
	}
	return prompts
}

func (a *sessionAgent) enqueueQueuedCall(sessionID string, call SessionAgentCall) {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()

	queuedCalls, _ := a.messageQueue.Get(sessionID)
	queuedCalls = append(append([]SessionAgentCall(nil), queuedCalls...), call)
	a.setQueuedCallsLocked(sessionID, queuedCalls)
}

func (a *sessionAgent) takeJoinActiveRunCalls(sessionID string) []SessionAgentCall {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()

	queuedCalls, ok := a.messageQueue.Get(sessionID)
	if !ok || len(queuedCalls) == 0 {
		return nil
	}

	joinActiveRunCalls := make([]SessionAgentCall, 0, len(queuedCalls))
	remainingCalls := make([]SessionAgentCall, 0, len(queuedCalls))
	for _, queuedCall := range queuedCalls {
		if queuedCall.JoinActiveRun {
			joinActiveRunCalls = append(joinActiveRunCalls, queuedCall)
			continue
		}
		remainingCalls = append(remainingCalls, queuedCall)
	}
	a.setQueuedCallsLocked(sessionID, remainingCalls)
	return joinActiveRunCalls
}

func (a *sessionAgent) popNextQueuedCall(sessionID string) (SessionAgentCall, bool) {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()

	queuedCalls, ok := a.messageQueue.Get(sessionID)
	if !ok || len(queuedCalls) == 0 {
		return SessionAgentCall{}, false
	}

	nextCall := queuedCalls[0]
	a.setQueuedCallsLocked(sessionID, queuedCalls[1:])
	return nextCall, true
}

func (a *sessionAgent) removeQueuedCall(sessionID string, index int) bool {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()

	queuedCalls, ok := a.messageQueue.Get(sessionID)
	if !ok || index < 0 || index >= len(queuedCalls) {
		return false
	}

	updatedQueue := append(queuedCalls[:index:index], queuedCalls[index+1:]...)
	a.setQueuedCallsLocked(sessionID, updatedQueue)
	return true
}

func (a *sessionAgent) prioritizeQueuedCall(sessionID string, index int) bool {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()

	queuedCalls, ok := a.messageQueue.Get(sessionID)
	if !ok || index < 0 || index >= len(queuedCalls) {
		return false
	}

	call := queuedCalls[index]
	call.JoinActiveRun = true

	newQueue := make([]SessionAgentCall, 0, len(queuedCalls))
	newQueue = append(newQueue, call)
	newQueue = append(newQueue, queuedCalls[:index]...)
	newQueue = append(newQueue, queuedCalls[index+1:]...)
	a.setQueuedCallsLocked(sessionID, newQueue)
	return true
}

func (a *sessionAgent) clearQueuedCalls(sessionID string) {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()

	a.messageQueue.Del(sessionID)
}

func (a *sessionAgent) queuedCallsSnapshot(sessionID string) []SessionAgentCall {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()

	queuedCalls, ok := a.messageQueue.Get(sessionID)
	if !ok || len(queuedCalls) == 0 {
		return nil
	}
	return append([]SessionAgentCall(nil), queuedCalls...)
}

func (a *sessionAgent) setQueuedCallsLocked(sessionID string, queuedCalls []SessionAgentCall) {
	if len(queuedCalls) == 0 {
		a.messageQueue.Del(sessionID)
		return
	}
	a.messageQueue.Set(sessionID, append([]SessionAgentCall(nil), queuedCalls...))
}

func (a *sessionAgent) SetModels(large Model, small Model) {
	a.largeModel.Set(large)
	a.smallModel.Set(small)
}

func (a *sessionAgent) SetTools(tools []fantasy.AgentTool) {
	a.tools.SetSlice(tools)
}

func (a *sessionAgent) SetSystemPrompt(systemPrompt string) {
	a.systemPrompt.Set(systemPrompt)
}

func (a *sessionAgent) SetSystemPromptPrefix(systemPromptPrefix string) {
	a.systemPromptPrefix.Set(systemPromptPrefix)
}

func (a *sessionAgent) Model() Model {
	return a.largeModel.Get()
}

// convertToToolResult converts a fantasy tool result to a message tool result.
func (a *sessionAgent) convertToToolResult(result fantasy.ToolResultContent) message.ToolResult {
	baseResult := message.ToolResult{
		ToolCallID: result.ToolCallID,
		Name:       result.ToolName,
		Metadata:   result.ClientMetadata,
	}

	switch result.Result.GetType() {
	case fantasy.ToolResultContentTypeText:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result.Result); ok {
			baseResult.Content = r.Text
		}
	case fantasy.ToolResultContentTypeError:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result.Result); ok {
			baseResult.Content = r.Error.Error()
			baseResult.IsError = true
		}
	case fantasy.ToolResultContentTypeMedia:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](result.Result); ok {
			content := r.Text
			if content == "" {
				content = fmt.Sprintf("Loaded %s content", r.MediaType)
			}
			baseResult.Content = content
			baseResult.Data = r.Data
			baseResult.MIMEType = r.MediaType
		}
	}

	return baseResult
}

const (
	mcpAdditionalMediaMetadataKey = "mcp_additional_media"
)

type additionalMediaItem struct {
	Type      string `json:"type"`
	Data      string `json:"data"`
	MediaType string `json:"media_type"`
}

func (a *sessionAgent) extractAdditionalMCPMedia(toolResult message.ToolResult) (message.ToolResult, []message.BinaryContent) {
	if toolResult.Metadata == "" {
		return toolResult, nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(toolResult.Metadata), &payload); err != nil {
		return toolResult, nil
	}

	rawAdditional, ok := payload[mcpAdditionalMediaMetadataKey]
	if !ok {
		return toolResult, nil
	}

	var additional []additionalMediaItem
	if err := json.Unmarshal(rawAdditional, &additional); err != nil {
		slog.Warn("Failed to decode MCP additional media metadata", "error", err, "tool_name", toolResult.Name, "tool_call_id", toolResult.ToolCallID)
		return toolResult, nil
	}

	delete(payload, mcpAdditionalMediaMetadataKey)
	if len(payload) == 0 {
		toolResult.Metadata = ""
	} else if cleaned, err := json.Marshal(payload); err != nil {
		slog.Warn("Failed to re-encode tool metadata after removing additional media", "error", err, "tool_name", toolResult.Name, "tool_call_id", toolResult.ToolCallID)
	} else {
		toolResult.Metadata = string(cleaned)
	}

	media := make([]message.BinaryContent, 0, len(additional))
	for index, item := range additional {
		if strings.TrimSpace(item.Data) == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(item.Data)
		if err != nil {
			slog.Warn("Failed to decode additional MCP media payload", "error", err, "tool_name", toolResult.Name, "tool_call_id", toolResult.ToolCallID)
			continue
		}
		mediaType := strings.TrimSpace(item.MediaType)
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		media = append(media, message.BinaryContent{
			Path:     fmt.Sprintf("tool-result-%s-extra-%d", toolResult.ToolCallID, index+1),
			MIMEType: mediaType,
			Data:     decoded,
		})
	}

	if len(media) == 0 {
		return toolResult, nil
	}
	return toolResult, media
}

// workaroundProviderMediaLimitations converts media content in tool results to
// user messages for providers that don't natively support images in tool results.
//
// Problem: OpenAI, Google, OpenRouter, and other OpenAI-compatible providers
// don't support sending images/media in tool result messages - they only accept
// text in tool results. However, they DO support images in user messages.
//
// If we send media in tool results to these providers, the API returns an error.
//
// Solution: For these providers, we:
//  1. Replace the media in the tool result with a text placeholder
//  2. Inject a user message immediately after with the image as a file attachment
//  3. This maintains the tool execution flow while working around API limitations
//
// Anthropic and Bedrock support images natively in tool results, so we skip
// this workaround for them.
//
// Example transformation:
//
//	BEFORE: [tool result: image data]
//	AFTER:  [tool result: "Image loaded - see attached"], [user: image attachment]
func (a *sessionAgent) workaroundProviderMediaLimitations(messages []fantasy.Message, largeModel Model) []fantasy.Message {
	providerSupportsMedia := largeModel.ModelCfg.Provider == string(catwalk.InferenceProviderAnthropic) ||
		largeModel.ModelCfg.Provider == string(catwalk.InferenceProviderBedrock)

	if providerSupportsMedia {
		return messages
	}

	convertedMessages := make([]fantasy.Message, 0, len(messages))

	for _, msg := range messages {
		if msg.Role != fantasy.MessageRoleTool {
			convertedMessages = append(convertedMessages, msg)
			continue
		}

		textParts := make([]fantasy.MessagePart, 0, len(msg.Content))
		var mediaFiles []fantasy.FilePart

		for _, part := range msg.Content {
			toolResult, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part)
			if !ok {
				textParts = append(textParts, part)
				continue
			}

			if media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](toolResult.Output); ok {
				decoded, err := base64.StdEncoding.DecodeString(media.Data)
				if err != nil {
					slog.Warn("Failed to decode media data", "error", err)
					textParts = append(textParts, part)
					continue
				}

				mediaFiles = append(mediaFiles, fantasy.FilePart{
					Data:      decoded,
					MediaType: media.MediaType,
					Filename:  fmt.Sprintf("tool-result-%s", toolResult.ToolCallID),
				})

				textParts = append(textParts, fantasy.ToolResultPart{
					ToolCallID: toolResult.ToolCallID,
					Output: fantasy.ToolResultOutputContentText{
						Text: "[Image/media content loaded - see attached file]",
					},
					ProviderOptions: toolResult.ProviderOptions,
				})
			} else {
				textParts = append(textParts, part)
			}
		}

		convertedMessages = append(convertedMessages, fantasy.Message{
			Role:    fantasy.MessageRoleTool,
			Content: textParts,
		})

		if len(mediaFiles) > 0 {
			convertedMessages = append(convertedMessages, fantasy.NewUserMessage(
				"Here is the media content from the tool result:",
				mediaFiles...,
			))
		}
	}

	return convertedMessages
}

// buildSummaryPrompt constructs the prompt text for session summarization.
func buildSummaryPrompt(todos []session.Todo) string {
	var sb strings.Builder
	sb.WriteString("Provide a detailed summary of our conversation above.")
	if len(todos) > 0 {
		sb.WriteString("\n\n## Tracked Tasks\n\n")
		for _, t := range todos {
			fmt.Fprintf(&sb, "- [%s] %s\n", t.Status, t.Content)
		}
		sb.WriteString("\nInclude these tasks and their statuses in your summary.")
	}
	return sb.String()
}
