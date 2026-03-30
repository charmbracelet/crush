package filepathext

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		require.Equal(t, "H:/Codes/crush", Normalize("/H:/Codes/crush"))
		require.Equal(t, "C:/", Normalize("/C:/"))
		require.Equal(t, "D:/repo", Normalize("D:/repo"))
		require.Equal(t, "/tmp/project", Normalize("/tmp/project"))
		return
	}

	require.Equal(t, "/H:/Codes/crush", Normalize("/H:/Codes/crush"))
}

func TestSmartJoin(t *testing.T) {
	t.Parallel()

	base := filepath.Clean("H:/Codes/crush")
	if runtime.GOOS == "windows" {
		require.Equal(t, filepath.Clean("H:/Codes/crush"), filepath.Clean(SmartJoin(base, "/H:/Codes/crush")))
		require.Equal(t, filepath.Clean("H:/Codes/crush/internal"), filepath.Clean(SmartJoin(base, "internal")))
		return
	}

	require.Equal(t, filepath.Clean("/H:/Codes/crush"), filepath.Clean(SmartJoin(base, "/H:/Codes/crush")))
}

func TestSmartIsAbs(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		require.True(t, SmartIsAbs("/H:/Codes/crush"))
		require.True(t, SmartIsAbs("H:/Codes/crush"))
		require.False(t, SmartIsAbs("internal/agent"))
		return
	}

	require.True(t, SmartIsAbs("/tmp/project"))
	require.False(t, SmartIsAbs("relative/path"))
}
