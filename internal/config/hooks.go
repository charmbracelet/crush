package config

import (
	"fmt"
	"regexp"

	"github.com/charmbracelet/crush/internal/hooks"
)

// ValidateHooks normalizes event names and checks that every configured
// hook names a known event, has a command, and only uses a matcher on
// events that carry a tool call. Matcher compilation used at run time is
// owned by hooks.Runner; this only surfaces config errors at load time
// rather than on the first tool call.
func (c *Config) ValidateHooks() error {
	for event, eventHooks := range c.Hooks {
		canonical, ok := hooks.NormalizeEvent(event)
		if !ok {
			return fmt.Errorf("hook %s: unknown event", event)
		}
		if canonical != event {
			c.Hooks[canonical] = append(c.Hooks[canonical], eventHooks...)
			delete(c.Hooks, event)
		}
	}

	for event, eventHooks := range c.Hooks {
		for i, h := range eventHooks {
			if h.Command == "" {
				return fmt.Errorf("hook %s[%d]: command is required", event, i)
			}
			if h.Matcher == "" {
				continue
			}
			if !hooks.EventHasTool(event) {
				return fmt.Errorf("hook %s[%d]: matcher is only supported on tool events", event, i)
			}
			if _, err := regexp.Compile(h.Matcher); err != nil {
				return fmt.Errorf("hook %s[%d]: invalid matcher regex %q: %w", event, i, h.Matcher, err)
			}
		}
	}
	return nil
}
