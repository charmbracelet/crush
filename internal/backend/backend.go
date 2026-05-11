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
	"sync"

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

	// mu protects the workspace deduplication maps.
	mu sync.Mutex
	// dataDirToID maps data directory paths to workspace IDs to prevent
	// multiple clients from opening separate DB connections to the same database.
	dataDirToID map[string]string
	// workspaceClients tracks which client IDs are using each workspace ID.
	workspaceClients map[string]map[string]struct{}
	// clientToWorkspace maps client IDs to their workspace ID for lookup during delete.
	clientToWorkspace map[string]string
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
		workspaces:        csync.NewMap[string, *Workspace](),
		cfg:               cfg,
		ctx:               ctx,
		shutdownFn:        shutdownFn,
		dataDirToID:       make(map[string]string),
		workspaceClients:  make(map[string]map[string]struct{}),
		clientToWorkspace: make(map[string]string),
	}
}

// GetWorkspace retrieves a workspace by ID. The ID can be either a workspace ID
// or a client ID that maps to a shared workspace.
func (b *Backend) GetWorkspace(id string) (*Workspace, error) {
	// First try direct lookup (id is a workspace ID).
	if ws, ok := b.workspaces.Get(id); ok {
		return ws, nil
	}

	// Try looking up as a client ID.
	b.mu.Lock()
	workspaceID, ok := b.clientToWorkspace[id]
	b.mu.Unlock()

	if ok {
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

// CreateWorkspace initializes a new workspace from the given
// parameters. It creates the config, database connection, and
// [app.App] instance. If a workspace already exists for the same
// data directory, the existing workspace is returned with a new
// client ID to avoid opening multiple DB connections to the same file.
func (b *Backend) CreateWorkspace(args proto.Workspace) (*Workspace, proto.Workspace, error) {
	if args.Path == "" {
		return nil, proto.Workspace{}, ErrPathRequired
	}

	// Generate a unique client ID for this connection.
	clientID := uuid.New().String()

	// Initialize config to determine the data directory.
	cfg, err := config.Init(args.Path, args.DataDir, args.Debug)
	if err != nil {
		return nil, proto.Workspace{}, fmt.Errorf("failed to initialize config: %w", err)
	}

	dataDir := cfg.Config().Options.DataDirectory

	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if a workspace already exists for this data directory.
	if existingID, ok := b.dataDirToID[dataDir]; ok {
		if ws, ok := b.workspaces.Get(existingID); ok {
			// Track this client as using the existing workspace.
			b.workspaceClients[existingID][clientID] = struct{}{}
			b.clientToWorkspace[clientID] = existingID

			slog.Info("Reusing existing workspace for data directory",
				"client_id", clientID,
				"workspace_id", existingID,
				"data_dir", dataDir,
				"client_count", len(b.workspaceClients[existingID]),
			)

			// Return the existing workspace with the new client ID.
			result := proto.Workspace{
				ID:      clientID,
				Path:    args.Path,
				DataDir: dataDir,
				Debug:   ws.Cfg.Config().Options.Debug,
				YOLO:    ws.Cfg.Overrides().SkipPermissionRequests,
				Config:  ws.Cfg.Config(),
				Env:     args.Env,
			}
			return ws, result, nil
		}
		// Workspace was in map but not found - clean up stale entry.
		delete(b.dataDirToID, dataDir)
		delete(b.workspaceClients, existingID)
	}

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

	// Use the client ID as the workspace ID for the first client.
	ws := &Workspace{
		App:  appWorkspace,
		ID:   clientID,
		Path: args.Path,
		Cfg:  cfg,
		Env:  args.Env,
	}

	b.workspaces.Set(clientID, ws)
	b.dataDirToID[dataDir] = clientID
	b.workspaceClients[clientID] = map[string]struct{}{clientID: {}}
	b.clientToWorkspace[clientID] = clientID

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
		ID:      clientID,
		Path:    args.Path,
		DataDir: dataDir,
		Debug:   cfg.Config().Options.Debug,
		YOLO:    cfg.Overrides().SkipPermissionRequests,
		Config:  cfg.Config(),
		Env:     args.Env,
	}

	return ws, result, nil
}

// DeleteWorkspace removes a client from its workspace. The workspace is only
// shut down when the last client disconnects. If it was the last workspace,
// the shutdown callback is invoked.
func (b *Backend) DeleteWorkspace(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Look up which workspace this client belongs to.
	workspaceID, ok := b.clientToWorkspace[clientID]
	if !ok {
		// Unknown client, try legacy direct lookup.
		if ws, ok := b.workspaces.Get(clientID); ok {
			ws.Shutdown()
			b.workspaces.Del(clientID)
		}
		if b.workspaces.Len() == 0 && b.shutdownFn != nil {
			slog.Info("Last workspace removed, shutting down server...")
			b.shutdownFn()
		}
		return
	}

	// Remove client from tracking.
	delete(b.clientToWorkspace, clientID)

	clients, hasClients := b.workspaceClients[workspaceID]
	if hasClients {
		delete(clients, clientID)
	}

	remainingClients := len(clients)
	slog.Info("Client disconnected from workspace",
		"client_id", clientID,
		"workspace_id", workspaceID,
		"remaining_clients", remainingClients,
	)

	// Only shut down the workspace if no clients remain.
	if remainingClients > 0 {
		return
	}

	// Clean up workspace.
	ws, ok := b.workspaces.Get(workspaceID)
	if ok {
		// Find and clean up the data directory mapping.
		dataDir := ws.Cfg.Config().Options.DataDirectory
		delete(b.dataDirToID, dataDir)

		ws.Shutdown()
	}
	delete(b.workspaceClients, workspaceID)
	b.workspaces.Del(workspaceID)

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
