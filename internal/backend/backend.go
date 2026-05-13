// Package backend provides transport-agnostic operations for managing
// workspaces, sessions, agents, permissions, and events. It is consumed
// by protocol-specific layers such as HTTP (server) and ACP.
package backend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/version"
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
		cfg:        cfg,
		ctx:        ctx,
		shutdownFn: shutdownFn,
	}
}

// GetWorkspace retrieves a workspace by ID.
func (b *Backend) GetWorkspace(id string) (*Workspace, error) {
	ws, ok := b.workspaces.Get(id)
	if !ok {
		return nil, ErrWorkspaceNotFound
	}
	return ws, nil
}

// ListWorkspaces returns all running workspaces.
func (b *Backend) ListWorkspaces() []proto.Workspace {
	workspaces := []proto.Workspace{}
	for _, ws := range b.workspaces.Seq2() {
		workspaces = append(workspaces, workspaceToProto(ws))
	}
	return workspaces
}

// CreateWorkspace initializes a new workspace from the given
// parameters. It creates the config, database connection, and
// [app.App] instance.
func (b *Backend) CreateWorkspace(args proto.Workspace) (*Workspace, proto.Workspace, error) {
	if args.Path == "" {
		return nil, proto.Workspace{}, ErrPathRequired
	}

	id := uuid.New().String()
	cfg, err := config.Init(args.Path, args.DataDir, args.Debug)
	if err != nil {
		return nil, proto.Workspace{}, fmt.Errorf("failed to initialize config: %w", err)
	}

	cfg.Overrides().SkipPermissionRequests = args.YOLO

	if err := createDotCrushDir(cfg.Config().Options.DataDirectory); err != nil {
		return nil, proto.Workspace{}, fmt.Errorf("failed to create data directory: %w", err)
	}

	conn, err := db.Connect(b.ctx, cfg.Config().Options.DataDirectory)
	if err != nil {
		return nil, proto.Workspace{}, fmt.Errorf("failed to connect to database: %w", err)
	}

	appWorkspace, err := app.New(b.ctx, conn, cfg)
	if err != nil {
		return nil, proto.Workspace{}, fmt.Errorf("failed to create app workspace: %w", err)
	}

	ws := &Workspace{
		App:  appWorkspace,
		ID:   id,
		Path: args.Path,
		Cfg:  cfg,
		Env:  args.Env,
	}

	b.workspaces.Set(id, ws)

	if args.Version != "" && args.Version != version.Version {
		slog.Warn("Client/server version mismatch",
			"client", args.Version,
			"server", version.Version,
		)
		appWorkspace.SendEvent(util.NewWarnMsg(fmt.Sprintf(
			"Server version %q differs from client version %q. Consider restarting the server.",
			version.Version, args.Version,
		)))
	}

	result := proto.Workspace{
		ID:      id,
		Path:    args.Path,
		DataDir: cfg.Config().Options.DataDirectory,
		Debug:   cfg.Config().Options.Debug,
		YOLO:    cfg.Overrides().SkipPermissionRequests,
		Config:  cfg.Config(),
		Env:     args.Env,
	}

	return ws, result, nil
}

// DeleteWorkspace shuts down and removes a workspace. If it was the
// last workspace, the shutdown callback is invoked.
func (b *Backend) DeleteWorkspace(id string) {
	ws, ok := b.workspaces.Get(id)
	if ok {
		ws.Shutdown()
	}
	b.workspaces.Del(id)

	if b.workspaces.Len() == 0 && b.shutdownFn != nil {
		slog.Info("Last workspace removed, shutting down server...")
		b.shutdownFn()
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
		ID:      ws.ID,
		Path:    ws.Path,
		YOLO:    ws.Cfg.Overrides().SkipPermissionRequests,
		DataDir: cfg.Options.DataDirectory,
		Debug:   cfg.Options.Debug,
		Config:  cfg,
	}
}
