package skills

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// Manager owns per-workspace skill discovery state: the latest discovery
// snapshot, the full skill metadata (with Instructions) for the
// coordinator, and a pubsub broker for change events. There is exactly
// one Manager per workspace.
//
// Package-level helpers (GetLatestStates, SetLatestStates,
// PublishStates, SubscribeEvents) are preserved for callers that share a
// process with the TUI. To bridge a Manager to those globals, construct
// it with WithGlobalMirror. Only do this when the process hosts a single
// workspace (local mode or a client process); the backend server hosts
// multiple workspaces concurrently and must not enable mirroring.
type Manager struct {
	mu           sync.RWMutex
	allSkills    []*Skill
	activeSkills []*Skill
	states       []*SkillState

	// resolvedPaths are the expanded SkillsPaths used during discovery.
	// Stored so Catalog/ReadContent can label skills without
	// re-resolving.
	resolvedPaths []string
	workingDir    string

	broker       *pubsub.Broker[Event]
	globalMirror bool
}

// ManagerOption configures a Manager at construction time.
type ManagerOption func(*Manager)

// WithGlobalMirror causes the manager to forward SetLatestStates and
// PublishStates calls to the package-level cache and broker. Only safe
// when the process hosts at most one Manager (e.g. local mode or the
// client process).
func WithGlobalMirror() ManagerOption {
	return func(m *Manager) {
		m.globalMirror = true
	}
}

// WithResolvedPaths stores the expanded skills directory paths that
// were used during discovery. Catalog and ReadContent use these for
// source labelling.
func WithResolvedPaths(paths []string) ManagerOption {
	return func(m *Manager) {
		m.resolvedPaths = paths
	}
}

// WithWorkingDir stores the workspace working directory. Catalog and
// ReadContent use it to distinguish project skills from user skills.
func WithWorkingDir(dir string) ManagerOption {
	return func(m *Manager) {
		m.workingDir = dir
	}
}

// NewManager constructs a workspace-scoped Manager with the given
// pre-computed discovery results. The slices are stored as-is; callers
// should not mutate them afterwards.
func NewManager(allSkills, activeSkills []*Skill, states []*SkillState, opts ...ManagerOption) *Manager {
	m := &Manager{
		allSkills:    allSkills,
		activeSkills: activeSkills,
		states:       states,
		broker:       pubsub.NewBroker[Event](),
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.globalMirror {
		SetLatestStates(states)
	}
	return m
}

// AllSkills returns the deduplicated list of all discovered skills.
func (m *Manager) AllSkills() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.allSkills
}

// ActiveSkills returns the post-filter list of active skills (after
// removing disabled entries).
func (m *Manager) ActiveSkills() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeSkills
}

// ResolvedPaths returns the expanded skills directory paths stored at
// construction time.
func (m *Manager) ResolvedPaths() []string {
	return m.resolvedPaths
}

// WorkingDir returns the workspace working directory stored at
// construction time.
func (m *Manager) WorkingDir() string {
	return m.workingDir
}

// States returns a clone of the latest discovery state snapshot.
func (m *Manager) States() []*SkillState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneStates(m.states)
}

// SetLatestStates updates the manager's cached discovery snapshot.
func (m *Manager) SetLatestStates(states []*SkillState) {
	m.mu.Lock()
	m.states = cloneStates(states)
	m.mu.Unlock()
	if m.globalMirror {
		SetLatestStates(states)
	}
}

// PublishStates updates the manager's cached snapshot and publishes a
// discovery event to subscribers. Callers should not call
// SetLatestStates separately — PublishStates is the single mutation
// point, keeping Manager.States(), workspaceToProto, and (when
// WithGlobalMirror is set) skills.GetLatestStates consistent with what
// subscribers observe.
func (m *Manager) PublishStates(states []*SkillState) {
	m.mu.Lock()
	m.states = cloneStates(states)
	m.mu.Unlock()
	if m.globalMirror {
		SetLatestStates(states)
	}
	m.broker.Publish(pubsub.UpdatedEvent, Event{States: cloneStates(states)})
	if m.globalMirror {
		PublishStates(states)
	}
}

// SubscribeEvents returns a channel of discovery events for the
// manager's workspace.
func (m *Manager) SubscribeEvents(ctx context.Context) <-chan pubsub.Event[Event] {
	return m.broker.Subscribe(ctx)
}

// Shutdown releases broker resources.
func (m *Manager) Shutdown() {
	if m.broker != nil {
		m.broker.Shutdown()
	}
}

// Reload re-runs discovery from the given config, replaces all internal
// skill lists and states, and publishes the new states to subscribers.
// This is the mechanism for hot-reloading skills without restarting Crush.
func (m *Manager) Reload(cfg DiscoveryConfig) {
	allSkills, activeSkills, states := DiscoverFromConfig(cfg)
	m.mu.Lock()
	m.allSkills = allSkills
	m.activeSkills = activeSkills
	m.resolvedPaths = cfg.ResolvePaths()
	m.mu.Unlock()
	m.PublishStates(states)
}

// DiscoverFromConfig walks the embedded builtin FS and every path in
// cfg.Options.SkillsPaths (after home / env expansion), then dedups and
// filters by cfg.Options.DisabledSkills. It returns the three slices the
// rest of the system needs:
//
//   - allSkills:    deduplicated, pre-filter (includes disabled).
//   - activeSkills: post-filter (DisabledSkills removed).
//   - states:       per-file discovery outcome for diagnostics/UI.
func DiscoverFromConfig(cfg DiscoveryConfig) (allSkills, activeSkills []*Skill, states []*SkillState) {
	builtin, builtinStates := DiscoverBuiltinWithStates()
	discovered := append([]*Skill(nil), builtin...)

	var userStates []*SkillState
	userPaths := cfg.ResolvePaths()
	if len(userPaths) > 0 {
		var userSkills []*Skill
		userSkills, userStates = DiscoverWithStates(userPaths)
		discovered = append(discovered, userSkills...)
	}

	allSkills = Deduplicate(discovered)
	activeSkills = Filter(allSkills, cfg.DisabledSkills)

	allStates := append([]*SkillState(nil), builtinStates...)
	allStates = append(allStates, userStates...)
	allStates = DeduplicateStates(allStates)
	slices.SortStableFunc(allStates, func(a, b *SkillState) int {
		return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path))
	})
	return allSkills, activeSkills, allStates
}

// DiscoveryConfig contains the inputs DiscoverFromConfig needs. Using a
// dedicated struct (rather than importing internal/config) keeps the
// skills package's dependency graph small.
type DiscoveryConfig struct {
	SkillsPaths    []string
	DisabledSkills []string
	WorkingDir     string
	// Resolver expands $VAR-style references in paths. May be nil.
	Resolver func(string) (string, error)
}

// ResolvePaths expands home-directory and $VAR references in
// SkillsPaths. This is the canonical path-resolution logic used by
// DiscoverFromConfig; callers that need the resolved list (e.g. for
// Catalog labels) can call this directly.
func (c DiscoveryConfig) ResolvePaths() []string {
	if len(c.SkillsPaths) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.SkillsPaths))
	for _, pth := range c.SkillsPaths {
		expanded := home.Long(pth)
		if strings.HasPrefix(expanded, "$") && c.Resolver != nil {
			if resolved, err := c.Resolver(expanded); err == nil {
				expanded = resolved
			}
		}
		out = append(out, expanded)
	}
	return out
}
