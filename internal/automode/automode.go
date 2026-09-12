package automode

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/tidwall/gjson"
)

// TranscriptProvider returns a reasoning-blind excerpt of the recent
// session conversation (user messages and tool calls only). It is
// satisfied by the agent layer's shared transcript builder.
type TranscriptProvider func(ctx context.Context, sessionID string) string

// Options configures the native auto mode.
type Options struct {
	// ModelResolver builds the classifier language model on demand.
	// When nil (or when it reports no model), classification runs in
	// rules-only mode: static allow/deny, ambiguities escalate.
	ModelResolver LanguageModelResolver
	// Transcript supplies the reasoning-blind conversation excerpt.
	Transcript TranscriptProvider
	// MaxTokens caps the classifier completion length. 0 uses the
	// default (1024).
	MaxTokens int64
	// Timeout bounds each PrePermission evaluation, including both LLM
	// stages. 0 uses the default (60s). This bounds how long the
	// permission service's request mutex is held.
	Timeout time.Duration
	// FailOpen allows on classifier failure instead of escalating.
	FailOpen bool
	// Environment prose injected into the classifier prompts.
	Environment []string
	// PromptStage1File / PromptStage2File override the built-in prompt
	// templates.
	PromptStage1File string
	PromptStage2File string
	// TranscriptMaxChars caps the transcript excerpt passed to the
	// classifier. 0 uses the default (24000).
	TranscriptMaxChars int
	// MaxConsecutiveDenials / MaxTotalDenials pause classification when
	// exceeded, escalating to the host prompt. 0 uses the defaults (3/20).
	MaxConsecutiveDenials int
	MaxTotalDenials       int
}

// Defaults fills in zero-value fields with the tested defaults.
func (o *Options) Defaults() {
	if o.MaxTokens <= 0 {
		o.MaxTokens = 1024
	}
	if o.Timeout <= 0 {
		o.Timeout = 60 * time.Second
	}
	if o.MaxConsecutiveDenials <= 0 {
		o.MaxConsecutiveDenials = 3
	}
	if o.MaxTotalDenials <= 0 {
		o.MaxTotalDenials = 20
	}
}

// AutoMode implements permission.PermissionHooks as a native auto mode:
// it runs before the permission prompt and grants, denies, or defers to
// the prompt based on static rules and an optional LLM classifier.
type AutoMode struct {
	opts         Options
	rules        *Rules
	quotas       *quotas
	classifyOpts ClassifierConfig
}

// New creates a native auto mode from the given options.
func New(opts Options) *AutoMode {
	opts.Defaults()
	prompts := LoadPromptSet(opts.PromptStage1File, opts.PromptStage2File)
	return &AutoMode{
		opts:   opts,
		rules:  DefaultRules(),
		quotas: newQuotas(),
		classifyOpts: ClassifierConfig{
			Prompts:            prompts,
			Environment:        opts.Environment,
			TranscriptMaxChars: opts.TranscriptMaxChars,
			FailOpen:           opts.FailOpen,
		},
	}
}

// generateFor builds the GenerateFunc for this evaluation, returning nil
// when no model is configured (rules-only mode).
func (a *AutoMode) generateFor(ctx context.Context) GenerateFunc {
	if a.opts.ModelResolver == nil {
		return nil
	}
	model, err := a.opts.ModelResolver(ctx)
	if err != nil || model == nil {
		if err != nil {
			slog.Warn("Automode classifier model unavailable, running rules-only", "error", err)
		}
		return nil
	}
	return languageModelGenerate(model, a.opts.MaxTokens)
}

// PrePermission implements permission.PermissionHooks. It runs only when
// a request is about to prompt the user.
func (a *AutoMode) PrePermission(ctx context.Context, req permission.PermissionRequest) permission.PreHookResult {
	toolName := strings.ToLower(req.ToolName)
	command, filePath := extractCallArgs(req.Params)

	// Quota pause: after too many denials, defer to the host prompt so a
	// runaway agent cannot act without human oversight.
	if a.quotas.paused(req.SessionID, a.opts.MaxConsecutiveDenials, a.opts.MaxTotalDenials) {
		q := a.quotas.get(req.SessionID)
		return permission.PreHookResult{
			Decision: permission.HookDecisionNone,
			Reason:   pausedReason(q, a.opts.MaxConsecutiveDenials, a.opts.MaxTotalDenials),
		}
	}

	// Bound the evaluation (both LLM stages) so the permission service's
	// request mutex is not held indefinitely.
	timeout := a.opts.Timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Lazily resolve the model; without one the classifier is
	// rules-only and ambiguities escalate.
	var gen GenerateFunc
	if a.opts.ModelResolver != nil {
		gen = a.generateFor(ctx)
	}
	cl := &classifier{generate: gen, config: a.classifyOpts, rules: a.rules}

	verdict, reason := cl.classify(ctx, toolName, command, filePath, req.Path, a.transcript(ctx, req.SessionID))

	switch verdict {
	case VerdictAllow:
		a.quotas.recordAllow(req.SessionID)
		return permission.PreHookResult{Decision: permission.HookDecisionAllow}
	case VerdictDeny:
		q := a.quotas.recordDenial(req.SessionID)
		if a.quotas.paused(req.SessionID, a.opts.MaxConsecutiveDenials, a.opts.MaxTotalDenials) {
			return permission.PreHookResult{
				Decision: permission.HookDecisionNone,
				Reason:   pausedReason(q, a.opts.MaxConsecutiveDenials, a.opts.MaxTotalDenials) + ": " + reason,
			}
		}
		return permission.PreHookResult{
			Decision: permission.HookDecisionDeny,
			Reason:   denyReason(q, a.opts.MaxConsecutiveDenials, a.opts.MaxTotalDenials, reason),
		}
	default:
		return permission.PreHookResult{Decision: permission.HookDecisionNone, Reason: reason}
	}
}

// PermissionDenied implements permission.PermissionHooks. The native
// auto mode records denials in PrePermission itself; this is a no-op.
func (a *AutoMode) PermissionDenied(context.Context, permission.PermissionRequest) {}

func (a *AutoMode) transcript(ctx context.Context, sessionID string) string {
	if a.opts.Transcript == nil {
		return ""
	}
	return a.opts.Transcript(ctx, sessionID)
}

// extractCallArgs pulls the shell command and file path out of a
// permission request's params JSON.
func extractCallArgs(params any) (command, filePath string) {
	if params == nil {
		return "", ""
	}
	data, err := json.Marshal(params)
	if err != nil {
		return "", ""
	}
	if c := gjson.Get(string(data), "command"); c.Exists() {
		command = c.String()
	}
	if fp := gjson.Get(string(data), "file_path"); fp.Exists() {
		filePath = fp.String()
	}
	return command, filePath
}

// denyReason formats a denial with quota counters so the model can see
// how close the session is to pausing.
func denyReason(q quotaState, maxConsecutive, maxTotal int, reason string) string {
	return "[auto-mode] Blocked (" +
		strconv.Itoa(q.consecutiveDenials) + "/" + strconv.Itoa(maxConsecutive) + " consecutive, " +
		strconv.Itoa(q.totalDenials) + "/" + strconv.Itoa(maxTotal) + " total): " + reason
}

// pausedReason explains that auto mode has paused and the prompt proceeds.
func pausedReason(q quotaState, maxConsecutive, maxTotal int) string {
	return "[auto-mode] PAUSED after " +
		strconv.Itoa(q.consecutiveDenials) + " consecutive / " +
		strconv.Itoa(q.totalDenials) + " total denials"
}
