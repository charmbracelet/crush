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
// subagents get their own large model so it bills them as agent-initiated;
// elsewhere subLarge is large. The small model is always built that way.
type coordinatorModels struct {
	large, subLarge, small Model
	// managedVersion is the managed-credential version these models were
	// built under. It is published with them, so a failed build never
	// marks a version built; a mismatch means they are stale.
	managedVersion uint64
}

// rebuildModels rebuilds every shared model from the live config and
// publishes them together. Rebuilds are serialized, so one that read an older
// config can never publish over one that read a newer credential.
func (c *coordinator) rebuildModels(ctx context.Context) error {
	c.rebuildMu.Lock()
	defer c.rebuildMu.Unlock()
	return c.rebuildModelsLocked(ctx)
}

// rebuildModelsLocked is rebuildModels for a caller holding rebuildMu.
func (c *coordinator) rebuildModelsLocked(ctx context.Context) error {
	// Read before building: a push that lands mid-build leaves the
	// recorded version behind, so the next check rebuilds again.
	managedVersion := c.cfg.ManagedCredentials().Version()

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
	c.models.Set(coordinatorModels{large: large, subLarge: subLarge, small: small, managedVersion: managedVersion})

	// Override models rebuild lazily on their next use.
	c.modelsGen.Add(1)
	return nil
}

// overrideModelKey identifies a per-dispatch override selection.
type overrideModelKey struct {
	provider string
	model    string
	think    bool
	effort   string
}

// overrideModel is the shared build of one override selection. It is built
// as a subagent model, so Copilot bills it as agent-initiated.
type overrideModel struct {
	sel config.SelectedModel

	mu    sync.Mutex
	model Model
	gen   uint64 // c.modelsGen the build was made under.
}

// current returns the build, rebuilding it first if the shared models were
// rebuilt since. A failed rebuild keeps the old build rather than moving a
// running subagent onto the session default.
func (o *overrideModel) current(c *coordinator) Model {
	model, _ := o.refresh(c)
	return model
}

func (o *overrideModel) refresh(c *coordinator) (Model, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	gen := c.modelsGen.Load()
	if o.gen == gen {
		return o.model, nil
	}
	model, err := c.buildModelFromSelection(context.Background(), o.sel, true)
	if err != nil {
		if o.model.Model != nil {
			slog.Warn("Failed to refresh an overridden subagent model",
				"model", o.sel.Model, "provider", o.sel.Provider, "error", err)
		}
		return o.model, err
	}
	o.model, o.gen = model, gen
	return o.model, nil
}

// overrideModel returns the shared, built model for a selection. A selection
// that fails its first build is not cached, so a later dispatch tries afresh.
func (c *coordinator) overrideModel(sel config.SelectedModel) (*overrideModel, error) {
	key := overrideModelKey{provider: sel.Provider, model: sel.Model, think: sel.Think, effort: sel.ReasoningEffort}
	c.overrideModelsMu.Lock()
	if c.overrideModels == nil {
		c.overrideModels = make(map[overrideModelKey]*overrideModel)
	}
	entry, ok := c.overrideModels[key]
	if !ok {
		entry = &overrideModel{sel: sel, gen: ^uint64(0)}
		c.overrideModels[key] = entry
	}
	c.overrideModelsMu.Unlock()

	if model, err := entry.refresh(c); model.Model == nil {
		c.overrideModelsMu.Lock()
		if c.overrideModels[key] == entry {
			delete(c.overrideModels, key)
		}
		c.overrideModelsMu.Unlock()
		return nil, err
	}
	return entry, nil
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
	return a.currentModelOf(started).Model
}

// currentModelOf is currentBuildOf returning the whole build, including the
// managed credential it presents.
func (a *sessionAgent) currentModelOf(started Model) Model {
	m := a.models().Large
	if m.ModelCfg.Provider != started.ModelCfg.Provider || m.ModelCfg.Model != started.ModelCfg.Model {
		return started
	}
	return m
}

// attemptModel selects the build an attempt runs on and records it on the
// call's managed state, so a 401 is matched to the credential it sent.
func (a *sessionAgent) attemptModel(ctx context.Context, started Model) fantasy.LanguageModel {
	m := a.currentModelOf(started)
	managedCallFrom(ctx).record(m)
	return m.Model
}
