package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filehistory"
	"github.com/stretchr/testify/require"
)

func TestFileHistoryRefusesToolAfterFailedPreimage(t *testing.T) {
	t.Parallel()
	if os.Getenv("FILESNAP_INTEGRATION_TESTS") != "1" {
		t.Skip("set FILESNAP_INTEGRATION_TESTS=1 to exercise the managed official executable")
	}
	base := t.TempDir()
	cwd := filepath.Join(base, "workspace")
	require.NoError(t, os.Mkdir(cwd, 0o700))
	store := filehistory.New(cwd, filepath.Join(base, "history"), "")
	ctx, release, err := store.Begin(t.Context(), "session")
	require.NoError(t, err)
	defer release()
	require.NoError(t, os.Mkdir(filepath.Join(cwd, "directory"), 0o700))
	inner := &fakeTool{name: "write"}
	tool := &fileHistoryTool{AgentTool: inner}
	result, err := tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: "write", Input: `{"file_path":"directory","content":"data"}`})
	require.NoError(t, err)
	require.True(t, result.StopTurn)
	require.False(t, inner.called)
}

type fileWritingTool struct {
	fakeTool
	cwd string
}

func (t *fileWritingTool) Run(_ context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var input struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal([]byte(call.Input), &input); err != nil {
		return fantasy.ToolResponse{}, err
	}
	return fantasy.NewTextResponse("written"), os.WriteFile(filepath.Join(t.cwd, input.FilePath), []byte(input.Content), 0o600)
}

func TestFileHistoryCapturesThePathRewrittenByAHook(t *testing.T) {
	t.Parallel()
	if os.Getenv("FILESNAP_INTEGRATION_TESTS") != "1" {
		t.Skip("set FILESNAP_INTEGRATION_TESTS=1 to exercise the managed official executable")
	}
	base := t.TempDir()
	cwd := filepath.Join(base, "workspace")
	require.NoError(t, os.Mkdir(cwd, 0o700))
	actual := filepath.Join(cwd, ".actual")
	require.NoError(t, os.WriteFile(actual, []byte("before"), 0o600))
	store := filehistory.New(cwd, filepath.Join(base, "history"), "")
	ctx, release, err := store.Begin(t.Context(), "session")
	require.NoError(t, err)
	runner := newRunner(t, `echo '{"updated_input":{"file_path":".actual","content":"after"}}'`)
	inner := &fileWritingTool{fakeTool: fakeTool{name: "write"}, cwd: cwd}
	tool := newHookedTool(&fileHistoryTool{AgentTool: inner}, runner)
	_, err = tool.Run(ctx, fantasy.ToolCall{ID: "rewrite", Name: "write", Input: `{"file_path":".wrong","content":"wrong"}`})
	require.NoError(t, err)
	content, err := os.ReadFile(actual)
	require.NoError(t, err)
	require.Equal(t, "after", string(content))
	release()
	log, err := store.Command(t.Context(), "session", "list", "")
	require.NoError(t, err)
	_, err = store.Command(t.Context(), "session", "restore", log[0]["turn"].(string))
	require.NoError(t, err)
	content, err = os.ReadFile(actual)
	require.NoError(t, err)
	require.Equal(t, "before", string(content))
}
