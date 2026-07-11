package workspace

import (
	"context"

	"github.com/charmbracelet/crush/internal/memory"
)

type MemoryFeature string

const (
	MemoryFeatureEnabled  MemoryFeature = "enabled"
	MemoryFeatureRecorder MemoryFeature = "recorder"
	MemoryFeatureRecall   MemoryFeature = "recall"
)

// MemorySnapshot is the frontend-safe state needed by memory controls.
type MemorySnapshot struct {
	Available       bool
	Enabled         bool
	RecorderEnabled bool
	RecallEnabled   bool
	Directory       string
	Project         memory.Project
	SessionID       string
	SessionMode     memory.SessionRecordingMode
	Stats           memory.Stats
	Records         []memory.Record
}

type MemoryRememberInput struct {
	Scope       memory.Scope
	Kind        memory.Kind
	Name        string
	Description string
	Content     string
	Pinned      bool
}

// MemoryWorkspace is optional so remote clients that do not expose memory
// endpoints remain compatible with the base Workspace interface.
type MemoryWorkspace interface {
	MemorySnapshot(ctx context.Context, sessionID string) (MemorySnapshot, error)
	MemoryRemember(ctx context.Context, input MemoryRememberInput) (memory.Record, error)
	MemorySetStatus(ctx context.Context, id string, status memory.Status) error
	MemorySetPinned(ctx context.Context, id string, pinned bool) error
	MemorySetFeature(ctx context.Context, feature MemoryFeature, enabled bool) error
	MemorySetSessionRecording(ctx context.Context, sessionID string, enabled bool) error
	MemoryMaintain(ctx context.Context) (memory.MaintenanceResult, error)
}
