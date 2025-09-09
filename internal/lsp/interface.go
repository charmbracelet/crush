package lsp

import (
	"context"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// Client defines the interface that all LSP clients must implement.
type Client interface {
	// Core lifecycle methods
	Initialize(ctx context.Context, workspaceDir string) (*protocol.InitializeResult, error)
	Shutdown(ctx context.Context) error
	Close() error

	// Server state management
	GetServerState() ServerState
	SetServerState(state ServerState)
	GetName() string
	WaitForServerReady(ctx context.Context) error

	// File management
	HandlesFile(path string) bool
	OpenFile(ctx context.Context, filepath string) error
	CloseFile(ctx context.Context, filepath string) error
	CloseAllFiles(ctx context.Context)
	IsFileOpen(filepath string) bool
	OpenFileOnDemand(ctx context.Context, filepath string) error
	NotifyChange(ctx context.Context, filepath string) error

	// Diagnostics
	GetFileDiagnostics(uri protocol.DocumentURI) []protocol.Diagnostic
	GetDiagnostics() map[protocol.DocumentURI][]protocol.Diagnostic
	GetDiagnosticsForFile(ctx context.Context, filepath string) ([]protocol.Diagnostic, error)
	ClearDiagnosticsForURI(uri protocol.DocumentURI)
	SetDiagnosticsCallback(callback func(name string, count int))

	// File watching
	DidChangeWatchedFiles(ctx context.Context, params protocol.DidChangeWatchedFilesParams) error

	// Event handling
	RegisterNotificationHandler(method string, handler NotificationHandler)
}

// ServerState represents the state of an LSP server
type ServerState int

const (
	StateStarting ServerState = iota
	StateReady
	StateShutdown
	StateError
	StateDisabled
)

// ServerType represents the type of LSP server
type ServerType string

const (
	ServerTypeGo         ServerType = "go"
	ServerTypeTypeScript ServerType = "typescript"
	ServerTypeRust       ServerType = "rust"
	ServerTypePython     ServerType = "python"
	ServerTypeGeneric    ServerType = "generic"
)

// OpenFileInfo contains information about an open file
type OpenFileInfo struct {
	URI     protocol.DocumentURI
	Version int32
	Content string
}

// NotificationHandler is a function that handles LSP notifications
type NotificationHandler func(method string, params interface{}) error

// FileWatchHandler is a function that handles file watch registrations from the server
type FileWatchHandler func(id string, watchers []protocol.FileSystemWatcher)

// Global file watch handler registry
var fileWatchHandler FileWatchHandler

// RegisterFileWatchHandler registers a global handler for file watch registrations
func RegisterFileWatchHandler(handler FileWatchHandler) {
	fileWatchHandler = handler
}
