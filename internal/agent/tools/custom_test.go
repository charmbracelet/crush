package tools

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

type denyPermissionService struct{ mockPermissionService }

func (denyPermissionService) Request(context.Context, permission.CreatePermissionRequest) (bool, error) {
	return false, nil
}

func runCustom(t *testing.T, tool *CustomTool, input string) fantasy.ToolResponse {
	t.Helper()
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session")
	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: tool.name, Input: input})
	require.NoError(t, err)
	return resp
}

func newTestCustomTool(t *testing.T, cfg config.CustomToolConfig) *CustomTool {
	t.Helper()
	return NewCustomTool("demo", cfg, &mockPermissionService{}, t.TempDir(), t.TempDir())
}

func TestCustomToolInfo(t *testing.T) {
	t.Parallel()

	info := newTestCustomTool(t, config.CustomToolConfig{
		Description: "d",
		Params:      map[string]string{"q": "query"},
		Required:    []string{"q"},
	}).Info()
	require.Equal(t, "demo", info.Name)
	require.Equal(t, map[string]any{"q": map[string]any{"type": "string", "description": "query"}}, info.Parameters)
	require.Equal(t, []string{"q"}, info.Required)

	info = newTestCustomTool(t, config.CustomToolConfig{
		Description: "d",
		Schema:      map[string]any{"properties": map[string]any{"n": map[string]any{"type": "number"}}},
	}).Info()
	require.Contains(t, info.Parameters, "n")
	require.Empty(t, info.Required)
}

func TestCustomToolRunReadsInputFromStdin(t *testing.T) {
	t.Parallel()

	source := filepath.Join(t.TempDir(), "plugin.sh")
	resp := runCustom(t, newTestCustomTool(t, config.CustomToolConfig{
		Command: `echo "$(jq -r .q) $CRUSH_TOOL_NAME $(basename "$CRUSH_PLUGIN_DIR") $GREETING"`,
		Env:     map[string]string{"GREETING": "hi"},
		Source:  source,
	}), `{"q":"hello"}`)
	require.False(t, resp.IsError, resp.Content)
	require.Equal(t, "hello demo "+filepath.Base(filepath.Dir(source))+" hi", resp.Content)
}

func TestCustomToolRunEmptyOutput(t *testing.T) {
	t.Parallel()

	resp := runCustom(t, newTestCustomTool(t, config.CustomToolConfig{Command: `true`}), `{}`)
	require.False(t, resp.IsError)
	require.Equal(t, "(no output)", resp.Content)
}

func TestCustomToolRunFailure(t *testing.T) {
	t.Parallel()

	resp := runCustom(t, newTestCustomTool(t, config.CustomToolConfig{Command: `echo partial; echo boom >&2; exit 3`}), `{}`)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "exit code 3")
	require.Contains(t, resp.Content, "boom")
	require.Contains(t, resp.Content, "partial")
}

func TestCustomToolRunTimeout(t *testing.T) {
	t.Parallel()

	resp := runCustom(t, newTestCustomTool(t, config.CustomToolConfig{Command: `sleep 5`, Timeout: 1}), `{}`)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "timed out after 1s")
}

func TestCustomToolAbandonsAStuckInterpreter(t *testing.T) {
	original := runCustomShell
	runCustomShell = func(ctx context.Context, _ shell.RunOptions) error {
		time.Sleep(3 * time.Second)
		return nil
	}
	t.Cleanup(func() { runCustomShell = original })

	start := time.Now()
	resp := runCustom(t, newTestCustomTool(t, config.CustomToolConfig{Command: `x`, Timeout: 1}), `{}`)
	require.True(t, resp.IsError)
	require.Less(t, time.Since(start), 3*time.Second)
}

func TestCustomToolRunPermissionDenied(t *testing.T) {
	t.Parallel()

	tool := NewCustomTool("demo", config.CustomToolConfig{Command: `echo ran`}, &denyPermissionService{}, t.TempDir(), t.TempDir())
	resp := runCustom(t, tool, `{}`)
	require.NotContains(t, resp.Content, "ran")
	require.Equal(t, NewPermissionDeniedResponse().Content, resp.Content)
}
