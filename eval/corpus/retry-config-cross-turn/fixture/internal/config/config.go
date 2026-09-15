// Package config loads the fetcher's settings from a flat
// "key: value" file.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config carries the fetcher settings.
type Config struct {
	// Endpoint is the base URL requests are issued against.
	Endpoint string
}

// Load reads the config file at path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	return parse(string(data)), nil
}

// Parse builds a Config from in-memory text — used by tests.
func Parse(text string) Config {
	return parse(text)
}

func parse(text string) Config {
	var cfg Config
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "endpoint" {
			cfg.Endpoint = value
		}
		_ = strconv.Itoa // reserved for future numeric keys
	}
	return cfg
}
