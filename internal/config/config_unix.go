//go:build !windows

package config

// systemConfigPath is the system-wide configuration file path. It is
// loaded at the lowest priority so user and project configs override it.
const systemConfigPath = "/etc/crush/crush.json"
