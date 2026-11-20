package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/lsphost"
	powernap "github.com/charmbracelet/x/powernap/pkg/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/charmbracelet/x/powernap/pkg/transport"
	"github.com/sourcegraph/jsonrpc2"
)

// DiagnosticCounts holds the count of diagnostics by severity.
type DiagnosticCounts struct {
	Error       int
	Warning     int
	Information int
	Hint        int
}

type Client struct {
	// client is the connection to the server. A restart replaces it while
	// other goroutines use the client, so it is read through
	// [Client.server].
	client atomic.Pointer[powernap.Client]
	name   string
	debug  bool

	// Working directory this LSP is scoped to.
	cwd string

	// File types this LSP server handles (e.g., .go, .rs, .py)
	fileTypes []string

	// Configuration for this LSP client
	config config.LSPConfig

	// Long-lived context for the client's lifetime, independent of any
	// request-scoped context. Used for restart and other operations that
	// must survive beyond the initial tool call that created the client.
	ctx       context.Context
	cancelCtx context.CancelFunc
	resolver  config.VariableResolver

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

	// host is the server of a host application this client is connected
	// to, or nil when the client runs a server of its own.
	host *hostTarget

	// lost is set once the connection to the server is known to be gone:
	// the server exited, or the host closed the connection. A lost client
	// is of no use and never comes back; see [Client.markLost].
	lost atomic.Bool
	// closing is set while the client is being stopped on purpose, so that
	// the connection going away is not taken for a failure.
	closing atomic.Bool
	// restarting is set while [Client.Restart] runs. A restart closes the
	// client and starts it again, which is not the client being stopped:
	// a start that is still waiting for the client leaves it alone.
	restarting atomic.Bool
	// onLost, when set, is called once, on its own goroutine, when the
	// client is lost. It is set before the client is initialized.
	onLost func(*Client)
}

// New creates a new LSP client using the powernap implementation.
func New(
	name string,
	cfg config.LSPConfig,
	resolver config.VariableResolver,
	cwd string,
	debug bool,
) (*Client, error) {
	return newClient(name, cfg, resolver, cwd, debug, nil)
}

// newClient creates a client for a server of its own or, when host is not
// nil, for that server of a host application.
func newClient(
	name string,
	cfg config.LSPConfig,
	resolver config.VariableResolver,
	cwd string,
	debug bool,
	host *hostTarget,
) (*Client, error) {
	// Use a long-lived context independent of the caller's request context.
	// The caller's context may be canceled when the tool call completes,
	// but the LSP client must survive across multiple requests and restarts.
	clientCtx, cancelCtx := context.WithCancel(context.Background())
	client := &Client{
		name:        name,
		fileTypes:   cfg.FileTypes,
		diagnostics: csync.NewVersionedMap[protocol.DocumentURI, []protocol.Diagnostic](),
		openFiles:   csync.NewMap[string, *OpenFileInfo](),
		config:      cfg,
		ctx:         clientCtx,
		cancelCtx:   cancelCtx,
		debug:       debug,
		resolver:    resolver,
		cwd:         cwd,
		host:        host,
	}
	client.serverState.Store(StateStopped)

	if err := client.createPowernapClient(); err != nil {
		return nil, err
	}

	return client, nil
}

// Initialize initializes the LSP client and returns the server capabilities.
func (c *Client) Initialize(ctx context.Context, workspaceDir string) (*protocol.InitializeResult, error) {
	// Register handlers for requests the server may send during the
	// initialize handshake itself (e.g. typescript-language-server issuing
	// window/workDoneProgress/create while loading the project, before
	// initialize has returned). Registering after client.Initialize() is too
	// late for those — the server treats an unhandled response as fatal.
	c.registerHandlers()

	if err := c.server().Initialize(ctx, false); err != nil {
		return nil, fmt.Errorf("failed to initialize the lsp client: %w", err)
	}

	// Convert powernap capabilities to protocol capabilities
	caps := c.server().GetCapabilities()
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

	return result, nil
}

// closeTimeout is the maximum time to wait for a graceful LSP shutdown.
const closeTimeout = 5 * time.Second

// server returns the client's current connection to its server.
func (c *Client) server() *powernap.Client {
	return c.client.Load()
}

// Kill kills the client without doing anything else.
func (c *Client) Kill() {
	c.closing.Store(true)
	c.server().Kill()
}

// Shutdown permanently cancels the client's long-lived context and kills the
// underlying process. Unlike Restart, this is terminal: the client cannot be
// reused after Shutdown.
func (c *Client) Shutdown() {
	c.closing.Store(true)
	c.cancelCtx()
	c.server().Kill()
}

// Host returns the name of the host application whose language server this
// client is connected to, and false when the client runs its own server.
func (c *Client) Host() (string, bool) {
	if c == nil || c.host == nil {
		return "", false
	}
	return c.host.name, true
}

// alive reports whether the client's connection to its server is not known
// to be gone.
func (c *Client) alive() bool {
	return !c.lost.Load()
}

// markLost records that the connection to the server is gone, unless the
// client is being stopped on purpose. The client's state becomes
// [StateError] and onLost runs, which is how the manager drops the client
// so that the next start connects or spawns again.
func (c *Client) markLost() {
	if c.closing.Load() || !c.lost.CompareAndSwap(false, true) {
		return
	}
	slog.Warn("LSP server connection lost", "name", c.name, "hosted", c.host != nil)
	c.SetServerState(StateError)
	if c.onLost != nil {
		// Not on this goroutine: it may be the connection's read loop,
		// which the handler will close.
		go c.onLost(c)
	}
}

// observe marks the client lost when err says its connection is gone, and
// returns err. The LSP library cannot tell that a server exited until
// something is written to it, so every call to the server goes through
// here.
func (c *Client) observe(err error) error {
	if err != nil && connectionLost(err) {
		c.markLost()
	}
	return err
}

// errConnectionLost is returned for a client whose server went away while
// it was starting.
var errConnectionLost = errors.New("the connection to the server was lost")

// errStopped is returned for a client that was stopped on purpose while it
// was starting. It is not a failure of the server or of its host.
var errStopped = errors.New("the client was stopped while it was starting")

// errRestarted is returned for a client that was restarted while it was
// starting. The restart has taken the client over.
var errRestarted = errors.New("the client was restarted while it was starting")

// stopReason returns why the client cannot become ready:
// [errConnectionLost] when its connection is gone, [errStopped] when it
// is being stopped on purpose, and nil otherwise. A lost client is shut
// down by its manager, which also marks it as closing, so being lost is
// checked first.
func (c *Client) stopReason() error {
	switch {
	case c.lost.Load():
		return errConnectionLost
	case c.closing.Load():
		return errStopped
	}
	return nil
}

// interrupted is stopReason for a start that is waiting for the client. A
// restart closes the client on its way, which is no reason for the start
// to give the client up: there is none while one runs.
func (c *Client) interrupted() error {
	// In this order: a restart marks itself before it closes the client.
	reason := c.stopReason()
	if c.restarting.Load() {
		return nil
	}
	return reason
}

// giveUp ends the wait for a client that was interrupted while starting,
// and returns why. A lost client is in [StateError] again, which waiting
// had replaced; the state of a stopped one is left to whoever stopped it.
func (c *Client) giveUp(why error) error {
	if errors.Is(why, errConnectionLost) {
		c.SetServerState(StateError)
	}
	return why
}

// connectionLost reports whether err means the server's end of the
// connection is gone.
func connectionLost(err error) bool {
	return errors.Is(err, jsonrpc2.ErrClosed) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, os.ErrClosed)
}

// GetOffsetEncoding returns the negotiated offset encoding for this client.
func (c *Client) GetOffsetEncoding() powernap.OffsetEncoding {
	return c.server().GetOffsetEncoding()
}

// Close closes all open files in the client, then shuts down gracefully.
// If shutdown takes longer than closeTimeout, it falls back to Kill().
//
// The exit notification is only sent after the server answered the shutdown
// request: a server that gets exit without having shut down exits with an
// error, and a server shared with other clients could take them with it.
// A server that fails to shut down is killed instead.
func (c *Client) Close(ctx context.Context) error {
	c.closing.Store(true)
	c.CloseAllFiles(ctx)

	// Use a timeout to prevent hanging on unresponsive LSP servers.
	// jsonrpc2's send lock doesn't respect context cancellation, so we
	// need to fall back to Kill() which closes the underlying connection.
	closeCtx, cancel := context.WithTimeout(ctx, closeTimeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		if err := c.server().Shutdown(closeCtx); err != nil {
			slog.Warn("Failed to shutdown LSP client", "error", err)
			c.server().Kill()
			done <- err
			return
		}
		done <- c.server().Exit()
	}()

	select {
	case err := <-done:
		return err
	case <-closeCtx.Done():
		c.server().Kill()
		return closeCtx.Err()
	}
}

// createPowernapClient creates a new powernap client with the current configuration.
func (c *Client) createPowernapClient() error {
	rootURI := string(protocol.URIFromPath(c.cwd))

	command, err := c.resolver.ResolveValue(c.config.Command)
	if err != nil {
		return fmt.Errorf("invalid lsp command: %w", err)
	}

	args, err := c.config.ResolvedArgs(c.resolver)
	if err != nil {
		return fmt.Errorf("invalid lsp args: %w", err)
	}

	envs, err := c.config.ResolvedEnv(c.resolver)
	if err != nil {
		return fmt.Errorf("invalid lsp env: %w", err)
	}

	command = home.Long(command)
	if c.host != nil {
		// The host's server is reached through this executable, which
		// connects the client's pipes to the host's socket. The server
		// runs with the host's arguments, environment and settings.
		command = c.host.executable
		args = lsphost.Args(c.host.socket, c.host.server, c.cwd, c.host.workspace, os.Getpid())
		envs = nil
	}

	clientConfig := powernap.ClientConfig{
		Command:     command,
		Args:        args,
		RootURI:     rootURI,
		Environment: envs,
		Settings:    c.config.Options,
		InitOptions: c.config.InitOptions,
		WorkspaceFolders: []protocol.WorkspaceFolder{
			{
				URI:  rootURI,
				Name: filepath.Base(c.cwd),
			},
		},
	}

	powernapClient, err := powernap.NewClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to create lsp client: %w", err)
	}

	c.client.Store(powernapClient)
	c.lost.Store(false)
	c.closing.Store(false)
	return nil
}

// registerHandlers registers the standard LSP notification and request handlers.
func (c *Client) registerHandlers() {
	c.RegisterServerRequestHandler("workspace/applyEdit", HandleApplyEdit(c.server().GetOffsetEncoding()))
	c.RegisterServerRequestHandler("workspace/configuration", HandleWorkspaceConfiguration)
	c.RegisterServerRequestHandler("client/registerCapability", HandleRegisterCapability)
	c.RegisterServerRequestHandler("window/workDoneProgress/create", HandleWorkDoneProgressCreate)
	c.RegisterNotificationHandler("window/showMessage", func(ctx context.Context, method string, params json.RawMessage) {
		if c.debug {
			HandleServerMessage(ctx, method, params)
		}
	})
	c.RegisterNotificationHandler("textDocument/publishDiagnostics", func(_ context.Context, _ string, params json.RawMessage) {
		HandleDiagnostics(c, params)
	})
	if c.host != nil {
		// Sent by `crush __lsp-dial` when the host closes the connection.
		c.RegisterNotificationHandler(lsphost.ClosedMethod, func(context.Context, string, json.RawMessage) {
			c.markLost()
		})
	}
}

// Restart closes the current LSP client and creates a new one with the same configuration.
func (c *Client) Restart() error {
	c.restarting.Store(true)
	defer c.restarting.Store(false)

	// The open files are keyed by URI; they are reopened by path.
	var openFiles []string
	for uri := range c.openFiles.Seq2() {
		path, err := protocol.DocumentURI(uri).Path()
		if err != nil {
			slog.Warn("Failed to convert URI to path", "uri", uri, "error", err)
			continue
		}
		openFiles = append(openFiles, path)
	}

	// Cancel the old long-lived context and create a fresh one so that
	// reinitialization is not affected by any prior cancellation.
	c.cancelCtx()
	c.ctx, c.cancelCtx = context.WithCancel(context.Background())

	closeCtx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()

	if err := c.Close(closeCtx); err != nil {
		slog.Warn("Error closing client during restart", "name", c.name, "error", err)
	}

	c.SetServerState(StateStopped)

	// Nothing is open on the new server, whether or not the old one was
	// told so.
	for uri := range c.openFiles.Seq2() {
		c.openFiles.Del(uri)
	}

	c.diagCountsCache = DiagnosticCounts{}
	c.diagCountsVersion = 0

	if err := c.createPowernapClient(); err != nil {
		return err
	}

	initCtx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	c.SetServerState(StateStarting)

	// Register handlers before Initialize so servers that send
	// requests during the handshake (e.g. window/workDoneProgress/create)
	// don't crash on an unhandled response.
	c.registerHandlers()

	if err := c.server().Initialize(initCtx, false); err != nil {
		c.SetServerState(StateError)
		return fmt.Errorf("failed to initialize lsp client: %w", err)
	}

	if err := c.waitForServerReady(initCtx, true); err != nil {
		slog.Error("Server failed to become ready after restart", "name", c.name, "error", err)
		c.SetServerState(StateError)
		return err
	}

	for _, path := range openFiles {
		if err := c.OpenFile(initCtx, path); err != nil {
			slog.Warn("Failed to reopen file after restart", "file", path, "error", err)
		}
	}
	return nil
}

// ServerState represents the state of an LSP server
type ServerState int

const (
	StateUnstarted ServerState = iota
	StateStarting
	StateReady
	StateError
	StateStopped
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

// FileTypes returns the file types this LSP client handles
func (c *Client) FileTypes() []string {
	return slices.Clone(c.fileTypes)
}

// SetDiagnosticsCallback sets the callback function for diagnostic changes
func (c *Client) SetDiagnosticsCallback(callback func(name string, count int)) {
	c.onDiagnosticsChanged = callback
}

// WaitForServerReady waits for the server to be ready. It gives up at once
// when the client is lost or stopped meanwhile: neither can become ready,
// and waiting out ctx would block the start that is waiting for this one.
func (c *Client) WaitForServerReady(ctx context.Context) error {
	return c.waitForServerReady(ctx, false)
}

// waitForServerReady is WaitForServerReady for a start, and for a restart
// when restart is set. A start that finds the client being restarted
// leaves the server alone until the restart is done with it.
func (c *Client) waitForServerReady(ctx context.Context, restart bool) error {
	// Set initial state
	c.SetServerState(StateStarting)

	// Try to ping the server with a simple request
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	if c.debug {
		slog.Debug("Waiting for LSP server to be ready...")
	}

	c.openKeyConfigFiles(ctx)

	for {
		select {
		case <-ctx.Done():
			c.SetServerState(StateError)
			return fmt.Errorf("timeout waiting for LSP server to be ready")
		case <-ticker.C:
			// In this order: a restart marks itself before it closes
			// the client.
			reason := c.stopReason()
			if !restart && c.restarting.Load() {
				continue
			}
			if reason != nil {
				return c.giveUp(reason)
			}
			// Check if client is running
			if !c.server().IsRunning() {
				if c.debug {
					slog.Debug("LSP server not ready yet", "server", c.name)
				}
				continue
			}

			// Server is ready
			c.SetServerState(StateReady)
			if c.debug {
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

// HandlesFile checks if this LSP client handles the given file based on its
// extension and whether it's within the working directory.
func (c *Client) HandlesFile(path string) bool {
	if c == nil {
		return false
	}
	if !fsext.HasPrefix(path, c.cwd) {
		slog.Debug("File outside workspace", "name", c.name, "file", path, "workDir", c.cwd)
		return false
	}
	return handlesFiletype(c.name, c.fileTypes, path)
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
	if err = c.server().NotifyDidOpenTextDocument(ctx, uri, string(powernap.DetectLanguage(filepath)), 1, string(content)); err != nil {
		return c.observe(err)
	}

	c.openFiles.Set(uri, &OpenFileInfo{
		Version: 1,
		URI:     protocol.DocumentURI(uri),
	})

	return nil
}

// NotifyChange notifies the server about a file change.
func (c *Client) NotifyChange(ctx context.Context, filepath string) error {
	if c == nil {
		return nil
	}
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

	return c.observe(c.server().NotifyDidChangeTextDocument(ctx, uri, int(fileInfo.Version), changes))
}

// IsFileOpen checks if a file is currently open.
func (c *Client) IsFileOpen(filepath string) bool {
	uri := string(protocol.URIFromPath(filepath))
	_, exists := c.openFiles.Get(uri)
	return exists
}

// CloseAllFiles closes all currently open files.
func (c *Client) CloseAllFiles(ctx context.Context) {
	for uri := range c.openFiles.Seq2() {
		if c.debug {
			slog.Debug("Closing file", "file", uri)
		}
		if err := c.server().NotifyDidCloseTextDocument(ctx, uri); err != nil {
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
	if c == nil {
		return nil
	}
	return c.diagnostics.Copy()
}

// GetDiagnosticCounts returns cached diagnostic counts by severity.
// Uses the VersionedMap version to avoid recomputing on every call.
func (c *Client) GetDiagnosticCounts() DiagnosticCounts {
	if c == nil {
		return DiagnosticCounts{}
	}
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
	if c == nil {
		return nil
	}
	// Check if the file is already open
	if c.IsFileOpen(filepath) {
		return nil
	}

	// Open the file
	return c.OpenFile(ctx, filepath)
}

// RegisterNotificationHandler registers a notification handler.
func (c *Client) RegisterNotificationHandler(method string, handler transport.NotificationHandler) {
	c.server().RegisterNotificationHandler(method, handler)
}

// RegisterServerRequestHandler handles server requests.
func (c *Client) RegisterServerRequestHandler(method string, handler transport.Handler) {
	c.server().RegisterHandler(method, handler)
}

// openKeyConfigFiles opens important configuration files that help initialize the server.
func (c *Client) openKeyConfigFiles(ctx context.Context) {
	// Try to open each file, ignoring errors if they don't exist
	for _, file := range c.config.RootMarkers {
		file = filepath.Join(c.cwd, file)
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

// NotifyWorkspaceChange sends a workspace-level file change notification to
// trigger re-analysis of all files. This is useful when the overall project
// state may have changed (e.g., after a project-wide refactoring) and
// diagnostics for files not currently being edited may be stale.
func (c *Client) NotifyWorkspaceChange(ctx context.Context) error {
	if c == nil {
		return nil
	}
	return c.observe(c.server().NotifyDidChangeWatchedFiles(ctx, []protocol.FileEvent{
		{URI: protocol.DocumentURI(protocol.URIFromPath(c.cwd)), Type: protocol.Changed},
	}))
}

// RefreshOpenFiles re-notifies the LSP server about all currently open files,
// which triggers re-analysis and fresh diagnostics for the entire project.
func (c *Client) RefreshOpenFiles(ctx context.Context) {
	if c == nil {
		return
	}
	for uri, info := range c.openFiles.Seq2() {
		path, err := protocol.DocumentURI(uri).Path()
		if err != nil {
			slog.Warn("Failed to convert URI to path", "uri", uri, "error", err)
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("Failed to read file for refresh", "path", path, "error", err)
			continue
		}
		info.Version++
		changes := []protocol.TextDocumentContentChangeEvent{
			{
				Value: protocol.TextDocumentContentChangeWholeDocument{
					Text: string(content),
				},
			},
		}
		if err := c.observe(c.server().NotifyDidChangeTextDocument(ctx, uri, int(info.Version), changes)); err != nil {
			slog.Warn("Failed to notify file change", "uri", uri, "error", err)
		}
	}
}

// WaitForDiagnostics waits until diagnostics stop changing for a settling
// period, indicating the LSP server has finished processing. If no
// diagnostics change within firstChangeDuration, it returns early since the
// server likely isn't going to republish.
func (c *Client) WaitForDiagnostics(ctx context.Context, timeout time.Duration) {
	if c == nil {
		return
	}

	const (
		firstChangeDuration = 1 * time.Second
		settleDuration      = 300 * time.Millisecond
	)

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	firstChangeTimer := time.NewTimer(min(timeout, firstChangeDuration))
	defer firstChangeTimer.Stop()
	previousVersion := c.diagnostics.Version()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			return
		case <-firstChangeTimer.C:
			// No change arrived quickly — server isn't republishing.
			return
		case <-ticker.C:
			currentVersion := c.diagnostics.Version()
			if currentVersion != previousVersion {
				// Diagnostics changed — now wait for them to settle.
				c.waitForDiagnosticsToSettle(ctx, deadline.C, settleDuration)
				return
			}
		}
	}
}

// waitForDiagnosticsToSettle waits until diagnostics version stays the same
// for settleDuration, indicating the LSP server has finished publishing.
func (c *Client) waitForDiagnosticsToSettle(ctx context.Context, deadline <-chan time.Time, settleDuration time.Duration) {
	lastVersion := c.diagnostics.Version()
	settleTicker := time.NewTicker(50 * time.Millisecond)
	defer settleTicker.Stop()

	// Track how long the version has been stable.
	stableStart := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			return
		case <-settleTicker.C:
			currentVersion := c.diagnostics.Version()
			if currentVersion != lastVersion {
				// New change detected — reset the stable timer.
				lastVersion = currentVersion
				stableStart = time.Now()
			} else if time.Since(stableStart) >= settleDuration {
				// Diagnostics have been stable for the settle duration.
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

	// Add timeout to prevent hanging on slow LSP servers.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// NOTE: line and character should be 0-based.
	// See: https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#position
	result, err := c.server().FindReferences(ctx, filepath, line-1, character-1, includeDeclaration)
	return result, c.observe(err) //nolint:wrapcheck
}

// Rename renames the symbol at the given position across all files.
func (c *Client) Rename(ctx context.Context, filepath string, line, character int, newName string) (*protocol.WorkspaceEdit, error) {
	if err := c.OpenFileOnDemand(ctx, filepath); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := c.server().RequestRename(ctx, filepath, line-1, character-1, newName)
	return result, c.observe(err) //nolint:wrapcheck
}

// DocumentSymbols returns the document symbols for the given file.
func (c *Client) DocumentSymbols(ctx context.Context, filepath string) ([]protocol.DocumentSymbolResult, error) {
	if err := c.OpenFileOnDemand(ctx, filepath); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.server().RequestDocumentSymbols(ctx, filepath)
	return result, c.observe(err) //nolint:wrapcheck
}

// Definition finds the definition of the symbol at the given position.
func (c *Client) Definition(ctx context.Context, filepath string, line, character int) ([]protocol.Location, error) {
	if err := c.OpenFileOnDemand(ctx, filepath); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.server().RequestDefinition(ctx, filepath, line-1, character-1)
	return result, c.observe(err) //nolint:wrapcheck
}

// PrepareCallHierarchy prepares a call hierarchy item at the given position.
func (c *Client) PrepareCallHierarchy(ctx context.Context, filepath string, line, character int) ([]protocol.CallHierarchyItem, error) {
	if err := c.OpenFileOnDemand(ctx, filepath); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.server().PrepareCallHierarchy(ctx, filepath, line-1, character-1)
	return result, c.observe(err) //nolint:wrapcheck
}

// IncomingCalls returns all callers of the given call hierarchy item.
func (c *Client) IncomingCalls(ctx context.Context, item protocol.CallHierarchyItem) ([]protocol.CallHierarchyIncomingCall, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := c.server().IncomingCalls(ctx, item)
	return result, c.observe(err) //nolint:wrapcheck
}

// OutgoingCalls returns all callees of the given call hierarchy item.
func (c *Client) OutgoingCalls(ctx context.Context, item protocol.CallHierarchyItem) ([]protocol.CallHierarchyOutgoingCall, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := c.server().OutgoingCalls(ctx, item)
	return result, c.observe(err) //nolint:wrapcheck
}
