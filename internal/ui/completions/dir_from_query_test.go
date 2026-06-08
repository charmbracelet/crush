package completions

import (
	"testing"
	"path/filepath"

	"github.com/stretchr/testify/require"
)

func TestDirFromQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"empty query", "", "."},
		{"only at symbol", "@", "."},
		{"no slashes", "@foo", "."},
		{"root", "@/", string(filepath.Separator)},
		{"directory only", "@/foo/", "/foo"},
		{"directory with prefix", "@/foo/b", "/foo"},
		{"relative parent", "@../", ".."},
		{"relative parent with prefix", "@../foo", ".."},
		{"deep relative", "@../../foo/bar", "../../foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DirFromQuery(tt.query)
			require.Equal(t, tt.want, got)
		})
	}
}
