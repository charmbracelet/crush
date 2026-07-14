package agent

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/discover"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/goal"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/skills"
	"golang.org/x/sync/errgroup"

	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/azure"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openaicompat"
	"charm.land/fantasy/providers/openrouter"
	"charm.land/fantasy/providers/vercel"
	openaisdk "github.com/charmbracelet/openai-go/option"
	"github.com/qjebbs/go-jsons"
)

// Coordinator errors.
var (
	errCoderAgentNotConfigured         = errors.New("coder agent not configured")
	errAgentNotConfigured              = errors.New("agent not configured")
	errAgentSwitchWhileBusy            = errors.New("cannot switch agent while busy")
	errModelProviderNotConfigured      = errors.New("model provider not configured")
	errLargeModelNotSelected           = errors.New("large model not selected")
	errSmallModelNotSelected           = errors.New("small model not selected")
	errLargeModelProviderNotConfigured = errors.New("large model provider not configured")
	errSmallModelProviderNotConfigured = errors.New("small model provider not configured")
	errLargeModelNotFound              = errors.New("large model not found in provider config")
	errSmallModelNotFound              = errors.New("small model not found in provider config")
)

// Copilot models that use the Responses API instead of Chat Completions.
var copilotResponsesModels = map[string]bool{
	"gpt-5.2":       true,
	"gpt-5.2-codex": true,
	"gpt-5.3-codex": true,
	"gpt-5.4":       true,
	"gpt-5.4-mini":  true,
	"gpt-5.5":       true,
	"gpt-5-mini":    true,
}

// OpenCode models that user Anthropic Messages API instead of Chat Completions.
var opencodeMessagesModels = map[string]bool{
	"qwen3.7-max": true,
}

type Coordinator interface {
	SetMainAgent(ctx context.Context, agentID string) error
	CurrentAgentID() string
	Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error)
	// RunAccepted runs a call that was already accepted via
	// BeginAccepted on the fire-and-forget dispatch path. The handle is
	// the only carrier of accept-state across the backend.runAgent /
	// Coordinator / sessionAgent.Run layers: it reaches
	// sessionAgent.Run as SessionAgentCall.Accepted, where it is
	// consumed under dispatchMu once the accepted -> (cancel-on-entry |
	// queued | active) transition is chosen.
	RunAccepted(ctx context.Context, accept *AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error)
	BeginAccepted(sessionID string) *AcceptedRun
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	QueuedPromptsList(sessionID string) []string
	ClearQueue(sessionID string)
	Summarize(context.Context, string) error
	Model() Model
	UpdateModels(ctx context.Context) error
	SetMemoryOptions(recorderEnabled, recallEnabled bool) error
	GenerateTitle(ctx context.Context, sessionID, prompt string)
}

// SkillLoadMarker records deterministic skill activation performed outside
// the model-facing View tool, such as a user-invoked skill command.
type SkillLoadMarker interface {
	MarkSkillLoaded(name string)
}

type coordinator struct {
	cfg            *config.ConfigStore
	sessions       session.Service
	messages       message.Service
	permissions    permission.Service
	history        history.Service
	filetracker    filetracker.Service
	lspManager     *lsp.Manager
	notify         pubsub.Publisher[notify.Notification]
	runComplete    pubsub.Publisher[notify.RunComplete]
	memory         *memory.Store
	project        memory.Project
	memoryRecorder bool
	memoryRecall   bool

	currentAgent    SessionAgent
	currentAgentID  string
	agents          map[string]SessionAgent
	agentModelTypes map[string]config.SelectedModelType

	// Skills discovery results (session-start snapshot).
	allSkills    []*skills.Skill // Pre-filter: all discovered after dedup.
	activeSkills []*skills.Skill // Post-filter: active skills only.
	skillTracker *skills.Tracker

	readyWg errgroup.Group
}

func (c *coordinator) MarkSkillLoaded(name string) {
	c.skillTracker.MarkLoaded(name)
	slog.Info("Activated skill", "skill", name, "source", "runtime")
}

type CoordinatorOption func(*coordinator)

func WithMemory(store *memory.Store, project memory.Project, recorderEnabled, recallEnabled bool) CoordinatorOption {
	return func(coordinator *coordinator) {
		coordinator.memory = store
		coordinator.project = project
		coordinator.memoryRecorder = recorderEnabled
		coordinator.memoryRecall = recallEnabled
	}
}

func NewCoordinator(
	ctx context.Context,
	cfg *config.ConfigStore,
	sessions session.Service,
	messages message.Service,
	permissions permission.Service,
	history history.Service,
	filetracker filetracker.Service,
	lspManager *lsp.Manager,
	notify pubsub.Publisher[notify.Notification],
	runComplete pubsub.Publisher[notify.RunComplete],
	skillsMgr *skills.Manager,
	opts ...CoordinatorOption,
) (Coordinator, error) {
	// Skills are pre-discovered by the caller (see app.New /
	// backend.CreateWorkspace) and passed in via the manager. If no
	// manager was provided (legacy callers), fall back to an in-line
	// discovery so the coordinator still works.
	var allSkills, activeSkills []*skills.Skill
	if skillsMgr != nil {
		allSkills = skillsMgr.AllSkills()
		activeSkills = skillsMgr.ActiveSkills()
	} else {
		allSkills, activeSkills = discoverSkills(cfg)
	}
	skillTracker := skills.NewTracker(activeSkills)

	c := &coordinator{
		cfg:             cfg,
		sessions:        sessions,
		messages:        messages,
		permissions:     permissions,
		history:         history,
		filetracker:     filetracker,
		lspManager:      lspManager,
		notify:          notify,
		runComplete:     runComplete,
		agents:          make(map[string]SessionAgent),
		agentModelTypes: make(map[string]config.SelectedModelType),
		allSkills:       allSkills,
		activeSkills:    activeSkills,
		skillTracker:    skillTracker,
	}
	for _, opt := range opts {
		opt(c)
	}

	if _, ok := cfg.Config().Agents[config.AgentCoder]; !ok {
		return nil, errCoderAgentNotConfigured
	}
	if err := c.SetMainAgent(ctx, config.AgentCoder); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *coordinator) SetMainAgent(ctx context.Context, agentID string) error {
	if c.currentAgent != nil && c.currentAgent.IsBusy() {
		return errAgentSwitchWhileBusy
	}
	if c.agents == nil {
		c.agents = make(map[string]SessionAgent)
	}
	if c.agentModelTypes == nil {
		c.agentModelTypes = make(map[string]config.SelectedModelType)
	}
	agentCfg, ok := c.cfg.Config().Agents[agentID]
	if !ok || agentCfg.ID == "" {
		return fmt.Errorf("%w: %s", errAgentNotConfigured, agentID)
	}
	selectedModel := c.cfg.Config().Models[agentCfg.Model]
	if agent, ok := c.agents[agentID]; ok &&
		c.agentModelTypes[agentID] == agentCfg.Model &&
		reflect.DeepEqual(agent.Model().ModelCfg, selectedModel) {
		c.currentAgent = agent
		c.currentAgentID = agentID
		return nil
	}
	systemPrompt, err := c.promptForAgent(agentID)
	if err != nil {
		return err
	}
	agent, err := c.buildAgent(ctx, systemPrompt, agentCfg, false)
	if err != nil {
		return err
	}
	c.agents[agentID] = agent
	c.agentModelTypes[agentID] = agentCfg.Model
	c.currentAgent = agent
	c.currentAgentID = agentID
	return nil
}

func (c *coordinator) CurrentAgentID() string {
	if c.currentAgentID == "" {
		return config.AgentCoder
	}
	return c.currentAgentID
}

func (c *coordinator) promptForAgent(agentID string) (*prompt.Prompt, error) {
	opts := []prompt.Option{prompt.WithWorkingDir(c.cfg.WorkingDir())}
	switch agentID {
	case config.AgentPlan:
		return planPrompt(opts...)
	case config.AgentTask:
		return taskPrompt(opts...)
	case config.AgentReview:
		return reviewPrompt(opts...)
	default:
		return coderPrompt(opts...)
	}
}

// Run implements Coordinator.
func (c *coordinator) Run(ctx context.Context, sessionID string, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return c.run(ctx, nil, sessionID, prompt, attachments...)
}

// RunAccepted implements Coordinator.
func (c *coordinator) RunAccepted(ctx context.Context, accept *AcceptedRun, sessionID string, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return c.run(ctx, accept, sessionID, prompt, attachments...)
}

// run is the shared implementation behind Run and RunAccepted. When
// accept is non-nil it is threaded onto the SessionAgentCall as
// Accepted so sessionAgent.Run can consume the accept reservation under
// dispatchMu; when nil (the in-process/local path) no accept tracking
// applies.
func (c *coordinator) run(ctx context.Context, accept *AcceptedRun, sessionID string, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	if err := c.readyWg.Wait(); err != nil {
		return nil, err
	}

	// refresh models before each run
	if err := c.UpdateModels(ctx); err != nil {
		return nil, fmt.Errorf("failed to update models: %w", err)
	}

	originalPrompt := prompt
	transientContext := ""
	var explicitlyLoaded []string
	goalMode := c.CurrentAgentID() == config.AgentGoal
	goalBaseContext := transientContext
	goalState := goal.State{}
	if goalMode {
		currentSession, err := c.sessions.Get(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to load goal state: %w", err)
		}
		if currentSession.Goal.Objective != "" && currentSession.Goal.Status != goal.StatusComplete {
			goalState = currentSession.Goal.Resume()
		} else {
			goalState = goal.Start(originalPrompt)
		}
		if workflow, ok := findSkill(c.activeSkills, "goal-project-init"); ok {
			transientContext = appendTransientContext(transientContext, workflow.FormatInvocation())
			explicitlyLoaded = append(explicitlyLoaded, workflow.Name)
		}
		goalBaseContext = transientContext
		transientContext = appendTransientContext(transientContext, goal.Context(goalState, nil))
	}
	for _, name := range explicitlyLoaded {
		c.MarkSkillLoaded(name)
	}

	model := c.currentAgent.Model()
	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	if !model.CatwalkCfg.SupportsImages && attachments != nil {
		// filter out image attachments
		filteredAttachments := make([]message.Attachment, 0, len(attachments))
		for _, att := range attachments {
			if att.IsText() {
				filteredAttachments = append(filteredAttachments, att)
			}
		}
		attachments = filteredAttachments
	}

	providerCfg, ok := c.cfg.Config().Providers.Get(model.ModelCfg.Provider)
	if !ok {
		return nil, errModelProviderNotConfigured
	}

	mergedOptions, temp, topP, topK, freqPenalty, presPenalty := mergeCallOptions(model, providerCfg)

	if err := c.refreshTokenIfExpired(ctx, providerCfg); err != nil {
		// NOTE(@andreynering): We don't return here because the event handling to ask the user to reauthenticate
		// depends on the flow below. If refresh fails, proceed with the token we have.
		slog.Error("Failed to refresh OAuth2 token. Proceeding with existing token.", "error", err)
	}

	// Coalesce per-attempt RunComplete payloads so only the final
	// outcome reaches subscribers. Without this, the first attempt's
	// failed RunComplete (unauthorized) would race ahead of the
	// retry's success, and `crush run` would exit on the stale error
	// before ever seeing the retry result. Each attempt's
	// SessionAgentCall.OnComplete hook overwrites latest; we publish
	// exactly once after retries resolve, via PublishMustDeliver, so
	// a momentarily-full subscriber buffer can't silently drop the
	// terminal event.
	var (
		latest    notify.RunComplete
		hasLatest bool
	)
	onComplete := func(rc notify.RunComplete) {
		latest = rc
		hasLatest = true
	}
	// Propagate the caller-supplied RunID (set via agent.WithRunID
	// at the HTTP boundary in backend.SendMessage) onto the
	// SessionAgentCall so the terminal RunComplete event echoes it
	// back. Both attempts in the retry chain reuse the same RunID;
	// the coalesce closure publishes the final outcome under that
	// same correlator.
	runID := RunIDFromContext(ctx)
	run := func() (*fantasy.AgentResult, error) {
		return c.currentAgent.Run(ctx, SessionAgentCall{
			SessionID:        sessionID,
			RunID:            runID,
			Prompt:           originalPrompt,
			TransientContext: transientContext,
			Attachments:      attachments,
			MaxOutputTokens:  maxTokens,
			ProviderOptions:  mergedOptions,
			Temperature:      temp,
			TopP:             topP,
			TopK:             topK,
			FrequencyPenalty: freqPenalty,
			PresencePenalty:  presPenalty,
			OnComplete:       onComplete,
			Accepted:         accept,
			originalIntent:   originalPrompt,
			goalMode:         goalMode,
			goalBaseContext:  goalBaseContext,
			goalState:        goalState,
		})
	}
	beforeLoaded := c.skillTracker.LoadedNames()
	var result *fantasy.AgentResult
	originalErr := c.runWithUnauthorizedRetry(ctx, providerCfg, func() error {
		var err error
		result, err = run()
		return err
	})
	logTurnSkillUsage(sessionID, prompt, c.activeSkills, c.skillTracker, beforeLoaded)

	// Notify only if still unauthorized after retry — a successful
	// retry means the user doesn't need to re-authenticate.
	if originalErr != nil && c.isUnauthorized(originalErr) && c.notify != nil && model.ModelCfg.Provider == hyper.Name {
		c.notify.Publish(pubsub.CreatedEvent, notify.Notification{
			Type:       notify.TypeReAuthenticate,
			ProviderID: model.ModelCfg.Provider,
		})
	}

	if hasLatest && c.runComplete != nil {
		c.runComplete.PublishMustDeliver(ctx, pubsub.UpdatedEvent, latest)
		// Signal to the dispatcher (backend.runAgent) that the
		// authoritative terminal RunComplete for this run was already
		// emitted, so it does not publish a duplicate fallback for the
		// error it is about to receive.
		MarkRunCompletePublished(ctx)
	}
	return result, originalErr
}

func findSkill(active []*skills.Skill, name string) (*skills.Skill, bool) {
	for _, skill := range active {
		if skill != nil && skill.Name == name && skill.Instructions != "" {
			return skill, true
		}
	}
	return nil, false
}

func appendTransientContext(existing, addition string) string {
	existing = strings.TrimSpace(existing)
	addition = strings.TrimSpace(addition)
	if existing == "" {
		return addition
	}
	if addition == "" {
		return existing
	}
	return existing + "\n\n" + addition
}

// effectiveReasoningEffort returns the reasoning effort to apply for provider calls.
// It prefers the user-selected effort when valid, otherwise the model default when
// valid, and finally falls back to the first configured reasoning level.
func effectiveReasoningEffort(model Model) string {
	if !model.CatwalkCfg.CanReason {
		return ""
	}

	if effort := model.ModelCfg.ReasoningEffort; effort != "" && slices.Contains(model.CatwalkCfg.ReasoningLevels, effort) {
		return effort
	}
	if effort := model.CatwalkCfg.DefaultReasoningEffort; effort != "" && slices.Contains(model.CatwalkCfg.ReasoningLevels, effort) {
		return effort
	}
	if len(model.CatwalkCfg.ReasoningLevels) > 0 {
		return model.CatwalkCfg.ReasoningLevels[0]
	}
	return ""
}

func isLMStudioProvider(providerCfg config.ProviderConfig) bool {
	return providerCfg.Type == catwalk.Type("lmstudio")
}

func mergedExtraBody(options map[string]any) map[string]any {
	extraBody := make(map[string]any)
	if existing, ok := options["extra_body"].(map[string]any); ok {
		maps.Copy(extraBody, existing)
	}
	return extraBody
}

func applyLMStudioThinkingOption(model Model, providerCfg config.ProviderConfig, options map[string]any) {
	if !isLMStudioProvider(providerCfg) || !model.CatwalkCfg.CanReason || len(model.CatwalkCfg.ReasoningLevels) > 0 {
		return
	}
	extraBody := mergedExtraBody(options)
	extraBody["enable_thinking"] = model.ModelCfg.Think
	options["extra_body"] = extraBody
}

func getProviderOptions(model Model, providerCfg config.ProviderConfig) fantasy.ProviderOptions {
	options := fantasy.ProviderOptions{}

	cfgOpts := []byte("{}")
	providerCfgOpts := []byte("{}")
	catwalkOpts := []byte("{}")

	if model.ModelCfg.ProviderOptions != nil {
		data, err := json.Marshal(model.ModelCfg.ProviderOptions)
		if err == nil {
			cfgOpts = data
		}
	}

	if providerCfg.ProviderOptions != nil {
		data, err := json.Marshal(providerCfg.ProviderOptions)
		if err == nil {
			providerCfgOpts = data
		}
	}

	if model.CatwalkCfg.Options.ProviderOptions != nil {
		data, err := json.Marshal(model.CatwalkCfg.Options.ProviderOptions)
		if err == nil {
			catwalkOpts = data
		}
	}

	readers := []io.Reader{
		bytes.NewReader(catwalkOpts),
		bytes.NewReader(providerCfgOpts),
		bytes.NewReader(cfgOpts),
	}

	got, err := jsons.Merge(readers)
	if err != nil {
		slog.Error("Could not merge call config", "err", err)
		return options
	}

	mergedOptions := make(map[string]any)

	err = json.Unmarshal([]byte(got), &mergedOptions)
	if err != nil {
		slog.Error("Could not create config for call", "err", err)
		return options
	}

	reasoningEffort := effectiveReasoningEffort(model)
	shouldSetEffort := model.CatwalkCfg.CanReason &&
		reasoningEffort != "" &&
		slices.Contains(model.CatwalkCfg.ReasoningLevels, reasoningEffort)

	switch providerCfg.Type {
	case openai.Name, azure.Name:
		_, hasReasoningEffort := mergedOptions["reasoning_effort"]
		if !hasReasoningEffort && shouldSetEffort {
			mergedOptions["reasoning_effort"] = reasoningEffort
		}
		if openai.IsResponsesModel(model.CatwalkCfg.ID) {
			if openai.IsResponsesReasoningModel(model.CatwalkCfg.ID) {
				mergedOptions["reasoning_summary"] = "auto"
				mergedOptions["include"] = []openai.IncludeType{openai.IncludeReasoningEncryptedContent}
			}
			parsed, err := openai.ParseResponsesOptions(mergedOptions)
			if err == nil {
				options[openai.Name] = parsed
			}
		} else {
			parsed, err := openai.ParseOptions(mergedOptions)
			if err == nil {
				options[openai.Name] = parsed
			}
		}

	case anthropic.Name, bedrock.Name:
		var (
			_, hasEffort = mergedOptions["effort"]
			_, hasThink  = mergedOptions["thinking"]
			extraBody    = make(map[string]any)
		)

		switch providerCfg.ID {
		case string(catwalk.InferenceProviderAlibabaSingapore), string(catwalk.InferenceProviderAlibabaUS):
			switch {
			case !hasEffort && shouldSetEffort:
				extraBody["reasoning_effort"] = reasoningEffort
			case !hasThink && model.CatwalkCfg.CanReason:
				if model.ModelCfg.Think {
					extraBody["thinking"] = map[string]any{"type": "enabled"}
				} else {
					extraBody["thinking"] = map[string]any{"type": "disabled"}
				}
			}
			mergedOptions["extra_body"] = extraBody

		default:
			switch {
			case !hasEffort && shouldSetEffort:
				mergedOptions["effort"] = reasoningEffort
			case !hasThink && model.ModelCfg.Think:
				mergedOptions["thinking"] = map[string]any{"budget_tokens": 2000}
			}
		}

		parsed, err := anthropic.ParseOptions(mergedOptions)
		if err == nil {
			options[anthropic.Name] = parsed
		}

	case openrouter.Name:
		_, hasReasoning := mergedOptions["reasoning"]
		if !hasReasoning && shouldSetEffort {
			mergedOptions["reasoning"] = map[string]any{
				"enabled": true,
				"effort":  reasoningEffort,
			}
		}
		parsed, err := openrouter.ParseOptions(mergedOptions)
		if err == nil {
			options[openrouter.Name] = parsed
		}

	case vercel.Name:
		_, hasReasoning := mergedOptions["reasoning"]
		if !hasReasoning && shouldSetEffort {
			mergedOptions["reasoning"] = map[string]any{
				"enabled": true,
				"effort":  reasoningEffort,
			}
		}
		parsed, err := vercel.ParseOptions(mergedOptions)
		if err == nil {
			options[vercel.Name] = parsed
		}

	case google.Name:
		_, hasReasoning := mergedOptions["thinking_config"]
		if !hasReasoning {
			if strings.HasPrefix(model.CatwalkCfg.ID, "gemini-2") {
				mergedOptions["thinking_config"] = map[string]any{
					"thinking_budget":  2000,
					"include_thoughts": true,
				}
			} else {
				mergedOptions["thinking_config"] = map[string]any{
					"thinking_level":   reasoningEffort,
					"include_thoughts": true,
				}
			}
		}
		parsed, err := google.ParseOptions(mergedOptions)
		if err == nil {
			options[google.Name] = parsed
		}

	case openaicompat.Name, hyper.Name:
		extraBody := make(map[string]any)

		_, hasReasoningEffort := mergedOptions["reasoning_effort"]
		if !hasReasoningEffort && shouldSetEffort {
			switch providerCfg.ID {
			case string(catwalk.InferenceProviderIoNet):
				extraBody["reasoning"] = map[string]string{"effort": reasoningEffort}
			default:
				mergedOptions["reasoning_effort"] = reasoningEffort
			}
		}

		// "reasoning effort" is a standard OpenAI field, but "thinking" is not.
		// Setting it in the right way for each provider.
		// TODO: Abstract this in Fantasy somehow?
		// TODO: Allow custom providers to specify how to set this?
		switch providerCfg.ID {
		case hyper.Name:
			extraBody["thinking"] = model.ModelCfg.Think
		case string(catwalk.InferenceProviderIoNet):
			if _, ok := extraBody["reasoning"]; !ok && model.CatwalkCfg.CanReason {
				if model.ModelCfg.Think {
					extraBody["reasoning"] = map[string]string{"effort": "medium"}
				} else {
					extraBody["reasoning"] = map[string]string{"effort": "none"}
				}
			}

		case string(catwalk.InferenceProviderZAI), string(catwalk.InferenceProviderDeepSeek):
			if model.ModelCfg.Think || reasoningEffort != "" {
				extraBody["thinking"] = map[string]any{"type": "enabled"}
			} else {
				extraBody["thinking"] = map[string]any{"type": "disabled"}
			}

		case string(catwalk.InferenceProviderFireworks):
			// NOTE: Fireworks break if we set both `reasoning_effort` and `thinking`.
			if reasoningEffort == "" {
				if model.ModelCfg.Think {
					extraBody["thinking"] = map[string]any{"type": "enabled"}
				} else {
					extraBody["thinking"] = map[string]any{"type": "disabled"}
				}
			}

		case string(catwalk.InferenceProviderBaseten):
			extraBody["chat_template_args"] = map[string]any{
				"enable_thinking": model.ModelCfg.Think || reasoningEffort != "",
			}

		case string(catwalk.InferenceProviderAlibabaSingapore), string(catwalk.InferenceProviderAlibabaUS):
			if model.CatwalkCfg.CanReason {
				extraBody["enable_thinking"] = model.ModelCfg.Think || reasoningEffort != ""
			}
		}

		mergedOptions["extra_body"] = extraBody

		parsed, err := openaicompat.ParseOptions(mergedOptions)
		if err == nil {
			options[openaicompat.Name] = parsed
		}

	default:
		// Known custom providers (litellm, ollama, omlx) are
		// openai-compat under the hood.
		if discover.IsKnownCustomProvider(string(providerCfg.Type)) {
			applyLMStudioThinkingOption(model, providerCfg, mergedOptions)
			parsed, err := openaicompat.ParseOptions(mergedOptions)
			if err == nil {
				options[openaicompat.Name] = parsed
			}
		}
	}

	return options
}

func mergeCallOptions(model Model, cfg config.ProviderConfig) (fantasy.ProviderOptions, *float64, *float64, *int64, *float64, *float64) {
	modelOptions := getProviderOptions(model, cfg)
	temp := cmp.Or(model.ModelCfg.Temperature, model.CatwalkCfg.Options.Temperature)
	topP := cmp.Or(model.ModelCfg.TopP, model.CatwalkCfg.Options.TopP)
	topK := cmp.Or(model.ModelCfg.TopK, model.CatwalkCfg.Options.TopK)
	freqPenalty := cmp.Or(model.ModelCfg.FrequencyPenalty, model.CatwalkCfg.Options.FrequencyPenalty)
	presPenalty := cmp.Or(model.ModelCfg.PresencePenalty, model.CatwalkCfg.Options.PresencePenalty)
	return modelOptions, temp, topP, topK, freqPenalty, presPenalty
}

func (c *coordinator) buildAgent(ctx context.Context, prompt *prompt.Prompt, agent config.Agent, isSubAgent bool) (SessionAgent, error) {
	models, err := c.buildAgentModels(ctx, agent.Model, isSubAgent)
	if err != nil {
		return nil, err
	}

	primaryProviderCfg, ok := c.cfg.Config().Providers.Get(models.Primary.ModelCfg.Provider)
	if !ok {
		return nil, errModelProviderNotConfigured
	}
	smallProviderCfg, ok := c.cfg.Config().Providers.Get(models.Small.ModelCfg.Provider)
	if !ok {
		return nil, errModelProviderNotConfigured
	}
	summaryProviderCfg, ok := c.cfg.Config().Providers.Get(models.Summary.ModelCfg.Provider)
	if !ok {
		return nil, errModelProviderNotConfigured
	}
	memoryAllowed := !isSubAgent && agent.ID != config.AgentReview
	disableMemoryOnExternalContext := false
	if memoryOptions := c.cfg.Config().Options.Memory; memoryOptions != nil {
		disableMemoryOnExternalContext = memoryOptions.DisableOnExternalContext
	}
	result := NewSessionAgent(SessionAgentOptions{
		Models: SessionAgentModels{
			Large:                     models.Primary,
			Small:                     models.Small,
			Summary:                   models.Summary,
			SmallProviderOpts:         getProviderOptions(models.Small, smallProviderCfg),
			SummaryProviderOpts:       getProviderOptions(models.Summary, summaryProviderCfg),
			LargeSystemPromptPrefix:   primaryProviderCfg.SystemPromptPrefix,
			SmallSystemPromptPrefix:   smallProviderCfg.SystemPromptPrefix,
			SummarySystemPromptPrefix: summaryProviderCfg.SystemPromptPrefix,
		},
		SystemPrompt:                   "",
		IsSubAgent:                     isSubAgent,
		DisableAutoSummarize:           c.cfg.Config().Options.DisableAutoSummarize,
		IsYolo:                         c.permissions.SkipRequests(),
		Sessions:                       c.sessions,
		Messages:                       c.messages,
		Tools:                          nil,
		Notify:                         c.notify,
		RunComplete:                    c.runComplete,
		UserPromptHooks:                c.buildHookRunner(hooks.EventUserPromptSubmit, isSubAgent),
		StopHooks:                      c.buildHookRunner(hooks.EventStop, isSubAgent),
		Memory:                         c.memory,
		MemoryProject:                  c.project,
		MemoryRecorder:                 c.memoryRecorder && memoryAllowed,
		MemoryRecall:                   c.memoryRecall && memoryAllowed,
		MemoryDisableOnExternalContext: disableMemoryOnExternalContext,
	})

	c.readyWg.Go(func() error {
		systemPrompt, err := prompt.Build(ctx, models.Primary.Model.Provider(), models.Primary.Model.Model(), c.cfg)
		if err != nil {
			return err
		}
		result.SetSystemPrompt(systemPrompt)
		result.SetRecoveryContext(prompt.BuildRecoveryContext(c.cfg, defaultContextRetryProjectCharacters))
		return nil
	})

	c.readyWg.Go(func() error {
		tools, err := c.buildTools(ctx, agent, isSubAgent)
		if err != nil {
			return err
		}
		result.SetTools(tools)
		return nil
	})

	return result, nil
}

func (c *coordinator) buildTools(ctx context.Context, agent config.Agent, isSubAgent bool) ([]fantasy.AgentTool, error) {
	var allTools []fantasy.AgentTool
	if slices.Contains(agent.AllowedTools, AgentToolName) {
		agentTool, err := c.agentTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, agentTool)
	}

	if slices.Contains(agent.AllowedTools, tools.AgenticFetchToolName) {
		agenticFetchTool, err := c.agenticFetchTool(ctx, nil)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, agenticFetchTool)
	}

	// Get the model name for the agent
	modelID := ""
	if modelCfg, ok := c.cfg.Config().Models[agent.Model]; ok {
		if model := c.cfg.Config().GetModel(modelCfg.Provider, modelCfg.Model); model != nil {
			modelID = model.ID
		}
	}

	logFile := filepath.Join(c.cfg.Config().Options.DataDirectory, "logs", "crush.log")

	hookRunner := c.buildHookRunner(hooks.EventPreToolUse, isSubAgent)
	postToolHookRunner := c.buildHookRunner(hooks.EventPostToolUse, isSubAgent)
	postToolFailureHookRunner := c.buildHookRunner(hooks.EventPostToolUseFailure, isSubAgent)

	allTools = append(
		allTools,
		tools.NewBashTool(c.permissions, c.cfg.WorkingDir(), c.cfg.Config().Options.Attribution, modelID),
		tools.NewCrushInfoTool(c.cfg, c.lspManager, c.allSkills, c.activeSkills, c.skillTracker),
		tools.NewSkillTool(c.activeSkills, c.skillTracker),
		tools.NewAddSourceTool(c.sessions, c.cfg.WorkingDir()),
		tools.NewRemoveSourceTool(c.sessions),
		tools.NewSourcesTool(c.sessions),
		tools.NewMCPAddTool(c.cfg, c.permissions),
		tools.NewMCPManageTool(c.cfg, c.permissions),
		tools.NewGoalStatusTool(),
		tools.NewCrushLogsTool(logFile),
		tools.NewJobOutputTool(),
		tools.NewJobListTool(),
		tools.NewJobKillTool(),
		tools.NewTmuxTool(c.permissions, c.cfg.WorkingDir()),
		tools.NewDownloadTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewEditTool(c.lspManager, c.permissions, c.history, c.filetracker, c.cfg.WorkingDir()),
		tools.NewMultiEditTool(c.lspManager, c.permissions, c.history, c.filetracker, c.cfg.WorkingDir()),
		tools.NewFetchTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewWebFetchTool(c.permissions, c.cfg.WorkingDir(), c.cfg.Config().Options.DataDirectory, nil),
		tools.NewWebSearchTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewGlobTool(c.cfg.WorkingDir(), c.cfg.Config().Tools.Glob),
		tools.NewGrepTool(c.cfg.WorkingDir(), c.cfg.Config().Tools.Grep),
		tools.NewLsTool(c.permissions, c.cfg.WorkingDir(), c.cfg.Config().Tools.Ls),
		tools.NewSourcegraphTool(nil),
		tools.NewTodosTool(c.sessions),
		tools.NewViewTool(c.lspManager, c.permissions, c.filetracker, c.skillTracker, c.cfg.WorkingDir(), c.cfg.Config().Options.SkillsPaths...),
		tools.NewWriteTool(c.lspManager, c.permissions, c.history, c.filetracker, c.cfg.WorkingDir()),
	)

	// Add LSP tools if user has configured LSPs or auto_lsp is enabled (nil or true).
	if len(c.cfg.Config().LSP) > 0 || c.cfg.Config().Options.AutoLSP == nil || *c.cfg.Config().Options.AutoLSP {
		allTools = append(allTools, tools.NewDiagnosticsTool(c.lspManager), tools.NewReferencesTool(c.lspManager), tools.NewLSPRestartTool(c.lspManager))
	}

	if len(c.cfg.Config().MCP) > 0 {
		allTools = append(
			allTools,
			tools.NewListMCPResourcesTool(c.cfg, c.permissions),
			tools.NewReadMCPResourceTool(c.cfg, c.permissions),
		)
	}
	if agent.AllowedMCP == nil || len(agent.AllowedMCP) > 0 {
		// Discovery reads live runtime state on every call. Keep it available
		// even before the first server is added so a newly connected server can
		// be selected without rebuilding the session agent.
		allTools = append(
			allTools,
			tools.NewMCPToolSearchTool(c.permissions, c.cfg, c.cfg.WorkingDir(), agent.AllowedMCP),
			tools.NewMCPToolCallTool(c.permissions, c.cfg, c.cfg.WorkingDir(), agent.AllowedMCP),
		)
	}

	var filteredTools []fantasy.AgentTool
	for _, tool := range allTools {
		if slices.Contains(agent.AllowedTools, tool.Info().Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	slices.SortFunc(filteredTools, func(a, b fantasy.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})

	// Wrap tools with hook interception for the top-level agent only.
	// Sub-agents (the `agent` task tool, `agentic_fetch`, etc.) run
	// without hook interception to avoid firing the user's hook N times
	// per delegated turn. The top-level invocation of the sub-agent tool
	// itself is still wrapped from the coder's side.
	filteredTools = wrapToolsWithHooks(filteredTools, hookRunner, postToolHookRunner, postToolFailureHookRunner, isSubAgent)

	return filteredTools, nil
}

func (c *coordinator) buildHookRunner(event string, isSubAgent bool) *hooks.Runner {
	if isSubAgent {
		return nil
	}
	eventHooks := c.cfg.Config().Hooks[event]
	if len(eventHooks) == 0 {
		return nil
	}
	return hooks.NewRunner(eventHooks, c.cfg.WorkingDir(), c.cfg.WorkingDir())
}

type agentModels struct {
	Primary Model
	Small   Model
	Summary Model
}

func (c *coordinator) buildAgentModels(ctx context.Context, primaryType config.SelectedModelType, isSubAgent bool) (agentModels, error) {
	if primaryType == "" {
		primaryType = config.SelectedModelTypeLarge
	}
	primary, err := c.buildModel(ctx, primaryType, isSubAgent)
	if err != nil {
		return agentModels{}, err
	}
	small, err := c.buildModel(ctx, config.SelectedModelTypeSmall, true)
	if err != nil {
		return agentModels{}, err
	}
	summary, err := c.buildModel(ctx, config.SelectedModelTypeSummary, isSubAgent)
	if err != nil {
		return agentModels{}, err
	}
	return agentModels{
		Primary: primary,
		Small:   small,
		Summary: summary,
	}, nil
}

func (c *coordinator) buildModel(ctx context.Context, modelType config.SelectedModelType, isSubAgent bool) (Model, error) {
	modelCfg, ok := c.cfg.Config().Models[modelType]
	if !ok {
		return Model{}, fmt.Errorf("%s model not selected", modelType)
	}
	providerCfg, ok := c.cfg.Config().Providers.Get(modelCfg.Provider)
	if !ok {
		return Model{}, fmt.Errorf("%s model provider not configured", modelType)
	}
	provider, err := c.buildProvider(providerCfg, modelCfg, isSubAgent)
	if err != nil {
		return Model{}, err
	}

	var catwalkModel *catwalk.Model
	for _, m := range providerCfg.Models {
		if m.ID == modelCfg.Model {
			catwalkModel = &m
			break
		}
	}
	if catwalkModel == nil {
		return Model{}, fmt.Errorf("%s model not found in provider config", modelType)
	}

	modelID := modelCfg.Model
	if modelCfg.Provider == openrouter.Name && isExactoSupported(modelID) {
		modelID += ":exacto"
	}
	languageModel, err := provider.LanguageModel(ctx, modelID)
	if err != nil {
		return Model{}, err
	}
	return Model{
		Model:      languageModel,
		CatwalkCfg: *catwalkModel,
		ModelCfg:   modelCfg,
		FlatRate:   providerCfg.FlatRate,
	}, nil
}

func (c *coordinator) buildAnthropicProvider(baseURL, apiKey string, headers map[string]string, providerID string) (fantasy.Provider, error) {
	var opts []anthropic.Option

	switch {
	case strings.HasPrefix(apiKey, "Bearer "):
		// NOTE: Prevent the SDK from picking up the API key from env.
		os.Setenv("ANTHROPIC_API_KEY", "")
		headers["Authorization"] = apiKey
	case providerID == string(catwalk.InferenceProviderMiniMax) || providerID == string(catwalk.InferenceProviderMiniMaxChina):
		// NOTE: Prevent the SDK from picking up the API key from env.
		os.Setenv("ANTHROPIC_API_KEY", "")
		headers["Authorization"] = "Bearer " + apiKey
	case apiKey != "":
		// X-Api-Key header
		opts = append(opts, anthropic.WithAPIKey(apiKey))
	}

	if len(headers) > 0 {
		opts = append(opts, anthropic.WithHeaders(headers))
	}

	if baseURL != "" {
		opts = append(opts, anthropic.WithBaseURL(baseURL))
	}

	if c.cfg.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, anthropic.WithHTTPClient(httpClient))
	}
	return anthropic.New(opts...)
}

func (c *coordinator) buildOpenaiProvider(baseURL, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []openai.Option{
		openai.WithAPIKey(apiKey),
		openai.WithUseResponsesAPI(),
	}
	if c.cfg.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openai.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openai.WithHeaders(headers))
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}
	return openai.New(opts...)
}

func (c *coordinator) buildOpenrouterProvider(_, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []openrouter.Option{
		openrouter.WithAPIKey(apiKey),
	}
	if c.cfg.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openrouter.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openrouter.WithHeaders(headers))
	}
	return openrouter.New(opts...)
}

func (c *coordinator) buildVercelProvider(_, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []vercel.Option{
		vercel.WithAPIKey(apiKey),
	}
	if c.cfg.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, vercel.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, vercel.WithHeaders(headers))
	}
	return vercel.New(opts...)
}

func (c *coordinator) buildOpenaiCompatProvider(baseURL, apiKey string, headers map[string]string, extraBody map[string]any, providerID string, isSubAgent bool) (fantasy.Provider, error) {
	opts := []openaicompat.Option{
		openaicompat.WithBaseURL(baseURL),
		openaicompat.WithAPIKey(apiKey),
	}

	// Set HTTP client based on provider and debug mode.
	var httpClient *http.Client
	switch providerID {
	case string(catwalk.InferenceProviderCopilot):
		opts = append(
			opts,
			openaicompat.WithUseResponsesAPI(),
			openaicompat.WithResponsesAPIFunc(func(modelID string) bool {
				return copilotResponsesModels[modelID]
			}),
		)
		httpClient = copilot.NewClient(isSubAgent, c.cfg.Config().Options.Debug)
	}
	if httpClient == nil && c.cfg.Config().Options.Debug {
		httpClient = log.NewHTTPClient()
	}
	if httpClient != nil {
		opts = append(opts, openaicompat.WithHTTPClient(httpClient))
	}

	if len(headers) > 0 {
		opts = append(opts, openaicompat.WithHeaders(headers))
	}

	for extraKey, extraValue := range extraBody {
		opts = append(opts, openaicompat.WithSDKOptions(openaisdk.WithJSONSet(extraKey, extraValue)))
	}

	return openaicompat.New(opts...)
}

func (c *coordinator) buildAzureProvider(baseURL, apiKey string, headers map[string]string, options map[string]string) (fantasy.Provider, error) {
	opts := []azure.Option{
		azure.WithBaseURL(baseURL),
		azure.WithAPIKey(apiKey),
		azure.WithUseResponsesAPI(),
	}
	if c.cfg.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, azure.WithHTTPClient(httpClient))
	}
	if options == nil {
		options = make(map[string]string)
	}
	if apiVersion, ok := options["apiVersion"]; ok {
		opts = append(opts, azure.WithAPIVersion(apiVersion))
	}
	if len(headers) > 0 {
		opts = append(opts, azure.WithHeaders(headers))
	}

	return azure.New(opts...)
}

func (c *coordinator) buildBedrockProvider(apiKey string, headers map[string]string, providerID string) (fantasy.Provider, error) {
	var opts []bedrock.Option
	if c.cfg.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, bedrock.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, bedrock.WithHeaders(headers))
	}

	switch {
	case apiKey != "":
		opts = append(opts, bedrock.WithAPIKey(apiKey))
	case os.Getenv("AWS_BEARER_TOKEN_BEDROCK") != "":
		opts = append(opts, bedrock.WithAPIKey(os.Getenv("AWS_BEARER_TOKEN_BEDROCK")))
	default:
		// Skip, let the SDK do authentication.
	}

	switch providerID {
	case string(catwalk.InferenceProviderBedrockEurope):
		opts = append(opts, bedrock.WithRegion("eu-west-1"))
	default:
		opts = append(opts, bedrock.WithRegion("us-east-1"))
	}

	return bedrock.New(opts...)
}

func (c *coordinator) buildGoogleProvider(baseURL, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []google.Option{
		google.WithBaseURL(baseURL),
		google.WithGeminiAPIKey(apiKey),
	}
	if c.cfg.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}
	return google.New(opts...)
}

func (c *coordinator) buildGoogleVertexProvider(headers map[string]string, options map[string]string) (fantasy.Provider, error) {
	opts := []google.Option{}
	if c.cfg.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}

	project := options["project"]
	location := options["location"]

	opts = append(opts, google.WithVertex(project, location))

	return google.New(opts...)
}

func (c *coordinator) isAnthropicThinking(model config.SelectedModel) bool {
	if model.Think {
		return true
	}
	opts, err := anthropic.ParseOptions(model.ProviderOptions)
	return err == nil && opts.Thinking != nil
}

func (c *coordinator) buildProvider(providerCfg config.ProviderConfig, model config.SelectedModel, isSubAgent bool) (fantasy.Provider, error) {
	headers := maps.Clone(providerCfg.ExtraHeaders)
	if headers == nil {
		headers = make(map[string]string)
	}

	// handle special headers for anthropic
	if providerCfg.Type == anthropic.Name && c.isAnthropicThinking(model) {
		if v, ok := headers["anthropic-beta"]; ok {
			headers["anthropic-beta"] = v + ",interleaved-thinking-2025-05-14"
		} else {
			headers["anthropic-beta"] = "interleaved-thinking-2025-05-14"
		}
	}

	apiKey, _ := c.cfg.Resolve(providerCfg.APIKey)
	baseURL, _ := c.cfg.Resolve(providerCfg.BaseURL)

	switch providerCfg.ID {
	case string(catwalk.InferenceProviderOpenCodeGo), string(catwalk.InferenceProviderOpenCodeZen):
		if opencodeMessagesModels[model.Model] {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
			return c.buildAnthropicProvider(baseURL, apiKey, headers, providerCfg.ID)
		}
	}

	switch providerCfg.Type {
	case openai.Name:
		return c.buildOpenaiProvider(baseURL, apiKey, headers)
	case anthropic.Name:
		return c.buildAnthropicProvider(baseURL, apiKey, headers, providerCfg.ID)
	case openrouter.Name:
		return c.buildOpenrouterProvider(baseURL, apiKey, headers)
	case vercel.Name:
		return c.buildVercelProvider(baseURL, apiKey, headers)
	case azure.Name:
		return c.buildAzureProvider(baseURL, apiKey, headers, providerCfg.ExtraParams)
	case bedrock.Name:
		return c.buildBedrockProvider(apiKey, headers, providerCfg.ID)
	case google.Name:
		return c.buildGoogleProvider(baseURL, apiKey, headers)
	case "google-vertex":
		return c.buildGoogleVertexProvider(headers, providerCfg.ExtraParams)
	case openaicompat.Name, hyper.Name:
		switch providerCfg.ID {
		case hyper.Name:
			baseURL = hyper.BaseURL() + "/v1"
			headers["x-crush-id"] = event.GetID()
		case string(catwalk.InferenceProviderZAI):
			if providerCfg.ExtraBody == nil {
				providerCfg.ExtraBody = map[string]any{}
			}
			providerCfg.ExtraBody["tool_stream"] = true
		}
		return c.buildOpenaiCompatProvider(baseURL, apiKey, headers, providerCfg.ExtraBody, providerCfg.ID, isSubAgent)
	default:
		// Known custom providers (litellm, ollama, omlx) are
		// openai-compat under the hood.
		if discover.IsKnownCustomProvider(string(providerCfg.Type)) {
			return c.buildOpenaiCompatProvider(baseURL, apiKey, headers, providerCfg.ExtraBody, providerCfg.ID, isSubAgent)
		}
		return nil, fmt.Errorf("provider type not supported: %q", providerCfg.Type)
	}
}

func isExactoSupported(modelID string) bool {
	supportedModels := []string{
		"moonshotai/kimi-k2-0905",
		"deepseek/deepseek-v3.1-terminus",
		"z-ai/glm-4.6",
		"openai/gpt-oss-120b",
		"qwen/qwen3-coder",
	}
	return slices.Contains(supportedModels, modelID)
}

// BeginAccepted reserves an accept slot for sessionID on the active
// agent and returns the ownership handle. It is the fire-and-forget
// dispatch path's only way to mark a run as accepted-but-not-yet-active
// so a cancel arriving before the run registers in activeRequests is not
// lost.
func (c *coordinator) BeginAccepted(sessionID string) *AcceptedRun {
	return c.currentAgent.BeginAccepted(sessionID)
}

func (c *coordinator) Cancel(sessionID string) {
	c.currentAgent.Cancel(sessionID)
}

func (c *coordinator) CancelAll() {
	c.currentAgent.CancelAll()
}

func (c *coordinator) ClearQueue(sessionID string) {
	c.currentAgent.ClearQueue(sessionID)
}

func (c *coordinator) IsBusy() bool {
	return c.currentAgent.IsBusy()
}

func (c *coordinator) IsSessionBusy(sessionID string) bool {
	return c.currentAgent.IsSessionBusy(sessionID)
}

func (c *coordinator) Model() Model {
	return c.currentAgent.Model()
}

func (c *coordinator) UpdateModels(ctx context.Context) error {
	// build the models again so we make sure we get the latest config
	agentID := c.CurrentAgentID()
	agentCfg, ok := c.cfg.Config().Agents[agentID]
	if !ok {
		return fmt.Errorf("%w: %s", errAgentNotConfigured, agentID)
	}
	models, err := c.buildAgentModels(ctx, agentCfg.Model, false)
	if err != nil {
		return err
	}
	largeProviderCfg, ok := c.cfg.Config().Providers.Get(models.Primary.ModelCfg.Provider)
	if !ok {
		return errModelProviderNotConfigured
	}
	smallProviderCfg, ok := c.cfg.Config().Providers.Get(models.Small.ModelCfg.Provider)
	if !ok {
		return errModelProviderNotConfigured
	}
	summaryProviderCfg, ok := c.cfg.Config().Providers.Get(models.Summary.ModelCfg.Provider)
	if !ok {
		return errModelProviderNotConfigured
	}
	c.currentAgent.SetModels(SessionAgentModels{
		Large:                     models.Primary,
		Small:                     models.Small,
		Summary:                   models.Summary,
		SmallProviderOpts:         getProviderOptions(models.Small, smallProviderCfg),
		SummaryProviderOpts:       getProviderOptions(models.Summary, summaryProviderCfg),
		LargeSystemPromptPrefix:   largeProviderCfg.SystemPromptPrefix,
		SmallSystemPromptPrefix:   smallProviderCfg.SystemPromptPrefix,
		SummarySystemPromptPrefix: summaryProviderCfg.SystemPromptPrefix,
	})

	tools, err := c.buildTools(ctx, agentCfg, false)
	if err != nil {
		return err
	}
	c.currentAgent.SetTools(tools)
	return nil
}

func (c *coordinator) SetMemoryOptions(recorderEnabled, recallEnabled bool) error {
	if c.IsBusy() {
		return ErrSessionBusy
	}
	c.memoryRecorder = recorderEnabled
	c.memoryRecall = recallEnabled
	for _, sessionAgent := range c.agents {
		sessionAgent.SetMemoryOptions(recorderEnabled, recallEnabled)
	}
	return nil
}

func (c *coordinator) QueuedPrompts(sessionID string) int {
	return c.currentAgent.QueuedPrompts(sessionID)
}

func (c *coordinator) QueuedPromptsList(sessionID string) []string {
	return c.currentAgent.QueuedPromptsList(sessionID)
}

func (c *coordinator) Summarize(ctx context.Context, sessionID string) error {
	summaryModel := c.currentAgent.SummaryModel()
	providerCfg, ok := c.cfg.Config().Providers.Get(summaryModel.ModelCfg.Provider)
	if !ok {
		return errModelProviderNotConfigured
	}

	if err := c.refreshTokenIfExpired(ctx, providerCfg); err != nil {
		slog.Error("Failed to refresh OAuth2 token before summarize. Proceeding with existing token.", "error", err)
	}

	summarize := func() error {
		return c.currentAgent.Summarize(ctx, sessionID)
	}

	return c.runWithUnauthorizedRetry(ctx, providerCfg, summarize)
}

// GenerateTitle generates a session title using the current agent.
func (c *coordinator) GenerateTitle(ctx context.Context, sessionID, prompt string) {
	if c.currentAgent == nil {
		return
	}
	c.currentAgent.GenerateTitle(ctx, sessionID, prompt)
}

// refreshTokenIfExpired proactively refreshes the OAuth token if it has expired.
func (c *coordinator) refreshTokenIfExpired(ctx context.Context, providerCfg config.ProviderConfig) error {
	if providerCfg.OAuthToken == nil || !providerCfg.OAuthToken.IsExpired() {
		return nil
	}
	slog.Debug("Token needs to be refreshed", "provider", providerCfg.ID)
	return c.refreshOAuth2Token(ctx, providerCfg)
}

// runWithUnauthorizedRetry executes fn. If fn returns a 401 error, it
// attempts to refresh credentials and re-runs fn once. Returns the
// final error: from the retry if a retry was attempted, otherwise from
// the original run. Callers that need to notify the user on persistent
// failure should check isUnauthorized on the returned error.
func (c *coordinator) runWithUnauthorizedRetry(ctx context.Context, providerCfg config.ProviderConfig, fn func() error) error {
	err := fn()
	if err != nil && c.isUnauthorized(err) {
		if retryErr := c.retryAfterUnauthorized(ctx, providerCfg); retryErr == nil {
			return fn()
		}
	}
	return err
}

// retryAfterUnauthorized attempts to refresh credentials after receiving a 401
// and returns nil if retry should be attempted.
func (c *coordinator) retryAfterUnauthorized(ctx context.Context, providerCfg config.ProviderConfig) error {
	switch {
	case providerCfg.OAuthToken != nil:
		slog.Debug("Received 401. Refreshing token and retrying", "provider", providerCfg.ID)
		return c.refreshOAuth2Token(ctx, providerCfg)
	case strings.Contains(providerCfg.APIKeyTemplate, "$"):
		slog.Debug("Received 401. Refreshing API Key template and retrying", "provider", providerCfg.ID)
		return c.refreshApiKeyTemplate(ctx, providerCfg)
	default:
		return nil
	}
}

func (c *coordinator) isUnauthorized(err error) bool {
	var providerErr *fantasy.ProviderError
	return errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusUnauthorized
}

func (c *coordinator) refreshOAuth2Token(ctx context.Context, providerCfg config.ProviderConfig) error {
	if err := c.cfg.RefreshOAuthToken(ctx, config.ScopeGlobal, providerCfg.ID); err != nil {
		slog.Error("Failed to refresh OAuth token after 401 error", "provider", providerCfg.ID, "error", err)
		return err
	}
	if err := c.UpdateModels(ctx); err != nil {
		return err
	}
	return nil
}

func (c *coordinator) refreshApiKeyTemplate(ctx context.Context, providerCfg config.ProviderConfig) error {
	newAPIKey, err := c.cfg.Resolve(providerCfg.APIKeyTemplate)
	if err != nil {
		slog.Error("Failed to re-resolve API key after 401 error", "provider", providerCfg.ID, "error", err)
		return err
	}

	providerCfg.APIKey = newAPIKey
	c.cfg.Config().Providers.Set(providerCfg.ID, providerCfg)

	if err := c.UpdateModels(ctx); err != nil {
		return err
	}
	return nil
}

// subAgentParams holds the parameters for running a sub-agent.
type subAgentParams struct {
	Agent          SessionAgent
	SessionID      string
	AgentMessageID string
	ToolCallID     string
	Prompt         string
	SessionTitle   string
	// SessionSetup is an optional callback invoked after session creation
	// but before agent execution, for custom session configuration.
	SessionSetup func(sessionID string)
}

// runSubAgent runs a sub-agent and handles session management and cost accumulation.
// It creates a sub-session, runs the agent with the given prompt, and propagates
// the cost to the parent session.
func (c *coordinator) runSubAgent(ctx context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
	// Create sub-session
	agentToolSessionID := c.sessions.CreateAgentToolSessionID(params.AgentMessageID, params.ToolCallID)
	session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, params.SessionID, params.SessionTitle)
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("create session: %w", err)
	}

	// Call session setup function if provided
	if params.SessionSetup != nil {
		params.SessionSetup(session.ID)
	}

	// Get model configuration
	model := params.Agent.Model()
	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	providerCfg, ok := c.cfg.Config().Providers.Get(model.ModelCfg.Provider)
	if !ok {
		return fantasy.ToolResponse{}, errModelProviderNotConfigured
	}

	// Run the agent
	run := func() (*fantasy.AgentResult, error) {
		return params.Agent.Run(ctx, SessionAgentCall{
			SessionID:        session.ID,
			Prompt:           params.Prompt,
			MaxOutputTokens:  maxTokens,
			ProviderOptions:  getProviderOptions(model, providerCfg),
			Temperature:      model.ModelCfg.Temperature,
			TopP:             model.ModelCfg.TopP,
			TopK:             model.ModelCfg.TopK,
			FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
			PresencePenalty:  model.ModelCfg.PresencePenalty,
			NonInteractive:   true,
		})
	}
	var result *fantasy.AgentResult
	err = c.runWithUnauthorizedRetry(ctx, providerCfg, func() error {
		var runErr error
		result, runErr = run()
		return runErr
	})
	// Notify only if still unauthorized after retry.
	if err != nil && c.isUnauthorized(err) && c.notify != nil && model.ModelCfg.Provider == hyper.Name {
		c.notify.Publish(pubsub.CreatedEvent, notify.Notification{
			Type:       notify.TypeReAuthenticate,
			ProviderID: model.ModelCfg.Provider,
		})
	}
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to generate response: %s", err)), nil
	}

	// Update parent session cost on a best-effort basis. A failure here must
	// not discard the sub-agent output that was already produced.
	if err := c.updateParentSessionCost(ctx, session.ID, params.SessionID); err != nil {
		slog.Warn(
			"Failed to update parent session cost",
			"child_session", session.ID,
			"parent_session", params.SessionID,
			"error", err,
		)
	}

	output := subAgentOutput(result)
	if output == "" {
		return fantasy.NewTextErrorResponse("Sub-agent completed but produced no text output."), nil
	}
	return fantasy.NewTextResponse(output), nil
}

func subAgentOutput(result *fantasy.AgentResult) string {
	if result == nil {
		return ""
	}
	if text := strings.TrimSpace(result.Response.Content.Text()); text != "" {
		return text
	}

	var evidence []string
	for _, step := range result.Steps {
		calls := make(map[string]fantasy.ToolCallContent)
		for _, call := range step.Content.ToolCalls() {
			calls[call.ToolCallID] = call
		}
		for _, toolResult := range step.Content.ToolResults() {
			call := calls[toolResult.ToolCallID]
			name := toolResult.ToolName
			if name == "" {
				name = call.ToolName
			}
			input := truncateSubAgentEvidence(call.Input, 300)
			output := truncateSubAgentEvidence(toolResultOutputString(toolResult.Result), 600)
			if output == "" {
				output = "(no output)"
			}
			evidence = append(evidence, fmt.Sprintf("- %s input=%s result=%s", name, input, output))
		}
	}
	if len(evidence) == 0 {
		return ""
	}
	if len(evidence) > 8 {
		evidence = evidence[len(evidence)-8:]
	}
	return "Sub-agent ended without a final prose response. Treat the following as evidence, not as a completed action:\n" + strings.Join(evidence, "\n")
}

func truncateSubAgentEvidence(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

// updateParentSessionCost accumulates the cost from a child session to its parent session.
func (c *coordinator) updateParentSessionCost(ctx context.Context, childSessionID, parentSessionID string) error {
	childSession, err := c.sessions.Get(ctx, childSessionID)
	if err != nil {
		return fmt.Errorf("get child session: %w", err)
	}

	parentSession, err := c.sessions.Get(ctx, parentSessionID)
	if err != nil {
		return fmt.Errorf("get parent session: %w", err)
	}

	parentSession.Cost += childSession.Cost

	if _, err := c.sessions.Save(ctx, parentSession); err != nil {
		return fmt.Errorf("save parent session: %w", err)
	}

	return nil
}

// discoverSkills is a thin fallback wrapper used only when no
// skills.Manager has been threaded through to the coordinator. All
// production call sites (backend.CreateWorkspace, setupLocalWorkspace)
// run discovery in advance and pass the results via the manager;
// reaching this path means a caller bypassed both. It deliberately does
// NOT publish to the package-level broker — there are no subscribers in
// that case, so doing so would be misleading without delivering the
// snapshot anywhere useful.
func discoverSkills(cfg *config.ConfigStore) (allSkills, activeSkills []*skills.Skill) {
	opts := cfg.Config().Options
	var paths, disabled []string
	if opts != nil {
		paths = opts.SkillsPaths
		disabled = opts.DisabledSkills
	}
	var resolver func(string) (string, error)
	if r := cfg.Resolver(); r != nil {
		resolver = r.ResolveValue
	}
	allSkills, activeSkills, states := skills.DiscoverFromConfig(skills.DiscoveryConfig{
		SkillsPaths:    paths,
		DisabledSkills: disabled,
		Resolver:       resolver,
	})
	logDiscoveryStats(states, paths, allSkills, activeSkills, disabled)
	return allSkills, activeSkills
}

// logTurnSkillUsage emits a per-turn diagnostic line showing which skills
// (if any) were loaded during this turn and which looked relevant based on
// a cheap keyword match against the user prompt. The goal is to surface
// "should-have-loaded but didn't" situations for later analysis.
//
// Logged at Info level under component=skills; heavy fields are elided when
// there is nothing interesting to report.
func logTurnSkillUsage(
	sessionID string,
	prompt string,
	activeSkills []*skills.Skill,
	tracker *skills.Tracker,
	before []string,
) {
	if tracker == nil || len(activeSkills) == 0 {
		return
	}

	after := tracker.LoadedNames()

	beforeSet := make(map[string]bool, len(before))
	for _, n := range before {
		beforeSet[n] = true
	}
	var loadedThisTurn []string
	for _, n := range after {
		if !beforeSet[n] {
			loadedThisTurn = append(loadedThisTurn, n)
		}
	}

	slog.Info(
		"Skill turn summary",
		"component", "skills",
		"session_id", sessionID,
		"prompt_len", len(prompt),
		"active_total", len(activeSkills),
		"loaded_total", len(after),
		"loaded_this_turn", loadedThisTurn,
	)
}

// logDiscoveryStats emits a single structured log line summarising skill
// discovery for the current session. It is intentionally low-volume: one
// line per session start. Builtin vs user counts are derived from the
// SkillState.Path — builtin states use the "builtin/" embed prefix.
func logDiscoveryStats(
	states []*skills.SkillState,
	userPaths []string,
	allSkills, activeSkills []*skills.Skill,
	disabled []string,
) {
	var builtinOK, builtinErr, userOK, userErr int
	for _, s := range states {
		isBuiltin := strings.HasPrefix(s.Path, "builtin/")
		switch {
		case isBuiltin && s.State == skills.StateNormal:
			builtinOK++
		case isBuiltin && s.State == skills.StateError:
			builtinErr++
		case !isBuiltin && s.State == skills.StateNormal:
			userOK++
		case !isBuiltin && s.State == skills.StateError:
			userErr++
		}
	}

	activeNames := make([]string, 0, len(activeSkills))
	for _, s := range activeSkills {
		activeNames = append(activeNames, s.Name)
	}

	xml := skills.ToPromptXML(activeSkills)

	slog.Info(
		"Skill discovery complete",
		"component", "skills",
		"builtin_ok", builtinOK,
		"builtin_errors", builtinErr,
		"user_ok", userOK,
		"user_errors", userErr,
		"user_paths", len(userPaths),
		"deduped_total", len(allSkills),
		"active", len(activeSkills),
		"disabled", len(disabled),
		"prompt_bytes", len(xml),
		"prompt_tok_est", skills.ApproxTokenCount(xml),
		"active_names", activeNames,
	)
}
