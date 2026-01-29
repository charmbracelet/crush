package app

import (
	"context"
	"log/slog"
	"os/exec"
	"slices"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	powernapconfig "github.com/charmbracelet/x/powernap/pkg/config"
)

// initLSPClients initializes LSP clients.
func (app *App) initLSPClients(ctx context.Context) {
	slog.Info("LSP clients initialization started")

	manager := powernapconfig.NewManager()
	manager.LoadDefaults()

	var userConfiguredLSPs []string
	for name, clientConfig := range app.config.LSP {
		if clientConfig.Disabled {
			slog.Info("Skipping disabled LSP client", "name", name)
			manager.RemoveServer(name)
			continue
		}

		// HACK: the user might have the command name in their config, instead
		// of the actual name. This finds out these cases, and adjusts the name
		// accordingly.
		if _, ok := manager.GetServer(name); !ok {
			for sname, server := range manager.GetServers() {
				if server.Command == name {
					name = sname
					break
				}
			}
		}
		userConfiguredLSPs = append(userConfiguredLSPs, name)
		manager.AddServer(name, &powernapconfig.ServerConfig{
			Command:     clientConfig.Command,
			Args:        clientConfig.Args,
			Environment: clientConfig.Env,
			FileTypes:   clientConfig.FileTypes,
			RootMarkers: clientConfig.RootMarkers,
			InitOptions: clientConfig.InitOptions,
			Settings:    clientConfig.Options,
		})
	}

	servers := manager.GetServers()
	filtered := lsp.FilterMatching(app.config.WorkingDir(), servers)

	for _, name := range userConfiguredLSPs {
		if _, ok := filtered[name]; !ok {
			updateLSPState(name, lsp.StateDisabled, nil, nil, 0)
		}
	}
	for name, server := range filtered {
		if app.config.Options.AutoLSP != nil && !*app.config.Options.AutoLSP && !slices.Contains(userConfiguredLSPs, name) {
			slog.Debug("Ignoring non user-define LSP client due to AutoLSP being disabled", "name", name)
			continue
		}
		go app.createAndStartLSPClient(
			ctx, name,
			toOurConfig(server),
			slices.Contains(userConfiguredLSPs, name),
		)
	}
}

func toOurConfig(in *powernapconfig.ServerConfig) config.LSPConfig {
	return config.LSPConfig{
		Command:     in.Command,
		Args:        in.Args,
		Env:         in.Environment,
		FileTypes:   in.FileTypes,
		RootMarkers: in.RootMarkers,
		InitOptions: in.InitOptions,
		Options:     in.Settings,
	}
}

// createAndStartLSPClient creates a new LSP client, initializes it, and starts its workspace watcher.
func (app *App) createAndStartLSPClient(ctx context.Context, name string, config config.LSPConfig, userConfigured bool) {
	if !userConfigured {
		if _, err := exec.LookPath(config.Command); err != nil {
			slog.Warn("Default LSP config skipped: server not installed", "name", name, "error", err)
			return
		}
	}

	slog.Debug("Creating LSP client", "name", name, "command", config.Command, "fileTypes", config.FileTypes, "args", config.Args)

	// Update state to starting.
	updateLSPState(name, lsp.StateStarting, nil, nil, 0)

	// Create LSP client.
	lspClient, err := lsp.New(ctx, name, config, app.config.Resolver())
	if err != nil {
		if !userConfigured {
			slog.Warn("Default LSP config skipped due to error", "name", name, "error", err)
			updateLSPState(name, lsp.StateDisabled, nil, nil, 0)
			return
		}
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
		// Server reached a ready state successfully.
		slog.Debug("LSP server is ready", "name", name)
		lspClient.SetServerState(lsp.StateReady)
		updateLSPState(name, lsp.StateReady, nil, lspClient, 0)
	}

	slog.Debug("LSP client initialized", "name", name)

	// Add to map with mutex protection before starting goroutine
	app.LSPClients.Set(name, lspClient)
}
