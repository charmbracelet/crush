// Package backend provides transport-agnostic operations for managing
// workspaces, sessions, agents, permissions, and events. It is consumed
// by protocol-specific layers such as HTTP (server) and ACP.
package backend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"

	"github.com/taigrr/crush/internal/app"
	"github.com/taigrr/crush/internal/config"
	"github.com/taigrr/crush/internal/csync"
	"github.com/taigrr/crush/internal/db"
	"github.com/taigrr/crush/internal/proto"
	"github.com/taigrr/crush/internal/ui/util"
	"github.com/taigrr/crush/internal/version"
	"github.com/google/uuid"
)

// Common errors returned by backend operations.
var (
	ErrWorkspaceNotFound       = errors.New("workspace not found")
	ErrLSPClientNotFound       = errors.New("LSP client not found")
	ErrAgentNotInitialized     = errors.New("agent coordinator not initialized")
	ErrPathRequired            = errors.New("path is required")
	ErrInvalidPermissionAction = errors.New("invalid permission action")
	ErrUnknownCommand          = errors.New("unknown command")
)

// ShutdownFunc is called when the backend needs to trigger a server
// shutdown (e.g. when the last workspace is removed).
type ShutdownFunc func()

// Backend provides transport-agnostic business logic for the Crush
// server. It manages workspaces and delegates to [app.App] services.
type Backend struct {
	workspaces *csync.Map[string, *Workspace]
	clients    *clientTracker
	cfg        *config.ConfigStore
	ctx        context.Context
	shutdownFn ShutdownFunc
}

// Workspace represents a running [app.App] workspace with its
// associated resources and state.
type Workspace struct {
	*app.App
	ID   string
	Path string
	Cfg  *config.ConfigStore
	Env  []string
}

// New creates a new [Backend].
func New(ctx context.Context, cfg *config.ConfigStore, shutdownFn ShutdownFunc) *Backend {
	return &Backend{
		workspaces: csync.NewMap[string, *Workspace](),
		clients:    newClientTracker(),
		cfg:        cfg,
		ctx:        ctx,
		shutdownFn: shutdownFn,
	}
}

// GetWorkspace retrieves a workspace by ID. The ID can be either a workspace ID
// or a client ID that maps to a shared workspace.
func (b *Backend) GetWorkspace(id string) (*Workspace, error) {
	// Try direct workspace lookup first.
	if ws, ok := b.workspaces.Get(id); ok {
		return ws, nil
	}

	// Try as a client ID.
	if workspaceID, ok := b.clients.workspaceForClient(id); ok {
		if ws, ok := b.workspaces.Get(workspaceID); ok {
			return ws, nil
		}
	}

	return nil, ErrWorkspaceNotFound
}

// ListWorkspaces returns all running workspaces.
func (b *Backend) ListWorkspaces() []proto.Workspace {
	workspaces := []proto.Workspace{}
	for _, ws := range b.workspaces.Seq2() {
		workspaces = append(workspaces, workspaceToProto(ws))
	}
	return workspaces
}

// CreateWorkspace initializes a new workspace from the given parameters.
// If a workspace already exists for the same data directory, the existing
// workspace is returned with a new client ID to avoid opening multiple
// DB connections to the same file.
func (b *Backend) CreateWorkspace(args proto.Workspace) (*Workspace, proto.Workspace, error) {
	if args.Path == "" {
		return nil, proto.Workspace{}, ErrPathRequired
	}

	clientID := uuid.New().String()

	cfg, err := config.Init(args.Path, args.DataDir, args.Debug)
	if err != nil {
		return nil, proto.Workspace{}, fmt.Errorf("failed to initialize config: %w", err)
	}

	dataDir := cfg.Config().Options.DataDirectory

	// Check for existing workspace.
	if existingID, ok := b.clients.workspaceForDataDir(dataDir); ok {
		if ws, ok := b.workspaces.Get(existingID); ok {
			count, _ := b.clients.addClient(clientID, existingID, dataDir)
			slog.Info(
				"Reusing existing workspace for data directory",
				"client_id", clientID,
				"workspace_id", existingID,
				"data_dir", dataDir,
				"client_count", count,
			)
			return ws, b.makeProtoWorkspace(clientID, args.Path, dataDir, args.Env, ws.Cfg), nil
		}
		// Stale entry - clean up.
		b.clients.cleanupStaleWorkspace(existingID, dataDir)
	}

	// Create new workspace.
	cfg.Overrides().SkipPermissionRequests = args.YOLO

	if err := createDotCrushDir(dataDir); err != nil {
		return nil, proto.Workspace{}, fmt.Errorf("failed to create data directory: %w", err)
	}

	conn, err := db.Connect(b.ctx, dataDir)
	if err != nil {
		return nil, proto.Workspace{}, fmt.Errorf("failed to connect to database: %w", err)
	}

	appWorkspace, err := app.New(b.ctx, conn, cfg)
	if err != nil {
		return nil, proto.Workspace{}, fmt.Errorf("failed to create app workspace: %w", err)
	}

	ws := &Workspace{
		App:  appWorkspace,
		ID:   clientID,
		Path: args.Path,
		Cfg:  cfg,
		Env:  args.Env,
	}

	b.workspaces.Set(clientID, ws)
	b.clients.addClient(clientID, clientID, dataDir)

	if args.Version != "" && args.Version != version.Version {
		slog.Warn(
			"Client/server version mismatch",
			"client", args.Version,
			"server", version.Version,
		)
		appWorkspace.SendEvent(util.NewWarnMsg(fmt.Sprintf(
			"Server version %q differs from client version %q. Consider restarting the server.",
			version.Version, args.Version,
		)))
	}

	return ws, b.makeProtoWorkspace(clientID, args.Path, dataDir, args.Env, cfg), nil
}

// DeleteWorkspace removes a client from its workspace. The workspace is only
// shut down when the last client disconnects.
func (b *Backend) DeleteWorkspace(clientID string) {
	workspaceID, _, lastClient, ok := b.clients.removeClient(clientID)
	if !ok {
		// Unknown client - try legacy direct lookup.
		if ws, ok := b.workspaces.Get(clientID); ok {
			ws.Shutdown()
			b.workspaces.Del(clientID)
		}
		b.maybeShutdown()
		return
	}

	slog.Info(
		"Client disconnected from workspace",
		"client_id", clientID,
		"workspace_id", workspaceID,
		"last_client", lastClient,
	)

	if !lastClient {
		return
	}

	if ws, ok := b.workspaces.Get(workspaceID); ok {
		ws.Shutdown()
	}
	b.workspaces.Del(workspaceID)
	b.maybeShutdown()
}

func (b *Backend) maybeShutdown() {
	if b.workspaces.Len() == 0 && b.shutdownFn != nil {
		slog.Info("Last workspace removed, shutting down server...")
		b.shutdownFn()
	}
}

func (b *Backend) makeProtoWorkspace(clientID, path, dataDir string, env []string, cfg *config.ConfigStore) proto.Workspace {
	c := cfg.Config()
	return proto.Workspace{
		ID:        clientID,
		Path:      path,
		GitBranch: getGitBranch(path),
		DataDir:   dataDir,
		Debug:     c.Options.Debug,
		YOLO:      cfg.Overrides().SkipPermissionRequests,
		Config:    c,
		Env:       env,
	}
}

// GetWorkspaceProto returns the proto representation of a workspace.
func (b *Backend) GetWorkspaceProto(id string) (proto.Workspace, error) {
	ws, err := b.GetWorkspace(id)
	if err != nil {
		return proto.Workspace{}, err
	}
	return workspaceToProto(ws), nil
}

// VersionInfo returns server version information.
func (b *Backend) VersionInfo() proto.VersionInfo {
	return proto.VersionInfo{
		Version:   version.Version,
		Commit:    version.Commit,
		BuildID:   version.BuildID,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// Config returns the server-level configuration.
func (b *Backend) Config() *config.ConfigStore {
	return b.cfg
}

// Shutdown initiates a graceful server shutdown.
func (b *Backend) Shutdown() {
	if b.shutdownFn != nil {
		b.shutdownFn()
	}
}

func workspaceToProto(ws *Workspace) proto.Workspace {
	cfg := ws.Cfg.Config()
	return proto.Workspace{
		ID:        ws.ID,
		Path:      ws.Path,
		GitBranch: getGitBranch(ws.Path),
		YOLO:      ws.Cfg.Overrides().SkipPermissionRequests,
		DataDir:   cfg.Options.DataDirectory,
		Debug:     cfg.Options.Debug,
		Config:    cfg,
	}
}

// getGitBranch returns the current git branch for the given directory.
// Returns empty string if not in a git repo or on error.
func getGitBranch(dir string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
