// Package config loads the service configuration from a simple
// key: value YAML-style file. The parser intentionally supports only
// the flat subset the project uses.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config carries the service settings shared by server, client, and
// the retry policy.
type Config struct {
	// Endpoint is the upstream API base URL.
	Endpoint string

	// TimeoutMS is the request timeout. Despite the name, the value
	// has always been interpreted as seconds by every consumer.
	TimeoutMS int

	// MaxConnections bounds the outbound connection pool.
	MaxConnections int
}

// Load reads a flat "key: value" file and returns the parsed Config.
// Unknown keys are ignored; missing keys keep their zero value.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	return parse(string(data)), nil
}

// Parse parses config text held in memory. It is used by tests and by
// callers that synthesize configuration programmatically.
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
		switch key {
		case "endpoint":
			cfg.Endpoint = value
		case "timeout_ms":
			if n, err := strconv.Atoi(value); err == nil {
				cfg.TimeoutMS = n
			}
		case "max_connections":
			if n, err := strconv.Atoi(value); err == nil {
				cfg.MaxConnections = n
			}
		}
	}
	return cfg
}
