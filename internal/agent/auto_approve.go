package agent

import "context"

// autoApproveContextKey is the unexported context key that carries a
// caller's request to auto-approve the turn it is about to start, from
// the workspace HTTP boundary (backend.runAgent) down into
// coordinator.run, which copies it onto SessionAgentCall.AutoApprove.
type autoApproveContextKey struct{}

// WithAutoApprove returns ctx tagged with a request to grant every
// permission request the resulting turn makes. It is the boundary
// helper for callers that have nobody to answer a permission prompt
// (`crush run`, in both the in-process and the client/server path).
//
// The approval is bound to the turn, not to the caller: sessionAgent.Run
// takes the hold when the call becomes the active turn and gives it back
// when the turn ends. A caller that exits early therefore cannot strand
// a still-running turn on an unanswerable prompt, nor leave the session
// approved for whoever keeps the workspace alive afterwards.
func WithAutoApprove(ctx context.Context) context.Context {
	return context.WithValue(ctx, autoApproveContextKey{}, true)
}

// AutoApproveFromContext reports whether [WithAutoApprove] tagged ctx.
// Exported because the coordinator and tests in other packages need to
// read it; safe to call on any context.
func AutoApproveFromContext(ctx context.Context) bool {
	approve, _ := ctx.Value(autoApproveContextKey{}).(bool)
	return approve
}
