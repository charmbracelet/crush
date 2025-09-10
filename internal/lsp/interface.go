package lsp

import (
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

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
type NotificationHandler func(method string, params any) error

// FileWatchHandler is a function that handles file watch registrations from the server
type FileWatchHandler func(id string, watchers []protocol.FileSystemWatcher)

// Global file watch handler registry
var fileWatchHandler FileWatchHandler

// RegisterFileWatchHandler registers a global handler for file watch registrations
func RegisterFileWatchHandler(handler FileWatchHandler) {
	fileWatchHandler = handler
}
