package tools

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathsEqual(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" {
		require.True(t, pathsEqual("/workspace/main.go", "/workspace/main.go"))
		require.False(t, pathsEqual("/workspace/main.go", "/workspace/MAIN.go"))
		require.False(t, pathsEqual(`/workspace\\main.go`, "/workspace/main.go"))
		return
	}

	require.True(t, pathsEqual(
		`C:/Users/Dev/project/main.py`,
		`c:\users\dev\project\main.py`,
	))
	require.True(t, pathsEqual(
		`file:///C:/Users/Dev/project/main.py`,
		`c:\users\dev\project\main.py`,
	))
	require.False(t, pathsEqual(
		`C:/Users/Dev/project/main.py`,
		`C:/Users/Dev/project/other.py`,
	))
}
