package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/home"
	powernap "github.com/charmbracelet/x/powernap/pkg/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/charmbracelet/x/powernap/pkg/transport"
)

// DiagnosticCounts holds the count of diagnostics by severity.
type DiagnosticCounts struct {
	Error       int
	Warning     int
	Information int
	Hint        int
}

type Client struct {
	client *powernap.Client
	name   string

	// File types this LSP server handles (e.g., .go, .rs, .py)
	fileTypes []string

	// Configuration for this LSP client
	config config.LSPConfig

	// Diagnostic change callback
	onDiagnosticsChanged func(name string, count int)

	// Diagnostic cache
	diagnostics *csync.VersionedMap[protocol.DocumentURI, []protocol.Diagnostic]

	// Cached diagnostic counts to avoid map copy on every UI render.
	diagCountsCache   DiagnosticCounts
	diagCountsVersion uint64
	diagCountsMu      sync.Mutex

	// Files are currently opened by the LSP
	openFiles *csync.Map[string, *OpenFileInfo]

	// Server state
	serverState atomic.Value
}

// New creates a new LSP client using the powernap implementation.
func New(ctx context.Context, name string, config config.LSPConfig, resolver config.VariableResolver) (*Client, error) {
	// Convert working directory to file URI
	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	rootURI := string(protocol.URIFromPath(workDir))

	command, err := resolver.ResolveValue(config.Command)
	if err != nil {
		return nil, fmt.Errorf("invalid lsp command: %w", err)
	}

	// Create powernap client config
	clientConfig := powernap.ClientConfig{
		Command: home.Long(command),
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
		return nil, fmt.Errorf("failed to create lsp client: %w", err)
	}

	client := &Client{
		client:      powernapClient,
		name:        name,
		fileTypes:   config.FileTypes,
		diagnostics: csync.NewVersionedMap[protocol.DocumentURI, []protocol.Diagnostic](),
		openFiles:   csync.NewMap[string, *OpenFileInfo](),
		config:      config,
	}

	// Initialize server state
	client.serverState.Store(StateStarting)

	return client, nil
}

// Initialize initializes the LSP client and returns the server capabilities.
func (c *Client) Initialize(ctx context.Context, workspaceDir string) (*protocol.InitializeResult, error) {
	if err := c.client.Initialize(ctx, false); err != nil {
		return nil, fmt.Errorf("failed to initialize the lsp client: %w", err)
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
	}

	result := &protocol.InitializeResult{
		Capabilities: protocolCaps,
	}

	c.RegisterServerRequestHandler("workspace/applyEdit", HandleApplyEdit)
	c.RegisterServerRequestHandler("workspace/configuration", HandleWorkspaceConfiguration)
	c.RegisterServerRequestHandler("client/registerCapability", HandleRegisterCapability)
	c.RegisterNotificationHandler("window/showMessage", HandleServerMessage)
	c.RegisterNotificationHandler("textDocument/publishDiagnostics", func(_ context.Context, _ string, params json.RawMessage) {
		HandleDiagnostics(c, params)
	})

	return result, nil
}

// Close closes the LSP client.
func (c *Client) Close(ctx context.Context) error {
	c.CloseAllFiles(ctx)

	// Shutdown and exit the client
	if err := c.client.Shutdown(ctx); err != nil {
		slog.Warn("Failed to shutdown LSP client", "error", err)
	}

	return c.client.Exit()
}

// ServerState represents the state of an LSP server
type ServerState int

const (
	StateStarting ServerState = iota
	StateReady
	StateError
	StateDisabled
)

// GetServerState returns the current state of the LSP server
func (c *Client) GetServerState() ServerState {
	if val := c.serverState.Load(); val != nil {
		return val.(ServerState)
	}
	return StateStarting
}

// SetServerState sets the current state of the LSP server
func (c *Client) SetServerState(state ServerState) {
	c.serverState.Store(state)
}

// GetName returns the name of the LSP client
func (c *Client) GetName() string {
	return c.name
}

// SetDiagnosticsCallback sets the callback function for diagnostic changes
func (c *Client) SetDiagnosticsCallback(callback func(name string, count int)) {
	c.onDiagnosticsChanged = callback
}

// WaitForServerReady waits for the server to be ready
func (c *Client) WaitForServerReady(ctx context.Context) error {
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

// OpenFileInfo contains information about an open file
type OpenFileInfo struct {
	Version int32
	URI     protocol.DocumentURI
}

// HandlesFile checks if this LSP client handles the given file based on its extension.
func (c *Client) HandlesFile(path string) bool {
	// If no file types are specified, handle all files (backward compatibility)
	if len(c.fileTypes) == 0 {
		return true
	}

	name := strings.ToLower(filepath.Base(path))
	for _, filetype := range c.fileTypes {
		suffix := strings.ToLower(filetype)
		if !strings.HasPrefix(suffix, ".") {
			suffix = "." + suffix
		}
		if strings.HasSuffix(name, suffix) {
			slog.Debug("handles file", "name", c.name, "file", name, "filetype", filetype)
			return true
		}
	}
	slog.Debug("doesn't handle file", "name", c.name, "file", name)
	return false
}

// OpenFile opens a file in the LSP server.
func (c *Client) OpenFile(ctx context.Context, filepath string) error {
	if !c.HandlesFile(filepath) {
		return nil
	}

	uri := string(protocol.URIFromPath(filepath))

	if _, exists := c.openFiles.Get(uri); exists {
		return nil // Already open
	}

	// Skip files that do not exist or cannot be read
	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Notify the server about the opened document
	if err = c.client.NotifyDidOpenTextDocument(ctx, uri, string(DetectLanguageID(uri)), 1, string(content)); err != nil {
		return err
	}

	c.openFiles.Set(uri, &OpenFileInfo{
		Version: 1,
		URI:     protocol.DocumentURI(uri),
	})

	return nil
}

// NotifyChange notifies the server about a file change.
func (c *Client) NotifyChange(ctx context.Context, filepath string) error {
	uri := string(protocol.URIFromPath(filepath))

	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	fileInfo, isOpen := c.openFiles.Get(uri)
	if !isOpen {
		return fmt.Errorf("cannot notify change for unopened file: %s", filepath)
	}

	// Increment version
	fileInfo.Version++

	// Create change event
	changes := []protocol.TextDocumentContentChangeEvent{
		{
			Value: protocol.TextDocumentContentChangeWholeDocument{
				Text: string(content),
			},
		},
	}

	return c.client.NotifyDidChangeTextDocument(ctx, uri, int(fileInfo.Version), changes)
}

// IsFileOpen checks if a file is currently open.
func (c *Client) IsFileOpen(filepath string) bool {
	uri := string(protocol.URIFromPath(filepath))
	_, exists := c.openFiles.Get(uri)
	return exists
}

// CloseAllFiles closes all currently open files.
func (c *Client) CloseAllFiles(ctx context.Context) {
	cfg := config.Get()
	debugLSP := cfg != nil && cfg.Options.DebugLSP
	for uri := range c.openFiles.Seq2() {
		if debugLSP {
			slog.Debug("Closing file", "file", uri)
		}
		if err := c.client.NotifyDidCloseTextDocument(ctx, uri); err != nil {
			slog.Warn("Error closing file", "uri", uri, "error", err)
			continue
		}
		c.openFiles.Del(uri)
	}
}

// GetFileDiagnostics returns diagnostics for a specific file.
func (c *Client) GetFileDiagnostics(uri protocol.DocumentURI) []protocol.Diagnostic {
	diags, _ := c.diagnostics.Get(uri)
	return diags
}

// GetDiagnostics returns all diagnostics for all files.
func (c *Client) GetDiagnostics() map[protocol.DocumentURI][]protocol.Diagnostic {
	return c.diagnostics.Copy()
}

// GetDiagnosticCounts returns cached diagnostic counts by severity.
// Uses the VersionedMap version to avoid recomputing on every call.
func (c *Client) GetDiagnosticCounts() DiagnosticCounts {
	currentVersion := c.diagnostics.Version()

	c.diagCountsMu.Lock()
	defer c.diagCountsMu.Unlock()

	if currentVersion == c.diagCountsVersion {
		return c.diagCountsCache
	}

	// Recompute counts.
	counts := DiagnosticCounts{}
	for _, diags := range c.diagnostics.Seq2() {
		for _, diag := range diags {
			switch diag.Severity {
			case protocol.SeverityError:
				counts.Error++
			case protocol.SeverityWarning:
				counts.Warning++
			case protocol.SeverityInformation:
				counts.Information++
			case protocol.SeverityHint:
				counts.Hint++
			}
		}
	}

	c.diagCountsCache = counts
	c.diagCountsVersion = currentVersion
	return counts
}

// OpenFileOnDemand opens a file only if it's not already open.
func (c *Client) OpenFileOnDemand(ctx context.Context, filepath string) error {
	// Check if the file is already open
	if c.IsFileOpen(filepath) {
		return nil
	}

	// Open the file
	return c.OpenFile(ctx, filepath)
}

// GetDiagnosticsForFile ensures a file is open and returns its diagnostics.
func (c *Client) GetDiagnosticsForFile(ctx context.Context, filepath string) ([]protocol.Diagnostic, error) {
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
	diagnostics, _ := c.diagnostics.Get(documentURI)

	return diagnostics, nil
}

// ClearDiagnosticsForURI removes diagnostics for a specific URI from the cache.
func (c *Client) ClearDiagnosticsForURI(uri protocol.DocumentURI) {
	c.diagnostics.Del(uri)
}

// RegisterNotificationHandler registers a notification handler.
func (c *Client) RegisterNotificationHandler(method string, handler transport.NotificationHandler) {
	c.client.RegisterNotificationHandler(method, handler)
}

// RegisterServerRequestHandler handles server requests.
func (c *Client) RegisterServerRequestHandler(method string, handler transport.Handler) {
	c.client.RegisterHandler(method, handler)
}

// DidChangeWatchedFiles sends a workspace/didChangeWatchedFiles notification to the server.
func (c *Client) DidChangeWatchedFiles(ctx context.Context, params protocol.DidChangeWatchedFilesParams) error {
	return c.client.NotifyDidChangeWatchedFiles(ctx, params.Changes)
}

// openKeyConfigFiles opens important configuration files that help initialize the server.
func (c *Client) openKeyConfigFiles(ctx context.Context) {
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
				slog.Error("Failed to open key config file", "file", file, "error", err)
			} else {
				slog.Debug("Opened key config file for initialization", "file", file)
			}
		}
	}
}

// WaitForDiagnostics waits until diagnostics change or the timeout is reached.
func (c *Client) WaitForDiagnostics(ctx context.Context, d time.Duration) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(d)
	pv := c.diagnostics.Version()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			return
		case <-ticker.C:
			if pv != c.diagnostics.Version() {
				return
			}
		}
	}
}

// FindReferences finds all references to the symbol at the given position.
func (c *Client) FindReferences(ctx context.Context, filepath string, line, character int, includeDeclaration bool) ([]protocol.Location, error) {
	if err := c.OpenFileOnDemand(ctx, filepath); err != nil {
		return nil, err
	}
	// NOTE: line and character should be 0-based.
	// See: https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#position
	return c.client.FindReferences(ctx, filepath, line-1, character-1, includeDeclaration)
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
