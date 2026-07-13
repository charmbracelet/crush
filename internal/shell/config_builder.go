package shell

import "context"

// ConfigBuilder accumulates config state as config builtins execute during
// a crush.sh config file load. It is stored on the shell context so that
// builtins (provider, model, mcp, etc.) can access shared state.
//
// This file lives in the shell package to avoid an import cycle:
// config imports shell (for ExpandValue), so shell cannot import config.
// The builder holds config as json.RawMessage fragments that are deep-merged
// by the config loader after the script finishes.
type ConfigBuilder struct {
	// Fragments is a list of JSON objects that will be deep-merged.
	// Each builtin call appends a fragment.
	Fragments [][]byte
}

type configBuilderCtxKey struct{}

// ConfigBuilderFromCtx returns the ConfigBuilder stored on the context, or
// nil if none is present (normal bash tool execution). Config builtins use
// this to gate themselves: they are no-ops without a builder.
func ConfigBuilderFromCtx(ctx context.Context) *ConfigBuilder {
	v, _ := ctx.Value(configBuilderCtxKey{}).(*ConfigBuilder)
	return v
}

// WithConfigBuilder returns a context with the given ConfigBuilder attached.
// Used by the config loader when executing a crush.sh file.
func WithConfigBuilder(ctx context.Context, b *ConfigBuilder) context.Context {
	return context.WithValue(ctx, configBuilderCtxKey{}, b)
}

// AppendFragment adds a JSON object fragment to the builder. These fragments
// are deep-merged by the config loader after the script finishes executing.
func (b *ConfigBuilder) AppendFragment(data []byte) {
	b.Fragments = append(b.Fragments, data)
}
