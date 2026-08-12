//go:build windows

package config

// systemConfigPath is empty on Windows: there is no system-wide config.
// loadFromConfigPaths skips paths that do not exist, so no special-case
// handling is needed.
const systemConfigPath = ""
