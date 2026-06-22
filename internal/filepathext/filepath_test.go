package filepathext

import (
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
