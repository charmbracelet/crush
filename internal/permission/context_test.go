package permission

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenSatisfies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		stored    string
		candidate string
		want      bool
	}{
		// Exact matches always satisfy.
		{"command:go", "command:go", true},
		{"path:/tmp", "path:/tmp", true},
		{"command!:sh -c echo", "command!:sh -c echo", true},

		// command: word-boundary prefix — stored is base command, candidate adds subcommand.
		{"command:go", "command:go test", true},
		{"command:go", "command:go build", true},
		{"command:kubectl", "command:kubectl get", true},
		{"command:kubectl get", "command:kubectl get pods", true},

		// command: must not match a different word.
		{"command:py", "command:python3", false},
		{"command:go", "command:golang", false},
		{"command:git", "command:gitk", false},

		// command: more-specific does not satisfy less-specific.
		{"command:go test", "command:go", false},

		// path: root / is parent of all absolute paths.
		{"path:/", "path:/etc/hosts", true},
		{"path:/", "path:/tmp/subdir/file.txt", true},
		// path:/  matches path:/ exactly (handled by the == branch above).
		{"path:/", "path:/", true},

		// path: directory-boundary prefix — stored is parent dir, candidate is child.
		{"path:/tmp", "path:/tmp/subdir", true},
		{"path:/tmp", "path:/tmp/subdir/deep", true},
		{"path:/home/user", "path:/home/user/projects/repo", true},

		// path: must not match a sibling with same prefix word.
		{"path:/tmp", "path:/tmpfiles", false},
		{"path:/home/user", "path:/home/users", false},

		// path: more-specific does not satisfy less-specific.
		{"path:/tmp/subdir", "path:/tmp", false},

		// Opaque command!: tokens — exact match only, no prefix extension.
		{"command!:sh -c echo", "command!:sh -c echo2", false},
		{"command!:sh -c echo", "command:echo", false},

		// Mixed kinds never match each other.
		{"command:go", "path:/usr/local/go", false},
		{"path:/tmp", "command:rm", false},
	}

	for _, tt := range tests {
		t.Run(tt.stored+"→"+tt.candidate, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tokenSatisfies(tt.stored, tt.candidate))
		})
	}
}
