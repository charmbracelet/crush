package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
)

//go:embed templates/router.md
var routerPrompt string

const (
	routerMaxPromptRunes    = 4000
	routerMaxPrevRunes      = 500
	routerContextGuardRatio = 0.8
)

var errRouteModelNotFound = errors.New("route model not found")

// TurnOptions are the per-call model options for a single turn.
type TurnOptions struct {
	MaxOutputTokens  int64
	ProviderOptions  fantasy.ProviderOptions
	Temperature      *float64
	TopP             *float64
	TopK             *int64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	OnAuthRefresh    func(context.Context, *fantasy.ProviderError) error
}

// RouteInput is what a router sees when choosing a model for a turn.
type RouteInput struct {
	SessionID     string
	Prompt        string
	PrevAssistant string
	// Activity summarizes the previous step of the current turn (what the
	// model said, which tools it called, what they returned). Empty for
	// the first step.
	Activity      string
	Attachments   int
	SessionTokens int64
}

// RouteResult is a router's decision for a turn.
type RouteResult struct {
	Router         string
	Route          string
	Model          Model
	Options        TurnOptions
	ModelProvider  func() fantasy.LanguageModel
	ClassifierCost float64
}

// ModelRouter picks the model for a turn.
type ModelRouter interface {
	Route(ctx context.Context, in RouteInput) (RouteResult, error)
}

// classifierRouter asks a classifier model which route a prompt belongs to.
type classifierRouter struct {
	c    *coordinator
	name string
	cfg  config.RouterConfig
}

var _ ModelRouter = (*classifierRouter)(nil)

// Route implements [ModelRouter]. Classification failures fall back to the
// default route; only a failure to build any route model is an error.
func (r *classifierRouter) Route(ctx context.Context, in RouteInput) (RouteResult, error) {
	choice, cost, err := r.classify(ctx, in)
	if err != nil {
		if ctx.Err() != nil {
			return RouteResult{}, ctx.Err()
		}
		slog.Warn("Router classifier failed, using default route", "router", r.name, "error", err)
	}
	route := r.selectRoute(choice, in.SessionTokens)

	m, err := r.c.buildModel(ctx, route.Model, false, errModelProviderNotConfigured, errRouteModelNotFound)
	if err != nil {
		def := r.cfg.DefaultRoute()
		if route.Name == def.Name {
			return RouteResult{}, fmt.Errorf("router %q: route %q: %w", r.name, route.Name, err)
		}
		slog.Warn("Route model unavailable, using default route", "router", r.name, "route", route.Name, "error", err)
		route = def
		if m, err = r.c.buildModel(ctx, route.Model, false, errModelProviderNotConfigured, errRouteModelNotFound); err != nil {
			return RouteResult{}, fmt.Errorf("router %q: default route %q: %w", r.name, route.Name, err)
		}
	}

	opts, err := r.c.turnOptions(ctx, m)
	if err != nil {
		return RouteResult{}, err
	}

	sel := route.Model
	return RouteResult{
		Router:  r.name,
		Route:   route.Name,
		Model:   m,
		Options: opts,
		ModelProvider: func() fantasy.LanguageModel {
			// Rebuild so credentials refreshed by OnAuthRefresh are used.
			fresh, err := r.c.buildModel(context.WithoutCancel(ctx), sel, false, errModelProviderNotConfigured, errRouteModelNotFound)
			if err != nil {
				return m.Model
			}
			return fresh.Model
		},
		ClassifierCost: cost,
	}, nil
}

// selectRoute resolves the classifier's answer to a route, applying the
// context guard: when the session is close to filling the chosen model's
// context window, the route with the largest window wins instead.
func (r *classifierRouter) selectRoute(choice string, sessionTokens int64) config.RouterRoute {
	route, ok := matchRoute(r.cfg.Routes, choice)
	if !ok {
		route = r.cfg.DefaultRoute()
	}
	cfg := r.c.cfg.Config()
	window := func(rt config.RouterRoute) int64 {
		if m := cfg.GetModel(rt.Model.Provider, rt.Model.Model); m != nil {
			return m.ContextWindow
		}
		return 0
	}
	return guardContext(r.cfg.Routes, route, sessionTokens, window)
}

func guardContext(routes []config.RouterRoute, route config.RouterRoute, sessionTokens int64, window func(config.RouterRoute) int64) config.RouterRoute {
	cw := window(route)
	if cw == 0 || float64(sessionTokens) < float64(cw)*routerContextGuardRatio {
		return route
	}
	best := route
	for _, rt := range routes {
		if window(rt) > window(best) {
			best = rt
		}
	}
	return best
}

// classify runs the classifier and returns its raw answer and cost.
func (r *classifierRouter) classify(ctx context.Context, in RouteInput) (string, float64, error) {
	if pc, ok := r.c.cfg.Config().Providers.Get(r.cfg.Classifier.Provider); ok && pc.Type == config.ProviderTypeTypeSafe {
		return r.classifyTypeSafe(ctx, pc, in)
	}
	classifier, err := r.c.buildModel(ctx, r.cfg.Classifier, true, errModelProviderNotConfigured, errors.New("classifier model not found"))
	if err != nil {
		return "", 0, err
	}
	providerCfg, _ := r.c.cfg.Config().Providers.Get(classifier.ModelCfg.Provider)

	cctx, cancel := context.WithTimeout(ctx, r.cfg.TimeoutDuration())
	defer cancel()

	tok := int64(32)
	if classifier.CatwalkCfg.CanReason {
		tok = classifier.CatwalkCfg.DefaultMaxTokens
	}
	agent := fantasy.NewAgent(
		classifier.Model,
		fantasy.WithSystemPrompt(buildRouterPrompt(r.cfg)+"\n /no_think"),
		fantasy.WithMaxOutputTokens(tok),
		fantasy.WithUserAgent(userAgent),
	)
	resp, err := agent.Generate(cctx, fantasy.AgentCall{
		Prompt:          buildRouterInput(in),
		Headers:         sessionHeaders(in.SessionID),
		ProviderOptions: getProviderOptions(classifier, providerCfg),
	})
	if err != nil {
		return "", 0, err
	}

	cost := 0.0
	if !classifier.FlatRate {
		mc := classifier.CatwalkCfg
		u := resp.TotalUsage
		cost = mc.CostPer1MInCached/1e6*float64(u.CacheCreationTokens) +
			mc.CostPer1MOutCached/1e6*float64(u.CacheReadTokens) +
			mc.CostPer1MIn/1e6*float64(u.InputTokens) +
			mc.CostPer1MOut/1e6*float64(u.OutputTokens)
	}
	return resp.Response.Content.Text(), cost, nil
}

func buildRouterPrompt(cfg config.RouterConfig) string {
	var routes strings.Builder
	for _, rt := range cfg.Routes {
		fmt.Fprintf(&routes, "- %s: %s\n", rt.Name, rt.Description)
	}
	instructions := ""
	if s := strings.TrimSpace(cfg.Instructions); s != "" {
		instructions = "\nAdditional instructions:\n" + s + "\n"
	}
	return strings.NewReplacer(
		"{{ROUTES}}", strings.TrimRight(routes.String(), "\n"),
		"{{INSTRUCTIONS}}", instructions,
	).Replace(routerPrompt)
}

func buildRouterInput(in RouteInput) string {
	var b strings.Builder
	if in.PrevAssistant != "" {
		b.WriteString("<previous_assistant_reply>\n")
		b.WriteString(lastRunes(in.PrevAssistant, routerMaxPrevRunes))
		b.WriteString("\n</previous_assistant_reply>\n\n")
	}
	b.WriteString("<request>\n")
	b.WriteString(firstRunes(in.Prompt, routerMaxPromptRunes))
	b.WriteString("\n</request>")
	if in.Activity != "" {
		b.WriteString("\n\n<work_so_far>\n")
		b.WriteString(in.Activity)
		b.WriteString("\n</work_so_far>\n\nChoose the route for the NEXT step of this work.")
	}
	if in.Attachments > 0 {
		fmt.Fprintf(&b, "\n\nThe request has %d attachment(s).", in.Attachments)
	}
	return b.String()
}

// matchRoute maps a classifier answer to a route: exact match first, then
// case-insensitive, then the first route name contained in the answer.
func matchRoute(routes []config.RouterRoute, answer string) (config.RouterRoute, bool) {
	answer = thinkTagRegex.ReplaceAllString(answer, "")
	answer = orphanThinkTagRegex.ReplaceAllString(answer, "")
	answer = strings.Trim(strings.TrimSpace(answer), "`'\".,:;*")
	if answer == "" {
		return config.RouterRoute{}, false
	}
	for _, rt := range routes {
		if rt.Name == answer {
			return rt, true
		}
	}
	lower := strings.ToLower(answer)
	for _, rt := range routes {
		if strings.ToLower(rt.Name) == lower {
			return rt, true
		}
	}
	// Prefer the longest contained name so "complex" beats "plex".
	var best config.RouterRoute
	found := false
	for _, rt := range routes {
		if strings.Contains(lower, strings.ToLower(rt.Name)) && len(rt.Name) > len(best.Name) {
			best, found = rt, true
		}
	}
	return best, found
}

// lastAssistantText returns the text of the most recent assistant message.
func lastAssistantText(msgs []message.Message) string {
	for _, msg := range slices.Backward(msgs) {
		if msg.Role == message.Assistant {
			if s := msg.Content().String(); s != "" {
				return s
			}
		}
	}
	return ""
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func lastRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n:])
}

// newRouter returns the router for the active router config, or nil.
func (c *coordinator) newRouter() ModelRouter {
	name, cfg, ok := c.cfg.Config().ActiveRouterConfig()
	if !ok {
		return nil
	}
	return &classifierRouter{c: c, name: name, cfg: cfg}
}

// turnOptions computes the call options for a model and refreshes an
// expired OAuth token for its provider.
func (c *coordinator) turnOptions(ctx context.Context, m Model) (TurnOptions, error) {
	providerCfg, ok := c.cfg.Config().Providers.Get(m.ModelCfg.Provider)
	if !ok {
		return TurnOptions{}, errModelProviderNotConfigured
	}
	if err := c.refreshTokenIfExpired(ctx, providerCfg); err != nil {
		slog.Error("Failed to refresh OAuth2 token. Proceeding with existing token.", "error", err)
	}
	maxTokens := m.CatwalkCfg.DefaultMaxTokens
	if m.ModelCfg.MaxTokens != 0 {
		maxTokens = m.ModelCfg.MaxTokens
	}
	opts, temp, topP, topK, freq, pres := mergeCallOptions(m, providerCfg)
	return TurnOptions{
		MaxOutputTokens:  maxTokens,
		ProviderOptions:  opts,
		Temperature:      temp,
		TopP:             topP,
		TopK:             callTopK(providerCfg, topK),
		FrequencyPenalty: freq,
		PresencePenalty:  pres,
		OnAuthRefresh:    c.makeAuthRefreshCallback(providerCfg),
	}, nil
}
