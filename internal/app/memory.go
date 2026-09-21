package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/honcho"
	"github.com/charmbracelet/crush/internal/version"
)

// newMemory builds the Honcho memory service for this workspace, or
// returns nil when memory is not configured.
//
// Returning nil rather than an error is deliberate. Memory is an
// enhancement: a missing key, an unreachable deployment, or a
// malformed shared config file should leave Crush working exactly as
// it did before, not refuse to start.
func newMemory(ctx context.Context, cfg *config.Config, workDir string) *honcho.Service {
	resolved := honcho.Resolve(honcho.FromCrushConfig(cfg.Honcho))
	if !resolved.Enabled {
		return nil
	}

	// The Crush session ID is not known at startup. Only the
	// per-session strategy needs it, and that strategy is not the
	// default, so deriving from the working directory here is
	// correct for every other case.
	svc := honcho.NewService(resolved, workDir, "", version.Version)
	if svc == nil {
		return nil
	}

	// Create the workspace, peers, and session up front so the first
	// turn does not pay for topology setup on top of its own latency.
	// Failure here is not fatal: the write queue retries per batch,
	// and reads degrade to no memory.
	go func() {
		if err := svc.EnsureTopology(context.WithoutCancel(ctx)); err != nil {
			slog.Debug("Honcho topology setup failed", "error", err)
		}
	}()

	id := svc.Identity()
	slog.Info("Honcho memory enabled",
		"workspace", id.Workspace,
		"session", id.SessionKey,
		"peer", id.UserPeer,
	)
	return svc
}

// memoryOrNil converts the service into the agent's Memory interface.
//
// A typed nil pointer stored in an interface is not nil, so a bare
// conversion would hand the agent a non-nil interface wrapping a nil
// service and defeat every nil check downstream.
func memoryOrNil(svc *honcho.Service) agent.Memory {
	if svc == nil {
		return nil
	}
	return svc
}

// memoryProvider returns the function the coordinator asks for the
// current memory backend.
//
// The indirection is what lets "Connect Memory" work without a
// restart. Connecting writes a credential and then triggers the same
// agent refresh a model change does; this provider re-reads the
// resolved config on that rebuild, starts the service if it is now
// enabled, and hands it over. Disconnecting is the mirror image.
//
// The service itself is built at most once and reused, so a rebuild
// for any other reason does not churn the write queue or re-run
// topology setup.
func (app *App) memoryProvider(ctx context.Context) agent.MemoryProvider {
	return func() agent.Memory {
		enabled := honcho.Resolve(honcho.FromCrushConfig(app.config.Config().Honcho)).Enabled

		app.memoryMu.Lock()
		defer app.memoryMu.Unlock()

		switch {
		case !enabled && app.memory != nil:
			// Stop the writer, but do not wait on the drain: this
			// runs on the tool-build path, and a flush that cannot
			// reach the network would stall the rebuild the user is
			// waiting on.
			svc := app.memory
			app.memory = nil
			go func() {
				drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), memoryDrainTimeout)
				defer cancel()
				svc.Close(drainCtx)
			}()
		case enabled && app.memory == nil:
			app.memory = newMemory(ctx, app.config.Config(), app.config.WorkingDir())
		}

		return memoryOrNil(app.memory)
	}
}

// memoryDrainTimeout bounds the background flush after the user
// disconnects memory.
const memoryDrainTimeout = 5 * time.Second

// currentMemory returns the running memory service, if any.
func (app *App) currentMemory() *honcho.Service {
	app.memoryMu.Lock()
	defer app.memoryMu.Unlock()
	return app.memory
}
