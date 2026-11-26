package filepathext

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitGlobPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern    string
		wantPrefix string
		wantRest   string
	}{
		{"*.go", "", "*.go"},
		{"**/foo.go", "", "**/foo.go"},
		{"internal/agent/*.go", "internal/agent", "*.go"},
		{"internal/**/*.go", "internal", "**/*.go"},
		{"a/b/c/*.txt", "a/b/c", "*.txt"},
		// A fully literal path walks its parent and matches the basename.
		{"internal/agent/glob.go", "internal/agent", "glob.go"},
		{"glob.go", "", "glob.go"},
		// A brace or bracket in the first segment means no literal prefix.
		{"{a,b}/x.go", "", "{a,b}/x.go"},
		// An absolute pattern keeps its root, so it is walked where it points.
		{"/etc/*.conf", "/etc", "*.conf"},
		{"/*.go", "/", "*.go"},
		{"/glob.go", "/", "glob.go"},
	}

	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			t.Parallel()
			gotPrefix, gotRest := SplitGlobPrefix(tc.pattern)
			require.Equal(t, tc.wantPrefix, gotPrefix, "prefix")
			require.Equal(t, tc.wantRest, gotRest, "rest")
		})
	}
}

func TestSplitGlobPrefixWindowsDriveRoot(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("Windows drive paths")
	}
	for _, pattern := range []string{`C:\*.go`, "C:/*.go"} {
		prefix, rest := SplitGlobPrefix(pattern)
		require.Equal(t, "C:/", prefix)
		require.Equal(t, "*.go", rest)
	}
}
