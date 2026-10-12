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

type askLogPermissionService struct {
	mockPermissionService
	asked []string
}

func (r *askLogPermissionService) Request(_ context.Context, req permission.CreatePermissionRequest) (bool, error) {
	r.asked = append(r.asked, req.ToolName+":"+req.Action)
	return true, nil
}

func actionTool(perms permission.Service) *CustomTool {
	return NewCustomTool("Demo", config.CustomToolConfig{
		Description: "Demo things.",
		Params:      map[string]string{"repo": "Repository"},
		Actions: map[string]config.CustomToolAction{
			"view":    {Description: "Show one", Params: map[string]string{"number": "Item number"}, Required: []string{"number"}, Command: `echo "view $number in ${repo:-here}"`, SkipPermission: true},
			"comment": {Description: "Comment", Params: map[string]string{"number": "Item number", "body": "Text"}, Required: []string{"number", "body"}, Command: `printf '%s' "$body"`},
		},
	}, perms, "/", "/")
}

func TestCustomToolActionsInfo(t *testing.T) {
	t.Parallel()

	info := actionTool(&mockPermissionService{}).Info()
	require.Equal(t, []string{"action"}, info.Required)
	require.Equal(t, []string{"comment", "view"}, info.Parameters["action"].(map[string]any)["enum"])
	require.Contains(t, info.Description, "- view: Show one (number required)")
	require.Contains(t, info.Description, "- comment: Comment (body required, number required)")
	require.Equal(t, "Item number (for comment, view)", info.Parameters["number"].(map[string]any)["description"])
	require.Equal(t, "Repository", info.Parameters["repo"].(map[string]any)["description"])
}

func TestCustomToolActionsPermissionAndVariables(t *testing.T) {
	t.Parallel()

	perms := &askLogPermissionService{}
	tool := actionTool(perms)

	resp := runCustom(t, tool, `{"action":"view","number":7}`)
	require.False(t, resp.IsError, resp.Content)
	require.Equal(t, "view 7 in here", resp.Content)
	require.Empty(t, perms.asked, "view skips the prompt")

	resp = runCustom(t, tool, `{"action":"comment","number":"7","body":"it's $(rm -rf /) safe"}`)
	require.False(t, resp.IsError, resp.Content)
	require.Equal(t, "it's $(rm -rf /) safe", resp.Content, "input is quoted, never run")
	require.Equal(t, []string{"Demo:comment"}, perms.asked, "the action is the permission key, so permissions allow Demo:comment works")
}

func TestCustomToolActionsRejectBadCalls(t *testing.T) {
	t.Parallel()

	tool := actionTool(&mockPermissionService{})
	resp := runCustom(t, tool, `{"action":"nope"}`)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "expected one of: comment, view")

	resp = runCustom(t, tool, `{"action":"comment","number":"1"}`)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, `missing required parameter "body"`)
}

func TestCustomToolSkipPermission(t *testing.T) {
	t.Parallel()

	tool := NewCustomTool("demo", config.CustomToolConfig{Command: `echo ran`, SkipPermission: true}, &denyPermissionService{}, t.TempDir(), t.TempDir())
	require.Equal(t, "ran", runCustom(t, tool, `{}`).Content)
}

func TestCustomToolDisplay(t *testing.T) {
	t.Parallel()

	cfg := config.CustomToolConfig{
		Description: "GitHub",
		Title:       "GitHub",
		Params:      map[string]string{"repo": "Repo"},
		Actions: map[string]config.CustomToolAction{
			"pr_list": {
				Description: "List",
				Params:      map[string]string{"limit": "Max", "state": "State"},
				Title:       "List PRs",
				Summary:     `Latest {{plural .limit 20 "pull request"}}, {{if eq .state "all"}}open or closed{{else}}{{or .state "open"}}{{end}}{{with .repo}} in {{.}}{{end}}`,
			},
			"pr_view": {Description: "View", Params: map[string]string{"number": "N"}},
		},
	}
	for _, tc := range []struct{ input, title, summary string }{
		{`{"action":"pr_list","limit":"1","state":"all"}`, "List PRs", "Latest 1 pull request, open or closed"},
		{`{"action":"pr_list","limit":5,"repo":"a/b"}`, "List PRs", "Latest 5 pull requests, open in a/b"},
		{`{"action":"pr_list"}`, "List PRs", "Latest 20 pull requests, open"},
		{`{"action":"pr_view","number":"3"}`, "GitHub", ""},
	} {
		title, summary, ok := CustomToolDisplay(cfg, "GitHub", tc.input)
		require.True(t, ok, tc.input)
		require.Equal(t, tc.title, title, tc.input)
		require.Equal(t, tc.summary, summary, tc.input)
	}

	_, _, ok := CustomToolDisplay(cfg, "GitHub", `{"action":"pr_li`)
	require.False(t, ok, "streaming input falls back to the plain rendering")
	_, _, ok = CustomToolDisplay(config.CustomToolConfig{Command: "x"}, "x", `{}`)
	require.False(t, ok, "no templates, no custom display")
}

type capturePermissionService struct {
	mockPermissionService
	req permission.CreatePermissionRequest
}

func (c *capturePermissionService) Request(_ context.Context, req permission.CreatePermissionRequest) (bool, error) {
	c.req = req
	return false, nil
}

func TestCustomToolPermissionPromptExplainsTheCall(t *testing.T) {
	t.Parallel()

	perms := &capturePermissionService{}
	tool := NewCustomTool("GitHub", config.CustomToolConfig{
		Description: "GitHub",
		Params:      map[string]string{"repo": "Repo"},
		Actions: map[string]config.CustomToolAction{"pr_comment": {
			Description: "Post a comment on a pull request.",
			Params:      map[string]string{"number": "N", "body": "B"},
			Required:    []string{"number", "body"},
			Title:       "Comment on PR #{{.number}}",
			Summary:     `"{{truncate 20 .body}}"`,
			Command:     `gh pr comment "$number" -F -`,
		}},
	}, perms, t.TempDir(), t.TempDir())

	runCustom(t, tool, `{"action":"pr_comment","number":"4052","body":"First line\nsecond line","repo":"a/b"}`)
	require.Equal(t, "GitHub:pr_comment", perms.req.ToolName+":"+perms.req.Action)
	require.Equal(t, "Comment on PR #4052\n\"First line second l…\"\n\nPost a comment on a pull request.", perms.req.Description)
	require.Equal(t, "number: 4052\nbody:\n  First line\n  second line\nrepo: a/b\n\nRuns:\n  gh pr comment \"$number\" -F -", perms.req.Params)
}
