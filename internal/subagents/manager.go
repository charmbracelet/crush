package subagents

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/skills"
)

// Manager owns per-workspace subagent discovery state: the latest discovery
// snapshot, the full subagent metadata, and a pubsub broker for change events.
// There is exactly one Manager per workspace.
type Manager struct {
	mu              sync.RWMutex
	allSubagents    []*Subagent
	activeSubagents []*Subagent
	states          []*SubagentState

	broker *pubsub.Broker[Event]
}

// ManagerOption configures a Manager at construction time.
type ManagerOption func(*Manager)

// NewManager constructs a workspace-scoped Manager with the given
// pre-computed discovery results. The slices are stored as-is; callers
// should not mutate them afterwards.
func NewManager(all, active []*Subagent, states []*SubagentState, opts ...ManagerOption) *Manager {
	m := &Manager{
		allSubagents:    all,
		activeSubagents: active,
		states:          states,
		broker:          pubsub.NewBroker[Event](),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// AllSubagents returns a copy of the deduplicated list of all discovered
// subagents. The returned slice is safe for the caller to mutate.
func (m *Manager) AllSubagents() []*Subagent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneSubagents(m.allSubagents)
}

// ActiveSubagents returns a copy of the post-filter list of active subagents
// (after removing disabled entries). The returned slice is safe for the caller
// to mutate.
func (m *Manager) ActiveSubagents() []*Subagent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneSubagents(m.activeSubagents)
}

// States returns a clone of the latest discovery state snapshot.
func (m *Manager) States() []*SubagentState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneStates(m.states)
}

// SetLatestStates updates the manager's cached discovery snapshot.
func (m *Manager) SetLatestStates(states []*SubagentState) {
	m.mu.Lock()
	m.states = cloneStates(states)
	m.mu.Unlock()
}

// PublishStates updates the manager's cached snapshot and publishes a
// discovery event to subscribers.
func (m *Manager) PublishStates(states []*SubagentState) {
	m.mu.Lock()
	m.states = cloneStates(states)
	m.mu.Unlock()
	m.broker.Publish(pubsub.UpdatedEvent, Event{States: cloneStates(states)})
}

// SubscribeEvents returns a channel of discovery events for the
// manager's workspace.
func (m *Manager) SubscribeEvents(ctx context.Context) <-chan pubsub.Event[Event] {
	if m == nil || m.broker == nil {
		ch := make(chan pubsub.Event[Event])
		close(ch)
		return ch
	}
	return m.broker.Subscribe(ctx)
}

// Reload atomically replaces the manager's allSubagents, activeSubagents, and
// states with the provided slices, then publishes an UpdatedEvent to
// subscribers. It is a no-op when m is nil.
func (m *Manager) Reload(all, active []*Subagent, states []*SubagentState) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.allSubagents = cloneSubagents(all)
	m.activeSubagents = cloneSubagents(active)
	m.states = cloneStates(states)
	m.mu.Unlock()
	m.broker.Publish(pubsub.UpdatedEvent, Event{States: cloneStates(states)})
}

// Shutdown releases broker resources.
func (m *Manager) Shutdown() {
	if m.broker != nil {
		m.broker.Shutdown()
	}
}

// DiscoveryConfig contains the inputs DiscoverFromConfig needs.
type DiscoveryConfig struct {
	SubagentsPaths    []string
	DisabledSubagents []string
	// Resolver expands $VAR-style references in paths. May be nil.
	Resolver func(string) (string, error)
	// IsKnownModel validates that a model id (anything other than the
	// "large"/"small" aliases) resolves to a real provider model. May be nil
	// during discovery in contexts where the config is not yet loaded; in that
	// case model-id validation is skipped.
	IsKnownModel func(provider, model string) bool
	// IsKnownSkill validates that a `skills:` frontmatter reference resolves
	// to an active skill, so a broken reference surfaces in the Library UI
	// instead of being silently dropped at dispatch time. May be nil when no
	// skills context is available; in that case the check is skipped.
	IsKnownSkill func(name string) bool
}

// DiscoveryConfigFromStore adapts a config store (plus the workspace's skills
// manager, which may be nil) into the DiscoveryConfig DiscoverFromConfig
// consumes. Shared by startup discovery (cmd, backend) and Library reloads so
// the discovery inputs cannot drift between call sites.
func DiscoveryConfigFromStore(store *config.ConfigStore, skillsMgr *skills.Manager) DiscoveryConfig {
	opts := store.Config().Options
	var paths, disabled []string
	if opts != nil {
		paths = opts.SubagentsPaths
		disabled = opts.DisabledSubagents
	}
	var resolver func(string) (string, error)
	if r := store.Resolver(); r != nil {
		resolver = r.ResolveValue
	}
	return DiscoveryConfig{
		SubagentsPaths:    paths,
		DisabledSubagents: disabled,
		Resolver:          resolver,
		IsKnownModel:      store.Config().IsKnownModel,
		IsKnownSkill:      knownSkillFunc(skillsMgr),
	}
}

// knownSkillFunc returns a name-membership check over the manager's active
// skills at call time, or nil when no manager is available (skipping skill
// validation).
func knownSkillFunc(mgr *skills.Manager) func(name string) bool {
	if mgr == nil {
		return nil
	}
	active := mgr.ActiveSkills()
	names := make(map[string]bool, len(active))
	for _, s := range active {
		names[s.Name] = true
	}
	return func(name string) bool { return names[name] }
}

// ResolvePaths expands home-directory and $VAR references in SubagentsPaths.
func (c DiscoveryConfig) ResolvePaths() []string {
	if len(c.SubagentsPaths) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.SubagentsPaths))
	for _, pth := range c.SubagentsPaths {
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

// DiscoverFromConfig walks every path in cfg.SubagentsPaths (after home / env
// expansion), then dedups and filters by cfg.DisabledSubagents. It returns the
// three slices the rest of the system needs:
//
//   - all:    deduplicated, pre-filter (includes disabled).
//   - active: post-filter (DisabledSubagents removed).
//   - states: per-file discovery outcome for diagnostics/UI.
func DiscoverFromConfig(cfg DiscoveryConfig) (all, active []*Subagent, states []*SubagentState) {
	userPaths := cfg.ResolvePaths()
	discovered, allStates := DiscoverWithStates(userPaths, cfg.IsKnownModel, cfg.IsKnownSkill)
	all = Deduplicate(discovered)
	active = Filter(all, cfg.DisabledSubagents)
	allStates = DeduplicateStates(allStates)
	slices.SortStableFunc(allStates, func(a, b *SubagentState) int {
		return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path))
	})
	return all, active, allStates
}
