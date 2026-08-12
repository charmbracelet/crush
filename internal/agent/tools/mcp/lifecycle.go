package mcp

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
)

// reinitAction describes how to reconcile one MCP server against the
// current config.
type reinitAction int

const (
	reinitDisable reinitAction = iota + 1
	reinitRemove
	reinitStart
)

// reconcile diffs running MCP state against the current config and returns
// the action to take for each server. It is a pure function: all state is
// passed in, nothing global is read or mutated, so the reconciliation
// decision can be tested with plain maps.
//
// The config a server last connected with lives on its ClientInfo (Config),
// and the config an in-flight attempt is connecting with lives on
// PendingConfig. Reconcile compares the live config against whichever is
// relevant for the server's state:
//
//   - A starting server is left alone only while it is connecting with the
//     current config. If the config changed since it started, it restarts so
//     the new config takes effect instead of being lost to the in-flight
//     attempt.
//   - A connected server restarts when its config differs from the one it
//     connected with.
//   - Every other state (new, errored, needs-auth, disabled) restarts: new
//     and disabled servers carry no config, and retrying a failed server on
//     each config write is the desired recovery path.
//
// Servers gone from config are removed entirely; enabled-in-config servers
// marked disabled are disabled.
func reconcile(current config.MCPs, running map[string]ClientInfo) map[string]reinitAction {
	actions := map[string]reinitAction{}

	// Servers no longer in config are removed entirely.
	for name := range running {
		if _, exists := current[name]; !exists {
			actions[name] = reinitRemove
		}
	}

	for name, m := range current {
		info, exists := running[name]
		if m.Disabled {
			if exists && info.State != StateDisabled {
				actions[name] = reinitDisable
			}
			continue
		}

		if exists {
			switch info.State {
			case StateStarting:
				// Restart only if the config changed since this attempt
				// started; otherwise let the in-flight attempt settle so
				// rapid writes don't pile up overlapping init goroutines.
				if info.PendingConfig != nil && mcpConfigEqual(*info.PendingConfig, m) {
					continue
				}
			case StateConnected:
				if mcpConfigEqual(info.Config, m) {
					continue
				}
			}
		}
		actions[name] = reinitStart
	}

	return actions
}

// reinitMu guards reinitRunning and reinitDirty.
var (
	reinitMu      sync.Mutex
	reinitRunning bool
	reinitDirty   bool
)

// Reinitialize reconciles running MCP servers against the current config.
// Servers added since the last call are started, servers removed are torn
// down, and servers whose config changed are restarted. Unchanged servers
// keep their existing sessions.
//
// MCP state is process-global, so reconciliation is single-flighted: at
// most one runs at a time. A config write that arrives mid-run just sets a
// dirty flag and returns; the running reconciliation loops once more to
// pick up the newer state. This coalesces a burst of rapid writes into at
// most two reconciles instead of queueing a redundant no-op pass per write,
// while still guaranteeing the final state reflects the latest config.
func Reinitialize(ctx context.Context, cfg *config.ConfigStore) {
	reinitMu.Lock()
	if reinitRunning {
		reinitDirty = true
		reinitMu.Unlock()
		return
	}
	reinitRunning = true
	reinitMu.Unlock()

	for {
		reconcileOnce(ctx, cfg)

		reinitMu.Lock()
		if !reinitDirty {
			reinitRunning = false
			reinitMu.Unlock()
			return
		}
		reinitDirty = false
		reinitMu.Unlock()
	}
}

// reconcileOnce applies one reconciliation pass against the current config.
func reconcileOnce(ctx context.Context, cfg *config.ConfigStore) {
	current := cfg.Config().MCP
	actions := reconcile(current, states.Copy())
	for name, action := range actions {
		switch action {
		case reinitRemove:
			slog.Info("Removing MCP server no longer in config", "name", name)
			removeServer(name)
		case reinitDisable:
			slog.Info("Disabling MCP server", "name", name)
			DisableSingle(cfg, name)
		case reinitStart:
			m := current[name]
			if _, exists := states.Get(name); exists {
				slog.Info("Re-initializing MCP server after config change", "name", name)
			} else {
				slog.Info("Initializing new MCP server after config change", "name", name)
			}
			// teardown bumps the generation, invalidating any in-flight
			// attempt for this server. The StateStarting transition records
			// m as PendingConfig so a subsequent reconcile can tell whether
			// the attempt now in flight matches the latest config.
			teardown(name)
			updateState(name, StateStarting, nil, nil, Counts{}, withPending(m))
			goInitClient(ctx, cfg, name, m, nil)
		}
	}
}

// removeServer fully tears down an MCP server and deletes its state
// entry. Unlike DisableSingle (which keeps the entry as StateDisabled),
// this is for servers that no longer exist in config at all.
func removeServer(name string) {
	teardown(name)
	states.Del(name)
	gens.Del(name)
}

// mcpConfigEqual reports whether two MCPConfig values are equal, ignoring
// the internally-managed OAuthToken field. Field-by-field rather than
// reflect.DeepEqual so the comparison is explicit about what matters.
// TestMCPConfigEqualExhaustive guards against drift: it fails at test
// time if a new field is added to MCPConfig without a decision about
// whether it participates here.
func mcpConfigEqual(a, b config.MCPConfig) bool {
	return a.Command == b.Command &&
		maps.Equal(a.Env, b.Env) &&
		slices.Equal(a.Args, b.Args) &&
		a.Type == b.Type &&
		a.URL == b.URL &&
		a.Disabled == b.Disabled &&
		slices.Equal(a.DisabledTools, b.DisabledTools) &&
		slices.Equal(a.EnabledTools, b.EnabledTools) &&
		a.Timeout == b.Timeout &&
		maps.Equal(a.Headers, b.Headers) &&
		a.OAuth == b.OAuth &&
		a.OAuthClientID == b.OAuthClientID &&
		a.OAuthClientSecret == b.OAuthClientSecret &&
		a.OAuthCallbackPort == b.OAuthCallbackPort
}
