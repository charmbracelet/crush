package plugin

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestRegisterTool(t *testing.T) {
	t.Cleanup(ResetTools)

	// Register a tool.
	factory := func(ctx context.Context, app *App) (Tool, error) {
		return fantasy.NewAgentTool(
			"test_tool",
			"A test tool",
			func(ctx context.Context, params struct{}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
				return fantasy.NewTextResponse("test"), nil
			},
		), nil
	}

	RegisterTool("test_tool", factory)

	// Verify registration.
	tools := RegisteredTools()
	require.Equal(t, []string{"test_tool"}, tools)

	// Get factory and create tool.
	f, ok := GetToolFactory("test_tool")
	require.True(t, ok)

	app := NewApp(WithWorkingDir("/tmp"))
	tool, err := f(context.Background(), app)
	require.NoError(t, err)
	require.Equal(t, "test_tool", tool.Info().Name)
}

func TestRegisterToolPanicOnDuplicate(t *testing.T) {
	t.Cleanup(ResetTools)

	factory := func(ctx context.Context, app *App) (Tool, error) {
		return nil, nil
	}

	RegisterTool("dup_tool", factory)

	require.Panics(t, func() {
		RegisterTool("dup_tool", factory)
	})
}

func TestRegistrationOrder(t *testing.T) {
	t.Cleanup(ResetTools)

	factory := func(ctx context.Context, app *App) (Tool, error) {
		return nil, nil
	}

	RegisterTool("tool_a", factory)
	RegisterTool("tool_b", factory)
	RegisterTool("tool_c", factory)

	tools := RegisteredTools()
	require.Equal(t, []string{"tool_a", "tool_b", "tool_c"}, tools)
}

func TestRegisterToolWithConfig(t *testing.T) {
	t.Cleanup(ResetTools)

	type TestConfig struct {
		APIKey  string `json:"api_key"`
		Timeout int    `json:"timeout"`
	}

	factory := func(ctx context.Context, app *App) (Tool, error) {
		return nil, nil
	}

	RegisterToolWithConfig("configured_tool", factory, &TestConfig{})

	// Verify registration.
	tools := RegisteredTools()
	require.Equal(t, []string{"configured_tool"}, tools)

	// Verify config schema is registered.
	schema, ok := GetConfigSchema("configured_tool")
	require.True(t, ok)
	require.NotNil(t, schema)
	require.IsType(t, &TestConfig{}, schema)
}

func TestGetToolRegistration(t *testing.T) {
	t.Cleanup(ResetTools)

	type TestConfig struct {
		Value string `json:"value"`
	}

	factory := func(ctx context.Context, app *App) (Tool, error) {
		return nil, nil
	}

	RegisterToolWithConfig("full_reg_tool", factory, &TestConfig{})

	reg, ok := GetToolRegistration("full_reg_tool")
	require.True(t, ok)
	require.NotNil(t, reg.Factory)
	require.NotNil(t, reg.ConfigSchema)

	// Non-existent tool.
	_, ok = GetToolRegistration("nonexistent")
	require.False(t, ok)
}

func TestAppIsPluginDisabled(t *testing.T) {
	t.Parallel()

	app := NewApp(
		WithDisabledPlugins([]string{"disabled1", "disabled2"}),
	)

	require.True(t, app.IsPluginDisabled("disabled1"))
	require.True(t, app.IsPluginDisabled("disabled2"))
	require.False(t, app.IsPluginDisabled("enabled"))
}

func TestAppLoadConfig(t *testing.T) {
	t.Parallel()

	type TestConfig struct {
		StringVal string `json:"string_val"`
		IntVal    int    `json:"int_val"`
		BoolVal   bool   `json:"bool_val"`
	}

	pluginConfig := map[string]map[string]any{
		"myplugin": {
			"string_val": "hello",
			"int_val":    42,
			"bool_val":   true,
		},
	}

	app := NewApp(WithPluginConfig(pluginConfig))

	var cfg TestConfig
	err := app.LoadConfig("myplugin", &cfg)
	require.NoError(t, err)

	require.Equal(t, "hello", cfg.StringVal)
	require.Equal(t, 42, cfg.IntVal)
	require.True(t, cfg.BoolVal)
}

func TestAppLoadConfigNoConfig(t *testing.T) {
	t.Parallel()

	type TestConfig struct {
		Value string `json:"value"`
	}

	app := NewApp()

	var cfg TestConfig
	err := app.LoadConfig("nonexistent", &cfg)
	require.NoError(t, err)

	// Should leave struct with zero values.
	require.Empty(t, cfg.Value)
}

func TestAppLoadConfigInvalidTarget(t *testing.T) {
	t.Parallel()

	app := NewApp()

	// Nil target.
	err := app.LoadConfig("test", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be nil")

	// Non-pointer.
	var s string
	err = app.LoadConfig("test", s)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a non-nil pointer")

	// Pointer to non-struct.
	err = app.LoadConfig("test", &s)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a pointer to a struct")
}

func TestAppLoadConfigTypeMismatch(t *testing.T) {
	t.Parallel()

	type TestConfig struct {
		IntVal int `json:"int_val"`
	}

	pluginConfig := map[string]map[string]any{
		"myplugin": {
			"int_val": "not an int",
		},
	}

	app := NewApp(WithPluginConfig(pluginConfig))

	var cfg TestConfig
	err := app.LoadConfig("myplugin", &cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse config")
}

func TestAppPluginConfig(t *testing.T) {
	t.Parallel()

	pluginConfig := map[string]map[string]any{
		"plugin1": {"key": "value1"},
		"plugin2": {"key": "value2"},
	}

	app := NewApp(WithPluginConfig(pluginConfig))

	cfg := app.PluginConfig("plugin1")
	require.NotNil(t, cfg)
	require.Equal(t, "value1", cfg["key"])

	cfg = app.PluginConfig("plugin2")
	require.NotNil(t, cfg)
	require.Equal(t, "value2", cfg["key"])

	cfg = app.PluginConfig("nonexistent")
	require.Nil(t, cfg)
}

func TestAppExtensionConfigBackwardsCompat(t *testing.T) {
	t.Parallel()

	pluginConfig := map[string]map[string]any{
		"plugin1": {"key": "value"},
	}

	app := NewApp(WithExtensionConfig(pluginConfig))

	// ExtensionConfig should work as alias for PluginConfig.
	cfg := app.ExtensionConfig("plugin1")
	require.NotNil(t, cfg)
	require.Equal(t, "value", cfg["key"])
}

func TestRegisterHook(t *testing.T) {
	t.Cleanup(ResetHooks)

	factory := func(ctx context.Context, app *App) (Hook, error) {
		return &mockHook{name: "test_hook"}, nil
	}

	RegisterHook("test_hook", factory)

	// Verify registration.
	hooks := RegisteredHooks()
	require.Equal(t, []string{"test_hook"}, hooks)

	// Get factory and create hook.
	f, ok := GetHookFactory("test_hook")
	require.True(t, ok)

	app := NewApp()
	hook, err := f(context.Background(), app)
	require.NoError(t, err)
	require.Equal(t, "test_hook", hook.Name())
}

func TestRegisterHookPanicOnDuplicate(t *testing.T) {
	t.Cleanup(ResetHooks)

	factory := func(ctx context.Context, app *App) (Hook, error) {
		return nil, nil
	}

	RegisterHook("dup_hook", factory)

	require.Panics(t, func() {
		RegisterHook("dup_hook", factory)
	})
}

func TestRegisterHookWithConfig(t *testing.T) {
	t.Cleanup(ResetHooks)

	type HookConfig struct {
		Endpoint string `json:"endpoint"`
	}

	factory := func(ctx context.Context, app *App) (Hook, error) {
		return &mockHook{name: "configured_hook"}, nil
	}

	RegisterHookWithConfig("configured_hook", factory, &HookConfig{})

	// Verify registration.
	hooks := RegisteredHooks()
	require.Equal(t, []string{"configured_hook"}, hooks)

	// Get registration.
	reg, ok := GetHookRegistration("configured_hook")
	require.True(t, ok)
	require.NotNil(t, reg.Factory)
	require.NotNil(t, reg.ConfigSchema)
}

func TestHookRegistrationOrder(t *testing.T) {
	t.Cleanup(ResetHooks)

	factory := func(ctx context.Context, app *App) (Hook, error) {
		return nil, nil
	}

	RegisterHook("hook_a", factory)
	RegisterHook("hook_b", factory)
	RegisterHook("hook_c", factory)

	hooks := RegisteredHooks()
	require.Equal(t, []string{"hook_a", "hook_b", "hook_c"}, hooks)
}

func TestAppMessages(t *testing.T) {
	t.Parallel()

	// Without message subscriber.
	app := NewApp()
	require.Nil(t, app.Messages())

	// With message subscriber.
	mock := &mockMessageSubscriber{}
	app = NewApp(WithMessageSubscriber(mock))
	require.NotNil(t, app.Messages())
	require.Equal(t, mock, app.Messages())
}

// mockHook is a test implementation of Hook.
type mockHook struct {
	name    string
	started bool
	stopped bool
}

func (h *mockHook) Name() string {
	return h.name
}

func (h *mockHook) Start(ctx context.Context) error {
	h.started = true
	<-ctx.Done()
	return nil
}

func (h *mockHook) Stop() error {
	h.stopped = true
	return nil
}

// mockMessageSubscriber is a test implementation of MessageSubscriber.
type mockMessageSubscriber struct{}

func (m *mockMessageSubscriber) SubscribeMessages(ctx context.Context) <-chan MessageEvent {
	ch := make(chan MessageEvent)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch
}
