package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/watcher"
)

// initLSPClients initializes LSP clients.
func (app *App) initLSPClients(ctx context.Context) {
	for name, clientConfig := range app.config.LSP {
		go app.createAndStartLSPClient(ctx, name, clientConfig)
	}
	slog.Info("LSP clients initialization started in background")
}

// createAndStartLSPClient creates a new LSP client, initializes it, and starts its workspace watcher
func (app *App) createAndStartLSPClient(ctx context.Context, name string, config config.LSPConfig) {
	slog.Info("Creating LSP client", "name", name, "command", config.ResolvedCommand(), "fileTypes", config.FileTypes, "args", config.Args)

	// Update state to starting
	updateLSPState(name, lsp.StateStarting, nil, nil, 0)

	// Create LSP client.
	lspClient, err := lsp.NewClient(ctx, name, config)
	if err != nil {
		slog.Error("Failed to create LSP client for", name, err)
		updateLSPState(name, lsp.StateError, err, nil, 0)
		return
	}

	// Set diagnostics callback
	lspClient.SetDiagnosticsCallback(updateLSPDiagnostics)

	// Increase initialization timeout as some servers take more time to start.
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Initialize LSP client.
	_, err = lspClient.InitializeLSPClient(initCtx, app.config.WorkingDir())
	if err != nil {
		slog.Error("Initialize failed", "name", name, "error", err)
		updateLSPState(name, lsp.StateError, err, lspClient, 0)
		lspClient.Close()
		return
	}

	// Wait for the server to be ready.
	if err := lspClient.WaitForServerReady(initCtx); err != nil {
		slog.Error("Server failed to become ready", "name", name, "error", err)
		// Server never reached a ready state, but let's continue anyway, as
		// some functionality might still work.
		lspClient.SetServerState(lsp.StateError)
		updateLSPState(name, lsp.StateError, err, lspClient, 0)
	} else {
		// Server reached a ready state scuccessfully.
		slog.Info("LSP server is ready", "name", name)
		lspClient.SetServerState(lsp.StateReady)
		updateLSPState(name, lsp.StateReady, nil, lspClient, 0)
	}

	slog.Info("LSP client initialized", "name", name)

	// Create a child context that can be canceled when the app is shutting
	// down.
	watchCtx, cancelFunc := context.WithCancel(ctx)

	// Create the workspace watcher.
	workspaceWatcher := watcher.NewWorkspaceWatcher(name, lspClient)

	// Store the cancel function to be called during cleanup.
	app.watcherCancelFuncs.Append(cancelFunc)

	// Add to map with mutex protection before starting goroutine
	app.clientsMutex.Lock()
	app.LSPClients[name] = lspClient
	app.clientsMutex.Unlock()

	// Run workspace watcher.
	app.lspWatcherWG.Add(1)
	go app.runWorkspaceWatcher(watchCtx, name, workspaceWatcher)
}

// runWorkspaceWatcher executes the workspace watcher for an LSP client.
func (app *App) runWorkspaceWatcher(ctx context.Context, name string, workspaceWatcher *watcher.WorkspaceWatcher) {
	defer app.lspWatcherWG.Done()
	defer log.RecoverPanic("LSP-"+name, func() {
		// Try to restart the client.
		app.restartLSPClient(ctx, name)
	})

	workspaceWatcher.WatchWorkspace(ctx, app.config.WorkingDir())
	slog.Info("Workspace watcher stopped", "client", name)
}

// restartLSPClient attempts to restart a crashed or failed LSP client.
func (app *App) restartLSPClient(ctx context.Context, name string) {
	// Get the original configuration.
	clientConfig, exists := app.config.LSP[name]
	if !exists {
		slog.Error("Cannot restart client, configuration not found", "client", name)
		return
	}

	// Clean up the old client if it exists.
	app.clientsMutex.Lock()
	oldClient, exists := app.LSPClients[name]
	if exists {
		// Remove from map before potentially slow shutdown.
		delete(app.LSPClients, name)
	}
	app.clientsMutex.Unlock()

	if exists && oldClient != nil {
		// Try to shut down client gracefully, but don't block on errors.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = oldClient.Shutdown(shutdownCtx)
		cancel()
	}

	// Create a new client using the shared function.
	app.createAndStartLSPClient(ctx, name, clientConfig)
	slog.Info("Successfully restarted LSP client", "client", name)
}
