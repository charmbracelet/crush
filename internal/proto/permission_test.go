package proto_test

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

// TestPermissionRequestParamsTypeAssertable guards the permission
// dialog's type assertions across the client/server boundary. The TUI
// asserts PermissionRequest.Params to tools.*PermissionsParams; when
// the request round-trips over the SSE wire (server → client), the
// decoded value must be the same Go type, otherwise the dialog
// renders empty content.
func TestPermissionRequestParamsTypeAssertable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		toolName string
		params   any
		assert   func(t *testing.T, got any)
	}{
		{
			name:     "bash",
			toolName: tools.BashToolName,
			params: tools.BashPermissionsParams{
				Description:     "list files",
				Command:         "ls -la",
				WorkingDir:      "/tmp",
				RunInBackground: false,
			},
			assert: func(t *testing.T, got any) {
				v, ok := got.(tools.BashPermissionsParams)
				require.True(t, ok, "params must decode as tools.BashPermissionsParams, got %T", got)
				require.Equal(t, "list files", v.Description)
				require.Equal(t, "ls -la", v.Command)
				require.Equal(t, "/tmp", v.WorkingDir)
			},
		},
		{
			name:     "edit",
			toolName: tools.EditToolName,
			params: tools.EditPermissionsParams{
				FilePath:   "/tmp/x.go",
				OldContent: "old",
				NewContent: "new",
			},
			assert: func(t *testing.T, got any) {
				v, ok := got.(tools.EditPermissionsParams)
				require.True(t, ok, "params must decode as tools.EditPermissionsParams, got %T", got)
				require.Equal(t, "/tmp/x.go", v.FilePath)
				require.Equal(t, "old", v.OldContent)
				require.Equal(t, "new", v.NewContent)
			},
		},
		{
			name:     "write",
			toolName: tools.WriteToolName,
			params: tools.WritePermissionsParams{
				FilePath:   "/tmp/x.go",
				NewContent: "new",
			},
			assert: func(t *testing.T, got any) {
				v, ok := got.(tools.WritePermissionsParams)
				require.True(t, ok, "params must decode as tools.WritePermissionsParams, got %T", got)
				require.Equal(t, "/tmp/x.go", v.FilePath)
				require.Equal(t, "new", v.NewContent)
			},
		},
		{
			name:     "multiedit",
			toolName: tools.MultiEditToolName,
			params: tools.MultiEditPermissionsParams{
				FilePath:   "/tmp/x.go",
				OldContent: "old",
				NewContent: "new",
			},
			assert: func(t *testing.T, got any) {
				v, ok := got.(tools.MultiEditPermissionsParams)
				require.True(t, ok, "params must decode as tools.MultiEditPermissionsParams, got %T", got)
				require.Equal(t, "/tmp/x.go", v.FilePath)
			},
		},
		{
			name:     "ls",
			toolName: tools.LSToolName,
			params: tools.LSPermissionsParams{
				Path:   "/tmp",
				Ignore: []string{".git"},
				Depth:  2,
			},
			assert: func(t *testing.T, got any) {
				v, ok := got.(tools.LSPermissionsParams)
				require.True(t, ok, "params must decode as tools.LSPermissionsParams, got %T", got)
				require.Equal(t, "/tmp", v.Path)
				require.Equal(t, []string{".git"}, v.Ignore)
				require.Equal(t, 2, v.Depth)
			},
		},
		{
			name:     "view",
			toolName: tools.ViewToolName,
			params: tools.ViewPermissionsParams{
				FilePath: "/tmp/x.go",
				Offset:   10,
				Limit:    100,
			},
			assert: func(t *testing.T, got any) {
				v, ok := got.(tools.ViewPermissionsParams)
				require.True(t, ok, "params must decode as tools.ViewPermissionsParams, got %T", got)
				require.Equal(t, "/tmp/x.go", v.FilePath)
			},
		},
		{
			name:     "fetch",
			toolName: tools.FetchToolName,
			params: tools.FetchPermissionsParams{
				URL:    "https://example.com",
				Format: "text",
			},
			assert: func(t *testing.T, got any) {
				v, ok := got.(tools.FetchPermissionsParams)
				require.True(t, ok, "params must decode as tools.FetchPermissionsParams, got %T", got)
				require.Equal(t, "https://example.com", v.URL)
			},
		},
		{
			name:     "download",
			toolName: tools.DownloadToolName,
			params: tools.DownloadPermissionsParams{
				URL:      "https://example.com/x.zip",
				FilePath: "/tmp/x.zip",
				Timeout:  30,
			},
			assert: func(t *testing.T, got any) {
				v, ok := got.(tools.DownloadPermissionsParams)
				require.True(t, ok, "params must decode as tools.DownloadPermissionsParams, got %T", got)
				require.Equal(t, "https://example.com/x.zip", v.URL)
				require.Equal(t, "/tmp/x.zip", v.FilePath)
			},
		},
		{
			name:     "agentic_fetch",
			toolName: tools.AgenticFetchToolName,
			params: tools.AgenticFetchPermissionsParams{
				URL:    "https://example.com",
				Prompt: "summarize this page",
			},
			assert: func(t *testing.T, got any) {
				v, ok := got.(tools.AgenticFetchPermissionsParams)
				require.True(t, ok, "params must decode as tools.AgenticFetchPermissionsParams, got %T", got)
				require.Equal(t, "https://example.com", v.URL)
				require.Equal(t, "summarize this page", v.Prompt)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Build a server-side request with the tool's concrete
			// params type, marshal to JSON (the wire path), then
			// decode back through proto.PermissionRequest.
			outbound := proto.PermissionRequest{
				ID:         "perm-1",
				SessionID:  "sess-1",
				ToolCallID: "call-1",
				ToolName:   tc.toolName,
				Path:       "/tmp",
				Params:     tc.params,
			}
			data, err := json.Marshal(outbound)
			require.NoError(t, err)

			var inbound proto.PermissionRequest
			require.NoError(t, json.Unmarshal(data, &inbound))

			tc.assert(t, inbound.Params)
		})
	}
}
