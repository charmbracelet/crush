package pinentry

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// IntegrationConfig configures the process-wide integrated pinentry.
type IntegrationConfig struct {
	// Prompter collects credentials in-process (bound to DefaultPrompts).
	Prompter Prompter
	// CacheTimeout enables the optional in-memory credential cache.
	CacheTimeout time.Duration
	// GPGPath overrides PATH lookup for the gpg binary, when set.
	GPGPath string
	// WrapperPath is the path to the git gpg wrapper script. Empty on
	// platforms without a POSIX shell (integration is disabled there).
	WrapperPath string
	// SocketPath is the CRUSH_PINENTRY_SOCK socket path.
	SocketPath string
}

var (
	integrationMu   sync.RWMutex
	integrationCfg  *IntegrationConfig
	integrationRuns *Runner
)

// ConfigureIntegration enables the in-process integrated pinentry for
// the lifetime of the process. See DisableIntegration.
func ConfigureIntegration(cfg IntegrationConfig) {
	integrationMu.Lock()
	defer integrationMu.Unlock()
	integrationCfg = &cfg
	integrationRuns = &Runner{
		Prompter: func(ctx context.Context, req PromptRequest) (string, error) {
			return defaultPrompts.Prompt(ctx, req)
		},
		CacheTimeout: cfg.CacheTimeout,
		GPGPath:      cfg.GPGPath,
	}
}

// DisableIntegration disables the integrated pinentry and drops any
// cached credentials.
func DisableIntegration() {
	integrationMu.Lock()
	defer integrationMu.Unlock()
	if integrationRuns != nil {
		integrationRuns.ClearCache()
	}
	integrationCfg = nil
	integrationRuns = nil
}

// IntegrationEnabled reports whether the integrated pinentry is active.
func IntegrationEnabled() bool {
	integrationMu.RLock()
	defer integrationMu.RUnlock()
	return integrationCfg != nil
}

// DefaultRunner returns the process-wide gpg runner, or nil when
// integration is disabled.
func DefaultRunner() *Runner {
	integrationMu.RLock()
	defer integrationMu.RUnlock()
	return integrationRuns
}

// GitWrapperEnv returns environment variables that redirect git's gpg
// program to the Crush wrapper for Crush-spawned processes without
// changing any configuration files. Empty when integration is disabled
// or there is no wrapper script.
func GitWrapperEnv() []string {
	integrationMu.RLock()
	defer integrationMu.RUnlock()
	if integrationCfg == nil || integrationCfg.WrapperPath == "" {
		return nil
	}
	return []string{
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=gpg.program",
		"GIT_CONFIG_VALUE_0=" + integrationCfg.WrapperPath,
		"CRUSH_PINENTRY_SOCK=" + integrationCfg.SocketPath,
	}
}

// WriteGPGWrapper creates a POSIX sh wrapper that invokes the Crush
// binary's hidden __pinentry-gpg subcommand and returns its path. Git
// execs gpg.program directly (never through a shell), so a single file
// path that carries the subcommand is required.
//
// Returns ("", nil, nil) on platforms without a POSIX sh (Windows),
// where git is pointed back at its own gpg and integration is disabled.
func WriteGPGWrapper(executable string) (string, func(), error) {
	if runtime.GOOS == "windows" {
		return "", nil, nil
	}
	dir, err := os.MkdirTemp("", "crush-pinentry-*")
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, "gpg-wrapper")
	script := "#!/bin/sh\nexec " + shellQuote(executable) + " __pinentry-gpg \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	return path, cleanup, nil
}

// shellQuote single-quotes s for POSIX sh, escaping embedded quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
