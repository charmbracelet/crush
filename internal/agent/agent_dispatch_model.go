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

// parentModel returns the model of the agent that owns a dispatch call. A
// model override that names only a model inherits this provider.
func (c *coordinator) parentModel(parentAgentID string) Model {
	c.agentMu.RLock()
	defer c.agentMu.RUnlock()
	if agent, ok := c.agents[parentAgentID]; ok {
		return agent.Model()
	}
	if c.mainAgent != nil {
		return c.mainAgent.Model()
	}
	return Model{}
}

// resolveModelOverride turns the dispatch tool's model-override params into
// the model selection a sub-agent should run on. ok is false when no
// override was requested. A non-empty refusal means one was requested but
// cannot be served; it goes back to the model so the call can be retried.
func (c *coordinator) resolveModelOverride(params AgentDispatchParams, parentModel Model) (sel config.SelectedModel, ok bool, refusal string) {
	provider := strings.TrimSpace(params.Provider)
	model := strings.TrimSpace(params.Model)
	level := strings.TrimSpace(params.ReasoningLevel)
	think := params.Think
	if provider == "" && model == "" && !think && level == "" {
		return config.SelectedModel{}, false, ""
	}
	if provider != "" && model == "" {
		return config.SelectedModel{}, false, "provider is given without model; pass model as well"
	}
	if model == "" {
		// Only reasoning settings were given: they apply to the sub-agent's
		// default model, keeping its other configured settings.
		sub := c.subAgentModels().Large
		if sub.Model == nil {
			return config.SelectedModel{}, false, "no sub-agent model is configured"
		}
		sel = sub.ModelCfg
	} else {
		if provider == "" {
			if parentModel.Model == nil {
				return config.SelectedModel{}, false, "the parent agent has no model to take a provider from"
			}
			provider = parentModel.ModelCfg.Provider
		}
		// A fresh selection: everything but the override itself is the
		// model's own defaults.
		sel = config.SelectedModel{
			Provider: provider,
			Model:    model,
		}
	}
	if think {
		sel.Think = true
	}
	if level != "" {
		sel.ReasoningEffort = level
	}
	return sel, true, c.validateModelOverride(sel, level, think)
}

// validateModelOverride refuses anything the dispatch tool's catalog would not list.
func (c *coordinator) validateModelOverride(sel config.SelectedModel, level string, think bool) string {
	cfg := c.cfg.Config()
	provider, ok := cfg.Providers.Get(sel.Provider)
	if !ok || provider.Disable {
		ids := make([]string, 0)
		for _, p := range usableProviders(cfg) {
			ids = append(ids, p.ID)
		}
		return fmt.Sprintf("provider %q is not available; use one of: %s", sel.Provider, strings.Join(ids, ", "))
	}
	models := provider.UsableModels()
	idx := slices.IndexFunc(models, func(m catwalk.Model) bool { return m.ID == sel.Model })
	if idx < 0 {
		return fmt.Sprintf("model %q is not available from provider %q; use one of: %s", sel.Model, sel.Provider, modelIDList(models))
	}
	if level != "" {
		if err := cfg.ValidateReasoningEffort(sel.Provider, sel.Model, level); err != nil {
			return err.Error()
		}
	}
	if think && !models[idx].CanReason() {
		return fmt.Sprintf("model %q does not support thinking", sel.Model)
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
		ids = append(ids, m.ID)
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

// taskAgentConfig returns the task agent configuration for an access level,
// mirroring the variants taskAgents builds.
func (c *coordinator) taskAgentConfig(access string) (config.Agent, bool) {
	if access == accessWrite {
		return writeAgentConfig(c.cfg.Config()), true
	}
	return c.cfg.Config().Agents[config.AgentTask], false
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

// overrideTaskAgent builds a dedicated task sub-agent variant running on an
// overridden model selection. The model function serves the selection's
// current build, so a model refresh reaches a running sub-agent on its next
// turn.
func (c *coordinator) overrideTaskAgent(ctx context.Context, access string, sel config.SelectedModel) (SessionAgent, string) {
	entry := c.overrideModelEntry(sel)
	if err := entry.build(ctx, c); err != nil {
		return nil, "failed to set up the requested model: " + err.Error()
	}
	agentCfg, canWrite := c.taskAgentConfig(access)
	// A group of its own: this build joins no one, and is fully set up
	// before the dispatch registers the sub-agent.
	var ready errgroup.Group
	agent, err := c.buildTaskAgent(ctx, &ready, agentCfg, canWrite, func() ModelSet {
		return ModelSet{Large: entry.current(), Small: c.subAgentModels().Small}
	})
	if err != nil {
		return nil, "failed to start a sub-agent on the requested model: " + err.Error()
	}
	if err := ready.Wait(); err != nil {
		return nil, "failed to start a sub-agent on the requested model: " + err.Error()
	}
	return agent, ""
}
