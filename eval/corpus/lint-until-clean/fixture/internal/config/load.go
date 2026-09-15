// Package config loads the toolkit's flat key=value settings file.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config carries toolkit settings.
type Config struct {
	Workspace string
	Verbose   bool
}

// Load reads a key=value file (one pair per line, '#' comments).
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("cannot read config %s", path))
	}
	var cfg Config
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "workspace":
			cfg.Workspace = strings.TrimSpace(value)
		case "verbose":
			cfg.Verbose = strings.TrimSpace(value) == "true"
		}
	}
	if cfg.Workspace == "" {
		fmt.Println("config: workspace not set, using .")
		cfg.Workspace = "."
	}
	return cfg, nil
}
