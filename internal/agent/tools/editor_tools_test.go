package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/taigrr/fantasy"

	"github.com/taigrr/crush/internal/editor"
)

// fakeBridge is a controllable Bridge for table tests.
type fakeBridge struct {
	available    bool
	contextOut   editor.EditorContext
	contextErr   error
	locationsErr error
	gotTitle     string
	gotItems     []editor.Location
}

func (f *fakeBridge) Available() bool { return f.available }
func (f *fakeBridge) Context(context.Context) (editor.EditorContext, error) {
	return f.contextOut, f.contextErr
}

func (f *fakeBridge) ShowLocations(_ context.Context, title string, items []editor.Location) error {
	f.gotTitle = title
	f.gotItems = items
	return f.locationsErr
}
func (f *fakeBridge) FlashEdit(context.Context, string, int, int) error { return nil }
func (f *fakeBridge) NotifyFileChanged(context.Context, string) error   { return nil }
func (f *fakeBridge) Close() error                                      { return nil }

func TestEditorContextTool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bridge     editor.Bridge
		wantSubstr string
		wantIsErr  bool
	}{
		{
			name: "happy path returns json",
			bridge: &fakeBridge{
				available: true,
				contextOut: editor.EditorContext{
					Path:        "/tmp/foo.go",
					URI:         "file:///tmp/foo.go",
					Cursor:      editor.Position{Line: 10, Column: 5},
					ContextLine: "func foo() {",
					TotalLines:  42,
				},
			},
			wantSubstr: `"path":"/tmp/foo.go"`,
			wantIsErr:  false,
		},
		{
			name:       "unavailable bridge surfaces clean message",
			bridge:     &fakeBridge{available: false, contextErr: editor.ErrUnavailable},
			wantSubstr: "Editor bridge is not available",
			wantIsErr:  true,
		},
		{
			name:       "nil bridge falls back to noop",
			bridge:     nil,
			wantSubstr: "Editor bridge is not available",
			wantIsErr:  true,
		},
		{
			name:       "transport error surfaces context",
			bridge:     &fakeBridge{available: true, contextErr: errors.New("dial: connection refused")},
			wantSubstr: "editor_context failed",
			wantIsErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tool := NewEditorContextTool(tt.bridge)
			require.Equal(t, EditorContextToolName, tool.Info().Name)

			resp, err := tool.Run(t.Context(), fantasy.ToolCall{Input: "{}"})
			require.NoError(t, err)
			require.Equal(t, tt.wantIsErr, resp.IsError, "unexpected IsError state for response %q", resp.Content)
			require.True(t, strings.Contains(resp.Content, tt.wantSubstr),
				"expected response to contain %q; got %q", tt.wantSubstr, resp.Content)
		})
	}
}

func TestShowLocationsTool(t *testing.T) {
	t.Parallel()

	type want struct {
		substr    string
		isErr     bool
		title     string
		itemCount int
	}
	tests := []struct {
		name   string
		bridge *fakeBridge
		input  string
		want   want
	}{
		{
			name:   "forwards items to bridge",
			bridge: &fakeBridge{available: true},
			input:  `{"title":"refs","items":[{"filename":"a.go","lnum":1,"text":"x","note":"why"}]}`,
			want:   want{substr: "Displayed 1 location", isErr: false, title: "refs", itemCount: 1},
		},
		{
			name:   "empty items rejected with clear error",
			bridge: &fakeBridge{available: true},
			input:  `{"items":[]}`,
			want:   want{substr: "at least one item", isErr: true},
		},
		{
			name:   "unavailable bridge surfaces clean message",
			bridge: &fakeBridge{available: false, locationsErr: editor.ErrUnavailable},
			input:  `{"items":[{"filename":"a.go","lnum":1,"note":"why"}]}`,
			want:   want{substr: "Editor bridge is not available", isErr: true},
		},
		{
			name:   "transport error surfaces context",
			bridge: &fakeBridge{available: true, locationsErr: errors.New("rpc broken")},
			input:  `{"items":[{"filename":"a.go","lnum":1,"note":"why"}]}`,
			want:   want{substr: "show_locations failed", isErr: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tool := NewShowLocationsTool(tt.bridge)
			require.Equal(t, ShowLocationsToolName, tool.Info().Name)

			resp, err := tool.Run(t.Context(), fantasy.ToolCall{Input: tt.input})
			require.NoError(t, err)
			require.Equal(t, tt.want.isErr, resp.IsError, "response: %q", resp.Content)
			require.True(t, strings.Contains(resp.Content, tt.want.substr),
				"expected response to contain %q; got %q", tt.want.substr, resp.Content)
			if tt.want.itemCount > 0 {
				require.Equal(t, tt.want.title, tt.bridge.gotTitle)
				require.Len(t, tt.bridge.gotItems, tt.want.itemCount)
			}
		})
	}
}

// Sanity check that nil bridge is safe in show_locations too.
func TestShowLocationsTool_NilBridge(t *testing.T) {
	t.Parallel()
	tool := NewShowLocationsTool(nil)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{
		Input: `{"items":[{"filename":"a.go","lnum":1,"note":"why"}]}`,
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "Editor bridge is not available")
}
