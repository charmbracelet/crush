package agent

import "context"

// autoApproveContextKey is the unexported context key that carries a
// caller's request to auto-approve the turn it is about to start, from
// the workspace HTTP boundary (backend.runAgent) down into
// coordinator.run, which copies it onto SessionAgentCall.AutoApprove.
// sessionAgent.Run then re-stamps it with the decision the turn
// actually made, so the turn's tool calls read that instead of whatever
// request arrived with the context.
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
	return withTurnAutoApprove(ctx, true)
}

// withTurnAutoApprove stamps the decision the active turn made onto its
// run context, so the turn's tool calls — and the sub-agent turns they
// start — inherit exactly this turn's approval.
//
// It always sets the value, false included. A queued prompt is run by
// the frame of the turn it queued behind, whose context carries that
// turn's request rather than this call's, so anything less than an
// unconditional stamp lets one turn's approval leak into another's tool
// calls.
func withTurnAutoApprove(ctx context.Context, approve bool) context.Context {
	return context.WithValue(ctx, autoApproveContextKey{}, approve)
}

// AutoApproveFromContext reports whether the turn owning ctx grants its
// own permission requests. Exported because the coordinator and tests in
// other packages need to read it; safe to call on any context.
func AutoApproveFromContext(ctx context.Context) bool {
	approve, _ := ctx.Value(autoApproveContextKey{}).(bool)
	return approve
}
