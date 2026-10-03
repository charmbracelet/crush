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

// SetLazyMCPConfig persists the global lazy-MCP default in the given scope.
// Laziness is on unless something writes it false, so this is the switch the
// TUI toggle flips. Like the per-server flag it changes only what the model
// is shown, never which servers are connected.
func (s *ConfigStore) SetLazyMCPConfig(scope Scope, enabled bool) error {
	return s.update(scope, func(c *Config) map[string]any {
		if c.Options == nil {
			c.Options = &Options{}
		}
		lazy := enabled
		c.Options.LazyMCP = &lazy
		return map[string]any{"options.lazy_mcp": enabled}
	})
}

// IsLazy reports whether this server's tool schemas are kept out of the
// model context until the agent loads them with mcp_search. The per-server
// flag wins; when it is unset the global options.lazy_mcp default applies.
func (m MCPConfig) IsLazy(o *Options) bool {
	if m.Lazy != nil {
		return *m.Lazy
	}
	return o.GetLazyMCP()
}

// AnyMCPLazy reports whether any enabled MCP server keeps its tools out of
// context. When nothing is lazy there is nothing for mcp_search to find, so
// the tool is not registered at all. The value receiver keeps the method
// callable from a system-prompt template.
func (c Config) AnyMCPLazy() bool {
	for _, m := range c.MCP {
		if m.Disabled {
			continue
		}
		if m.IsLazy(c.Options) {
			return true
		}
	}
	return false
}

// SetMCPServerLazyConfig persists the lazy flag of a single MCP server in
// the given scope. A nil value drops the per-server override so the server
// follows options.lazy_mcp again. It fails when the server is not
// configured, so a typo can never create an empty config entry.
//
// Unlike the disabled flag this is not a connection setting: the server
// stays connected either way, so callers never need to restart it.
func (s *ConfigStore) SetMCPServerLazyConfig(scope Scope, name string, lazy *bool) error {
	if _, ok := s.Config().MCP[name]; !ok {
		return fmt.Errorf("mcp %q is not configured", name)
	}
	if lazy == nil {
		return s.RemoveConfigField(scope, "mcp."+name+".lazy")
	}
	return s.update(scope, func(c *Config) map[string]any {
		m := c.MCP[name]
		m.Lazy = lazy
		c.MCP[name] = m
		return map[string]any{"mcp." + name + ".lazy": *lazy}
	})
}
