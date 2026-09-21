package mcp

import (
	"slices"
	"sort"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
)

// MaxActivatedPerSession bounds how many deferred tools one session can
// hold in context at once. Without a ceiling a session that searches
// repeatedly ends up paying more than the eager list ever did, which
// defeats the feature. Oldest activations are evicted first.
const MaxActivatedPerSession = 32

// activationsBySession records which deferred MCP tools a session has
// loaded. It is keyed by session because the coder agent object is shared
// by every session in a workspace: hiding tools by mutating that agent's
// tool list would leak one session's choices into another.
//
// Activations are in-process on purpose. They describe what a running
// conversation is using, and a restarted conversation re-searches in one
// cheap step.
var activationsBySession = &activationSet{bySession: map[string][]string{}}

type activationSet struct {
	mu        sync.Mutex
	bySession map[string][]string
}

// activate marks names visible for a session and returns the ones that were
// not visible before, in the order they were given. Re-activating a known
// name refreshes it so it becomes the last candidate to be evicted.
func (s *activationSet) activate(sessionID string, names []string) []string {
	if sessionID == "" || len(names) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	activated := slices.Clone(s.bySession[sessionID])
	var news []string
	for _, name := range names {
		if name == "" {
			continue
		}
		if i := slices.Index(activated, name); i >= 0 {
			activated = slices.Delete(activated, i, i+1)
		} else {
			news = append(news, name)
		}
		activated = append(activated, name)
	}
	for len(activated) > MaxActivatedPerSession {
		activated = activated[1:]
	}

	s.bySession[sessionID] = activated
	return news
}

// has reports whether a session already loaded name.
func (s *activationSet) has(sessionID, name string) bool {
	if sessionID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Contains(s.bySession[sessionID], name)
}

// snapshot returns a copy of a session's activated names, oldest first.
func (s *activationSet) snapshot(sessionID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.bySession[sessionID])
}

// clear drops every activation for a session.
func (s *activationSet) clear(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.bySession, sessionID)
}

// Activate marks deferred MCP tools as in use for a session, making them
// visible to the model on its next step. It returns the names that were not
// visible before; names are model-facing tool names, see ToolName.
func Activate(sessionID string, names ...string) []string {
	return activationsBySession.activate(sessionID, names)
}

// IsActivated reports whether a session already loaded a tool.
func IsActivated(sessionID, name string) bool {
	return activationsBySession.has(sessionID, name)
}

// ActivatedTools lists the tools a session has loaded, sorted by name.
func ActivatedTools(sessionID string) []string {
	activated := activationsBySession.snapshot(sessionID)
	sort.Strings(activated)
	return activated
}

// ClearSession forgets everything a session loaded. Call it when a session
// is deleted so long-lived servers do not accumulate entries.
func ClearSession(sessionID string) {
	activationsBySession.clear(sessionID)
}

// DeferredNames returns the MCP tool names to keep out of the model's tool
// array for a session: every registered tool of a lazy server that the
// session has not activated. It returns nil when nothing is hidden so
// callers can skip the exposure filter entirely.
//
// Servers disabled for the whole repository are not reported here. They are
// dropped from the tool slice outright, so nothing can bring them back.
func DeferredNames(cfg *config.ConfigStore, sessionID string) map[string]struct{} {
	if cfg == nil {
		return nil
	}

	var deferred map[string]struct{}
	for server, tools := range allTools.Seq2() {
		if !isLazyServer(cfg, server) {
			continue
		}
		for _, tool := range tools {
			if tool == nil {
				continue
			}
			name := ToolName(server, tool.Name)
			if IsActivated(sessionID, name) {
				continue
			}
			if deferred == nil {
				deferred = make(map[string]struct{})
			}
			deferred[name] = struct{}{}
		}
	}
	return deferred
}

// ExposedNames filters a step's candidate tool names down to the ones that
// may be serialized to the model: everything except the MCP tools this
// session has not loaded. The second return value is false when nothing is
// hidden, which tells the caller to leave ActiveTools unset so fantasy sends
// the whole slice.
//
// An empty allow-list is never returned even if every candidate is hidden:
// fantasy reads an empty ActiveTools as "everything is active", the exact
// opposite of the intent.
func ExposedNames(cfg *config.ConfigStore, sessionID string, names []string) ([]string, bool) {
	deferred := DeferredNames(cfg, sessionID)
	if len(deferred) == 0 {
		return nil, false
	}

	exposed := make([]string, 0, len(names))
	for _, name := range names {
		if _, hidden := deferred[name]; hidden {
			continue
		}
		exposed = append(exposed, name)
	}
	if len(exposed) == 0 {
		return nil, false
	}
	return exposed, true
}

// DeferredCount reports how many MCP tools are hidden from the model for a
// session right now.
func DeferredCount(cfg *config.ConfigStore, sessionID string) int {
	return len(DeferredNames(cfg, sessionID))
}
