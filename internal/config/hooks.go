package config

import (
	"fmt"
	"regexp"
	"strings"
)

// Hook event names. The hooks package re-exports these; they live here
// because config validation needs them and hooks imports config.
const (
	HookEventPreToolUse         = "PreToolUse"
	HookEventPostToolUse        = "PostToolUse"
	HookEventPostToolUseFailure = "PostToolUseFailure"
	HookEventPermissionRequest  = "PermissionRequest"
	HookEventPermissionResult   = "PermissionResult"
	HookEventSessionStart       = "SessionStart"
	HookEventSessionEnd         = "SessionEnd"
	HookEventTurnStart          = "TurnStart"
	HookEventTurnEnd            = "TurnEnd"
	HookEventStopFailure        = "StopFailure"
	HookEventInterrupt          = "Interrupt"
	HookEventPreCompact         = "PreCompact"
	HookEventPostCompact        = "PostCompact"
)

// hookEvents maps each canonical event name to whether it carries a tool
// name that a matcher can be tested against.
var hookEvents = map[string]bool{
	HookEventPreToolUse:         true,
	HookEventPostToolUse:        true,
	HookEventPostToolUseFailure: true,
	HookEventPermissionRequest:  true,
	HookEventPermissionResult:   true,
	HookEventSessionStart:       false,
	HookEventSessionEnd:         false,
	HookEventTurnStart:          false,
	HookEventTurnEnd:            false,
	HookEventStopFailure:        false,
	HookEventInterrupt:          false,
	HookEventPreCompact:         false,
	HookEventPostCompact:        false,
}

// hookEventByKey maps a folded event name (lowercase, underscores
// removed) to its canonical form.
var hookEventByKey = func() map[string]string {
	m := make(map[string]string, len(hookEvents))
	for name := range hookEvents {
		m[foldHookEvent(name)] = name
	}
	return m
}()

func foldHookEvent(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", ""))
}

// NormalizeHookEvent maps a user-provided event name to its canonical
// form. Matching is case-insensitive and accepts snake_case, so
// "pre_tool_use" and "PRETOOLUSE" both resolve to "PreToolUse". The
// second result is false when the name is not a known event.
func NormalizeHookEvent(name string) (string, bool) {
	canonical, ok := hookEventByKey[foldHookEvent(name)]
	return canonical, ok
}

// HookEventHasTool reports whether hooks for the event can use a matcher.
func HookEventHasTool(event string) bool {
	return hookEvents[event]
}

// ValidateHooks normalizes event names and checks that every configured
// hook names a known event, has a command, and only uses a matcher on
// events that carry a tool name. Matcher compilation used at run time is
// owned by hooks.Runner; this only surfaces config errors at load time
// rather than on the first tool call.
func (c *Config) ValidateHooks() error {
	for event, eventHooks := range c.Hooks {
		canonical, ok := NormalizeHookEvent(event)
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
			if !HookEventHasTool(event) {
				return fmt.Errorf("hook %s[%d]: matcher is only supported on tool events", event, i)
			}
			if _, err := regexp.Compile(h.Matcher); err != nil {
				return fmt.Errorf("hook %s[%d]: invalid matcher regex %q: %w", event, i, h.Matcher, err)
			}
		}
	}
	return nil
}
