package subagents

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/pubsub"
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
	// IsKnownModelID validates that a model id (anything other than the
	// "large"/"small" aliases) resolves to a real provider model. May be nil
	// during discovery in contexts where the config is not yet loaded; in that
	// case model-id validation is skipped.
	IsKnownModelID func(string) bool
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
	discovered, allStates := DiscoverWithStates(userPaths, cfg.IsKnownModelID)
	all = Deduplicate(discovered)
	active = Filter(all, cfg.DisabledSubagents)
	allStates = DeduplicateStates(allStates)
	slices.SortStableFunc(allStates, func(a, b *SubagentState) int {
		return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path))
	})
	return all, active, allStates
}
