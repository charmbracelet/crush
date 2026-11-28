package app

import (
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// LSPEventType represents the type of LSP event
type LSPEventType string

const (
	LSPEventStateChanged       LSPEventType = "state_changed"
	LSPEventDiagnosticsChanged LSPEventType = "diagnostics_changed"
)

// LSPEvent represents an event in the LSP system
type LSPEvent struct {
	Type            LSPEventType
	Name            string
	State           lsp.ServerState
	Error           error
	DiagnosticCount int
}

// LSPClientInfo holds information about an LSP client's state
type LSPClientInfo struct {
	Name            string
	State           lsp.ServerState
	Error           error
	Client          *lsp.Client
	DiagnosticCount int
	ConnectedAt     time.Time
}

var (
	lspStates = csync.NewMap[string, LSPClientInfo]()
	lspBroker = pubsub.NewBroker[LSPEvent]()
)

// SubscribeLSPEvents returns a channel for LSP events
func SubscribeLSPEvents(ctx context.Context) <-chan pubsub.Event[LSPEvent] {
	return lspBroker.Subscribe(ctx)
}

// GetLSPStates returns the current state of all LSP clients
func GetLSPStates() map[string]LSPClientInfo {
	return lspStates.Copy()
}

// GetLSPState returns the state of a specific LSP client
func GetLSPState(name string) (LSPClientInfo, bool) {
	return lspStates.Get(name)
}

// updateLSPState updates the state of an LSP client and publishes an event
func updateLSPState(name string, state lsp.ServerState, err error, client *lsp.Client, diagnosticCount int) {
	info := LSPClientInfo{
		Name:            name,
		State:           state,
		Error:           err,
		Client:          client,
		DiagnosticCount: diagnosticCount,
	}
	if state == lsp.StateReady {
		info.ConnectedAt = time.Now()
	} else if existing, ok := lspStates.Get(name); ok {
		info.ConnectedAt = existing.ConnectedAt
	}
	lspStates.Set(name, info)

	// Publish state change event
	lspBroker.Publish(pubsub.UpdatedEvent, LSPEvent{
		Type:            LSPEventStateChanged,
		Name:            name,
		State:           state,
		Error:           err,
		DiagnosticCount: diagnosticCount,
	})
}

// handleLSPClientUpdate is the LSP manager callback. The manager calls it
// for every running server on every Start, so it only records and publishes
// a state when the client or its state actually changed, keeping the
// diagnostic count and connection time of an unchanged client. A client
// that is no longer running (stopped by StopAll or KillAll, for example)
// drops its count, so stale diagnostics are not reported.
func handleLSPClientUpdate(name string, client *lsp.Client) {
	if client == nil {
		updateLSPState(name, lsp.StateUnstarted, nil, nil, 0)
		return
	}
	state := client.GetServerState()
	if existing, ok := lspStates.Get(name); ok && existing.Client == client {
		if existing.State == state {
			return
		}
		count := 0
		if lspStateKeepsDiagnostics(state) {
			count = existing.DiagnosticCount
		}
		updateLSPState(name, state, nil, client, count)
		return
	}
	client.SetDiagnosticsCallback(updateLSPDiagnostics)
	updateLSPState(name, state, nil, client, 0)
}

// lspStateKeepsDiagnostics reports whether a client in the given state may
// still hold live diagnostics. A client in the error state is included
// because the manager keeps a server that did not become ready running.
func lspStateKeepsDiagnostics(state lsp.ServerState) bool {
	switch state {
	case lsp.StateStarting, lsp.StateReady, lsp.StateError:
		return true
	default:
		return false
	}
}

// diagnosticsPublishInterval caps per-server diagnostics events; servers report file by file.
const diagnosticsPublishInterval = 150 * time.Millisecond

// keyedThrottle runs at most one pending call per key, after interval.
type keyedThrottle struct {
	interval time.Duration
	mu       sync.Mutex
	pending  map[string]bool
}

func (t *keyedThrottle) trigger(key string, fn func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pending[key] {
		return
	}
	t.pending[key] = true
	time.AfterFunc(t.interval, func() {
		t.mu.Lock()
		delete(t.pending, key)
		t.mu.Unlock()
		fn()
	})
}

var diagnosticsThrottle = &keyedThrottle{interval: diagnosticsPublishInterval, pending: map[string]bool{}}

// updateLSPDiagnostics stores the count now and publishes the latest one at most once per interval.
func updateLSPDiagnostics(name string, diagnosticCount int) {
	info, exists := lspStates.Get(name)
	if !exists {
		return
	}
	info.DiagnosticCount = diagnosticCount
	lspStates.Set(name, info)
	diagnosticsThrottle.trigger(name, func() { publishLSPDiagnostics(name) })
}

func publishLSPDiagnostics(name string) {
	info, exists := lspStates.Get(name)
	if !exists {
		return
	}
	lspBroker.Publish(pubsub.UpdatedEvent, LSPEvent{
		Type:            LSPEventDiagnosticsChanged,
		Name:            name,
		State:           info.State,
		Error:           info.Error,
		DiagnosticCount: info.DiagnosticCount,
	})
}
