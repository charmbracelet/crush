package config

import "fmt"

// SetMCPServerDisabledConfig persists the disabled flag of a single MCP
// server in the given scope's config file. It does not touch the running
// client; callers coordinate the runtime state separately. It fails when
// the server is not configured, so a typo can never create an empty
// config entry carrying only the disabled flag.
func (s *ConfigStore) SetMCPServerDisabledConfig(scope Scope, name string, disabled bool) error {
	if _, ok := s.Config().MCP[name]; !ok {
		return fmt.Errorf("mcp %q is not configured", name)
	}
	return s.update(scope, func(c *Config) map[string]any {
		m := c.MCP[name]
		m.Disabled = disabled
		c.MCP[name] = m
		return map[string]any{"mcp." + name + ".disabled": disabled}
	})
}
