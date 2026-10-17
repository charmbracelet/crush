package message

import "context"

type operatorSteeringKey struct{}

// WithOperatorSteering records whether a submission may amend the active turn
// under its existing permission policy. It does not change that policy.
func WithOperatorSteering(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, operatorSteeringKey{}, enabled)
}

// OperatorSteering reports an explicit operator steering request.
func OperatorSteering(ctx context.Context) bool {
	enabled, _ := ctx.Value(operatorSteeringKey{}).(bool)
	return enabled
}
