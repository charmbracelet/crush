package config

import "slices"

// AddAllowedTool persists a tool in the always-allow list of the given
// scope's config file, under permissions.allowed_tools. It is a no-op
// when the tool is already on the list.
func (s *ConfigStore) AddAllowedTool(scope Scope, tool string) error {
	return s.update(scope, func(c *Config) map[string]any {
		var perms Permissions
		if c.Permissions != nil {
			perms = *c.Permissions
		}
		if slices.Contains(perms.AllowedTools, tool) {
			return nil
		}
		perms.AllowedTools = append(perms.AllowedTools, tool)
		c.Permissions = &perms
		return map[string]any{"permissions.allowed_tools": perms.AllowedTools}
	})
}
