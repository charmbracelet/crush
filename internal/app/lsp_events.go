package app

import (
	"context"
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

// lspDiscoveryKey is a sentinel key used to track the discovery state
const lspDiscoveryKey = "_discovery"

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

// GetLSPStates returns the current state of all LSP clients (excluding internal sentinel keys)
func GetLSPStates() map[string]LSPClientInfo {
	states := lspStates.Copy()
	delete(states, lspDiscoveryKey)
	return states
}

// GetLSPState returns the state of a specific LSP client
func GetLSPState(name string) (LSPClientInfo, bool) {
	return lspStates.Get(name)
}

// IsLSPDiscovering returns true if LSP discovery is in progress
func IsLSPDiscovering() bool {
	info, exists := lspStates.Get(lspDiscoveryKey)
	return exists && info.State == lsp.StateDiscovering
}

// setLSPDiscovering sets the LSP discovery state using the state machine
func setLSPDiscovering(discovering bool) {
	if discovering {
		updateLSPState(lspDiscoveryKey, lsp.StateDiscovering, nil, nil, 0)
	} else {
		lspStates.Del(lspDiscoveryKey)
		// Publish event to trigger UI update
		lspBroker.Publish(pubsub.UpdatedEvent, LSPEvent{
			Type:  LSPEventStateChanged,
			Name:  lspDiscoveryKey,
			State: lsp.StateDisabled,
		})
	}
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

// updateLSPDiagnostics updates the diagnostic count for an LSP client and publishes an event
func updateLSPDiagnostics(name string, diagnosticCount int) {
	if info, exists := lspStates.Get(name); exists {
		info.DiagnosticCount = diagnosticCount
		lspStates.Set(name, info)

		// Publish diagnostics change event
		lspBroker.Publish(pubsub.UpdatedEvent, LSPEvent{
			Type:            LSPEventDiagnosticsChanged,
			Name:            name,
			State:           info.State,
			Error:           info.Error,
			DiagnosticCount: diagnosticCount,
		})
	}
}
