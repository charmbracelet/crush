package shellconfig

import (
	"context"
	"encoding/json"
)

// ConfigBuilder accumulates config state as config builtins execute during a
// crushrc load. Builtins mutate the nested map directly, in execution order,
// so imperative operations (append, set, remove, reset) resolve exactly as the
// script intends. The builder is stored on the shell context so builtins can
// reach it; it is absent during normal bash tool execution, which makes the
// config builtins no-ops there.
//
// At the end of a script the builder marshals to a single JSON object, which
// the config loader merges with any other config files.
type ConfigBuilder struct {
	root map[string]any
}

// newConfigBuilder returns an empty builder.
func newConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{root: make(map[string]any)}
}

// section returns the top-level object stored at key, creating it if absent.
func (b *ConfigBuilder) section(key string) map[string]any {
	m, ok := b.root[key].(map[string]any)
	if !ok {
		m = make(map[string]any)
		b.root[key] = m
	}
	return m
}

// empty reports whether the builder holds no config.
func (b *ConfigBuilder) empty() bool {
	return len(b.root) == 0
}

// JSON marshals the accumulated config to a single JSON object.
func (b *ConfigBuilder) JSON() ([]byte, error) {
	return json.Marshal(b.root)
}

// childMap returns/creates the nested object parent[key].
func childMap(parent map[string]any, key string) map[string]any {
	m, ok := parent[key].(map[string]any)
	if !ok {
		m = make(map[string]any)
		parent[key] = m
	}
	return m
}

type configBuilderCtxKey struct{}

// configBuilderFromCtx returns the ConfigBuilder on the context, or nil if
// none is present (normal bash tool execution).
func configBuilderFromCtx(ctx context.Context) *ConfigBuilder {
	v, _ := ctx.Value(configBuilderCtxKey{}).(*ConfigBuilder)
	return v
}

// withConfigBuilder returns a context carrying the given ConfigBuilder.
func withConfigBuilder(ctx context.Context, b *ConfigBuilder) context.Context {
	return context.WithValue(ctx, configBuilderCtxKey{}, b)
}
