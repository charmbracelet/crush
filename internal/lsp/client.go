package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fsext"
	powernap "github.com/charmbracelet/x/powernap/pkg/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type clientImpl struct {
	client *powernap.Client
	name   string

	// File types this LSP server handles (e.g., .go, .rs, .py)
	fileTypes []string

	// Configuration for this LSP client
	config config.LSPConfig

	// Diagnostic change callback
	onDiagnosticsChanged func(name string, count int)

	// Diagnostic cache
	diagnostics   map[protocol.DocumentURI][]protocol.Diagnostic
	diagnosticsMu sync.RWMutex

	// Files are currently opened by the LSP
	openFiles   map[string]*OpenFileInfo
	openFilesMu sync.RWMutex

	// Server state
	serverState atomic.Value

	// Server capabilities as returned by initialize
	caps    protocol.ServerCapabilities
	capsMu  sync.RWMutex
	capsSet atomic.Bool
}

// New creates a new LSP client using the powernap implementation.
func New(ctx context.Context, name string, config config.LSPConfig) (Client, error) {
	// Convert working directory to file URI
	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	rootURI := "file://" + workDir

	// Create powernap client config
	clientConfig := powernap.ClientConfig{
		Command: config.Command,
		Args:    config.Args,
		RootURI: rootURI,
		Environment: func() map[string]string {
			env := make(map[string]string)
			maps.Copy(env, config.Env)
			return env
		}(),
		Settings:    config.Options,
		InitOptions: config.InitOptions,
		WorkspaceFolders: []protocol.WorkspaceFolder{
			{
				URI:  rootURI,
				Name: filepath.Base(workDir),
			},
		},
	}

	// Create the powernap client
	powernapClient, err := powernap.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create powernap client: %w", err)
	}

	client := &clientImpl{
		client:      powernapClient,
		name:        name,
		fileTypes:   config.FileTypes,
		diagnostics: make(map[protocol.DocumentURI][]protocol.Diagnostic),
		openFiles:   make(map[string]*OpenFileInfo),
		config:      config,
	}

	// Initialize server state
	client.serverState.Store(StateStarting)

	// Register diagnostic handler
	client.client.RegisterNotificationHandler("textDocument/publishDiagnostics", func(ctx context.Context, method string, params json.RawMessage) {
		var diagParams protocol.PublishDiagnosticsParams
		if err := json.Unmarshal(params, &diagParams); err != nil {
			return
		}

		// Convert powernap diagnostics to protocol diagnostics
		protocolDiags := make([]protocol.Diagnostic, len(diagParams.Diagnostics))
		for i, diag := range diagParams.Diagnostics {
			protocolDiags[i] = protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      uint32(diag.Range.Start.Line),
						Character: uint32(diag.Range.Start.Character),
					},
					End: protocol.Position{
						Line:      uint32(diag.Range.End.Line),
						Character: uint32(diag.Range.End.Character),
					},
				},
				Severity: protocol.DiagnosticSeverity(diag.Severity),
				Code:     diag.Code,
				Source:   diag.Source,
				Message:  diag.Message,
			}
		}

		// Update diagnostic cache
		client.diagnosticsMu.Lock()
		uri := protocol.DocumentURI(diagParams.URI)
		client.diagnostics[uri] = protocolDiags
		client.diagnosticsMu.Unlock()

		// Notify callback if set
		if client.onDiagnosticsChanged != nil {
			totalDiags := 0
			client.diagnosticsMu.RLock()
			for _, diags := range client.diagnostics {
				totalDiags += len(diags)
			}
			client.diagnosticsMu.RUnlock()
			client.onDiagnosticsChanged(client.name, totalDiags)
		}
	})

	return client, nil
}

// Initialize initializes the LSP client and returns the server capabilities.
func (c *clientImpl) Initialize(ctx context.Context, workspaceDir string) (*protocol.InitializeResult, error) {
	if err := c.client.Initialize(ctx, false); err != nil {
		return nil, fmt.Errorf("failed to initialize powernap client: %w", err)
	}

	// Convert powernap capabilities to protocol capabilities
	caps := c.client.GetCapabilities()
	protocolCaps := protocol.ServerCapabilities{
		TextDocumentSync: caps.TextDocumentSync,
		CompletionProvider: func() *protocol.CompletionOptions {
			if caps.CompletionProvider != nil {
				return &protocol.CompletionOptions{
					TriggerCharacters:   caps.CompletionProvider.TriggerCharacters,
					AllCommitCharacters: caps.CompletionProvider.AllCommitCharacters,
					ResolveProvider:     caps.CompletionProvider.ResolveProvider,
				}
			}
			return nil
		}(),
		// Note: These need proper type conversion from interface{}
		// HoverProvider:      caps.HoverProvider,
		// DefinitionProvider: caps.DefinitionProvider,
	}

	c.setCapabilities(protocolCaps)

	result := &protocol.InitializeResult{
		Capabilities: protocolCaps,
	}

	return result, nil
}

// Shutdown sends a shutdown request to the LSP server.
func (c *clientImpl) Shutdown(ctx context.Context) error {
	return c.client.Shutdown(ctx)
}

// Close closes the LSP client.
func (c *clientImpl) Close() error {
	// Try to close all open files first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c.CloseAllFiles(ctx)

	// Shutdown and exit the client
	if err := c.client.Shutdown(ctx); err != nil {
		slog.Warn("Failed to shutdown LSP client", "error", err)
	}

	return c.client.Exit()
}

// GetServerState returns the current state of the LSP server
func (c *clientImpl) GetServerState() ServerState {
	if val := c.serverState.Load(); val != nil {
		return val.(ServerState)
	}
	return StateStarting
}

// SetServerState sets the current state of the LSP server
func (c *clientImpl) SetServerState(state ServerState) {
	c.serverState.Store(state)
}

// GetName returns the name of the LSP client
func (c *clientImpl) GetName() string {
	return c.name
}

// SetDiagnosticsCallback sets the callback function for diagnostic changes
func (c *clientImpl) SetDiagnosticsCallback(callback func(name string, count int)) {
	c.onDiagnosticsChanged = callback
}

// WaitForServerReady waits for the server to be ready
func (c *clientImpl) WaitForServerReady(ctx context.Context) error {
	cfg := config.Get()

	// Set initial state
	c.SetServerState(StateStarting)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Try to ping the server with a simple request
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	if cfg != nil && cfg.Options.DebugLSP {
		slog.Debug("Waiting for LSP server to be ready...")
	}

	c.openKeyConfigFiles(ctx)

	for {
		select {
		case <-ctx.Done():
			c.SetServerState(StateError)
			return fmt.Errorf("timeout waiting for LSP server to be ready")
		case <-ticker.C:
			// Check if client is running
			if !c.client.IsRunning() {
				if cfg != nil && cfg.Options.DebugLSP {
					slog.Debug("LSP server not ready yet", "server", c.name)
				}
				continue
			}

			// Server is ready
			c.SetServerState(StateReady)
			if cfg != nil && cfg.Options.DebugLSP {
				slog.Debug("LSP server is ready")
			}
			return nil
		}
	}
}

// HandlesFile checks if this LSP client handles the given file based on its extension.
func (c *clientImpl) HandlesFile(path string) bool {
	// If no file types are specified, handle all files (backward compatibility)
	if len(c.fileTypes) == 0 {
		return true
	}

	name := strings.ToLower(filepath.Base(path))
	for _, filetype := range c.fileTypes {
		if c.matchesFileType(name, filetype) {
			slog.Debug("handles file", "name", c.name, "file", name, "filetype", filetype)
			return true
		}
	}
	slog.Debug("doesn't handle file", "name", c.name, "file", name)
	return false
}

// matchesFileType checks if a filename matches a given file type pattern
func (c *clientImpl) matchesFileType(filename, filetype string) bool {
	filetype = strings.ToLower(filetype)

	// Handle special Go file types from powernap
	switch filetype {
	case "gomod":
		return filename == "go.mod"
	case "gowork":
		return filename == "go.work"
	case "gosum":
		return filename == "go.sum"
	default:
		// Handle regular extensions
		suffix := filetype
		if !strings.HasPrefix(suffix, ".") {
			suffix = "." + suffix
		}
		return strings.HasSuffix(filename, suffix)
	}
}

// OpenFile opens a file in the LSP server.
func (c *clientImpl) OpenFile(ctx context.Context, filepath string) error {
	if !c.HandlesFile(filepath) {
		return nil
	}

	uri := "file://" + filepath

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[uri]; exists {
		c.openFilesMu.Unlock()
		return nil // Already open
	}
	c.openFilesMu.Unlock()

	// Skip files that do not exist or cannot be read
	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Notify the server about the opened document
	err = c.client.NotifyDidOpenTextDocument(ctx, uri, string(DetectLanguageID(uri)), 1, string(content))
	if err != nil {
		return err
	}

	c.openFilesMu.Lock()
	c.openFiles[uri] = &OpenFileInfo{
		Version: 1,
		URI:     protocol.DocumentURI(uri),
	}
	c.openFilesMu.Unlock()

	return nil
}

// NotifyChange notifies the server about a file change.
func (c *clientImpl) NotifyChange(ctx context.Context, filepath string) error {
	uri := "file://" + filepath

	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	c.openFilesMu.Lock()
	fileInfo, isOpen := c.openFiles[uri]
	if !isOpen {
		c.openFilesMu.Unlock()
		return fmt.Errorf("cannot notify change for unopened file: %s", filepath)
	}

	// Increment version
	fileInfo.Version++
	version := int(fileInfo.Version)
	c.openFilesMu.Unlock()

	// Create change event
	changes := []protocol.TextDocumentContentChangeEvent{
		{
			Value: protocol.TextDocumentContentChangeWholeDocument{
				Text: string(content),
			},
		},
	}

	return c.client.NotifyDidChangeTextDocument(ctx, uri, version, changes)
}

// CloseFile closes a file in the LSP server.
func (c *clientImpl) CloseFile(ctx context.Context, filepath string) error {
	cfg := config.Get()
	uri := "file://" + filepath

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[uri]; !exists {
		c.openFilesMu.Unlock()
		return nil // Already closed
	}
	c.openFilesMu.Unlock()

	if cfg != nil && cfg.Options.DebugLSP {
		slog.Debug("Closing file", "file", filepath)
	}

	// Note: powernap doesn't have a direct NotifyDidCloseTextDocument method
	// We'll need to implement this or handle it differently

	c.openFilesMu.Lock()
	delete(c.openFiles, uri)
	c.openFilesMu.Unlock()

	return nil
}

// IsFileOpen checks if a file is currently open.
func (c *clientImpl) IsFileOpen(filepath string) bool {
	uri := "file://" + filepath
	c.openFilesMu.RLock()
	defer c.openFilesMu.RUnlock()
	_, exists := c.openFiles[uri]
	return exists
}

// CloseAllFiles closes all currently open files.
func (c *clientImpl) CloseAllFiles(ctx context.Context) {
	cfg := config.Get()
	c.openFilesMu.Lock()
	filesToClose := make([]string, 0, len(c.openFiles))

	// First collect all URIs that need to be closed
	for uri := range c.openFiles {
		// Convert URI back to file path
		filePath := strings.TrimPrefix(uri, "file://")
		filesToClose = append(filesToClose, filePath)
	}
	c.openFilesMu.Unlock()

	// Then close them all
	for _, filePath := range filesToClose {
		err := c.CloseFile(ctx, filePath)
		if err != nil && cfg != nil && cfg.Options.DebugLSP {
			slog.Warn("Error closing file", "file", filePath, "error", err)
		}
	}

	if cfg != nil && cfg.Options.DebugLSP {
		slog.Debug("Closed all files", "files", filesToClose)
	}
}

// GetFileDiagnostics returns diagnostics for a specific file.
func (c *clientImpl) GetFileDiagnostics(uri protocol.DocumentURI) []protocol.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	return c.diagnostics[uri]
}

// GetDiagnostics returns all diagnostics for all files.
func (c *clientImpl) GetDiagnostics() map[protocol.DocumentURI][]protocol.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	result := make(map[protocol.DocumentURI][]protocol.Diagnostic)
	for uri, diags := range c.diagnostics {
		result[uri] = slices.Clone(diags)
	}
	return result
}

// OpenFileOnDemand opens a file only if it's not already open.
func (c *clientImpl) OpenFileOnDemand(ctx context.Context, filepath string) error {
	// Check if the file is already open
	if c.IsFileOpen(filepath) {
		return nil
	}

	// Open the file
	return c.OpenFile(ctx, filepath)
}

// GetDiagnosticsForFile ensures a file is open and returns its diagnostics.
func (c *clientImpl) GetDiagnosticsForFile(ctx context.Context, filepath string) ([]protocol.Diagnostic, error) {
	documentURI := protocol.URIFromPath(filepath)

	// Make sure the file is open
	if !c.IsFileOpen(filepath) {
		if err := c.OpenFile(ctx, filepath); err != nil {
			return nil, fmt.Errorf("failed to open file for diagnostics: %w", err)
		}

		// Give the LSP server a moment to process the file
		time.Sleep(100 * time.Millisecond)
	}

	// Get diagnostics
	c.diagnosticsMu.RLock()
	diagnostics := c.diagnostics[documentURI]
	c.diagnosticsMu.RUnlock()

	return diagnostics, nil
}

// ClearDiagnosticsForURI removes diagnostics for a specific URI from the cache.
func (c *clientImpl) ClearDiagnosticsForURI(uri protocol.DocumentURI) {
	c.diagnosticsMu.Lock()
	defer c.diagnosticsMu.Unlock()
	delete(c.diagnostics, uri)
}

// setCapabilities sets the server capabilities.
func (c *clientImpl) setCapabilities(caps protocol.ServerCapabilities) {
	c.capsMu.Lock()
	defer c.capsMu.Unlock()
	c.caps = caps
	c.capsSet.Store(true)
}

// RegisterNotificationHandler registers a notification handler.
func (c *clientImpl) RegisterNotificationHandler(method string, handler NotificationHandler) {
	// Convert the handler to the powernap format
	powernapHandler := func(ctx context.Context, methodName string, params json.RawMessage) {
		// Convert params to interface{} for the handler
		var paramsInterface any
		if err := json.Unmarshal(params, &paramsInterface); err != nil {
			paramsInterface = params
		}
		handler(methodName, paramsInterface)
	}
	c.client.RegisterNotificationHandler(method, powernapHandler)
}

// DidChangeWatchedFiles sends a workspace/didChangeWatchedFiles notification to the server.
func (c *clientImpl) DidChangeWatchedFiles(ctx context.Context, params protocol.DidChangeWatchedFilesParams) error {
	cfg := config.Get()
	if cfg != nil && cfg.Options.DebugLSP {
		slog.Debug("Sending file change notification to LSP server",
			"client", c.name,
			"changes", len(params.Changes))

		for _, change := range params.Changes {
			slog.Debug("File change notification details",
				"uri", change.URI,
				"type", change.Type,
				"client", c.name)
		}
	}

	// Convert protocol.FileEvent to powernap.FileEvent
	changes := make([]protocol.FileEvent, len(params.Changes))
	for i, change := range params.Changes {
		changes[i] = protocol.FileEvent{
			URI:  change.URI,
			Type: change.Type,
		}
	}

	// Use the new PowerNap method to send the notification
	err := c.client.NotifyDidChangeWatchedFiles(ctx, changes)
	if err != nil && cfg != nil && cfg.Options.DebugLSP {
		slog.Error("Failed to send file change notification",
			"client", c.name,
			"error", err)
	}

	return err
}

// openKeyConfigFiles opens important configuration files that help initialize the server.
func (c *clientImpl) openKeyConfigFiles(ctx context.Context) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	// Try to open each file, ignoring errors if they don't exist
	for _, file := range c.config.RootMarkers {
		file = filepath.Join(wd, file)
		if _, err := os.Stat(file); err == nil {
			// File exists, try to open it
			if err := c.OpenFile(ctx, file); err != nil {
				slog.Debug("Failed to open key config file", "file", file, "error", err)
			} else {
				slog.Debug("Opened key config file for initialization", "file", file)
			}
		}
	}
}

// HasRootMarkers checks if any of the specified root marker patterns exist in the given directory.
// Uses glob patterns to match files, allowing for more flexible matching.
func HasRootMarkers(dir string, rootMarkers []string) bool {
	if len(rootMarkers) == 0 {
		return true
	}

	for _, pattern := range rootMarkers {
		// Use fsext.GlobWithDoubleStar to find matches
		matches, _, err := fsext.GlobWithDoubleStar(pattern, dir, 1)
		if err == nil && len(matches) > 0 {
			return true
		}
	}
	return false
}
