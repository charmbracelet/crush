package agent

import "context"

type contentWidthContextKey struct{}

// WithContentWidth returns ctx tagged with the UI's current chat content
// width in cells. Tools that read width-sensitive remote content (e.g. an
// A2UI resource whose server pre-renders bar geometry) use it to ask the
// server for exactly the right size. Zero means "no hint".
func WithContentWidth(ctx context.Context, width int) context.Context {
	return context.WithValue(ctx, contentWidthContextKey{}, width)
}

// ContentWidthFromContext returns the UI content width in cells, or 0 when
// the turn did not originate from an interactive UI (remote clients, headless
// runs, channels).
func ContentWidthFromContext(ctx context.Context) int {
	if w, ok := ctx.Value(contentWidthContextKey{}).(int); ok {
		return w
	}
	return 0
}
