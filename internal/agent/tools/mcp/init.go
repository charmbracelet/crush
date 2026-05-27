// Package mcp provides functionality for managing Model Context Protocol (MCP)
// clients within the Crush application.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/oauth"
	mcpoauth "github.com/charmbracelet/crush/internal/oauth/mcp"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func parseLevel(level mcp.LoggingLevel) slog.Level {
	switch level {
	case "info":
		return slog.LevelInfo
	case "notice":
		return slog.LevelInfo
	case "warning":
		return slog.LevelWarn
	default:
		return slog.LevelDebug
	}
}

// ClientSession wraps an mcp.ClientSession with a context cancel function so
// that the context created during session establishment is properly cleaned up
// on close.
type ClientSession struct {
	*mcp.ClientSession
	cancel       context.CancelFunc
	oauthHandler *mcpoauth.Handler
}

// Close cancels the session context and then closes the underlying session.
func (s *ClientSession) Close() error {
	s.cancel()
	if s.oauthHandler != nil {
		s.oauthHandler.Close()
	}
	return s.ClientSession.Close()
}

var (
	sessions = csync.NewMap[string, *ClientSession]()
	states   = csync.NewMap[string, ClientInfo]()
	authURLs = csync.NewMap[string, *mcpoauth.Handler]()
	broker   = pubsub.NewBroker[Event]()
	initOnce sync.Once
	initDone = make(chan struct{})
)

// State represents the current state of an MCP client
type State int

const (
	StateDisabled State = iota
	StateStarting
	StateConnected
	StateError
	StateNeedsAuth
)

func (s State) String() string {
	switch s {
	case StateDisabled:
		return "disabled"
	case StateStarting:
		return "starting"
	case StateConnected:
		return "connected"
	case StateError:
		return "error"
	case StateNeedsAuth:
		return "needs auth"
	default:
		return "unknown"
	}
}

// EventType represents the type of MCP event
type EventType uint

const (
	EventStateChanged EventType = iota
	EventToolsListChanged
	EventPromptsListChanged
	EventResourcesListChanged
)

// Event represents an event in the MCP system
type Event struct {
	Type   EventType
	Name   string
	State  State
	Error  error
	Counts Counts
}

// Counts number of available tools, prompts, etc.
type Counts struct {
	Tools     int
	Prompts   int
	Resources int
}

// ClientInfo holds information about an MCP client's state
type ClientInfo struct {
	Name        string
	State       State
	Error       error
	Client      *ClientSession
	Counts      Counts
	ConnectedAt time.Time
}

// SubscribeEvents returns a channel for MCP events
func SubscribeEvents(ctx context.Context) <-chan pubsub.Event[Event] {
	return broker.Subscribe(ctx)
}

// GetStates returns the current state of all MCP clients
func GetStates() map[string]ClientInfo {
	return states.Copy()
}

// GetState returns the state of a specific MCP client
func GetState(name string) (ClientInfo, bool) {
	return states.Get(name)
}

// Close closes all MCP clients. This should be called during application shutdown.
func Close(ctx context.Context) error {
	var wg sync.WaitGroup
	for name, session := range sessions.Seq2() {
		wg.Go(func() {
			done := make(chan error, 1)
			go func() {
				done <- session.Close()
			}()
			select {
			case err := <-done:
				if err != nil &&
					!errors.Is(err, io.EOF) &&
					!errors.Is(err, context.Canceled) &&
					err.Error() != "signal: killed" {
					slog.Warn("Failed to shutdown MCP client", "name", name, "error", err)
				}
			case <-ctx.Done():
			}
		})
	}
	wg.Wait()
	// Clean up any remaining OAuth handlers.
	for _, h := range authURLs.Seq2() {
		h.Close()
	}
	broker.Shutdown()
	return nil
}

// Initialize initializes MCP clients based on the provided configuration.
func Initialize(ctx context.Context, permissions permission.Service, cfg *config.ConfigStore) {
	slog.Info("Initializing MCP clients")
	var wg sync.WaitGroup
	// Initialize states for all configured MCPs
	for name, m := range cfg.Config().MCP {
		if m.Disabled {
			updateState(name, StateDisabled, nil, nil, Counts{})
			slog.Debug("Skipping disabled MCP", "name", name)
			continue
		}

		// Set initial starting state
		wg.Add(1)
		go func(name string, m config.MCPConfig) {
			defer func() {
				wg.Done()
				if r := recover(); r != nil {
					var err error
					switch v := r.(type) {
					case error:
						err = v
					case string:
						err = fmt.Errorf("panic: %s", v)
					default:
						err = fmt.Errorf("panic: %v", v)
					}
					updateState(name, StateError, err, nil, Counts{})
					slog.Error("Panic in MCP client initialization", "error", err, "name", name)
				}
			}()

			if err := initClient(ctx, cfg, name, m, cfg.Resolver()); err != nil {
				slog.Debug("Failed to initialize MCP client", "name", name, "error", err)
			}
		}(name, m)
	}
	wg.Wait()
	initOnce.Do(func() { close(initDone) })
}

// WaitForInit blocks until MCP initialization is complete.
// If Initialize was never called, this returns immediately.
func WaitForInit(ctx context.Context) error {
	select {
	case <-initDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// InitializeSingle initializes a single MCP client by name.
func InitializeSingle(ctx context.Context, name string, cfg *config.ConfigStore) error {
	m, exists := cfg.Config().MCP[name]
	if !exists {
		return fmt.Errorf("mcp '%s' not found in configuration", name)
	}

	if m.Disabled {
		updateState(name, StateDisabled, nil, nil, Counts{})
		slog.Debug("Skipping disabled MCP", "name", name)
		return nil
	}

	return initClient(ctx, cfg, name, m, cfg.Resolver())
}

// AuthenticateMCP initiates the OAuth flow for an MCP server that is in
// StateNeedsAuth. It creates the OAuth handler (which starts a local
// callback server), connects to the server (which triggers the browser
// auth flow on 401), and transitions to StateConnected on success.
func AuthenticateMCP(ctx context.Context, cfg *config.ConfigStore, name string) error {
	m, exists := cfg.Config().MCP[name]
	if !exists {
		return fmt.Errorf("mcp '%s' not found in configuration", name)
	}

	if !m.OAuth || m.Type != config.MCPHttp {
		return fmt.Errorf("mcp '%s' does not use OAuth authentication", name)
	}

	updateState(name, StateStarting, nil, nil, Counts{})

	session, err := connectAndRegister(ctx, cfg, name, m, cfg.Resolver())
	if err != nil {
		return err
	}

	persistOAuthToken(cfg, name, session)
	return nil
}

// PendingAuthServer describes an MCP server awaiting OAuth.
type PendingAuthServer struct {
	Name string
	URL  string
}

// MCPAuthURL returns the current OAuth authorization URL for the named
// MCP, or empty if none is in progress.
func MCPAuthURL(name string) string {
	h, ok := authURLs.Get(name)
	if !ok || h == nil {
		return ""
	}
	return h.AuthURL()
}

// PendingAuthMCPs returns MCP servers in StateNeedsAuth with their URLs.
func PendingAuthMCPs(cfg *config.ConfigStore) []PendingAuthServer {
	var pending []PendingAuthServer
	for name, info := range states.Seq2() {
		if info.State == StateNeedsAuth {
			url := ""
			if m, ok := cfg.Config().MCP[name]; ok {
				url = m.URL
			}
			pending = append(pending, PendingAuthServer{Name: name, URL: url})
		}
	}
	slices.SortFunc(pending, func(a, b PendingAuthServer) int {
		return strings.Compare(a.Name, b.Name)
	})
	return pending
}

// initClient initializes a single MCP client with the given configuration.
func initClient(ctx context.Context, cfg *config.ConfigStore, name string, m config.MCPConfig, resolver config.VariableResolver) error {
	// OAuth MCPs without a cached token require user interaction (browser
	// auth). Defer them so the UI can show a dialog and let the user
	// approve. If a cached token exists, try connecting with it directly.
	if m.OAuth && m.Type == config.MCPHttp && (m.OAuthToken == nil || m.OAuthToken.IsExpired()) {
		updateState(name, StateNeedsAuth, nil, nil, Counts{})
		clearMCPData(name)
		slog.Info("MCP server requires OAuth authentication", "name", name)
		return nil
	}

	updateState(name, StateStarting, nil, nil, Counts{})
	_, err := connectAndRegister(ctx, cfg, name, m, resolver)
	return err
}

// connectAndRegister creates a session, lists tools and prompts,
// registers them in global state, and transitions to StateConnected.
// Returns the session so callers can perform post-processing (e.g.
// token persistence).
func connectAndRegister(ctx context.Context, cfg *config.ConfigStore, name string, m config.MCPConfig, resolver config.VariableResolver) (*ClientSession, error) {
	session, err := createSession(ctx, name, m, resolver)
	if err != nil {
		return nil, err
	}

	tools, err := getTools(ctx, session)
	if err != nil {
		slog.Error("Error listing tools", "error", err)
		updateState(name, StateError, err, nil, Counts{})
		session.Close()
		return nil, err
	}

	prompts, err := getPrompts(ctx, session)
	if err != nil {
		slog.Error("Error listing prompts", "error", err)
		updateState(name, StateError, err, nil, Counts{})
		session.Close()
		return nil, err
	}

	toolCount := updateTools(cfg, name, tools)
	updatePrompts(name, prompts)
	sessions.Set(name, session)

	updateState(name, StateConnected, nil, session, Counts{
		Tools:   toolCount,
		Prompts: len(prompts),
	})

	return session, nil
}

// persistOAuthToken saves the OAuth token from a session to the global
// config so it survives restarts.
func persistOAuthToken(cfg *config.ConfigStore, name string, session *ClientSession) {
	if session.oauthHandler == nil {
		return
	}
	tok := session.oauthHandler.Token()
	if tok == nil {
		return
	}
	oauthToken := &oauth.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresIn:    int(time.Until(tok.Expiry).Seconds()),
	}
	oauthToken.SetExpiresAt()
	if err := cfg.SetConfigField(config.ScopeGlobal, fmt.Sprintf("mcp.%s.oauth_token", name), oauthToken); err != nil {
		slog.Warn("Failed to persist MCP OAuth token", "name", name, "error", err)
	} else {
		slog.Info("Persisted MCP OAuth token", "name", name)
	}
}

// DisableSingle disables and closes a single MCP client by name.
func DisableSingle(cfg *config.ConfigStore, name string) error {
	session, ok := sessions.Get(name)
	if ok {
		if err := session.Close(); err != nil &&
			!errors.Is(err, io.EOF) &&
			!errors.Is(err, context.Canceled) &&
			err.Error() != "signal: killed" {
			slog.Warn("Error closing MCP session", "name", name, "error", err)
		}
		sessions.Del(name)
	}

	// Clear tools, prompts, resources, and auth state for this MCP.
	clearMCPData(name)

	// Update state to disabled.
	updateState(name, StateDisabled, nil, nil, Counts{})

	slog.Info("Disabled mcp client", "name", name)
	return nil
}

func getOrRenewClient(ctx context.Context, cfg *config.ConfigStore, name string) (*ClientSession, error) {
	sess, ok := sessions.Get(name)
	if !ok {
		return nil, fmt.Errorf("mcp '%s' not available", name)
	}

	m := cfg.Config().MCP[name]
	state, _ := states.Get(name)

	timeout := mcpTimeout(m)
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := sess.Ping(pingCtx, nil)
	if err == nil {
		return sess, nil
	}
	updateState(name, StateError, maybeTimeoutErr(err, timeout), nil, state.Counts)

	sess, err = createSession(ctx, name, m, cfg.Resolver())
	if err != nil {
		clearMCPData(name)
		// If an OAuth MCP fails to reconnect, prompt the user to
		// re-authenticate instead of leaving it in an error state.
		if m.OAuth && m.Type == config.MCPHttp {
			updateState(name, StateNeedsAuth, nil, nil, Counts{})
			slog.Info("MCP OAuth session expired, re-authentication required", "name", name)
		}
		return nil, err
	}

	updateState(name, StateConnected, nil, sess, state.Counts)
	sessions.Set(name, sess)
	return sess, nil
}

// updateState updates the state of an MCP client and publishes an event
func updateState(name string, state State, err error, client *ClientSession, counts Counts) {
	info := ClientInfo{
		Name:   name,
		State:  state,
		Error:  err,
		Client: client,
		Counts: counts,
	}
	switch state {
	case StateConnected:
		info.ConnectedAt = time.Now()
	case StateError:
		sessions.Del(name)
	}
	states.Set(name, info)

	// Publish state change event
	broker.Publish(pubsub.UpdatedEvent, Event{
		Type:   EventStateChanged,
		Name:   name,
		State:  state,
		Error:  err,
		Counts: counts,
	})
}

func createSession(ctx context.Context, name string, m config.MCPConfig, resolver config.VariableResolver) (*ClientSession, error) {
	timeout := mcpTimeout(m)
	mcpCtx, cancel := context.WithCancel(ctx)
	cancelTimer := time.AfterFunc(timeout, cancel)

	transport, oauthHandler, err := createTransport(mcpCtx, name, m, resolver)
	if err != nil {
		updateState(name, StateError, err, nil, Counts{})
		slog.Error("Error creating MCP client", "error", err, "name", name)
		cancel()
		cancelTimer.Stop()
		return nil, err
	}

	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "crush",
			Version: version.Version,
			Title:   "Crush",
		},
		&mcp.ClientOptions{
			ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
				broker.Publish(pubsub.UpdatedEvent, Event{
					Type: EventToolsListChanged,
					Name: name,
				})
			},
			PromptListChangedHandler: func(context.Context, *mcp.PromptListChangedRequest) {
				broker.Publish(pubsub.UpdatedEvent, Event{
					Type: EventPromptsListChanged,
					Name: name,
				})
			},
			ResourceListChangedHandler: func(context.Context, *mcp.ResourceListChangedRequest) {
				broker.Publish(pubsub.UpdatedEvent, Event{
					Type: EventResourcesListChanged,
					Name: name,
				})
			},
			LoggingMessageHandler: func(ctx context.Context, req *mcp.LoggingMessageRequest) {
				level := parseLevel(req.Params.Level)
				slog.Log(ctx, level, "MCP log", "name", name, "logger", req.Params.Logger, "data", req.Params.Data)
			},
		},
	)

	session, err := client.Connect(mcpCtx, transport, nil)
	if err != nil {
		err = maybeStdioErr(err, transport)
		updateState(name, StateError, maybeTimeoutErr(err, timeout), nil, Counts{})
		slog.Error("MCP client failed to initialize", "error", err, "name", name)
		cancel()
		cancelTimer.Stop()
		return nil, err
	}

	cancelTimer.Stop()
	slog.Debug("MCP client initialized", "name", name)
	return &ClientSession{
		ClientSession: session,
		cancel:        cancel,
		oauthHandler:  oauthHandler,
	}, nil
}

// maybeStdioErr if a stdio mcp prints an error in non-json format, it'll fail
// to parse, and the cli will then close it, causing the EOF error.
// so, if we got an EOF err, and the transport is STDIO, we try to exec it
// again with a timeout and collect the output so we can add details to the
// error.
// this happens particularly when starting things with npx, e.g. if node can't
// be found or some other error like that.
func maybeStdioErr(err error, transport mcp.Transport) error {
	if !errors.Is(err, io.EOF) {
		return err
	}
	ct, ok := transport.(*mcp.CommandTransport)
	if !ok {
		return err
	}
	if err2 := stdioCheck(ct.Command); err2 != nil {
		err = errors.Join(err, err2)
	}
	return err
}

func maybeTimeoutErr(err error, timeout time.Duration) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("timed out after %s", timeout)
	}
	return err
}

func createTransport(ctx context.Context, name string, m config.MCPConfig, resolver config.VariableResolver) (mcp.Transport, *mcpoauth.Handler, error) {
	switch m.Type {
	case config.MCPStdio:
		command, err := resolver.ResolveValue(m.Command)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid mcp command: %w", err)
		}
		if strings.TrimSpace(command) == "" {
			return nil, nil, fmt.Errorf("mcp stdio config requires a non-empty 'command' field")
		}
		args, err := m.ResolvedArgs(resolver)
		if err != nil {
			return nil, nil, err
		}
		envs, err := m.ResolvedEnv(resolver)
		if err != nil {
			return nil, nil, err
		}
		cmd := exec.CommandContext(ctx, home.Long(command), args...)
		cmd.Env = append(os.Environ(), envs...)
		return &mcp.CommandTransport{
			Command: cmd,
		}, nil, nil
	case config.MCPHttp:
		url, err := m.ResolvedURL(resolver)
		if err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(url) == "" {
			return nil, nil, fmt.Errorf("mcp http config requires a non-empty 'url' field")
		}

		// OAuth-enabled HTTP transport.
		if m.OAuth {
			oauthHandler, oauthErr := mcpoauth.NewHandler(name, m.OAuthToken)
			if oauthErr != nil {
				return nil, nil, fmt.Errorf("failed to create OAuth handler for mcp %q: %w", name, oauthErr)
			}
			authURLs.Set(name, oauthHandler)
			return &mcp.StreamableClientTransport{
				Endpoint:     url,
				OAuthHandler: oauthHandler,
			}, oauthHandler, nil
		}

		headers, err := m.ResolvedHeaders(resolver)
		if err != nil {
			return nil, nil, err
		}
		client := &http.Client{
			Transport: &headerRoundTripper{
				headers: headers,
			},
		}
		return &mcp.StreamableClientTransport{
			Endpoint:   url,
			HTTPClient: client,
		}, nil, nil
	case config.MCPSSE:
		url, err := m.ResolvedURL(resolver)
		if err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(url) == "" {
			return nil, nil, fmt.Errorf("mcp sse config requires a non-empty 'url' field")
		}
		headers, err := m.ResolvedHeaders(resolver)
		if err != nil {
			return nil, nil, err
		}
		client := &http.Client{
			Transport: &headerRoundTripper{
				headers: headers,
			},
		}
		return &mcp.SSEClientTransport{
			Endpoint:   url,
			HTTPClient: client,
		}, nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported mcp type: %s", m.Type)
	}
}

type headerRoundTripper struct {
	headers map[string]string
}

func (rt headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range rt.headers {
		req.Header.Set(k, v)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func mcpTimeout(m config.MCPConfig) time.Duration {
	if m.Timeout > 0 {
		return time.Duration(m.Timeout) * time.Second
	}
	// OAuth flows require user interaction in a browser, so use a
	// generous default to avoid timing out mid-auth.
	if m.OAuth {
		return 5 * time.Minute
	}
	return 15 * time.Second
}

// clearMCPData removes a stale MCP server's tools, prompts,
// resources, and auth handlers from global state so they are not
// served to the agent.
func clearMCPData(name string) {
	allTools.Del(name)
	allPrompts.Del(name)
	allResources.Del(name)
	if h, ok := authURLs.Get(name); ok {
		h.Close()
		authURLs.Del(name)
	}
}

func stdioCheck(old *exec.Cmd) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	cmd := exec.CommandContext(ctx, old.Path, old.Args...)
	cmd.Env = old.Env
	out, err := cmd.CombinedOutput()
	if err == nil || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil
	}
	return fmt.Errorf("%w: %s", err, string(out))
}
