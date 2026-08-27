package agent

import "context"

// nonInteractiveContextKey is the unexported context key that carries
// "no human is watching this run" from the boundary that starts a run
// (backend.runAgent for the client/server path, app.RunNonInteractive
// for the in-process one) down into coordinator.run, which copies it
// onto SessionAgentCall.NonInteractive.
type nonInteractiveContextKey struct{}

// WithNonInteractive returns ctx tagged as a run nobody can answer
// prompts for: `crush run`, in both the in-process and the client/server
// path.
//
// Two things depend on it. The run waits for MCP initialization to
// settle, because it gets a single shot at the tool palette and cannot
// pick up a late server on a later prompt. And interactive-only tools
// are withheld from its turn: a call to one would block until the run
// was cancelled, since nobody is there to answer.
//
// It is a property of the run, not of the workspace. One workspace on
// the shared server serves an attached TUI and headless prompts at the
// same time, so the coordinator cannot answer this question for
// everyone.
func WithNonInteractive(ctx context.Context) context.Context {
	return context.WithValue(ctx, nonInteractiveContextKey{}, true)
}

// NonInteractiveFromContext reports whether [WithNonInteractive] tagged
// ctx. Exported because the boundary packages and their tests need to
// read it; safe to call on any context.
func NonInteractiveFromContext(ctx context.Context) bool {
	nonInteractive, _ := ctx.Value(nonInteractiveContextKey{}).(bool)
	return nonInteractive
}
