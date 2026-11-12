package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestDeniedCreateLeavesNoDirectories(t *testing.T) {
	t.Parallel()

	tests := map[string]func(t *testing.T, workingDir, filePath string){
		"write": func(t *testing.T, workingDir, filePath string) {
			tool := NewWriteTool(nil, &mockPermissionService{deny: true}, &mockHistoryService{}, mockFileTrackerService{}, workingDir)
			input, err := json.Marshal(WriteParams{FilePath: filePath, Content: "hello"})
			require.NoError(t, err)
			ctx := context.WithValue(t.Context(), SessionIDContextKey, "session")
			_, err = tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: WriteToolName, Input: string(input)})
			require.NoError(t, err)
		},
		"edit": func(t *testing.T, workingDir, filePath string) {
			_, err := createNewFile(deniedEditContext(t, workingDir), filePath, "hello", fantasy.ToolCall{ID: "call"})
			require.NoError(t, err)
		},
		"multiedit": func(t *testing.T, workingDir, filePath string) {
			params := MultiEditParams{
				FilePath: filePath,
				Edits:    []MultiEditOperation{{OldString: "", NewString: "hello"}},
			}
			_, err := processMultiEditWithCreation(deniedEditContext(t, workingDir), params, fantasy.ToolCall{ID: "call"})
			require.NoError(t, err)
		},
	}

	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			workingDir := t.TempDir()
			run(t, workingDir, filepath.Join(workingDir, "a", "b", "c.txt"))

			_, err := os.Stat(filepath.Join(workingDir, "a"))
			require.True(t, os.IsNotExist(err), "denied request created directories")
		})
	}
}

func deniedEditContext(t *testing.T, workingDir string) editContext {
	return editContext{
		ctx:         context.WithValue(t.Context(), SessionIDContextKey, "session"),
		permissions: &mockPermissionService{deny: true},
		files:       &mockHistoryService{},
		filetracker: &mockEditFileTracker{},
		workingDir:  workingDir,
	}
}
