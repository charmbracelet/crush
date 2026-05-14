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
	"golang.org/x/oauth2"
)

// parseLevel converts an MCP logging level string to a slog.Level. The
// entire MCP logging feature is deprecated per SEP-2577 but remains
// functional; servers may still send log notifications during the
// deprecation window.
func parseLevel(level string) slog.Level {
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

	// initStarted records whether Initialize has been armed. WaitForInit only
	// blocks once initialization is expected; coordinators built outside app
	// startup never arm it and so must not wait forever.
	initMu      sync.Mutex
	initStarted bool

	// renewMus serializes lazy session renewals per server so concurrent tool
	// calls cannot race to rebuild the same session.
	renewMusMu sync.Mutex
	renewMus   = map[string]*sync.Mutex{}

	// newSession creates a client session. It is a seam so tests can exercise
	// renewal concurrency without spawning a real transport.
	newSession = createSession
)

// ArmInit marks that MCP initialization is expected so WaitForInit blocks
// until it completes. Call this synchronously before launching Initialize in a
// goroutine; otherwise WaitForInit could observe the not-yet-started state and
// return early, letting the tool list be read before MCP tools register.
func ArmInit() {
	initMu.Lock()
	initStarted = true
	initMu.Unlock()
}

// renewLock returns the per-server mutex used to serialize session renewals,
// creating it on first use.
func renewLock(name string) *sync.Mutex {
	renewMusMu.Lock()
	defer renewMusMu.Unlock()
	mu, ok := renewMus[name]
	if !ok {
		mu = &sync.Mutex{}
		renewMus[name] = mu
	}
	return mu
}

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
	// EventChannelMessage is published when a channel server pushes a
	// notifications/claude/channel event. ChannelMessage carries the rendered,
	// escaped <channel> element ready for injection into the session.
	EventChannelMessage
)

// Event represents an event in the MCP system
type Event struct {
	Type   EventType
	Name   string
	State  State
	Error  error
	Counts Counts
	// ChannelMessage is set only for EventChannelMessage: the fully rendered
	// and escaped <channel>...</channel> element to inject into the session.
	ChannelMessage string
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

// SubscribeEvents returns a channel for MCP events.
//
// Channel message events (EventChannelMessage) are excluded: they carry no
// workspace or session identity, and the MCP broker is process-global. Without
// this filter, every workspace that calls SubscribeEvents would receive every
// other workspace's channel events — a cross-workspace injection path. Channel
// delivery requires workspace-scoped routing, which is deferred to a later PR;
// until then, channel events must not flow through the shared event fan-out.
func SubscribeEvents(ctx context.Context) <-chan pubsub.Event[Event] {
	raw := broker.Subscribe(ctx)
	filtered := make(chan pubsub.Event[Event], 64)
	go func() {
		defer close(filtered)
		for ev := range raw {
			if ev.Payload.Type == EventChannelMessage {
				continue
			}
			select {
			case filtered <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return filtered
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
	ArmInit()
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

// WaitForInit blocks until MCP initialization is complete, i.e. until
// Initialize has finished and closed initDone. If initialization was never
// armed (ArmInit was not called, e.g. a coordinator built outside app
// startup), there is nothing to wait for and this returns nil immediately
// rather than blocking until ctx is cancelled.
func WaitForInit(ctx context.Context) error {
	initMu.Lock()
	started := initStarted
	initMu.Unlock()
	if !started {
		return nil
	}
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

	// This is the user-initiated flow, so permit the interactive browser
	// authorization the handler otherwise withholds during startup.
	ctx = mcpoauth.WithInteractive(ctx)

	// The OAuth handler persists the token automatically as it is
	// exchanged, so a successful connection has already saved it.
	_, err := connectAndRegister(ctx, cfg, name, m, cfg.Resolver(), channelEnabled(cfg.Overrides().EnabledChannels, name))
	if err != nil {
		return err
	}
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
	// OAuth MCPs without a usable cached token require user interaction
	// (browser auth). If a cached token exists with an access token
	// (even if expired), try connecting first so the SDK can attempt a
	// silent refresh. Only defer to the UI if no token is available at
	// all or the token is structurally invalid (empty access token).
	if m.OAuth && m.Type == config.MCPHttp && !hasUsableToken(m.OAuthToken) {
		if m.OAuthToken != nil {
			clearOAuthToken(cfg, name)
		}
		updateState(name, StateNeedsAuth, nil, nil, Counts{})
		clearMCPData(name)
		slog.Info("MCP server requires OAuth authentication", "name", name)
		return nil
	}

	updateState(name, StateStarting, nil, nil, Counts{})
	_, err := connectAndRegister(ctx, cfg, name, m, resolver, channelEnabled(cfg.Overrides().EnabledChannels, name))
	if err != nil {
		// If an OAuth MCP fails because the saved token is no longer
		// valid (e.g. refresh token expired or revoked) or no token
		// could be obtained, clear the stale token and prompt the user
		// to re-authenticate instead of leaving the server stuck in
		// StateError.
		if m.OAuth && m.Type == config.MCPHttp && isOAuthInitErr(err) {
			if m.OAuthToken != nil {
				clearOAuthToken(cfg, name)
			}
			updateState(name, StateNeedsAuth, nil, nil, Counts{})
			slog.Info("MCP OAuth token is no longer valid, re-authentication required", "name", name, "error", err)
			return nil
		}
		return err
	}
	return nil
}

// connectAndRegister creates a session, lists tools and prompts,
// registers them in global state, and transitions to StateConnected.
// Returns the session so callers can perform post-processing (e.g.
// token persistence).
func connectAndRegister(ctx context.Context, cfg *config.ConfigStore, name string, m config.MCPConfig, resolver config.VariableResolver, channelOptIn bool) (*ClientSession, error) {
	session, err := createSession(ctx, cfg, name, m, resolver, channelOptIn)
	if err != nil {
		return nil, err
	}

	toolCount, err := registerSessionTools(ctx, cfg, name, session)
	if err != nil {
		slog.Error("Error listing tools", "error", err)
		updateState(name, StateError, err, nil, Counts{})
		closeSession(name, session)
		return nil, err
	}

	prompts, err := getPrompts(ctx, session)
	if err != nil {
		slog.Error("Error listing prompts", "error", err)
		updateState(name, StateError, err, nil, Counts{})
		closeSession(name, session)
		return nil, err
	}

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

// DisableSingle disables and closes a single MCP client by name.
func DisableSingle(cfg *config.ConfigStore, name string) error {
	if session, ok := sessions.Take(name); ok {
		closeSession(name, session)
	}

	// Clear tools, prompts, resources, and auth state for this MCP.
	clearMCPData(name)

	// Update state to disabled.
	updateState(name, StateDisabled, nil, nil, Counts{})

	slog.Info("Disabled mcp client", "name", name)
	return nil
}

func getOrRenewClient(ctx context.Context, cfg *config.ConfigStore, name string) (*ClientSession, error) {
	m := cfg.Config().MCP[name]
	timeout := mcpTimeout(m)

	// Fast path: reuse a healthy session without taking the renewal lock.
	if sess, ok := sessions.Get(name); ok {
		if err := pingSession(ctx, sess, timeout); err == nil {
			return sess, nil
		}
	}

	// Serialize renewals per server. Two concurrent tool calls can both
	// observe a dead session and race to rebuild it: one may close the
	// session the other just registered, or overwrite and leak a live
	// replacement. Under this lock only the first arrival rebuilds; later
	// arrivals re-check and reuse the healthy result.
	mu := renewLock(name)
	mu.Lock()
	defer mu.Unlock()

	// Under the lock the map is stable: any in-flight renewal has finished and
	// either re-registered its session or failed and left none. A renewal
	// removes the session transiently (StateError takes it before rebuilding),
	// so this check must happen here rather than before the lock — otherwise a
	// caller arriving mid-renewal sees no session and wrongly reports the
	// server unavailable.
	sess, ok := sessions.Get(name)
	if !ok {
		return nil, fmt.Errorf("mcp '%s' not available", name)
	}

	// A concurrent goroutine may have already renewed the session while we
	// waited for the lock. Reuse it if it is now healthy.
	pingErr := pingSession(ctx, sess, timeout)
	if pingErr == nil {
		return sess, nil
	}

	state, _ := states.Get(name)
	// StateError closes the dead session and clears its tools, prompts, and
	// resources from the registry.
	updateState(name, StateError, maybeTimeoutErr(pingErr, timeout), nil, state.Counts)

	newSess, err := newSession(ctx, cfg, name, m, cfg.Resolver(), channelEnabled(cfg.Overrides().EnabledChannels, name))
	if err != nil {
		clearMCPData(name)
		// If an OAuth MCP fails to reconnect because the token is no
		// longer valid, clear the stale token and prompt the user to
		// re-authenticate instead of leaving it in an error state.
		if m.OAuth && m.Type == config.MCPHttp {
			if m.OAuthToken != nil && isOAuthInitErr(err) {
				clearOAuthToken(cfg, name)
			}
			updateState(name, StateNeedsAuth, nil, nil, Counts{})
			slog.Info("MCP OAuth session expired, re-authentication required", "name", name, "error", err)
		}
		return nil, err
	}

	// StateError cleared this server's tools, prompts, and resources from the
	// registry. Re-list and re-register them all on the fresh session and
	// recompute the counts from what actually registered; otherwise the agent
	// reconnects but the registries stay empty (the next tool call fails with
	// "tool not found") while the reported counts still advertise capabilities
	// that are no longer there.
	var counts Counts
	counts.Tools, err = registerSessionTools(ctx, cfg, name, newSess)
	if err != nil {
		updateState(name, StateError, err, nil, Counts{})
		closeSession(name, newSess)
		return nil, err
	}

	prompts, err := getPrompts(ctx, newSess)
	if err != nil {
		updateState(name, StateError, err, nil, Counts{})
		closeSession(name, newSess)
		return nil, err
	}
	updatePrompts(name, prompts)
	counts.Prompts = len(prompts)

	resources, err := getResources(ctx, newSess)
	if err != nil {
		updateState(name, StateError, err, nil, Counts{})
		closeSession(name, newSess)
		return nil, err
	}
	counts.Resources = updateResources(name, resources)

	sessions.Set(name, newSess)
	updateState(name, StateConnected, nil, newSess, counts)
	return newSess, nil
}

// pingSession pings a session with the server's configured timeout.
func pingSession(ctx context.Context, s *ClientSession, timeout time.Duration) error {
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.Ping(pingCtx, nil)
}

// closeSession closes an MCP session, logging only unexpected errors. EOF,
// context cancellation, and a killed child are the ordinary result of tearing
// a session down and are not worth surfacing.
func closeSession(name string, s *ClientSession) {
	if err := s.Close(); err != nil &&
		!errors.Is(err, io.EOF) &&
		!errors.Is(err, context.Canceled) &&
		err.Error() != "signal: killed" {
		slog.Warn("Error closing MCP session", "name", name, "error", err)
	}
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
		// A session that has errored is dead to us. Atomically remove it and
		// close it so the child process and its stdio pipes are released — the
		// bare map delete this used to do leaked both. Clearing the tool
		// registry keeps the agent from advertising tools it can no longer
		// call: without it, crush_info / the `/mcp` menu and the tool list
		// handed to the LLM diverge, so a server still reads "connected, N
		// tools" while every call fails with "tool not found".
		if old, ok := sessions.Take(name); ok {
			closeSession(name, old)
		}
		// Drop every registry entry for the dead server. Leaving prompts or
		// resources behind lets a disconnected server keep advertising
		// capabilities the agent can no longer fulfil, the same divergence the
		// tool clear prevents.
		allTools.Del(name)
		allPrompts.Del(name)
		allResources.Del(name)
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

func createSession(ctx context.Context, cfg *config.ConfigStore, name string, m config.MCPConfig, resolver config.VariableResolver, channelOptIn bool) (*ClientSession, error) {
	timeout := mcpTimeout(m)
	mcpCtx, cancel := context.WithCancel(ctx)
	cancelTimer := time.AfterFunc(timeout, cancel)

	transport, oauthHandler, err := createTransport(mcpCtx, cfg, name, m, resolver)
	if err != nil {
		updateState(name, StateError, err, nil, Counts{})
		slog.Error("Error creating MCP client", "error", err, "name", name)
		cancel()
		cancelTimer.Stop()
		return nil, err
	}

	// Wrap the transport so channel notifications can be intercepted. The
	// gate starts undecided: notifications that arrive during capability
	// negotiation are buffered. After Connect resolves, the gate is opened
	// (and the buffer drained) only when the server declares the channel
	// capability AND was opted in via --channels; otherwise it is closed
	// (buffer discarded). This prevents early notifications from being lost.
	channelGate := newChannelGate()
	transport = &channelTransport{inner: transport, name: name, gate: channelGate}

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
				level := parseLevel(string(req.Params.Level))
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

	// Resolve the channel gate: open only for a server that both declares
	// the claude/channel capability and was opted in via --channels.
	// Otherwise close it (fail closed). Resolving drains buffered messages
	// that arrived during negotiation so a fast server does not lose early
	// events.
	if channelOptIn && hasChannelCapability(session.InitializeResult()) {
		buffered := channelGate.resolve(true)
		for _, raw := range buffered {
			publishChannelMessage(mcpCtx, name, raw)
		}
		slog.Info("MCP channel enabled", "name", name, "buffered", len(buffered))
	} else {
		channelGate.resolve(false)
	}

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

func createTransport(ctx context.Context, cfg *config.ConfigStore, name string, m config.MCPConfig, resolver config.VariableResolver) (mcp.Transport, *mcpoauth.Handler, error) {
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
		// Run the child in its own process group and kill the whole group when
		// the session context is cancelled. A stdio server often spawns its own
		// children (signal-mcp launches signal-cli); os/exec's default
		// cancellation kills only the direct child, orphaning the rest with
		// PPID 1 — production accumulated 15+ such zombies over two days.
		configureStdioProcess(cmd)
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

		// OAuth-enabled HTTP transport. The handler persists the token
		// (and the client registration/endpoints needed to refresh it)
		// on every exchange and refresh via this saver.
		if m.OAuth {
			tokenSaver := func(tok *oauth.Token) {
				if err := cfg.SetConfigField(config.ScopeGlobal, fmt.Sprintf("mcp.%s.oauth_token", name), tok); err != nil {
					slog.Warn("Failed to persist MCP OAuth token", "name", name, "error", err)
				} else {
					slog.Info("Persisted MCP OAuth token", "name", name)
				}
			}

			// A pre-registered client is required for servers that do not
			// support dynamic client registration (e.g. GitHub, Slack).
			// Resolve the credentials through the shell like other config
			// values so $VAR and $(cmd) work.
			var preregistered *oauth.OAuthClient
			if strings.TrimSpace(m.OAuthClientID) != "" {
				clientID, err := resolver.ResolveValue(m.OAuthClientID)
				if err != nil {
					return nil, nil, fmt.Errorf("oauth_client_id: %w", err)
				}
				clientSecret, err := resolver.ResolveValue(m.OAuthClientSecret)
				if err != nil {
					return nil, nil, fmt.Errorf("oauth_client_secret: %w", err)
				}
				preregistered = &oauth.OAuthClient{
					ClientID:     strings.TrimSpace(clientID),
					ClientSecret: strings.TrimSpace(clientSecret),
				}
			}

			oauthHandler, oauthErr := mcpoauth.NewHandler(name, url, m.OAuthToken, preregistered, tokenSaver, mcpoauth.IsInteractive(ctx), m.OAuthCallbackPort)
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

// hasUsableToken returns true if the saved OAuth token has an access
// token that can be used or refreshed. A token with an empty access
// token is structurally invalid and should be treated as missing.
func hasUsableToken(tok *oauth.Token) bool {
	return tok != nil && tok.AccessToken != ""
}

// isOAuthInitErr returns true if the error indicates the OAuth token
// is missing, no longer valid, or cannot be refreshed. This covers:
//   - invalid_grant: expired or revoked refresh tokens
//   - invalid_client: deleted or deactivated client registrations
//   - "no token available": the handler had no cached token to use
//   - interactive authorization was required but withheld during startup
func isOAuthInitErr(err error) bool {
	if errors.Is(err, mcpoauth.ErrInteractiveAuthRequired) {
		return true
	}
	var rErr *oauth2.RetrieveError
	if errors.As(err, &rErr) {
		return rErr.ErrorCode == "invalid_grant" || rErr.ErrorCode == "invalid_client"
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid_grant") ||
		strings.Contains(msg, "invalid_client") ||
		strings.Contains(msg, "no token available")
}

// clearOAuthToken removes the persisted OAuth token for a named MCP
// server from the global config so subsequent startups don't retry
// with a known-bad refresh token.
func clearOAuthToken(cfg *config.ConfigStore, name string) {
	key := fmt.Sprintf("mcp.%s.oauth_token", name)
	if err := cfg.RemoveConfigField(config.ScopeGlobal, key); err != nil {
		slog.Warn("Failed to clear stale MCP OAuth token", "name", name, "error", err)
	}
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
