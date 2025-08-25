package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
)

type Client struct {
	Cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser

	// Client name for identification
	name string

	// File types this LSP server handles (e.g., .go, .rs, .py)
	fileTypes []string

	// Diagnostic change callback
	onDiagnosticsChanged func(name string, count int)

	// Request ID counter
	nextID atomic.Int32

	// Response handlers
	handlers   map[int32]chan *Message
	handlersMu sync.RWMutex

	// Server request handlers
	serverRequestHandlers map[string]ServerRequestHandler
	serverHandlersMu      sync.RWMutex

	// Notification handlers
	notificationHandlers map[string]NotificationHandler
	notificationMu       sync.RWMutex

	// Diagnostic cache
	diagnostics   map[protocol.DocumentURI][]protocol.Diagnostic
	diagnosticsMu sync.RWMutex

	// Files are currently opened by the LSP
	openFiles   map[string]*OpenFileInfo
	openFilesMu sync.RWMutex

	// Server state
	serverState atomic.Value
}

// NewClient creates a new LSP client.
func NewClient(ctx context.Context, name string, config config.LSPConfig) (*Client, error) {
	cmd := exec.CommandContext(ctx, config.ResolvedCommand(), config.Args...)

	// Copy env
	cmd.Env = slices.Concat(os.Environ(), config.ResolvedEnv())

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	client := &Client{
		Cmd:                   cmd,
		name:                  name,
		fileTypes:             config.FileTypes,
		stdin:                 stdin,
		stdout:                bufio.NewReader(stdout),
		stderr:                stderr,
		handlers:              make(map[int32]chan *Message),
		notificationHandlers:  make(map[string]NotificationHandler),
		serverRequestHandlers: make(map[string]ServerRequestHandler),
		diagnostics:           make(map[protocol.DocumentURI][]protocol.Diagnostic),
		openFiles:             make(map[string]*OpenFileInfo),
	}

	// Initialize server state
	client.serverState.Store(StateStarting)

	// Start the LSP server process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start LSP server: %w", err)
	}

	// Handle stderr in a separate goroutine
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			slog.Error("LSP Server", "err", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			slog.Error("Error reading", "err", err)
		}
	}()

	// Start message handling loop
	go func() {
		defer log.RecoverPanic("LSP-message-handler", func() {
			slog.Error("LSP message handler crashed, LSP functionality may be impaired")
		})
		client.handleMessages()
	}()

	return client, nil
}

func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.notificationMu.Lock()
	defer c.notificationMu.Unlock()
	c.notificationHandlers[method] = handler
}

func (c *Client) RegisterServerRequestHandler(method string, handler ServerRequestHandler) {
	c.serverHandlersMu.Lock()
	defer c.serverHandlersMu.Unlock()
	c.serverRequestHandlers[method] = handler
}

func (c *Client) InitializeLSPClient(ctx context.Context, workspaceDir string) (*protocol.InitializeResult, error) {
	initParams := &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: []protocol.WorkspaceFolder{
				{
					URI:  protocol.URI(protocol.URIFromPath(workspaceDir)),
					Name: workspaceDir,
				},
			},
		},

		XInitializeParams: protocol.XInitializeParams{
			ProcessID: int32(os.Getpid()),
			ClientInfo: &protocol.ClientInfo{
				Name:    "mcp-language-server",
				Version: "0.1.0",
			},
			RootPath: workspaceDir,
			RootURI:  protocol.URIFromPath(workspaceDir),
			Capabilities: protocol.ClientCapabilities{
				Workspace: protocol.WorkspaceClientCapabilities{
					Configuration: true,
					DidChangeConfiguration: protocol.DidChangeConfigurationClientCapabilities{
						DynamicRegistration: true,
					},
					DidChangeWatchedFiles: protocol.DidChangeWatchedFilesClientCapabilities{
						DynamicRegistration:    true,
						RelativePatternSupport: true,
					},
				},
				TextDocument: protocol.TextDocumentClientCapabilities{
					Synchronization: &protocol.TextDocumentSyncClientCapabilities{
						DynamicRegistration: true,
						DidSave:             true,
					},
					Completion: protocol.CompletionClientCapabilities{
						CompletionItem: protocol.ClientCompletionItemOptions{},
					},
					CodeLens: &protocol.CodeLensClientCapabilities{
						DynamicRegistration: true,
					},
					DocumentSymbol: protocol.DocumentSymbolClientCapabilities{},
					CodeAction: protocol.CodeActionClientCapabilities{
						CodeActionLiteralSupport: protocol.ClientCodeActionLiteralOptions{
							CodeActionKind: protocol.ClientCodeActionKindOptions{
								ValueSet: []protocol.CodeActionKind{},
							},
						},
					},
					PublishDiagnostics: protocol.PublishDiagnosticsClientCapabilities{
						VersionSupport: true,
					},
					SemanticTokens: protocol.SemanticTokensClientCapabilities{
						Requests: protocol.ClientSemanticTokensRequestOptions{
							Range: &protocol.Or_ClientSemanticTokensRequestOptions_range{},
							Full:  &protocol.Or_ClientSemanticTokensRequestOptions_full{},
						},
						TokenTypes:     []string{},
						TokenModifiers: []string{},
						Formats:        []protocol.TokenFormat{},
					},
				},
				Window: protocol.WindowClientCapabilities{},
			},
			InitializationOptions: map[string]any{
				"codelenses": map[string]bool{
					"generate":           true,
					"regenerate_cgo":     true,
					"test":               true,
					"tidy":               true,
					"upgrade_dependency": true,
					"vendor":             true,
					"vulncheck":          false,
				},
			},
		},
	}

	var result protocol.InitializeResult
	if err := c.Call(ctx, "initialize", initParams, &result); err != nil {
		return nil, fmt.Errorf("initialize failed: %w", err)
	}

	if err := c.Notify(ctx, "initialized", struct{}{}); err != nil {
		return nil, fmt.Errorf("initialized notification failed: %w", err)
	}

	// Register handlers
	c.RegisterServerRequestHandler("workspace/applyEdit", HandleApplyEdit)
	c.RegisterServerRequestHandler("workspace/configuration", HandleWorkspaceConfiguration)
	c.RegisterServerRequestHandler("client/registerCapability", HandleRegisterCapability)
	c.RegisterNotificationHandler("window/showMessage", HandleServerMessage)
	c.RegisterNotificationHandler("textDocument/publishDiagnostics",
		func(params json.RawMessage) { HandleDiagnostics(c, params) })

	// Notify the LSP server
	err := c.Initialized(ctx, protocol.InitializedParams{})
	if err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	return &result, nil
}

func (c *Client) Close() error {
	// Try to close all open files first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt to close files but continue shutdown regardless
	c.CloseAllFiles(ctx)

	// Close stdin to signal the server
	if err := c.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	// Use a channel to handle the Wait with timeout
	done := make(chan error, 1)
	go func() {
		done <- c.Cmd.Wait()
	}()

	// Wait for process to exit with timeout
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		// If we timeout, try to kill the process
		if err := c.Cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		return fmt.Errorf("process killed after timeout")
	}
}

type ServerState int

const (
	StateStarting ServerState = iota
	StateReady
	StateError
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

// WaitForServerReady waits for the server to be ready by polling the server
// with a simple request until it responds successfully or times out
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

	if cfg.Options.DebugLSP {
		slog.Debug("Waiting for LSP server to be ready...")
	}

	// Determine server type for specialized initialization
	serverType := c.detectServerType()

	// For TypeScript-like servers, we need to open some key files first
	if serverType == ServerTypeTypeScript {
		if cfg.Options.DebugLSP {
			slog.Debug("TypeScript-like server detected, opening key configuration files")
		}
		c.openKeyConfigFiles(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			c.SetServerState(StateError)
			return fmt.Errorf("timeout waiting for LSP server to be ready")
		case <-ticker.C:
			// Try a ping method appropriate for this server type
			err := c.pingServerByType(ctx, serverType)
			if err == nil {
				// Server responded successfully
				c.SetServerState(StateReady)
				if cfg.Options.DebugLSP {
					slog.Debug("LSP server is ready")
				}
				return nil
			} else {
				slog.Debug("LSP server not ready yet", "error", err, "serverType", serverType)
			}

			if cfg.Options.DebugLSP {
				slog.Debug("LSP server not ready yet", "error", err, "serverType", serverType)
			}
		}
	}
}

// ServerType represents the type of LSP server
type ServerType int

const (
	ServerTypeUnknown ServerType = iota
	ServerTypeGo
	ServerTypeTypeScript
	ServerTypeRust
	ServerTypePython
	ServerTypeGeneric
)

// detectServerType tries to determine what type of LSP server we're dealing with
func (c *Client) detectServerType() ServerType {
	if c.Cmd == nil {
		return ServerTypeUnknown
	}

	cmdPath := strings.ToLower(c.Cmd.Path)

	switch {
	case strings.Contains(cmdPath, "gopls"):
		return ServerTypeGo
	case strings.Contains(cmdPath, "typescript") || strings.Contains(cmdPath, "vtsls") || strings.Contains(cmdPath, "tsserver"):
		return ServerTypeTypeScript
	case strings.Contains(cmdPath, "rust-analyzer"):
		return ServerTypeRust
	case strings.Contains(cmdPath, "pyright") || strings.Contains(cmdPath, "pylsp") || strings.Contains(cmdPath, "python"):
		return ServerTypePython
	default:
		return ServerTypeGeneric
	}
}

// openKeyConfigFiles opens important configuration files that help initialize the server
func (c *Client) openKeyConfigFiles(ctx context.Context) {
	workDir := config.Get().WorkingDir()
	serverType := c.detectServerType()

	var filesToOpen []string

	switch serverType {
	case ServerTypeTypeScript:
		// TypeScript servers need these config files to properly initialize
		filesToOpen = []string{
			filepath.Join(workDir, "tsconfig.json"),
			filepath.Join(workDir, "package.json"),
			filepath.Join(workDir, "jsconfig.json"),
		}

		// Also find and open a few TypeScript files to help the server initialize
		c.openTypeScriptFiles(ctx, workDir)
	case ServerTypeGo:
		filesToOpen = []string{
			filepath.Join(workDir, "go.mod"),
			filepath.Join(workDir, "go.sum"),
		}
	case ServerTypeRust:
		filesToOpen = []string{
			filepath.Join(workDir, "Cargo.toml"),
			filepath.Join(workDir, "Cargo.lock"),
		}
	}

	// Try to open each file, ignoring errors if they don't exist
	for _, file := range filesToOpen {
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

// pingServerByType sends a ping request appropriate for the server type
func (c *Client) pingServerByType(ctx context.Context, serverType ServerType) error {
	switch serverType {
	case ServerTypeTypeScript:
		// For TypeScript, try a document symbol request on an open file
		return c.pingTypeScriptServer(ctx)
	case ServerTypeGo:
		// For Go, workspace/symbol works well
		return c.pingWithWorkspaceSymbol(ctx)
	case ServerTypeRust:
		// For Rust, workspace/symbol works well
		return c.pingWithWorkspaceSymbol(ctx)
	default:
		// Default ping method
		return c.pingWithWorkspaceSymbol(ctx)
	}
}

// pingTypeScriptServer tries to ping a TypeScript server with appropriate methods
func (c *Client) pingTypeScriptServer(ctx context.Context) error {
	// First try workspace/symbol which works for many servers
	if err := c.pingWithWorkspaceSymbol(ctx); err == nil {
		return nil
	}

	// If that fails, try to find an open file and request document symbols
	c.openFilesMu.RLock()
	defer c.openFilesMu.RUnlock()

	// If we have any open files, try to get document symbols for one
	for uri := range c.openFiles {
		filePath, err := protocol.DocumentURI(uri).Path()
		if err != nil {
			slog.Error("Failed to convert URI to path for TypeScript symbol collection", "uri", uri, "error", err)
			continue
		}

		if strings.HasSuffix(filePath, ".ts") || strings.HasSuffix(filePath, ".js") ||
			strings.HasSuffix(filePath, ".tsx") || strings.HasSuffix(filePath, ".jsx") {
			var symbols []protocol.DocumentSymbol
			err := c.Call(ctx, "textDocument/documentSymbol", protocol.DocumentSymbolParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI(uri),
				},
			}, &symbols)
			if err == nil {
				return nil
			}
		}
	}

	// If we have no open TypeScript files, try to find and open one
	workDir := config.Get().WorkingDir()
	err := filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-TypeScript files
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext == ".ts" || ext == ".js" || ext == ".tsx" || ext == ".jsx" {
			// Found a TypeScript file, try to open it
			if err := c.OpenFile(ctx, path); err == nil {
				// Successfully opened, stop walking
				return filepath.SkipAll
			}
		}

		return nil
	})
	if err != nil {
		slog.Debug("Error walking directory for TypeScript files", "error", err)
	}

	// Final fallback - just try a generic capability
	return c.pingWithServerCapabilities(ctx)
}

// openTypeScriptFiles finds and opens TypeScript files to help initialize the server
func (c *Client) openTypeScriptFiles(ctx context.Context, workDir string) {
	cfg := config.Get()
	filesOpened := 0
	maxFilesToOpen := 5 // Limit to a reasonable number of files

	// Find and open TypeScript files
	err := filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-TypeScript files
		if d.IsDir() {
			// Skip common directories to avoid wasting time
			if shouldSkipDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if we've opened enough files
		if filesOpened >= maxFilesToOpen {
			return filepath.SkipAll
		}

		// Check file extension
		ext := filepath.Ext(path)
		if ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" {
			// Try to open the file
			if err := c.OpenFile(ctx, path); err == nil {
				filesOpened++
				if cfg.Options.DebugLSP {
					slog.Debug("Opened TypeScript file for initialization", "file", path)
				}
			}
		}

		return nil
	})

	if err != nil && cfg.Options.DebugLSP {
		slog.Debug("Error walking directory for TypeScript files", "error", err)
	}

	if cfg.Options.DebugLSP {
		slog.Debug("Opened TypeScript files for initialization", "count", filesOpened)
	}
}

// shouldSkipDir returns true if the directory should be skipped during file search
func shouldSkipDir(path string) bool {
	dirName := filepath.Base(path)

	// Skip hidden directories
	if strings.HasPrefix(dirName, ".") {
		return true
	}

	// Skip common directories that won't contain relevant source files
	skipDirs := map[string]bool{
		"node_modules": true,
		"dist":         true,
		"build":        true,
		"coverage":     true,
		"vendor":       true,
		"target":       true,
	}

	return skipDirs[dirName]
}

// pingWithWorkspaceSymbol tries a workspace/symbol request
func (c *Client) pingWithWorkspaceSymbol(ctx context.Context) error {
	var result []protocol.SymbolInformation
	return c.Call(ctx, "workspace/symbol", protocol.WorkspaceSymbolParams{
		Query: "",
	}, &result)
}

// pingWithServerCapabilities tries to get server capabilities
func (c *Client) pingWithServerCapabilities(ctx context.Context) error {
	// This is a very lightweight request that should work for most servers
	return c.Notify(ctx, "$/cancelRequest", struct{ ID int }{ID: -1})
}

type OpenFileInfo struct {
	Version int32
	URI     protocol.DocumentURI
}

// HandlesFile checks if this LSP client handles the given file based on its
// extension.
func (c *Client) HandlesFile(path string) bool {
	// If no file types are specified, handle all files (backward compatibility)
	if len(c.fileTypes) == 0 {
		return true
	}

	name := strings.ToLower(filepath.Base(path))
	for _, filetpe := range c.fileTypes {
		suffix := strings.ToLower(filetpe)
		if !strings.HasPrefix(suffix, ".") {
			suffix = "." + suffix
		}
		if strings.HasSuffix(name, suffix) {
			slog.Debug("handles file", "name", c.name, "file", name, "filetype", filetpe)
			return true
		}
	}
	slog.Debug("doesn't handle file", "name", c.name, "file", name)
	return false
}

func (c *Client) OpenFile(ctx context.Context, filepath string) error {
	if !c.HandlesFile(filepath) {
		return nil
	}

	uri := string(protocol.URIFromPath(filepath))

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

	params := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentURI(uri),
			LanguageID: DetectLanguageID(uri),
			Version:    1,
			Text:       string(content),
		},
	}

	if err := c.Notify(ctx, "textDocument/didOpen", params); err != nil {
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

func (c *Client) NotifyChange(ctx context.Context, filepath string) error {
	uri := string(protocol.URIFromPath(filepath))

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
	version := fileInfo.Version
	c.openFilesMu.Unlock()

	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(uri),
			},
			Version: version,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Value: protocol.TextDocumentContentChangeWholeDocument{
					Text: string(content),
				},
			},
		},
	}

	return c.Notify(ctx, "textDocument/didChange", params)
}

func (c *Client) CloseFile(ctx context.Context, filepath string) error {
	cfg := config.Get()
	uri := string(protocol.URIFromPath(filepath))

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[uri]; !exists {
		c.openFilesMu.Unlock()
		return nil // Already closed
	}
	c.openFilesMu.Unlock()

	params := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentURI(uri),
		},
	}

	if cfg.Options.DebugLSP {
		slog.Debug("Closing file", "file", filepath)
	}
	if err := c.Notify(ctx, "textDocument/didClose", params); err != nil {
		return err
	}

	c.openFilesMu.Lock()
	delete(c.openFiles, uri)
	c.openFilesMu.Unlock()

	return nil
}

func (c *Client) IsFileOpen(filepath string) bool {
	uri := string(protocol.URIFromPath(filepath))
	c.openFilesMu.RLock()
	defer c.openFilesMu.RUnlock()
	_, exists := c.openFiles[uri]
	return exists
}

// CloseAllFiles closes all currently open files
func (c *Client) CloseAllFiles(ctx context.Context) {
	cfg := config.Get()
	c.openFilesMu.Lock()
	filesToClose := make([]string, 0, len(c.openFiles))

	// First collect all URIs that need to be closed
	for uri := range c.openFiles {
		// Convert URI back to file path using proper URI handling
		filePath, err := protocol.DocumentURI(uri).Path()
		if err != nil {
			slog.Error("Failed to convert URI to path for file closing", "uri", uri, "error", err)
			continue
		}
		filesToClose = append(filesToClose, filePath)
	}
	c.openFilesMu.Unlock()

	// Then close them all
	for _, filePath := range filesToClose {
		err := c.CloseFile(ctx, filePath)
		if err != nil && cfg.Options.DebugLSP {
			slog.Warn("Error closing file", "file", filePath, "error", err)
		}
	}

	if cfg.Options.DebugLSP {
		slog.Debug("Closed all files", "files", filesToClose)
	}
}

func (c *Client) GetFileDiagnostics(uri protocol.DocumentURI) []protocol.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	return c.diagnostics[uri]
}

// GetDiagnostics returns all diagnostics for all files
func (c *Client) GetDiagnostics() map[protocol.DocumentURI][]protocol.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	return maps.Clone(c.diagnostics)
}

// OpenFileOnDemand opens a file only if it's not already open
// This is used for lazy-loading files when they're actually needed
func (c *Client) OpenFileOnDemand(ctx context.Context, filepath string) error {
	// Check if the file is already open
	if c.IsFileOpen(filepath) {
		return nil
	}

	// Open the file
	return c.OpenFile(ctx, filepath)
}

// GetDiagnosticsForFile ensures a file is open and returns its diagnostics
// This is useful for on-demand diagnostics when using lazy loading
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
	c.diagnosticsMu.RLock()
	diagnostics := c.diagnostics[documentURI]
	c.diagnosticsMu.RUnlock()

	return diagnostics, nil
}

// ClearDiagnosticsForURI removes diagnostics for a specific URI from the cache
func (c *Client) ClearDiagnosticsForURI(uri protocol.DocumentURI) {
	c.diagnosticsMu.Lock()
	defer c.diagnosticsMu.Unlock()
	delete(c.diagnostics, uri)
}
