package util

import (
	"testing"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/stretchr/testify/require"
)

func TestNormalizeWindowsDrivePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"unix absolute", "/home/user/file.go", "/home/user/file.go"},
		{"unix relative", "file.go", "file.go"},
		{"windows drive backslash prefix", `\G:\_AIXM\AIXMUnity\GameManager.cs`, `G:\_AIXM\AIXMUnity\GameManager.cs`},
		{"windows drive slash prefix", "/G:/_AIXM/AIXMUnity/GameManager.cs", "G:/_AIXM/AIXMUnity/GameManager.cs"},
		{"windows drive no prefix", `G:\_AIXM\AIXMUnity\GameManager.cs`, `G:\_AIXM\AIXMUnity\GameManager.cs`},
		{"unc path unchanged", `\\server\share\file.go`, `\\server\share\file.go`},
		{"short string", "a:", "a:"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, normalizeWindowsDrivePath(tc.in))
		})
	}
}

func TestPathFromURIStripsWindowsDrivePrefix(t *testing.T) {
	t.Parallel()

	// A Windows file URI whose URI-to-path conversion leaves a leading path
	// separator in front of the drive letter must be normalized back to a
	// valid absolute path. See issue #3089.
	path, err := PathFromURI(protocol.DocumentURI("file:///G:/_AIXM/AIXMUnity/GameManager.cs"))
	require.NoError(t, err)
	require.Equal(t, "G:/_AIXM/AIXMUnity/GameManager.cs", path)
}

func TestPathFromURINonWindows(t *testing.T) {
	t.Parallel()

	path, err := PathFromURI(protocol.DocumentURI("file:///home/user/file.go"))
	require.NoError(t, err)
	require.Equal(t, "/home/user/file.go", path)
}
