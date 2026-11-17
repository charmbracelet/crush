package config

import (
	"context"
	"os"
	"strconv"
)

// LoadExperimental reads only the experimental options from the config
// files for workingDir, skipping the provider setup a full [Load] does. It
// is for decisions made before the full load, such as whether to start in
// client/server mode.
func LoadExperimental(ctx context.Context, workingDir string) (*ExperimentalOptions, error) {
	cfg, _, err := loadFromConfigPaths(ctx, lookupConfigs(workingDir))
	if err != nil || cfg.Options == nil {
		return nil, err
	}
	return cfg.Options.Experimental, nil
}

// GetClientServer reports whether client/server mode is configured on,
// ignoring the environment.
func (e *ExperimentalOptions) GetClientServer() bool {
	return e != nil && e.ClientServer != nil && *e.ClientServer
}

// ExportEnv sets the environment variable behind each configured option,
// leaving any variable that is already set alone so the environment keeps
// priority over the config. Processes Crush spawns, such as the server,
// inherit the result.
func (e *ExperimentalOptions) ExportEnv() {
	if e == nil {
		return
	}
	setDefault := func(name, value string) {
		if os.Getenv(name) == "" {
			_ = os.Setenv(name, value)
		}
	}
	if e.ClientServer != nil {
		setDefault("CRUSH_CLIENT_SERVER", strconv.FormatBool(*e.ClientServer))
	}
	if e.ServerReadyTimeout != nil {
		setDefault("CRUSH_SERVER_READY_TIMEOUT", strconv.Itoa(*e.ServerReadyTimeout)+"s")
	}
	if e.ServerIdleTimeout != nil {
		setDefault("CRUSH_SERVER_IDLE_TIMEOUT", strconv.Itoa(*e.ServerIdleTimeout))
	}
	if e.ServerDetachGrace != nil {
		setDefault("CRUSH_SERVER_DETACH_GRACE", strconv.Itoa(*e.ServerDetachGrace))
	}
}
