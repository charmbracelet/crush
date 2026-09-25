package agent

import "context"

type channelContextKey struct{}

// WithChannel returns ctx tagged with the channel that originated the turn.
func WithChannel(ctx context.Context, channel string) context.Context {
	return context.WithValue(ctx, channelContextKey{}, channel)
}

// ChannelFromContext returns the originating channel, or an empty string for local turns.
func ChannelFromContext(ctx context.Context) string {
	if channel, ok := ctx.Value(channelContextKey{}).(string); ok {
		return channel
	}
	return ""
}
