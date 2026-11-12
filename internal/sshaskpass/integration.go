package sshaskpass

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// IntegrationConfig configures the process-wide integrated ssh askpass.
type IntegrationConfig struct {
	// AskpassPath is the path to the SSH_ASKPASS wrapper script. Empty
	// on platforms without a POSIX shell (integration is disabled there).
	AskpassPath string
	// SocketPath is the CRUSH_SSH_SOCK socket path that the wrapper
	// subprocess connects to.
	SocketPath string
}

var (
	integrationMu  sync.RWMutex
	integrationCfg *IntegrationConfig
)

// ConfigureIntegration enables the in-process integrated ssh askpass
// for the lifetime of the process. See DisableIntegration.
func ConfigureIntegration(cfg IntegrationConfig) {
	integrationMu.Lock()
	defer integrationMu.Unlock()
	integrationCfg = &cfg
}

// DisableIntegration disables the integrated ssh askpass.
func DisableIntegration() {
	integrationMu.Lock()
	defer integrationMu.Unlock()
	integrationCfg = nil
}

// IntegrationEnabled reports whether the integrated ssh askpass is
// active.
func IntegrationEnabled() bool {
	integrationMu.RLock()
	defer integrationMu.RUnlock()
	return integrationCfg != nil
}

// AskpassEnv returns environment variables that redirect OpenSSH's
// credential and confirmation prompts (the SSH_ASKPASS program) to the
// integrated prompt service for Crush-spawned processes, without
// changing any user or ssh configuration files. Empty when integration
// is disabled or the askpass wrapper is unavailable.
//
// SSH_ASKPASS_REQUIRE=force makes OpenSSH (>= 8.4) consult askpass even
// when DISPLAY is unset or a tty is present, which is exactly the
// headless environment Crush shells run in.
func AskpassEnv() []string {
	integrationMu.RLock()
	defer integrationMu.RUnlock()
	if integrationCfg == nil || integrationCfg.AskpassPath == "" {
		return nil
	}
	return []string{
		"SSH_ASKPASS=" + integrationCfg.AskpassPath,
		"SSH_ASKPASS_REQUIRE=force",
		"CRUSH_SSH_SOCK=" + integrationCfg.SocketPath,
	}
}

// WriteAskpassWrapper creates a POSIX sh wrapper that invokes the Crush
// binary's hidden __ssh-askpass subcommand and returns its path.
// OpenSSH execs SSH_ASKPASS directly as a single program path (commands
// are never parsed from the value), so a file that carries the
// subcommand is required.
//
// Returns ("", nil, nil) on platforms without a POSIX sh (Windows),
// where the integration is disabled entirely.
func WriteAskpassWrapper(executable string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "crush-sshaskpass-*")
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, "ssh-askpass")
	script := "#!/bin/sh\nexec " + shellQuote(executable) + " __ssh-askpass \"$@\"\n"
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
