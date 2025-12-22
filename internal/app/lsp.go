package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
)

// initLSPClients initializes LSP clients.
func (app *App) initLSPClients(ctx context.Context) {
	for name, clientConfig := range app.config.LSP {
		if clientConfig.Disabled {
			slog.Info("Skipping disabled LSP client", "name", name)
			continue
		}
		go app.createAndStartLSPClient(ctx, name, clientConfig)
	}
	slog.Info("LSP clients initialization started in background")
}

// createAndStartLSPClient creates a new LSP client, initializes it, and starts its workspace watcher
func (app *App) createAndStartLSPClient(ctx context.Context, name string, config config.LSPConfig) {
	slog.Debug("Creating LSP client", "name", name, "command", config.Command, "fileTypes", config.FileTypes, "args", config.Args)

	// Check if any root markers exist in the working directory (config now has defaults)
	if !lsp.HasRootMarkers(app.config.WorkingDir(), config.RootMarkers) {
		slog.Debug("Skipping LSP client: no root markers found", "name", name, "rootMarkers", config.RootMarkers)
		updateLSPState(name, lsp.StateDisabled, nil, nil, 0)
		return
	}

	// Update state to starting
	updateLSPState(name, lsp.StateStarting, nil, nil, 0)

	// Create LSP client.
	lspClient, err := lsp.New(ctx, name, config, app.config.Resolver())
	if err != nil {
		slog.Error("Failed to create LSP client for", "name", name, "error", err)
		updateLSPState(name, lsp.StateError, err, nil, 0)
		return
	}

	// Set diagnostics callback
	lspClient.SetDiagnosticsCallback(updateLSPDiagnostics)

	// Increase initialization timeout as some servers take more time to start.
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Initialize LSP client.
	_, err = lspClient.Initialize(initCtx, app.config.WorkingDir())
	if err != nil {
		slog.Error("LSP client initialization failed", "name", name, "error", err)
		updateLSPState(name, lsp.StateError, err, lspClient, 0)
		lspClient.Close(ctx)
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
		slog.Debug("LSP server is ready", "name", name)
		lspClient.SetServerState(lsp.StateReady)
		updateLSPState(name, lsp.StateReady, nil, lspClient, 0)
	}

	slog.Info("LSP client initialized", "name", name)

	// Add to map with mutex protection before starting goroutine
	app.LSPClients.Set(name, lspClient)
}
