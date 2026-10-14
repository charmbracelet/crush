package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// TestEditToolsAcceptEditsGrantWithoutPrompt runs every permission request
// of edit, multiedit and write against the real permission service with
// accept-edits on. Each must pass the file it writes (FilePath), or the
// request waits for a prompt nobody answers and the timeout fails it.
func TestEditToolsAcceptEditsGrantWithoutPrompt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	perms := permission.NewPermissionService(dir, false, nil)
	perms.SetAcceptEdits("session", true)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, SessionIDContextKey, "session")
	edit := editContext{
		ctx:         ctx,
		permissions: perms,
		files:       &mockHistoryService{},
		filetracker: &mockEditFileTracker{lastRead: time.Now().Add(time.Hour)},
		workingDir:  dir,
	}
	file := func(name, content string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		return p
	}
	call := fantasy.ToolCall{ID: "call"}

	for name, run := range map[string]func() (fantasy.ToolResponse, error){
		"edit create": func() (fantasy.ToolResponse, error) {
			return createNewFile(edit, filepath.Join(dir, "new.txt"), "x", call)
		},
		"edit delete": func() (fantasy.ToolResponse, error) {
			return deleteContent(edit, file("del.txt", "a b"), " b", false, call)
		},
		"edit replace": func() (fantasy.ToolResponse, error) {
			return replaceContent(edit, file("rep.txt", "a"), "a", "b", false, call)
		},
		"multiedit create": func() (fantasy.ToolResponse, error) {
			return processMultiEditWithCreation(edit, MultiEditParams{
				FilePath: filepath.Join(dir, "sub", "multi.txt"),
				Edits:    []MultiEditOperation{{NewString: "x"}},
			}, call)
		},
		"multiedit existing": func() (fantasy.ToolResponse, error) {
			return processMultiEditExistingFile(edit, MultiEditParams{
				FilePath: file("multi-old.txt", "a"),
				Edits:    []MultiEditOperation{{OldString: "a", NewString: "b"}},
			}, call)
		},
		"write": func() (fantasy.ToolResponse, error) {
			input, err := json.Marshal(WriteParams{FilePath: "written.txt", Content: "x"})
			require.NoError(t, err)
			tool := NewWriteTool(nil, perms, &mockHistoryService{}, mockFileTrackerService{}, dir)
			return tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: WriteToolName, Input: string(input)})
		},
	} {
		resp, err := run()
		require.NoError(t, err, name)
		require.False(t, resp.IsError, "%s: %s", name, resp.Content)
	}
}

// TestEditToolsAskForDotDotAfterSymlink runs the real edit, multiedit and
// write tools on "<root>/link/../escaped.txt" with link pointing outside.
// Cleaned lexically that is inside the root, but the tools write the raw
// path and the kernel follows link first. Accept-edits must leave it to the
// prompt; the prompt is denied, so nothing may land outside.
func TestEditToolsAskForDotDotAfterSymlink(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	root := filepath.Join(base, "proj")
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "sub"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join(outside, "sub"), filepath.Join(root, "link")))

	perms := permission.NewPermissionService(root, false, nil)
	perms.SetAcceptEdits("session", true)
	prompts := perms.Subscribe(t.Context())
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session")
	evil := root + "/link/../escaped.txt"

	for _, tc := range []struct {
		tool  fantasy.AgentTool
		input any
	}{
		{
			NewEditTool(nil, perms, &mockHistoryService{}, mockFileTrackerService{}, root),
			EditParams{FilePath: evil, NewString: "pwned"},
		},
		{
			NewMultiEditTool(nil, perms, &mockHistoryService{}, mockFileTrackerService{}, root),
			MultiEditParams{FilePath: evil, Edits: []MultiEditOperation{{NewString: "pwned"}}},
		},
		{
			NewWriteTool(nil, perms, &mockHistoryService{}, mockFileTrackerService{}, root),
			WriteParams{FilePath: evil, Content: "pwned"},
		},
	} {
		name := tc.tool.Info().Name
		input, err := json.Marshal(tc.input)
		require.NoError(t, err)
		type result struct {
			resp fantasy.ToolResponse
			err  error
		}
		done := make(chan result, 1)
		go func() {
			resp, err := tc.tool.Run(ctx, fantasy.ToolCall{ID: name, Name: name, Input: string(input)})
			done <- result{resp, err}
		}()
		select {
		case ev := <-prompts:
			require.Equal(t, name, ev.Payload.ToolName)
			require.True(t, perms.Deny(ev.Payload))
		case r := <-done:
			t.Fatalf("%s ran without a prompt: %v %q", name, r.err, r.resp.Content)
		case <-time.After(5 * time.Second):
			t.Fatalf("%s neither prompted nor returned", name)
		}
		r := <-done
		require.NoError(t, r.err, name)
		require.True(t, r.resp.IsError, name)
		_, err = os.Stat(filepath.Join(outside, "escaped.txt"))
		require.ErrorIs(t, err, os.ErrNotExist, "%s wrote outside the project", name)
	}
}
