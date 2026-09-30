package agent

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/discover"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	openaioauth "github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/router"
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
	errPlanAgentNotConfigured          = errors.New("plan agent not configured")
	errMainAgentNotFound               = errors.New("main agent not found")
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
	"gpt-5.6-luna":  true,
	"gpt-5.6-terra": true,
	"gpt-5.6-sol":   true,
	"gpt-6-astra":   true,
	"grok-4.5":      true,
	"grok-4.6":      true,
}

// OpenCode models that use the Anthropic Messages API instead of Chat
// Completions. Which endpoint serves each model differs per provider, see
// https://opencode.ai/docs/zen and https://opencode.ai/docs/go.
func isOpenCodeMessagesModel(providerID, modelID string) bool {
	switch providerID {
	case string(catwalk.InferenceProviderOpenCodeGo):
		return strings.HasPrefix(modelID, "minimax-") ||
			strings.HasPrefix(modelID, "qwen3.6-") ||
			strings.HasPrefix(modelID, "qwen3.7-") ||
			strings.HasPrefix(modelID, "qwen3.8-")
	case string(catwalk.InferenceProviderOpenCodeZen):
		return strings.HasPrefix(modelID, "claude-") ||
			strings.HasPrefix(modelID, "qwen3.5-") ||
			strings.HasPrefix(modelID, "qwen3.6-") ||
			strings.HasPrefix(modelID, "qwen3.7-") ||
			strings.HasPrefix(modelID, "qwen3.8-")
	}
	return false
}

// OpenCode models that use the OpenAI Responses API instead of Chat
// Completions. See https://opencode.ai/docs/zen and https://opencode.ai/docs/go.
func isOpenCodeResponsesModel(modelID string) bool {
	return strings.HasPrefix(modelID, "gpt-") ||
		strings.HasPrefix(modelID, "grok-") ||
		strings.HasPrefix(modelID, "muse-spark-")
}

type Coordinator interface {
	SetMainAgent(agentName string) error
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
	LastRouterDecision() (router.Decision, bool)
	RouterQuerying() (string, bool)
	LastRouterModel() string
	LastRouterError() string
	RouterSavings(sessionID string) float64
	UpdateModels(ctx context.Context) error
	GenerateTitle(ctx context.Context, sessionID, prompt string)
}

type coordinator struct {
	cfg         *config.ConfigStore
	sessions    session.Service
	messages    message.Service
	permissions permission.Service
	questions   question.Service
	history     history.Service
	filetracker filetracker.Service
	lspManager  *lsp.Manager
	notify      pubsub.Publisher[notify.Notification]
	runComplete pubsub.Publisher[notify.RunComplete]
	interactive bool

	// agentMu guards mainAgent and mainAgentName: SetMainAgent runs on
	// HTTP handler goroutines while runs, cancels, and probes read the
	// current agent from their own goroutines.
	agentMu       sync.RWMutex
	mainAgent     SessionAgent
	mainAgentName string
	agents        map[string]SessionAgent

	// routerMu guards lastRouterDecision/hasRouterDecision, recorded by
	// run() after each router call and read by LastRouterDecision from a
	// future status-bar indicator. It also guards
	// routerQuerying/routerQueryingModel, set for the duration of the
	// in-flight router HTTP call so the sidebar can show a live
	// "consulting" indicator.
	routerMu            sync.RWMutex
	lastRouterDecision  router.Decision
	hasRouterDecision   bool
	routerQuerying      bool
	routerQueryingModel string
	// lastRouterModel is the router backend's own model id (the
	// classifier, e.g. "~typesafe/jev-latest") used for the most recent
	// router call, kept around after routerQueryingModel clears so the
	// sidebar can still say which model produced the last decision.
	lastRouterModel string
	// lastRouterError is the most recent router failure message (a
	// consulted-but-failed call, an unrecognized provider, or a missing
	// base URL for a local provider) while the router is enabled, so the
	// UI can show that the router isn't working instead of just quietly
	// showing nothing. Cleared to "" on the next successful call.
	lastRouterError string

	// savingsMu guards routerSavingsBySession: cumulative estimated
	// dollar savings from the router's per-message reasoning/model
	// override, vs. what the session's statically configured model would
	// have cost for the same token usage. Session-scoped (not
	// coordinator-wide like the fields above) and in-memory only — it
	// resets on process restart, which matches "this session's savings"
	// rather than a durable ledger.
	savingsMu              sync.Mutex
	routerSavingsBySession map[string]float64

	// Skills discovery results (session-start snapshot).
	allSkills    []*skills.Skill // Pre-filter: all discovered after dedup.
	activeSkills []*skills.Skill // Post-filter: active skills only.
	skillTracker *skills.Tracker

	readyWg errgroup.Group
}

// CoordinatorOptions holds the dependencies for NewCoordinator. Using a
// struct keeps the constructor self-documenting and avoids a long
// positional parameter list.
type CoordinatorOptions struct {
	Config      *config.ConfigStore
	Sessions    session.Service
	Messages    message.Service
	Permissions permission.Service
	Questions   question.Service
	History     history.Service
	FileTracker filetracker.Service
	LSPManager  *lsp.Manager
	Notify      pubsub.Publisher[notify.Notification]
	RunComplete pubsub.Publisher[notify.RunComplete]
	Skills      *skills.Manager
	Interactive bool
}

func NewCoordinator(ctx context.Context, opts CoordinatorOptions) (Coordinator, error) {
	// Skills are pre-discovered by the caller (see app.New /
	// backend.CreateWorkspace) and passed in via the manager. If no
	// manager was provided (legacy callers), fall back to an in-line
	// discovery so the coordinator still works.
	var allSkills, activeSkills []*skills.Skill
	if opts.Skills != nil {
		allSkills = opts.Skills.AllSkills()
		activeSkills = opts.Skills.ActiveSkills()
	} else {
		allSkills, activeSkills = discoverSkills(opts.Config)
	}
	skillTracker := skills.NewTracker(activeSkills)

	c := &coordinator{
		cfg:                    opts.Config,
		sessions:               opts.Sessions,
		messages:               opts.Messages,
		permissions:            opts.Permissions,
		questions:              opts.Questions,
		history:                opts.History,
		filetracker:            opts.FileTracker,
		lspManager:             opts.LSPManager,
		notify:                 opts.Notify,
		runComplete:            opts.RunComplete,
		agents:                 make(map[string]SessionAgent),
		routerSavingsBySession: make(map[string]float64),
		allSkills:              allSkills,
		activeSkills:           activeSkills,
		skillTracker:           skillTracker,
		interactive:            opts.Interactive,
	}

	agentCfg, ok := opts.Config.Config().Agents[config.AgentCoder]
	if !ok {
		return nil, errCoderAgentNotConfigured
	}

	coderPrompt, err := coderPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}

	agent, err := c.buildAgent(ctx, coderPrompt, agentCfg, false)
	if err != nil {
		return nil, err
	}
	c.agents[config.AgentCoder] = agent

	planCfg, ok := c.cfg.Config().Agents[config.AgentPlan]
	if !ok {
		return nil, errPlanAgentNotConfigured
	}

	planSystemPrompt, err := planPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}

	planAgent, err := c.buildAgent(ctx, planSystemPrompt, planCfg, false)
	if err != nil {
		return nil, err
	}
	c.agents[config.AgentPlan] = planAgent

	c.mainAgent = agent
	c.mainAgentName = config.AgentCoder
	return c, nil
}

// activeAgent returns the coordinator's current main agent and its config
// name as one snapshot. Callers use the snapshot for the whole operation,
// so a SetMainAgent racing mid-flight never splits a run, model refresh, or
// summarize across two agents.
func (c *coordinator) activeAgent() (SessionAgent, string) {
	c.agentMu.RLock()
	defer c.agentMu.RUnlock()
	return c.mainAgent, c.mainAgentName
}

// currentAgent returns the current main agent.
func (c *coordinator) currentAgent() SessionAgent {
	agent, _ := c.activeAgent()
	return agent
}

// setLastRouterDecision records the most recent router decision so a
// future status-bar indicator can read it. It is coordinator-wide, not
// per-session, matching how Model() already exposes a single "current"
// snapshot rather than per-session state.
func (c *coordinator) setLastRouterDecision(d router.Decision) {
	c.routerMu.Lock()
	defer c.routerMu.Unlock()
	c.lastRouterDecision = d
	c.hasRouterDecision = true
}

// clearLastRouterDecision marks that no router decision was actually
// applied to the most recent message — either the router failed open, or
// its chosen effort was not supported by the model in use. The status
// bar must reflect "no decision was applied to the last message", not a
// stale decision from an earlier message.
func (c *coordinator) clearLastRouterDecision() {
	c.routerMu.Lock()
	defer c.routerMu.Unlock()
	c.lastRouterDecision = router.Decision{}
	c.hasRouterDecision = false
}

// LastRouterDecision returns the most recent router decision and whether
// one has been made yet in this process.
func (c *coordinator) LastRouterDecision() (router.Decision, bool) {
	c.routerMu.RLock()
	defer c.routerMu.RUnlock()
	return c.lastRouterDecision, c.hasRouterDecision
}

// setRouterQuerying records whether a router HTTP call is currently in
// flight, and which model it was sent to, so the sidebar can show a live
// "consulting" indicator instead of only the decision the call eventually
// produces. Cleared (active=false) once the call returns, whether it
// succeeded or failed.
func (c *coordinator) setRouterQuerying(model string, active bool) {
	c.routerMu.Lock()
	defer c.routerMu.Unlock()
	c.routerQuerying = active
	if active {
		c.routerQueryingModel = model
		c.lastRouterModel = model
	} else {
		c.routerQueryingModel = ""
	}
}

// RouterQuerying returns the model a router call is currently in flight
// against, and whether one is in flight at all.
func (c *coordinator) RouterQuerying() (string, bool) {
	c.routerMu.RLock()
	defer c.routerMu.RUnlock()
	return c.routerQueryingModel, c.routerQuerying
}

// LastRouterModel returns the router backend's own model id used for the
// most recent router call (regardless of whether a decision from it was
// applied), or "" if the router has never been called this process.
func (c *coordinator) LastRouterModel() string {
	c.routerMu.RLock()
	defer c.routerMu.RUnlock()
	return c.lastRouterModel
}

// setLastRouterError records the most recent router failure message, or
// clears it ("") after a successful call.
func (c *coordinator) setLastRouterError(msg string) {
	c.routerMu.Lock()
	defer c.routerMu.Unlock()
	c.lastRouterError = msg
}

// LastRouterError returns the most recent router failure message while
// the router is enabled, or "" if the last consulted call succeeded (or
// the router has never been consulted this process).
func (c *coordinator) LastRouterError() string {
	c.routerMu.RLock()
	defer c.routerMu.RUnlock()
	return c.lastRouterError
}

// addRouterSavings accumulates an estimated dollar savings (or, when
// negative, an overspend) for sessionID from one router-applied message.
func (c *coordinator) addRouterSavings(sessionID string, delta float64) {
	c.savingsMu.Lock()
	defer c.savingsMu.Unlock()
	if c.routerSavingsBySession == nil {
		c.routerSavingsBySession = make(map[string]float64)
	}
	c.routerSavingsBySession[sessionID] += delta
}

// RouterSavings returns the cumulative estimated dollar savings the
// router has produced for sessionID this process, by comparing what each
// router-applied message actually cost against what the session's
// statically configured model would have cost for the same token usage.
// It is an estimate, not a measured figure — the counterfactual call is
// never actually made — and is 0 for a session the router has never
// affected.
func (c *coordinator) RouterSavings(sessionID string) float64 {
	c.savingsMu.Lock()
	defer c.savingsMu.Unlock()
	return c.routerSavingsBySession[sessionID]
}

// estimateModelCost prices usage against model's catalog rates, the same
// formula sessionAgent.updateSessionUsage uses for the real session
// cost, so a router-savings estimate stays comparable to it.
func estimateModelCost(model catwalk.Model, usage fantasy.Usage) float64 {
	return model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		model.CostPer1MIn/1e6*float64(usage.InputTokens) +
		model.CostPer1MOut/1e6*float64(usage.OutputTokens)
}

func (c *coordinator) SetMainAgent(agentName string) error {
	c.agentMu.Lock()
	defer c.agentMu.Unlock()
	agent, ok := c.agents[agentName]
	if !ok {
		return fmt.Errorf("%w: %s", errMainAgentNotFound, agentName)
	}
	c.mainAgent = agent
	c.mainAgentName = agentName
	return nil
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

	// MCP servers connect asynchronously (see mcp.Initialize).
	//
	// Interactive runs never wait for that to finish: the tool list below
	// is built from whatever is registered right now, servers still
	// connecting are simply absent from this run's palette, and they are
	// picked up by later runs once they register and publish
	// EventToolsListChanged. Blocking here froze the TUI for the duration
	// of the slowest server's connect timeout whenever a prompt was sent
	// before initialization finished — most visibly on the first message.
	//
	// Non-interactive runs get a single shot at the tool palette, so they
	// do wait for initialization to settle — but bounded by InitWaitBudget
	// rather than each server's connect timeout, so a server wedged
	// mid-handshake cannot stall a headless run for minutes. Past the
	// budget the turn proceeds without the stragglers; their tools simply
	// stay absent from this run.
	if !c.interactive {
		if err := mcp.WaitForInitBudget(ctx, mcp.InitWaitBudget); err != nil {
			return nil, fmt.Errorf("failed to wait for MCP initialization: %w", err)
		}
	}

	// refresh models before each run. Snapshot the agent first: the run,
	// its model settings, and the model refresh below must all target the
	// same agent even if SetMainAgent swaps the main agent mid-flight.
	agent, agentName := c.activeAgent()

	routerCfg := c.cfg.Config().Options.Router
	providerAPIKey := ""
	if p, ok := c.cfg.Config().Providers.Get(routerProviderName(routerCfg)); ok {
		providerAPIKey = p.APIKey

	}
	decision, decisionOK, routerErr := resolveRouterDecision(ctx, routerCfg, providerAPIKey, prompt, c.setRouterQuerying)
	if routerCfg != nil && routerCfg.Enabled {
		// Only surface an error while the router is actually turned on —
		// routerErr is nil (not an error) when it's simply disabled, but
		// this also guards against a future resolveRouterDecision that
		// returns a stray non-nil error on the disabled path.
		if routerErr != nil {
			c.setLastRouterError(routerErr.Error())
		} else {
			c.setLastRouterError("")
		}
	}
	if decisionOK {
		// The classifier call happened and cost this regardless of
		// whether its decision is applied below, or of whether the main
		// LLM call that follows even succeeds — charge it now rather
		// than deferring to where the rest of the savings math lives
		// further down, which is gated on that call having succeeded.
		c.addRouterSavings(sessionID, -decision.CallCost)
	}

	if err := c.updateAgentModels(ctx, agent, agentName); err != nil {
		return nil, fmt.Errorf("failed to update models: %w", err)
	}

	model := agent.Model()
	// baselineModel is the "what if" the router savings estimate below
	// prices the same usage against. With a model pool configured, that's
	// the pool's priciest model — "how much do I save vs. always sending
	// this to the top model" — since the pool is the actual alternative
	// the router is choosing between. Without one (reasoning-effort-only
	// routing, no model switching), it falls back to the session's
	// statically configured model, which is the only alternative there
	// is.
	baselineModel := model.CatwalkCfg
	if routerCfg != nil && len(routerCfg.ModelPool) > 0 {
		if priciest, ok := c.mostExpensivePoolModelAcrossProviders(routerCfg.ModelPool); ok {
			baselineModel = priciest
		}
	}
	var modelOverride *Model
	// Gate on the model_choice answer's own confidence being at least
	// somewhat better than picking blindly among the pool — not on
	// routerCfg's confidence_threshold (that one is calibrated for "is
	// this a good decision", a soft LowConfidence flag applied either
	// way; a classifier can be well-calibrated there while essentially
	// guessing on model_choice, since it was never trained to judge
	// unfamiliar model ids). Below the random-chance floor isn't "low
	// confidence", it's noise indistinguishable from a coin flip across
	// the pool, and applying it every single message just because a
	// low bar like 0.42 is normally still meaningful signal would defeat
	// routing entirely. See router.Decision.ModelConfidence and
	// RouterOptions.MinModelConfidence (the floor itself is configurable
	// per EffectiveMinModelConfidence — 0/unset computes 1/pool size).
	if decisionOK && routerCfg != nil && len(routerCfg.ModelPool) > 0 &&
		decision.ModelConfidence > routerCfg.EffectiveMinModelConfidence(len(routerCfg.ModelPool)) {
		if override, ok := c.resolveRouterModelOverride(ctx, routerCfg.ModelPool, decision.ModelID); ok {
			modelOverride = &override
			model = override
		}
	}

	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	var effortOverride string
	appliedDecision := router.Decision{}
	appliedAny := false
	if decisionOK {
		appliedDecision.Confidence = decision.Confidence
		appliedDecision.LowConfidence = decision.LowConfidence
	}
	if decisionOK && slices.Contains(model.CatwalkCfg.ReasoningLevels, decision.ReasoningEffort) {
		effortOverride = decision.ReasoningEffort
		appliedDecision.ReasoningEffort = decision.ReasoningEffort
		appliedAny = true
	}
	if modelOverride != nil {
		appliedDecision.ModelID = decision.ModelID
		appliedAny = true
	}
	if appliedAny {
		c.setLastRouterDecision(appliedDecision)
	} else {
		c.clearLastRouterDecision()
	}

	providerCfg, ok := c.cfg.Config().Providers.Get(model.ModelCfg.Provider)
	if !ok {
		return nil, errModelProviderNotConfigured
	}

	mergedOptions, temp, topP, topK, freqPenalty, presPenalty := mergeCallOptions(model, providerCfg, effortOverride)

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
	channel := ChannelFromContext(ctx)
	c.syncSessionChannel(ctx, sessionID, channel)
	// Propagate the applied reasoning-effort and model decisions to "task"
	// sub-agents spawned during this turn, but only when the router
	// opted into it (RouterOptions.ApplySubagents) — sub-agents keep
	// their own statically configured model/effort otherwise.
	if routerCfg != nil && routerCfg.ApplySubagents {
		if effortOverride != "" {
			ctx = WithRouterSubAgentEffort(ctx, effortOverride)
		}
		if modelOverride != nil {
			ctx = WithRouterSubAgentModel(ctx, modelOverride)
		}
	}
	run := func() (*fantasy.AgentResult, error) {
		return agent.Run(ctx, SessionAgentCall{
			SessionID:         sessionID,
			RunID:             runID,
			Channel:           channel,
			Prompt:            prompt,
			HiddenUserMessage: message.HiddenUserMessage(ctx),
			Attachments:       attachments,
			MaxOutputTokens:   maxTokens,
			ProviderOptions:   mergedOptions,
			ModelOverride:     modelOverride,
			Temperature:       temp,
			TopP:              topP,
			TopK:              callTopK(providerCfg, topK),
			FrequencyPenalty:  freqPenalty,
			PresencePenalty:   presPenalty,
			OnComplete:        onComplete,
			Accepted:          accept,
			OnAuthRefresh:     c.makeAuthRefreshCallback(providerCfg),
		})
	}
	beforeLoaded := c.skillTracker.LoadedNames()
	result, originalErr := run()
	logTurnSkillUsage(sessionID, prompt, c.activeSkills, c.skillTracker, beforeLoaded)

	// Estimate what the router saved (or cost extra) on this message: the
	// same usage priced at the model actually used vs. baselineModel.
	// appliedAny (set above) is the router-savings signal that also
	// gates setLastRouterDecision, so this only accumulates when the
	// router actually changed something about the call, never merely
	// because it was consulted.
	if decisionOK && result != nil {
		usage := result.TotalUsage
		fields := []any{
			"session", sessionID, "model", model.CatwalkCfg.ID,
			"input_tokens", usage.InputTokens, "output_tokens", usage.OutputTokens,
			"cache_creation_tokens", usage.CacheCreationTokens, "cache_read_tokens", usage.CacheReadTokens,
			"router_call_cost_usd", decision.CallCost,
			"router_model_choice", decision.ModelID, "router_model_confidence", decision.ModelConfidence,
			"router_model_applied", modelOverride != nil,
		}
		if appliedAny {
			actualCost := estimateModelCost(model.CatwalkCfg, usage)
			baselineCost := estimateModelCost(baselineModel, usage)
			delta := baselineCost - actualCost
			c.addRouterSavings(sessionID, delta)
			fields = append(fields,
				"baseline_model", baselineModel.ID,
				"reasoning_effort", appliedDecision.ReasoningEffort, "confidence", appliedDecision.Confidence,
				"delta_usd", delta)
		}
		fields = append(fields, "session_total_usd", c.RouterSavings(sessionID))
		slog.Info("Router turn usage", fields...)
	}

	// Notify only if still unauthorized after retry — a successful
	// retry means the user doesn't need to re-authenticate. AWS SSO is
	// handled transparently inside OnAuthRefresh, so it needs no post-run
	// notification here.
	if originalErr != nil && isUnauthorized(originalErr) && c.notify != nil && model.ModelCfg.Provider == hyper.Name {
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

// syncSessionChannel reconciles the session's persisted channel binding with
// the origin of the turn about to run. A channel-originated turn (re)binds
// the session to that channel — the newest push wins — and a local turn
// clears a stale binding, since the session is no longer channel-driven once
// the user takes it over directly. This is the binding's whole lifecycle:
// it is only ever a reflection of the most recent turn's origin, and the
// column is dropped with the session row when the session is deleted, so
// there is no separate state to reap.
//
// Failures are logged and the turn proceeds: the binding is provenance for
// reply routing, not a precondition for running.
func (c *coordinator) syncSessionChannel(ctx context.Context, sessionID, channel string) {
	sess, err := c.sessions.Get(ctx, sessionID)
	if err != nil {
		// A missing session is expected (it may be created later in the
		// run), but a real database failure would otherwise be silent.
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("Failed to load session for channel binding sync",
				"session", sessionID, "channel", channel, "error", err)
		}
		return
	}
	if sess.Channel == channel {
		return
	}
	if _, err := c.sessions.SetChannel(ctx, sessionID, channel); err != nil {
		slog.Warn("Failed to sync session channel binding",
			"session", sessionID, "channel", channel, "error", err)
	}
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

// resolveRouterDecision asks the configured router backend how much
// reasoning effort to use for prompt, when the router is enabled. It
// returns ok=false whenever the decision cannot be trusted or applied:
// router disabled, an unrecognized provider, no base URL for a local
// provider, request failure or timeout, or an unusable response. All of
// these fail open — the caller keeps using its currently active agent and
// that agent's normally configured reasoning effort, unchanged. The router
// never selects which agent handles the call — only its reasoning effort —
// because Crush's per-turn dispatch state (Cancel, IsSessionBusy, the
// message queue) lives per active agent, and switching agents mid-router
// would leave that state pointing at the wrong agent.
//
// querying, when non-nil, is called with the endpoint's model and true
// right before the HTTP call, and with the same model and false once it
// returns (success or failure) — a live "consulting" signal for the UI,
// entirely separate from the decision this function returns.
func resolveRouterDecision(ctx context.Context, cfg *config.RouterOptions, providerAPIKey string, prompt string, querying func(model string, active bool)) (router.Decision, bool, error) {
	if cfg == nil || !cfg.Enabled {
		return router.Decision{}, false, nil
	}

	endpoint, err := resolveRouterEndpoint(cfg, providerAPIKey)
	if err != nil {
		return router.Decision{}, false, err
	}

	if querying != nil {
		querying(endpoint.model, true)
		defer querying(endpoint.model, false)
	}

	var clientOpts []router.Option
	if endpoint.nestedInput {
		clientOpts = append(clientOpts, router.WithNestedInput())
	}
	client := router.NewClient(endpoint.baseURL, endpoint.path, endpoint.apiKey, endpoint.model, cfg.EffectiveTimeout(), clientOpts...)
	callCtx, cancel := context.WithTimeout(ctx, cfg.EffectiveTimeout())
	defer cancel()

	answers, callCost, err := client.Decide(callCtx, prompt, router.BuildQuestions(cfg.ModelPool))
	if err != nil {
		slog.Warn("Router call failed, keeping current effort", "error", err)
		return router.Decision{}, false, fmt.Errorf("router call failed: %w", err)
	}

	decision, err := router.MapDecision(answers, cfg.EffectiveConfidenceThreshold())
	if err != nil {
		slog.Warn("Router returned an unusable decision, keeping current effort", "error", err)
		return router.Decision{}, false, fmt.Errorf("router returned an unusable decision: %w", err)
	}
	decision.CallCost = callCost

	return decision, true, nil
}

// resolveSubAgentRouterOverrides asks the router for a fresh decision
// scoped to a sub-agent's own task prompt — not the parent turn's prompt —
// so a "task" sub-agent handling a small, simple piece of a larger turn
// can get a cheaper model/effort than whatever the parent turn needed,
// and vice versa. Gates effort and model_choice independently, exactly
// like coordinator.run does for the top-level turn: an unsupported effort
// or a model_choice below EffectiveMinModelConfidence comes back as a
// legitimate "no override" for that field, not a failure.
//
// decisionOK is false only when the router call itself didn't produce a
// usable decision at all (disabled, erroring, timing out) — callers use
// this to fall back a tier, to the parent turn's already-applied
// decision, rather than to defaults every field individually.
func (c *coordinator) resolveSubAgentRouterOverrides(ctx context.Context, routerCfg *config.RouterOptions, providerAPIKey, prompt string, model Model) (effortOverride string, modelOverride *Model, decisionOK bool) {
	decision, ok, _ := resolveRouterDecision(ctx, routerCfg, providerAPIKey, prompt, nil)
	if !ok {
		return "", nil, false
	}

	if slices.Contains(model.CatwalkCfg.ReasoningLevels, decision.ReasoningEffort) {
		effortOverride = decision.ReasoningEffort
	}
	if len(routerCfg.ModelPool) > 0 && decision.ModelConfidence > routerCfg.EffectiveMinModelConfidence(len(routerCfg.ModelPool)) {
		if override, ok := c.resolveRouterModelOverride(ctx, routerCfg.ModelPool, decision.ModelID); ok {
			modelOverride = &override
		}
	}
	return effortOverride, modelOverride, true
}

// routerPoolProvider finds modelID in every configured provider's catalog
// and returns the provider that offers it along with its catalog entry,
// so the pool may name a chat model from any provider, not just
// OpenRouter. Providers are searched in a stable (id-sorted) order so a
// model id present under more than one provider always resolves to the
// same one.
func (c *coordinator) routerPoolProvider(modelID string) (config.ProviderConfig, catwalk.Model, bool) {
	providers := c.cfg.Config().Providers
	ids := make([]string, 0, providers.Len())
	for id := range providers.Seq2() {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		providerCfg, ok := providers.Get(id)
		if !ok || providerCfg.Disable {
			continue
		}
		for _, m := range providerCfg.Models {
			if m.ID == modelID {
				return providerCfg, m, true
			}
		}
	}
	return config.ProviderConfig{}, catwalk.Model{}, false
}

// resolveRouterModelOverride builds a one-off Model for a router-chosen
// model id, without mutating any shared sessionAgent state. It returns
// ok=false — meaning the caller must fail open and keep using the agent's
// normally configured model — when modelID is empty, isn't a member of
// pool, isn't in any configured provider's known model catalog, or when
// building its provider/language-model client fails for any reason.
func (c *coordinator) resolveRouterModelOverride(ctx context.Context, pool []string, modelID string) (Model, bool) {
	if modelID == "" || !slices.Contains(pool, modelID) {
		return Model{}, false
	}

	providerCfg, catwalkModel, ok := c.routerPoolProvider(modelID)
	if !ok {
		slog.Warn("Router chose a model outside every configured provider's catalog, keeping current model", "model", modelID)
		return Model{}, false
	}

	selected := config.SelectedModel{Model: modelID, Provider: providerCfg.ID}
	provider, err := c.buildProvider(providerCfg, selected, false)
	if err != nil {
		slog.Warn("Router-chosen model's provider failed to build, keeping current model", "model", modelID, "error", err)
		return Model{}, false
	}

	langModel, err := provider.LanguageModel(ctx, modelID)
	if err != nil {
		slog.Warn("Router-chosen model's client failed to build, keeping current model", "model", modelID, "error", err)
		return Model{}, false
	}
	langModel = newRequestTimeoutModel(langModel, c.cfg.Config().Options.GetRequestTimeout())

	return Model{
		Model:      langModel,
		CatwalkCfg: catwalkModel,
		ModelCfg:   selected,
		FlatRate:   providerCfg.FlatRate,
	}, true
}

// mostExpensivePoolModelAcrossProviders returns the catalog entry for
// whichever pool model has the highest combined per-1M input+output rate
// across every configured provider, so the savings estimate prices the
// pool's top model regardless of which provider it lives on.
func (c *coordinator) mostExpensivePoolModelAcrossProviders(pool []string) (catwalk.Model, bool) {
	var priciest catwalk.Model
	found := false
	for id := range c.cfg.Config().Providers.Seq2() {
		providerCfg, ok := c.cfg.Config().Providers.Get(id)
		if !ok || providerCfg.Disable {
			continue
		}
		for _, m := range providerCfg.Models {
			if !slices.Contains(pool, m.ID) {
				continue
			}
			if !found || m.CostPer1MIn+m.CostPer1MOut > priciest.CostPer1MIn+priciest.CostPer1MOut {
				priciest = m
				found = true
			}
		}
	}
	return priciest, found
}

// mostExpensivePoolModel returns the catalog entry for whichever model in
// pool has the highest combined per-1M input+output rate, so the router
// savings estimate can answer "how much did picking a cheaper model save
// vs. always using the top of this pool" — the actual alternative the
// router is choosing between, unlike the session's static default model
// (which may be on an unrelated, unpriced, or flat-rate provider). ok is
// false when none of pool's ids are in the provider's known catalog.
func mostExpensivePoolModel(providerCfg config.ProviderConfig, pool []string) (catwalk.Model, bool) {
	var priciest catwalk.Model
	found := false
	for _, id := range pool {
		for _, m := range providerCfg.Models {
			if m.ID != id {
				continue
			}
			if !found || m.CostPer1MIn+m.CostPer1MOut > priciest.CostPer1MIn+priciest.CostPer1MOut {
				priciest = m
				found = true
			}
			break
		}
	}
	return priciest, found
}

// routerEndpoint is the resolved backend a router call is sent to.
type routerEndpoint struct {
	baseURL     string
	path        string
	apiKey      string
	model       string
	nestedInput bool
}

// resolveRouterEndpoint maps the router config onto a concrete backend
// from router.Backends. It is split out from resolveRouterDecision so the
// provider switch (per-backend model default, the API key fallback to
// Crush's own provider of the same name, and the fail-open on unrecognized
// providers) can be tested without any network call. providerAPIKey is the
// key Crush already has for the provider named like cfg.Provider (e.g.
// "openrouter" or "opencode-zen"), used when the router has no key of its
// own. It returns ok=false for an unrecognized provider or a provider that
// needs a base URL but has none.
func resolveRouterEndpoint(cfg *config.RouterOptions, providerAPIKey string) (routerEndpoint, error) {
	backend, ok := router.Backends[routerProviderName(cfg)]
	if !ok {
		slog.Warn("Router enabled with unrecognized provider, keeping current effort", "provider", cfg.Provider)
		return routerEndpoint{}, fmt.Errorf("unrecognized router provider %q", cfg.Provider)
	}

	endpoint := routerEndpoint{
		baseURL:     cmp.Or(cfg.BaseURL, backend.BaseURL),
		path:        backend.Path,
		apiKey:      cmp.Or(cfg.APIKey, providerAPIKey),
		model:       cmp.Or(cfg.Model, backend.DefaultModel),
		nestedInput: backend.NestedInput,
	}
	if backend.FixedBaseURL {
		endpoint.baseURL = backend.BaseURL
	}
	if endpoint.baseURL == "" {
		slog.Warn("Router enabled but no base URL configured", "provider", cfg.Provider)
		return routerEndpoint{}, fmt.Errorf("router provider %q needs a base_url and none is configured", cfg.Provider)
	}
	return endpoint, nil
}

// routerProviderName returns cfg.Provider, or "openrouter" when unset.
func routerProviderName(cfg *config.RouterOptions) string {
	if cfg == nil || cfg.Provider == "" {
		return "openrouter"
	}
	return cfg.Provider
}

func getProviderOptions(model Model, providerCfg config.ProviderConfig, effortOverride string) fantasy.ProviderOptions {
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
	if effortOverride != "" && slices.Contains(model.CatwalkCfg.ReasoningLevels, effortOverride) {
		reasoningEffort = effortOverride
	}
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
			case string(catwalk.InferenceProviderOpenCodeGo), string(catwalk.InferenceProviderOpenCodeZen):
				// MiniMax models use the "thinking" parameter instead of
				// "reasoning_effort". Other models on these providers still
				// use the standard field.
				if !strings.HasPrefix(strings.ToLower(model.CatwalkCfg.ID), "minimax") {
					mergedOptions["reasoning_effort"] = reasoningEffort
				}
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
				"enable_thinking": model.ModelCfg.Think || reasoningEffort != "" && reasoningEffort != "none",
			}

		case string(catwalk.InferenceProviderOpenCodeGo), string(catwalk.InferenceProviderOpenCodeZen):
			// MiniMax M3 uses the "thinking" parameter to control reasoning.
			// "reasoning_split" must be true so thinking content is returned
			// in the "reasoning_content" field instead of inline in "content".
			if strings.HasPrefix(strings.ToLower(model.CatwalkCfg.ID), "minimax") {
				if model.CatwalkCfg.CanReason && (model.ModelCfg.Think || reasoningEffort != "") {
					extraBody["thinking"] = map[string]any{"type": "adaptive"}
					extraBody["reasoning_split"] = true
				} else {
					extraBody["thinking"] = map[string]any{"type": "disabled"}
				}
			}

		case string(catwalk.InferenceProviderAlibabaSingapore), string(catwalk.InferenceProviderAlibabaUS):
			if model.CatwalkCfg.CanReason && !shouldSetEffort {
				extraBody["enable_thinking"] = model.ModelCfg.Think
			}
		}

		mergedOptions["extra_body"] = extraBody

		parsed, err := openaicompat.ParseOptions(mergedOptions)
		if err == nil {
			options[openaicompat.Name] = parsed
		}

	default:
		// Known custom providers (litellm, llamacpp, lmstudio, ollama,
		// omlx) are openai-compat under the hood.
		if discover.IsKnownCustomProvider(string(providerCfg.Type)) {
			// Set "top_k" under "extra_body", as it is not part of the OpenAI protocol
			// and will be explicitly omitted by Fantasy downstream.
			topK := cmp.Or(model.ModelCfg.TopK, model.CatwalkCfg.Options.TopK)
			if topK != nil {
				extraBody, hasExtraBody := mergedOptions["extra_body"].(map[string]any)
				if !hasExtraBody {
					extraBody = make(map[string]any)
					mergedOptions["extra_body"] = extraBody
				}
				if _, hasTopK := extraBody["top_k"]; !hasTopK {
					extraBody["top_k"] = *topK
				}
			}

			_, hasReasoningEffort := mergedOptions["reasoning_effort"]
			if !hasReasoningEffort && shouldSetEffort {
				mergedOptions["reasoning_effort"] = reasoningEffort
			}

			parsed, err := openaicompat.ParseOptions(mergedOptions)
			if err == nil {
				options[openaicompat.Name] = parsed
			} else {
				if topK != nil {
					slog.Warn(
						"Failed to parse provider_options, falling back to top_k only",
						"provider", providerCfg.ID,
						"error", err,
					)

					fallbackMergeOptions := map[string]any{
						"extra_body": map[string]any{"top_k": *topK},
					}
					parsed, err := openaicompat.ParseOptions(fallbackMergeOptions)
					if err == nil {
						options[openaicompat.Name] = parsed
					} else {
						slog.Warn(
							"Failed to parse fallback provider options, this should never happen",
							"provider", providerCfg.ID,
							"error", err,
						)
					}
				}
			}
		}
	}

	return options
}

func mergeCallOptions(model Model, cfg config.ProviderConfig, effortOverride string) (fantasy.ProviderOptions, *float64, *float64, *int64, *float64, *float64) {
	modelOptions := getProviderOptions(model, cfg, effortOverride)
	temp := cmp.Or(model.ModelCfg.Temperature, model.CatwalkCfg.Options.Temperature)
	topP := cmp.Or(model.ModelCfg.TopP, model.CatwalkCfg.Options.TopP)
	topK := cmp.Or(model.ModelCfg.TopK, model.CatwalkCfg.Options.TopK)
	freqPenalty := cmp.Or(model.ModelCfg.FrequencyPenalty, model.CatwalkCfg.Options.FrequencyPenalty)
	presPenalty := cmp.Or(model.ModelCfg.PresencePenalty, model.CatwalkCfg.Options.PresencePenalty)
	return modelOptions, temp, topP, topK, freqPenalty, presPenalty
}

func (c *coordinator) buildAgent(ctx context.Context, prompt *prompt.Prompt, agent config.Agent, isSubAgent bool) (SessionAgent, error) {
	large, small, err := c.buildAgentModels(ctx, isSubAgent)
	if err != nil {
		return nil, err
	}

	largeProviderCfg, _ := c.cfg.Config().Providers.Get(large.ModelCfg.Provider)
	result := NewSessionAgent(SessionAgentOptions{
		LargeModel:           large,
		SmallModel:           small,
		SystemPromptPrefix:   largeProviderCfg.SystemPromptPrefix,
		SystemPrompt:         "",
		IsSubAgent:           isSubAgent,
		DisableAutoSummarize: c.cfg.Config().Options.DisableAutoSummarize,
		IsYolo:               c.permissions.SkipRequests(),
		Sessions:             c.sessions,
		Messages:             c.messages,
		Cfg:                  c.cfg,
		Tools:                nil,
		Notify:               c.notify,
		RunComplete:          c.runComplete,
	})

	// The readiness goroutines below perform one-time setup — building the
	// system prompt and the initial tool list — whose results the
	// coordinator needs for its whole lifetime, so they must survive the
	// caller's context being canceled. Several entry points build an agent
	// from a short-lived HTTP request context: the server's
	// InitAgent/UpdateAgent handlers, and UpdateModels -> buildTools ->
	// agentTool -> buildAgent for the sub-agent. The tool-list build reads
	// the MCP registry as it stands; servers still connecting are picked up
	// by later runs. WithoutCancel drops cancellation while keeping context
	// values; the work is local and always completes.
	initCtx := context.WithoutCancel(ctx)

	c.readyWg.Go(func() error {
		systemPrompt, err := prompt.Build(initCtx, large.Model.Provider(), large.Model.Model(), c.cfg)
		if err != nil {
			return err
		}
		result.SetSystemPrompt(systemPrompt)
		return nil
	})

	c.readyWg.Go(func() error {
		tools, err := c.buildTools(initCtx, agent, isSubAgent)
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

	// Build hook runner if PreToolUse hooks are configured.
	var hookRunner *hooks.Runner
	if preToolHooks := c.cfg.Config().Hooks[hooks.EventPreToolUse]; len(preToolHooks) > 0 {
		hookRunner = hooks.NewRunner(preToolHooks, c.cfg.WorkingDir(), c.cfg.WorkingDir())
	}

	allTools = append(
		allTools,
		tools.NewBashTool(c.permissions, c.cfg.WorkingDir(), c.cfg.Config().Options.DataDirectory, c.cfg.Config().Options.Attribution, modelID),
		tools.NewCrushInfoTool(c.cfg, c.lspManager, c.allSkills, c.activeSkills, c.skillTracker),
		tools.NewCrushLogsTool(logFile),
		tools.NewJobOutputTool(c.cfg.Config().Options.DataDirectory),
		tools.NewJobKillTool(),
		tools.NewDownloadTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewEditTool(c.lspManager, c.permissions, c.history, c.filetracker, c.cfg.WorkingDir()),
		tools.NewMultiEditTool(c.lspManager, c.permissions, c.history, c.filetracker, c.cfg.WorkingDir()),
		tools.NewFetchTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewGlobTool(c.cfg.WorkingDir(), c.cfg.Config().Tools.Glob),
		tools.NewGrepTool(c.cfg.WorkingDir(), c.cfg.Config().Tools.Grep),
		tools.NewLsTool(c.permissions, c.cfg.WorkingDir(), c.cfg.Config().Tools.Ls),
		tools.NewSourcegraphTool(nil),
		tools.NewTodosTool(c.sessions),
		tools.NewViewTool(c.lspManager, c.permissions, c.filetracker, c.skillTracker, c.cfg.WorkingDir(), c.cfg.Config().Options.SkillsPaths...),
		tools.NewWriteTool(c.lspManager, c.permissions, c.history, c.filetracker, c.cfg.WorkingDir()),
	)

	// Question tool is interactive-only and not available to sub-agents.
	if !isSubAgent && c.interactive {
		allTools = append(allTools, tools.NewQuestionTool(c.questions))
	}

	// Add LSP tools if user has configured LSPs or auto_lsp is enabled (nil or true).
	if len(c.cfg.Config().LSP) > 0 || c.cfg.Config().Options.AutoLSP == nil || *c.cfg.Config().Options.AutoLSP {
		allTools = append(
			allTools,
			tools.NewDiagnosticsTool(c.lspManager),
			tools.NewReferencesTool(c.lspManager),
			tools.NewLSPRestartTool(c.lspManager),
			tools.NewSymbolsTool(c.lspManager),
			tools.NewDefinitionTool(c.lspManager),
			tools.NewCallHierarchyTool(c.lspManager),
			tools.NewRenameTool(c.lspManager, c.permissions, c.history, c.filetracker),
			tools.NewReplaceSymbolTool(c.lspManager, c.permissions, c.history, c.filetracker),
		)
	}

	if len(c.cfg.Config().MCP) > 0 {
		allTools = append(
			allTools,
			tools.NewListMCPResourcesTool(c.cfg, c.permissions),
			tools.NewReadMCPResourceTool(c.cfg, c.permissions),
		)
	}

	var filteredTools []fantasy.AgentTool
	for _, tool := range allTools {
		if slices.Contains(agent.AllowedTools, tool.Info().Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	for _, tool := range tools.GetMCPTools(c.permissions, c.cfg, c.cfg.WorkingDir()) {
		if agent.AllowedMCP == nil {
			// No MCP restrictions
			filteredTools = append(filteredTools, tool)
			continue
		}
		if len(agent.AllowedMCP) == 0 {
			// No MCPs allowed
			slog.Debug("No MCPs allowed", "tool", tool.Name(), "agent", agent.Name)
			break
		}

		for mcp, tools := range agent.AllowedMCP {
			if mcp != tool.MCP() {
				continue
			}
			if len(tools) == 0 || slices.Contains(tools, tool.MCPToolName()) {
				filteredTools = append(filteredTools, tool)
				break
			}
			slog.Debug("MCP not allowed", "tool", tool.Name(), "agent", agent.Name)
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
	filteredTools = wrapToolsWithHooks(filteredTools, hookRunner, isSubAgent)

	return filteredTools, nil
}

// TODO: when we support multiple agents we need to change this so that we pass in the agent specific model config
func (c *coordinator) buildAgentModels(ctx context.Context, isSubAgent bool) (Model, Model, error) {
	largeModelCfg, ok := c.cfg.Config().Models[config.SelectedModelTypeLarge]
	if !ok {
		return Model{}, Model{}, errLargeModelNotSelected
	}
	smallModelCfg, ok := c.cfg.Config().Models[config.SelectedModelTypeSmall]
	if !ok {
		return Model{}, Model{}, errSmallModelNotSelected
	}

	largeProviderCfg, ok := c.cfg.Config().Providers.Get(largeModelCfg.Provider)
	if !ok {
		return Model{}, Model{}, errLargeModelProviderNotConfigured
	}

	largeProvider, err := c.buildProvider(largeProviderCfg, largeModelCfg, isSubAgent)
	if err != nil {
		return Model{}, Model{}, err
	}

	smallProviderCfg, ok := c.cfg.Config().Providers.Get(smallModelCfg.Provider)
	if !ok {
		return Model{}, Model{}, errSmallModelProviderNotConfigured
	}

	smallProvider, err := c.buildProvider(smallProviderCfg, smallModelCfg, true)
	if err != nil {
		return Model{}, Model{}, err
	}

	var largeCatwalkModel *catwalk.Model
	var smallCatwalkModel *catwalk.Model

	for _, m := range largeProviderCfg.Models {
		if m.ID == largeModelCfg.Model {
			largeCatwalkModel = &m
		}
	}
	for _, m := range smallProviderCfg.Models {
		if m.ID == smallModelCfg.Model {
			smallCatwalkModel = &m
		}
	}

	if largeCatwalkModel == nil {
		return Model{}, Model{}, errLargeModelNotFound
	}

	if smallCatwalkModel == nil {
		return Model{}, Model{}, errSmallModelNotFound
	}

	largeModelID := largeModelCfg.Model
	smallModelID := smallModelCfg.Model

	if largeModelCfg.Provider == openrouter.Name && isExactoSupported(largeModelID) {
		largeModelID += ":exacto"
	}

	if smallModelCfg.Provider == openrouter.Name && isExactoSupported(smallModelID) {
		smallModelID += ":exacto"
	}

	largeModel, err := largeProvider.LanguageModel(ctx, largeModelID)
	if err != nil {
		return Model{}, Model{}, err
	}
	smallModel, err := smallProvider.LanguageModel(ctx, smallModelID)
	if err != nil {
		return Model{}, Model{}, err
	}

	// Bound each request with the configured timeout so unreachable or hung
	// providers fail instead of blocking a session forever. The wrapper is
	// applied per request, so retries get a fresh budget each attempt.
	requestTimeout := c.cfg.Config().Options.GetRequestTimeout()
	largeModel = newRequestTimeoutModel(largeModel, requestTimeout)
	smallModel = newRequestTimeoutModel(smallModel, requestTimeout)

	// Hyper completions no longer report the hypercredit balance, so wrap
	// the Hyper models to fetch it from /v1/credits on every request.
	if largeModelCfg.Provider == hyper.Name {
		largeModel = newHyperCreditsModel(largeModel, c.hyperAPIKey)
	}
	if smallModelCfg.Provider == hyper.Name {
		smallModel = newHyperCreditsModel(smallModel, c.hyperAPIKey)
	}

	large := Model{
		Model:      largeModel,
		CatwalkCfg: *largeCatwalkModel,
		ModelCfg:   largeModelCfg,
		FlatRate:   largeProviderCfg.FlatRate,
	}
	small := Model{
		Model:      smallModel,
		CatwalkCfg: *smallCatwalkModel,
		ModelCfg:   smallModelCfg,
		FlatRate:   smallProviderCfg.FlatRate,
	}

	return large, small, nil
}

// hyperAPIKey resolves the Hyper API key from the live config, so an
// OAuth token refreshed after the models were built is picked up by the
// next credits fetch.
func (c *coordinator) hyperAPIKey() string {
	return config.ResolveHyperAPIKey(c.cfg.Config())
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

func (c *coordinator) buildOpenaiProvider(baseURL, apiKey string, headers map[string]string, token *oauth.Token) (fantasy.Provider, error) {
	opts := []openai.Option{
		openai.WithAPIKey(apiKey),
		openai.WithUseResponsesAPI(),
	}
	var httpClient *http.Client
	if c.cfg.Config().Options.Debug {
		httpClient = log.NewHTTPClient()
	}
	if token != nil {
		// ChatGPT OAuth: requests go through the Codex backend, which
		// expects account headers and rejects some request fields, so
		// they pass through the Codex transport.
		if httpClient == nil {
			httpClient = &http.Client{}
		}
		httpClient.Transport = &openaioauth.Transport{
			Base:  httpClient.Transport,
			Token: token,
		}
	}
	if httpClient != nil {
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

	case string(catwalk.InferenceProviderOpenCodeGo), string(catwalk.InferenceProviderOpenCodeZen):
		opts = append(
			opts,
			openaicompat.WithUseResponsesAPI(),
			openaicompat.WithResponsesAPIFunc(isOpenCodeResponsesModel),
		)

	case hyper.Name:
		// Hyper may route requests through a Prism model; capture the
		// router headers so the UI can show which model answered.
		opts = append(
			opts,
			openaicompat.WithLanguageModelOptions(
				openai.WithLanguageModelHeaderFunc(hyper.HeaderFunc),
			),
		)
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
		if isOpenCodeMessagesModel(providerCfg.ID, model.Model) {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
			return c.buildAnthropicProvider(baseURL, apiKey, headers, providerCfg.ID)
		}
	}

	switch providerCfg.Type {
	case openai.Name:
		// A ChatGPT login is the provider's single credential: every
		// request goes through the Codex backend with the OAuth token.
		token := providerCfg.OAuthToken
		if token != nil {
			baseURL = openaioauth.CodexBaseURL
			apiKey = token.AccessToken
			headers["originator"] = "crush"
			if token.AccountID != "" {
				headers["chatgpt-account-id"] = token.AccountID
			}
		}
		return c.buildOpenaiProvider(baseURL, apiKey, headers, token)
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
		// Known custom providers (litellm, llamacpp, lmstudio, ollama,
		// omlx) are openai-compat under the hood.
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
	return c.currentAgent().BeginAccepted(sessionID)
}

func (c *coordinator) Cancel(sessionID string) {
	c.currentAgent().Cancel(sessionID)
}

func (c *coordinator) CancelAll() {
	c.currentAgent().CancelAll()
}

func (c *coordinator) ClearQueue(sessionID string) {
	c.currentAgent().ClearQueue(sessionID)
}

func (c *coordinator) IsBusy() bool {
	return c.currentAgent().IsBusy()
}

func (c *coordinator) IsSessionBusy(sessionID string) bool {
	return c.currentAgent().IsSessionBusy(sessionID)
}

func (c *coordinator) Model() Model {
	return c.currentAgent().Model()
}

func (c *coordinator) UpdateModels(ctx context.Context) error {
	// A ChatGPT login without its model catalog — the fetch at login
	// failed, or the credentials predate it — would leave the models
	// dialog's ChatGPT section empty. Fill it in lazily; the guard makes
	// this a no-op once the catalog exists.
	c.cfg.RefetchOpenAIChatGPTModels(ctx)

	agent, name := c.activeAgent()
	return c.updateAgentModels(ctx, agent, name)
}

// updateAgentModels rebuilds the model and tool configuration for the
// given agent from the current config.
func (c *coordinator) updateAgentModels(ctx context.Context, agent SessionAgent, name string) error {
	// build the models again so we make sure we get the latest config
	large, small, err := c.buildAgentModels(ctx, false)
	if err != nil {
		return err
	}
	agent.SetModels(large, small)

	agentCfg, ok := c.cfg.Config().Agents[name]
	if !ok {
		return fmt.Errorf("%w: %s", errMainAgentNotFound, name)
	}

	tools, err := c.buildTools(ctx, agentCfg, false)
	if err != nil {
		return err
	}
	agent.SetTools(tools)
	return nil
}

func (c *coordinator) QueuedPrompts(sessionID string) int {
	return c.currentAgent().QueuedPrompts(sessionID)
}

func (c *coordinator) QueuedPromptsList(sessionID string) []string {
	return c.currentAgent().QueuedPromptsList(sessionID)
}

func (c *coordinator) Summarize(ctx context.Context, sessionID string) error {
	agent := c.currentAgent()
	providerCfg, ok := c.cfg.Config().Providers.Get(agent.Model().ModelCfg.Provider)
	if !ok {
		return errModelProviderNotConfigured
	}

	if err := c.refreshTokenIfExpired(ctx, providerCfg); err != nil {
		slog.Error("Failed to refresh OAuth2 token before summarize. Proceeding with existing token.", "error", err)
	}

	// Auth failures during summarize flow through fantasy's OnAuthRefresh,
	// the same path used by regular turns.
	return agent.Summarize(ctx, sessionID, getProviderOptions(agent.Model(), providerCfg, ""), c.makeAuthRefreshCallback(providerCfg), nil)
}

// GenerateTitle generates a session title using the current agent.
func (c *coordinator) GenerateTitle(ctx context.Context, sessionID, prompt string) {
	agent := c.currentAgent()
	if agent == nil {
		return
	}
	agent.GenerateTitle(ctx, sessionID, prompt)
}

// refreshTokenIfExpired proactively refreshes the OAuth token if it has expired.
func (c *coordinator) refreshTokenIfExpired(ctx context.Context, providerCfg config.ProviderConfig) error {
	if providerCfg.OAuthToken == nil || !providerCfg.OAuthToken.IsExpired() {
		return nil
	}
	slog.Debug("Token needs to be refreshed", "provider", providerCfg.ID)
	return c.refreshOAuth2Token(ctx, providerCfg)
}

// retryAfterUnauthorized attempts to refresh credentials after an auth error
// and returns nil if the request should be retried. For OAuth providers whose
// refresh token is revoked, and for Bedrock providers whose AWS SSO session
// has expired, it triggers interactive re-authentication and blocks until the
// user completes it (or the context is cancelled).
func (c *coordinator) retryAfterUnauthorized(ctx context.Context, providerCfg config.ProviderConfig) error {
	switch {
	case providerCfg.OAuthToken != nil:
		slog.Debug("Received 401. Refreshing token and retrying", "provider", providerCfg.ID)
		if err := c.refreshOAuth2Token(ctx, providerCfg); err != nil {
			// If the refresh token was revoked, trigger interactive
			// re-auth and wait for the user to complete it.
			var exchangeErr *oauth.TokenExchangeError
			if c.notify != nil && errors.As(err, &exchangeErr) && exchangeErr.IsRefreshTokenRevoked() {
				slog.Info("Refresh token revoked, waiting for re-authentication", "provider", providerCfg.ID)
				c.notify.Publish(pubsub.CreatedEvent, notify.Notification{
					Type:       notify.TypeReAuthenticate,
					ProviderID: providerCfg.ID,
				})
				return c.waitForInteractiveReauth(ctx, providerCfg.ID)
			}
			return err
		}
		return nil
	case providerCfg.AWSAuthRefresh != "":
		return c.refreshAWSCredentials(ctx, providerCfg)
	case strings.Contains(providerCfg.APIKeyTemplate, "$"):
		slog.Debug("Received 401. Refreshing API Key template and retrying", "provider", providerCfg.ID)
		return c.refreshApiKeyTemplate(ctx, providerCfg)
	default:
		return nil
	}
}

// errNoInteractiveAuth is returned by an OnAuthRefresh callback when a
// provider needs interactive re-authentication but no notifier is available
// to drive it (e.g. headless runs). Returning it surfaces the original auth
// error rather than retrying.
var errNoInteractiveAuth = errors.New("interactive authentication unavailable")

// waitForInteractiveReauth blocks until interactive re-authentication for the
// provider completes (signalled via SignalAuthComplete) or the context is
// cancelled, then rebuilds models so the next attempt picks up fresh
// credentials. Returns nil when the caller should retry.
func (c *coordinator) waitForInteractiveReauth(ctx context.Context, providerID string) error {
	// Use a detached context with a generous timeout so the wait survives
	// agent run cancellation. The user needs time to complete browser-based
	// authentication.
	waitCtx, waitCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
	defer waitCancel()
	slog.Info("Blocking on WaitForTokenChange", "provider", providerID)
	if waitErr := c.cfg.WaitForTokenChange(waitCtx, providerID); waitErr != nil {
		slog.Info("WaitForTokenChange returned error", "provider", providerID, "error", waitErr)
		return waitErr
	}
	// If the original context was cancelled during the wait, fantasy's retry
	// would fail immediately, so surface the cancellation instead.
	if ctx.Err() != nil {
		slog.Warn("Original context cancelled during auth wait, cannot retry",
			"provider", providerID, "ctx_err", ctx.Err())
		return ctx.Err()
	}
	// Rebuild models so ModelProvider picks up the fresh credentials.
	if updateErr := c.UpdateModels(waitCtx); updateErr != nil {
		slog.Error("Failed to update models after re-authentication", "error", updateErr)
		return updateErr
	}
	slog.Info("Models updated, returning nil to retry", "provider", providerID)
	return nil
}

// isUnauthorized reports whether err is an HTTP 401 from a provider.
func isUnauthorized(err error) bool {
	var providerErr *fantasy.ProviderError
	return errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusUnauthorized
}

// makeAuthRefreshCallback returns an OnAuthRefresh callback for fantasy that
// delegates to the coordinator's existing credential refresh logic. Returns
// nil if no refresh mechanism is configured for the provider.
func (c *coordinator) makeAuthRefreshCallback(providerCfg config.ProviderConfig) func(context.Context, *fantasy.ProviderError) error {
	if providerCfg.OAuthToken == nil &&
		!strings.Contains(providerCfg.APIKeyTemplate, "$") &&
		providerCfg.AWSAuthRefresh == "" {
		return nil
	}
	return func(ctx context.Context, _ *fantasy.ProviderError) error {
		return c.retryAfterUnauthorized(ctx, providerCfg)
	}
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
	// EffortOverride is the parent turn's already-applied router effort,
	// tagged onto the context in coordinator.run when
	// RouterOptions.ApplySubagents is enabled. runSubAgent only falls
	// back to it (tier 2) when its own fresh, prompt-scoped router call
	// (tier 1) is unavailable; empty means no override at either tier,
	// so the sub-agent keeps its own configured effort (tier 3) exactly
	// as before this field existed. Ignored outright for models whose
	// ReasoningLevels doesn't contain it.
	EffortOverride string
	// ModelOverride is the parent turn's already-applied router model
	// choice, same tiering as EffortOverride: runSubAgent prefers its
	// own fresh decision for this sub-agent's own prompt, and only falls
	// back to this parent-turn value when that fresh call is
	// unavailable. nil means no override at either tier — the sub-agent
	// keeps its statically configured model.
	ModelOverride *Model
}

// callTopK returns topK for use on fantasy.Call.TopK, suppressing it for
// known custom providers: getProviderOptions already carries top_k for
// them via extra_body, and passing it here too makes Fantasy emit a
// spurious "top_k unsupported" warning for every turn.
func callTopK(providerCfg config.ProviderConfig, topK *int64) *int64 {
	if discover.IsKnownCustomProvider(string(providerCfg.Type)) {
		return nil
	}
	return topK
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
	staticModelID := model.CatwalkCfg.ID

	// Resilience chain when ApplySubagents is enabled: prefer a decision
	// scoped to this sub-agent's own prompt (tier 1); if the router call
	// itself is unavailable, fall back to the parent turn's already-applied
	// decision, passed in via params.EffortOverride/ModelOverride from the
	// context the coordinator tagged in run() (tier 2); if that's also
	// empty, the zero values below leave the sub-agent on its own static
	// model/effort, exactly as before ApplySubagents existed (tier 3).
	effortOverride := params.EffortOverride
	modelOverride := params.ModelOverride
	if routerCfg := c.cfg.Config().Options.Router; routerCfg != nil && routerCfg.Enabled && routerCfg.ApplySubagents {
		providerAPIKey := ""
		if p, ok := c.cfg.Config().Providers.Get(routerProviderName(routerCfg)); ok {
			providerAPIKey = p.APIKey
		}
		if effort, modelOv, ok := c.resolveSubAgentRouterOverrides(ctx, routerCfg, providerAPIKey, params.Prompt, model); ok {
			effortOverride, modelOverride = effort, modelOv
		}
	}
	params.EffortOverride = effortOverride
	params.ModelOverride = modelOverride

	// Apply model override from router if present and ApplySubagents is enabled.
	if params.ModelOverride != nil {
		model = *params.ModelOverride
	}
	if params.ModelOverride != nil || params.EffortOverride != "" {
		slog.Info("Router applied to sub-agent",
			"session", session.ID,
			"static_model", staticModelID,
			"applied_model", model.CatwalkCfg.ID,
			"router_effort_override", params.EffortOverride,
		)
	}
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
			ProviderOptions:  getProviderOptions(model, providerCfg, params.EffortOverride),
			ModelOverride:    params.ModelOverride,
			Temperature:      model.ModelCfg.Temperature,
			TopP:             model.ModelCfg.TopP,
			TopK:             callTopK(providerCfg, model.ModelCfg.TopK),
			FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
			PresencePenalty:  model.ModelCfg.PresencePenalty,
			NonInteractive:   true,
			OnAuthRefresh:    c.makeAuthRefreshCallback(providerCfg),
		})
	}
	result, err := run()
	// Notify only if still unauthorized after retry. AWS SSO is handled
	// transparently inside OnAuthRefresh, so it needs no post-run notice.
	if err != nil && isUnauthorized(err) && c.notify != nil && model.ModelCfg.Provider == hyper.Name {
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
	return result.Response.Content.Text()
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
