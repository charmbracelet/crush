package cmd

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	fang "charm.land/fang/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/event"
	crushlog "github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/projects"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	ui "github.com/charmbracelet/crush/internal/ui/model"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
	xstrings "github.com/charmbracelet/x/exp/strings"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var clientHost string

func init() {
	rootCmd.PersistentFlags().StringP("cwd", "c", "", "Current working directory")
	rootCmd.PersistentFlags().StringP("data-dir", "D", "", "Custom crush data directory")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Debug")
	rootCmd.PersistentFlags().StringVarP(&clientHost, "host", "H", server.DefaultHost(), "Connect to a specific crush server host (for advanced users)")
	rootCmd.Flags().BoolP("help", "h", false, "Help")
	rootCmd.Flags().BoolP("yolo", "y", false, "Automatically accept all permissions (dangerous mode)")
	rootCmd.Flags().StringP("session", "s", "", "Continue a previous session by ID")
	rootCmd.Flags().BoolP("continue", "C", false, "Continue the most recent session")
	rootCmd.MarkFlagsMutuallyExclusive("session", "continue")

	rootCmd.AddCommand(
		runCmd,
		dirsCmd,
		projectsCmd,
		updateProvidersCmd,
		logsCmd,
		schemaCmd,
		loginCmd,
		statsCmd,
		sessionCmd,
	)
}

var rootCmd = &cobra.Command{
	Use:   "crush",
	Short: "A terminal-first AI assistant for software development",
	Long:  "A glamorous, terminal-first AI assistant for software development and adjacent tasks",
	Example: `
# Run in interactive mode
crush

# Run non-interactively
crush run "Guess my 5 favorite Pokémon"

# Run a non-interactively with pipes and redirection
cat README.md | crush run "make this more glamorous" > GLAMOROUS_README.md

# Run with debug logging in a specific directory
crush --debug --cwd /path/to/project

# Run in yolo mode (auto-accept all permissions; use with care)
crush --yolo

# Run with custom data directory
crush --data-dir /path/to/custom/.crush

# Continue a previous session
crush --session {session-id}

# Continue the most recent session
crush --continue
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session")
		continueLast, _ := cmd.Flags().GetBool("continue")

		ws, cleanup, err := setupWorkspaceWithProgressBar(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		if sessionID != "" {
			sess, err := resolveWorkspaceSessionID(cmd.Context(), ws, sessionID)
			if err != nil {
				return err
			}
			sessionID = sess.ID
		}

		event.AppInitialized()

		com := common.DefaultCommon(ws)
		model := ui.New(com, sessionID, continueLast)

		var env uv.Environ = os.Environ()
		program := tea.NewProgram(
			model,
			tea.WithEnvironment(env),
			tea.WithContext(cmd.Context()),
			tea.WithFilter(ui.MouseEventFilter),
		)
		go ws.Subscribe(program)

		if _, err := program.Run(); err != nil {
			event.Error(err)
			slog.Error("TUI run error", "error", err)
			return errors.New("Crush crashed. If metrics are enabled, we were notified about it. If you'd like to report it, please copy the stacktrace above and open an issue at https://github.com/charmbracelet/crush/issues/new?template=bug.yml") //nolint:staticcheck
		}
		return nil
	},
}

var heartbit = lipgloss.NewStyle().Foreground(charmtone.Dolly).SetString(`
    ▄▄▄▄▄▄▄▄    ▄▄▄▄▄▄▄▄
  ███████████  ███████████
████████████████████████████
████████████████████████████
██████████▀██████▀██████████
██████████ ██████ ██████████
▀▀██████▄████▄▄████▄██████▀▀
  ████████████████████████
    ████████████████████
       ▀▀██████████▀▀
           ▀▀▀▀▀▀
`)

// copied from cobra:
const defaultVersionTemplate = `{{with .DisplayName}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
`

func Execute() {
	// FIXME: config.Load uses slog internally during provider resolution,
	// but the file-based logger isn't set up until after config is loaded
	// (because the log path depends on the data directory from config).
	// This creates a window where slog calls in config.Load leak to
	// stderr. We discard early logs here as a workaround. The proper
	// fix is to remove slog calls from config.Load and have it return
	// warnings/diagnostics instead of logging them as a side effect.
	slog.SetDefault(slog.New(slog.DiscardHandler))

	// NOTE: very hacky: we create a colorprofile writer with STDOUT, then make
	// it forward to a bytes.Buffer, write the colored heartbit to it, and then
	// finally prepend it in the version template.
	// Unfortunately cobra doesn't give us a way to set a function to handle
	// printing the version, and PreRunE runs after the version is already
	// handled, so that doesn't work either.
	// This is the only way I could find that works relatively well.
	if term.IsTerminal(os.Stdout.Fd()) {
		var b bytes.Buffer
		w := colorprofile.NewWriter(os.Stdout, os.Environ())
		w.Forward = &b
		_, _ = w.WriteString(heartbit.String())
		rootCmd.SetVersionTemplate(b.String() + "\n" + defaultVersionTemplate)
	}
	if err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithVersion(version.Version),
		fang.WithNotifySignal(os.Interrupt),
	); err != nil {
		os.Exit(1)
	}
}

// supportsProgressBar tries to determine whether the current terminal supports
// progress bars by looking into environment variables.
func supportsProgressBar() bool {
	if !term.IsTerminal(os.Stderr.Fd()) {
		return false
	}
	termProg := os.Getenv("TERM_PROGRAM")
	_, isWindowsTerminal := os.LookupEnv("WT_SESSION")

	return isWindowsTerminal || xstrings.ContainsAnyOf(strings.ToLower(termProg), "ghostty", "iterm2", "rio")
}

// useClientServer returns true when the client/server architecture is
// enabled via the CRUSH_CLIENT_SERVER environment variable.
func useClientServer() bool {
	v, _ := strconv.ParseBool(os.Getenv("CRUSH_CLIENT_SERVER"))
	return v
}

// setupWorkspaceWithProgressBar wraps setupWorkspace with an optional
// terminal progress bar shown during initialization.
func setupWorkspaceWithProgressBar(cmd *cobra.Command) (workspace.Workspace, func(), error) {
	showProgress := supportsProgressBar()
	if showProgress {
		_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
	}

	ws, cleanup, err := setupWorkspace(cmd)

	if showProgress {
		_, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar)
	}

	return ws, cleanup, err
}

// setupWorkspace returns a Workspace and cleanup function. When
// CRUSH_CLIENT_SERVER=1, it connects to a server process and returns a
// ClientWorkspace. Otherwise it creates an in-process app.App and
// returns an AppWorkspace.
func setupWorkspace(cmd *cobra.Command) (workspace.Workspace, func(), error) {
	if useClientServer() {
		return setupClientServerWorkspace(cmd)
	}
	return setupLocalWorkspace(cmd)
}

// setupLocalWorkspace creates an in-process app.App and wraps it in an
// AppWorkspace.
func setupLocalWorkspace(cmd *cobra.Command) (workspace.Workspace, func(), error) {
	debug, _ := cmd.Flags().GetBool("debug")
	yolo, _ := cmd.Flags().GetBool("yolo")
	dataDir, _ := cmd.Flags().GetString("data-dir")
	ctx := cmd.Context()

	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return nil, nil, err
	}

	store, err := config.Init(cwd, dataDir, debug)
	if err != nil {
		return nil, nil, err
	}

	cfg := store.Config()
	store.Overrides().SkipPermissionRequests = yolo

	if err := os.MkdirAll(cfg.Options.DataDirectory, 0o700); err != nil {
		return nil, nil, fmt.Errorf("failed to create data directory: %q %w", cfg.Options.DataDirectory, err)
	}

	gitIgnorePath := filepath.Join(cfg.Options.DataDirectory, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitIgnorePath, []byte("*\n"), 0o644); err != nil {
			return nil, nil, fmt.Errorf("failed to create .gitignore file: %q %w", gitIgnorePath, err)
		}
	}

	if err := projects.Register(cwd, cfg.Options.DataDirectory); err != nil {
		slog.Warn("Failed to register project", "error", err)
	}

	conn, err := db.Connect(ctx, cfg.Options.DataDirectory)
	if err != nil {
		return nil, nil, err
	}

	logFile := filepath.Join(cfg.Options.DataDirectory, "logs", "crush.log")
	crushlog.Setup(logFile, debug)

	appInstance, err := app.New(ctx, conn, store)
	if err != nil {
		_ = conn.Close()
		slog.Error("Failed to create app instance", "error", err)
		return nil, nil, err
	}

	if shouldEnableMetrics(cfg) {
		event.Init()
	}

	ws := workspace.NewAppWorkspace(appInstance, store)
	cleanup := func() { appInstance.Shutdown() }
	return ws, cleanup, nil
}

// setupClientServerWorkspace connects to a server process and wraps the
// result in a ClientWorkspace.
func setupClientServerWorkspace(cmd *cobra.Command) (workspace.Workspace, func(), error) {
	c, protoWs, cleanupServer, err := connectToServer(cmd)
	if err != nil {
		return nil, nil, err
	}

	clientWs := workspace.NewClientWorkspace(c, *protoWs)

	if protoWs.Config.IsConfigured() {
		if err := clientWs.InitCoderAgent(cmd.Context()); err != nil {
			slog.Error("Failed to initialize coder agent", "error", err)
		}
	}

	return clientWs, cleanupServer, nil
}

// connectToServer ensures the server is running, creates a client and
// workspace, and returns a cleanup function that deletes the workspace.
func connectToServer(cmd *cobra.Command) (*client.Client, *proto.Workspace, func(), error) {
	hostURL, err := server.ParseHostURL(clientHost)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid host URL: %v", err)
	}

	if err := ensureServer(cmd, hostURL); err != nil {
		return nil, nil, nil, err
	}

	debug, _ := cmd.Flags().GetBool("debug")
	yolo, _ := cmd.Flags().GetBool("yolo")
	dataDir, _ := cmd.Flags().GetString("data-dir")
	ctx := cmd.Context()

	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return nil, nil, nil, err
	}

	c, err := client.NewClient(cwd, hostURL.Scheme, hostURL.Host)
	if err != nil {
		return nil, nil, nil, err
	}

	wsReq := proto.Workspace{
		Path:    cwd,
		DataDir: dataDir,
		Debug:   debug,
		YOLO:    yolo,
		Version: version.Version,
		Env:     os.Environ(),
	}

	ws, err := c.CreateWorkspace(ctx, wsReq)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create workspace: %v", err)
	}

	if shouldEnableMetrics(ws.Config) {
		event.Init()
	}

	if ws.Config != nil {
		logFile := filepath.Join(ws.Config.Options.DataDirectory, "logs", "crush.log")
		crushlog.Setup(logFile, debug)
	}

	cleanup := func() { _ = c.DeleteWorkspace(context.Background(), ws.ID) }
	return c, ws, cleanup, nil
}

// ensureServer auto-starts a detached server if the socket file does not
// exist. When the socket exists, it verifies that the running server
// version matches the client; on mismatch it shuts down the old server
// and starts a fresh one.
func ensureServer(cmd *cobra.Command, hostURL *url.URL) error {
	switch hostURL.Scheme {
	case "unix", "npipe":
		needsStart := false
		_, statErr := os.Stat(hostURL.Host)
		switch {
		case statErr == nil:
			restarted, err := restartIfStale(cmd, hostURL)
			if err != nil {
				slog.Warn("Failed to check server version", "error", err)
			}
			needsStart = restarted || err != nil
		case errors.Is(statErr, fs.ErrNotExist):
			needsStart = true
		default:
			slog.Warn("Unexpected error stat'ing server socket, attempting cleanup",
				"path", hostURL.Host, "error", statErr)
			if err := os.Remove(hostURL.Host); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("failed to remove stale server socket %q: %v", hostURL.Host, err)
			}
			needsStart = true
		}

		if needsStart {
			if err := startDetachedServer(cmd); err != nil {
				return err
			}
		}

		if err := waitForServerReady(cmd.Context(), hostURL); err != nil {
			return fmt.Errorf("failed to initialize crush server: %v", err)
		}
	}

	return nil
}

// serverReadyTimeout returns the total budget for the readiness probe.
// Overridable via CRUSH_SERVER_READY_TIMEOUT (parsed as a Go duration).
func serverReadyTimeout() time.Duration {
	const def = 10 * time.Second
	v := os.Getenv("CRUSH_SERVER_READY_TIMEOUT")
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// waitForServerReady polls GET /v1/health until the server responds with
// any 2xx status or the total timeout elapses. Each attempt uses a short
// per-attempt timeout so a hung listener doesn't burn the whole budget.
//
// The HTTP transport is built to mirror how *client.Client dials so the
// same unix socket / npipe / tcp setups all work uniformly here.
func waitForServerReady(ctx context.Context, hostURL *url.URL) error {
	httpClient, reqURL, err := readinessHTTPClient(hostURL)
	if err != nil {
		return err
	}

	const perAttempt = 100 * time.Millisecond
	deadline := time.Now().Add(serverReadyTimeout())

	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return fmt.Errorf("timed out waiting for server readiness")
		}

		attemptCtx, cancel := context.WithTimeout(ctx, perAttempt)
		err := probeHealth(attemptCtx, httpClient, reqURL, hostURL)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(perAttempt):
		}
	}
}

// readinessHTTPClient builds an *http.Client whose transport dials the
// server using the same scheme-aware logic as *client.Client (unix
// socket, named pipe, or tcp).
func readinessHTTPClient(hostURL *url.URL) (*http.Client, string, error) {
	c, err := client.NewClient("", hostURL.Scheme, hostURL.Host)
	if err != nil {
		return nil, "", err
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return c.Dial(ctx, network, addr)
	}
	if hostURL.Scheme == "unix" || hostURL.Scheme == "npipe" {
		tr.DisableCompression = true
	}

	httpClient := &http.Client{Transport: tr}

	// For unix sockets / named pipes we still need a syntactically valid
	// HTTP URL; the actual address is resolved by the dialer.
	host := hostURL.Host
	if hostURL.Scheme == "unix" || hostURL.Scheme == "npipe" {
		host = client.DummyHost
	}
	reqURL := (&url.URL{Scheme: "http", Host: host, Path: "/v1/health"}).String()
	return httpClient, reqURL, nil
}

// probeHealth issues a single GET to the readiness endpoint and treats
// any 2xx response as success.
func probeHealth(ctx context.Context, h *http.Client, reqURL string, hostURL *url.URL) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	if hostURL.Scheme == "unix" || hostURL.Scheme == "npipe" {
		req.Host = client.DummyHost
	}
	rsp, err := h.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	_, _ = io.Copy(io.Discard, rsp.Body)
	if rsp.StatusCode < 200 || rsp.StatusCode >= 300 {
		return fmt.Errorf("server health check failed: %s", rsp.Status)
	}
	return nil
}

// restartIfStale checks whether the running server matches the current
// client version. When they differ, it sends a shutdown command and
// removes the stale socket so the caller can start a fresh server.
//
// It returns restarted=true when it has shut down a stale server and the
// caller must spawn a new one. When the server matches the client version
// (or the check itself fails), restarted is false.
func restartIfStale(cmd *cobra.Command, hostURL *url.URL) (restarted bool, err error) {
	c, err := client.NewClient("", hostURL.Scheme, hostURL.Host)
	if err != nil {
		return false, err
	}
	vi, err := c.VersionInfo(cmd.Context())
	if err != nil {
		return false, err
	}
	if vi.Version == version.Version {
		return false, nil
	}
	slog.Info("Server version mismatch, restarting",
		"server", vi.Version,
		"client", version.Version,
	)
	_ = c.ShutdownServer(cmd.Context())
	// Give the old process a moment to release the socket.
	for range 20 {
		if _, err := os.Stat(hostURL.Host); errors.Is(err, fs.ErrNotExist) {
			break
		}
		select {
		case <-cmd.Context().Done():
			return true, cmd.Context().Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	// Force-remove if the socket is still lingering.
	_ = os.Remove(hostURL.Host)
	return true, nil
}

var safeNameRegexp = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func startDetachedServer(cmd *cobra.Command) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	safeClientHost := safeNameRegexp.ReplaceAllString(clientHost, "_")
	chDir := filepath.Join(config.GlobalCacheDir(), "server-"+safeClientHost)
	if err := os.MkdirAll(chDir, 0o700); err != nil {
		return fmt.Errorf("failed to create server working directory: %v", err)
	}

	cmdArgs := []string{"server"}
	if clientHost != server.DefaultHost() {
		cmdArgs = append(cmdArgs, "--host", clientHost)
	}

	// Use exec.Command (not exec.CommandContext) so the parent's context
	// cancellation does not kill the spawned server. detachProcess
	// (Setsid on !windows, DETACHED_PROCESS on windows) is what truly
	// detaches the child from this process's lifetime.
	c := exec.Command(exe, cmdArgs...)
	stdoutPath := filepath.Join(chDir, "stdout.log")
	stderrPath := filepath.Join(chDir, "stderr.log")
	detachProcess(c)

	stdout, err := os.Create(stdoutPath)
	if err != nil {
		return fmt.Errorf("failed to create stdout log file: %v", err)
	}
	defer stdout.Close()
	c.Stdout = stdout

	stderr, err := os.Create(stderrPath)
	if err != nil {
		return fmt.Errorf("failed to create stderr log file: %v", err)
	}
	defer stderr.Close()
	c.Stderr = stderr

	if err := c.Start(); err != nil {
		return fmt.Errorf("failed to start crush server: %v", err)
	}

	if err := c.Process.Release(); err != nil {
		return fmt.Errorf("failed to detach crush server process: %v", err)
	}

	return nil
}

func shouldEnableMetrics(cfg *config.Config) bool {
	if v, _ := strconv.ParseBool(os.Getenv("CRUSH_DISABLE_METRICS")); v {
		return false
	}
	if v, _ := strconv.ParseBool(os.Getenv("DO_NOT_TRACK")); v {
		return false
	}
	if cfg.Options.DisableMetrics {
		return false
	}
	return true
}

func MaybePrependStdin(prompt string) (string, error) {
	if term.IsTerminal(os.Stdin.Fd()) {
		return prompt, nil
	}
	fi, err := os.Stdin.Stat()
	if err != nil {
		return prompt, err
	}
	// Check if stdin is a named pipe ( | ) or regular file ( < ).
	if fi.Mode()&os.ModeNamedPipe == 0 && !fi.Mode().IsRegular() {
		return prompt, nil
	}
	bts, err := io.ReadAll(os.Stdin)
	if err != nil {
		return prompt, err
	}
	return string(bts) + "\n\n" + prompt, nil
}

// resolveWorkspaceSessionID resolves a session ID that may be a full
// UUID, full hash, or hash prefix. Works against the Workspace
// interface so both local and client/server paths get hash prefix
// support.
func resolveWorkspaceSessionID(ctx context.Context, ws workspace.Workspace, id string) (session.Session, error) {
	if sess, err := ws.GetSession(ctx, id); err == nil {
		return sess, nil
	}

	sessions, err := ws.ListSessions(ctx)
	if err != nil {
		return session.Session{}, err
	}

	var matches []session.Session
	for _, s := range sessions {
		hash := session.HashID(s.ID)
		if hash == id || strings.HasPrefix(hash, id) {
			matches = append(matches, s)
		}
	}

	switch len(matches) {
	case 0:
		return session.Session{}, fmt.Errorf("session not found: %s", id)
	case 1:
		return matches[0], nil
	default:
		return session.Session{}, fmt.Errorf("session ID %q is ambiguous (%d matches)", id, len(matches))
	}
}

func ResolveCwd(cmd *cobra.Command) (string, error) {
	cwd, _ := cmd.Flags().GetString("cwd")
	if cwd != "" {
		err := os.Chdir(cwd)
		if err != nil {
			return "", fmt.Errorf("failed to change directory: %v", err)
		}
		return cwd, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %v", err)
	}
	return cwd, nil
}

func createDotCrushDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create data directory: %q %w", dir, err)
	}

	gitIgnorePath := filepath.Join(dir, ".gitignore")
	content, err := os.ReadFile(gitIgnorePath)

	// create or update if old version
	if os.IsNotExist(err) || string(content) == oldGitIgnore {
		if err := os.WriteFile(gitIgnorePath, []byte(defaultGitIgnore), 0o644); err != nil {
			return fmt.Errorf("failed to create .gitignore file: %q %w", gitIgnorePath, err)
		}
	}

	return nil
}

//go:embed gitignore/old
var oldGitIgnore string

//go:embed gitignore/default
var defaultGitIgnore string
