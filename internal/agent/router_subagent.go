package agent

import "context"

// routerSubAgentEffortContextKey is the unexported context key used to
// carry the parent turn's router-applied reasoning effort down into the
// "task" sub-agent tool (see agentTool in agent_tool.go), without
// forcing a breaking change to Coordinator.Run's signature. It is only
// ever set when RouterOptions.ApplySubagents is true and the router
// actually applied a reasoning-effort decision for the turn.
type routerSubAgentEffortContextKey struct{}

// WithRouterSubAgentEffort returns ctx tagged with the reasoning effort
// that router-driven sub-agents launched during this turn should use.
// An empty effort is stored as-is; downstream code treats an empty
// value as "no override", identical to not having called this at all.
func WithRouterSubAgentEffort(ctx context.Context, effort string) context.Context {
	return context.WithValue(ctx, routerSubAgentEffortContextKey{}, effort)
}

// RouterSubAgentEffortFromContext returns the effort set by
// [WithRouterSubAgentEffort], or "" if none was set or the value is not
// a string. Safe to call on any context.
func RouterSubAgentEffortFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(routerSubAgentEffortContextKey{}).(string); ok {
		return v
	}
	return ""
}

// routerSubAgentModelContextKey is the unexported context key used to
// carry the parent turn's router-applied model decision down into the
// "task" sub-agent tool, without forcing a breaking change to
// Coordinator.Run's signature. It is only ever set when
// RouterOptions.ApplySubagents is true and the router actually applied
// a model choice decision for the turn.
type routerSubAgentModelContextKey struct{}

// WithRouterSubAgentModel returns ctx tagged with the model that
// router-driven sub-agents launched during this turn should use.
// A nil model is stored as-is; downstream code treats nil as "no
// override", identical to not having called this at all.
func WithRouterSubAgentModel(ctx context.Context, model *Model) context.Context {
	return context.WithValue(ctx, routerSubAgentModelContextKey{}, model)
}

// RouterSubAgentModelFromContext returns the model set by
// [WithRouterSubAgentModel], or nil if none was set or the value is not
// a *Model. Safe to call on any context.
func RouterSubAgentModelFromContext(ctx context.Context) *Model {
	if v, ok := ctx.Value(routerSubAgentModelContextKey{}).(*Model); ok {
		return v
	}
	return nil
}
