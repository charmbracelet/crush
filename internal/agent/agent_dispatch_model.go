package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/catwalk/v3/pkg/catwalk"
	"golang.org/x/sync/errgroup"

	"github.com/charmbracelet/crush/internal/config"
)

// mainModel returns the model the main agent runs on, or the zero Model
// before one is set.
func (c *coordinator) mainModel() Model {
	if agent, _ := c.activeAgent(); agent != nil {
		return agent.Model()
	}
	return Model{}
}

// resolveModelOverride turns the dispatch tool's model-override params into
// the model selection a subagent should run on. ok is false when no
// override was requested. A non-empty refusal means one was requested but
// cannot be served; it goes back to the model so the call can be retried.
func (c *coordinator) resolveModelOverride(params AgentDispatchParams, parentModel Model) (sel config.SelectedModel, ok bool, refusal string) {
	provider := strings.TrimSpace(params.Provider)
	model := strings.TrimSpace(params.Model)
	reasoning := strings.TrimSpace(params.Reasoning)
	switch {
	case provider == "" && model == "" && reasoning == "":
		return config.SelectedModel{}, false, ""
	case provider != "" && model == "":
		return config.SelectedModel{}, false, "provider is given without model; pass model as well"
	case model == "":
		// Reasoning alone rides the subagent's default selection.
		sub := c.subAgentModels().Large
		if sub.Model == nil {
			return config.SelectedModel{}, false, "no subagent model is configured"
		}
		sel = sub.ModelCfg
	default:
		if provider == "" {
			if parentModel.Model == nil {
				return config.SelectedModel{}, false, "the parent agent has no model to take a provider from"
			}
			provider = parentModel.ModelCfg.Provider
		}
		// A fresh selection: everything else is the model's own default.
		sel = config.SelectedModel{Provider: provider, Model: model}
	}
	catwalkModel, refusal := c.availableModel(sel)
	if refusal == "" {
		refusal = applyReasoning(&sel, catwalkModel, reasoning)
	}
	return sel, true, refusal
}

// availableModel finds a selection's model among what the catalog would list.
func (c *coordinator) availableModel(sel config.SelectedModel) (catwalk.Model, string) {
	cfg := c.cfg.Config()
	provider, ok := cfg.Providers.Get(sel.Provider)
	if !ok || provider.Disable {
		ids := make([]string, 0)
		for _, p := range usableProviders(cfg) {
			ids = append(ids, p.ID)
		}
		return catwalk.Model{}, fmt.Sprintf("provider %q is not available; use one of: %s", sel.Provider, strings.Join(ids, ", "))
	}
	models := provider.UsableModels()
	idx := slices.IndexFunc(models, func(m catwalk.Model) bool { return m.ID == sel.Model })
	if idx < 0 {
		return catwalk.Model{}, fmt.Sprintf("model %q is not available from provider %q; use one of: %s", sel.Model, sel.Provider, modelIDList(models))
	}
	return models[idx], ""
}

// reasoningOptions lists the values a model takes for reasoning: its effort
// levels, or on and off for a model that can only toggle thinking.
func reasoningOptions(m catwalk.Model) []string {
	if levels := m.ReasoningEffortLevels(); len(levels) > 0 {
		return levels
	}
	if m.Reasoning.Thinking == catwalk.ThinkingToggleable {
		return []string{"on", "off"}
	}
	return nil
}

// applyReasoning sets the selection's effort level or thinking toggle,
// whichever the model reasons with.
func applyReasoning(sel *config.SelectedModel, m catwalk.Model, reasoning string) string {
	if reasoning == "" {
		return ""
	}
	options := reasoningOptions(m)
	if !slices.Contains(options, reasoning) {
		if len(options) == 0 {
			return fmt.Sprintf("model %q does not support reasoning", m.ID)
		}
		return fmt.Sprintf("model %q takes reasoning %s", m.ID, strings.Join(options, ", "))
	}
	if len(m.ReasoningEffortLevels()) > 0 {
		sel.ReasoningEffort = reasoning
	} else {
		sel.Think = reasoning == "on"
	}
	return ""
}

// catalogModelsPerProvider keeps one huge catalog from crowding out the prompt.
const catalogModelsPerProvider = 25

// usableProviders lists enabled providers by ID, so the catalog text is stable across turns.
func usableProviders(cfg *config.Config) []config.ProviderConfig {
	providers := cfg.EnabledProviders()
	slices.SortFunc(providers, func(a, b config.ProviderConfig) int { return strings.Compare(a.ID, b.ID) })
	return providers
}

func modelIDList(models []catwalk.Model) string {
	listed := models[:min(len(models), catalogModelsPerProvider)]
	ids := make([]string, 0, len(listed))
	for _, m := range listed {
		if options := reasoningOptions(m); len(options) > 0 {
			ids = append(ids, m.ID+" ["+strings.Join(options, "|")+"]")
		} else {
			ids = append(ids, m.ID)
		}
	}
	list := strings.Join(ids, ", ")
	if extra := len(models) - len(listed); extra > 0 {
		list += fmt.Sprintf(", and %d more", extra)
	}
	return list
}

// dispatchModelCatalog is appended to the dispatch tool's description; empty when nothing is usable.
func dispatchModelCatalog(cfg *config.Config, parent Model) string {
	var b strings.Builder
	for _, p := range usableProviders(cfg) {
		if models := p.UsableModels(); len(models) > 0 {
			fmt.Fprintf(&b, "\n- `%s`: %s", p.ID, modelIDList(models))
		}
	}
	if b.Len() == 0 {
		return ""
	}
	header := "\n\nModels available to `model`, by `provider`"
	if parent.Model != nil {
		header += fmt.Sprintf(" (you are running on `%s`)", modelDisplayName(parent))
	}
	return header + ":\n" + b.String()
}

// modelDisplayName names a model as provider/model.
func modelDisplayName(m Model) string {
	return m.ModelCfg.Provider + "/" + m.ModelCfg.Model
}

// overrideDispatchAgent resolves a dispatch's model override and, when one
// was requested, builds the SessionAgent it should run on. Without an
// override the caller keeps the shared agent taskAgents built; with one,
// the entry keeps the dedicated agent so reopens run on the same model.
func (c *coordinator) overrideDispatchAgent(ctx context.Context, access string, params AgentDispatchParams, parentModel Model) (SessionAgent, config.SelectedModel, bool, string) {
	sel, ok, refusal := c.resolveModelOverride(params, parentModel)
	if refusal != "" || !ok {
		return nil, config.SelectedModel{}, false, refusal
	}
	agent, refusal := c.overrideTaskAgent(ctx, access, sel)
	if refusal != "" {
		return nil, sel, true, refusal
	}
	return agent, sel, true, ""
}

// overrideTaskAgent builds a dedicated task subagent variant running on an
// overridden model selection. The model function serves the selection's
// current build, so a model refresh reaches a running subagent on its next
// turn.
func (c *coordinator) overrideTaskAgent(ctx context.Context, access string, sel config.SelectedModel) (SessionAgent, string) {
	entry, err := c.overrideModel(sel)
	if err != nil {
		return nil, "failed to set up the requested model: " + err.Error()
	}
	agentCfg, canWrite := c.taskAgentConfig(access)
	// A group of its own: this build joins no one, and is fully set up
	// before the dispatch registers the subagent.
	var ready errgroup.Group
	agent, err := c.buildTaskAgent(ctx, &ready, agentCfg, canWrite, func() ModelSet {
		return ModelSet{Large: entry.current(c), Small: c.subAgentModels().Small}
	})
	if err != nil {
		return nil, "failed to start a subagent on the requested model: " + err.Error()
	}
	if err := ready.Wait(); err != nil {
		return nil, "failed to start a subagent on the requested model: " + err.Error()
	}
	return agent, ""
}
