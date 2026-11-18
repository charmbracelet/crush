package agent

import (
	"context"
	"encoding/json"
	"fmt"
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

	// Override models rebuild lazily on their next use.
	c.modelsGen.Add(1)
	return nil
}

// buildKey identifies one shared build: the whole selection, settings
// included, and whether it bills as a subagent.
type buildKey struct {
	sel      string
	subAgent bool
}

func newBuildKey(sel config.SelectedModel, subAgent bool) buildKey {
	// encoding/json sorts map keys, so equal selections give equal keys.
	b, _ := json.Marshal(sel)
	return buildKey{sel: string(b), subAgent: subAgent}
}

// sharedBuild is the build of one selection that sessions and override
// subagents share. It rebuilds lazily when the shared models do.
type sharedBuild struct {
	sel      config.SelectedModel
	subAgent bool

	mu    sync.Mutex
	model Model
	gen   uint64 // c.modelsGen the build was made under.
}

// current returns the build, refreshed if the shared models were rebuilt.
// A failed rebuild keeps the old build rather than switching models.
func (b *sharedBuild) current(c *coordinator) Model {
	model, _ := b.refresh(c)
	model.latest = func() fantasy.LanguageModel { return b.current(c).Model }
	return model
}

func (b *sharedBuild) refresh(c *coordinator) (Model, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	gen := c.modelsGen.Load()
	if b.gen == gen {
		return b.model, nil
	}
	model, err := c.buildModelFromSelection(context.Background(), b.sel, b.subAgent)
	if err != nil {
		if b.model.Model != nil {
			slog.Warn("Failed to refresh a model; keeping the previous build",
				"model", b.sel.Model, "provider", b.sel.Provider, "error", err)
		}
		return b.model, err
	}
	b.model, b.gen = model, gen
	return b.model, nil
}

// sharedBuild returns the built model for a selection. A selection that fails
// its first build is not cached, so a later call tries afresh.
func (c *coordinator) sharedBuild(sel config.SelectedModel, subAgent bool) (*sharedBuild, error) {
	key := newBuildKey(sel, subAgent)
	c.buildsMu.Lock()
	if c.builds == nil {
		c.builds = make(map[buildKey]*sharedBuild)
	}
	entry, ok := c.builds[key]
	if !ok {
		entry = &sharedBuild{sel: sel, subAgent: subAgent, gen: ^uint64(0)}
		c.builds[key] = entry
	}
	c.buildsMu.Unlock()

	if model, err := entry.refresh(c); model.Model == nil {
		c.buildsMu.Lock()
		if c.builds[key] == entry {
			delete(c.builds, key)
		}
		c.buildsMu.Unlock()
		return nil, err
	}
	return entry, nil
}

func (c *coordinator) mainModels() ModelSet {
	m := c.models.Get()
	return ModelSet{Large: m.large, Small: m.small}
}

// modelForSession returns the large model a session's turns run on, pinning
// the current default on first use so a later pick elsewhere can't move it.
func (c *coordinator) modelForSession(sessionID string) (Model, error) {
	c.sessionModelsMu.Lock()
	sel, ok := c.sessionModels[sessionID]
	if !ok {
		def, configured := c.cfg.Config().Models[config.SelectedModelTypeLarge]
		if !configured {
			c.sessionModelsMu.Unlock()
			return Model{}, fmt.Errorf("%s %w", config.SelectedModelTypeLarge, errModelNotSelected)
		}
		sel = def
		c.setSessionModelLocked(sessionID, sel)
	}
	c.sessionModelsMu.Unlock()

	build, err := c.sharedBuild(sel, false)
	if err != nil {
		return Model{}, err
	}
	return build.current(c), nil
}

// SessionModel implements Coordinator. Unlike modelForSession it never pins,
// so a session that has not run yet follows the default.
func (c *coordinator) SessionModel(sessionID string) Model {
	c.sessionModelsMu.Lock()
	sel, ok := c.sessionModels[sessionID]
	c.sessionModelsMu.Unlock()
	if !ok {
		return c.mainModels().Large
	}
	build, err := c.sharedBuild(sel, false)
	if err != nil {
		return Model{}
	}
	return build.current(c)
}

// SetSessionModel implements Coordinator. The selection must build.
func (c *coordinator) SetSessionModel(_ context.Context, sessionID string, sel config.SelectedModel) error {
	if _, err := c.sharedBuild(sel, false); err != nil {
		return err
	}
	c.sessionModelsMu.Lock()
	c.setSessionModelLocked(sessionID, sel)
	c.sessionModelsMu.Unlock()
	return nil
}

func (c *coordinator) setSessionModelLocked(sessionID string, sel config.SelectedModel) {
	if c.sessionModels == nil {
		c.sessionModels = make(map[string]config.SelectedModel)
	}
	c.sessionModels[sessionID] = sel
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
	if started.latest != nil {
		return started.latest()
	}
	m := a.models().Large
	if m.ModelCfg.Provider != started.ModelCfg.Provider || m.ModelCfg.Model != started.ModelCfg.Model {
		return started.Model
	}
	return m.Model
}
