package tools

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/taigrr/fantasy"
	"github.com/stretchr/testify/require"
)

// fakeViewTool emulates the single-file view tool: returns canned content
// per path (or an error). Counts calls so we can assert concurrency
// happened.
type fakeViewTool struct {
	responses map[string]fakeViewResponse
	calls     atomic.Int64
}

type fakeViewResponse struct {
	content string
	isErr   bool
	err     error
}

func (f *fakeViewTool) Info() fantasy.ToolInfo                  { return fantasy.ToolInfo{Name: ViewToolName} }
func (f *fakeViewTool) ProviderOptions() fantasy.ProviderOptions { return fantasy.ProviderOptions{} }
func (f *fakeViewTool) SetProviderOptions(fantasy.ProviderOptions) {}

func (f *fakeViewTool) Run(_ context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	f.calls.Add(1)
	// Trivially decode the JSON: rely on substring match instead of
	// import-cycle-prone re-parse logic in the test.
	for path, resp := range f.responses {
		if strings.Contains(call.Input, `"file_path":"`+path+`"`) {
			if resp.err != nil {
				return fantasy.ToolResponse{}, resp.err
			}
			if resp.isErr {
				return fantasy.NewTextErrorResponse(resp.content), nil
			}
			return fantasy.NewTextResponse(resp.content), nil
		}
	}
	return fantasy.NewTextErrorResponse("unexpected path"), nil
}

func TestMultiViewTool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		responses map[string]fakeViewResponse
		want      []string
		notWant   []string
		isError   bool
	}{
		{
			name:  "happy path concatenates results with delimiters",
			input: `{"file_paths":["/a.go","/b.go"]}`,
			responses: map[string]fakeViewResponse{
				"/a.go": {content: "AAA"},
				"/b.go": {content: "BBB"},
			},
			want: []string{"===== /a.go =====", "AAA", "===== /b.go =====", "BBB"},
		},
		{
			name:  "per-file errors do not abort the batch",
			input: `{"file_paths":["/a.go","/missing"]}`,
			responses: map[string]fakeViewResponse{
				"/a.go":    {content: "AAA"},
				"/missing": {content: "no such file", isErr: true},
			},
			want: []string{"===== /a.go =====", "AAA", "===== /missing =====", "ERROR: no such file"},
		},
		{
			name:    "empty list rejected",
			input:   `{"file_paths":[]}`,
			isError: true,
			want:    []string{"file_paths is required"},
		},
		{
			name:    "exceeds max files",
			input:   `{"file_paths":["` + strings.Repeat(`/a","`, maxMultiViewFiles) + `/over"]}`,
			isError: true,
			want:    []string{"too many files"},
		},
		{
			name:  "transport error surfaces as per-file error",
			input: `{"file_paths":["/a.go"]}`,
			responses: map[string]fakeViewResponse{
				"/a.go": {err: errors.New("disk on fire")},
			},
			want: []string{"ERROR: disk on fire"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fake := &fakeViewTool{responses: tt.responses}
			tool := NewMultiViewTool(fake)
			require.Equal(t, MultiViewToolName, tool.Info().Name)

			resp, err := tool.Run(t.Context(), fantasy.ToolCall{Input: tt.input})
			require.NoError(t, err)
			require.Equal(t, tt.isError, resp.IsError, "response: %s", resp.Content)
			for _, sub := range tt.want {
				require.Contains(t, resp.Content, sub)
			}
			for _, sub := range tt.notWant {
				require.NotContains(t, resp.Content, sub)
			}
		})
	}
}

// Sanity: order of response entries follows input order even when paths
// finish in a different order. We can't directly time-stagger fake reads
// without a real clock, so just verify the structural ordering by
// requesting a known sequence and checking offset positions.
func TestMultiViewTool_PreservesInputOrder(t *testing.T) {
	t.Parallel()
	fake := &fakeViewTool{
		responses: map[string]fakeViewResponse{
			"/x": {content: "x"},
			"/y": {content: "y"},
			"/z": {content: "z"},
		},
	}
	tool := NewMultiViewTool(fake)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{
		Input: `{"file_paths":["/z","/x","/y"]}`,
	})
	require.NoError(t, err)
	zPos := strings.Index(resp.Content, "===== /z =====")
	xPos := strings.Index(resp.Content, "===== /x =====")
	yPos := strings.Index(resp.Content, "===== /y =====")
	require.True(t, zPos < xPos && xPos < yPos, "out of order:\n%s", resp.Content)
}
