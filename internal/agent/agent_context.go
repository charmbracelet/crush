package agent

import "context"

// modeContextKey is the unexported context key under which the current
// AgentMode travels. Use ModeFromContext to read it.
type modeContextKey struct{}

// WithMode returns a new context that carries the given AgentMode. The
// hookedTool plan guard reads this to decide whether a tool is allowed.
func WithMode(ctx context.Context, m AgentMode) context.Context {
	if !m.Valid() {
		return ctx
	}
	return context.WithValue(ctx, modeContextKey{}, m)
}

// ModeFromContext returns the AgentMode stored in ctx, or AgentModeBuild
// when none is present. The default is intentionally permissive so that
// the plan guard never accidentally blocks a build-mode agent.
func ModeFromContext(ctx context.Context) AgentMode {
	if ctx == nil {
		return AgentModeBuild
	}
	m, ok := ctx.Value(modeContextKey{}).(AgentMode)
	if !ok || !m.Valid() {
		return AgentModeBuild
	}
	return m
}
