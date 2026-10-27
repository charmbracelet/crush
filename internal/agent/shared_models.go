package agent

import (
	"context"
	"log/slog"
	"sync"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
)

// ModelSet is the large and small model an agent sends requests through.
type ModelSet struct {
	Large, Small Model
}

// coordinatorModels is every model the coordinator's agents share. On Copilot,
// sub-agents get their own large model so it bills them as agent-initiated;
// elsewhere subLarge is large. The small model is always built that way.
type coordinatorModels struct {
	large, subLarge, small Model
}

// rebuildModels rebuilds every shared model from the live config and
// publishes them together. Rebuilds are serialized, so one that read an older
// config can never publish over one that read a newer credential.
func (c *coordinator) rebuildModels(ctx context.Context) error {
	c.rebuildMu.Lock()
	defer c.rebuildMu.Unlock()

	large, err := c.buildModel(ctx, config.SelectedModelTypeLarge, false)
	if err != nil {
		return err
	}
	subLarge := large
	if large.ModelCfg.Provider == string(catwalk.InferenceProviderCopilot) {
		subLarge, err = c.buildModel(ctx, config.SelectedModelTypeLarge, true)
		if err != nil {
			return err
		}
	}
	small, err := c.buildModel(ctx, config.SelectedModelTypeSmall, true)
	if err != nil {
		return err
	}
	c.models.Set(coordinatorModels{large: large, subLarge: subLarge, small: small})

	// Refresh the override models from the same live config, so a sub-agent
	// running on an overridden model picks up credential and config changes
	// the same way the shared agents do. A failed refresh keeps the previous
	// build: the alternative, dropping the override, would silently move a
	// running sub-agent to the session default.
	c.overrideModelsMu.Lock()
	for _, entry := range c.overrideModels {
		entry.refresh(ctx, c)
	}
	c.overrideModelsMu.Unlock()
	return nil
}

// overrideModelKey identifies a per-dispatch override selection.
type overrideModelKey struct {
	provider string
	model    string
	think    bool
	effort   string
}

// overrideModel is one per-dispatch model selection together with its current
// build. It is always built as a sub-agent model, so Copilot in particular
// bills it as agent-initiated like any other sub-agent. The build is guarded
// so rebuildModels can swap it under a running dispatch, and the model
// function an overridden agent serves hands out current() on every turn
// rather than closing over one build.
type overrideModel struct {
	sel config.SelectedModel

	mu    sync.Mutex
	model Model
}

// current returns the latest build of the selection. Before the first build
// it is the zero Model; the dispatch that requested the override always
// builds before the agent exists, so agents only ever see a real one.
func (o *overrideModel) current() Model {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.model
}

// build produces the first build of the selection. The dispatch that
// requested the override is refused when this fails.
func (o *overrideModel) build(ctx context.Context, c *coordinator) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.model.Model != nil {
		return nil
	}
	model, err := c.buildModelFromSelection(ctx, o.sel, true)
	if err != nil {
		return err
	}
	o.model = model
	return nil
}

// refresh rebuilds the selection from the live config, keeping the previous
// build when the rebuild fails. Building and refreshing both hold o.mu and
// rebuildModels holds c.rebuildMu across every refresh, so a build can never
// publish a pre-rebuild model over a post-rebuild one.
func (o *overrideModel) refresh(ctx context.Context, c *coordinator) {
	o.mu.Lock()
	defer o.mu.Unlock()
	model, err := c.buildModelFromSelection(ctx, o.sel, true)
	if err != nil {
		slog.Warn(
			"Failed to refresh an overridden sub-agent model",
			"model", o.sel.Model,
			"provider", o.sel.Provider,
			"error", err,
		)
		return
	}
	o.model = model
}

// overrideModelEntry returns the shared build of a selection, creating it on
// first use, so dispatches on the same selection share one model.
func (c *coordinator) overrideModelEntry(sel config.SelectedModel) *overrideModel {
	key := overrideModelKey{
		provider: sel.Provider,
		model:    sel.Model,
		think:    sel.Think,
		effort:   sel.ReasoningEffort,
	}
	c.overrideModelsMu.Lock()
	defer c.overrideModelsMu.Unlock()
	if c.overrideModels == nil {
		c.overrideModels = make(map[overrideModelKey]*overrideModel)
	}
	entry, ok := c.overrideModels[key]
	if !ok {
		entry = &overrideModel{sel: sel}
		c.overrideModels[key] = entry
	}
	return entry
}

func (c *coordinator) mainModels() ModelSet {
	m := c.models.Get()
	return ModelSet{Large: m.large, Small: m.small}
}

func (c *coordinator) subAgentModels() ModelSet {
	m := c.models.Get()
	return ModelSet{Large: m.subLarge, Small: m.small}
}

// smallOnlyModels serves agents that use the small model for everything.
func (c *coordinator) smallOnlyModels() ModelSet {
	m := c.models.Get()
	return ModelSet{Large: m.small, Small: m.small}
}

// currentBuildOf returns the latest build of the model a turn started with, so
// a retry gets a refreshed credential but a model switch waits for the next
// turn.
func (a *sessionAgent) currentBuildOf(started Model) fantasy.LanguageModel {
	m := a.models().Large
	if m.ModelCfg.Provider != started.ModelCfg.Provider || m.ModelCfg.Model != started.ModelCfg.Model {
		return started.Model
	}
	return m.Model
}
